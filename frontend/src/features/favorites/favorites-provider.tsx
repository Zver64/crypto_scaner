import { notifications } from "@mantine/notifications";
import { useQueryClient } from "@tanstack/react-query";
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
	getListFavoritesQueryKey,
	useAddFavorite,
	useListFavorites,
	useRemoveFavorite,
} from "@/api/generated/api";
import type { Favorite } from "@/api/generated/models";
import { telegramRequestOptions } from "@/app/telegram";
import { FavoriteRemovalConfirmation } from "@/features/favorites/favorite-removal-confirmation";
import { invalidateFavoriteQueries } from "@/features/favorites/query-cache";
import {
	scopedUserQueryKey,
	telegramUserScope,
} from "@/features/favorites/user-query-scope";

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
		fetch: telegramRequestOptions(),
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
		fetch: telegramRequestOptions(),
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
	const removeMutation = useRemoveFavorite({
		fetch: telegramRequestOptions(),
		mutation: { onSuccess: refreshFavorites },
	});
	const favorites = useMemo(
		() => new Map((query.data?.items ?? []).map((item) => [item.symbol, item])),
		[query.data],
	);
	const remove = (symbol: string, confirmAlerts: boolean) => {
		removeMutation.mutate(
			{ symbol, params: { confirm_alerts: confirmAlerts } },
			{
				onError: (error) => {
					handleAccessError(error);
					const alertCount = conflictAlertCount(error);
					if (!confirmAlerts && error.status === 409 && alertCount) {
						setConfirmation({ symbol, alertCount });
						return;
					}
					notifications.show({
						color: "red",
						message: "The favorite could not be removed.",
						title: "Favorite update failed",
					});
				},
			},
		);
	};
	const toggle = (symbol: string) => {
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
		const symbol = confirmation.symbol;
		removeMutation.mutate(
			{ symbol, params: { confirm_alerts: true } },
			{
				onError: (error) => {
					handleAccessError(error);
					notifications.show({
						color: "red",
						message: "The favorite could not be removed.",
						title: "Favorite update failed",
					});
				},
				onSuccess: () => setConfirmation(undefined),
			},
		);
	};
	const pendingSymbol =
		(addMutation.variables?.symbol ?? removeMutation.variables?.symbol) ||
		undefined;

	return (
		<FavoritesContext
			value={{
				favorites,
				handleAccessError,
				isError: query.isError,
				isLoading: query.isPending,
				isMutating: (symbol) =>
					(addMutation.isPending || removeMutation.isPending) &&
					pendingSymbol === symbol,
				toggle,
			}}
		>
			{children}
			<FavoriteRemovalConfirmation
				alertCount={confirmation?.alertCount ?? 0}
				isPending={removeMutation.isPending}
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
