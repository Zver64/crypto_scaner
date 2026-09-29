import type { UnresolvedInstrument } from "@/api/generated/models";
import type { DataTableColumn } from "@/components/data-table";
import { marketCapUnavailableReason } from "@/features/market-scan/results-table/utils";

export const unresolvedInstrumentColumns: readonly DataTableColumn<UnresolvedInstrument>[] =
	[
		{
			key: "symbol",
			header: "Symbol",
			cell: (item) => item.symbol,
		},
		{
			key: "reason",
			header: "Reason",
			cell: (item) => marketCapUnavailableReason(item.code),
		},
	];
