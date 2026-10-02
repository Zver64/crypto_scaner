import { Paper, useMantineTheme } from "@mantine/core";
import { RuleGroup, type RuleGroupProps } from "react-querybuilder";
import { nestingBorder } from "@/features/strategy-settings/nesting-colors";

// A group bordered in the color of its depth; nested groups indent inside
// their parent.
export function StrategyGroup(props: RuleGroupProps) {
	const border = nestingBorder(useMantineTheme(), props.path.length);
	return (
		<Paper
			mt={props.path.length > 0 ? "xs" : 0}
			p="xs"
			radius="sm"
			style={{ borderColor: border }}
			withBorder
		>
			<RuleGroup {...props} />
		</Paper>
	);
}
