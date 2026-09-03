package market

import (
	"context"
	"time"
)

// MarketStore is the persistence seam used by synchronization and analysis.
type MarketStore interface {
	ApplyInstrumentSnapshot(context.Context, []Instrument) error
	ListActiveInstruments(context.Context) ([]Instrument, error)
	UpsertCandles(context.Context, []Candle) error
	ListLatestCandlesByInterval(context.Context, int64, string, int) ([]Candle, error)
	ListHourlyPrices(context.Context, []int64, time.Time, time.Time) ([]HourlyPrice, error)
	GetSyncState(context.Context, SyncProfile) (SyncState, error)
	SaveSyncState(context.Context, SyncState) error
}
