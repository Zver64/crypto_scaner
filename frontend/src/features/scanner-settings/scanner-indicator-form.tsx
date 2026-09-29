import {
	Button,
	Checkbox,
	Group,
	NumberInput,
	Paper,
	SegmentedControl,
	Select,
	Stack,
	TagsInput,
	Text,
	Title,
} from "@mantine/core";
import { type FormEvent, useState } from "react";
import type {
	IndicatorType,
	ScannerIndicatorInput,
} from "@/api/generated/models";
import { chartIntervalOptions } from "@/components/price-history-chart/config";
import { ScannerIndicatorParameterField } from "@/features/scanner-settings/scanner-indicator-parameter-field";
import type { ScannerIndicatorDraft } from "@/features/scanner-settings/types";
import {
	defaultParameterValues,
	indicatorTypeOptions,
	parseLevels,
	scannerIndicatorInput,
	tableColumnAllowed,
} from "@/features/scanner-settings/utils";

const emptyDraft: ScannerIndicatorDraft = {
	interval: "1d",
	levels: [],
	parameters: {},
	scaleMax: "",
	scaleMin: "",
	showInTable: false,
	type: null,
};

interface ScannerIndicatorFormProps {
	isSaving: boolean;
	onSubmit(input: ScannerIndicatorInput, reset: () => void): void;
	types: readonly IndicatorType[];
}

export function ScannerIndicatorForm({
	isSaving,
	onSubmit,
	types,
}: ScannerIndicatorFormProps) {
	const [draft, setDraft] = useState(emptyDraft);
	const type = types.find((item) => item.type === draft.type);
	const levelsValid = parseLevels(draft.levels) !== undefined;
	const submit = (event: FormEvent) => {
		event.preventDefault();
		if (!type) return;
		const input = scannerIndicatorInput(draft, type);
		if (input) onSubmit(input, () => setDraft(emptyDraft));
	};
	const selectType = (value: string | null) => {
		const next = types.find((item) => item.type === value);
		setDraft((current) => ({
			...current,
			levels: [],
			parameters: next ? defaultParameterValues(next) : {},
			scaleMax: "",
			scaleMin: "",
			showInTable: current.showInTable && tableColumnAllowed(next),
			type: value,
		}));
	};

	return (
		<Paper p="sm" radius="md" withBorder>
			<Stack component="form" gap="sm" onSubmit={submit}>
				<Title order={2} size="h4">
					Add indicator
				</Title>
				<Select
					data={indicatorTypeOptions(types)}
					label="Indicator"
					nothingFoundMessage="No indicators found"
					onChange={selectType}
					placeholder="Search indicators"
					searchable
					value={draft.type}
				/>
				<SegmentedControl
					data={chartIntervalOptions.map(({ label, value }) => ({
						label,
						value,
					}))}
					onChange={(value) =>
						setDraft((current) => ({
							...current,
							interval:
								chartIntervalOptions.find((option) => option.value === value)
									?.value ?? current.interval,
						}))
					}
					value={draft.interval}
				/>
				{type?.parameters.map((parameter) => (
					<ScannerIndicatorParameterField
						key={parameter.key}
						onChange={(value) =>
							setDraft((current) => ({
								...current,
								parameters: { ...current.parameters, [parameter.key]: value },
							}))
						}
						parameter={parameter}
						value={draft.parameters[parameter.key]}
					/>
				))}
				{type ? (
					<Text c="dimmed" size="xs">
						{type.overlay
							? "Drawn over the candles"
							: "Drawn in a pane below the candles"}
						{" · "}
						Outputs: {type.outputs.join(", ")}
					</Text>
				) : null}
				<Checkbox
					checked={draft.showInTable && tableColumnAllowed(type)}
					description="Only indicators with one output can be table columns."
					disabled={!tableColumnAllowed(type)}
					label="Show in tables"
					onChange={(event) => {
						const { checked } = event.currentTarget;
						setDraft((current) => ({ ...current, showInTable: checked }));
					}}
				/>
				{type && !type.overlay ? (
					<>
						<Group grow>
							<NumberInput
								label="Scale min"
								onChange={(value) =>
									setDraft((current) => ({ ...current, scaleMin: value }))
								}
								placeholder="auto"
								value={draft.scaleMin}
							/>
							<NumberInput
								label="Scale max"
								onChange={(value) =>
									setDraft((current) => ({ ...current, scaleMax: value }))
								}
								placeholder="auto"
								value={draft.scaleMax}
							/>
						</Group>
						<TagsInput
							error={levelsValid ? undefined : "Levels must be numbers"}
							label="Levels"
							onChange={(levels) =>
								setDraft((current) => ({ ...current, levels }))
							}
							placeholder="For example 30, 70"
							value={draft.levels}
						/>
					</>
				) : null}
				<Group justify="flex-end">
					<Button
						disabled={!type || !levelsValid}
						loading={isSaving}
						type="submit"
					>
						Add
					</Button>
				</Group>
			</Stack>
		</Paper>
	);
}
