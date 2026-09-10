package volatility

import "testing"

func TestExceedancePercentileSortsValuesAndGetsStricterAsPercentileIncreases(t *testing.T) {
	for _, test := range []struct {
		name       string
		values     []float64
		percentile float64
		want       float64
	}{
		{name: "lower percentile", values: []float64{8, 1, 4, 2}, percentile: 25, want: 5},
		{name: "higher percentile", values: []float64{4, 8, 2, 1}, percentile: 75, want: 1.75},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := exceedancePercentile(test.values, test.percentile); got != test.want {
				t.Fatalf("exceedancePercentile() = %v, want %v", got, test.want)
			}
		})
	}
}
