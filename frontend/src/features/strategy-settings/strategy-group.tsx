import { RuleGroup, type RuleGroupProps } from "react-querybuilder";
import { NestingLine } from "@/features/strategy-settings/nesting-line";

// A group of conditions. Nested groups indent behind a line in the color of
// their depth instead of a frame, so conditions keep the screen width.
export function StrategyGroup(props: RuleGroupProps) {
	if (props.path.length === 0) return <RuleGroup {...props} />;
	return (
		<NestingLine depth={props.path.length} mt="md">
			<RuleGroup {...props} />
		</NestingLine>
	);
}
