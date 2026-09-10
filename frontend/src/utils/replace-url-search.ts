type SearchValue = number | string | undefined;

export function replaceUrlSearch(updates: Record<string, SearchValue>) {
	const url = new URL(window.location.href);
	for (const [key, value] of Object.entries(updates)) {
		if (value === undefined) {
			url.searchParams.delete(key);
		} else {
			url.searchParams.set(key, String(value));
		}
	}
	window.history.replaceState(
		window.history.state,
		"",
		`${url.pathname}${url.search}${url.hash}`,
	);
}
