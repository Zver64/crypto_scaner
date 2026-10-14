import { describe, expect, it } from "vitest";

import { dataChange } from "@/components/lightweight-chart/utils";

const point = (time: number, value: number) => ({ time, value });

describe("dataChange", () => {
	const applied = [point(1, 10), point(2, 20)];

	it("skips equal data in a new array", () => {
		expect(dataChange(applied, [point(1, 10), point(2, 20)])).toBe("none");
		expect(dataChange([], [])).toBe("none");
	});

	it("updates only a changed or appended last point", () => {
		expect(dataChange(applied, [point(1, 10), point(2, 21)])).toBe("tail");
		expect(dataChange(applied, [...applied, point(3, 30)])).toBe("tail");
	});

	it("replaces data changed before the last point, prepended or shortened", () => {
		expect(dataChange(applied, [point(1, 11), point(2, 20)])).toBe("all");
		expect(dataChange(applied, [point(0, 0), ...applied])).toBe("all");
		expect(dataChange(applied, [point(1, 10)])).toBe("all");
		expect(dataChange([], applied)).toBe("all");
	});
});
