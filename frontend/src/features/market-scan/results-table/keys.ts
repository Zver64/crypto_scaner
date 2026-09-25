// Data column keys are the corresponding fields of MarketScanRow.
export const marketScanColumnKeys = {
	symbol: "symbol",
	dailyRange: "dailyRangePercent",
	hourlyRange: "hourlyRangePercent",
	dailyRsi14: "dailyRsi14",
	marketCap: "marketCapUsd",
	priceHistory: "priceHistory",
	sevenDayChangePercent: "sevenDayChangePercent",
	binance: "binance",
	reason: "reason",
} as const;
