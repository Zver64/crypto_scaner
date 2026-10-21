// Package candle exposes the candle fields themselves as an internal
// indicator, so strategies read prices and volumes through the same
// closed-candle tracking as indicator values.
package candle

import (
	"fmt"
	"slices"

	"crypto-scanner/internal/indicator"
)

// Type is the registry identifier of the candle fields.
const Type indicator.Type = "candle"

// New returns the candle field module.
func New() indicator.Implementation { return module{} }

// module returns every candle field unchanged under its own name.
type module struct{}

func (module) Describe() indicator.Descriptor {
	descriptor := indicator.Descriptor{
		Type: Type, Title: "Candle", Group: "Candle", Internal: true,
		Inputs: slices.Clone(indicator.CandleFields),
	}
	for _, field := range indicator.CandleFields {
		descriptor.Outputs = append(descriptor.Outputs, indicator.OutputDescriptor{Name: field, Style: indicator.OutputLine})
	}
	return descriptor
}

func (module) Normalize(parameters indicator.Parameters) (indicator.Parameters, error) {
	if len(parameters) > 0 {
		return nil, fmt.Errorf("%w: candle fields have no parameters", indicator.ErrInvalidRequest)
	}
	return indicator.Parameters{}, nil
}

func (m module) Fields(parameters indicator.Parameters) ([]string, error) {
	if _, err := m.Normalize(parameters); err != nil {
		return nil, err
	}
	return slices.Clone(indicator.CandleFields), nil
}

func (m module) Lookback(parameters indicator.Parameters) (int, error) {
	_, err := m.Normalize(parameters)
	return 0, err
}

func (m module) Calculate(parameters indicator.Parameters, inputs indicator.Inputs) (indicator.Result, error) {
	if _, err := m.Normalize(parameters); err != nil {
		return indicator.Result{}, err
	}
	series, err := indicator.ReadInputs(inputs, indicator.CandleFields)
	if err != nil {
		return indicator.Result{}, fmt.Errorf("%w: %w", indicator.ErrInvalidRequest, err)
	}
	outputs := make(indicator.Outputs, len(indicator.CandleFields))
	for index, field := range indicator.CandleFields {
		outputs[field] = indicator.Series{Values: slices.Clone(series[index])}
	}
	return indicator.Result{Outputs: outputs}, nil
}
