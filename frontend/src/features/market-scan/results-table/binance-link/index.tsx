import { Anchor, Text, useMatches } from "@mantine/core";
import { openTelegramExternalLink } from "@/app/telegram";
import { binanceSpotUrl } from "@/features/market-scan/results-table/binance-link/utils";

export function BinanceLink({ symbol }: { symbol: string }) {
	const iconSize = useMatches({ base: 16, sm: 18 });
	const url = binanceSpotUrl(symbol);
	return url ? (
		<Anchor
			aria-label={`Open ${symbol} on Binance Spot`}
			href={url}
			onClick={(event) => {
				event.stopPropagation();
				if (openTelegramExternalLink(url)) {
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
	) : (
		<Text c="dimmed">—</Text>
	);
}
