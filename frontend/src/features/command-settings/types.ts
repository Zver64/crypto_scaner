import type { z } from "zod";

import type { historyLoadSchema } from "@/features/command-settings/schemas";

export type HistoryLoadValues = z.input<typeof historyLoadSchema>;
