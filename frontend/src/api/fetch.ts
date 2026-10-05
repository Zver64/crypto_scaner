import { readResponseData } from "@/api/response";
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
	const data = await readResponseData(response);
	return { data, headers: response.headers, status: response.status } as T;
}

/** Error type of generated queries: the response body and status of a rejection. */
export type ErrorType<ErrorBody> = globalThis.Error & {
	info?: ErrorBody;
	status?: number;
};
