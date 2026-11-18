package indicator

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"crypto-scanner/internal/market"
)

// Key identifies the type and the parameters of selection, so normalized
// selections with equal settings have equal keys.
func (selection Selection) Key() string {
	var key strings.Builder
	key.WriteString(string(selection.Type))
	for _, name := range slices.Sorted(maps.Keys(selection.Parameters)) {
		fmt.Fprintf(&key, "|%s=%v", name, selection.Parameters[name])
	}
	return key.String()
}

// Target is a selection calculated on one candle interval. Its key is
// computed once, when it is made, so comparing targets builds no strings;
// the parameters of its selection must not change afterwards.
type Target struct {
	interval  market.CandleInterval
	selection Selection
	key       string
}

// NewTarget makes the target of selection on interval.
func NewTarget(interval market.CandleInterval, selection Selection) Target {
	return Target{interval: interval, selection: selection, key: string(interval) + "|" + selection.Key()}
}

func (target Target) Interval() market.CandleInterval { return target.interval }
func (target Target) Selection() Selection            { return target.selection }

// Key identifies the interval and the selection of target, such as
// "1h|rsi|period=14".
func (target Target) Key() string { return target.key }

// Equal reports whether both targets identify the same interval and selection.
func (target Target) Equal(other Target) bool { return target.key == other.key }
