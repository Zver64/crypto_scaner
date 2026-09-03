import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { PriceHistoryChart } from "@/features/market-scan/price-history-chart";

const window = { from: "2026-08-26T23:00:00Z", to: "2026-09-02T23:00:00Z" };
function render(prices: (number | null)[]) {
	return renderToStaticMarkup(
		<PriceHistoryChart prices={prices} symbol="BTCUSDT" window={window} />,
	);
}

it("renders a static rising line with an accessible description", () => {
	const html = render(Array.from({ length: 169 }, (_, i) => i + 1));
	expect(html).toContain('viewBox="0 0 140 40"');
	expect(html).toContain("stroke:var(--mantine-color-green-6)");
	expect(html).toContain('role="img"');
	expect(html).toContain("BTCUSDT: 7-day hourly closing prices");
	expect(html).not.toMatch(/<circle|<text|tabindex|data-mc-ink="fill"/);
});

function series(entries: [number, number][]) {
	const values: (number | null)[] = Array.from({ length: 169 }, () => null);
	for (const [slot, value] of entries) values[slot] = value;
	return values;
}

it.each([
	[1.00000001, 0.99999999, "red"],
	[0.99999999, 1.00000001, "green"],
	[1, 1, "gray"],
] as const)("compares unrounded endpoints %s → %s despite intermediate peaks", (first, last, color) => {
	const html = render(
		series([
			[0, first],
			[1, 1000],
			[2, last],
		]),
	);
	expect(html).toContain(`stroke:var(--mantine-color-${color}-6)`);
});

it("preserves leading, internal and trailing gaps on the seven-day scale", () => {
	const html = render(
		series([
			[42, 1],
			[84, 2],
			[85, 3],
			[126, 4],
		]),
	);
	const path = html.match(/<path d="([^"]+)"/)?.[1];
	expect(path).toBe("M36 38 M70 26 L70.81 14 M104 2");
	// Isolated observations on either side of the segment remain visible.
	expect(html).toContain('cx="36" cy="38"');
	expect(html).toContain('cx="104" cy="2"');
});

it("leaves the first four days blank for a three-day history", () => {
	const values = Array.from({ length: 169 }, (_, i) => (i < 96 ? null : i));
	const html = render(values);
	expect(Number(html.match(/d="M([\d.]+)/)?.[1])).toBeCloseTo(79.71, 2);
	expect(html).toContain('L138 2"');
});

it.each([
	0, 42, 168,
])("renders one neutral point at slot %s without recentering", (slot) => {
	const html = render(series([[slot, 5]]));
	const x = slot === 0 ? 2 : slot === 42 ? 36 : 138;
	expect(html).toContain(`cx="${x}" cy="20"`);
	expect(html).toContain('fill="var(--mantine-color-gray-6)"');
	expect(html).toContain("1 of 169 observations");
});

it("shows a dash when history is unavailable", () => {
	const html = render(series([]));
	expect(html).toContain("—");
	expect(html).toContain("No hourly price history");
	expect(html).not.toContain("<svg");
});
