package market

import "context"

// MarketStore is the persistence seam used by synchronization and analysis.
type MarketStore interface {
	ApplyInstrumentSnapshot(context.Context, []Instrument) error
	ListActiveInstruments(context.Context) ([]Instrument, error)
	UpsertCandles(context.Context, []Candle) error
	ListLatestCandles(context.Context, int64, int) ([]Candle, error)
	GetSyncState(context.Context, SyncProfile) (SyncState, error)
	SaveSyncState(context.Context, SyncState) error
}
