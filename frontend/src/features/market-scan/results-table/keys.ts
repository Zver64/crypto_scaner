// Data column keys are the corresponding fields of MarketScanRow.
export const marketScanColumnKeys = {
	symbol: "symbol",
	dailyRange: "dailyRangePercent",
	hourlyRange: "hourlyRangePercent",
	hourlyCandleCount: "hourlyCandleCount",
	dailyCandleCount: "dailyCandleCount",
	marketCap: "marketCapUsd",
	priceHistory: "priceHistory",
	binance: "binance",
	reason: "reason",
} as const;
