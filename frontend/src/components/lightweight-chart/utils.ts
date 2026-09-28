// Whether next only changes or appends its last item relative to previous, so
// a series can apply it with update() instead of replacing all data.
export function isTailChange<T extends object>(
	previous: readonly T[],
	next: readonly T[],
): boolean {
	if (
		previous.length === 0 ||
		(next.length !== previous.length && next.length !== previous.length + 1)
	) {
		return false;
	}
	for (let index = 0; index < next.length - 1; index++) {
		const before = previous[index];
		const after = next[index];
		if (!before || !after || !shallowEqual(before, after)) return false;
	}
	return true;
}

function shallowEqual(left: object, right: object): boolean {
	const leftEntries = Object.entries(left);
	return (
		leftEntries.length === Object.keys(right).length &&
		leftEntries.every(
			([key, value]) => (right as Record<string, unknown>)[key] === value,
		)
	);
}
