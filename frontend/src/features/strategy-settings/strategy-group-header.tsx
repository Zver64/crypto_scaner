import {
	ActionIcon,
	Button,
	Group,
	SegmentedControl,
	Switch,
} from "@mantine/core";
import { IconTrash } from "@tabler/icons-react";
import type { RuleGroupProps, UseRuleGroup } from "react-querybuilder";

// The combinator, NOT toggle, and add/remove actions of one group.
export function StrategyGroupHeader({
	addGroup,
	addRule,
	disabled,
	onCombinatorChange,
	onNotToggleChange,
	path,
	removeGroup,
	ruleGroup,
	schema,
}: RuleGroupProps & UseRuleGroup) {
	return (
		<Group gap="xs" justify="space-between">
			<Group gap="xs">
				<SegmentedControl
					data={[
						{ label: "AND", value: "and" },
						{ label: "OR", value: "or" },
					]}
					disabled={disabled}
					onChange={onCombinatorChange}
					size="xs"
					value={"combinator" in ruleGroup ? ruleGroup.combinator : "and"}
				/>
				<Switch
					checked={Boolean(ruleGroup.not)}
					disabled={disabled}
					label="NOT"
					onChange={(event) => onNotToggleChange(event.currentTarget.checked)}
					size="xs"
				/>
			</Group>
			<Group gap={4}>
				<Button
					disabled={disabled}
					onClick={(event) => addRule(event)}
					size="compact-xs"
					variant="light"
				>
					+ Condition
				</Button>
				{path.length < schema.maxLevels ? (
					<Button
						disabled={disabled}
						onClick={(event) => addGroup(event)}
						size="compact-xs"
						variant="light"
					>
						+ Group
					</Button>
				) : null}
				{path.length > 0 ? (
					<ActionIcon
						aria-label="Remove group"
						color="red"
						disabled={disabled}
						onClick={(event) => removeGroup(event)}
						size="sm"
						variant="subtle"
					>
						<IconTrash size={14} />
					</ActionIcon>
				) : null}
			</Group>
		</Group>
	);
}
