import type {
	IndicatorType,
	ScannerIndicator,
	ScannerIndicatorInput,
	ScannerIndicatorScale,
} from "@/api/generated/models";
import type {
	ParameterValues,
	ScannerIndicatorDraft,
} from "@/features/scanner-settings/types";

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

export function tableColumnAllowed(type: IndicatorType | undefined): boolean {
	return type?.outputs.length === 1;
}

export function parseLevels(levels: readonly string[]): number[] | undefined {
	const values = levels.map((level) => Number(level.trim()));
	return values.every(Number.isFinite) ? values : undefined;
}

// Builds the create request; returns undefined while a level is not a number.
export function scannerIndicatorInput(
	draft: ScannerIndicatorDraft,
	type: IndicatorType,
): ScannerIndicatorInput | undefined {
	const parameters = Object.fromEntries(
		Object.entries(draft.parameters).flatMap(([key, value]) =>
			value === "" ? [] : [[key, Number(value)]],
		),
	);
	const input: ScannerIndicatorInput = {
		interval: draft.interval,
		parameters,
		show_in_table: draft.showInTable && tableColumnAllowed(type),
		type: type.type,
	};
	if (type.overlay) return input;
	const levels = parseLevels(draft.levels);
	if (!levels) return undefined;
	const scale: ScannerIndicatorScale = { levels };
	if (draft.scaleMin !== "") scale.min = Number(draft.scaleMin);
	if (draft.scaleMax !== "") scale.max = Number(draft.scaleMax);
	return { ...input, scale };
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
export function mutationErrorMessage(error: unknown): string {
	const info = (
		error as { info?: { error?: { code?: unknown; message?: unknown } } }
	).info?.error;
	switch (info?.code) {
		case "invalid_argument":
		case "scanner_indicator_exists":
		case "scanner_indicator_limit":
			return typeof info.message === "string"
				? info.message
				: "The indicator could not be saved.";
		case "scanner_indicator_not_found":
			return "This indicator no longer exists.";
		case "administrator_required":
		case "access_denied":
			return "Only the scanner administrator can change indicators.";
		case "unauthenticated":
			return "Telegram authorization has expired. Reopen the Mini App.";
		default:
			return "The indicator could not be saved.";
	}
}
