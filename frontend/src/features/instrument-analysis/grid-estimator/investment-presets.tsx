import { Button, Group, Text } from "@mantine/core";
import { INVESTMENT_PRESETS } from "@/features/instrument-analysis/grid-estimator/config";
import type { GridMarket } from "@/features/instrument-analysis/grid-estimator/types";
import { investmentFromUsdt } from "@/features/instrument-analysis/grid-estimator/utils";
import { formatNumber } from "@/utils/number-format";

interface InvestmentPresetsProps {
	anchor: number | null;
	disabled: boolean;
	market: GridMarket;
	onSelect: (investment: string) => void;
}

// Investment buttons in USDT, converted into the market's margin asset.
export function InvestmentPresets({
	anchor,
	disabled,
	market,
	onSelect,
}: InvestmentPresetsProps) {
	return (
		<Group gap="xs" justify="flex-end">
			<Text fw={500} mr="auto" size="sm">
				Investment
			</Text>
			{INVESTMENT_PRESETS.map((usdt) => {
				const investment = investmentFromUsdt(market, anchor, usdt);
				return (
					<Button
						disabled={disabled || investment === null}
						key={usdt}
						onClick={(event) => {
							if (investment !== null) onSelect(investment);
							event.currentTarget.blur();
						}}
						size="xs"
						type="button"
						variant="default"
					>
						{formatNumber(usdt)}
					</Button>
				);
			})}
		</Group>
	);
}
