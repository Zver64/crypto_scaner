import { z } from "zod";

import { parseLevels } from "@/features/scanner-settings/utils";

export const indicatorTypeSchema = z.string({ error: "Choose an indicator" });

export const levelsSchema = z
	.array(z.string())
	.refine(
		(levels) => parseLevels(levels) !== undefined,
		"Levels must be numbers",
	);
