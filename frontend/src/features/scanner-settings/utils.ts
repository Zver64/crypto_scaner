import type { ErrorType } from "@/api/fetch";
import type {
	CandleInterval,
	ErrorResponse,
	IndicatorType,
	ScannerIndicator,
	ScannerIndicatorInput,
	ScannerIndicatorScale,
} from "@/api/generated/models";
import type {
	ParameterValues,
	PeriodChoices,
	ScaleDraft,
	ScannerIndicatorDraft,
} from "@/features/scanner-settings/types";
import { describeApiError } from "@/utils/api-error";

export function defaultParameterValues(type: IndicatorType): ParameterValues {
	return Object.fromEntries(
		type.parameters.map((parameter) => [parameter.key, parameter.default]),
	);
}

// Select data grouped like the TA-Lib function groups.
export function indicatorTypeOptions(types: readonly IndicatorType[]) {
	const groups = new Map<string, { label: string; value: string }[]>();
	for (const type of types) {
		const items = groups.get(type.group) ?? [];
		items.push({
			label: `${type.type.toUpperCase()} — ${type.title}`,
			value: type.type,
		});
		groups.set(type.group, items);
	}
	return [...groups].map(([group, items]) => ({ group, items }));
}

// Distinct indicator groups in catalog order.
export function indicatorGroups(types: readonly IndicatorType[]): string[] {
	return [...new Set(types.map((type) => type.group))];
}

export function tableColumnAllowed(type: IndicatorType | undefined): boolean {
	return type?.outputs.length === 1;
}

// Keeps the chosen periods and their chart choices and clears their table
// column choices.
export function withoutTableColumns(periods: PeriodChoices): PeriodChoices {
	return Object.fromEntries(
		Object.entries(periods).map(([interval, choice]) => [
			interval,
			{ ...choice, showInTable: false },
		]),
	);
}

export function parseLevels(levels: readonly string[]): number[] | undefined {
	const values = levels.map((level) => Number(level.trim()));
	return values.every(Number.isFinite) ? values : undefined;
}

// Builds a pane scale; returns undefined while a level is not a number.
export function indicatorScale(
	draft: ScaleDraft,
): ScannerIndicatorScale | undefined {
	const levels = parseLevels(draft.levels);
	if (!levels) return undefined;
	const scale: ScannerIndicatorScale = { levels };
	if (draft.scaleMin !== "") scale.min = Number(draft.scaleMin);
	if (draft.scaleMax !== "") scale.max = Number(draft.scaleMax);
	return scale;
}

export function scaleDraft(scale: ScannerIndicatorScale): ScaleDraft {
	return {
		levels: scale.levels.map(String),
		scaleMax: scale.max ?? "",
		scaleMin: scale.min ?? "",
	};
}

// Builds the create request, one entry per chosen period in the given order;
// returns undefined while no period is chosen or a level is not a number.
export function scannerIndicatorInput(
	draft: ScannerIndicatorDraft,
	type: IndicatorType,
	periods: readonly CandleInterval[],
): ScannerIndicatorInput | undefined {
	const intervals = periods.flatMap((interval) => {
		const choice = draft.periods[interval];
		return choice
			? [
					{
						interval,
						show_in_table: choice.showInTable && tableColumnAllowed(type),
						show_in_chart: choice.showInChart,
					},
				]
			: [];
	});
	if (intervals.length === 0) return undefined;
	const parameters = Object.fromEntries(
		Object.entries(draft.parameters).flatMap(([key, value]) =>
			value === "" ? [] : [[key, Number(value)]],
		),
	);
	const input: ScannerIndicatorInput = {
		intervals,
		parameters,
		type: type.type,
	};
	if (type.overlay) return input;
	const scale = indicatorScale(draft);
	return scale ? { ...input, scale } : undefined;
}

// Moves one indicator id, as a drag and drop does, and returns the new order.
export function moveIndicator(
	ids: readonly number[],
	from: number,
	to: number,
): number[] {
	const result = [...ids];
	const [moved] = result.splice(from, 1);
	if (moved !== undefined) result.splice(to, 0, moved);
	return result;
}

// Orders indicators by ids; indicators missing from ids keep their place at
// the end.
export function orderIndicators(
	indicators: readonly ScannerIndicator[],
	ids: readonly number[] | undefined,
): ScannerIndicator[] {
	if (!ids) return [...indicators];
	const positions = new Map(ids.map((id, index) => [id, index]));
	return [...indicators].sort(
		(left, right) =>
			(positions.get(left.id) ?? ids.length) -
			(positions.get(right.id) ?? ids.length),
	);
}

export function formatParameters(parameters: Record<string, unknown>): string {
	const entries = Object.entries(parameters);
	return entries.length === 0
		? "No parameters"
		: entries.map(([key, value]) => `${key} ${String(value)}`).join(", ");
}

export function formatScale(scale: ScannerIndicatorScale): string {
	const range =
		scale.min === undefined && scale.max === undefined
			? "auto"
			: `${scale.min ?? "auto"}…${scale.max ?? "auto"}`;
	return scale.levels.length > 0
		? `${range}, levels ${scale.levels.join(" / ")}`
		: range;
}

// Validation and conflict messages come from the backend.
export function mutationErrorMessage(
	error: ErrorType<ErrorResponse> | null,
): string {
	return describeApiError(error, {
		fallback: "The indicator could not be saved.",
		forbidden: "Only the scanner administrator can change indicators.",
		messages: {
			scanner_indicator_in_use:
				"A strategy uses this indicator. Change or delete the strategy first.",
			scanner_indicator_not_found: "This indicator no longer exists.",
		},
		server: [
			"invalid_argument",
			"scanner_indicator_exists",
			"scanner_indicator_limit",
		],
	});
}
