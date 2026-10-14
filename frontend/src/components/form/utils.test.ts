import { expect, it } from "vitest";

import { fieldError } from "@/components/form/utils";

it("shows the first error of a validator function or a schema", () => {
	expect(fieldError(["Enter a name", "Too long"])).toBe("Enter a name");
	expect(fieldError([{ message: "Choose a coin", path: [] }])).toBe(
		"Choose a coin",
	);
	expect(fieldError([])).toBeUndefined();
	expect(fieldError([null])).toBeUndefined();
});
