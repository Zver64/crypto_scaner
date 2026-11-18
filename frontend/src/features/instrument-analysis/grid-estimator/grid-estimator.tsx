import {
	Alert,
	SegmentedControl,
	Stack,
	Text,
	useMantineTheme,
} from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";

import { SegmentedValueGroup } from "@/components/segmented-value-group";
import { FUTURES_GRID_MAINTENANCE_MARGIN_RATE } from "@/features/instrument-analysis/grid-estimator/calculator/futures-grid";
import { GRID_TYPE_OPTIONS } from "@/features/instrument-analysis/grid-estimator/config";
import {
	marginAsset,
	profitSplitRows,
} from "@/features/instrument-analysis/grid-estimator/estimate";
import { marketField } from "@/features/instrument-analysis/grid-estimator/fields";
import { FuturesControls } from "@/features/instrument-analysis/grid-estimator/futures-controls";
import { GridPriceInputs } from "@/features/instrument-analysis/grid-estimator/grid-price-inputs";
import { InvestmentPresets } from "@/features/instrument-analysis/grid-estimator/investment-presets";
import { LiquidationRangeBar } from "@/features/instrument-analysis/grid-estimator/liquidation-range-bar";
import { MarkupSliders } from "@/features/instrument-analysis/grid-estimator/markup-sliders";
import {
	type GridMarketOptions,
	useGridMarket,
} from "@/features/instrument-analysis/grid-estimator/use-grid-market";
import { formatRangePercent } from "@/utils/range-percent";

export type GridEstimatorProps = GridMarketOptions & {
	disabled?: boolean;
};

export function GridEstimator({
	disabled = false,
	...marketOptions
}: GridEstimatorProps) {
	const { form, market } = marketOptions;
	const {
		alignLowerPrice,
		alignUpperPrice,
		applyInvestment,
		bounds,
		changeLowerMarkup,
		changeMarkup,
		changeRange,
		commitError,
		commitField,
		dailyRange,
		defaultRangePercent,
		estimate,
		hourlyRange,
		priceUnit,
		rangeError,
		values: formValues,
	} = useGridMarket(marketOptions);
	const { colors } = themeToVars(useMantineTheme());
	const { anchor } = bounds;
	const hasAnchor = anchor !== null;
	const isFutures = market !== "spot";
	const selectedRangePercent = formValues.rangePercent;
	const canSelectRange =
		hourlyRange !== null && dailyRange !== null && dailyRange > hourlyRange;
	const { futuresEstimate, values } = estimate;

	return (
		<Stack gap="md">
			<form.Field
				listeners={{ onChange: changeRange }}
				name={marketField(market, "gridType")}
			>
				{(field) => (
					<SegmentedControl
						aria-label="Grid type"
						data={GRID_TYPE_OPTIONS}
						disabled={disabled}
						fullWidth
						onChange={field.handleChange}
						value={field.state.value}
					/>
				)}
			</form.Field>
			{isFutures ? (
				<FuturesControls disabled={disabled} form={form} market={market} />
			) : null}
			<Stack gap="sm">
				<MarkupSliders
					bounds={bounds}
					disabled={disabled || !hasAnchor}
					form={form}
					market={market}
					onLowerAlign={alignLowerPrice}
					onLowerMarkupChange={changeLowerMarkup}
					onUpperAlign={alignUpperPrice}
					onUpperMarkupChange={changeMarkup}
				/>
				<form.AppField
					listeners={{ onChange: changeRange }}
					name={marketField(market, "rangePercent")}
				>
					{(field) => (
						<field.ScaleSliderField
							disabled={disabled || !canSelectRange}
							formatValue={formatRangePercent}
							label="Minimum grid step"
							marks={
								canSelectRange ? [{ value: defaultRangePercent }] : undefined
							}
							max={canSelectRange ? dailyRange : selectedRangePercent + 1}
							min={selectedRangePercent > 0 ? (hourlyRange ?? 0) : 0}
							scaleLabels={[
								{ label: "Hourly range", position: 0 },
								{ label: "Daily range", position: 100 },
							]}
							step={canSelectRange ? (dailyRange - hourlyRange) / 100 : 1}
						/>
					)}
				</form.AppField>
				{rangeError ? (
					<Text c="red" size="sm">
						{rangeError}
					</Text>
				) : null}
				{!hasAnchor ? (
					<Text c="dimmed" size="sm">
						Price markups need the current price.
					</Text>
				) : null}
				{market === "spot" && marketOptions.gridLimits === null ? (
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
				commitField={commitField}
				committed={formValues.committed}
				disabled={disabled}
				commitError={commitError}
				form={form}
				investmentAsset={marginAsset(market, marketOptions.baseAsset)}
				market={market}
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
