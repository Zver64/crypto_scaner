import {
	keepPreviousData,
	queryOptions,
	useQuery,
} from "@tanstack/react-query";
import { hasExpectedInstrumentAnalysisEvaluations } from "@/api/analysis-contract";
import {
	ApiError,
	type CriterionSelection,
	fetchInstrumentAnalysis,
} from "@/api/client";
import { getTelegramInitData } from "@/app/telegram";

export function instrumentAnalysisQueryOptions(
	symbol: string,
	criteria: readonly CriterionSelection[],
) {
	return queryOptions({
		queryFn: async () => {
			const result = await fetchInstrumentAnalysis(symbol, criteria, {
				initData: getTelegramInitData(),
			});
			if (
				!hasExpectedInstrumentAnalysisEvaluations(result.evaluations, criteria)
			) {
				throw new ApiError("unexpected_error");
			}
			return result;
		},
		queryKey: instrumentAnalysisQueryKey(symbol, criteria),
		placeholderData: keepPreviousData,
		retry: false,
		staleTime: Number.POSITIVE_INFINITY,
	});
}

export function useInstrumentAnalysisQuery(
	symbol: string,
	criteria: readonly CriterionSelection[],
	enabled: boolean,
) {
	return useQuery({
		...instrumentAnalysisQueryOptions(symbol, criteria),
		enabled,
	});
}

export function instrumentAnalysisQueryKey(
	symbol: string,
	criteria: readonly CriterionSelection[],
) {
	return ["instrument-analysis", symbol, criteria] as const;
}
