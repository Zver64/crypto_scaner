import { Button, Group, TextInput } from "@mantine/core";
import { useForm } from "@mantine/form";

interface ApiTokenCreateProps {
	isPending: boolean;
	// Calls onCreated once the token exists, which clears the name.
	onCreate(name: string, onCreated: () => void): void;
}

export function ApiTokenCreate({ isPending, onCreate }: ApiTokenCreateProps) {
	// Controlled, because the Create button follows the name as it is typed.
	const form = useForm({ initialValues: { name: "" }, mode: "controlled" });
	return (
		<form
			onSubmit={form.onSubmit(({ name }) => {
				const trimmed = name.trim();
				if (trimmed === "") return;
				onCreate(trimmed, form.reset);
			})}
		>
			<Group align="flex-end" wrap="nowrap">
				<TextInput
					disabled={isPending}
					flex={1}
					label="New token"
					maxLength={64}
					placeholder="Where it is used, such as laptop CLI"
					{...form.getInputProps("name")}
				/>
				<Button
					disabled={form.values.name.trim() === ""}
					loading={isPending}
					type="submit"
				>
					Create
				</Button>
			</Group>
		</form>
	);
}
