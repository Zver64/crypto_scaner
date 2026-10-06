package analysis

// SelectionFact is a closed set of persisted facts supported by candidate SQL.
type SelectionFact uint8

// MarketCapUSD is the market capitalization metric and the only sort field.
const MarketCapUSD = "market_cap_usd"

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
	Symbols       []string
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
