package marketsync

import (
	"context"
	"maps"
	"sync"
	"sync/atomic"

	"crypto-scanner/internal/market"
)

// ObservableStore is the one store every Synchronizer and Extender of the
// process writes candles through, and the source of their events: committed
// changes of the latest market.SyncDepth candles, each read of the active
// instruments that finds another set of them (by ID and symbol, in any order)
// than it last reported, and the end of a batch of candle changes of one
// interval. Listeners must not block.
// Every write of the latest market.SyncDepth candles notifies
// OnCandlesChanged: the closed indicator windows rely on it and never reread
// the store.
type ObservableStore struct {
	Store

	candlesChanged     listener[[]market.Candle]
	instrumentsChanged listener[[]market.Instrument]
	historySynced      listener[market.CandleInterval]

	// mu orders reads of the active instruments, so notifications follow
	// them; last is the set last reported, nil before the first report.
	mu   sync.Mutex
	last map[instrumentKey]struct{}
}

// NewObservableStore wraps store; until a listener is set, its events are
// dropped.
func NewObservableStore(store Store) *ObservableStore {
	return &ObservableStore{Store: store}
}

// OnCandlesChanged sets the consumer of committed closed-candle changes of the
// live history, replacing any earlier one.
func (store *ObservableStore) OnCandlesChanged(listener func([]market.Candle)) {
	store.candlesChanged.set(listener)
}

// OnInstrumentsChanged sets the consumer of the active instruments after
// their set changed, replacing any earlier one. It receives all of them.
func (store *ObservableStore) OnInstrumentsChanged(listener func([]market.Instrument)) {
	store.instrumentsChanged.set(listener)
}

// OnHistorySynced sets the consumer of the end of a batch of candle changes
// of one interval, replacing any earlier one: a synchronization round, even a
// failed one, which may have committed some candles, or a history load that
// changed the live history, so consumers process its changes together.
func (store *ObservableStore) OnHistorySynced(listener func(market.CandleInterval)) {
	store.historySynced.set(listener)
}

type instrumentKey struct {
	id     int64
	symbol string
}

// UpsertCandlesWithChanges notifies the committed changes.
func (store *ObservableStore) UpsertCandlesWithChanges(ctx context.Context, candles []market.Candle) ([]market.Candle, error) {
	changed, err := store.Store.UpsertCandlesWithChanges(ctx, candles)
	if err != nil {
		return nil, err
	}
	if len(changed) > 0 {
		store.candlesChanged.notify(changed)
	}
	return changed, nil
}

// upsertOlderCandles writes candles older than the latest market.SyncDepth
// ones, which no live value reads, so it notifies nothing.
func (store *ObservableStore) upsertOlderCandles(ctx context.Context, candles []market.Candle) ([]market.Candle, error) {
	return store.Store.UpsertCandlesWithChanges(ctx, candles)
}

// ListActiveInstrumentsByMarketCap reports the active instruments it reads
// when their set differs from the last report.
func (store *ObservableStore) ListActiveInstrumentsByMarketCap(ctx context.Context) ([]market.Instrument, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	active, err := store.Store.ListActiveInstrumentsByMarketCap(ctx)
	if err != nil {
		return nil, err
	}
	set := make(map[instrumentKey]struct{}, len(active))
	for _, instrument := range active {
		set[instrumentKey{instrument.ID, instrument.Symbol}] = struct{}{}
	}
	if store.last == nil || !maps.Equal(store.last, set) {
		store.instrumentsChanged.notify(active)
	}
	store.last = set
	return active, nil
}

// synced reports the end of a batch of candle changes of interval.
func (store *ObservableStore) synced(interval market.CandleInterval) {
	store.historySynced.notify(interval)
}

// listener holds one consumer of events of type T, nil until it is set.
type listener[T any] struct{ consumer atomic.Pointer[func(T)] }

func (listener *listener[T]) set(consumer func(T)) { listener.consumer.Store(&consumer) }

func (listener *listener[T]) notify(value T) {
	if consumer := listener.consumer.Load(); consumer != nil && *consumer != nil {
		(*consumer)(value)
	}
}
