import { useMantineTheme } from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";

interface BacktestSignalSuccessProps {
	success: boolean | null;
}

// Whether a signal succeeded, green Yes or red No; empty without an
// evaluation.
export function BacktestSignalSuccess({ success }: BacktestSignalSuccessProps) {
	const { colors } = themeToVars(useMantineTheme());
	if (success === null) {
		return null;
	}
	return (
		<span style={{ color: success ? colors.green[6] : colors.red[6] }}>
			{success ? "Yes" : "No"}
		</span>
	);
}
