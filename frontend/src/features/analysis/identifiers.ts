// Criterion names identify implementations; keys identify configured instances.
export const criterionNames = {
	volatility: "volatility",
	marketCap: "market_cap",
	rsi: "rsi",
} as const;

export const criterionKeys = {
	volatility: criterionNames.volatility,
	dailyVolatility: "daily_volatility",
	hourlyVolatility: "hourly_volatility",
	marketCap: criterionNames.marketCap,
	dailyRsi: "daily_rsi",
	weeklyRsi: "weekly_rsi",
} as const;

export const evaluationMetricKeys = {
	rangePercent: "range_percent",
	marketCapUsd: "market_cap_usd",
	rsi: "rsi",
} as const;
