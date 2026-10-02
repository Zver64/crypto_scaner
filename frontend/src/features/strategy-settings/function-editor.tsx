import { ActionIcon, Button, Group, Paper, Select, Stack } from "@mantine/core";
import type { ExpressionNode } from "@react-querybuilder/expr";
import { IconPlus, IconX } from "@tabler/icons-react";
import {
	ExpressionEditor,
	type ExpressionEditorProps,
} from "@/features/strategy-settings/expression-editor";
import {
	functionCall,
	maxVariadicArguments,
	strategyFunctions,
} from "@/features/strategy-settings/expressions";

const functionOptions = Object.entries(strategyFunctions).map(
	([value, { label }]) => ({ label, value }),
);

export interface FunctionEditorProps
	extends Omit<ExpressionEditorProps, "label"> {
	node: Extract<ExpressionNode, { kind: "func" }>;
}

// A function and its arguments, indented below it. Changing the function
// keeps the arguments that still fit.
export function FunctionEditor({
	disabled,
	node,
	onChange,
	variables,
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
					const next = name ? strategyFunctions[name] : undefined;
					if (!name || !next) return;
					const call = functionCall(
						name,
						node.args[0] ?? { kind: "value", value: 0 },
					);
					const kept = call.kind === "func" ? call.args : [];
					onChange({
						kind: "func",
						fn: name,
						args: kept.map((arg, index) => node.args[index] ?? arg),
					});
				}}
				size="sm"
				value={node.fn}
			/>
			<Paper p="xs" radius="sm" withBorder>
				<Stack gap="xs">
					{node.args.map((arg, index) => (
						<Group
							align="flex-start"
							gap="xs"
							// biome-ignore lint/suspicious/noArrayIndexKey: arguments are positional; an argument is its position.
							key={index}
							wrap="nowrap"
						>
							<Stack flex={1} miw={0}>
								<ExpressionEditor
									disabled={disabled}
									label={titles[Math.min(index, titles.length - 1)] ?? "Value"}
									node={arg}
									onChange={(next) => replaceArg(index, next)}
									variables={variables}
								/>
							</Stack>
							{definition?.variadic && node.args.length > 2 ? (
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
							) : null}
						</Group>
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
			</Paper>
		</Stack>
	);
}
