import { useMatches } from "@mantine/core";

// Responsive spacing and text size shared by the coin page blocks.
export function useCoinPageLayout() {
	return {
		contentSpacing: useMatches({ base: "sm", sm: "md" }),
		paperPadding: useMatches({ base: "xs", sm: "md" }),
		textSize: useMatches({ base: "sm", sm: "md" }),
	};
}
