import { Box, Group } from "@mantine/core";

import { useAppForm } from "@/components/form";

interface ApiTokenCreateProps {
	isPending: boolean;
	// Calls onCreated once the token exists, which clears the name.
	onCreate(name: string, onCreated: () => void): void;
}

export function ApiTokenCreate({ isPending, onCreate }: ApiTokenCreateProps) {
	const form = useAppForm({
		defaultValues: { name: "" },
		onSubmit: ({ value }) => onCreate(value.name.trim(), () => form.reset()),
	});
	return (
		<form
			onSubmit={(event) => {
				event.preventDefault();
				void form.handleSubmit();
			}}
		>
			<form.AppForm>
				<form.AppField
					name="name"
					validators={{
						onChange: ({ value }) =>
							value.trim() === "" ? "Enter a token name" : undefined,
					}}
				>
					{(field) => (
						<field.TextField
							disabled={isPending}
							// The button sits beside the input, above its error.
							inputContainer={(input) => (
								<Group gap="sm" wrap="nowrap">
									<Box flex={1}>{input}</Box>
									<form.SubmitButton loading={isPending}>
										Create
									</form.SubmitButton>
								</Group>
							)}
							label="New token"
							maxLength={64}
							placeholder="Where it is used, such as laptop CLI"
						/>
					)}
				</form.AppField>
			</form.AppForm>
		</form>
	);
}
