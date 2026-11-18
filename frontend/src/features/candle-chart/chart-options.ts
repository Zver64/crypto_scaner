import type { DeepPartial, Time, TimeChartOptions } from "lightweight-charts";

import type { CandleInterval } from "@/api/generated/models";
import { chartOptions } from "@/features/candle-chart/config";
import { candleIntervals } from "@/features/candle-chart/intervals";
import { formatChartTime, intervalRecord } from "@/features/candle-chart/utils";

type ColorScheme = keyof typeof chartOptions;

function intervalChartOptions(
	colorScheme: ColorScheme,
	interval: CandleInterval,
) {
	const base = chartOptions[colorScheme];
	return {
		...base,
		localization: {
			timeFormatter: (time: Time) =>
				typeof time === "number"
					? formatChartTime(time, interval)
					: String(time),
		},
		timeScale: {
			...base.timeScale,
			timeVisible: candleIntervals[interval].showTime,
		},
	} satisfies DeepPartial<TimeChartOptions>;
}

function schemeChartOptions(colorScheme: ColorScheme) {
	return intervalRecord((interval) =>
		intervalChartOptions(colorScheme, interval),
	);
}

// Chart options of every color scheme and candle interval, created once so
// their time formatters keep their identity and the options compare equal.
export const timeChartOptions = {
	dark: schemeChartOptions("dark"),
	light: schemeChartOptions("light"),
} satisfies Record<ColorScheme, Record<CandleInterval, unknown>>;
