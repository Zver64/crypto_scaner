import { z } from "zod";

// What a number input holds: a number, or a string while it is empty or being
// typed. Parses to a number that passes `checks`; an empty input is required,
// any other string is not a number.
export function numberInputSchema(
	label: string,
	checks: (number: z.ZodNumber) => z.ZodNumber = (number) => number,
) {
	return z
		.union([z.number(), z.string()], {
			error: `${label} must be a number`,
		})
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
