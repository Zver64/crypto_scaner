import { useMantineTheme } from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";
import {
	displayedPercentSign,
	formatRangePercent,
} from "@/utils/range-percent";

interface PercentChangeProps {
	maximumFractionDigits?: number;
	value: number | null;
}

export function PercentChange({
	maximumFractionDigits,
	value,
}: PercentChangeProps) {
	const { colors } = themeToVars(useMantineTheme());
	const sign =
		value === null ? 0 : displayedPercentSign(value, maximumFractionDigits);
	const color =
		sign === 0 ? undefined : sign > 0 ? colors.green[6] : colors.red[6];

	return (
		<span style={{ color }}>
			{value === null ? "—" : formatRangePercent(value, maximumFractionDigits)}
		</span>
	);
}
