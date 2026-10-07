import { NumberInput } from "@mantine/core";
import { backtestHold } from "@/features/strategy-backtest/constants";
import { formatNumber } from "@/utils/number-format";

interface HoldInputProps {
	// The default the backend used for this strategy, once a run shows it.
	defaultHold: number | undefined;
	hold: number | undefined;
	onChange(hold: number | undefined): void;
}

// Candles of the strategy's interval each trade holds; empty uses the
// backend default of that interval.
export function HoldInput({ defaultHold, hold, onChange }: HoldInputProps) {
	return (
		<NumberInput
			allowDecimal={false}
			allowNegative={false}
			clampBehavior="strict"
			description={
				defaultHold === undefined
					? "Candles; empty uses the default"
					: `Candles; default ${formatNumber(defaultHold)}`
			}
			label="Hold"
			max={backtestHold.max}
			min={backtestHold.min}
			onChange={(value) =>
				onChange(typeof value === "number" ? value : undefined)
			}
			placeholder={
				defaultHold === undefined ? "Default" : formatNumber(defaultHold)
			}
			value={hold ?? ""}
		/>
	);
}
