// Data column keys are the corresponding fields of MarketScanRow.
export const marketScanColumnKeys = {
	symbol: "symbol",
	dailyRange: "dailyRangePercent",
	hourlyRange: "hourlyRangePercent",
	marketCap: "marketCapUsd",
	priceHistory: "priceHistory",
	sevenDayChangePercent: "sevenDayChangePercent",
	binance: "binance",
	reason: "reason",
} as const;
