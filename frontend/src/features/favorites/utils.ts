import type { InstrumentUse } from "@/api/generated/models";

// The message of an instrument_used_by_strategy conflict: the administrator
// removes a favorite that strategies read. Other errors have none.
export function instrumentUsesMessage(error: unknown): string | undefined {
	const info = (
		error as { info?: { error?: { code?: unknown; details?: unknown } } }
	).info?.error;
	if (info?.code !== "instrument_used_by_strategy") return undefined;
	const uses = (info.details as { uses?: InstrumentUse[] } | undefined)?.uses;
	if (!uses?.length) {
		return "Strategies read this coin. Delete or change those strategies first.";
	}
	const coins = uses
		.map(({ strategies, symbol }) => `${symbol} (${strategies.join(", ")})`)
		.join("; ");
	return `Strategies read ${coins}. Delete or change those strategies first.`;
}
