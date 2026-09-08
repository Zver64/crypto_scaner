import { formatRangePercent } from "@/utils/range-percent";

interface PercentChangeProps {
	value: number | null;
}

export function PercentChange({ value }: PercentChangeProps) {
	const color =
		value === null || value === 0
			? undefined
			: value > 0
				? "var(--mantine-color-green-6)"
				: "var(--mantine-color-red-6)";

	return (
		<span style={{ color }}>
			{value === null ? "—" : formatRangePercent(value)}
		</span>
	);
}
