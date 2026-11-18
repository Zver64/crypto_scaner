import { useLayoutEffect, useState } from "react";

import type { GridLimitsResponse } from "@/api/generated/models";
import { useAppForm } from "@/components/form";
import type { PriceCandle } from "@/features/instrument-analysis/candle-page";
import { latestAvailableCandle } from "@/features/instrument-analysis/grid-estimator/bounds";
import { GRID_MARKET_TABS } from "@/features/instrument-analysis/grid-estimator/config";
import { gridCalculatorValues } from "@/features/instrument-analysis/grid-estimator/defaults";
import { gridCountLimitErrors } from "@/features/instrument-analysis/grid-estimator/grid-count-limit";
import { investmentMinimumErrors } from "@/features/instrument-analysis/grid-estimator/investment-minimum";
import type {
	GridCalculatorBuild,
	GridMarket,
} from "@/features/instrument-analysis/grid-estimator/types";

export interface GridCalculatorFormOptions {
	baseAsset: string;
	candles: readonly (PriceCandle | null)[];
	dailyVolatilityPercent: number | undefined;
	gridLimits: GridLimitsResponse | null;
	hourlyVolatilityPercent: number | undefined;
	// Identifies the volatility ranges: the settings and where their result
	// comes from.
	rangesKey: string;
	// Whether the history and grid limits the calculator starts from are known.
	ready: boolean;
}

/**
 * Keeps the grid calculator of a coin page: one form with the values of every
 * market, rebuilt once its data is ready and whenever new volatility ranges
 * arrive.
 */
export function useGridCalculatorForm({
	baseAsset,
	candles,
	dailyVolatilityPercent,
	gridLimits,
	hourlyVolatilityPercent,
	rangesKey,
	ready,
}: GridCalculatorFormOptions) {
	const builtFor = `${ready}:${rangesKey}`;
	const close = latestAvailableCandle(candles)?.close ?? null;
	const [defaultValues] = useState(() =>
		gridCalculatorValues(
			{
				builtFor,
				close,
				dailyVolatilityPercent,
				gridLimits,
				hourlyVolatilityPercent,
			},
			null,
			new Set(),
		),
	);
	const form = useAppForm({
		defaultValues,
		validators: {
			onChange: ({ value }) => ({
				fields: {
					...investmentMinimumErrors(value, gridLimits, baseAsset).fields,
					...gridCountLimitErrors(value, gridLimits).fields,
				},
			}),
		},
	});
	// The one effect that synchronizes state (see frontend/AGENTS.md): new
	// ranges arrive without a user event, and edited values must be
	// recalculated for them, which neither keys nor TanStack's defaultValues
	// can do. A layout effect, so the old values are never painted. The
	// defaults stay the first ones, so TanStack does not reset the form to
	// them.
	useLayoutEffect(() => {
		const { fieldMeta, values } = form.state;
		if (values.builtFor !== builtFor) {
			const build: GridCalculatorBuild = {
				builtFor,
				close,
				dailyVolatilityPercent,
				gridLimits,
				hourlyVolatilityPercent,
			};
			form.reset(
				gridCalculatorValues(build, values, changedMarkets(fieldMeta)),
				{ keepDefaultValues: true },
			);
		}
		// Neither the first values nor a reset are validated, so the
		// investments and grid counts are checked against the Binance limits
		// here.
		void form.validate("change");
	}, [
		baseAsset,
		builtFor,
		close,
		dailyVolatilityPercent,
		form,
		gridLimits,
		hourlyVolatilityPercent,
	]);
	return form;
}

// The markets with a field changed since the form was last built.
function changedMarkets(
	fieldMeta: Readonly<Record<string, { isDirty: boolean } | undefined>>,
): Set<GridMarket> {
	const dirtyFields = Object.entries(fieldMeta)
		.filter(([, meta]) => meta?.isDirty)
		.map(([name]) => name);
	return new Set(
		GRID_MARKET_TABS.map(({ value }) => value).filter((market) =>
			dirtyFields.some((name) => name.startsWith(`markets.${market}.`)),
		),
	);
}
