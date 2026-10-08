import { Select } from "@mantine/core";
import { useListStrategies } from "@/api/generated/api";
import { kindNouns } from "@/features/strategy-settings/constants";
import { strategyKind } from "@/features/strategy-settings/utils";

interface StrategySelectProps {
	onChange(strategy: number): void;
	strategy: number | undefined;
}

// Offers every signal, then every strategy, disabled ones included; one whose
// expression no longer compiles cannot be backtested.
export function StrategySelect({ onChange, strategy }: StrategySelectProps) {
	const strategies = useListStrategies({
		query: { retry: false, select: (response) => response.data.items },
	});
	return (
		<Select
			allowDeselect={false}
			data={(["signal", "strategy"] as const)
				.map((kind) => ({
					group: kindNouns[kind].title,
					items: (strategies.data ?? [])
						.filter((item) => strategyKind(item) === kind)
						.map(({ enabled, id, name, valid }) => ({
							disabled: !valid,
							label: !valid
								? `#${id} ${name} (invalid)`
								: enabled
									? `#${id} ${name}`
									: `#${id} ${name} (disabled)`,
							value: String(id),
						})),
				}))
				.filter(({ items }) => items.length > 0)}
			disabled={strategies.isPending}
			error={
				strategies.isError
					? "Signals and strategies could not be loaded."
					: undefined
			}
			nothingFoundMessage="No signals or strategies"
			onChange={(value) => {
				if (value !== null) onChange(Number(value));
			}}
			label="Signal / strategy"
			placeholder="Choose a signal or strategy"
			searchable
			value={strategy === undefined ? null : String(strategy)}
		/>
	);
}
