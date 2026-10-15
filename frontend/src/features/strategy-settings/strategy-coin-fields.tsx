import { SegmentedControl, Stack } from "@mantine/core";
import { useSelector } from "@tanstack/react-form";

import { StrategyCoinSource } from "@/api/generated/models";
import { CoinSelectionButtons } from "@/app/coin-selection-buttons";
import { useInstrumentSymbols } from "@/app/use-instrument-symbols";
import type { StrategyFormApi } from "@/features/strategy-settings/types";

interface StrategyCoinFieldsProps {
	disabled: boolean;
	form: StrategyFormApi;
	// Whether the strategy only signals; only a signal may evaluate the
	// favorites or all coins.
	signal: boolean;
}

// The coins the strategy or signal evaluates: chosen among all active coins,
// or a signal's favorites or all coins.
export function StrategyCoinFields({
	disabled,
	form,
	signal,
}: StrategyCoinFieldsProps) {
	const symbols = useInstrumentSymbols();
	const selected = useSelector(
		form.store,
		(state) =>
			!signal ||
			state.values.coinSource === StrategyCoinSource.CoinSourceSelected,
	);
	return (
		<Stack gap="xs">
			{signal ? (
				<form.Field name="coinSource">
					{(field) => (
						<SegmentedControl
							data={[
								{
									label: "Selected coins",
									value: StrategyCoinSource.CoinSourceSelected,
								},
								{
									label: "Favorites",
									value: StrategyCoinSource.CoinSourceFavorites,
								},
								{ label: "All coins", value: StrategyCoinSource.CoinSourceAll },
							]}
							disabled={disabled}
							onChange={field.handleChange}
							value={field.state.value}
						/>
					)}
				</form.Field>
			) : null}
			{selected ? (
				<form.AppField name="coins">
					{(field) => (
						<>
							<field.MultiSelectField
								clearable
								data={symbols.data ?? []}
								disabled={disabled || symbols.isPending}
								hidePickedOptions
								label="Coins"
								nothingFoundMessage="No coins found"
								placeholder="Choose coins"
								searchable
								withAsterisk
							/>
							<CoinSelectionButtons
								coins={field.state.value}
								disabled={disabled}
								onChange={field.handleChange}
							/>
						</>
					)}
				</form.AppField>
			) : null}
		</Stack>
	);
}
