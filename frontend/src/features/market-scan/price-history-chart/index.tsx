import { Sparkline } from "@microcharts/react/sparkline";
import type { PriceHistoryWindow } from "@/api/client";

interface PriceHistoryChartProps {
	prices: readonly (number | null)[];
	symbol: string;
	window: PriceHistoryWindow;
}

export function PriceHistoryChart({
	prices,
	symbol,
	window,
}: PriceHistoryChartProps) {
	const available = prices.filter((price) => price !== null);
	const first = available[0];
	const last = available.at(-1);
	if (first === undefined || last === undefined) {
		return (
			<span
				role="img"
				aria-label={`${symbol}: No hourly price history in the seven-day window`}
			>
				—
			</span>
		);
	}
	const direction = last > first ? "green" : last < first ? "red" : "gray";
	const color = `var(--mantine-color-${direction}-6)`;
	const low = Math.min(...available);
	const high = Math.max(...available);
	return (
		<Sparkline
			data={prices}
			domain={[low, high]}
			width={140}
			height={40}
			dots="none"
			fill={false}
			label="none"
			curve="linear"
			color={color}
			summary={`${symbol}: 7-day hourly closing prices. ${available.length} of 169 observations. ${last > first ? "Rising" : last < first ? "Falling" : "Unchanged"}. Candle hours ${window.from} to ${window.to}.`}
			style={{ width: 140, height: 40, minWidth: 140, display: "block" }}
		>
			{/* Sparkline's two-unit inset and fixed y-domain also position isolated
		    observations. A move-only line segment would otherwise be invisible. */}
			{prices.map((price, index) =>
				price !== null &&
				prices[index - 1] == null &&
				prices[index + 1] == null ? (
					<circle
						// biome-ignore lint/suspicious/noArrayIndexKey: fixed hourly slots never reorder; the key is the observation's UTC time.
						key={Date.parse(window.from) + index * 3_600_000}
						cx={2 + (index * 136) / 168}
						cy={high === low ? 20 : 38 - ((price - low) / (high - low)) * 36}
						r={2}
						fill={color}
					/>
				) : null,
			)}
		</Sparkline>
	);
}
