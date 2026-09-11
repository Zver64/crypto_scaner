package volatility

import (
	"math"
	"sort"
)

// exceedancePercentile returns the greatest observed value that at least the
// configured percentage of observations meet or exceed. The caller must
// provide at least one value and a percentile between 0 and 100.
func exceedancePercentile(values []float64, percentile float64) float64 {
	sort.Float64s(values)
	count := int(math.Ceil((percentile / 100) * float64(len(values))))
	index := min(len(values)-1, max(0, len(values)-count))
	return values[index]
}
