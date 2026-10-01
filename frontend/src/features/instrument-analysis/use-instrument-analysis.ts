import { notifications } from "@mantine/notifications";
import { keepPreviousData } from "@tanstack/react-query";
import { useEffect } from "react";
import { useAnalyzeInstrument } from "@/api/generated/api";
import type {
	CriterionRequest,
	InstrumentAnalysisResponse,
} from "@/api/generated/models";
import {
	apiErrorCode,
	apiErrorMessage,
	unexpectedApiError,
} from "@/features/analysis/api-error";
import { hasExpectedInstrumentAnalysisEvaluations } from "@/features/analysis/semantics";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";

// Analyzes the instrument and reports failures, except missing history, which
// the page shows in place.
export function useInstrumentAnalysis(
	symbol: string,
	criterionSelections: readonly CriterionRequest[],
	enabled: boolean,
) {
	const query = useAnalyzeInstrument<InstrumentAnalysisResponse>(
		symbol,
		{ criteria: [...criterionSelections] },
		{
			query: {
				enabled,
				placeholderData: keepPreviousData,
				refetchOnMount: "always",
				retry: false,
				staleTime: 0,
				select: (response) => {
					if (
						!hasExpectedInstrumentAnalysisEvaluations(
							response.data.evaluations,
							criterionSelections,
						)
					) {
						throw unexpectedApiError();
					}
					return response.data;
				},
			},
		},
	);
	const insufficientHistory =
		query.isError && apiErrorCode(query.error) === "insufficient_data";
	useEffect(() => {
		if (query.isError && apiErrorCode(query.error) !== "insufficient_data") {
			notifications.show({
				id: `instrument-analysis-${symbol}-error`,
				autoClose: 5000,
				color: "red",
				message: apiErrorMessage(query.error),
				title: "Instrument Analysis failed",
			});
		}
	}, [query.error, query.isError, symbol]);
	useAnalysisWarningNotification(
		query.data?.warnings,
		"Instrument Analysis warning",
	);
	return {
		insufficientHistory,
		isFetching: query.isFetching,
		result: query.data,
	};
}
