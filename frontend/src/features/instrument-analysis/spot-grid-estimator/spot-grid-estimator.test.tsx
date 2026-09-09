import { MantineProvider } from "@mantine/core";
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { SpotGridEstimator } from "@/features/instrument-analysis/spot-grid-estimator/spot-grid-estimator";

it("shows combined zero-valued profit per step before input is complete", () => {
	const html = renderToStaticMarkup(
		<MantineProvider>
			<SpotGridEstimator paperPadding="md" />
		</MantineProvider>,
	);
	const text = html.replace(/<[^>]*>/g, "");

	expect(text).toContain("Profit per step0 USDT, 0%");
	expect(text).not.toContain("Profit per step (%)");
	expect(text).toContain("Average entry price0 USDT");
	expect(text).not.toContain("Allocation per buy");
	expect(text).not.toContain("Arithmetic quote step");
	expect(text).not.toContain("Estimated fee impact");
	expect(text).not.toContain("all buys fill before any sells");
});
