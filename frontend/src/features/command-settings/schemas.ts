import { z } from "zod";

import { historyLoadDepth } from "@/features/command-settings/constants";
import { formatNumber } from "@/utils/number-format";

const depthRange = `Depth must be from ${formatNumber(historyLoadDepth.min)} to ${formatNumber(historyLoadDepth.max)}`;

// The history load form, in the shape its inputs emit: depth is a string
// while its input is empty, and intervals keep the order they were chosen in.
export const historyLoadSchema = z.object({
	coins: z.array(z.string()).min(1, "Choose at least one coin"),
	depth: z
		.union([z.number(), z.string()], { error: depthRange })
		.pipe(
			z
				.number({ error: depthRange })
				.min(historyLoadDepth.min)
				.max(historyLoadDepth.max),
		),
	intervals: z.array(z.string()).min(1, "Choose at least one interval"),
});
