import { keepPreviousData } from "@tanstack/react-query";

import {
	analyzeInstrument,
	type analyzeInstrumentResponseSuccess,
	useAnalyzeInstrument,
} from "@/api/generated/api";
import type { CriterionRequest } from "@/api/generated/models";
import {
	apiErrorCode,
	unexpectedApiError,
} from "@/features/analysis/api-error";
import { hasExpectedInstrumentAnalysisEvaluations } from "@/features/analysis/semantics";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";

const selectAnalysis = (response: analyzeInstrumentResponseSuccess) =>
	response.data;

// Analyzes the instrument and reports failures, except missing history, which
// the page shows in place.
export function useInstrumentAnalysis(
	symbol: string,
	criterionSelections: readonly CriterionRequest[],
	enabled: boolean,
) {
	const request = { criteria: [...criterionSelections] };
	const query = useAnalyzeInstrument(symbol, request, {
		query: {
			enabled,
			meta: {
				errorNotification: {
					id: `instrument-analysis-${symbol}-error`,
					ignoredCode: "insufficient_data",
					title: "Instrument Analysis failed",
				},
			},
			placeholderData: keepPreviousData,
			// Validated when fetched, so an unexpected response fails the query
			// and reaches the query cache error handler.
			queryFn: async ({ signal }) => {
				const response = await analyzeInstrument(symbol, request, { signal });
				if (
					!hasExpectedInstrumentAnalysisEvaluations(
						response.data.evaluations,
						criterionSelections,
					)
				) {
					throw unexpectedApiError();
				}
				return response;
			},
			refetchOnMount: "always",
			retry: false,
			staleTime: 0,
			select: selectAnalysis,
		},
	});
	const insufficientHistory =
		query.isError && apiErrorCode(query.error) === "insufficient_data";
	useAnalysisWarningNotification(
		query.data?.warnings,
		"Instrument Analysis warning",
	);
	return {
		insufficientHistory,
		isFetching: query.isFetching,
		// The result shown is that of earlier criteria or another symbol.
		isPlaceholderData: query.isPlaceholderData,
		result: query.data,
	};
}
