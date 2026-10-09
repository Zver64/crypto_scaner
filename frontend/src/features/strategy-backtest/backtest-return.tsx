import { PercentChange } from "@/components/percent-change";
import { fractionToPercent } from "@/features/strategy-backtest/utils";

interface BacktestReturnProps {
	value: number | null;
}

// A backtest return fraction as a percentage colored by sign.
export function BacktestReturn({ value }: BacktestReturnProps) {
	return (
		<PercentChange maximumFractionDigits={2} value={fractionToPercent(value)} />
	);
}
