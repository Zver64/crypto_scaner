export function assertPositiveFiniteNumber(value: number, name: string): void {
	if (!Number.isFinite(value) || value <= 0) {
		throw new RangeError(`${name} must be a positive finite number`);
	}
}

export function assertMaintenanceMarginRatio(value: number): void {
	if (!Number.isFinite(value) || value < 0 || value >= 1) {
		throw new RangeError(
			"Maintenance-margin ratio must be a finite number from 0 up to 1",
		);
	}
}
