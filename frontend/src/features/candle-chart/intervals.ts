import type { ManipulateType } from "dayjs";

import { CandleInterval } from "@/api/generated/models";

interface CandleIntervalPresentation {
	label: string;
	// Whether chart times show the hour.
	showTime: boolean;
	// The dayjs unit of one candle.
	unit: ManipulateType;
}

// Every candle interval, in the order charts, forms, and requests list them.
export const candleIntervals = {
	[CandleInterval["1h"]]: { label: "Hourly", showTime: true, unit: "hour" },
	[CandleInterval["1d"]]: { label: "Daily", showTime: false, unit: "day" },
	[CandleInterval["1w"]]: { label: "Weekly", showTime: false, unit: "week" },
	[CandleInterval["1M"]]: { label: "Monthly", showTime: false, unit: "month" },
} as const satisfies Record<CandleInterval, CandleIntervalPresentation>;

// The keys of candleIntervals in their order (none is an integer string).
export const chartIntervals = Object.keys(candleIntervals) as CandleInterval[];

export const chartIntervalOptions = chartIntervals.map((value) => ({
	label: candleIntervals[value].label,
	value,
}));

export const defaultChartInterval: CandleInterval = CandleInterval["1d"];
