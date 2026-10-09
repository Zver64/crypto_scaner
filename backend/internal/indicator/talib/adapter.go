package talib

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"

	ta "github.com/TA-Lib/ta-lib-cgo"
)

const (
	// RSIType is the stable registry identifier for the relative strength index.
	RSIType indicator.Type = "rsi"
	// EMAType is the stable registry identifier for the exponential moving average.
	EMAType indicator.Type = "ema"
)

// ErrInvalidRequest indicates invalid parameters or inputs supplied to a
// TA-Lib-backed indicator.
var ErrInvalidRequest = errors.New("invalid TA-Lib indicator request")

// maTypes are the TA-Lib moving average types in TA_MAType order.
var maTypes = []indicator.Choice{
	{Value: 0, Title: "SMA"}, {Value: 1, Title: "EMA"}, {Value: 2, Title: "WMA"},
	{Value: 3, Title: "DEMA"}, {Value: 4, Title: "TEMA"}, {Value: 5, Title: "TRIMA"},
	{Value: 6, Title: "KAMA"}, {Value: 7, Title: "MAMA"}, {Value: 8, Title: "T3"},
}

// sourceChoices are the candle fields a series input can read, in
// indicator.CandleFields order.
var sourceChoices = func() []indicator.Choice {
	titles := map[string]string{
		"open": "Open", "high": "High", "low": "Low", "close": "Close", "volume": "Volume",
		"quote_asset_volume": "Quote Asset Volume", "trade_count": "Trade Count",
	}
	choices := make([]indicator.Choice, len(indicator.CandleFields))
	for index, field := range indicator.CandleFields {
		choices[index] = indicator.Choice{Value: index, Title: titles[field], Name: field}
	}
	return choices
}()

// sourceClose is the default source: the close price.
var sourceClose = float64(slices.Index(indicator.CandleFields, "close"))

type paramKind int

const (
	paramInteger paramKind = iota
	paramReal
	paramMAType
	// paramSource chooses the candle field of the input named by its key.
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
}

// spec is one generated TA-Lib function. Option values reach lookback and
// call in params order, followed by the source parameters they ignore; call
// receives inputs in inputs order and returns outputs in outputs order, each
// as long as the inputs. An input named after a source parameter reads the
// field that parameter chooses.
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
			parameter.Kind = indicator.ParameterChoice
			parameter.Choices = slices.Clone(sourceChoices)
		}
		descriptor.Parameters = append(descriptor.Parameters, parameter)
	}
	for _, o := range f.spec.outputs {
		descriptor.Outputs = append(descriptor.Outputs, indicator.OutputDescriptor{Name: o.name, Style: o.style})
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
		if p.kind == paramReal {
			result[p.key] = values[index]
		} else {
			result[p.key] = int(values[index])
		}
	}
	return result, nil
}

func (f *function) Fields(parameters indicator.Parameters) ([]string, error) {
	values, err := f.values(parameters)
	if err != nil {
		return nil, err
	}
	return f.fields(values), nil
}

// fields resolves the inputs to candle fields.
func (f *function) fields(values []float64) []string {
	fields := slices.Clone(f.spec.inputs)
	for index, name := range fields {
		position := slices.IndexFunc(f.spec.params, func(p param) bool { return p.kind == paramSource && p.key == name })
		if position >= 0 {
			fields[index] = indicator.CandleFields[int(values[position])]
		}
	}
	return fields
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
		return 0, fmt.Errorf("%w: %s rejected parameters %v", ErrInvalidRequest, f.spec.name, values)
	}
	return lookback, nil
}

func (f *function) Calculate(parameters indicator.Parameters, inputs indicator.Inputs) (indicator.Result, error) {
	values, err := f.values(parameters)
	if err != nil {
		return indicator.Result{}, err
	}
	series, err := indicator.ReadInputs(inputs, f.fields(values))
	if err != nil {
		return indicator.Result{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
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
			return indicator.Result{}, fmt.Errorf("%w: %s returned %d values for %d inputs", ErrInvalidRequest, f.spec.name, len(calculated[index]), length)
		}
		outputs[o.name] = indicator.Series{Offset: offset, Values: calculated[index][offset:]}
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
			err = fmt.Errorf("%w: %s: %v", ErrInvalidRequest, f.spec.name, taErr)
		}
	}()
	return f.spec.call(series, values), nil
}

// values validates parameters and returns them in spec order with defaults for
// omitted keys.
func (f *function) values(parameters indicator.Parameters) ([]float64, error) {
	for key := range parameters {
		if !slices.ContainsFunc(f.spec.params, func(p param) bool { return p.key == key }) {
			return nil, fmt.Errorf("%w: %s has no parameter %q", ErrInvalidRequest, f.spec.indicatorType, key)
		}
	}
	values := make([]float64, len(f.spec.params))
	for index, p := range f.spec.params {
		raw, exists := parameters[p.key]
		if !exists {
			values[index] = p.defaultValue
			continue
		}
		value, err := indicator.ParseNumber(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: %s %w", ErrInvalidRequest, p.key, err)
		}
		switch p.kind {
		case paramInteger:
			if math.Trunc(value) != value || value < p.minimum || value > integerMaximum(p) {
				return nil, fmt.Errorf("%w: %s must be an integer between %g and %g", ErrInvalidRequest, p.key, p.minimum, integerMaximum(p))
			}
		case paramReal:
			if value < p.minimum || value > p.maximum {
				return nil, fmt.Errorf("%w: %s must be between %g and %g", ErrInvalidRequest, p.key, p.minimum, p.maximum)
			}
		case paramMAType:
			// Compared before converting, which could overflow.
			if math.Trunc(value) != value || value < 0 || value >= float64(len(maTypes)) {
				return nil, fmt.Errorf("%w: %s must be a moving average type between 0 and %d", ErrInvalidRequest, p.key, len(maTypes)-1)
			}
		case paramSource:
			if math.Trunc(value) != value || value < 0 || value >= float64(len(sourceChoices)) {
				return nil, fmt.Errorf("%w: %s must be a candle field between 0 and %d", ErrInvalidRequest, p.key, len(sourceChoices)-1)
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
