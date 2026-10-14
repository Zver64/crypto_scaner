// The first error of a field as Mantine shows it: validator functions return
// strings, and schemas return issues with a message.
export function fieldError(errors: readonly unknown[]): string | undefined {
	const [error] = errors;
	if (typeof error === "string") return error;
	if (
		typeof error === "object" &&
		error !== null &&
		"message" in error &&
		typeof error.message === "string"
	) {
		return error.message;
	}
	return undefined;
}
