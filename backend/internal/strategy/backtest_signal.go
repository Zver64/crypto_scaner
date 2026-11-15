package strategy

import (
	"fmt"
	"slices"
	"time"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

// signalATRPeriod is the common ATR period used to judge every signal.
const signalATRPeriod = 14

func (stats *SignalStats) record(evaluation *SignalEvaluation) {
	stats.Count++
	switch evaluation.Outcome {
	case SignalSuccess:
		stats.Successes++
	case SignalNeutral:
		stats.Neutrals++
	}
}

// judgeSignal judges the occurrences of signal, at the indexes signals of
// history, candles of interval, setting their evaluations, and every
// evaluated candle, from first through last (none when first is negative).
func (replay *replay) judgeSignal(signal Strategy, interval market.CandleInterval, history []market.Candle, occurrences []SignalOccurrence, signals []int, first, last int) (*SignalJudgement, error) {
	// The ATR at a candle is calculated over its Depth window, the warm-up
	// policy of the replayed values, so it does not depend on how much
	// history is stored before that window.
	target := closedindicator.Target{Interval: interval, Selection: indicator.Selection{Type: "atr", Parameters: indicator.Parameters{"period": signalATRPeriod}}}
	depth, err := closedindicator.Depth(replay.registry, target, 1)
	if err != nil {
		return nil, fmt.Errorf("signal ATR warm-up: %w", err)
	}
	// The replayed values' warm-up applies to both statistics alike.
	evaluate := func(at int) (*SignalEvaluation, error) {
		if replay.cutOff(interval, len(history), at+1, depth) {
			return nil, nil
		}
		value, err := closedindicator.Calculate(replay.registry, target, history[max(0, at+1-depth):at+1], 1)
		if err != nil {
			return nil, fmt.Errorf("calculate signal ATR: %w", err)
		}
		var atr float64
		if len(value.Outputs) > 0 {
			atr = value.Outputs[0].Value
		}
		return evaluateSignal(signal.Direction, interval, history, at, signal.Window, signal.TargetRatio, atr), nil
	}
	report := &SignalJudgement{Window: signal.Window, TargetRatio: signal.TargetRatio}
	var moves []float64
	next := 0
	for index, at := range signals {
		evaluation, err := evaluate(at)
		if err != nil {
			return nil, err
		}
		if evaluation == nil {
			continue
		}
		report.Evaluated++
		if at >= next {
			evaluation.Counted = true
			next = at + signal.Window
			report.Signals.record(evaluation)
			moves = append(moves, evaluation.Move)
		}
		occurrences[index].Evaluation = evaluation
	}
	report.Signals.MedianMove = median(moves)
	if first < 0 {
		return report, nil
	}
	moves = nil
	for at := first; at <= last; at++ {
		evaluation, err := evaluate(at)
		if err != nil {
			return nil, err
		}
		if evaluation != nil {
			report.All.record(evaluation)
			moves = append(moves, evaluation.Move)
		}
	}
	report.All.MedianMove = median(moves)
	return report, nil
}

// evaluateSignal judges the close of the candle of history, candles of
// interval, at at by a target ratio stops away in direction over the window
// candles after it. It is nil without a positive ATR or the consecutive future
// window, or when a level would not be a positive price.
func evaluateSignal(direction Direction, interval market.CandleInterval, history []market.Candle, at, window, ratio int, atr float64) *SignalEvaluation {
	if at < signalATRPeriod || at+window >= len(history) ||
		!consecutive(interval, history[at:at+window+1]) {
		return nil
	}
	price := history[at].Close
	stop := atr / price
	target := float64(ratio) * stop
	if !(price > 0) || !(stop > 0 && stop < 1) || (direction != DirectionLong && target >= 1) {
		return nil
	}
	future := history[at+1 : at+window+1]
	if direction == DirectionSideways {
		return sidewaysEvaluation(price, target, future)
	}
	evaluation := &SignalEvaluation{Stop: new(stop), Target: target}
	evaluation.Outcome, evaluation.Move, evaluation.ExitTime = directionalOutcome(direction, price, stop, target, future)
	return evaluation
}

// directionalOutcome reports whether the price, from price, reached the
// target or stop over future, with the final close deciding untouched windows.
// It also reports the largest move in direction until
// the stop. A candle reaching both decides by its open when that is already
// at one of them, and fails otherwise; the stop candle's move counts only its
// open, since its high and low may come after the stop.
func directionalOutcome(direction Direction, price, stop, target float64, future []market.Candle) (SignalOutcome, float64, time.Time) {
	sign := 1.0
	if direction == DirectionShort {
		sign = -1
	}
	// favorable is the move to value in direction, as a fraction of price.
	favorable := func(value float64) float64 { return sign * (value - price) / price }
	decided, outcome, move := false, SignalFailure, 0.0
	exitTime := future[len(future)-1].OpenTime.UTC()
	for _, candle := range future {
		best, worst := candle.High, candle.Low
		if direction == DirectionShort {
			best, worst = candle.Low, candle.High
		}
		openAtTarget, openAtStop := favorable(candle.Open) >= target, favorable(candle.Open) <= -stop
		reachesTarget, reachesStop := favorable(best) >= target, favorable(worst) <= -stop
		if !decided {
			switch {
			case openAtTarget:
				decided, outcome = true, SignalSuccess
			case openAtStop, reachesTarget && reachesStop:
				decided = true
			case reachesTarget:
				decided, outcome = true, SignalSuccess
			case reachesStop:
				decided = true
			}
			if decided {
				exitTime = candle.OpenTime.UTC()
			}
		}
		if reachesStop || openAtStop {
			if !openAtStop {
				move = max(move, favorable(candle.Open))
			}
			break
		}
		move = max(move, favorable(best))
	}
	if !decided {
		finalClose := future[len(future)-1].Close
		oneATR := price + sign*price*stop
		switch {
		case sign*(finalClose-oneATR) >= 0:
			outcome = SignalSuccess
		case favorable(finalClose) >= 0:
			outcome = SignalNeutral
		}
	}
	return outcome, move, exitTime
}

// sidewaysEvaluation judges a sideways signal at price with targets target
// times price above and below it over future: it succeeds when no candle
// touches either, and its move is the largest deviation either way.
func sidewaysEvaluation(price, target float64, future []market.Candle) *SignalEvaluation {
	evaluation := &SignalEvaluation{Target: target, Outcome: SignalSuccess, ExitTime: future[len(future)-1].OpenTime.UTC()}
	for _, candle := range future {
		up, down := (candle.High-price)/price, (price-candle.Low)/price
		if (up >= target || down >= target) && evaluation.Outcome != SignalFailure {
			evaluation.ExitTime = candle.OpenTime.UTC()
			evaluation.Outcome = SignalFailure
		}
		evaluation.Move = max(evaluation.Move, up, down)
	}
	return evaluation
}

// consecutive reports whether every candle of candles, of interval, opens one
// interval after the one before.
func consecutive(interval market.CandleInterval, candles []market.Candle) bool {
	for index := 1; index < len(candles); index++ {
		if !interval.NextOpenTime(candles[index-1].OpenTime).Equal(candles[index].OpenTime) {
			return false
		}
	}
	return true
}

// median is the median of values, nil without values; it sorts values.
func median(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	slices.Sort(values)
	middle := len(values) / 2
	if len(values)%2 == 0 {
		return new((values[middle-1] + values[middle]) / 2)
	}
	return new(values[middle])
}
