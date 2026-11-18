// Package kline defines the exchange-neutral live candle feed boundary.
package kline

import (
	"time"

	"crypto-scanner/internal/market"
)

// Key identifies one live candle stream.
type Key struct {
	Symbol   string
	Interval market.CandleInterval
}

// Event is a complete exchange snapshot of one forming or final candle.
type Event struct {
	Key       Key
	Candle    market.Candle
	Final     bool
	EventTime time.Time
	// Epoch identifies the upstream connection that delivered the event.
	Epoch int64
}

// Status reports freshness for the keys owned by one upstream connection.
type Status struct {
	Keys      []Key
	Connected bool
	// Epoch identifies the connection attempt the status is about; a later
	// attempt has a larger epoch.
	Epoch int64
	Err   error
}

// Feed is a dynamically subscribed live candle source.
type Feed interface {
	Subscribe(Key)
	Unsubscribe(Key)
	Events() <-chan Event
	// StatusesReady signals, without ever blocking the feed, that
	// DrainStatuses has statuses.
	StatusesReady() <-chan struct{}
	// DrainStatuses returns and clears the pending statuses in the order they
	// happened. Statuses never wait for the consumer: per key, a status
	// replaces the pending ones, except that a connection keeps the pending
	// disconnect before it.
	DrainStatuses() []Status
}
