package market

// PriceFilters are the order price filters of a symbol. A down multiplier is
// the lowest share of the average price a buy order may have, an up
// multiplier the highest share a sell order may have; each is zero when the
// symbol has no such filter.
type PriceFilters struct {
	MinPrice          float64 // PRICE_FILTER
	MaxPrice          float64 // PRICE_FILTER
	TickSize          float64 // PRICE_FILTER
	BidMultiplierDown float64 // PERCENT_PRICE_BY_SIDE
	AskMultiplierUp   float64 // PERCENT_PRICE_BY_SIDE
	MultiplierDown    float64 // PERCENT_PRICE
	MultiplierUp      float64 // PERCENT_PRICE
}

// GridLimits are the current average price of a symbol and the order price
// filters a spot grid bot derives its price range limits from. The
// multipliers are the stricter of the two percent price filters, zero when
// the symbol has neither.
type GridLimits struct {
	AveragePrice      float64
	BidMultiplierDown float64
	AskMultiplierUp   float64
	MinPrice          float64
	MaxPrice          float64
	TickSize          float64
}
