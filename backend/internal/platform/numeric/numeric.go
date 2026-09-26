// Package numeric validates floating-point values crossing process boundaries.
package numeric

import (
	"fmt"
	"math"
	"strconv"
)

// Finite reports whether value is neither NaN nor infinite.
func Finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// ParseFinite parses a decimal string and rejects NaN and infinities.
func ParseFinite(raw string) (float64, error) {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	if !Finite(value) {
		return 0, fmt.Errorf("non-finite number %q", raw)
	}
	return value, nil
}
