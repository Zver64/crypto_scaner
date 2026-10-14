import { Box, Button, Group, NumberInput, Stack, Text } from "@mantine/core";
import type { FormEvent, ReactNode } from "react";

import type { CoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import { TargetChange } from "@/features/price-alerts/target-change";

interface PriceAlertFormProps {
	isEditing: boolean;
	isSaving: boolean;
	limit?: number;
	limitReached: boolean;
	onCancel(): void;
	onSubmit(event: FormEvent<HTMLFormElement>): void;
	onTargetChange(target: number | string): void;
	// Live price the target's percent change is measured from.
	priceSource?: CoinChartData;
	// The form's submit button, with the label and lock this form gives it.
	renderSubmit(props: { children: string; disabled: boolean }): ReactNode;
	target: number | string;
	targetError?: ReactNode;
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
	renderSubmit,
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
						<Group gap="xs" wrap="nowrap">
							<Box flex={1} miw={0}>
								{input}
							</Box>
							{isEditing ? (
								<Button onClick={onCancel} type="button" variant="default">
									Cancel
								</Button>
							) : null}
							{renderSubmit({
								children: isEditing ? "Save alert" : "Create alert",
								disabled: formLocked,
							})}
						</Group>
					)}
					label={isEditing ? "Edit target" : "New target"}
					onChange={onTargetChange}
					placeholder="0.00"
					rightSection={<TargetChange source={priceSource} target={target} />}
					rightSectionPointerEvents="none"
					rightSectionWidth="12ch"
					value={target}
				/>
				{formLocked ? (
					<Text c="dimmed" size="sm">
						The limit of {limit} alerts has been reached.
					</Text>
				) : null}
			</Stack>
		</form>
	);
}
