package market

import (
	"context"
	"errors"
	"time"
)

var ErrInstrumentNotFound = errors.New("instrument not found")

// MarketStore is the persistence seam used by synchronization and analysis.
type MarketStore interface {
	ApplyInstrumentSnapshot(context.Context, []Instrument) error
	ListActiveInstruments(context.Context) ([]Instrument, error)
	GetActiveInstrumentBySymbol(context.Context, string) (Instrument, error)
	UpsertCandles(context.Context, []Candle) error
	ListLatestCandlesByInterval(context.Context, int64, string, int) ([]Candle, error)
	ListCandlePage(context.Context, int64, CandleInterval, *time.Time, int) (CandlePage, error)
	ListHourlyPrices(context.Context, []int64, time.Time, time.Time) ([]HourlyPrice, error)
	GetSyncState(context.Context, SyncProfile) (SyncState, error)
	SaveSyncState(context.Context, SyncState) error
}
