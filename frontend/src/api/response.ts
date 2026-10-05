/** Reads JSON responses while preserving HTTP errors with non-JSON bodies. */
export async function readResponseData(response: Response): Promise<unknown> {
	const body = [204, 205, 304].includes(response.status)
		? ""
		: await response.text();
	let data: unknown = {};
	if (body) {
		try {
			data = JSON.parse(body);
		} catch (error) {
			if (response.ok) throw error;
			data = undefined;
		}
	}
	if (!response.ok) {
		throw Object.assign(new Error(`HTTP ${response.status}`), {
			info: data,
			status: response.status,
		});
	}
	return data;
}
