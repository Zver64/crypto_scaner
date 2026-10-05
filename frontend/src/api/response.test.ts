import { expect, it } from "vitest";
import { readResponseData } from "@/api/response";

it("preserves the status without info for a non-JSON error", async () => {
	await expect(
		readResponseData(new Response("<html>Bad gateway</html>", { status: 502 })),
	).rejects.toMatchObject({ status: 502, info: undefined });
});

it("preserves a JSON error body and status", async () => {
	const info = { error: { code: "access_denied" } };
	await expect(
		readResponseData(Response.json(info, { status: 403 })),
	).rejects.toMatchObject({ status: 403, info });
});

it("returns an empty object for a 204 response", async () => {
	await expect(
		readResponseData(new Response(null, { status: 204 })),
	).resolves.toEqual({});
});

it("still rejects invalid JSON in a successful response", async () => {
	await expect(
		readResponseData(new Response("not JSON", { status: 200 })),
	).rejects.toBeInstanceOf(SyntaxError);
});
