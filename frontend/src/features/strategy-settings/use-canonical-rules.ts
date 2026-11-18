import { useMutation } from "@tanstack/react-query";

import type { ErrorType } from "@/api/fetch";
import { validateStrategy } from "@/api/generated/api";
import type {
	ErrorResponse,
	Strategy,
	StrategyValidationInputKind,
} from "@/api/generated/models";
import type { CanonicalRules } from "@/features/strategy-settings/types";

// Spells the rules of a strategy canonically, which the builder needs to
// open it.
export function useCanonicalRules() {
	return useMutation<CanonicalRules, ErrorType<ErrorResponse>, Strategy>({
		mutationFn: async (strategy) => {
			const canonical = async (
				expression: string,
				kind: StrategyValidationInputKind,
			) =>
				expression === ""
					? expression
					: (await validateStrategy({ expression, kind })).data.expression;
			const [expression, exit, takeProfit, stopLoss] = await Promise.all([
				canonical(strategy.rules.expression, "entry"),
				canonical(strategy.rules.exit_expression, "exit"),
				canonical(strategy.rules.take_profit_expression, "price"),
				canonical(strategy.rules.stop_loss_expression, "price"),
			]);
			return {
				expression,
				exit_expression: exit,
				take_profit_expression: takeProfit,
				stop_loss_expression: stopLoss,
			};
		},
	});
}
