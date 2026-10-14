import { Button, Group, Stack, Text } from "@mantine/core";
import { useSelector } from "@tanstack/react-form";

import { StrategyConditions } from "@/features/strategy-settings/strategy-conditions";
import type {
	StrategyFormApi,
	StrategyRuleVariables,
} from "@/features/strategy-settings/types";
import { emptyStrategyQuery } from "@/features/strategy-settings/utils";

interface StrategyExitConditionsProps {
	disabled: boolean;
	form: StrategyFormApi;
	variables: StrategyRuleVariables;
}

// The optional exit rule, added and removed as a whole. It follows only
// whether the rule exists, so its own edits stay in its builder.
export function StrategyExitConditions({
	disabled,
	form,
	variables,
}: StrategyExitConditionsProps) {
	const exists = useSelector(
		form.store,
		({ values }) => values.exitQuery !== undefined,
	);
	if (exists) {
		return (
			<StrategyConditions
				actions={
					<Button
						color="red"
						disabled={disabled}
						onClick={() => form.setFieldValue("exitQuery", undefined)}
						size="compact-sm"
						variant="subtle"
					>
						Remove
					</Button>
				}
				disabled={disabled}
				exit
				field="exitQuery"
				form={form}
				title="Exit"
				variables={variables}
			/>
		);
	}
	return (
		<Stack gap={4}>
			<Group justify="space-between">
				<Text fw={500} size="sm">
					Exit
				</Text>
				<Button
					disabled={disabled}
					onClick={() => form.setFieldValue("exitQuery", emptyStrategyQuery())}
					size="compact-sm"
					variant="subtle"
				>
					Add exit rule
				</Button>
			</Group>
		</Stack>
	);
}
