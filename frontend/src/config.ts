export const applicationConfig = {
	marketCap: {
		defaultMinimumMillions: 500,
		presets: [100, 500, 1000],
	},
	topMarketCap: {
		resultLimit: 50,
	},
	volatility: {
		days: {
			defaultCandleRange: 2,
			defaultPercentile: 90,
			defaultPeriod: 30,
			candleRangePresets: [2, 3, 5, 7],
			periodPresets: [15, 30, 60],
		},
		hours: {
			defaultCandleRange: 1,
			defaultPercentile: 80,
			defaultPeriod: 60,
			candleRangePresets: [0.7, 1, 1.5],
			periodPresets: [24, 60, 100],
		},
		percentilePresets: [75, 80, 90, 95],
	},
} as const;
