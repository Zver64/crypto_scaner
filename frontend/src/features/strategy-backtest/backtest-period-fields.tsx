import { Group } from "@mantine/core";
import { DatePickerInput } from "@mantine/dates";
import { backtestDateFormat } from "@/features/strategy-backtest/constants";

interface BacktestPeriodFieldsProps {
	error: string | undefined;
	from: string | undefined;
	onChange(from: string | undefined, to: string | undefined): void;
	to: string | undefined;
}

// The first and last UTC days of the evaluated period; an empty day leaves
// that side of the stored history open.
export function BacktestPeriodFields({
	error,
	from,
	onChange,
	to,
}: BacktestPeriodFieldsProps) {
	return (
		<Group align="flex-start" grow wrap="nowrap">
			<DatePickerInput
				clearable
				error={error}
				label="From (UTC)"
				maxDate={to}
				onChange={(value) => onChange(value ?? undefined, to)}
				placeholder={backtestDateFormat}
				value={from ?? null}
				valueFormat={backtestDateFormat}
			/>
			<DatePickerInput
				clearable
				label="To (UTC)"
				minDate={from}
				onChange={(value) => onChange(from, value ?? undefined)}
				placeholder={backtestDateFormat}
				value={to ?? null}
				valueFormat={backtestDateFormat}
			/>
		</Group>
	);
}
