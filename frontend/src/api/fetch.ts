import { fetchWithSession } from "@/api/session";

/**
 * Orval mutator for every generated operation: it authenticates with the
 * session and returns or throws the same shapes as the generated fetch client.
 */
export async function apiFetch<T>(
	url: string,
	options: RequestInit,
): Promise<T> {
	const response = await fetchWithSession(url, options);
	const body = [204, 205, 304].includes(response.status)
		? null
		: await response.text();
	const data: unknown = body ? JSON.parse(body) : {};
	if (!response.ok) {
		const error: ErrorType<unknown> = new globalThis.Error();
		error.info = data;
		error.status = response.status;
		throw error;
	}
	return { data, headers: response.headers, status: response.status } as T;
}

/** Error type of generated queries: the response body and status of a rejection. */
export type ErrorType<ErrorBody> = globalThis.Error & {
	info?: ErrorBody;
	status?: number;
};
