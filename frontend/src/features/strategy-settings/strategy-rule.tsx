import {
	ActionIcon,
	Group,
	NumberInput,
	Paper,
	SegmentedControl,
	Select,
	Stack,
} from "@mantine/core";
import { IconX } from "@tabler/icons-react";
import type { RuleProps } from "react-querybuilder";
import type { StrategyVariable } from "@/api/generated/models";
import type { StrategyOperator } from "@/features/strategy-settings/types";
import {
	isCrossOperator,
	isRangeOperator,
	valueForOperator,
	variableSelectData,
} from "@/features/strategy-settings/utils";

const operatorOptions: { label: string; value: StrategyOperator }[] = [
	{ label: "> above", value: ">" },
	{ label: "≥ at least", value: ">=" },
	{ label: "< below", value: "<" },
	{ label: "≤ at most", value: "<=" },
	{ label: "between", value: "between" },
	{ label: "not between", value: "notBetween" },
	{ label: "crosses above", value: "crossesAbove" },
	{ label: "crosses below", value: "crossesBelow" },
];

interface StrategyRuleProps extends RuleProps {
	variables: readonly StrategyVariable[];
}

// One comparison, stacked vertically for phones: indicator, operator, then a
// number, a range, or another indicator.
export function StrategyRule({
	actions,
	disabled,
	path,
	rule,
	variables,
}: StrategyRuleProps) {
	const change = (
		prop: "field" | "operator" | "value" | "valueSource",
		value: unknown,
	) => actions.onPropChange(prop, value, path);
	const variableOptions = variableSelectData(variables);
	const range = isRangeOperator(rule.operator);
	const byIndicator = rule.valueSource === "field";
	return (
		<Paper mt="xs" p="xs" radius="sm" withBorder>
			<Group align="flex-start" gap="xs" wrap="nowrap">
				<Stack flex={1} gap="xs" miw={0}>
					<Select
						aria-label="Indicator"
						data={variableOptions}
						disabled={disabled}
						onChange={(value) => {
							if (value) change("field", value);
						}}
						searchable
						size="sm"
						value={rule.field || null}
					/>
					<Select
						aria-label="Comparison"
						allowDeselect={false}
						data={operatorOptions}
						disabled={disabled}
						onChange={(value) => {
							if (!value) return;
							change("operator", value);
							const source = isRangeOperator(value)
								? "value"
								: rule.valueSource;
							if (source !== rule.valueSource) change("valueSource", source);
							change("value", valueForOperator(value, rule.value, source));
						}}
						size="sm"
						value={rule.operator}
					/>
					{range ? null : (
						<SegmentedControl
							data={[
								{ label: "Number", value: "value" },
								{ label: "Indicator", value: "field" },
							]}
							disabled={disabled}
							onChange={(source) => {
								change("valueSource", source);
								change(
									"value",
									source === "field"
										? (variables.find(({ name }) => name !== rule.field)
												?.name ?? "")
										: 0,
								);
							}}
							size="xs"
							value={byIndicator ? "field" : "value"}
						/>
					)}
					{range ? (
						<Group gap="xs" grow wrap="nowrap">
							{[0, 1].map((index) => (
								<NumberInput
									aria-label={index === 0 ? "From" : "To"}
									disabled={disabled}
									key={index}
									onChange={(value) => {
										const bounds: unknown[] = Array.isArray(rule.value)
											? [...rule.value]
											: [0, 0];
										bounds[index] = value;
										change("value", bounds);
									}}
									placeholder={index === 0 ? "From" : "To"}
									size="sm"
									value={Array.isArray(rule.value) ? rule.value[index] : ""}
								/>
							))}
						</Group>
					) : byIndicator ? (
						<Select
							aria-label={
								isCrossOperator(rule.operator)
									? "Crossed indicator"
									: "Compared indicator"
							}
							data={variableOptions}
							disabled={disabled}
							onChange={(value) => {
								if (value) change("value", value);
							}}
							searchable
							size="sm"
							value={
								typeof rule.value === "string" && rule.value !== ""
									? rule.value
									: null
							}
						/>
					) : (
						<NumberInput
							aria-label="Number"
							disabled={disabled}
							onChange={(value) => change("value", value)}
							size="sm"
							value={typeof rule.value === "number" ? rule.value : ""}
						/>
					)}
				</Stack>
				<ActionIcon
					aria-label="Remove condition"
					color="red"
					disabled={disabled}
					onClick={() => actions.onRuleRemove(path)}
					variant="subtle"
				>
					<IconX size={16} />
				</ActionIcon>
			</Group>
		</Paper>
	);
}
