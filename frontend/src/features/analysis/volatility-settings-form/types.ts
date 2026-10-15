import type { z } from "zod";

import type { volatilitySettingsSchema } from "@/features/analysis/schemas";

export type VolatilitySettings = z.output<typeof volatilitySettingsSchema>;

// The settings as the number inputs emit them.
export type VolatilitySettingsDraft = z.input<typeof volatilitySettingsSchema>;

// Volatility settings a page keeps in its URL.
export interface VolatilitySettingsStateProps {
	initialSettings: VolatilitySettings;
	onSettingsCommit(settings: VolatilitySettings): void;
}
