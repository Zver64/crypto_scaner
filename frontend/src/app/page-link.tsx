import { Box, useMantineTheme } from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";
import { Link } from "@tanstack/react-router";

interface PageLinkProps {
	active: boolean;
	label: string;
	to: "/" | "/favorites" | "/top-coins";
	withDivider?: boolean;
}

export function PageLink({ active, label, to, withDivider }: PageLinkProps) {
	const { colors } = themeToVars(useMantineTheme());

	return (
		<Box
			aria-current={active ? "page" : undefined}
			bg={active ? colors.defaultHover : "transparent"}
			component={Link}
			c="inherit"
			fz="sm"
			fw={600}
			px="xs"
			style={{
				alignItems: "center",
				borderInlineStart: withDivider
					? `1px solid ${colors.defaultBorder}`
					: undefined,
				display: "flex",
				justifyContent: "center",
				lineHeight: 1.2,
				minHeight: 44,
				textAlign: "center",
				textDecoration: "none",
				whiteSpace: "nowrap",
			}}
			to={to}
		>
			{label}
		</Box>
	);
}
