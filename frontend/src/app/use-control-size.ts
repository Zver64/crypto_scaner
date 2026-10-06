import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";

// The size of form controls: large touch targets on phones and tablets,
// compact controls on wide screens.
export function useControlSize() {
	return useWideLayout() ? "sm" : "md";
}
