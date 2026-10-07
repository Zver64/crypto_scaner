import { Button, Group, TextInput } from "@mantine/core";
import { useState } from "react";

interface ApiTokenCreateProps {
	isPending: boolean;
	// Calls onCreated once the token exists, which clears the name.
	onCreate(name: string, onCreated: () => void): void;
}

export function ApiTokenCreate({ isPending, onCreate }: ApiTokenCreateProps) {
	const [name, setName] = useState("");
	const trimmed = name.trim();
	return (
		<form
			onSubmit={(event) => {
				event.preventDefault();
				if (trimmed === "") return;
				onCreate(trimmed, () => setName(""));
			}}
		>
			<Group align="flex-end" wrap="nowrap">
				<TextInput
					disabled={isPending}
					flex={1}
					label="New token"
					maxLength={64}
					onChange={(event) => setName(event.currentTarget.value)}
					placeholder="Where it is used, such as laptop CLI"
					value={name}
				/>
				<Button disabled={trimmed === ""} loading={isPending} type="submit">
					Create
				</Button>
			</Group>
		</form>
	);
}
