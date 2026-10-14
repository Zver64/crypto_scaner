import type { DeepPartial, Time, TimeChartOptions } from "lightweight-charts";

import {
	chartIntervalOptions,
	chartOptions,
} from "@/features/candle-chart/config";
import type { ChartInterval } from "@/features/candle-chart/types";
import { formatChartTime } from "@/features/candle-chart/utils";

type ColorScheme = keyof typeof chartOptions;

function intervalChartOptions(
	colorScheme: ColorScheme,
	interval: ChartInterval,
) {
	const base = chartOptions[colorScheme];
	const showTime =
		chartIntervalOptions.find(({ value }) => value === interval)?.showTime ??
		false;
	return {
		...base,
		localization: {
			timeFormatter: (time: Time) =>
				typeof time === "number"
					? formatChartTime(time, interval)
					: String(time),
		},
		timeScale: { ...base.timeScale, timeVisible: showTime },
	} satisfies DeepPartial<TimeChartOptions>;
}

// One entry per interval, so a missing interval fails typecheck.
function schemeChartOptions(colorScheme: ColorScheme) {
	return {
		"1h": intervalChartOptions(colorScheme, "1h"),
		"1d": intervalChartOptions(colorScheme, "1d"),
		"1w": intervalChartOptions(colorScheme, "1w"),
		"1M": intervalChartOptions(colorScheme, "1M"),
	} satisfies Record<ChartInterval, ReturnType<typeof intervalChartOptions>>;
}

// Chart options of every color scheme and candle interval, created once so
// their time formatters keep their identity and the options compare equal.
export const timeChartOptions = {
	dark: schemeChartOptions("dark"),
	light: schemeChartOptions("light"),
} satisfies Record<ColorScheme, Record<ChartInterval, unknown>>;
