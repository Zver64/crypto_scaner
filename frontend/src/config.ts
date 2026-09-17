export const applicationConfig = {
	marketCap: {
		presets: [100, 500, 1000, 5000, 10000],
	},
	topMarketCap: {
		defaultSettings: {
			hourlyPercentile: 80,
			hourlyPeriod: 60,
			percentile: 80,
			period: 30,
		},
	},
	volatility: {
		days: {
			candleRangePresets: [3, 5, 7, 10],
			periodPresets: [15, 30, 60],
		},
		hours: {
			candleRangePresets: [1, 1.5, 2, 3],
			periodPresets: [24, 60, 100],
		},
		percentilePresets: [75, 80, 90, 95],
	},
} as const;
