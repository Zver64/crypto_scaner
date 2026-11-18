import {
	type getCurrentUserResponseSuccess,
	useGetCurrentUser,
} from "@/api/generated/api";

const selectAdministrator = (response: getCurrentUserResponseSuccess) =>
	response.data.administrator;

// Whether the current user manages the global scanner settings.
export function useAdministrator(enabled: boolean): boolean {
	const { data } = useGetCurrentUser({
		query: {
			enabled,
			retry: false,
			select: selectAdministrator,
			staleTime: Number.POSITIVE_INFINITY,
		},
	});
	return data === true;
}
