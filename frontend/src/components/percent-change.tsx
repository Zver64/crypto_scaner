import { useMantineTheme } from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";
import {
	displayedPercentSign,
	formatRangePercent,
} from "@/utils/range-percent";

// How a percentage is colored: green when positive and red when negative,
// the reverse, or not at all.
export type PercentChangeColors = "sign" | "inverted" | "none";

interface PercentChangeProps {
	colors?: PercentChangeColors;
	maximumFractionDigits?: number;
	value: number | null;
}

export function PercentChange({
	colors: coloring = "sign",
	maximumFractionDigits,
	value,
}: PercentChangeProps) {
	const { colors } = themeToVars(useMantineTheme());
	const sign =
		value === null || coloring === "none"
			? 0
			: displayedPercentSign(value, maximumFractionDigits) *
				(coloring === "inverted" ? -1 : 1);
	const color =
		sign === 0 ? undefined : sign > 0 ? colors.green[6] : colors.red[6];

	return (
		<span style={{ color }}>
			{value === null ? "—" : formatRangePercent(value, maximumFractionDigits)}
		</span>
	);
}
