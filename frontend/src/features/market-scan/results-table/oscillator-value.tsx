import { useMantineTheme } from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";
import { oscillatorColor } from "@/features/market-scan/results-table/utils";
import { formatNumber } from "@/utils/number-format";

export function OscillatorValue({ value }: { value: number | null }) {
	const { colors } = themeToVars(useMantineTheme());
	if (value === null) {
		return "—";
	}
	return (
		<span style={{ color: oscillatorColor(value, colors) }}>
			{formatNumber(value, 1)}
		</span>
	);
}
