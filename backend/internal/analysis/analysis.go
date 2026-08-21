package analysis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"crypto-scanner/internal/market"
)

// ErrInvalidCandleData reports persisted candle data that cannot be analyzed.
var ErrInvalidCandleData = errors.New("invalid candle data")

// ErrInvalidArgument reports analyzer input outside the supported bounds.
var ErrInvalidArgument = errors.New("invalid analysis argument")

// InsufficientHistoryError reports that the requested period is unavailable.
type InsufficientHistoryError struct {
	Required  int
	Available int
}

// Error implements error.
func (err *InsufficientHistoryError) Error() string {
	return fmt.Sprintf("insufficient candle history: required %d, available %d", err.Required, err.Available)
}

// AnalysisInput contains exchange-independent candle data for one analysis.
type AnalysisInput struct {
	Candles    []market.Candle
	Unit       Unit
	Period     int
	Percentile float64
}

// Unit identifies the candle granularity used by an analysis.
type Unit string

const (
	UnitDays  Unit = "days"
	UnitHours Unit = "hours"
)

func (unit Unit) Interval() string {
	if unit == UnitDays {
		return "1d"
	}
	return "1h"
}

// AnalysisResult is the unrounded outcome of an analysis.
type AnalysisResult struct {
	RangePercent float64
	CandleCount  int
	From         time.Time
	To           time.Time
}

// Analyzer calculates one kind of market analysis.
type Analyzer interface {
	Name() string
	Analyze(context.Context, AnalysisInput) (AnalysisResult, error)
}
