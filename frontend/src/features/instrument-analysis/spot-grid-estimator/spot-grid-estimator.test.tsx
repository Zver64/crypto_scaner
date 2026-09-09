import { MantineProvider } from "@mantine/core";
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { SpotGridEstimator } from "@/features/instrument-analysis/spot-grid-estimator/spot-grid-estimator";

it("shows profit and grid info with their subgroups", () => {
	const html = renderToStaticMarkup(
		<MantineProvider>
			<SpotGridEstimator
				candles={[
					{
						close: 99,
						high: 100,
						low: 98,
						open: 99,
						open_time: "2026-09-01T00:00:00Z",
					},
				]}
				hourlyStepPercent={1}
				paperPadding="md"
			/>
		</MantineProvider>,
	);
	const text = html.replace(/<[^>]*>/g, "");

	expect(text).toContain("Arithmetic");
	expect(text).toContain("Geometric");
	expect(html).toContain('aria-label="Grid type"');
	const slider = html.match(/<[^>]*role="slider"[^>]*>/)?.[0] ?? "";
	expect(slider).toContain('aria-label="Upper price markup"');
	expect(slider).toContain('aria-valuetext="5%"');
	expect(html).toContain('value="70.5"');
	expect(html).toContain('value="105"');
	expect(html).toContain('value="40"');
	expect(html).toContain('value="1000"');
	expect(html).toMatch(/checked="" value="geometric"/);
	expect(html).toContain('role="group" aria-label="Profit"');
	expect(html).toContain('role="group" aria-label="Grid info"');
	expect(html.match(/<h3\b/g)).toHaveLength(2);
	expect(text).toContain("Profit");
	expect(text).toContain("USDT (nominal)0.2 USDTPercent0.799%");
	expect(text).not.toContain("Profit per step");
	expect(text).toContain("Grid info");
	expect(text).toContain("Average price85.1 USDTGrid step1%");
	expect(text).not.toContain("Average entry price");
	expect(text).not.toContain("Allocation per buy");
	expect(text).not.toContain("Arithmetic quote step");
	expect(text).not.toContain("Estimated fee impact");
	expect(text).not.toContain("all buys fill before any sells");
});

it("disables calculator controls while market data refreshes", () => {
	const html = renderToStaticMarkup(
		<MantineProvider>
			<SpotGridEstimator disabled paperPadding="md" />
		</MantineProvider>,
	);

	expect(html).toContain('aria-busy="true"');
	expect(html.match(/disabled=""/g)?.length).toBeGreaterThanOrEqual(6);
});
