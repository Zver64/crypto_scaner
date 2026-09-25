// Package talib contains reusable adapter families and indicator definitions
// backed by github.com/markcheno/go-talib. It translates the library's
// zero-filled warm-up prefixes into the offset-based indicator contract.
package talib

import (
	"errors"
	"fmt"
	"math"

	"crypto-scanner/internal/indicator"
)

// ErrInvalidRequest indicates invalid parameters or inputs supplied to a
// go-talib-backed indicator adapter.
var ErrInvalidRequest = errors.New("invalid TA-Lib indicator request")

// singleInputPeriod adapts the common TA-Lib function shape that accepts one
// numeric input series and one integer period and returns one numeric series.
type singleInputPeriod struct {
	spec singleInputPeriodSpec
}

type singleInputPeriodSpec struct {
	indicatorType indicator.Type
	inputName     string
	outputName    string
	minimumPeriod int
	maximumPeriod int
	offset        func(int) int
	lookback      func(int) int
	calculate     func([]float64, int) []float64
}

func newSingleInputPeriod(spec singleInputPeriodSpec) indicator.Implementation {
	return &singleInputPeriod{spec: spec}
}

func (adapter *singleInputPeriod) Type() indicator.Type {
	return adapter.spec.indicatorType
}

func (adapter *singleInputPeriod) Inputs() []string { return []string{adapter.spec.inputName} }

func (adapter *singleInputPeriod) Lookback(parameters indicator.Parameters) (int, error) {
	period, err := adapter.period(parameters)
	if err != nil {
		return 0, err
	}
	lookback := adapter.spec.lookback(period)
	if lookback < 0 {
		return 0, fmt.Errorf("%w: %q produced negative lookback", ErrInvalidRequest, adapter.spec.indicatorType)
	}
	return lookback, nil
}

func (adapter *singleInputPeriod) Calculate(parameters indicator.Parameters, inputs indicator.Inputs) (indicator.Result, error) {
	period, err := adapter.period(parameters)
	if err != nil {
		return indicator.Result{}, err
	}
	values, exists := inputs[adapter.spec.inputName]
	if !exists {
		return indicator.Result{}, fmt.Errorf("%w: %s input is required", ErrInvalidRequest, adapter.spec.inputName)
	}
	for index, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return indicator.Result{}, fmt.Errorf("%w: %s value %d is not finite", ErrInvalidRequest, adapter.spec.inputName, index)
		}
	}

	offset := adapter.spec.offset(period)
	if offset < 0 {
		return indicator.Result{}, fmt.Errorf("%w: %q produced negative offset", ErrInvalidRequest, adapter.spec.indicatorType)
	}
	output := []float64{}
	if len(values) > offset {
		calculated := adapter.spec.calculate(values, period)
		if len(calculated) < offset {
			return indicator.Result{}, fmt.Errorf("%w: %q returned a series shorter than its offset", ErrInvalidRequest, adapter.spec.indicatorType)
		}
		output = append(output, calculated[offset:]...)
	}
	return indicator.Result{Outputs: indicator.Outputs{
		adapter.spec.outputName: {Offset: offset, Values: output},
	}}, nil
}

func (adapter *singleInputPeriod) period(parameters indicator.Parameters) (int, error) {
	value, exists := parameters["period"]
	if !exists {
		return 0, fmt.Errorf("%w: period is required", ErrInvalidRequest)
	}
	var period int
	switch typed := value.(type) {
	case int:
		period = typed
	case int32:
		period = int(typed)
	case int64:
		period = int(typed)
	case float64:
		if math.Trunc(typed) != typed || typed > float64(math.MaxInt) || typed < float64(math.MinInt) {
			return 0, fmt.Errorf("%w: period must be an integer", ErrInvalidRequest)
		}
		period = int(typed)
	default:
		return 0, fmt.Errorf("%w: period must be an integer", ErrInvalidRequest)
	}
	if period < adapter.spec.minimumPeriod || period > adapter.spec.maximumPeriod {
		return 0, fmt.Errorf("%w: period must be between %d and %d", ErrInvalidRequest, adapter.spec.minimumPeriod, adapter.spec.maximumPeriod)
	}
	return period, nil
}
