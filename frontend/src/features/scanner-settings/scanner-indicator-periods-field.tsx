import { Checkbox, Group, Input, Stack } from "@mantine/core";
import type { CandleInterval } from "@/api/generated/models";
import { chartIntervalOptions } from "@/features/candle-chart/config";
import type {
	PeriodChoice,
	PeriodChoices,
} from "@/features/scanner-settings/types";

interface ScannerIndicatorPeriodsFieldProps {
	// Whether the chosen indicator can be a table column.
	tableAllowed: boolean;
	onChange(periods: PeriodChoices): void;
	value: PeriodChoices;
}

// The periods an indicator is added on, each with its own table column and
// chart choices.
export function ScannerIndicatorPeriodsField({
	tableAllowed,
	onChange,
	value,
}: ScannerIndicatorPeriodsFieldProps) {
	const toggle = (interval: CandleInterval, chosen: boolean) => {
		const { [interval]: _removed, ...rest } = value;
		onChange(
			chosen
				? { ...rest, [interval]: { showInTable: false, showInChart: true } }
				: rest,
		);
	};
	const change = (interval: CandleInterval, choice: PeriodChoice) =>
		onChange({ ...value, [interval]: choice });
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
								label="Table"
								onChange={(event) => {
									if (choice)
										change(interval, {
											...choice,
											showInTable: event.currentTarget.checked,
										});
								}}
								size="xs"
							/>
							<Checkbox
								checked={choice?.showInChart === true}
								aria-label={`Show ${label} in charts`}
								disabled={choice === undefined}
								label="Chart"
								onChange={(event) => {
									if (choice)
										change(interval, {
											...choice,
											showInChart: event.currentTarget.checked,
										});
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
