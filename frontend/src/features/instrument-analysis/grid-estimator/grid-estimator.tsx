import {
	Alert,
	SegmentedControl,
	Stack,
	Text,
	useMantineTheme,
} from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";
import { SegmentedValueGroup } from "@/components/segmented-value-group";
import { SliderField } from "@/components/slider-field";
import {
	DEFAULT_MARKUPS,
	GRID_TYPE_OPTIONS,
} from "@/features/instrument-analysis/grid-estimator/config";
import { FuturesControls } from "@/features/instrument-analysis/grid-estimator/futures-controls";
import { GridPriceInputs } from "@/features/instrument-analysis/grid-estimator/grid-price-inputs";
import { InvestmentPresets } from "@/features/instrument-analysis/grid-estimator/investment-presets";
import { LiquidationRangeBar } from "@/features/instrument-analysis/grid-estimator/liquidation-range-bar";
import { MarkupSliders } from "@/features/instrument-analysis/grid-estimator/markup-sliders";
import type { GridMarket } from "@/features/instrument-analysis/grid-estimator/types";
import {
	type GridEstimatorFormOptions,
	useGridEstimatorForm,
} from "@/features/instrument-analysis/grid-estimator/use-grid-estimator-form";
import {
	marginAsset,
	profitSplitRows,
} from "@/features/instrument-analysis/grid-estimator/utils";
import { FUTURES_GRID_MAINTENANCE_MARGIN_RATE } from "@/utils/calculator/futures-grid";
import { formatRangePercent } from "@/utils/range-percent";

export type GridEstimatorProps = Omit<GridEstimatorFormOptions, "market"> & {
	disabled?: boolean;
};

export function GridEstimator({
	disabled = false,
	market,
	...formOptions
}: GridEstimatorProps & { market: GridMarket }) {
	const {
		applyInvestment,
		bounds,
		changeGridType,
		changeLowerMarkup,
		changeMarkup,
		changeRange,
		dailyRange,
		estimate,
		form,
		hourlyRange,
		inputProps,
		priceUnit,
	} = useGridEstimatorForm({ ...formOptions, market });
	const { colors } = themeToVars(useMantineTheme());
	const { anchor } = bounds;
	const hasAnchor = anchor !== null;
	const isFutures = market !== "spot";
	const selectedRangePercent = form.values.rangePercent;
	const canSelectRange =
		hourlyRange !== null && dailyRange !== null && dailyRange > hourlyRange;
	const { futuresEstimate, values } = estimate;

	return (
		<Stack gap="md">
			{isFutures ? (
				<FuturesControls
					direction={form.values.direction}
					disabled={disabled}
					leverage={form.values.leverage}
					onDirectionChange={(direction) =>
						form.setFieldValue("direction", direction)
					}
					onLeverageChange={(leverage) =>
						form.setFieldValue("leverage", leverage)
					}
				/>
			) : null}
			<SegmentedControl
				aria-label="Grid type"
				data={GRID_TYPE_OPTIONS}
				disabled={disabled}
				fullWidth
				onChange={changeGridType}
				value={form.values.gridType}
			/>
			<Stack gap="sm">
				<MarkupSliders
					bounds={bounds}
					defaultMarkups={DEFAULT_MARKUPS[market]}
					disabled={disabled || !hasAnchor}
					lowerMarkup={form.values.lowerMarkup}
					onLowerMarkupChange={changeLowerMarkup}
					onUpperMarkupChange={changeMarkup}
					upperMarkup={form.values.markup}
				/>
				<SliderField
					disabled={disabled || !canSelectRange}
					formatValue={formatRangePercent}
					label="Minimum grid step"
					max={canSelectRange ? dailyRange : selectedRangePercent + 1}
					min={selectedRangePercent > 0 ? (hourlyRange ?? 0) : 0}
					onChange={changeRange}
					scaleLabels={[
						{ label: "Hourly range", position: 0 },
						{ label: "Daily range", position: 100 },
					]}
					step={canSelectRange ? (dailyRange - hourlyRange) / 100 : 1}
					value={selectedRangePercent}
				/>
				{!hasAnchor ? (
					<Text c="dimmed" size="sm">
						Price markups need the current price.
					</Text>
				) : null}
				{market === "spot" && formOptions.gridLimits === null ? (
					<Text c="dimmed" size="sm">
						Binance grid limits are unavailable, so markups start from the
						latest hourly close and the prices are not checked.
					</Text>
				) : null}
				{hasAnchor && hourlyRange === null ? (
					<Text c="dimmed" size="sm">
						Hourly range is unavailable, so the grid count is not derived from
						it.
					</Text>
				) : null}
				{hourlyRange !== null && dailyRange === null ? (
					<Text c="dimmed" size="sm">
						Daily range is unavailable, so the grid range stays hourly.
					</Text>
				) : null}
			</Stack>
			<InvestmentPresets
				anchor={anchor}
				disabled={disabled}
				market={market}
				onSelect={applyInvestment}
			/>
			<GridPriceInputs
				disabled={disabled}
				inputProps={inputProps}
				investmentAsset={marginAsset(market, formOptions.baseAsset)}
				priceUnit={priceUnit}
			/>
			{estimate.error ? (
				<Alert color="red" title="Check calculator inputs">
					{estimate.error}
				</Alert>
			) : null}
			<Stack aria-live="polite" gap="md">
				{futuresEstimate ? (
					<LiquidationRangeBar
						currentPrice={anchor}
						estimate={futuresEstimate}
						priceUnit={priceUnit}
					/>
				) : null}
				<SegmentedValueGroup
					title="Profit per trade"
					rows={profitSplitRows(values.profitSplits, {
						fee: colors.orange[6],
						profit: colors.green[6],
					})}
				/>
			</Stack>
			{isFutures ? (
				<Text c="dimmed" size="sm">
					Liquidation assumes the price moves straight to it without a completed
					trade, with a{" "}
					{formatRangePercent(
						FUTURES_GRID_MAINTENANCE_MARGIN_RATE.times(100).toNumber(),
					)}{" "}
					maintenance margin and no fees.
				</Text>
			) : null}
		</Stack>
	);
}
