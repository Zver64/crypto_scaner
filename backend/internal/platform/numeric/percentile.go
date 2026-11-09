package numeric

import (
	"math"
	"sort"
)

// ExceedancePercentile returns the greatest observed value that at least the
// percentile percentage of values meet or exceed, sorting values in place.
// The caller must provide at least one value and a percentile between 0 and
// 100.
func ExceedancePercentile(values []float64, percentile float64) float64 {
	sort.Float64s(values)
	count := int(math.Ceil((percentile / 100) * float64(len(values))))
	index := min(len(values)-1, max(0, len(values)-count))
	return values[index]
}
