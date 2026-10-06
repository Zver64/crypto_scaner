import { Button, useMantineTheme } from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";
import { Link } from "@tanstack/react-router";

interface PageLinkProps {
	active: boolean;
	label: string;
	to: "/" | "/favorites" | "/top-coins";
	withDivider?: boolean;
}

// A page tab sized like the small inputs through Mantine's button sizes.
export function PageLink({ active, label, to, withDivider }: PageLinkProps) {
	const { colors } = themeToVars(useMantineTheme());

	return (
		<Button
			aria-current={active ? "page" : undefined}
			bg={active ? colors.defaultHover : undefined}
			c="inherit"
			color="gray"
			component={Link}
			// Like the preset buttons: a tap leaves no focus ring behind.
			onClick={(event) => event.currentTarget.blur()}
			px="xs"
			radius={0}
			size="sm"
			style={{
				borderInlineStart: withDivider
					? `1px solid ${colors.defaultBorder}`
					: undefined,
			}}
			to={to}
			variant="subtle"
		>
			{label}
		</Button>
	);
}
