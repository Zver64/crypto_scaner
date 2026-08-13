export const readinessQueryKey = ["backend-readiness"] as const;

export async function fetchReadiness(request: typeof fetch = fetch) {
	const response = await request("/health/ready", {
		headers: { Accept: "application/json" },
		method: "GET",
	});

	return response.ok;
}
