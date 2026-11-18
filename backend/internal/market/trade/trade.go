// Package trade defines the exchange-neutral live trade feed boundary.
package trade

import "time"

// Event summarizes the trades of one symbol on one connection epoch since the
// previous event. Every traded price lies within [Low, High]; Price, TradeID,
// and EventTime belong to the last trade. A price crossed between consecutive
// trades is therefore visible from the range alone.
type Event struct {
	Symbol, Price string
	Low, High     string
	TradeID       int64
	Epoch         int64
	EventTime     time.Time
}

type Status struct {
	Symbols   []string
	Connected bool
	Err       error
}

type Feed interface {
	// Ready signals, without ever blocking the feed, that Drain has events.
	Ready() <-chan struct{}
	// Drain returns and clears the pending events. Trades never wait for the
	// consumer: they merge into the pending event of their symbol and epoch.
	Drain() []Event
	// StatusesReady signals, without ever blocking the feed, that
	// DrainStatuses has statuses.
	StatusesReady() <-chan struct{}
	// DrainStatuses returns and clears the pending statuses in the order they
	// happened. Statuses never wait for the consumer: per symbol, a status
	// replaces the pending ones, except that a connection keeps the pending
	// disconnect before it.
	DrainStatuses() []Status
	SetSymbols([]string)
}
