import { Group } from "@mantine/core";
import type { UseRuleGroup } from "react-querybuilder";

// The header of one group: the combinator and NOT toggle on the left, the
// add and remove actions on the right.
export function StrategyGroupHeader(rg: UseRuleGroup) {
	const {
		addGroupAction: AddGroupAction,
		addRuleAction: AddRuleAction,
		combinatorSelector: CombinatorSelector,
		notToggle: NotToggle,
		removeGroupAction: RemoveGroupAction,
	} = rg.schema.controls;
	const common = {
		context: rg.context,
		disabled: rg.disabled,
		level: rg.path.length,
		path: rg.path,
		schema: rg.schema,
		validation: rg.validationResult,
	};
	const actionProps = {
		...common,
		ruleOrGroup: rg.ruleGroup,
		rules: rg.ruleGroup.rules,
	};
	return (
		<Group gap="xs" justify="space-between">
			<Group gap="xs">
				<CombinatorSelector
					{...common}
					handleOnChange={rg.onCombinatorChange}
					options={rg.schema.combinators}
					ruleGroup={rg.ruleGroup}
					rules={rg.ruleGroup.rules}
					value={rg.combinator}
				/>
				<NotToggle
					{...common}
					checked={rg.ruleGroup.not}
					handleOnChange={rg.onNotToggleChange}
					label={rg.translations.notToggle.label}
					ruleGroup={rg.ruleGroup}
				/>
			</Group>
			<Group gap={4}>
				<AddRuleAction
					{...actionProps}
					handleOnClick={rg.addRule}
					label={rg.translations.addRule.label}
				/>
				{rg.path.length < rg.schema.maxLevels ? (
					<AddGroupAction
						{...actionProps}
						handleOnClick={rg.addGroup}
						label={rg.translations.addGroup.label}
					/>
				) : null}
				{rg.path.length > 0 ? (
					<RemoveGroupAction
						{...actionProps}
						handleOnClick={rg.removeGroup}
						title={rg.translations.removeGroup.title}
					/>
				) : null}
			</Group>
		</Group>
	);
}
