import { Button, Group, Paper, Select, Stack, Text } from "@mantine/core";
import { useForm } from "@mantine/form";
import { useState } from "react";
import type {
	IndicatorType,
	ScannerIndicatorInput,
} from "@/api/generated/models";
import { chartIntervalOptions } from "@/features/candle-chart/config";
import { ScannerIndicatorParameterField } from "@/features/scanner-settings/scanner-indicator-parameter-field";
import { ScannerIndicatorPeriodsField } from "@/features/scanner-settings/scanner-indicator-periods-field";
import { ScannerIndicatorScaleFields } from "@/features/scanner-settings/scanner-indicator-scale-fields";
import type { ScannerIndicatorDraft } from "@/features/scanner-settings/types";
import {
	defaultParameterValues,
	indicatorGroups,
	indicatorTypeOptions,
	levelsError,
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
	// Controlled, because the chosen type decides which fields are shown and
	// the levels error and the Add button follow the values as they change.
	const form = useForm({
		initialValues: emptyDraft,
		mode: "controlled",
		validate: { levels: levelsError },
		validateInputOnChange: true,
	});
	// The group only narrows the indicator choice and is not submitted.
	const [group, setGroup] = useState(allGroups);
	const draft = form.values;
	const type = types.find((item) => item.type === draft.type);
	const groupTypes =
		group === allGroups ? types : types.filter((item) => item.group === group);
	const submit = form.onSubmit((values) => {
		// The form keeps its values after adding, so a similar indicator needs
		// only the changed fields.
		const input = scannerIndicatorInput(
			values,
			types.find((item) => item.type === values.type),
			periodOrder,
		);
		if (input) onSubmit(input);
	});
	const selectType = (value: string | null) => {
		const next = types.find((item) => item.type === value);
		const { periods } = form.getValues();
		form.setValues({
			levels: [],
			parameters: next ? defaultParameterValues(next) : {},
			scaleMax: "",
			scaleMin: "",
			// A table choice made for a single-output type does not carry over.
			periods: tableColumnAllowed(next)
				? periods
				: withoutTableColumns(periods),
			type: value,
		});
	};

	const selectGroup = (value: string | null) => {
		const next = value ?? allGroups;
		setGroup(next);
		// The chosen indicator is cleared when the group no longer offers it.
		if (type && next !== allGroups && type.group !== next) selectType(null);
	};

	return (
		<Paper p="sm" radius="md" withBorder>
			<form onSubmit={submit}>
				<Stack gap="sm">
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
							form={form}
							key={parameter.key}
							parameter={parameter}
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
						form={form}
						tableAllowed={tableColumnAllowed(type)}
					/>
					{type && !type.overlay ? (
						<ScannerIndicatorScaleFields form={form} />
					) : null}
					<Group justify="flex-end">
						<Button
							disabled={!scannerIndicatorInput(draft, type, periodOrder)}
							loading={isSaving}
							type="submit"
						>
							Add
						</Button>
					</Group>
				</Stack>
			</form>
		</Paper>
	);
}
