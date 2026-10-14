import equal from "fast-deep-equal";

// How next differs from the data a series shows: not at all, only in its last
// item (changed or appended), so update() can apply it without redrawing the
// whole series, or otherwise. Items are compared by value, one pass over the
// data; a series compares only when it receives a new data array.
export function dataChange<T extends object>(
	previous: readonly T[],
	next: readonly T[],
): "none" | "tail" | "all" {
	if (previous === next) return "none";
	if (previous.length === 0) return next.length === 0 ? "none" : "all";
	if (next.length !== previous.length && next.length !== previous.length + 1) {
		return "all";
	}
	for (let index = 0; index < next.length - 1; index++) {
		if (!equal(previous[index], next[index])) return "all";
	}
	return next.length === previous.length && equal(previous.at(-1), next.at(-1))
		? "none"
		: "tail";
}
