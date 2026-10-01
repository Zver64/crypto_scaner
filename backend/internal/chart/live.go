package chart

import (
	"context"
	"fmt"
	"slices"
	"time"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	marketlive "crypto-scanner/internal/market/live"
)

const (
	// A failed history build is retried on trades no more often than this.
	liveRetryDelay   = 10 * time.Second
	liveBuildTimeout = 5 * time.Second
	// tailReloadSize is how many of the newest closed candles a stream event
	// rereads from storage; it matches the live service's retained candles.
	tailReloadSize = 16
)

// Trigger is the stream event that asks a live session for a new frame.
// Triggers are ordered by strength, so a batch of events needs only one frame
// for the strongest of them.
type Trigger int

const (
	// TriggerTrade is a non-final update of the forming candle. It keeps the
	// closed range intact and yields a tail frame.
	TriggerTrade Trigger = iota
	// TriggerClose is a final candle from the stream. It rereads only the
	// newest stored candles of a complete cached range.
	TriggerClose
	// TriggerStream is a snapshot or correction, such as committed sync
	// history. It rebuilds the closed range.
	TriggerStream
	// TriggerRange is an explicit client range request. It always rebuilds
	// and always reports failures.
	TriggerRange
)

// Frame is one chart delivery. A snapshot replaces the client chart; a tail
// carries only candles and indicator points opened at or after From.
type Frame struct {
	Page     Page
	Snapshot bool
	From     time.Time
}

// LiveSession merges one stream's candle states with persisted closed history.
// It is not safe for concurrent use.
type LiveSession struct {
	service  *Service
	symbol   string
	interval market.CandleInterval
	states   map[time.Time]marketlive.CandleState
	closed   *Page
	retryAt  time.Time
	// instrument is resolved by the first successful load.
	instrument *market.Instrument
}

// NewLiveSession starts an empty session for one symbol and interval.
func (service *Service) NewLiveSession(symbol string, interval market.CandleInterval) *LiveSession {
	return &LiveSession{service: service, symbol: symbol, interval: interval, states: map[time.Time]marketlive.CandleState{}}
}

// Apply records a live-service message and returns the trigger for the next
// frame. ok is false for messages that carry no candle state, such as status.
func (session *LiveSession) Apply(message marketlive.Message) (trigger Trigger, ok bool) {
	switch {
	case message.Kind == marketlive.KindSnapshot:
		session.reset(message.Candles)
		return TriggerStream, true
	case message.Kind != marketlive.KindUpdate:
		return 0, false
	case message.Candle == nil:
		return TriggerStream, true
	}
	session.observe(*message.Candle)
	if message.Candle.Final {
		return TriggerClose, true
	}
	return TriggerTrade, true
}

// reset replaces the stream states with a snapshot.
func (session *LiveSession) reset(states []marketlive.CandleState) {
	session.states = make(map[time.Time]marketlive.CandleState, len(states))
	for _, state := range states {
		session.states[state.Candle.OpenTime] = state
	}
}

// observe records one streamed candle; a final state is never downgraded.
func (session *LiveSession) observe(state marketlive.CandleState) {
	if old := session.states[state.Candle.OpenTime]; !old.Final || state.Final {
		session.states[state.Candle.OpenTime] = state
	}
}

// Forget drops the closed range and any failure state after an unsubscribe.
func (session *LiveSession) Forget() {
	session.DropRange()
	session.retryAt = time.Time{}
}

// DropRange drops the closed range, e.g. after the client range changed, so
// the next frame rebuilds it. A failure in progress keeps its retry state.
func (session *LiveSession) DropRange() {
	session.closed = nil
}

// Next returns the frame for trigger. ok is false when nothing should be sent.
// A non-nil error must be reported to the client; it is returned once per
// failure and always for a range request.
func (session *LiveSession) Next(ctx context.Context, trigger Trigger, limit int, configs []indicator.Selection) (frame Frame, ok bool, err error) {
	cached := session.closed != nil
	var page Page
	if cached {
		page = *session.closed
	}
	rebuild := !cached || trigger != TriggerTrade
	if !cached && trigger == TriggerTrade && time.Now().Before(session.retryAt) {
		return Frame{}, false, nil
	}
	if rebuild {
		var loaded Page
		var err error
		if cached && trigger == TriggerClose {
			loaded, err = session.reloadTail(ctx, limit, configs, session.closed)
		} else {
			loaded, err = session.load(ctx, limit, configs, session.closed)
		}
		if err != nil && ctx.Err() != nil {
			// The client is gone; its cancellation is not a history failure.
			return Frame{}, false, nil
		}
		if err != nil {
			failing := !session.retryAt.IsZero()
			session.retryAt = time.Now().Add(liveRetryDelay)
			if !failing {
				session.service.logger.WarnContext(ctx, "chart history build failed", "symbol", session.symbol, "interval", session.interval, "error", err)
			}
			if !cached || trigger == TriggerRange {
				if !failing || trigger == TriggerRange {
					return Frame{}, false, err
				}
				return Frame{}, false, nil
			}
			// Keep the last closed range; final WS candles below still advance
			// it, and the result is delivered as a snapshot.
			loaded = page
		} else {
			session.retryAt = time.Time{}
		}
		page = loaded
	}
	var pending *market.Candle
	page.Candles, pending = session.merge(page.Candles)
	page.keepNewest(rangeSize(limit))
	candles := page.Candles
	page.NextBefore = market.CandlePage{Candles: candles, HasMore: page.HasMore}.NextBefore()
	if pending != nil && len(candles) > 0 {
		last := candles[len(candles)-1].OpenTime
		if pending.OpenTime.Before(last) || pending.OpenTime.After(session.interval.NextOpenTime(last)) {
			pending = nil
		}
	}
	session.closed = &page
	if !rebuild && pending == nil {
		return Frame{}, false, nil
	}
	extended, err := session.service.extend(page, session.interval, pending, configs)
	if err != nil {
		if trigger == TriggerRange {
			return Frame{}, false, fmt.Errorf("%w: calculate: %w", ErrInvalidRequest, err)
		}
		return Frame{}, false, nil
	}
	frame = Frame{Page: extended, Snapshot: rebuild}
	if !rebuild {
		frame.From = pending.OpenTime
	}
	return frame, true, nil
}

// load builds the closed range. An extended range keeps its oldest candle when
// new closes shift the window, up to MaxRange.
func (session *LiveSession) load(ctx context.Context, limit int, configs []indicator.Selection, cached *Page) (Page, error) {
	ctx, cancel := context.WithTimeout(ctx, liveBuildTimeout)
	defer cancel()
	request := Request{Symbol: session.symbol, Interval: session.interval, Limit: limit, Indicators: configs}
	extended := cached != nil && limit > DefaultRange && len(cached.Candles) > 0
	if extended && len(cached.Candles) > limit {
		request.Limit = min(len(cached.Candles), MaxRange)
	}
	loaded, err := session.build(ctx, request)
	if err != nil || !extended || len(loaded.Candles) == 0 || !loaded.HasMore {
		return loaded, err
	}
	oldest := cached.Candles[0].OpenTime
	cursor := oldest
	for cursor.Before(loaded.Candles[0].OpenTime) && request.Limit < MaxRange {
		request.Limit++
		cursor = session.interval.NextOpenTime(cursor)
	}
	if cursor.After(oldest) {
		return session.build(ctx, request)
	}
	return loaded, nil
}

// build loads the closed range, resolving the symbol only once per session.
func (session *LiveSession) build(ctx context.Context, request Request) (Page, error) {
	page, err := session.service.build(ctx, session.instrument, request, false)
	if err == nil {
		session.instrument = &page.instrument
	}
	return page, err
}

// reloadTail rereads only the newest closed candles of the cached range after
// a candle closes. An incomplete range or warm-up may have been backfilled and
// a gap repaired in storage since, so those fall back to a full load, as does
// a tail that does not line up. Older corrections arrive as stream snapshots.
func (session *LiveSession) reloadTail(ctx context.Context, limit int, configs []indicator.Selection, cached *Page) (Page, error) {
	warmup, err := session.service.warmup(configs)
	if err != nil || session.instrument == nil || len(cached.Candles) < max(limit, tailReloadSize) ||
		len(cached.warmup) < warmup || !session.contiguous(cached) {
		return session.load(ctx, limit, configs, cached)
	}
	loadCtx, cancel := context.WithTimeout(ctx, liveBuildTimeout)
	defer cancel()
	tail, err := session.service.store.ListCandlePage(loadCtx, session.instrument.ID, session.interval, nil, tailReloadSize)
	if err != nil {
		return Page{}, fmt.Errorf("list chart tail: %w", err)
	}
	if len(tail.Candles) == 0 {
		return session.load(ctx, limit, configs, cached)
	}
	index, found := slices.BinarySearchFunc(cached.Candles, tail.Candles[0].OpenTime, func(candle market.Candle, target time.Time) int {
		return candle.OpenTime.Compare(target)
	})
	if !found {
		return session.load(ctx, limit, configs, cached)
	}
	page := *cached
	page.Candles = append(slices.Clip(cached.Candles[:index]), tail.Candles...)
	page.keepNewest(rangeSize(limit))
	// A full load reads exactly the largest lookback before the range.
	if len(page.warmup) > warmup {
		page.warmup = page.warmup[len(page.warmup)-warmup:]
	}
	return page, nil
}

// rangeSize is the most closed candles a range of limit shows. The initial
// range stays fixed; an extended one keeps its oldest candle but never exceeds
// the maximum chart range.
func rangeSize(limit int) int {
	if limit == DefaultRange {
		return limit
	}
	return MaxRange
}

// keepNewest moves candles beyond the newest count into the warm-up, since
// candles leaving the range still warm up the indicators.
func (page *Page) keepNewest(count int) {
	if len(page.Candles) <= count {
		return
	}
	shift := len(page.Candles) - count
	page.warmup = append(slices.Clip(page.warmup), page.Candles[:shift]...)
	page.Candles = page.Candles[shift:]
	page.HasMore = true
}

// contiguous reports whether the warm-up and range candles have no gaps.
func (session *LiveSession) contiguous(page *Page) bool {
	var previous time.Time
	for _, candles := range [][]market.Candle{page.warmup, page.Candles} {
		for _, candle := range candles {
			if !previous.IsZero() && !session.interval.NextOpenTime(previous).Equal(candle.OpenTime) {
				return false
			}
			previous = candle.OpenTime
		}
	}
	return true
}

// merge applies stream states to closed history. Final WS candles take
// precedence over stale REST history, while a persisted close wins over a
// delayed WS final. Until the previous candle is confirmed final, a newer
// trade still updates its price instead of opening the next candle.
func (session *LiveSession) merge(history []market.Candle) ([]market.Candle, *market.Candle) {
	keys := make([]time.Time, 0, len(session.states))
	for key := range session.states {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, time.Time.Compare)
	candles := append([]market.Candle(nil), history...)
	var pending *market.Candle
	for _, key := range keys {
		state := session.states[key]
		value := state.Candle
		if !state.Final {
			if len(candles) > 0 && !key.After(candles[len(candles)-1].OpenTime) {
				continue
			}
			if pending == nil {
				pending = &value
			} else if value.OpenTime.After(pending.OpenTime) {
				// Keep the candle valid as the newer price leaves its range.
				pending.Close = value.Close
				pending.High = max(pending.High, value.High)
				pending.Low = min(pending.Low, value.Low)
			}
			continue
		}
		if len(candles) == 0 || session.interval.NextOpenTime(candles[len(candles)-1].OpenTime).Equal(key) {
			candles = append(candles, value)
		}
		if pending != nil && pending.OpenTime.Equal(key) {
			pending = nil
		}
	}
	return candles, pending
}
