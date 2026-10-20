// The last characters of a long id, enough to tell ids apart at a glance.
export function shortId(id: string): string {
	return id.slice(-6);
}
