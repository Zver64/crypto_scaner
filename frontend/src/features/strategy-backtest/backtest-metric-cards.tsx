import { SimpleGrid } from "@mantine/core";
import type { StrategyBacktest } from "@/api/generated/models";
import { BacktestReturn } from "@/features/strategy-backtest/backtest-return";
import {
	buyAndHoldHint,
	dcaHint,
} from "@/features/strategy-backtest/constants";
import { HintLabel } from "@/features/strategy-backtest/hint-label";
import { MetricCard } from "@/features/strategy-backtest/metric-card";
import {
	formatFractionPercent,
	formatProfitFactor,
} from "@/features/strategy-backtest/utils";
import { formatNumber } from "@/utils/number-format";

interface BacktestMetricCardsProps {
	backtest: StrategyBacktest;
	gap: string;
	paperPadding: string;
}

// The headline metrics of the strategy, compared with a baseline where one
// applies, like TradingView's Strategy Report.
export function BacktestMetricCards({
	backtest: { baselines, fee, skipped_alerts, summary },
	gap,
	paperPadding,
}: BacktestMetricCardsProps) {
	const { stats } = summary;
	return (
		<SimpleGrid cols={{ base: 2, sm: 3, lg: 5 }} spacing={gap}>
			<MetricCard
				comparison={
					<>
						<HintLabel hint={buyAndHoldHint}>Buy & Hold</HintLabel>{" "}
						<BacktestReturn value={baselines.buy_and_hold} />{" "}
						<HintLabel hint={dcaHint}>DCA</HintLabel>{" "}
						<BacktestReturn value={baselines.dca} />
					</>
				}
				hint={`Compounded result of all trades, an open one valued at the last close, after a ${formatFractionPercent(fee)} fee on each buy and sell. Every buy spends the same amount.`}
				label="Net profit"
				paperPadding={paperPadding}
				value={<BacktestReturn value={summary.net_profit} />}
			/>
			<MetricCard
				hint={`Closed trades, one at a time; an open trade counts only in Net profit. Entry signals that bought nothing, without accumulation or past max buys: ${formatNumber(skipped_alerts)}.`}
				label="Trades"
				paperPadding={paperPadding}
				value={formatNumber(stats.trade_count)}
			/>
			<MetricCard
				hint="Share of trades closed in profit."
				label="Win rate"
				paperPadding={paperPadding}
				value={formatFractionPercent(stats.win_rate)}
			/>
			<MetricCard
				hint="Sum of profits divided by sum of losses; empty when there are no losses."
				label="Profit factor"
				paperPadding={paperPadding}
				value={formatProfitFactor(stats.profit_factor)}
			/>
			<MetricCard
				hint="Largest drop of equity from its peak."
				label="Max drawdown"
				paperPadding={paperPadding}
				value={formatFractionPercent(summary.max_drawdown)}
			/>
		</SimpleGrid>
	);
}
