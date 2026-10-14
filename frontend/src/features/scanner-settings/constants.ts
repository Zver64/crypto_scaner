import { createFieldMap } from "@tanstack/react-form";

import type { ScaleDraft } from "@/features/scanner-settings/types";

// Automatic bounds without levels.
export const emptyScaleDraft: ScaleDraft = {
	levels: [],
	scaleMax: "",
	scaleMin: "",
};

// Where the scale fields sit in a form: at its top level.
export const scaleFields = createFieldMap(emptyScaleDraft);
