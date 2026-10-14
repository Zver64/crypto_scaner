// The history load form, in the shape its inputs emit: depth stays a string
// while its input is empty, and intervals keep the order they were chosen in.
export interface HistoryLoadValues {
	coins: string[];
	depth: number | string;
	intervals: string[];
}
