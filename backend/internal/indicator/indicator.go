// Package indicator defines the transport-independent contract for calculating
// technical indicators.
//
// Inputs and outputs are named numeric series. The package has no notion of
// candles, timestamps, persistence, HTTP, or a particular indicator library.
// Concrete implementations live in adapter packages and are selected by a
// Registry.
package indicator

// Type is the stable identifier of an indicator implementation.
type Type string

// Parameters contains implementation-specific indicator settings.
// Implementations own the validation and interpretation of their parameters.
type Parameters map[string]any

// Inputs contains numeric input series keyed by their implementation-defined
// names, for example "close". The core does not require all series to have the
// same length; that is part of each implementation's contract.
type Inputs map[string][]float64

// Request describes one indicator calculation.
type Request struct {
	Type       Type
	Parameters Parameters
	Inputs     Inputs
}

// Series is one output series. Offset is the zero-based input position matching
// Values[0]. Values contains only valid values; warm-up placeholders must not be
// included.
type Series struct {
	Offset int
	Values []float64
}

// Outputs contains indicator output series keyed by their
// implementation-defined names, for example "rsi".
type Outputs map[string]Series

// Result is the unified result returned by every indicator implementation.
type Result struct {
	Outputs Outputs
}

// Implementation is the contract implemented by a concrete indicator adapter.
// Parameter and input validation specific to an indicator belongs in the
// implementation, while Registry validates the common result contract.
type Implementation interface {
	Type() Type
	Lookback(parameters Parameters) (int, error)
	Calculate(parameters Parameters, inputs Inputs) (Result, error)
}

// Calculator is the consumer-facing indicator calculation contract.
type Calculator interface {
	Lookback(indicatorType Type, parameters Parameters) (int, error)
	Calculate(request Request) (Result, error)
}
