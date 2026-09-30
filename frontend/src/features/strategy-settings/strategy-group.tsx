import { Paper } from "@mantine/core";
import { RuleGroup, type RuleGroupProps } from "react-querybuilder";

// A bordered group; nested groups indent inside their parent.
export function StrategyGroup(props: RuleGroupProps) {
	return (
		<Paper mt={props.path.length > 0 ? "xs" : 0} p="xs" radius="sm" withBorder>
			<RuleGroup {...props} />
		</Paper>
	);
}
