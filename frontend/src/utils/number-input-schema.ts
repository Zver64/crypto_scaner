import { z } from "zod";

// What a number input holds: a number, or a string while it is empty, being
// typed, or keeping trailing zeros, such as "1.", "1.50", or a large integer.
// Parses to a number that passes `checks`; an empty input is required, any
// other text is not a number.
export function numberInputSchema(
	label: string,
	checks: (number: z.ZodNumber) => z.ZodNumber = (number) => number,
) {
	return z
		.union([z.number(), z.string()], {
			error: `${label} must be a number`,
		})
		.transform((value) =>
			typeof value === "string" && value.trim() !== "" ? Number(value) : value,
		)
		.pipe(
			checks(
				z.number({
					error: (issue) =>
						issue.input === ""
							? `${label} is required`
							: `${label} must be a number`,
				}),
			),
		);
}
