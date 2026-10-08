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
				hint={`Compounded result of all trades, an open one valued at the last close, after a ${formatFractionPercent(fee)} fee on each buy and sell. Every buy spends the same amount; a strategy with an exit holds one buy per trade.`}
				label="Net profit"
				paperPadding={paperPadding}
				value={<BacktestReturn value={summary.net_profit} />}
			/>
			<MetricCard
				comparison={
					stats.trade_count > 0
						? `TP ${formatNumber(stats.take_profit_exits)} · SL ${formatNumber(stats.stop_loss_exits)} · Rule ${formatNumber(stats.exit_rule_exits)}`
						: undefined
				}
				hint={`Closed trades, one at a time, by what sold them: take profit, stop loss, or exit rule; an open trade counts only in Net profit. Average candles held: ${stats.average_bars === null ? "—" : formatNumber(stats.average_bars, 1)}. Entry signals that bought nothing, during a trade or with the take profit or stop loss on the wrong side of the close: ${formatNumber(skipped_alerts)}.`}
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
				hint="Largest drop of equity from its peak, checked at every candle close with an open trade valued there."
				label="Max drawdown"
				paperPadding={paperPadding}
				value={formatFractionPercent(summary.max_drawdown)}
			/>
		</SimpleGrid>
	);
}
