package market

import "time"

// Instrument is an exchange-independent tradable instrument.
type Instrument struct {
	ID         int64
	Symbol     string
	BaseAsset  string
	QuoteAsset string
	Status     string
	Active     bool
	// MarketCapUSD is populated by persisted market selection queries.
	MarketCapUSD *float64
}

// Candle is a closed market interval represented for analysis.
type Candle struct {
	InstrumentID     int64
	Interval         CandleInterval
	OpenTime         time.Time
	CloseTime        time.Time
	Open             float64
	High             float64
	Low              float64
	Close            float64
	Volume           float64
	QuoteAssetVolume float64
	TradeCount       int64
}

// CandleRequest describes one bounded closed-candle query at the exchange
// boundary. ClosedBefore is the synchronization start and is exclusive.
// CandlePage is a chronological page of closed candles. HasMore reports that
// an older page exists before Candles[0].OpenTime.
type CandlePage struct {
	Candles []Candle
	HasMore bool
}

type CandleRequest struct {
	Symbol        string
	Interval      CandleInterval
	Limit         int
	ClosedBefore  time.Time
	AfterOpenTime *time.Time
}

// SyncProfile identifies one independently synchronized market dataset.
type SyncProfile struct {
	Exchange   string
	Market     string
	QuoteAsset string
	Interval   CandleInterval
	TimeZone   string
}

// Key returns the stable persistence identity for a synchronization profile.
func (profile SyncProfile) Key() string {
	return profile.Exchange + ":" + profile.Market + ":" + profile.QuoteAsset + ":" + string(profile.Interval) + ":" + profile.TimeZone
}

// BinanceSpotSyncProfile returns the code-owned synchronization profile for an interval.
func BinanceSpotSyncProfile(interval CandleInterval) SyncProfile {
	return SyncProfile{Exchange: "binance", Market: "spot", QuoteAsset: "USDT", Interval: interval, TimeZone: "UTC"}
}

func DailySyncProfile() SyncProfile   { return BinanceSpotSyncProfile(IntervalDay) }
func HourlySyncProfile() SyncProfile  { return BinanceSpotSyncProfile(IntervalHour) }
func WeeklySyncProfile() SyncProfile  { return BinanceSpotSyncProfile(IntervalWeek) }
func MonthlySyncProfile() SyncProfile { return BinanceSpotSyncProfile(IntervalMonth) }

// CoinMetadataSyncProfile identifies persisted CoinGecko metadata refresh state.
func CoinMetadataSyncProfile() SyncProfile {
	return SyncProfile{Exchange: "coingecko", Market: "coin_metadata", QuoteAsset: "USD", Interval: IntervalHour, TimeZone: "UTC"}
}

// SyncStatus is the durable outcome of market synchronization.
type SyncStatus string

const (
	SyncStatusNeverRun  SyncStatus = "never_run"
	SyncStatusRunning   SyncStatus = "running"
	SyncStatusSucceeded SyncStatus = "succeeded"
	SyncStatusFailed    SyncStatus = "failed"
)

// SyncState is restart and observability metadata for one profile.
type SyncState struct {
	Profile            SyncProfile
	LastStartedAt      *time.Time
	LastSucceededAt    *time.Time
	LastClosedOpenTime *time.Time
	Status             SyncStatus
	ErrorMessage       string
}
