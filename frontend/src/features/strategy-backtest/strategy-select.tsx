import { Select } from "@mantine/core";
import { useListStrategies } from "@/api/generated/api";

interface StrategySelectProps {
	onChange(strategy: number): void;
	strategy: number | undefined;
}

// Offers every strategy, disabled ones included; a strategy whose expression no
// longer compiles cannot be backtested.
export function StrategySelect({ onChange, strategy }: StrategySelectProps) {
	const strategies = useListStrategies({
		query: { retry: false, select: (response) => response.data.items },
	});
	return (
		<Select
			allowDeselect={false}
			data={(strategies.data ?? []).map(({ enabled, id, name, valid }) => ({
				disabled: !valid,
				label: !valid
					? `#${id} ${name} (invalid)`
					: enabled
						? `#${id} ${name}`
						: `#${id} ${name} (disabled)`,
				value: String(id),
			}))}
			disabled={strategies.isPending}
			error={strategies.isError ? "Strategies could not be loaded." : undefined}
			nothingFoundMessage="No strategies"
			onChange={(value) => {
				if (value !== null) onChange(Number(value));
			}}
			label="Strategy"
			placeholder="Choose a strategy"
			searchable
			value={strategy === undefined ? null : String(strategy)}
		/>
	);
}
