import { useMutation } from "@tanstack/react-query";

import type { ErrorType } from "@/api/fetch";
import { validateStrategy } from "@/api/generated/api";
import type {
	ErrorResponse,
	Strategy,
	StrategyValidationInputKind,
} from "@/api/generated/models";
import type { CanonicalRules } from "@/features/strategy-settings/types";
import { withIndicators } from "@/features/strategy-settings/utils";

// Spells the rules of a strategy canonically and finds the indicators they
// read, which the builder needs to open it.
export function useCanonicalRules() {
	return useMutation<CanonicalRules, ErrorType<ErrorResponse>, Strategy>({
		mutationFn: async (strategy) => {
			const canonical = async (
				expression: string,
				kind: StrategyValidationInputKind,
			) =>
				expression === ""
					? { expression, indicators: [] }
					: (await validateStrategy({ expression, kind })).data;
			const [entry, exit, takeProfit, stopLoss] = await Promise.all([
				canonical(strategy.expression, "entry"),
				canonical(strategy.exit_expression, "exit"),
				canonical(strategy.take_profit_expression, "price"),
				canonical(strategy.stop_loss_expression, "price"),
			]);
			return {
				expressions: {
					expression: entry.expression,
					exit_expression: exit.expression,
					take_profit_expression: takeProfit.expression,
					stop_loss_expression: stopLoss.expression,
				},
				indicators: [entry, exit, takeProfit, stopLoss].reduce<
					CanonicalRules["indicators"]
				>((all, { indicators }) => withIndicators(all, indicators), []),
			};
		},
	});
}
