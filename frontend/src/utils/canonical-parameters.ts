// Serializes indicator parameters independently of key order, so selections
// can be compared with the parameters the backend echoes back.
export function canonicalParameters(
	parameters: Record<string, unknown>,
): string {
	return JSON.stringify(parameters, Object.keys(parameters).sort());
}
