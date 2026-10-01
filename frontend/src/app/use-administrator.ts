import { useGetCurrentUser } from "@/api/generated/api";

// Whether the current user manages the global scanner settings.
export function useAdministrator(enabled: boolean): boolean {
	const { data } = useGetCurrentUser({
		query: {
			enabled,
			retry: false,
			select: (response) => response.data.administrator,
			staleTime: Number.POSITIVE_INFINITY,
		},
	});
	return data === true;
}
