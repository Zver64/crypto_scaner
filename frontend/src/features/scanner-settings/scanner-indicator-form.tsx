import { Button, Group, Paper, Select, Stack, Text } from "@mantine/core";
import { useSelector } from "@tanstack/react-form";
import { useState } from "react";

import type {
	IndicatorType,
	ScannerIndicatorInput,
} from "@/api/generated/models";
import { useAppForm } from "@/components/form/use-app-form";
import { chartIntervalOptions } from "@/features/candle-chart/config";
import {
	emptyScaleDraft,
	scaleFields,
} from "@/features/scanner-settings/constants";
import { ScannerIndicatorParameterField } from "@/features/scanner-settings/scanner-indicator-parameter-field";
import { ScannerIndicatorPeriodsField } from "@/features/scanner-settings/scanner-indicator-periods-field";
import { ScannerIndicatorScaleFields } from "@/features/scanner-settings/scanner-indicator-scale-fields";
import type { ScannerIndicatorDraft } from "@/features/scanner-settings/types";
import {
	defaultParameterValues,
	indicatorGroups,
	indicatorTypeOptions,
	scannerIndicatorInput,
	tableColumnAllowed,
	withoutTableColumns,
} from "@/features/scanner-settings/utils";

const emptyDraft: ScannerIndicatorDraft = {
	...emptyScaleDraft,
	parameters: {},
	periods: {},
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
	const form = useAppForm({
		defaultValues: emptyDraft,
		onSubmit: ({ value }) => {
			// The form keeps its values after adding, so a similar indicator
			// needs only the changed fields.
			const input = scannerIndicatorInput(
				value,
				types.find((item) => item.type === value.type),
				periodOrder,
			);
			if (input) onSubmit(input);
		},
	});
	// The group only narrows the indicator choice and is not submitted.
	const [group, setGroup] = useState(allGroups);
	// The chosen type decides which fields are shown, and the Add button
	// follows the values as they change.
	const draft = useSelector(form.store, (state) => state.values);
	const type = types.find((item) => item.type === draft.type);
	const groupTypes =
		group === allGroups ? types : types.filter((item) => item.group === group);
	// A new type starts from its own defaults.
	const resetForType = (value: string | null) => {
		const next = types.find((item) => item.type === value);
		form.setFieldValue("levels", emptyScaleDraft.levels);
		form.setFieldValue("scaleMax", emptyScaleDraft.scaleMax);
		form.setFieldValue("scaleMin", emptyScaleDraft.scaleMin);
		form.setFieldValue("parameters", next ? defaultParameterValues(next) : {});
		// A table choice made for a single-output type does not carry over.
		if (!tableColumnAllowed(next)) {
			form.setFieldValue("periods", withoutTableColumns);
		}
	};

	const selectGroup = (value: string | null) => {
		const next = value ?? allGroups;
		setGroup(next);
		// The chosen indicator is cleared when the group no longer offers it.
		if (type && next !== allGroups && type.group !== next) {
			form.setFieldValue("type", null);
		}
	};

	return (
		<Paper p="sm" radius="md" withBorder>
			<form
				onSubmit={(event) => {
					event.preventDefault();
					void form.handleSubmit();
				}}
			>
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
					<form.AppField
						listeners={{ onChange: ({ value }) => resetForType(value) }}
						name="type"
					>
						{(field) => (
							<field.SelectField
								aria-label="Indicator"
								clearable
								data={indicatorTypeOptions(groupTypes)}
								nothingFoundMessage="No indicators found"
								placeholder="Search indicators"
								searchable
							/>
						)}
					</form.AppField>
					{type?.parameters.map((parameter) => (
						<form.AppField
							key={parameter.key}
							name={`parameters.${parameter.key}`}
						>
							{() => <ScannerIndicatorParameterField parameter={parameter} />}
						</form.AppField>
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
					<form.Field name="periods">
						{(field) => (
							<ScannerIndicatorPeriodsField
								onChange={field.handleChange}
								tableAllowed={tableColumnAllowed(type)}
								value={field.state.value}
							/>
						)}
					</form.Field>
					{type && !type.overlay ? (
						<ScannerIndicatorScaleFields fields={scaleFields} form={form} />
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
