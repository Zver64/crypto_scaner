import { Button, Group, Stack, Text, TextInput } from "@mantine/core";
import type { FormEvent } from "react";

interface PriceAlertFormProps {
	isEditing: boolean;
	isSaving: boolean;
	limit?: number;
	limitReached: boolean;
	onCancel(): void;
	onSubmit(event: FormEvent<HTMLFormElement>): void;
	onTargetChange(target: string): void;
	target: string;
	targetError?: string;
}

export function PriceAlertForm({
	isEditing,
	isSaving,
	limit,
	limitReached,
	onCancel,
	onSubmit,
	onTargetChange,
	target,
	targetError,
}: PriceAlertFormProps) {
	return (
		<form onSubmit={onSubmit}>
			<Stack gap="sm">
				<TextInput
					description="Positive USDT price, up to 18 decimal places"
					disabled={isSaving || (limitReached && !isEditing)}
					error={targetError}
					label={isEditing ? "Edit target" : "New target"}
					onChange={(event) => onTargetChange(event.currentTarget.value)}
					placeholder="0.00"
					value={target}
				/>
				<Group justify="flex-end">
					{isEditing ? (
						<Button onClick={onCancel} type="button" variant="default">
							Cancel
						</Button>
					) : null}
					<Button
						disabled={limitReached && !isEditing}
						loading={isSaving}
						type="submit"
					>
						{isEditing ? "Save alert" : "Create alert"}
					</Button>
				</Group>
				{limitReached && !isEditing ? (
					<Text c="dimmed" size="sm">
						The limit of {limit} alerts has been reached.
					</Text>
				) : null}
			</Stack>
		</form>
	);
}
