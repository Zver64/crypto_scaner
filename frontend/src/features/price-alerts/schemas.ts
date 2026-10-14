import { z } from "zod";

import { normalizePriceTarget } from "@/features/price-alerts/target";

// A target as the number input emits it, parsed to its normalized decimal.
export const priceTargetSchema = z
	.union([z.number(), z.string()], { error: "Enter a valid decimal price." })
	.transform((target, context) => {
		const result = normalizePriceTarget(target);
		if (result.error !== undefined) {
			context.addIssue({ code: "custom", message: result.error });
			return z.NEVER;
		}
		return result.value;
	});
