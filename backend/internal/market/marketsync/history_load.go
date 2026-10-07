package marketsync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"crypto-scanner/internal/market"
)

// HistoryLoad is the stored history of one instrument and interval after an
// on-demand load.
type HistoryLoad struct {
	Symbol   string
	Interval market.CandleInterval
	// Count is the number of stored candles, at most market.RetentionDepth,
	// and Oldest the open time of the oldest of them; it is zero without
	// candles.
	Count  int
	Oldest time.Time
	// Exhausted reports that the exchange has no candles before Oldest.
	Exhausted bool
	// Changed reports committed candle changes, including when a later page
	// fails. The other fields are a completed result only when err is nil.
	Changed bool
	// NotReady reports a history skipped by a HistoryLoader because it has
	// no candles yet: synchronization owns the initial backfill.
	NotReady bool
}

// LoadHistory extends the stored history of instrument backwards, page by
// page through the history repair rate limit, until depth candles, at most
// market.RetentionDepth, are stored
// or the exchange has no older ones, which is recorded as coverage like the
// exhaustion Sync finds. A history that already has them, or whose current
// coverage says the exchange has nothing older, costs no request.
//
// changed receives committed changes while filling a history shorter than
// SyncDepth; deeper pages do not affect live values and notify nothing. The
// run lock is not taken, and a concurrent Sync at worst writes the same
// candles. An instrument without candles fails with ErrHistoryNotReady:
// Sync owns the initial backfill.
func (synchronizer *Synchronizer) LoadHistory(ctx context.Context, instrument market.Instrument, depth int, changed func([]market.Candle)) (HistoryLoad, error) {
	interval := synchronizer.profile.Interval
	// Coverage is shared with Sync, so it is read and written under its
	// policy: the oldest open time the exchange has does not depend on the
	// depth requested.
	policy := policyForInterval(interval, synchronizer.depth)
	startedAt := time.Now().UTC()
	history, err := synchronizer.summary(ctx, instrument, market.RetentionDepth)
	if err != nil {
		return HistoryLoad{}, err
	}
	if history.Count == 0 {
		return HistoryLoad{}, fmt.Errorf("%w: %s %s", ErrHistoryNotReady, instrument.Symbol, interval)
	}
	coverages, err := synchronizer.store.ListCandleHistoryCoverage(ctx, []int64{instrument.ID}, interval)
	if err != nil {
		return HistoryLoad{}, fmt.Errorf("load candle history coverage for %s: %w", instrument.Symbol, err)
	}
	coverage, covered := coverages[instrument.ID]
	exhausted := covered && coverageCurrent(coverage, policy, startedAt) && coverage.VerifiedOldestOpenTime.Equal(history.Oldest)
	historyChanged := false
	// Stop at the live-depth boundary before fetching deeper pages. This
	// reuses committed-change notifications without notifying for a page's
	// older tail, and a fresh summary also accounts for concurrent Sync.
	for _, target := range []int{min(depth, market.SyncDepth), depth} {
		if history.Count >= target || exhausted {
			continue
		}
		store := synchronizer.store
		if target <= market.SyncDepth {
			store = ObservableStore{Store: store, Changed: changed}
		}
		loaded, ranOut := synchronizer.loadOlder(ctx, store, instrument, policy, history.Oldest.UTC(), target-history.Count, startedAt)
		historyChanged, exhausted = historyChanged || loaded.rowsChanged > 0, ranOut
		if loaded.err != nil {
			return HistoryLoad{Changed: historyChanged}, loaded.err
		}
		if history, err = synchronizer.summary(ctx, instrument, market.RetentionDepth); err != nil {
			return HistoryLoad{Changed: historyChanged}, err
		}
	}
	return HistoryLoad{Symbol: instrument.Symbol, Interval: interval, Count: history.Count, Oldest: history.Oldest, Exhausted: exhausted, Changed: historyChanged}, nil
}

var (
	// ErrInvalidHistoryLoad rejects a history load with repeated symbols or
	// a depth outside MinHistoryLoadDepth to market.RetentionDepth.
	ErrInvalidHistoryLoad = errors.New("invalid history load")
	// ErrHistoryLoadRunning rejects a history load while another one runs.
	ErrHistoryLoadRunning = errors.New("a history load is running")
	// ErrHistoryNotReady leaves the initial backfill to synchronization.
	ErrHistoryNotReady = errors.New("candle history has not been synchronized yet")
)

// MinHistoryLoadDepth is the shallowest depth worth loading: synchronization
// already keeps market.SyncDepth candles.
const MinHistoryLoadDepth = market.SyncDepth + 1

// HistoryJobStatus is the state of a history load job.
type HistoryJobStatus string

const (
	HistoryJobRunning HistoryJobStatus = "running"
	HistoryJobDone    HistoryJobStatus = "done"
	HistoryJobFailed  HistoryJobStatus = "failed"
)

// HistoryJob is a history load: what it loads and, as it progresses, the
// stored history of each symbol and interval it finished, by symbol and then
// interval. FinishedAt is zero and Error empty while it runs.
type HistoryJob struct {
	Status     HistoryJobStatus
	Symbols    []string
	Intervals  []market.CandleInterval
	Depth      int
	StartedAt  time.Time
	FinishedAt time.Time
	Items      []HistoryLoad
	// HistoryChanged includes partial loads that failed after committing pages.
	HistoryChanged bool
	Error          string
}

func (job HistoryJob) clone() HistoryJob {
	job.Symbols, job.Intervals, job.Items = slices.Clone(job.Symbols), slices.Clone(job.Intervals), slices.Clone(job.Items)
	return job
}

// Instruments resolves the instruments a history load names.
type Instruments interface {
	// GetActiveInstrumentBySymbol fails with market.ErrInstrumentNotFound.
	GetActiveInstrumentBySymbol(context.Context, string) (market.Instrument, error)
}

// HistoryExtender extends the stored history of one interval; Synchronizer
// is one. Its result reports Changed even when the load fails.
type HistoryExtender interface {
	LoadHistory(ctx context.Context, instrument market.Instrument, depth int, changed func([]market.Candle)) (HistoryLoad, error)
}

// HistoryLoaderOptions connects committed live-window changes to consumers.
// Both callbacks must be non-blocking; Synced ends a load that notified Changed,
// even when a later page failed.
type HistoryLoaderOptions struct {
	Changed func([]market.Candle)
	Synced  func()
}

// HistoryLoader runs one history load at a time in the background, for the
// whole process lifetime, and keeps the current or last job in memory only.
type HistoryLoader struct {
	instruments Instruments
	extenders   map[market.CandleInterval]HistoryExtender
	logger      *slog.Logger
	options     HistoryLoaderOptions
	// started hands Run the instruments of the job Start began; it holds
	// one, and only Run's finish of that job lets Start begin another.
	started chan []market.Instrument

	mu  sync.Mutex
	job *HistoryJob
}

// NewHistoryLoader creates a loader that extends histories with the extender
// of each supported interval.
func NewHistoryLoader(instruments Instruments, extenders map[market.CandleInterval]HistoryExtender, logger *slog.Logger, options HistoryLoaderOptions) *HistoryLoader {
	return &HistoryLoader{instruments: instruments, extenders: extenders, logger: logger.With("module", "market_history_load"), options: options, started: make(chan []market.Instrument, 1)}
}

// Start validates a history load of every symbol on every supported interval
// to depth candles and begins it in the background, returning the new job.
// It fails with ErrInvalidHistoryLoad, market.ErrInstrumentNotFound when a
// symbol is not an active instrument, or ErrHistoryLoadRunning; it never
// blocks on the load.
func (loader *HistoryLoader) Start(ctx context.Context, symbols []string, intervals []market.CandleInterval, depth int) (HistoryJob, error) {
	if depth < MinHistoryLoadDepth || depth > market.RetentionDepth {
		return HistoryJob{}, fmt.Errorf("%w: the depth must be %d to %d candles", ErrInvalidHistoryLoad, MinHistoryLoadDepth, market.RetentionDepth)
	}
	if len(intervals) == 0 {
		return HistoryJob{}, fmt.Errorf("%w: no interval", ErrInvalidHistoryLoad)
	}
	for index, interval := range intervals {
		if loader.extenders[interval] == nil || slices.Contains(intervals[:index], interval) {
			return HistoryJob{}, fmt.Errorf("%w: intervals must be distinct and supported", ErrInvalidHistoryLoad)
		}
	}
	instruments := make([]market.Instrument, len(symbols))
	for index, symbol := range symbols {
		symbol = market.NormalizeSymbol(symbol)
		if slices.ContainsFunc(instruments[:index], func(instrument market.Instrument) bool { return instrument.Symbol == symbol }) {
			return HistoryJob{}, fmt.Errorf("%w: symbols must be distinct", ErrInvalidHistoryLoad)
		}
		instrument, err := loader.instruments.GetActiveInstrumentBySymbol(ctx, symbol)
		if err != nil {
			return HistoryJob{}, fmt.Errorf("resolve %s: %w", symbol, err)
		}
		instruments[index] = instrument
	}
	loader.mu.Lock()
	defer loader.mu.Unlock()
	if loader.job != nil && loader.job.Status == HistoryJobRunning {
		return HistoryJob{}, ErrHistoryLoadRunning
	}
	job := &HistoryJob{Status: HistoryJobRunning, Intervals: slices.Clone(intervals), Depth: depth, StartedAt: time.Now().UTC()}
	for _, instrument := range instruments {
		job.Symbols = append(job.Symbols, instrument.Symbol)
	}
	// Run took the previous job before finishing it, so the channel has
	// room; the default only keeps a handler from ever blocking.
	select {
	case loader.started <- instruments:
	default:
		return HistoryJob{}, ErrHistoryLoadRunning
	}
	loader.job = job
	return job.clone(), nil
}

// Job returns the current or last job, false when none ran since startup.
func (loader *HistoryLoader) Job() (HistoryJob, bool) {
	loader.mu.Lock()
	defer loader.mu.Unlock()
	if loader.job == nil {
		return HistoryJob{}, false
	}
	return loader.job.clone(), true
}

// Run runs the jobs Start begins until ctx is cancelled, which fails the
// running job.
func (loader *HistoryLoader) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case instruments := <-loader.started:
			loader.run(ctx, instruments)
		}
	}
}

func (loader *HistoryLoader) run(ctx context.Context, instruments []market.Instrument) {
	loader.mu.Lock()
	intervals, depth := loader.job.Intervals, loader.job.Depth
	loader.mu.Unlock()
	err := func() error {
		for _, instrument := range instruments {
			for _, interval := range intervals {
				liveChanged := false
				load, err := loader.extenders[interval].LoadHistory(ctx, instrument, depth, func(candles []market.Candle) {
					liveChanged = true
					if loader.options.Changed != nil {
						loader.options.Changed(candles)
					}
				})
				if liveChanged && loader.options.Synced != nil {
					loader.options.Synced()
				}
				if errors.Is(err, ErrHistoryNotReady) {
					// One new coin or interval does not stop the others.
					load, err = HistoryLoad{Symbol: instrument.Symbol, Interval: interval, NotReady: true}, nil
				}
				loader.mu.Lock()
				loader.job.HistoryChanged = loader.job.HistoryChanged || load.Changed
				if err == nil {
					loader.job.Items = append(loader.job.Items, load)
				}
				loader.mu.Unlock()
				if err != nil {
					return err
				}
			}
		}
		return nil
	}()
	loader.mu.Lock()
	defer loader.mu.Unlock()
	loader.job.FinishedAt = time.Now().UTC()
	switch {
	case err == nil:
		loader.job.Status = HistoryJobDone
		loader.logger.InfoContext(ctx, "history load finished", "operation", "history_load", "outcome", "success", "depth", depth, "loads", len(loader.job.Items))
	case ctx.Err() != nil:
		loader.job.Status, loader.job.Error = HistoryJobFailed, "stopped by a server shutdown"
		loader.logger.InfoContext(ctx, "history load stopped by shutdown", "operation", "history_load", "outcome", "cancelled", "loads", len(loader.job.Items))
	default:
		// The cause stays in the log.
		loader.job.Status, loader.job.Error = HistoryJobFailed, "the history load failed; see the server log"
		loader.logger.WarnContext(ctx, "history load failed", "operation", "history_load", "outcome", "failure", "error", err)
	}
}
