import {
	PercentChange,
	type PercentChangeColors,
} from "@/components/percent-change";
import { fractionToPercent } from "@/features/strategy-backtest/utils";

interface BacktestReturnProps {
	colors?: PercentChangeColors;
	value: number | null;
}

// A backtest return fraction as a percentage colored by sign.
export function BacktestReturn({ colors, value }: BacktestReturnProps) {
	return (
		<PercentChange
			colors={colors}
			maximumFractionDigits={2}
			value={fractionToPercent(value)}
		/>
	);
}
