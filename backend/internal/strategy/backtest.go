package strategy

import (
	"context"
	"fmt"
	"time"

	"crypto-scanner/internal/market"

	"github.com/google/uuid"
)

// BacktestFee is the fee of each side of a simulated trade, as a fraction of
// the traded value.
const BacktestFee = 0.001

// Backtest is how a strategy would have traded one instrument over its
// stored closed history: the monitor's trades replayed candle by candle.
// Every buy spends one quote unit at the open of the next processed candle
// after its signal. A take profit or stop loss sells every buy of the trade
// on the candle that reaches it, and an exit rule at the open of the next
// processed candle after its signal, paying BacktestFee on both sides.
type Backtest struct {
	// Interval is the finest interval the rules read, where signals fire
	// and trades count candles.
	Interval market.CandleInterval
	Symbol   string
	// Direction is the strategy's: how it trades or what a signal expects.
	Direction Direction
	// From and To are the open times of the first and the last evaluated
	// candles; both are zero when none is evaluated.
	From, To time.Time
	// Alerts are the open times of the candles whose close signaled a buy,
	// oldest first.
	Alerts []time.Time
	// Trades are the trades, oldest first; the last one is open when the
	// history ends before its sell. Skipped counts entry signals that bought
	// nothing.
	Trades  []Trade
	Skipped int
	// Stats describes the closed trades. NetProfit, Equity, and MaxDrawdown
	// include the open trade, valued at the last close: the equity compounds
	// from 1 through the trades, and the drawdown is its largest fall from an
	// earlier peak, as a fraction of that peak, at the close of every
	// evaluated candle, with an open trade valued there.
	Stats                  TradeStats
	NetProfit, MaxDrawdown float64
	Equity                 []EquityPoint
	// BuyAndHold is the net return from the open of the candle after the
	// first evaluated candle to the close of the last; DCA the net return of
	// buying one quote unit at the open after every evaluated candle but the
	// last, valued at the close of the last. Both are nil without such
	// candles, for a signal, and for a short strategy.
	BuyAndHold, DCA *float64
	// Signal describes the entry signals of a signal, which trades nothing;
	// nil for a trading strategy.
	Signal           *SignalReport
	IndicatorColumns []IndicatorColumn
	Charts           []BacktestChart
}

// SignalReport lists the entry signals of a signal, oldest first, beside
// Candles, the number of candles the entry was evaluated on, and, unless the signal is simple, judges
// them: Judgement is nil for a simple signal, which expects no particular
// move.
type SignalReport struct {
	Occurrences []SignalOccurrence
	Candles     int
	Judgement   *SignalJudgement
}

// SignalJudgement judges the entry signals of a signal by a target and a stop
// over the Window candles after each, beside the same judgment after every
// evaluated candle. A candle's stop lies one ATR(14) away from its close,
// against the expected move, independent of Window. The target lies TargetRatio
// stops away in the expected direction; a sideways signal has targets on both sides and
// no stop. Outcomes may read stored candles after the evaluated period.
type SignalJudgement struct {
	Window, TargetRatio int
	// Evaluated counts the entry signals with an evaluation; Signals
	// describes the counted ones and All every evaluated candle with an
	// evaluation.
	Evaluated    int
	Signals, All SignalStats
}

// SignalOccurrence is an entry signal: Time is the open time of the candle
// whose close signaled it, Close that close, and Values what the entry rule
// read there. Evaluation is nil for a simple signal and when the stored
// history lacks the candles to judge it.
type SignalOccurrence struct {
	Time            time.Time
	Close           float64
	Values          map[string]float64
	IndicatorValues map[string]float64
	Evaluation      *SignalEvaluation
}

// SignalEvaluation judges a candle's close by its target and stop. Stop and
// Target are their distances from the close as fractions of it, Stop nil for
// sideways. Long/short use the first touched level, or the final close when
// neither level is touched: at least one ATR forward succeeds, a smaller
// nonnegative move is neutral, and a backward move fails. Sideways succeeds
// when neither target is touched. Move is the
// largest move from the close in the expected direction until the stop is
// reached or the window ends, and for sideways the largest move either way.
// Counted tells a signal counted in the statistics: one at least Window
// candles after the previous counted one.
type SignalEvaluation struct {
	// ExitTime is the open time of the first decisive candle, or the window's last candle.
	ExitTime     time.Time
	Counted      bool
	Stop         *float64
	Target, Move float64
	Outcome      SignalOutcome
}

// SignalOutcome is the result of a fully evaluated signal window.
type SignalOutcome string

const (
	SignalSuccess SignalOutcome = "success"
	SignalFailure SignalOutcome = "failure"
	SignalNeutral SignalOutcome = "neutral"
)

// SignalStats counts all evaluated outcomes, including neutrals. MedianMove
// includes every counted outcome and is nil without candles.
type SignalStats struct {
	Count, Successes, Neutrals int
	MedianMove                 *float64
}

// SuccessRate excludes neutral outcomes; nil means no decisive outcomes.
func (stats SignalStats) SuccessRate() *float64 {
	decisive := stats.Count - stats.Neutrals
	if decisive == 0 {
		return nil
	}
	return new(float64(stats.Successes) / float64(decisive))
}

// Trade is a trade of one or more buys. EntryTime is the open time of the
// candle its first buy filled at and EntryPrice the average price of its
// buys; ExitTime is the open time of the candle it sold at, or, for an open
// trade, of the last candle, whose close values it. Return is net of fees.
// TakeProfit and StopLoss are the prices fixed at its entry signal, 0
// without them; Reason tells what sold a closed trade, ExitSignal is the open
// time of the candle that signaled the sell, and ExitValues what an exit
// rule read there.
type Trade struct {
	EntryTime, ExitTime   time.Time
	EntryPrice, ExitPrice float64
	Buys                  int
	Open                  bool
	Return                float64
	Fills                 []Fill
	TakeProfit, StopLoss  float64
	Reason                ExitReason
	ExitSignal            time.Time
	ExitValues            map[string]float64
}

// Fill is one buy of a trade: Signal is the open time of the candle whose
// close signaled it, Values what the entry rule read there, and Time and
// Price the open time and the open of the candle it filled at.
type Fill struct {
	Signal, Time time.Time
	Price        float64
	Values       map[string]float64
}

// EquityPoint is the equity after the trade that exited at Time.
type EquityPoint struct {
	Time   time.Time
	Equity float64
}

// TradeStats describes the net returns of trades. WinRate, AverageTrade, and
// AverageBars, the mean count of candles from the one the first buy filled
// at through the one the trade sold at, are nil
// without trades, AverageWin without wins, and AverageLoss and ProfitFactor,
// the sum of winning returns over the sum of losing ones, without losses.
// TakeProfits, StopLosses, and ExitRules count the trades each sold.
type TradeStats struct {
	Count                                                        int
	WinRate, ProfitFactor, AverageTrade, AverageWin, AverageLoss *float64
	AverageBars                                                  *float64
	TakeProfits, StopLosses, ExitRules                           int
}

// maxBacktests bounds the backtests that run at once, saved or drafts; each
// keeps a core busy for up to the server's time limit.
const maxBacktests = 2

// Backtest replays the trades of a saved strategy, enabled or not, on the
// instrument symbol over its stored closed history, the way the monitor
// trades: the same values at the close of every candle of its interval, the
// same freshness, and the same signals, starting without a trade and with a
// false entry. Candles are evaluated once every value is calculated over the
// whole window the tracker loads, so the replay never reads indicators the
// pruned history leaves unsettled. Only candles opening from from to to are
// evaluated, a zero bound leaving that side open; older candles still warm
// the indicators up. It fails with ErrNotFound, ErrInvalidArgument for a
// strategy that no longer compiles or reads a coin that is not active
// through of, an empty symbol, or from after to, market.ErrInstrumentNotFound, and
// ErrBacktestBusy while maxBacktests others run.
func (service *Service) Backtest(ctx context.Context, id uuid.UUID, symbol string, from, to time.Time, withChart bool) (Backtest, error) {
	entry, err := service.Get(id)
	if err != nil {
		return Backtest{}, err
	}
	if entry.Compiled == nil {
		return Backtest{}, invalid(entry.Problem)
	}
	return service.backtest(ctx, entry, symbol, from, to, withChart)
}

// BacktestDraft replays item like Backtest replays a saved strategy, without
// saving it: its rules and trading settings are checked and compiled as a
// save checks them. It fails like Backtest, with ErrInvalidArgument for an
// invalid item.
func (service *Service) BacktestDraft(ctx context.Context, item Strategy, symbol string, from, to time.Time, withChart bool) (Backtest, error) {
	entry, err := service.trading(item)
	if err != nil {
		return Backtest{}, err
	}
	return service.backtest(ctx, entry, symbol, from, to, withChart)
}

// backtest replays the compiled entry for Backtest and BacktestDraft.
func (service *Service) backtest(ctx context.Context, entry Entry, symbol string, from, to time.Time, withChart bool) (Backtest, error) {
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return Backtest{}, invalid("the period starts after it ends")
	}
	if symbol = market.NormalizeSymbol(symbol); symbol == "" {
		return Backtest{}, invalid("the symbol is empty")
	}
	select {
	case service.backtests <- struct{}{}:
		defer func() { <-service.backtests }()
	default:
		return Backtest{}, ErrBacktestBusy
	}
	found, err := service.store.GetActiveInstrumentBySymbol(ctx, symbol)
	if err != nil {
		return Backtest{}, fmt.Errorf("resolve backtest instrument %s: %w", symbol, err)
	}
	instrument := Instrument{ID: found.ID, Symbol: found.Symbol}
	active := service.activeInstruments()
	// Values of coins that are not active stay unknown, so nothing would
	// signal.
	if absent := inactive(active, entry.symbols); len(absent) > 0 {
		return Backtest{}, invalid(notActive(absent))
	}
	engine, err := newReplay(service.registry, entry, instrument, active)
	if err != nil {
		return Backtest{}, err
	}
	if err := loadHistories(ctx, service.store, func(market.CandleInterval) int { return market.RetentionDepth }, engine); err != nil {
		return Backtest{}, fmt.Errorf("load backtest history: %w", err)
	}
	displayed, err := service.displayedIndicators(entry)
	if err != nil {
		return Backtest{}, err
	}
	var recording *chartRecording
	var observe func(*replay)
	if withChart {
		if recording, err = service.chartRecording(ctx, engine, entry, instrument.ID, displayed); err != nil {
			return Backtest{}, err
		}
		observe = recording.record
	}
	result := Backtest{Interval: entry.Interval, Symbol: instrument.Symbol, Direction: entry.Direction, IndicatorColumns: indicatorColumns(displayed)}
	if err := simulate(ctx, engine, entry, instrument, from, to, displayed, observe, &result); err != nil {
		return Backtest{}, err
	}
	if withChart {
		if result.Charts, err = recording.pages(ctx, service.charts, engine, entry, result); err != nil {
			return Backtest{}, err
		}
	}
	return result, nil
}
