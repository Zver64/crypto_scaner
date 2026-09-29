import { Checkbox, Group, Input, Stack } from "@mantine/core";
import type { CandleInterval } from "@/api/generated/models";
import { chartIntervalOptions } from "@/components/price-history-chart/config";
import type { PeriodChoices } from "@/features/scanner-settings/types";

interface ScannerIndicatorPeriodsFieldProps {
	// Whether the chosen indicator can be a table column.
	tableAllowed: boolean;
	onChange(periods: PeriodChoices): void;
	value: PeriodChoices;
}

// The periods an indicator is added on, each with its own table column choice.
export function ScannerIndicatorPeriodsField({
	tableAllowed,
	onChange,
	value,
}: ScannerIndicatorPeriodsFieldProps) {
	const toggle = (interval: CandleInterval, chosen: boolean) => {
		const { [interval]: _removed, ...rest } = value;
		onChange(chosen ? { ...rest, [interval]: { showInTable: false } } : rest);
	};
	return (
		<Input.Wrapper
			description={
				tableAllowed
					? undefined
					: "Only indicators with one output can be table columns."
			}
			label="Periods"
		>
			<Stack gap="xs" mt="xs">
				{chartIntervalOptions.map(({ label, value: interval }) => {
					const choice = value[interval];
					return (
						<Group key={interval} wrap="nowrap">
							<Checkbox
								checked={choice !== undefined}
								label={label}
								onChange={(event) =>
									toggle(interval, event.currentTarget.checked)
								}
								w="6rem"
							/>
							<Checkbox
								checked={tableAllowed && choice?.showInTable === true}
								aria-label={`Show ${label} in tables`}
								disabled={!tableAllowed || choice === undefined}
								label="Show in tables"
								onChange={(event) => {
									const { checked } = event.currentTarget;
									onChange({ ...value, [interval]: { showInTable: checked } });
								}}
								size="xs"
							/>
						</Group>
					);
				})}
			</Stack>
		</Input.Wrapper>
	);
}
