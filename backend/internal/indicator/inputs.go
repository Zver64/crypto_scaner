package indicator

import (
	"errors"
	"fmt"

	"crypto-scanner/internal/platform/numeric"
)

// ParseNumber reads a parameter value supplied as any Go number. Modules wrap
// its error in their own invalid request error.
func ParseNumber(raw any) (float64, error) {
	var value float64
	switch typed := raw.(type) {
	case int:
		value = float64(typed)
	case int32:
		value = float64(typed)
	case int64:
		value = float64(typed)
	case float64:
		value = typed
	default:
		return 0, errors.New("must be a number")
	}
	if !numeric.Finite(value) {
		return 0, errors.New("must be finite")
	}
	return value, nil
}

// ReadInputs returns the named input series in names order, checking that
// each is present, finite, and as long as the others. Modules wrap its error
// in their own invalid request error.
func ReadInputs(inputs Inputs, names []string) ([][]float64, error) {
	result := make([][]float64, len(names))
	for index, name := range names {
		values, exists := inputs[name]
		if !exists {
			return nil, fmt.Errorf("%s input is required", name)
		}
		if index > 0 && len(values) != len(result[0]) {
			return nil, fmt.Errorf("%s has %d values, want %d", name, len(values), len(result[0]))
		}
		for position, value := range values {
			if !numeric.Finite(value) {
				return nil, fmt.Errorf("%s value %d is not finite", name, position)
			}
		}
		result[index] = values
	}
	return result, nil
}
