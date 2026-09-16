// Criterion names identify implementations; keys identify configured instances.
export const criterionNames = {
	volatility: "volatility",
	marketCap: "market_cap",
} as const;

export const criterionKeys = {
	volatility: criterionNames.volatility,
	dailyVolatility: "daily_volatility",
	hourlyVolatility: "hourly_volatility",
	marketCap: criterionNames.marketCap,
} as const;

export const evaluationMetricKeys = {
	rangePercent: "range_percent",
	marketCapUsd: "market_cap_usd",
} as const;
