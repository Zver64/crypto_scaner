import type { PriceCandle } from "@/api/candle-history";

const hourMilliseconds = 60 * 60 * 1_000;
const sevenDaysInHours = 7 * 24;

export function currentSevenDayHourlyCloses(
	candles: readonly PriceCandle[],
	now: number = Date.now(),
): number[] {
	const currentHourOpen = Math.floor(now / hourMilliseconds) * hourMilliseconds;
	const lastClosedOpen = currentHourOpen - hourMilliseconds;
	const firstOpen = lastClosedOpen - sevenDaysInHours * hourMilliseconds;
	return candles
		.filter((candle) => {
			const open = Date.parse(candle.open_time);
			return open >= firstOpen && open <= lastClosedOpen;
		})
		.map((candle) => candle.close);
}
