// Preset buttons labeled with their values.
export function presetOptions(values: readonly number[]) {
	return values.map((value) => ({ label: String(value), value }));
}
