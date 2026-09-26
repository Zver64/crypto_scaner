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
}

// Status reports freshness for the keys owned by one upstream connection.
type Status struct {
	Keys      []Key
	Connected bool
	Err       error
}

// Feed is a dynamically subscribed live candle source.
type Feed interface {
	Subscribe(Key) error
	Unsubscribe(Key)
	Events() <-chan Event
	Statuses() <-chan Status
}
