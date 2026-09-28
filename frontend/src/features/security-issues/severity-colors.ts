import type { SecurityIssueSeverity } from "@/api/generated/models";

// Translucent tints of table rows whose coin has failed audit checks.
export const securitySeverityColors: Record<SecurityIssueSeverity, string> = {
	risk: "var(--mantine-color-red-light)",
	caution: "var(--mantine-color-yellow-light)",
};
