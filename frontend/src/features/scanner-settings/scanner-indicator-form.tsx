import { Button, Group, Paper, Select, Stack, Text } from "@mantine/core";
import { type FormEvent, useState } from "react";
import type {
	IndicatorType,
	ScannerIndicatorInput,
} from "@/api/generated/models";
import { chartIntervalOptions } from "@/components/price-history-chart/config";
import { ScannerIndicatorParameterField } from "@/features/scanner-settings/scanner-indicator-parameter-field";
import { ScannerIndicatorPeriodsField } from "@/features/scanner-settings/scanner-indicator-periods-field";
import { ScannerIndicatorScaleFields } from "@/features/scanner-settings/scanner-indicator-scale-fields";
import type { ScannerIndicatorDraft } from "@/features/scanner-settings/types";
import {
	defaultParameterValues,
	indicatorGroups,
	indicatorTypeOptions,
	parseLevels,
	scannerIndicatorInput,
	tableColumnAllowed,
	withoutTableColumns,
} from "@/features/scanner-settings/utils";

const emptyDraft: ScannerIndicatorDraft = {
	levels: [],
	parameters: {},
	periods: {},
	scaleMax: "",
	scaleMin: "",
	type: null,
};

const periodOrder = chartIntervalOptions.map(({ value }) => value);

const allGroups = "all";

interface ScannerIndicatorFormProps {
	isSaving: boolean;
	onSubmit(input: ScannerIndicatorInput): void;
	types: readonly IndicatorType[];
}

export function ScannerIndicatorForm({
	isSaving,
	onSubmit,
	types,
}: ScannerIndicatorFormProps) {
	const [draft, setDraft] = useState(emptyDraft);
	const [group, setGroup] = useState(allGroups);
	const type = types.find((item) => item.type === draft.type);
	const groupTypes =
		group === allGroups ? types : types.filter((item) => item.group === group);
	const levelsValid = parseLevels(draft.levels) !== undefined;
	const submit = (event: FormEvent) => {
		event.preventDefault();
		if (!type) return;
		// The form keeps its values after adding, so a similar indicator needs
		// only the changed fields.
		const input = scannerIndicatorInput(draft, type, periodOrder);
		if (input) onSubmit(input);
	};
	const selectType = (value: string | null) => {
		const next = types.find((item) => item.type === value);
		setDraft((current) => ({
			...current,
			levels: [],
			parameters: next ? defaultParameterValues(next) : {},
			scaleMax: "",
			scaleMin: "",
			// A table choice made for a single-output type does not carry over.
			periods: tableColumnAllowed(next)
				? current.periods
				: withoutTableColumns(current.periods),
			type: value,
		}));
	};

	const selectGroup = (value: string | null) => {
		const next = value ?? allGroups;
		setGroup(next);
		// The chosen indicator is cleared when the group no longer offers it.
		if (type && next !== allGroups && type.group !== next) selectType(null);
	};

	return (
		<Paper p="sm" radius="md" withBorder>
			<Stack component="form" gap="sm" onSubmit={submit}>
				<Select
					allowDeselect={false}
					aria-label="Indicator group"
					data={[
						{ label: "All", value: allGroups },
						...indicatorGroups(types).map((item) => ({
							label: item,
							value: item,
						})),
					]}
					onChange={selectGroup}
					value={group}
				/>
				<Select
					aria-label="Indicator"
					clearable
					data={indicatorTypeOptions(groupTypes)}
					nothingFoundMessage="No indicators found"
					onChange={selectType}
					placeholder="Search indicators"
					searchable
					value={draft.type}
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
				<ScannerIndicatorPeriodsField
					onChange={(periods) =>
						setDraft((current) => ({ ...current, periods }))
					}
					tableAllowed={tableColumnAllowed(type)}
					value={draft.periods}
				/>
				{type && !type.overlay ? (
					<ScannerIndicatorScaleFields
						onChange={(scale) =>
							setDraft((current) => ({ ...current, ...scale }))
						}
						value={{
							levels: draft.levels,
							scaleMax: draft.scaleMax,
							scaleMin: draft.scaleMin,
						}}
					/>
				) : null}
				<Group justify="flex-end">
					<Button
						disabled={
							!type || !levelsValid || Object.keys(draft.periods).length === 0
						}
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
