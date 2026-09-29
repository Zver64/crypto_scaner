import {
	Collapse,
	Fieldset,
	type FieldsetProps,
	Group,
	UnstyledButton,
} from "@mantine/core";
import { useId } from "@mantine/hooks";
import { IconChevronDown } from "@tabler/icons-react";

interface CollapsibleFieldsetProps extends FieldsetProps {
	collapsed?: boolean;
	onCollapsedChange?(collapsed: boolean): void;
}

// A Mantine Fieldset whose legend toggles the visibility of its content.
export function CollapsibleFieldset({
	children,
	collapsed = false,
	legend,
	onCollapsedChange,
	...fieldsetProps
}: CollapsibleFieldsetProps) {
	const contentId = useId();

	return (
		<Fieldset
			{...fieldsetProps}
			legend={
				<UnstyledButton
					aria-controls={contentId}
					aria-expanded={!collapsed}
					onClick={() => onCollapsedChange?.(!collapsed)}
				>
					<Group gap={4}>
						{legend}
						<IconChevronDown
							size={16}
							style={{
								transform: collapsed ? undefined : "rotate(180deg)",
								transition: "transform 200ms ease",
							}}
						/>
					</Group>
				</UnstyledButton>
			}
		>
			<Collapse expanded={!collapsed} id={contentId}>
				{children}
			</Collapse>
		</Fieldset>
	);
}
