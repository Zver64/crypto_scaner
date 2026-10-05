import { notifications } from "@mantine/notifications";
import { useMutationState, useQueryClient } from "@tanstack/react-query";
import {
	createContext,
	type ReactNode,
	useCallback,
	useContext,
	useEffect,
	useMemo,
	useRef,
	useState,
} from "react";
import {
	type AddFavoriteMutationVariables,
	getAddFavoriteMutationKey,
	getListFavoritesQueryKey,
	getRemoveFavoriteMutationKey,
	type RemoveFavoriteMutationVariables,
	useAddFavorite,
	useListFavorites,
	useRemoveFavorite,
} from "@/api/generated/api";
import type { Favorite } from "@/api/generated/models";
import { FavoriteRemovalConfirmation } from "@/features/favorites/favorite-removal-confirmation";
import { invalidateFavoriteQueries } from "@/features/favorites/query-cache";
import {
	scopedUserQueryKey,
	telegramUserScope,
} from "@/features/favorites/user-query-scope";
import { instrumentUsesMessage } from "@/features/favorites/utils";

interface FavoritesContextValue {
	favorites: ReadonlyMap<string, Favorite>;
	isError: boolean;
	isLoading: boolean;
	handleAccessError(error: unknown): boolean;
	isMutating(symbol: string): boolean;
	toggle(symbol: string): void;
}

const FavoritesContext = createContext<FavoritesContextValue | undefined>(
	undefined,
);

function isAccessError(error: unknown): boolean {
	const status = (error as { status?: unknown }).status;
	return status === 401 || status === 403;
}

function conflictAlertCount(error: unknown): number | undefined {
	const details = (
		error as { info?: { error?: { details?: Record<string, unknown> } } }
	).info?.error?.details;
	const count = details?.alert_count;
	return typeof count === "number" && Number.isInteger(count) && count > 0
		? count
		: undefined;
}

export function FavoritesProvider({
	children,
	enabled,
}: {
	children: ReactNode;
	enabled: boolean;
}) {
	const queryClient = useQueryClient();
	const scope = telegramUserScope();
	const pendingSymbols = new Set(
		useMutationState({
			filters: {
				status: "pending",
				predicate: ({ options }) =>
					options.mutationKey?.[0] === getAddFavoriteMutationKey()[0] ||
					options.mutationKey?.[0] === getRemoveFavoriteMutationKey()[0],
			},
			select: ({ state }) =>
				(
					state.variables as
						| AddFavoriteMutationVariables
						| RemoveFavoriteMutationVariables
				).symbol,
		}),
	);
	const previousScope = useRef(scope);
	const [accessBlocked, setAccessBlocked] = useState(false);
	const [confirmation, setConfirmation] = useState<{
		symbol: string;
		alertCount: number;
	}>();
	const clearScope = useCallback(() => {
		void queryClient.removeQueries({
			predicate: ({ queryKey }) =>
				queryKey.some(
					(part) =>
						typeof part === "object" &&
						part !== null &&
						"userScope" in part &&
						(part as { userScope?: unknown }).userScope === scope,
				),
		});
	}, [queryClient, scope]);
	const handleAccessError = useCallback(
		(error: unknown) => {
			if (!isAccessError(error)) return false;
			setAccessBlocked(true);
			clearScope();
			return true;
		},
		[clearScope],
	);
	const query = useListFavorites({
		query: {
			enabled: enabled && !accessBlocked,
			queryKey: scopedUserQueryKey(getListFavoritesQueryKey(), scope),
			refetchInterval: 15_000,
			refetchOnMount: "always",
			retry: false,
			select: (response) => response.data,
		},
	});
	useEffect(() => {
		if (previousScope.current === scope) return;
		const staleScope = previousScope.current;
		previousScope.current = scope;
		setAccessBlocked(false);
		void queryClient.removeQueries({
			predicate: ({ queryKey }) =>
				queryKey.some(
					(part) =>
						typeof part === "object" &&
						part !== null &&
						"userScope" in part &&
						(part as { userScope?: unknown }).userScope === staleScope,
				),
		});
	}, [queryClient, scope]);
	useEffect(() => {
		if (query.error) handleAccessError(query.error);
	}, [handleAccessError, query.error]);
	useEffect(() => {
		if (!accessBlocked) return;
		const timer = window.setTimeout(() => setAccessBlocked(false), 15_000);
		return () => window.clearTimeout(timer);
	}, [accessBlocked]);

	const refreshFavorites = () => invalidateFavoriteQueries(queryClient);
	const addMutation = useAddFavorite({
		mutation: {
			onError: (error) => {
				handleAccessError(error);
				notifications.show({
					color: "red",
					message: "The favorite could not be added.",
					title: "Favorite update failed",
				});
			},
			onSuccess: refreshFavorites,
		},
	});
	const favorites = useMemo(
		() => new Map((query.data?.items ?? []).map((item) => [item.symbol, item])),
		[query.data],
	);
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
			onError: (error, { symbol, params }) => {
				handleAccessError(error);
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
				handleAccessError,
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

export function useFavorites() {
	const value = useContext(FavoritesContext);
	if (!value)
		throw new Error("useFavorites must be used inside FavoritesProvider");
	return value;
}
