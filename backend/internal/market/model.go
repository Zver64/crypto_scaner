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
}

// Candle is a closed market interval represented for analysis.
type Candle struct {
	InstrumentID     int64
	Interval         string
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
type CandleRequest struct {
	Symbol        string
	Interval      string
	Limit         int
	ClosedBefore  time.Time
	AfterOpenTime *time.Time
}

// SyncProfile identifies one independently synchronized market dataset.
type SyncProfile struct {
	Exchange   string
	Market     string
	QuoteAsset string
	Interval   string
	TimeZone   string
}

// Key returns the stable persistence identity for a synchronization profile.
func (profile SyncProfile) Key() string {
	return profile.Exchange + ":" + profile.Market + ":" + profile.QuoteAsset + ":" + profile.Interval + ":" + profile.TimeZone
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
