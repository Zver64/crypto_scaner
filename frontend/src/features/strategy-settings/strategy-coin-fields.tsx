import { SegmentedControl, Stack } from "@mantine/core";
import { useSelector } from "@tanstack/react-form";

import { CoinSelectionButtons } from "@/app/coin-selection-buttons";
import { useInstrumentSymbols } from "@/app/use-instrument-symbols";
import type { StrategyFormApi } from "@/features/strategy-settings/types";

interface StrategyCoinFieldsProps {
	disabled: boolean;
	form: StrategyFormApi;
	// Whether the strategy only signals; only a signal may evaluate all coins.
	signal: boolean;
}

// The coins the strategy or signal evaluates, chosen among all active coins.
export function StrategyCoinFields({
	disabled,
	form,
	signal,
}: StrategyCoinFieldsProps) {
	const symbols = useInstrumentSymbols();
	const allCoins = useSelector(
		form.store,
		(state) => signal && state.values.allCoins,
	);
	return (
		<Stack gap="xs">
			{signal ? (
				<form.Field name="allCoins">
					{(field) => (
						<SegmentedControl
							data={[
								{ label: "Selected coins", value: "selected" },
								{ label: "All coins", value: "all" },
							]}
							disabled={disabled}
							onChange={(value) => field.handleChange(value === "all")}
							value={field.state.value ? "all" : "selected"}
						/>
					)}
				</form.Field>
			) : null}
			{allCoins ? null : (
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
			)}
		</Stack>
	);
}
