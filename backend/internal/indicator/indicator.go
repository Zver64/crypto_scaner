// Package indicator defines the transport-independent contract for calculating
// technical indicators.
//
// Implementations consume and produce named numeric series and know nothing
// about candles, timestamps, persistence, HTTP, or a particular indicator
// library. Registry.CalculateCandles is the candle-aware layer on top of that
// contract: it converts market.Candle history into input series and aligns the
// results with candle open times. Concrete implementations live in adapter
// packages and are selected by a Registry.
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

// ParameterKind is the value domain of one parameter.
type ParameterKind string

const (
	ParameterInteger ParameterKind = "integer"
	ParameterReal    ParameterKind = "real"
	// ParameterChoice is an integer limited to the listed Choices.
	ParameterChoice ParameterKind = "choice"
)

// Choice is one allowed value of a ParameterChoice parameter. Name is the
// stable identifier of a named value, such as the candle field "volume", and
// is empty for plain enumerations.
type Choice struct {
	Value int
	Title string
	Name  string
}

// ParameterDescriptor describes one parameter accepted by Normalize.
type ParameterDescriptor struct {
	Key         string
	Title       string
	Description string
	Kind        ParameterKind
	Default     float64
	// Minimum and Maximum bound integer and real parameters.
	Minimum float64
	Maximum float64
	Choices []Choice
}

// OutputStyle suggests how clients draw an output series.
type OutputStyle string

const (
	OutputLine       OutputStyle = "line"
	OutputDashedLine OutputStyle = "dashed_line"
	OutputHistogram  OutputStyle = "histogram"
	OutputUpperLimit OutputStyle = "upper_limit"
	OutputLowerLimit OutputStyle = "lower_limit"
	// OutputHidden is an output that charts do not draw, such as a distance
	// in candles that strategies read.
	OutputHidden OutputStyle = "hidden"
)

// OutputDescriptor describes one output series returned by Calculate.
type OutputDescriptor struct {
	Name  string
	Style OutputStyle
	// Count marks an output that counts candles, a whole number that
	// strategies may use as a prev shift.
	Count bool
}

// Descriptor is the self-description of an implementation. Registries use it
// to dispatch and validate results, and clients use it to build settings.
type Descriptor struct {
	Type  Type
	Title string
	Group string
	// Overlay reports outputs on the price scale.
	Overlay bool
	// Pattern reports a candlestick pattern: an output is non-zero on candles
	// where the pattern is found, positive when bullish and negative when
	// bearish.
	Pattern bool
	// Unstable reports that values depend on all preceding history, not only
	// on Lookback, so comparable values need the same history depth.
	Unstable bool
	// Internal reports a module the backend uses itself; clients cannot
	// configure it.
	Internal bool
	// Inputs names the input slots consumed by Calculate: candle fields, or
	// the key of the parameter that chooses the field. Fields resolves them.
	Inputs     []string
	Parameters []ParameterDescriptor
	// Outputs names the series returned by Calculate, for example "rsi".
	Outputs []OutputDescriptor
}

// Implementation is the contract implemented by a concrete indicator adapter.
// Parameter and input validation specific to an indicator belongs in the
// implementation, while Registry validates the common result contract.
type Implementation interface {
	Describe() Descriptor
	// Normalize validates parameters and returns their canonical form with
	// defaults filled in, so equal settings compare equal.
	Normalize(parameters Parameters) (Parameters, error)
	// Fields returns the candle fields Calculate reads, in input order and
	// keyed by those names in Inputs.
	Fields(parameters Parameters) ([]string, error)
	Lookback(parameters Parameters) (int, error)
	Calculate(parameters Parameters, inputs Inputs) (Result, error)
}
