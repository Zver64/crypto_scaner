import { ActionIcon, Button, Stack, Text } from "@mantine/core";
import type { ExpressionNode } from "@react-querybuilder/expr";
import { IconPlus, IconX } from "@tabler/icons-react";
import {
	ExpressionEditor,
	type ExpressionEditorProps,
} from "@/features/strategy-settings/expression-editor";
import {
	maxVariadicArguments,
	strategyFunctions,
} from "@/features/strategy-settings/expressions";
import { withoutPositionVariables } from "@/features/strategy-settings/utils";

export interface FunctionEditorProps
	extends Omit<ExpressionEditorProps, "label"> {
	node: Extract<ExpressionNode, { kind: "func" }>;
}

// The arguments of a function, each under its title; the operand editor
// chooses the function itself in its row.
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
	const argVariables = definition?.earlier
		? withoutPositionVariables(variables)
		: variables;
	const replaceArg = (index: number, arg: ExpressionNode) =>
		onChange({
			...node,
			args: node.args.map((old, i) => (i === index ? arg : old)),
		});
	return (
		<Stack gap="xs">
			{node.args.map((arg, index) => {
				const title = titles[Math.min(index, titles.length - 1)] ?? "Value";
				return (
					// biome-ignore lint/suspicious/noArrayIndexKey: arguments are positional; an argument is its position.
					<Stack gap={2} key={index}>
						<Text c="dimmed" size="xs">
							{title}
						</Text>
						<ExpressionEditor
							action={
								definition?.variadic && node.args.length > 2 ? (
									<ActionIcon
										aria-label="Remove value"
										size="input-sm"
										disabled={disabled}
										onClick={() =>
											onChange({
												...node,
												args: node.args.filter((_, i) => i !== index),
											})
										}
										variant="default"
									>
										<IconX size={18} />
									</ActionIcon>
								) : null
							}
							depth={depth + 1}
							disabled={disabled}
							label={title}
							node={arg}
							onChange={(next) => replaceArg(index, next)}
							variables={argVariables}
							withoutCoin={withoutCoin}
						/>
					</Stack>
				);
			})}
			{definition?.variadic && node.args.length < maxVariadicArguments ? (
				<Button
					disabled={disabled}
					justify="flex-start"
					leftSection={<IconPlus size={14} />}
					onClick={() =>
						onChange({
							...node,
							args: [...node.args, { kind: "value", value: 0 }],
						})
					}
					size="compact-xs"
					variant="subtle"
				>
					Add value
				</Button>
			) : null}
		</Stack>
	);
}
