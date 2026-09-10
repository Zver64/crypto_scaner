package volatility

import (
	"math"
	"sort"
)

// exceedancePercentile returns the range that the configured percentage of
// observations meet or exceed. The caller must provide at least one value and
// a percentile between 0 and 100.
func exceedancePercentile(values []float64, percentile float64) float64 {
	sort.Float64s(values)
	rank := ((100 - percentile) / 100) * float64(len(values)-1)
	lower, upper := int(math.Floor(rank)), int(math.Ceil(rank))
	return values[lower] + (rank-float64(lower))*(values[upper]-values[lower])
}
