import { useCanGoBack, useNavigate, useRouter } from "@tanstack/react-router";

// Leaves the scanner settings for the previous page, or the Market Scan when
// the settings were opened directly.
export function useCloseScannerSettings(): () => void {
	const router = useRouter();
	const navigate = useNavigate();
	const canGoBack = useCanGoBack();
	return () => {
		if (canGoBack) {
			router.history.back();
		} else {
			void navigate({ to: "/" });
		}
	};
}
