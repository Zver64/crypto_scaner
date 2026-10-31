// Telegram passes the start parameter of a link, the coin of a bot message,
// to the Mini App it opens as this search parameter.
export interface LaunchSearch {
	tgWebAppStartParam?: string;
}

export function parseLaunchSearch(
	search: Record<string, unknown>,
): LaunchSearch {
	const symbol = search.tgWebAppStartParam;
	return typeof symbol === "string" && /^[A-Z0-9]+$/.test(symbol)
		? { tgWebAppStartParam: symbol }
		: {};
}
