package market

import "context"

// MarketStore is the persistence seam used by synchronization and analysis.
type MarketStore interface {
	ApplyInstrumentSnapshot(context.Context, []Instrument) error
	ListActiveInstruments(context.Context) ([]Instrument, error)
	UpsertCandles(context.Context, []Candle) error
	ListLatestCandlesByInterval(context.Context, int64, string, int) ([]Candle, error)
	GetSyncState(context.Context, SyncProfile) (SyncState, error)
	SaveSyncState(context.Context, SyncState) error
}
