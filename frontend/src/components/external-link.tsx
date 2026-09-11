import { Anchor, useMatches } from "@mantine/core";
import { openTelegramExternalLink } from "@/app/telegram";

interface ExternalLinkProps {
	ariaLabel: string;
	href: string;
}

export function ExternalLink({ ariaLabel, href }: ExternalLinkProps) {
	const iconSize = useMatches({ base: 16, sm: 18 });

	return (
		<Anchor
			aria-label={ariaLabel}
			href={href}
			onClick={(event) => {
				event.stopPropagation();
				if (openTelegramExternalLink(href)) {
					event.preventDefault();
				}
			}}
			onKeyDown={(event) => event.stopPropagation()}
			rel="noopener noreferrer"
			style={{
				display: "inline-flex",
				lineHeight: 0,
				verticalAlign: "middle",
			}}
			target="_blank"
		>
			<svg
				aria-hidden="true"
				data-icon="external-link"
				fill="none"
				height={iconSize}
				stroke="currentColor"
				strokeLinecap="round"
				strokeLinejoin="round"
				strokeWidth="2"
				viewBox="0 0 24 24"
				width={iconSize}
			>
				<path d="m6 18 12-12" />
				<path d="M9 6h9v9" />
			</svg>
		</Anchor>
	);
}
