import { Checkbox, Group, Input, Stack } from "@mantine/core";
import type { UseFormReturnType } from "@mantine/form";

import type { CandleInterval } from "@/api/generated/models";
import { chartIntervalOptions } from "@/features/candle-chart/config";
import type { ScannerIndicatorDraft } from "@/features/scanner-settings/types";

interface ScannerIndicatorPeriodsFieldProps {
	form: UseFormReturnType<ScannerIndicatorDraft>;
	// Whether the chosen indicator can be a table column.
	tableAllowed: boolean;
}

// The periods an indicator is added on, each with its own table column and
// chart choices.
export function ScannerIndicatorPeriodsField({
	form,
	tableAllowed,
}: ScannerIndicatorPeriodsFieldProps) {
	const value = form.values.periods;
	// A period is chosen by adding its entry and cleared by removing it.
	const toggle = (interval: CandleInterval, chosen: boolean) =>
		form.setFieldValue("periods", ({ [interval]: _removed, ...rest }) =>
			chosen
				? { ...rest, [interval]: { showInTable: false, showInChart: true } }
				: rest,
		);
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
								{...form.getInputProps(`periods.${interval}.showInTable`, {
									type: "checkbox",
								})}
								// A period without an entry has no flags; Mantine's Checkbox
								// passes an undefined checked on to the native input, which
								// would keep showing its last state.
								checked={choice?.showInTable ?? false}
								aria-label={`Show ${label} in tables`}
								disabled={!tableAllowed || choice === undefined}
								label="Table"
								size="xs"
							/>
							<Checkbox
								{...form.getInputProps(`periods.${interval}.showInChart`, {
									type: "checkbox",
								})}
								checked={choice?.showInChart ?? false}
								aria-label={`Show ${label} in charts`}
								disabled={choice === undefined}
								label="Chart"
								size="xs"
							/>
						</Group>
					);
				})}
			</Stack>
		</Input.Wrapper>
	);
}
