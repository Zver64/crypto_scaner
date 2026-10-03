package market

// PriceRange is the exchange's PRICE_RANGE execution rule of a symbol: buy
// orders may not go below ReferencePrice × BidLimitMultDown and sell orders
// may not go above ReferencePrice × AskLimitMultUp.
type PriceRange struct {
	BidLimitMultDown float64
	AskLimitMultUp   float64
}

// PriceLimits is the current reference price of a symbol and, when the
// exchange defines one, its price range rule.
type PriceLimits struct {
	ReferencePrice float64
	Range          *PriceRange
}
