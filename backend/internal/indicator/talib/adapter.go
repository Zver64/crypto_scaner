package talib

import (
	"fmt"
	"math"
	"slices"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"

	ta "github.com/Zver64/ta-lib-cgo"
)

const (
	// RSIType is the stable registry identifier for the relative strength index.
	RSIType indicator.Type = "rsi"
	// EMAType is the stable registry identifier for the exponential moving average.
	EMAType indicator.Type = "ema"
)

// maTypes are the TA-Lib moving average types by their TA_MAType values,
// which calls write by name. TA_MAType_DISABLED (10) and TA_MAType_DEFAULT
// (11) are not moving averages.
var maTypes = []indicator.Choice{
	{Value: 0, Title: "SMA", Name: "sma"}, {Value: 1, Title: "EMA", Name: "ema"}, {Value: 2, Title: "WMA", Name: "wma"},
	{Value: 3, Title: "DEMA", Name: "dema"}, {Value: 4, Title: "TEMA", Name: "tema"}, {Value: 5, Title: "TRIMA", Name: "trima"},
	{Value: 6, Title: "KAMA", Name: "kama"}, {Value: 7, Title: "MAMA", Name: "mama"}, {Value: 8, Title: "T3", Name: "t3"},
	{Value: 9, Title: "HMA", Name: "hma"}, {Value: 12, Title: "ZLEMA", Name: "zlema"}, {Value: 13, Title: "RMA", Name: "rma"},
	{Value: 14, Title: "VIDYA", Name: "vidya"}, {Value: 15, Title: "ALMA", Name: "alma"},
}

// sourceClose is the default source: the close price.
var sourceClose = float64(slices.Index(indicator.CandleFields, "close"))

type paramKind int

const (
	paramInteger paramKind = iota
	paramReal
	paramMAType
	// paramSource chooses the series of the input named by its key
	// (indicator.ParameterSource).
	paramSource
)

type param struct {
	key          string
	title        string
	description  string
	kind         paramKind
	minimum      float64
	maximum      float64
	defaultValue float64
}

type output struct {
	name  string
	style indicator.OutputStyle
	// sparse outputs are empty where TA-Lib returns NaN or an infinity.
	sparse    bool
	direction indicator.OutputDirection
}

// spec is one generated TA-Lib function. Option values reach lookback and
// call in params order, followed by the source parameters they ignore; call
// receives inputs in inputs order and returns outputs in outputs order, each
// as long as the inputs. An input named after a source parameter reads the
// series that parameter chooses, which the registry supplies.
type spec struct {
	name          string
	indicatorType indicator.Type
	title         string
	group         string
	overlap       bool
	candlestick   bool
	unstable      bool
	inputs        []string
	params        []param
	outputs       []output
	lookback      func(p []float64) int
	call          func(in [][]float64, p []float64) [][]float64
}

// New returns one implementation per generated TA-Lib function.
func New() []indicator.Implementation {
	result := make([]indicator.Implementation, len(functions))
	for index := range functions {
		result[index] = &function{spec: &functions[index]}
	}
	return result
}

// function adapts any generated spec to the indicator contract.
type function struct {
	spec *spec
}

func (f *function) Describe() indicator.Descriptor {
	descriptor := indicator.Descriptor{
		Type:     f.spec.indicatorType,
		Title:    f.spec.title,
		Group:    f.spec.group,
		Overlay:  f.spec.overlap,
		Pattern:  f.spec.candlestick,
		Unstable: f.spec.unstable,
		Inputs:   slices.Clone(f.spec.inputs),
	}
	for _, p := range f.spec.params {
		parameter := indicator.ParameterDescriptor{
			Key: p.key, Title: p.title, Description: p.description, Default: p.defaultValue,
		}
		switch p.kind {
		case paramInteger:
			parameter.Kind = indicator.ParameterInteger
			parameter.Minimum, parameter.Maximum = p.minimum, integerMaximum(p)
		case paramReal:
			parameter.Kind = indicator.ParameterReal
			parameter.Minimum, parameter.Maximum = p.minimum, p.maximum
		case paramMAType:
			parameter.Kind = indicator.ParameterChoice
			parameter.Choices = slices.Clone(maTypes)
		case paramSource:
			parameter.Kind = indicator.ParameterSource
			parameter.Choices = indicator.FieldChoices()
		}
		descriptor.Parameters = append(descriptor.Parameters, parameter)
	}
	for _, o := range f.spec.outputs {
		descriptor.Outputs = append(descriptor.Outputs, indicator.OutputDescriptor{Name: o.name, Style: o.style, Sparse: o.sparse, Direction: o.direction})
	}
	return descriptor
}

func (f *function) Normalize(parameters indicator.Parameters) (indicator.Parameters, error) {
	values, err := f.values(parameters)
	if err != nil {
		return nil, err
	}
	result := make(indicator.Parameters, len(values))
	for index, p := range f.spec.params {
		switch p.kind {
		case paramSource:
		case paramReal:
			result[p.key] = values[index]
		default:
			result[p.key] = int(values[index])
		}
	}
	return result, nil
}

func (f *function) Fields(parameters indicator.Parameters) ([]string, error) {
	if _, err := f.values(parameters); err != nil {
		return nil, err
	}
	return slices.Clone(f.spec.inputs), nil
}

func (f *function) Lookback(parameters indicator.Parameters) (int, error) {
	values, err := f.values(parameters)
	if err != nil {
		return 0, err
	}
	return f.lookback(values)
}

func (f *function) lookback(values []float64) (int, error) {
	lookback := f.spec.lookback(values)
	if lookback < 0 {
		return 0, fmt.Errorf("%w: %s rejected parameters %v", indicator.ErrInvalidRequest, f.spec.name, values)
	}
	return lookback, nil
}

func (f *function) Calculate(parameters indicator.Parameters, inputs indicator.Inputs) (indicator.Result, error) {
	values, err := f.values(parameters)
	if err != nil {
		return indicator.Result{}, err
	}
	series, err := indicator.ReadInputs(inputs, f.spec.inputs)
	if err != nil {
		return indicator.Result{}, fmt.Errorf("%w: %w", indicator.ErrInvalidRequest, err)
	}
	length := len(series[0])

	offset, err := f.lookback(values)
	if err != nil {
		return indicator.Result{}, err
	}
	outputs := make(indicator.Outputs, len(f.spec.outputs))
	if length <= offset {
		for _, o := range f.spec.outputs {
			outputs[o.name] = indicator.Series{Offset: offset, Values: []float64{}}
		}
		return indicator.Result{Outputs: outputs}, nil
	}
	calculated, err := f.invoke(series, values)
	if err != nil {
		return indicator.Result{}, err
	}
	for index, o := range f.spec.outputs {
		if len(calculated[index]) != length {
			return indicator.Result{}, fmt.Errorf("%w: %s returned %d values for %d inputs", indicator.ErrInvalidRequest, f.spec.name, len(calculated[index]), length)
		}
		values := calculated[index][offset:]
		if o.sparse {
			for position, value := range values {
				if math.IsInf(value, 0) {
					values[position] = math.NaN()
				}
			}
		}
		outputs[o.name] = indicator.Series{Offset: offset, Values: values}
	}
	return indicator.Result{Outputs: outputs}, nil
}

// invoke converts the wrapper's panics on TA-Lib return codes into errors, so
// a rejected calculation cannot stop a background service.
func (f *function) invoke(series [][]float64, values []float64) (calculated [][]float64, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			taErr, ok := recovered.(*ta.TALibError)
			if !ok {
				panic(recovered)
			}
			err = fmt.Errorf("%w: %s: %v", indicator.ErrInvalidRequest, f.spec.name, taErr)
		}
	}()
	return f.spec.call(series, values), nil
}

// values validates parameters and returns them in spec order with defaults for
// omitted keys. Source parameters, which the registry validates, stay zero.
func (f *function) values(parameters indicator.Parameters) ([]float64, error) {
	for key := range parameters {
		if !slices.ContainsFunc(f.spec.params, func(p param) bool { return p.key == key }) {
			return nil, fmt.Errorf("%w: %s has no parameter %q", indicator.ErrInvalidRequest, f.spec.indicatorType, key)
		}
	}
	values := make([]float64, len(f.spec.params))
	for index, p := range f.spec.params {
		raw, exists := parameters[p.key]
		if p.kind == paramSource {
			continue
		}
		if !exists {
			values[index] = p.defaultValue
			continue
		}
		value, err := indicator.ParseNumber(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: %s %w", indicator.ErrInvalidRequest, p.key, err)
		}
		switch p.kind {
		case paramInteger:
			if math.Trunc(value) != value || value < p.minimum || value > integerMaximum(p) {
				return nil, fmt.Errorf("%w: %s must be an integer between %g and %g", indicator.ErrInvalidRequest, p.key, p.minimum, integerMaximum(p))
			}
		case paramReal:
			if value < p.minimum || value > p.maximum {
				return nil, fmt.Errorf("%w: %s must be between %g and %g", indicator.ErrInvalidRequest, p.key, p.minimum, p.maximum)
			}
		case paramMAType:
			if !slices.ContainsFunc(maTypes, func(choice indicator.Choice) bool { return float64(choice.Value) == value }) {
				return nil, fmt.Errorf("%w: %s must be a moving average type", indicator.ErrInvalidRequest, p.key)
			}
		}
		values[index] = value
	}
	return values, nil
}

// integerMaximum caps TA-Lib's period limits (up to 100000) at the stored
// history depth, which no calculation can exceed.
func integerMaximum(p param) float64 { return min(p.maximum, market.SyncDepth) }

func integers(values []int32) []float64 {
	result := make([]float64, len(values))
	for index, value := range values {
		result[index] = float64(value)
	}
	return result
}
