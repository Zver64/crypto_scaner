import { Button, type ButtonProps } from "@mantine/core";
import { useSelector } from "@tanstack/react-form";
import type { ComponentProps, ReactNode } from "react";

import { useFormContext } from "@/components/form/form-context";

interface SubmitButtonProps
	extends
		Omit<ButtonProps, "type">,
		// Submits a form rendered without an HTML form element.
		Pick<ComponentProps<"button">, "onClick"> {
	children: ReactNode;
}

// Submits the form; disabled while its values are invalid, and loading while
// it submits.
export function SubmitButton({
	disabled,
	loading,
	...props
}: SubmitButtonProps) {
	const form = useFormContext();
	const canSubmit = useSelector(form.store, (state) => state.canSubmit);
	const isSubmitting = useSelector(form.store, (state) => state.isSubmitting);
	return (
		<Button
			{...props}
			disabled={disabled || !canSubmit}
			loading={loading || isSubmitting}
			type="submit"
		/>
	);
}
