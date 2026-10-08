import dayjs, { type ManipulateType } from "dayjs";
import utc from "dayjs/plugin/utc";

dayjs.extend(utc);

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

// A timestamp in UTC in a dayjs format, such as the format of a date
// picker plus the time.
export function formatUtcDateTime(value: string, format: string): string {
	return `${dayjs.utc(value).format(format)} UTC`;
}

// The time in milliseconds of a timestamp, or a UTC day such as
// 2026-01-31 at its start, moved by amount units in UTC.
export function shiftedUtcTime(
	value: string,
	amount: number,
	unit: ManipulateType,
): number {
	return dayjs.utc(value).add(amount, unit).valueOf();
}
