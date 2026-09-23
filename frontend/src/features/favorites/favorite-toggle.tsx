import { ActionIcon, Tooltip } from "@mantine/core";
import { IconStar, IconStarFilled } from "@tabler/icons-react";
import { useFavorites } from "@/features/favorites/favorites-provider";

export function FavoriteToggle({ symbol }: { symbol: string }) {
	const { favorites, isMutating, toggle } = useFavorites();
	const selected = favorites.has(symbol);
	const label = selected
		? `Remove ${symbol} from favorites`
		: `Add ${symbol} to favorites`;
	return (
		<Tooltip label={label}>
			<ActionIcon
				aria-label={label}
				color="yellow"
				loading={isMutating(symbol)}
				onClick={(event) => {
					event.preventDefault();
					event.stopPropagation();
					toggle(symbol);
				}}
				onKeyDown={(event) => event.stopPropagation()}
				variant="subtle"
			>
				{selected ? <IconStarFilled size={18} /> : <IconStar size={18} />}
			</ActionIcon>
		</Tooltip>
	);
}
