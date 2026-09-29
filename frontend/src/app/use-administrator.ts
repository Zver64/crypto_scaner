import { useGetCurrentUser } from "@/api/generated/api";
import { telegramRequestOptions } from "@/app/telegram";

// Whether the current user manages the global scanner settings.
export function useAdministrator(enabled: boolean): boolean {
	const { data } = useGetCurrentUser({
		fetch: telegramRequestOptions(),
		query: {
			enabled,
			retry: false,
			select: (response) => response.data.administrator,
			staleTime: Number.POSITIVE_INFINITY,
		},
	});
	return data === true;
}
