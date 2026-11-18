import { useNavigate } from "@tanstack/react-router";

export type ReplaceSearch = (
	updates: Record<string, number | string | undefined>,
) => void;

// Writes search params of the page shown in place: no new history entry, and
// the scroll position and the hash (Telegram launch data) stay.
export function useReplaceSearch(): ReplaceSearch {
	const navigate = useNavigate();
	return (updates) => {
		void navigate({
			hash: true,
			hashScrollIntoView: false,
			replace: true,
			resetScroll: false,
			search: (previous) => ({ ...previous, ...updates }),
			to: ".",
		});
	};
}
