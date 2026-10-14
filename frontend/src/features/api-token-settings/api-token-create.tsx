import { Button, Group } from "@mantine/core";
import { useSelector } from "@tanstack/react-form";

import { useAppForm } from "@/components/form/use-app-form";

interface ApiTokenCreateProps {
	isPending: boolean;
	// Calls onCreated once the token exists, which clears the name.
	onCreate(name: string, onCreated: () => void): void;
}

export function ApiTokenCreate({ isPending, onCreate }: ApiTokenCreateProps) {
	const form = useAppForm({
		defaultValues: { name: "" },
		onSubmit: ({ value }) => {
			const trimmed = value.name.trim();
			if (trimmed === "") return;
			onCreate(trimmed, () => form.reset());
		},
	});
	// The Create button follows the name as it is typed.
	const empty = useSelector(
		form.store,
		(state) => state.values.name.trim() === "",
	);
	return (
		<form
			onSubmit={(event) => {
				event.preventDefault();
				void form.handleSubmit();
			}}
		>
			<Group align="flex-end" wrap="nowrap">
				<form.AppField name="name">
					{(field) => (
						<field.TextField
							disabled={isPending}
							flex={1}
							label="New token"
							maxLength={64}
							placeholder="Where it is used, such as laptop CLI"
						/>
					)}
				</form.AppField>
				<Button disabled={empty} loading={isPending} type="submit">
					Create
				</Button>
			</Group>
		</form>
	);
}
