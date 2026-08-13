const utcDateFormatter = new Intl.DateTimeFormat("en", {
	dateStyle: "medium",
	timeZone: "UTC",
});

export function formatUtcCoverageDate(value: string): string {
	return `${utcDateFormatter.format(new Date(value))} UTC`;
}
