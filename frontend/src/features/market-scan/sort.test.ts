import { describe, expect, it } from "vitest";
import type { MarketTable, TableRow } from "@/api/generated/models";
import {
	nextMarketScanSort,
	resolveTableSort,
	sortTableRows,
} from "@/features/market-scan/sort";

const table: MarketTable = {
	columns: [
		{ id: "symbol", kind: "text", sortable: false, title: "Symbol" },
		{ id: "mcap", kind: "usd_compact", sortable: true, title: "MCap" },
		{ id: "chart", kind: "sparkline", sortable: false, title: "7d chart" },
	],
	default_sort: { column: "mcap", direction: "desc" },
	rows: [],
};

function row(symbol: string, value: number | null): TableRow {
	return { symbol, cells: { mcap: value === null ? {} : { value } } };
}

describe("Market Scan sorting", () => {
	it("starts a new column descending and toggles the active column", () => {
		const current = { column: "mcap", direction: "desc" } as const;
		expect(nextMarketScanSort(current, "range")).toEqual({
			column: "range",
			direction: "desc",
		});
		expect(nextMarketScanSort(current, "mcap")).toEqual({
			column: "mcap",
			direction: "asc",
		});
	});

	it("keeps a requested sortable column and otherwise uses the backend default", () => {
		const ascending = { column: "mcap", direction: "asc" } as const;
		expect(resolveTableSort(table, ascending)).toBe(ascending);
		expect(
			resolveTableSort(table, { column: "chart", direction: "asc" }),
		).toEqual(table.default_sort);
		expect(
			resolveTableSort(table, { column: "missing", direction: "asc" }),
		).toEqual(table.default_sort);
		expect(resolveTableSort(table, undefined)).toEqual(table.default_sort);
	});
});

it.each([
	"asc",
	"desc",
] as const)("sorts unavailable values last (%s), separately from zero, with alphabetical ties", (direction) => {
	const rows = [
		row("B", null),
		row("ZERO", 0),
		{ symbol: "A", cells: {} },
		row("VALUE", 100),
		row("LOSS", -25),
		row("TIE", 100),
	];
	expect(
		sortTableRows(rows, { column: "mcap", direction }).map(
			(item) => item.symbol,
		),
	).toEqual(
		direction === "asc"
			? ["LOSS", "ZERO", "TIE", "VALUE", "A", "B"]
			: ["TIE", "VALUE", "ZERO", "LOSS", "A", "B"],
	);
	expect(rows.map((item) => item.symbol)).toEqual([
		"B",
		"ZERO",
		"A",
		"VALUE",
		"LOSS",
		"TIE",
	]);
});
