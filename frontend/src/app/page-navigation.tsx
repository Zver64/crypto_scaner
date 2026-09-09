import { Box, Group, Paper, Title, VisuallyHidden } from "@mantine/core";
import { Link } from "@tanstack/react-router";

type Page = "market-scan" | "top-coins";

interface PageNavigationProps {
	current: Page;
	title: string;
}

export function PageNavigation({ current, title }: PageNavigationProps) {
	return (
		<>
			<VisuallyHidden>
				<Title order={1}>{title}</Title>
			</VisuallyHidden>
			<Paper radius="sm" w="100%" withBorder>
				<Group aria-label="Pages" component="nav" gap={0} grow wrap="nowrap">
					<PageLink
						active={current === "market-scan"}
						label="Market Scan"
						to="/"
					/>
					<PageLink
						active={current === "top-coins"}
						label="Top Market Cap"
						to="/top-coins"
						withDivider
					/>
				</Group>
			</Paper>
		</>
	);
}

interface PageLinkProps {
	active: boolean;
	label: string;
	to: "/" | "/top-coins";
	withDivider?: boolean;
}

function PageLink({ active, label, to, withDivider }: PageLinkProps) {
	return (
		<Box
			aria-current={active ? "page" : undefined}
			bg={active ? "var(--mantine-color-default-hover)" : "transparent"}
			component={Link}
			c="inherit"
			fz="sm"
			fw={600}
			px="xs"
			style={{
				alignItems: "center",
				borderInlineStart: withDivider
					? "1px solid var(--mantine-color-default-border)"
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
