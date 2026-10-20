import { Select } from "@mantine/core";

import {
	type listStrategiesResponseSuccess,
	useListStrategies,
} from "@/api/generated/api";
import type { StrategyID } from "@/api/generated/models";
import { kindNouns } from "@/features/strategy-settings/constants";
import { strategyKind } from "@/features/strategy-settings/utils";
import { shortId } from "@/utils/short-id";

const selectStrategies = (response: listStrategiesResponseSuccess) =>
	response.data.items;

interface StrategySelectProps {
	disabled?: boolean;
	onChange(strategy: StrategyID): void;
	strategy: StrategyID | undefined;
}

// Offers every signal, then every strategy, disabled ones included; one whose
// expression no longer compiles cannot be backtested.
export function StrategySelect({
	disabled = false,
	onChange,
	strategy,
}: StrategySelectProps) {
	const strategies = useListStrategies({
		query: { retry: false, select: selectStrategies },
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
								? `#${shortId(id)} ${name} (invalid)`
								: enabled
									? `#${shortId(id)} ${name}`
									: `#${shortId(id)} ${name} (disabled)`,
							value: id,
						})),
				}))
				.filter(({ items }) => items.length > 0)}
			disabled={disabled || strategies.isPending}
			error={
				strategies.isError
					? "Signals and strategies could not be loaded."
					: undefined
			}
			nothingFoundMessage="No signals or strategies"
			onChange={(value) => {
				if (value !== null) onChange(value);
			}}
			label="Signal / strategy"
			placeholder="Choose a signal or strategy"
			searchable
			value={strategy ?? null}
		/>
	);
}
