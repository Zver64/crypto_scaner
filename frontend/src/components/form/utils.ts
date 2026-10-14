// The first error of a field as Mantine shows it; validators return strings.
export function fieldError(errors: readonly unknown[]): string | undefined {
	const [error] = errors;
	return typeof error === "string" ? error : undefined;
}
