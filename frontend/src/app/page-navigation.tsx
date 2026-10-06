import { Group, Paper, Title, VisuallyHidden } from "@mantine/core";
import { PageLink } from "@/app/page-link";

type Page = "favorites" | "market-scan" | "top-coins";

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
					<PageLink active={current === "market-scan"} label="Scan" to="/" />
					<PageLink
						active={current === "top-coins"}
						label="Top"
						to="/top-coins"
						withDivider
					/>
					<PageLink
						active={current === "favorites"}
						label="Favorites"
						to="/favorites"
						withDivider
					/>
				</Group>
			</Paper>
		</>
	);
}
