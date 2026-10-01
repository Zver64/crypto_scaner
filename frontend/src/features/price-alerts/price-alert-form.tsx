import { Box, Button, Group, NumberInput, Stack, Text } from "@mantine/core";
import type { FormEvent } from "react";
import type { CoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import { TargetChange } from "@/features/price-alerts/target-change";

interface PriceAlertFormProps {
	isEditing: boolean;
	isSaving: boolean;
	limit?: number;
	limitReached: boolean;
	onCancel(): void;
	onSubmit(event: FormEvent<HTMLFormElement>): void;
	onTargetChange(target: string): void;
	// Live price the target's percent change is measured from.
	priceSource?: CoinChartData;
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
	priceSource,
	target,
	targetError,
}: PriceAlertFormProps) {
	const formLocked = limitReached && !isEditing;
	return (
		<form onSubmit={onSubmit}>
			<Stack gap="sm">
				<NumberInput
					allowNegative={false}
					decimalScale={18}
					description="Positive USDT price, up to 18 decimal places"
					disabled={isSaving || formLocked}
					error={targetError}
					inputContainer={(input) => (
						<Group gap="sm" wrap="nowrap">
							<Box flex={1}>{input}</Box>
							<TargetChange source={priceSource} target={target} />
						</Group>
					)}
					label={isEditing ? "Edit target" : "New target"}
					// onChange converts to a JS number; the raw string keeps all 18 decimals.
					onValueChange={({ value }) => onTargetChange(value)}
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
						disabled={formLocked || target.trim() === ""}
						loading={isSaving}
						type="submit"
					>
						{isEditing ? "Save alert" : "Create alert"}
					</Button>
				</Group>
				{formLocked ? (
					<Text c="dimmed" size="sm">
						The limit of {limit} alerts has been reached.
					</Text>
				) : null}
			</Stack>
		</form>
	);
}
