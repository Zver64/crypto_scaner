import { useMantineTheme } from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";
import { formatRangePercent } from "@/utils/range-percent";

interface PercentChangeProps {
	value: number | null;
}

export function PercentChange({ value }: PercentChangeProps) {
	const { colors } = themeToVars(useMantineTheme());
	const color =
		value === null || value === 0
			? undefined
			: value > 0
				? colors.green[6]
				: colors.red[6];

	return (
		<span style={{ color }}>
			{value === null ? "—" : formatRangePercent(value)}
		</span>
	);
}
