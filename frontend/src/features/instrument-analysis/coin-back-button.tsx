import { Button } from "@mantine/core";
import { useTelegramBackButton } from "@/app/telegram";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";

interface CoinBackButtonProps {
	onBack(): void;
}

// Uses Telegram's native back button when available and renders one otherwise.
export function CoinBackButton({ onBack }: CoinBackButtonProps) {
	const { textSize } = useCoinPageLayout();
	const hasNativeBackButton = useTelegramBackButton(onBack);
	return hasNativeBackButton ? null : (
		<Button onClick={onBack} size={textSize} variant="subtle">
			Back to Market Scan
		</Button>
	);
}
