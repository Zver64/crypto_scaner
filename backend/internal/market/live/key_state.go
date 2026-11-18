package live

import (
	"slices"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/kline"
)

// keyState is one live key: its clients, its bounded candle buffer, its
// upstream connectivity, and the gap the stream missed. Its methods are pure
// state transitions; Service holds service.mu around them and publishes.
type keyState struct {
	instrument      market.Instrument
	clients         map[string]Client
	candles         map[time.Time]CandleState
	latestConfirmed time.Time // REST-confirmed high-water mark; independent of clock skew
	// releaseAt is when Run releases the key if it still has no clients; zero
	// while clients remain.
	releaseAt time.Time
	// seedRequested asks Run to read the forming candle of a new key.
	seedRequested bool

	// link is the upstream connectivity of the key. Only connections newer
	// than disconnectEpoch, the newest connection reported down, connect it,
	// so buffered events of a dead connection never do.
	link            upstreamLink
	disconnectEpoch int64
	// reconnectFrom is the first candle a reconnected stream may have missed;
	// the next accepted candle turns it into the gap.
	reconnectFrom *time.Time
	// gap are the closed candles the stream missed and storage has not
	// confirmed yet. The key is recovering until committed candles cover it.
	gap gap
	// readWanted asks for one read of the stored candles covering the gap:
	// set when the gap grows or a read fails. reading is set while a read runs.
	readWanted bool
	reading    bool
}

func newKeyState(instrument market.Instrument) *keyState {
	return &keyState{instrument: instrument, clients: make(map[string]Client), candles: make(map[time.Time]CandleState), seedRequested: true}
}

type upstreamLink int

const (
	linkWaiting upstreamLink = iota
	linkConnected
	linkDisconnected
)

// freshness derives the freshness clients see from the key's state.
func (state *keyState) freshness() Freshness {
	switch {
	case state.link == linkDisconnected:
		return FreshnessStale
	case state.link == linkWaiting || len(state.candles) == 0:
		return FreshnessWaiting
	case !state.gap.empty() || state.reconnectFrom != nil:
		return FreshnessRecovering
	default:
		return FreshnessFresh
	}
}

// applyStatus records an upstream connectivity change of the key.
func (state *keyState) applyStatus(interval market.CandleInterval, status kline.Status) {
	if status.Connected {
		state.connectedBy(interval, status.Epoch)
		return
	}
	state.link = linkDisconnected
	state.disconnectEpoch = max(state.disconnectEpoch, status.Epoch)
}

// connectedBy marks the key connected by data or a status of connection
// epoch, unless a connection at least as new was reported down. A reconnect
// recovers from the latest buffered candle, whichever signal proved it.
func (state *keyState) connectedBy(interval market.CandleInterval, epoch int64) {
	if epoch <= state.disconnectEpoch {
		return
	}
	if state.link == linkDisconnected && len(state.candles) != 0 {
		latest := latestOpen(state.candles)
		from := latest
		if state.candles[latest].Final {
			from = interval.NextOpenTime(latest)
		}
		state.reconnectFrom = &from
	}
	state.link = linkConnected
}

// receive applies a stream event and reports whether its candle entered the
// buffer. Data proves its connection alive even when the candle is refused.
// The first candle after a reconnect, or one after a hole in the buffer,
// extends the gap by the closed candles the stream missed.
func (state *keyState) receive(event kline.Event) (CandleState, bool) {
	interval := event.Key.Interval
	state.connectedBy(interval, event.Epoch)
	incoming := CandleState{Candle: event.Candle, Final: event.Final}
	if !state.accepts(incoming) {
		return incoming, false
	}
	open := event.Candle.OpenTime
	previousLatest := latestOpen(state.candles)
	previous, hadPrevious := state.candles[previousLatest]
	state.candles[open] = incoming
	// Evicted candles need no recovery: their corrections arrive through
	// HistoryChanged.
	trimCandles(state.candles)
	var from time.Time
	switch {
	case state.reconnectFrom != nil:
		from = *state.reconnectFrom
		state.reconnectFrom = nil
	case hadPrevious && open.After(interval.NextOpenTime(previousLatest)):
		from = previousLatest
		if previous.Final {
			from = interval.NextOpenTime(previousLatest)
		}
	}
	if state.gap.extend(from, closedThrough(interval, incoming)) {
		state.readWanted = true
	}
	return incoming, true
}

// accepts reports whether incoming may enter the buffer.
func (state *keyState) accepts(incoming CandleState) bool {
	open := incoming.Candle.OpenTime
	if incoming.Final && !state.latestConfirmed.IsZero() && !open.After(state.latestConfirmed) {
		return false
	}
	current, exists := state.candles[open]
	if exists {
		// A final candle is never rolled back to an open one.
		return !current.Final || incoming.Final
	}
	// Old packets cannot re-enter the bounded buffer after their REST
	// confirmation has been evicted and replace persisted chart history.
	if len(state.candles) < maxRetainedCandles {
		return true
	}
	for buffered := range state.candles {
		if buffered.Before(open) {
			return true
		}
	}
	return false
}

// commit applies committed closed candles of the key: they replace buffered
// candles, fill the gap, and cover it through the newest of them.
func (state *keyState) commit(interval market.CandleInterval, candles []market.Candle) {
	var newest time.Time
	for _, candle := range candles {
		open := candle.OpenTime
		state.latestConfirmed = maxTime(state.latestConfirmed, open)
		if _, exists := state.candles[open]; exists || state.gap.contains(open) {
			state.candles[open] = CandleState{Candle: candle, Final: true}
		}
		newest = maxTime(newest, open)
	}
	trimCandles(state.candles)
	state.gap.storedThrough(interval, newest)
}

// restore applies stored candles read for the gap: they fill it where the
// buffer lacks a final candle and cover it through the newest of them.
func (state *keyState) restore(interval market.CandleInterval, candles []market.Candle) {
	var newest time.Time
	for _, candle := range candles {
		open := candle.OpenTime
		if current, exists := state.candles[open]; state.gap.contains(open) && (!exists || !current.Final) {
			state.candles[open] = CandleState{Candle: candle, Final: true}
		}
		newest = maxTime(newest, open)
	}
	trimCandles(state.candles)
	state.gap.storedThrough(interval, newest)
}

// gap is an inclusive range of candle open times; the zero value is empty.
type gap struct{ from, through time.Time }

func (g gap) empty() bool { return g.from.IsZero() }

func (g gap) contains(open time.Time) bool {
	return !g.empty() && !open.Before(g.from) && !open.After(g.through)
}

// extend adds the range from..through and reports whether the gap grew.
func (g *gap) extend(from, through time.Time) bool {
	if from.IsZero() || through.Before(from) {
		return false
	}
	if g.empty() {
		*g = gap{from: from, through: through}
		return true
	}
	grown := gap{from: minTime(g.from, from), through: maxTime(g.through, through)}
	if grown == *g {
		return false
	}
	*g = grown
	return true
}

// storedThrough removes what storage holds once it holds the candle opened
// at open: synchronization writes an instrument's newest closed candles
// together, so a stored candle has every older one stored before it, and a
// hole below it is a period without exchange data.
func (g *gap) storedThrough(interval market.CandleInterval, open time.Time) {
	if g.empty() || open.Before(g.from) {
		return
	}
	next := interval.NextOpenTime(open)
	if next.After(g.through) {
		*g = gap{}
		return
	}
	g.from = next
}

func closedThrough(interval market.CandleInterval, candle CandleState) time.Time {
	if candle.Final {
		return candle.Candle.OpenTime
	}
	return interval.PreviousOpenTime(candle.Candle.OpenTime)
}

func latestOpen(values map[time.Time]CandleState) time.Time {
	var latest time.Time
	for open := range values {
		if open.After(latest) {
			latest = open
		}
	}
	return latest
}

func trimCandles(values map[time.Time]CandleState) {
	if len(values) <= maxRetainedCandles {
		return
	}
	keys := make([]time.Time, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, time.Time.Compare)
	for _, key := range keys[:len(keys)-maxRetainedCandles] {
		delete(values, key)
	}
}

func minTime(left, right time.Time) time.Time {
	if right.Before(left) {
		return right
	}
	return left
}

func maxTime(left, right time.Time) time.Time {
	if right.After(left) {
		return right
	}
	return left
}
