import { Divider, Stack } from "@mantine/core";
import { type RuleProps, useRule } from "react-querybuilder";

// One comparison, stacked vertically for phones: the compared operand, the
// operator, then a range or the operand it is compared with. Conditions of a
// group are separated by a line rather than framed, so operands keep the
// screen width.
export function StrategyRule(props: RuleProps) {
	const r = useRule(props);
	const {
		fieldSelector: FieldSelector,
		operatorSelector: OperatorSelector,
		valueEditor: ValueEditor,
	} = r.schema.controls;
	const common = {
		context: r.context,
		disabled: r.disabled,
		level: r.path.length,
		path: r.path,
		rule: r.rule,
		schema: r.schema,
		validation: r.validationResult,
	};
	return (
		<Stack gap="xs" mt="sm">
			{(r.path.at(-1) ?? 0) > 0 ? <Divider mb="xs" /> : null}
			<FieldSelector
				{...common}
				handleOnChange={r.onChangeField}
				operator={r.rule.operator}
				options={r.schema.fields}
				value={r.rule.field}
			/>
			<OperatorSelector
				{...common}
				field={r.rule.field}
				fieldData={r.fieldData}
				handleOnChange={r.onChangeOperator}
				options={r.operators}
				value={r.rule.operator}
			/>
			<ValueEditor
				{...common}
				field={r.rule.field}
				fieldData={r.fieldData}
				handleOnChange={r.onChangeValue}
				operator={r.rule.operator}
				value={r.rule.value}
				valueSource={r.rule.valueSource ?? "value"}
			/>
		</Stack>
	);
}
