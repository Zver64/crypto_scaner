import { notifications } from "@mantine/notifications";
import {
	type Mutation,
	useMutationState,
	useQueryClient,
} from "@tanstack/react-query";
import { type ReactNode, useState } from "react";
import {
	type AddFavoriteMutationVariables,
	getAddFavoriteMutationKey,
	getListFavoritesQueryKey,
	getRemoveFavoriteMutationKey,
	type listFavorites,
	type RemoveFavoriteMutationVariables,
	useAddFavorite,
	useListFavorites,
	useRemoveFavorite,
} from "@/api/generated/api";
import type { Favorite } from "@/api/generated/models";
import { useUserAccessBlocked } from "@/features/favorites/access-block";
import { FavoriteRemovalConfirmation } from "@/features/favorites/favorite-removal-confirmation";
import { FavoritesContext } from "@/features/favorites/favorites-context";
import { invalidateFavoriteQueries } from "@/features/favorites/query-cache";
import { scopedUserQueryKey } from "@/features/favorites/user-query-scope";
import { instrumentUsesMessage } from "@/features/favorites/utils";

const noFavorites: ReadonlyMap<string, Favorite> = new Map();

// Module-level, so the query recomputes the map only when the data changes.
function favoritesBySymbol(
	response: Awaited<ReturnType<typeof listFavorites>>,
): ReadonlyMap<string, Favorite> {
	return new Map(response.data.items.map((item) => [item.symbol, item]));
}

const selectMutationSymbol = ({ state }: Mutation) =>
	(
		state.variables as
			| AddFavoriteMutationVariables
			| RemoveFavoriteMutationVariables
	).symbol;

function conflictAlertCount(error: unknown): number | undefined {
	const details = (
		error as { info?: { error?: { details?: Record<string, unknown> } } }
	).info?.error?.details;
	const count = details?.alert_count;
	return typeof count === "number" && Number.isInteger(count) && count > 0
		? count
		: undefined;
}

// The favorites of one Telegram user.
export function UserFavoritesProvider({
	children,
	enabled,
	scope,
}: {
	children: ReactNode;
	enabled: boolean;
	scope: string;
}) {
	const queryClient = useQueryClient();
	const pendingSymbols = new Set(
		useMutationState({
			filters: {
				status: "pending",
				predicate: ({ options }) =>
					options.mutationKey?.[0] === getAddFavoriteMutationKey()[0] ||
					options.mutationKey?.[0] === getRemoveFavoriteMutationKey()[0],
			},
			select: selectMutationSymbol,
		}),
	);
	const accessBlocked = useUserAccessBlocked(scope);
	const [confirmation, setConfirmation] = useState<{
		symbol: string;
		alertCount: number;
	}>();
	const query = useListFavorites({
		query: {
			enabled: enabled && !accessBlocked,
			meta: { userScoped: true },
			queryKey: scopedUserQueryKey(getListFavoritesQueryKey(), scope),
			refetchInterval: 15_000,
			refetchOnMount: "always",
			retry: false,
			select: favoritesBySymbol,
		},
	});
	const favorites = query.data ?? noFavorites;

	const refreshFavorites = () => invalidateFavoriteQueries(queryClient);
	const addMutation = useAddFavorite({
		mutation: {
			meta: { userScoped: true },
			onError: () => {
				notifications.show({
					color: "red",
					message: "The favorite could not be added.",
					title: "Favorite update failed",
				});
			},
			onSuccess: refreshFavorites,
		},
	});
	const removalFailed = (error: unknown) => {
		notifications.show({
			color: "red",
			message:
				instrumentUsesMessage(error) ?? "The favorite could not be removed.",
			title: "Favorite update failed",
		});
	};
	const closeConfirmation = (symbol: string) => {
		setConfirmation((current) =>
			current?.symbol === symbol ? undefined : current,
		);
	};
	const removeMutation = useRemoveFavorite({
		mutation: {
			meta: { userScoped: true },
			onError: (error, { symbol, params }) => {
				const alertCount = conflictAlertCount(error);
				if (!params?.confirm_alerts && error.status === 409 && alertCount) {
					setConfirmation({ symbol, alertCount });
					return;
				}
				// Confirming again cannot help while strategies read the coin.
				if (params?.confirm_alerts && instrumentUsesMessage(error)) {
					closeConfirmation(symbol);
				}
				removalFailed(error);
			},
			onSuccess: (_response, { symbol }) => {
				closeConfirmation(symbol);
				refreshFavorites();
			},
		},
	});
	const remove = (symbol: string, confirmAlerts: boolean) => {
		if (pendingSymbols.has(symbol)) return;
		removeMutation.mutate({
			symbol,
			params: { confirm_alerts: confirmAlerts },
		});
	};
	const toggle = (symbol: string) => {
		if (pendingSymbols.has(symbol)) return;
		const favorite = favorites.get(symbol);
		if (!favorite) {
			addMutation.mutate({ symbol });
			return;
		}
		if (favorite.alert_count > 0) {
			setConfirmation({ symbol, alertCount: favorite.alert_count });
			return;
		}
		remove(symbol, false);
	};
	const confirmRemoval = () => {
		if (!confirmation) return;
		remove(confirmation.symbol, true);
	};

	return (
		<FavoritesContext
			value={{
				favorites,
				isError: query.isError,
				isLoading: query.isPending,
				isMutating: (symbol) => pendingSymbols.has(symbol),
				toggle,
			}}
		>
			{children}
			<FavoriteRemovalConfirmation
				alertCount={confirmation?.alertCount ?? 0}
				isPending={
					confirmation !== undefined && pendingSymbols.has(confirmation.symbol)
				}
				onCancel={() => setConfirmation(undefined)}
				onConfirm={confirmRemoval}
				opened={confirmation !== undefined}
				symbol={confirmation?.symbol ?? ""}
			/>
		</FavoritesContext>
	);
}
