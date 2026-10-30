export const applicationConfig = {
	marketCap: {
		presets: [100, 500, 1000],
	},
	topMarketCap: {
		resultLimit: 50,
		defaultSettings: {
			hourlyPercentile: 80,
			hourlyPeriod: 60,
			percentile: 80,
			period: 30,
		},
	},
	volatility: {
		days: {
			defaultCandleRange: 2,
			candleRangePresets: [2, 3, 5, 7],
			periodPresets: [15, 30, 60],
		},
		hours: {
			defaultCandleRange: 1,
			candleRangePresets: [0.7, 1, 1.5],
			periodPresets: [24, 60, 100],
		},
		percentilePresets: [75, 80, 90, 95],
	},
} as const;
