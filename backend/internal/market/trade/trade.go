// Package trade defines the exchange-neutral live trade feed boundary.
package trade

import "time"

type Event struct {
	Symbol, Price string
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
	Events() <-chan Event
	Statuses() <-chan Status
	SetSymbols([]string)
}
