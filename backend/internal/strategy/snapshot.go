package strategy

import (
	"slices"
	"time"

	"crypto-scanner/internal/closedindicator"
)

// snapshot is the tracked values of one evaluation round.
type snapshot struct {
	reads reads
	// values holds the value of each subscription of reads.
	values []closedindicator.Value
	// bySymbol finds the monitored instruments read through of.
	bySymbol map[string]int64
	now      time.Time
}

// match evaluates expression, a rule of entry, over the fresh values of one
// instrument, the instruments it reads through of, and positions.
func (current snapshot) match(expression *Expression, entry Entry, instrumentID int64, positions map[string]float64) (bool, bool, error) {
	return expression.Evaluate(current.resolver(entry, instrumentID, positions))
}

// resolver reads the values of entry on the evaluated instrument, and the
// position variables from positions.
func (current snapshot) resolver(entry Entry, instrumentID int64, positions map[string]float64) func(Read) (float64, bool) {
	return func(read Read) (float64, bool) {
		if read.Variable.Position {
			value, ok := positions[read.Variable.Name]
			return value, ok
		}
		output, ok := current.output(entry, read, instrumentID)
		if !ok {
			return 0, false
		}
		return output.At(read.Shift)
	}
}

// advance processes the latest closed candle of the interval of entry on
// one instrument after state; see Entry.step. outOfRange skips its entry
// signal. Without the fresh candle nothing is processed. A rule error names
// the rule and the candle (ruleError).
func (current snapshot) advance(entry Entry, instrumentID int64, state TradeState, outOfRange bool) (TradeState, []TradeEvent, bool, error) {
	at := entry.Interval.LastClosedOpenTime(current.now)
	latest, ok := current.candle(entry, instrumentID)
	if !ok {
		return state, nil, false, nil
	}
	candle := TradeCandle{
		OpenTime:   at,
		OutOfRange: outOfRange,
		Exit: func(positions map[string]float64) (bool, bool, error) {
			result, known, err := current.match(entry.Exit, entry, instrumentID, positions)
			if err != nil {
				return false, false, ruleError(at, "the exit rule", err)
			}
			return result, known, nil
		},
		Levels: func() (float64, float64, bool, error) {
			takeProfit, knownTakeProfit, err := current.price(entry.TakeProfit, entry, instrumentID)
			if err != nil {
				return 0, 0, false, ruleError(at, "the take profit", err)
			}
			stopLoss, knownStopLoss, err := current.price(entry.StopLoss, entry, instrumentID)
			if err != nil {
				return 0, 0, false, ruleError(at, "the stop loss", err)
			}
			return takeProfit, stopLoss, knownTakeProfit && knownStopLoss, nil
		},
	}
	for _, output := range latest.Outputs {
		switch output.Name {
		case "open":
			candle.Open = output.Value
		case "high":
			candle.High = output.Value
		case "low":
			candle.Low = output.Value
		case "close":
			candle.Close = output.Value
		}
	}
	var err error
	if candle.Entry, candle.EntryKnown, err = current.match(entry.Compiled, entry, instrumentID, nil); err != nil {
		candle.EntryErr = ruleError(at, "the entry rule", err)
	}
	return entry.step(state, candle)
}

// candle returns the latest closed candle of the interval of entry on one
// instrument, when it is fresh.
func (current snapshot) candle(entry Entry, instrumentID int64) (closedindicator.Value, bool) {
	position, ok := current.reads.positions[pair{instrumentID, CandleTarget(entry.Interval).Key()}]
	if !ok || !current.values[position].OpenTime.Equal(entry.Interval.LastClosedOpenTime(current.now)) {
		return closedindicator.Value{}, false
	}
	return current.values[position], true
}

// closePrice returns the close of the latest closed candle of the interval of
// entry on one instrument, when it is fresh.
func (current snapshot) closePrice(entry Entry, instrumentID int64) (float64, bool) {
	latest, ok := current.candle(entry, instrumentID)
	if !ok {
		return 0, false
	}
	for _, output := range latest.Outputs {
		if output.Name == "close" {
			return output.Value, true
		}
	}
	return 0, false
}

// price evaluates the price expression, which may be nil for a price the
// strategy lacks, 0 and known then, on the evaluated instrument.
func (current snapshot) price(expression *Expression, entry Entry, instrumentID int64) (float64, bool, error) {
	if expression == nil {
		return 0, true, nil
	}
	return expression.Price(current.resolver(entry, instrumentID, nil))
}

// output returns the fresh output read reads of the evaluated instrument or
// of the instrument read names. Values whose latest candle is older are stale
// and left out.
func (current snapshot) output(entry Entry, read Read, instrumentID int64) (closedindicator.Output, bool) {
	if read.Symbol != "" {
		var ok bool
		if instrumentID, ok = current.bySymbol[read.Symbol]; !ok {
			return closedindicator.Output{}, false
		}
	}
	position, ok := current.reads.positions[pair{instrumentID, current.reads.keys[entry.ID][read.Variable.Name]}]
	if !ok {
		return closedindicator.Output{}, false
	}
	value := current.values[position]
	if !value.OpenTime.Equal(read.Variable.Target.Interval.LastClosedOpenTime(current.now)) {
		return closedindicator.Output{}, false
	}
	for _, output := range value.Outputs {
		if output.Name == read.Variable.Output {
			return output, true
		}
	}
	return closedindicator.Output{}, false
}

// pair identifies the value of one target of one instrument.
type pair struct {
	instrumentID int64
	target       string
}

// reads are the values strategies read in one round.
type reads struct {
	// subscriptions are the distinct pairs, each with the points its deepest
	// read needs.
	subscriptions []closedindicator.Subscription
	positions     map[pair]int
	// keys maps the variables of each strategy to the keys of their targets.
	keys map[int64]map[string]string
}

// readsOf subscribes every read of the evaluated instrument on each
// instrument evaluated lists among active for its strategy, and every read
// through of only on the instrument it names among active.
func readsOf(strategies []Entry, evaluated func(Entry, []Instrument) []Instrument, active []Instrument) reads {
	result := reads{positions: map[pair]int{}, keys: make(map[int64]map[string]string, len(strategies))}
	symbols := bySymbol(active)
	subscribe := func(instrumentID int64, read Read, key string) {
		at := pair{instrumentID, key}
		position, ok := result.positions[at]
		if !ok {
			position = len(result.subscriptions)
			result.positions[at] = position
			result.subscriptions = append(result.subscriptions, closedindicator.Subscription{InstrumentID: instrumentID, Target: read.Variable.Target})
		}
		result.subscriptions[position].Points = max(result.subscriptions[position].Points, read.Shift+1)
	}
	for _, entry := range strategies {
		instruments := evaluated(entry, active)
		keys := map[string]string{}
		result.keys[entry.ID] = keys
		// The candles of the interval of entry price its trades.
		candles := Read{Variable: Variable{Target: CandleTarget(entry.Interval)}}
		for _, instrument := range instruments {
			subscribe(instrument.ID, candles, candles.Variable.Target.Key())
		}
		for _, read := range entry.reads {
			key, ok := keys[read.Variable.Name]
			if !ok {
				key = read.Variable.Target.Key()
				keys[read.Variable.Name] = key
			}
			if read.Symbol != "" {
				if id, ok := symbols[read.Symbol]; ok {
					subscribe(id, read, key)
				}
				continue
			}
			for _, instrument := range instruments {
				subscribe(instrument.ID, read, key)
			}
		}
	}
	return result
}

// readsAny reports whether entry reads any of targets.
func readsAny(entry Entry, targets []closedindicator.Target) bool {
	return slices.ContainsFunc(entry.reads, func(read Read) bool {
		return slices.ContainsFunc(targets, read.Variable.Target.Equal)
	})
}

func bySymbol(instruments []Instrument) map[string]int64 {
	result := make(map[string]int64, len(instruments))
	for _, instrument := range instruments {
		result[instrument.Symbol] = instrument.ID
	}
	return result
}
