package chart

import (
	"context"
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
)

// Trigger is the stream event that asks a live session for a new frame.
type Trigger int

const (
	// TriggerTrade is a non-final update of the forming candle. It keeps the
	// closed range intact and yields a tail frame.
	TriggerTrade Trigger = iota
	// TriggerStream is a snapshot, final candle, or correction. It rebuilds
	// the closed range.
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
		return TriggerStream, true
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
		loaded, err := session.load(ctx, limit, configs, session.closed)
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
	candles, pending := session.merge(page.Candles)
	// The initial range stays fixed; an extended one keeps its oldest candle
	// but never exceeds the maximum chart range.
	keep := MaxRange
	if limit == DefaultRange {
		keep = limit
	}
	if len(candles) > keep {
		candles = candles[len(candles)-keep:]
		page.HasMore = true
	}
	page.Candles = candles
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
	loaded, err := session.service.Build(ctx, request)
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
		return session.service.Build(ctx, request)
	}
	return loaded, nil
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
