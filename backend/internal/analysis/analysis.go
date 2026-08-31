package analysis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"crypto-scanner/internal/market"
)

var ErrInvalidCandleData = errors.New("invalid candle data")
var ErrInvalidArgument = errors.New("invalid analysis argument")

// InsufficientHistoryError identifies the criterion whose required history is unavailable.
type InsufficientHistoryError struct {
	Criterion string
	Required  int
	Available int
}

func (err *InsufficientHistoryError) Error() string {
	return fmt.Sprintf("insufficient candle history for %s: required %d, available %d", err.Criterion, err.Required, err.Available)
}

// Unit identifies a supported candle granularity.
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

// CandleRequirement declares candle data a criterion needs.
type CandleRequirement struct {
	Unit  Unit
	Count int
}

// CriterionConfig selects and configures a criterion for the application service.
type CriterionConfig struct {
	Key        string
	Name       string
	Label      string
	Parameters map[string]any
}

// Factory builds a request-scoped, typed criterion.
type Factory interface {
	Name() string
	Build(map[string]any) (Criterion, error)
}

// Criterion evaluates one typed rule against its required candle data.
type Criterion interface {
	Name() string
	Requirements() []CandleRequirement
	Prepare(context.Context, []market.Instrument) ([]Warning, error)
	Evaluate(context.Context, Input) (Evaluation, error)
}
type Input struct {
	Instrument market.Instrument
	Candles    map[Unit][]market.Candle
}
type Warning struct{ Code, Message string }

// Evaluation is an unrounded criterion result.
type Evaluation struct {
	Key           string
	Name          string
	Label         string
	Matched       bool
	Metrics       map[string]float64
	OrderingScore *float64
	CandleCount   int
	From          time.Time
	To            time.Time
}
