package indicator_test

import (
	"errors"
	"math"
	"testing"

	"crypto-scanner/internal/indicator"
)

type fakeImplementation struct {
	indicatorType indicator.Type
	lookback      int
	lookbackErr   error
	result        indicator.Result
	calculateErr  error
	calls         int
	parameters    indicator.Parameters
	inputs        indicator.Inputs
}

func (f *fakeImplementation) Type() indicator.Type {
	return f.indicatorType
}

func (f *fakeImplementation) Lookback(parameters indicator.Parameters) (int, error) {
	f.parameters = parameters
	return f.lookback, f.lookbackErr
}

func (f *fakeImplementation) Calculate(parameters indicator.Parameters, inputs indicator.Inputs) (indicator.Result, error) {
	f.calls++
	f.parameters = parameters
	f.inputs = inputs
	return f.result, f.calculateErr
}

func validResult() indicator.Result {
	return indicator.Result{Outputs: indicator.Outputs{
		"value": {Offset: 2, Values: []float64{10, 11}},
	}}
}

func TestRegistryDispatchesCalculationByType(t *testing.T) {
	first := &fakeImplementation{indicatorType: "first", result: validResult()}
	second := &fakeImplementation{indicatorType: "second", result: validResult()}
	registry, err := indicator.NewRegistry(first, second)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	parameters := indicator.Parameters{"period": 14}
	inputs := indicator.Inputs{"close": {1, 2, 3}}
	result, err := registry.Calculate(indicator.Request{
		Type:       "second",
		Parameters: parameters,
		Inputs:     inputs,
	})
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	if first.calls != 0 || second.calls != 1 {
		t.Fatalf("calls = first %d, second %d; want 0, 1", first.calls, second.calls)
	}
	if second.parameters["period"] != 14 {
		t.Fatalf("parameters were not passed to implementation: %#v", second.parameters)
	}
	if len(second.inputs["close"]) != 3 {
		t.Fatalf("inputs were not passed to implementation: %#v", second.inputs)
	}
	if result.Outputs["value"].Offset != 2 {
		t.Fatalf("result offset = %d, want 2", result.Outputs["value"].Offset)
	}
}

func TestRegistryReportsLookback(t *testing.T) {
	implementation := &fakeImplementation{indicatorType: "fake", lookback: 23, result: validResult()}
	registry, err := indicator.NewRegistry(implementation)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	parameters := indicator.Parameters{"window": 20}
	lookback, err := registry.Lookback("fake", parameters)
	if err != nil {
		t.Fatalf("Lookback() error = %v", err)
	}
	if lookback != 23 {
		t.Fatalf("Lookback() = %d, want 23", lookback)
	}
	if implementation.parameters["window"] != 20 {
		t.Fatalf("parameters were not passed to Lookback: %#v", implementation.parameters)
	}
}

func TestRegistryRejectsInvalidRegistrations(t *testing.T) {
	var typedNil *fakeImplementation
	tests := []struct {
		name            string
		implementations []indicator.Implementation
		want            error
	}{
		{name: "none", want: indicator.ErrEmptyRegistration},
		{name: "nil", implementations: []indicator.Implementation{nil}, want: indicator.ErrEmptyRegistration},
		{name: "typed nil", implementations: []indicator.Implementation{typedNil}, want: indicator.ErrEmptyRegistration},
		{name: "empty type", implementations: []indicator.Implementation{&fakeImplementation{}}, want: indicator.ErrEmptyRegistration},
		{
			name: "duplicate type",
			implementations: []indicator.Implementation{
				&fakeImplementation{indicatorType: "same"},
				&fakeImplementation{indicatorType: "same"},
			},
			want: indicator.ErrDuplicateRegistration,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := indicator.NewRegistry(test.implementations...)
			if !errors.Is(err, test.want) {
				t.Fatalf("NewRegistry() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestRegistryRejectsUnknownType(t *testing.T) {
	registry, err := indicator.NewRegistry(&fakeImplementation{indicatorType: "known"})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	if _, err := registry.Lookback("missing", nil); !errors.Is(err, indicator.ErrUnknownType) {
		t.Fatalf("Lookback() error = %v, want ErrUnknownType", err)
	}
	if _, err := registry.Calculate(indicator.Request{Type: "missing"}); !errors.Is(err, indicator.ErrUnknownType) {
		t.Fatalf("Calculate() error = %v, want ErrUnknownType", err)
	}
}

func TestRegistryRejectsInvalidImplementationResults(t *testing.T) {
	tests := []struct {
		name   string
		result indicator.Result
	}{
		{name: "missing outputs", result: indicator.Result{}},
		{name: "empty output name", result: indicator.Result{Outputs: indicator.Outputs{"": {Values: []float64{1}}}}},
		{name: "negative offset", result: indicator.Result{Outputs: indicator.Outputs{"value": {Offset: -1, Values: []float64{1}}}}},
		{name: "nil values", result: indicator.Result{Outputs: indicator.Outputs{"value": {Values: nil}}}},
		{name: "NaN value", result: indicator.Result{Outputs: indicator.Outputs{"value": {Values: []float64{math.NaN()}}}}},
		{name: "infinite value", result: indicator.Result{Outputs: indicator.Outputs{"value": {Values: []float64{math.Inf(1)}}}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry, err := indicator.NewRegistry(&fakeImplementation{indicatorType: "fake", result: test.result})
			if err != nil {
				t.Fatalf("NewRegistry() error = %v", err)
			}

			_, err = registry.Calculate(indicator.Request{Type: "fake"})
			if !errors.Is(err, indicator.ErrInvalidResult) {
				t.Fatalf("Calculate() error = %v, want ErrInvalidResult", err)
			}
		})
	}
}

func TestRegistryAllowsEmptyValidValues(t *testing.T) {
	result := indicator.Result{Outputs: indicator.Outputs{
		"value": {Offset: 14, Values: []float64{}},
	}}
	registry, err := indicator.NewRegistry(&fakeImplementation{indicatorType: "fake", result: result})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	if _, err := registry.Calculate(indicator.Request{Type: "fake"}); err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
}

func TestRegistryRejectsNegativeLookback(t *testing.T) {
	registry, err := indicator.NewRegistry(&fakeImplementation{indicatorType: "fake", lookback: -1})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	_, err = registry.Lookback("fake", nil)
	if !errors.Is(err, indicator.ErrInvalidResult) {
		t.Fatalf("Lookback() error = %v, want ErrInvalidResult", err)
	}

	var _ indicator.Calculator = registry
}
