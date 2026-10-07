// Created once; Intl.DateTimeFormat is expensive to build.
const dateTimeFormat = new Intl.DateTimeFormat("en", {
	day: "numeric",
	hour: "2-digit",
	hourCycle: "h23",
	minute: "2-digit",
	month: "short",
	timeZone: "UTC",
	year: "numeric",
});

// A timestamp such as an RFC 3339 API time, in UTC like chart times.
export function formatDateTime(value: string): string {
	return `${dateTimeFormat.format(new Date(value))} UTC`;
}
