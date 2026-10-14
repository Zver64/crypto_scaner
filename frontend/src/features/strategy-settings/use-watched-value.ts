import type { UseFormReturnType } from "@mantine/form";
import { useState } from "react";

import type {
	StrategyFormField,
	StrategyFormValues,
} from "@/features/strategy-settings/types";

// What the calling component shows of an uncontrolled form field, kept
// current through the form's watch, so a field's edits render the components
// that show it, not the whole form (except when the field turns dirty or
// clean, which re-renders the form's owner). Selecting a part, such as
// whether a rule exists, renders them only when that part changes.
export function useWatchedValue<Field extends StrategyFormField, Selected>(
	form: UseFormReturnType<StrategyFormValues>,
	field: Field,
	select: (value: StrategyFormValues[Field]) => Selected,
): Selected {
	const [selected, setSelected] = useState(() =>
		select(form.getValues()[field]),
	);
	form.watch(field, () => setSelected(select(form.getValues()[field])));
	return selected;
}
