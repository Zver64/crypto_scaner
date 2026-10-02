import { ActionIcon, Button, Select, Stack } from "@mantine/core";
import type { ExpressionNode } from "@react-querybuilder/expr";
import { IconPlus, IconX } from "@tabler/icons-react";
import {
	ExpressionEditor,
	type ExpressionEditorProps,
} from "@/features/strategy-settings/expression-editor";
import {
	maxVariadicArguments,
	replaceFunction,
	strategyFunctions,
} from "@/features/strategy-settings/expressions";

const functionOptions = Object.entries(strategyFunctions).map(
	([value, { label }]) => ({ label, value }),
);

export interface FunctionEditorProps
	extends Omit<ExpressionEditorProps, "label"> {
	node: Extract<ExpressionNode, { kind: "func" }>;
}

// A function and its arguments below it; the operand editor frames them as a
// whole. Changing the function keeps the arguments that still fit.
export function FunctionEditor({
	depth = 0,
	disabled,
	node,
	onChange,
	variables,
	withoutCoin,
}: FunctionEditorProps) {
	const definition = strategyFunctions[node.fn];
	const titles = definition?.args ?? [];
	const replaceArg = (index: number, arg: ExpressionNode) =>
		onChange({
			...node,
			args: node.args.map((old, i) => (i === index ? arg : old)),
		});
	return (
		<Stack gap="xs">
			<Select
				allowDeselect={false}
				aria-label="Function"
				data={functionOptions}
				disabled={disabled}
				onChange={(name) => {
					if (name && strategyFunctions[name]) {
						onChange(replaceFunction(node, name));
					}
				}}
				size="sm"
				value={node.fn}
			/>
			<Stack gap="xs">
				{node.args.map((arg, index) => (
					<ExpressionEditor
						action={
							definition?.variadic && node.args.length > 2 ? (
								<ActionIcon
									aria-label="Remove value"
									color="red"
									disabled={disabled}
									onClick={() =>
										onChange({
											...node,
											args: node.args.filter((_, i) => i !== index),
										})
									}
									variant="subtle"
								>
									<IconX size={16} />
								</ActionIcon>
							) : null
						}
						depth={depth + 1}
						disabled={disabled}
						// biome-ignore lint/suspicious/noArrayIndexKey: arguments are positional; an argument is its position.
						key={index}
						label={titles[Math.min(index, titles.length - 1)] ?? "Value"}
						node={arg}
						onChange={(next) => replaceArg(index, next)}
						variables={variables}
						withoutCoin={withoutCoin}
					/>
				))}
				{definition?.variadic && node.args.length < maxVariadicArguments ? (
					<Button
						disabled={disabled}
						leftSection={<IconPlus size={14} />}
						onClick={() =>
							onChange({
								...node,
								args: [...node.args, { kind: "value", value: 0 }],
							})
						}
						size="xs"
						variant="subtle"
					>
						Add value
					</Button>
				) : null}
			</Stack>
		</Stack>
	);
}
