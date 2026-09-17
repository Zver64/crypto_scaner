package analysis

import (
	"context"
	"fmt"

	"crypto-scanner/internal/market"
)

// SelectionFact is a closed set of persisted facts supported by candidate SQL.
type SelectionFact uint8

const (
	SelectionFactStablecoin SelectionFact = iota + 1
	SelectionFactMarketCapUSD
)

// SelectionOperator is a closed set of comparisons supported by candidate SQL.
type SelectionOperator uint8

const (
	SelectionEqual SelectionOperator = iota + 1
	SelectionAtLeast
)

// SelectionConstraint is a typed predicate. It cannot carry SQL fragments or
// identifiers; the PostgreSQL adapter maps this bounded vocabulary to values.
type SelectionConstraint struct {
	Fact     SelectionFact
	Operator SelectionOperator
	Boolean  bool
	Number   float64
}

// Selection is the persisted-fact portion of a market search. Candle criteria
// remain separate and run after this database selection.
type Selection struct {
	Constraints   []SelectionConstraint
	Limit         int
	SortFact      SelectionFact
	SortDirection string
	Symbol        string
}

// ConstrainBoolean composes an equality predicate over a supported boolean fact.
func (selection *Selection) ConstrainBoolean(fact SelectionFact, value bool) {
	for _, constraint := range selection.Constraints {
		if constraint.Fact == fact && constraint.Operator == SelectionEqual && constraint.Boolean == value {
			return
		}
	}
	selection.Constraints = append(selection.Constraints, SelectionConstraint{Fact: fact, Operator: SelectionEqual, Boolean: value})
}

// ConstrainAtLeast combines repeated lower bounds using max, so every request
// predicate is enforced in SQL before ordering and limiting.
func (selection *Selection) ConstrainAtLeast(fact SelectionFact, value float64) {
	for index := range selection.Constraints {
		constraint := &selection.Constraints[index]
		if constraint.Fact == fact && constraint.Operator == SelectionAtLeast {
			if value > constraint.Number {
				constraint.Number = value
			}
			return
		}
	}
	selection.Constraints = append(selection.Constraints, SelectionConstraint{Fact: fact, Operator: SelectionAtLeast, Number: value})
}

func (selection Selection) HasConstraint(fact SelectionFact) bool {
	for _, constraint := range selection.Constraints {
		if constraint.Fact == fact {
			return true
		}
	}
	return false
}

// SelectionStore applies all constraints before ordering and limiting.
type SelectionStore interface {
	SelectActiveInstruments(context.Context, Selection) ([]market.Instrument, error)
}

// SelectionFilter is shared by backend defaults and request-activated filters.
// BackendDefault controls activation only; all modules contribute constraints
// through the same Apply method and the same registry.
type SelectionFilter interface {
	Name() string
	BackendDefault() bool
	SortField() string
	Apply(Criterion, *Selection) error
	ApplySort(string, *Selection) error
}

type stablecoinSelectionFilter struct{}

func (stablecoinSelectionFilter) Name() string         { return "exclude_stablecoins" }
func (stablecoinSelectionFilter) BackendDefault() bool { return true }
func (stablecoinSelectionFilter) SortField() string    { return "" }
func (stablecoinSelectionFilter) Apply(_ Criterion, selection *Selection) error {
	selection.ConstrainBoolean(SelectionFactStablecoin, false)
	return nil
}
func (stablecoinSelectionFilter) ApplySort(string, *Selection) error {
	return ErrInvalidArgument
}

type marketCapSelectionFilter struct{}

func (marketCapSelectionFilter) Name() string         { return "market_cap" }
func (marketCapSelectionFilter) BackendDefault() bool { return false }
func (marketCapSelectionFilter) SortField() string    { return "market_cap_usd" }
func (marketCapSelectionFilter) Apply(criterion Criterion, selection *Selection) error {
	provider, ok := criterion.(interface{ MinimumMarketCapUSD() float64 })
	if !ok {
		return fmt.Errorf("market cap selection criterion: %w", ErrInvalidArgument)
	}
	selection.ConstrainAtLeast(SelectionFactMarketCapUSD, provider.MinimumMarketCapUSD())
	return nil
}
func (marketCapSelectionFilter) ApplySort(direction string, selection *Selection) error {
	if !selection.HasConstraint(SelectionFactMarketCapUSD) {
		return ErrInvalidArgument
	}
	selection.SortFact = SelectionFactMarketCapUSD
	selection.SortDirection = direction
	return nil
}

// selectionFilterModules is the single composition point. A filter over an
// already supported fact needs its module plus one registry entry here.
func selectionFilterModules() []SelectionFilter {
	return []SelectionFilter{stablecoinSelectionFilter{}, marketCapSelectionFilter{}}
}
