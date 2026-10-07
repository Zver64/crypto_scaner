import { SimpleGrid } from "@mantine/core";
import type { StrategyBacktest } from "@/api/generated/models";
import { BacktestReturn } from "@/features/strategy-backtest/backtest-return";
import {
	buyAndHoldHint,
	everyCandleHint,
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
	backtest: {
		baselines,
		fee,
		hold,
		interval,
		skipped_alerts,
		summary,
		unfinished_trades,
	},
	gap,
	paperPadding,
}: BacktestMetricCardsProps) {
	const { stats } = summary;
	const everyCandle = baselines.every_candle;
	const everyCandleLabel = (
		<HintLabel hint={everyCandleHint}>Every candle</HintLabel>
	);
	return (
		<SimpleGrid cols={{ base: 2, sm: 3, lg: 5 }} spacing={gap}>
			<MetricCard
				comparison={
					<>
						<HintLabel hint={buyAndHoldHint}>Buy & Hold</HintLabel>{" "}
						<BacktestReturn value={baselines.buy_and_hold} />
					</>
				}
				hint={`Compounded result of all closed trades after a ${formatFractionPercent(fee)} fee on each buy and sell.`}
				label="Net profit"
				paperPadding={paperPadding}
				value={<BacktestReturn value={summary.net_profit} />}
			/>
			<MetricCard
				comparison={
					<>
						{everyCandleLabel} {formatNumber(everyCandle.trade_count)}
					</>
				}
				hint={`Closed trades, each held ${formatNumber(hold)} candles (${interval}), one at a time. Alerts during an open trade are skipped: ${formatNumber(skipped_alerts)}. Unfinished trades are not counted: ${formatNumber(unfinished_trades)}.`}
				label="Trades"
				paperPadding={paperPadding}
				value={formatNumber(stats.trade_count)}
			/>
			<MetricCard
				comparison={
					<>
						{everyCandleLabel} {formatFractionPercent(everyCandle.win_rate)}
					</>
				}
				hint="Share of trades closed in profit."
				label="Win rate"
				paperPadding={paperPadding}
				value={formatFractionPercent(stats.win_rate)}
			/>
			<MetricCard
				comparison={
					<>
						{everyCandleLabel} {formatProfitFactor(everyCandle.profit_factor)}
					</>
				}
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
