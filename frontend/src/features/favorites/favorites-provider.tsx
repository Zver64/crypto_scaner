import type { ReactNode } from "react";

import { UserFavoritesProvider } from "@/features/favorites/user-favorites-provider";
import { telegramUserScope } from "@/features/favorites/user-query-scope";

// Keyed by the Telegram user scope, so another user starts with fresh state.
export function FavoritesProvider({
	children,
	enabled,
}: {
	children: ReactNode;
	enabled: boolean;
}) {
	const scope = telegramUserScope();
	return (
		<UserFavoritesProvider enabled={enabled} key={scope} scope={scope}>
			{children}
		</UserFavoritesProvider>
	);
}
