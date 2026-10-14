import {
	type getCandleHistoryLoadResponseSuccess,
	useGetCandleHistoryLoad,
} from "@/api/generated/api";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { historyLoadPollInterval } from "@/features/command-settings/constants";

const selectJob = (response: getCandleHistoryLoadResponseSuccess) =>
	response.status === 200 ? response.data : null;

// The latest history load, polled while it runs, or null when none has run
// since the server started.
export function useHistoryLoadJob() {
	const permission = useBusinessRequestPermission();
	return useGetCandleHistoryLoad({
		query: {
			enabled: permission.allowed,
			retry: false,
			select: selectJob,
			refetchInterval: (query) =>
				query.state.data?.status === 200 &&
				query.state.data.data.status === "running"
					? historyLoadPollInterval
					: false,
		},
	});
}
