package indicator

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"crypto-scanner/internal/platform/numeric"
)

var (
	// ErrUnknownType indicates that no implementation is registered for a type.
	ErrUnknownType = errors.New("unknown indicator type")
	// ErrEmptyRegistration indicates an empty registry, nil implementation, or
	// implementation with an empty type.
	ErrEmptyRegistration = errors.New("empty indicator registration")
	// ErrDuplicateRegistration indicates that a type was registered more than once.
	ErrDuplicateRegistration = errors.New("duplicate indicator registration")
	// ErrInvalidResult indicates that an implementation violated the common
	// lookback or output contract.
	ErrInvalidResult = errors.New("invalid indicator result")
	// ErrInvalidRequest indicates parameters or inputs a module rejects.
	ErrInvalidRequest = errors.New("invalid indicator request")
)

// Registry dispatches generic requests to registered implementations. A
// registry is immutable after construction and is safe for concurrent use when
// its implementations are safe for concurrent use.
type Registry struct {
	implementations map[Type]registered
}

type registered struct {
	implementation Implementation
	descriptor     Descriptor
	outputs        []string
	// sparse holds the outputs that may be empty.
	sparse map[string]bool
	// sources are the source parameters.
	sources []ParameterDescriptor
}

// NewRegistry creates a calculator from concrete implementations. At least one
// non-nil implementation is required, and each non-empty type may occur once.
func NewRegistry(implementations ...Implementation) (*Registry, error) {
	if len(implementations) == 0 {
		return nil, ErrEmptyRegistration
	}

	byType := make(map[Type]registered, len(implementations))
	for index, implementation := range implementations {
		if implementation == nil {
			return nil, fmt.Errorf("%w at index %d", ErrEmptyRegistration, index)
		}

		descriptor := implementation.Describe()
		indicatorType := descriptor.Type
		if strings.TrimSpace(string(indicatorType)) == "" {
			return nil, fmt.Errorf("%w at index %d: type is empty", ErrEmptyRegistration, index)
		}
		if _, exists := byType[indicatorType]; exists {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateRegistration, indicatorType)
		}
		outputs := make([]string, len(descriptor.Outputs))
		sparse := map[string]bool{}
		for position, output := range descriptor.Outputs {
			outputs[position] = output.Name
			sparse[output.Name] = output.Sparse
		}
		if err := validateOutputs(outputs); err != nil {
			return nil, fmt.Errorf("%w: %q: %v", ErrEmptyRegistration, indicatorType, err)
		}
		for _, output := range descriptor.Outputs {
			if output.Shift != "" && !slices.ContainsFunc(descriptor.Parameters, func(parameter ParameterDescriptor) bool {
				return parameter.Key == output.Shift && parameter.Kind == ParameterInteger
			}) {
				return nil, fmt.Errorf("%w: %q: output %q is shifted by %q, which is not an integer parameter", ErrEmptyRegistration, indicatorType, output.Name, output.Shift)
			}
		}
		var sources []ParameterDescriptor
		for _, parameter := range descriptor.Parameters {
			if parameter.Kind == ParameterSource {
				sources = append(sources, parameter)
			}
		}
		byType[indicatorType] = registered{implementation: implementation, descriptor: descriptor, outputs: outputs, sparse: sparse, sources: sources}
	}

	return &Registry{implementations: byType}, nil
}

// Lookback reports the number of preceding input values required by the
// selected implementation, with the lookback of its nested source: the
// source warms up first. A negative value is an invalid implementation
// result.
func (r *Registry) Lookback(indicatorType Type, parameters Parameters) (int, error) {
	entry, err := r.entry(indicatorType)
	if err != nil {
		return 0, err
	}

	lookback, err := entry.implementation.Lookback(parameters)
	if err != nil {
		return 0, err
	}
	if lookback < 0 {
		return 0, fmt.Errorf("%w: indicator %q returned negative lookback %d", ErrInvalidResult, indicatorType, lookback)
	}
	nested := 0
	for _, parameter := range entry.sources {
		source, err := r.source(parameter, parameters)
		if err != nil {
			return 0, err
		}
		if source.nested != nil {
			depth, err := r.Lookback(source.nested.Selection.Type, source.nested.Selection.Parameters)
			if err != nil {
				return 0, err
			}
			nested = max(nested, depth)
		}
	}

	return lookback + nested, nil
}

// Fields returns the candle fields the selection reads, those of its nested
// sources included.
func (r *Registry) Fields(selection Selection) ([]string, error) {
	entry, err := r.entry(selection.Type)
	if err != nil {
		return nil, err
	}
	sources, err := r.inputSources(entry, selection)
	if err != nil {
		return nil, err
	}
	var fields []string
	for _, source := range sources {
		if source.nested == nil {
			fields = append(fields, source.field)
			continue
		}
		nested, err := r.Fields(source.nested.Selection)
		if err != nil {
			return nil, err
		}
		fields = append(fields, nested...)
	}
	return fields, nil
}

// inputSource is what one input of a calculation reads: a candle field or
// the output of a nested selection, which is normalized.
type inputSource struct {
	// name keys the series in the Inputs of the implementation.
	name   string
	field  string
	nested *Source
	// value is the canonical parameter value of a source parameter.
	value any
}

// inputSources resolves the inputs the implementation reads with the
// parameters of selection, in input order.
func (r *Registry) inputSources(entry registered, selection Selection) ([]inputSource, error) {
	names, err := entry.implementation.Fields(selection.Parameters)
	if err != nil {
		return nil, err
	}
	sources := make([]inputSource, len(names))
	for index, name := range names {
		if slices.Contains(CandleFields, name) {
			sources[index] = inputSource{name: name, field: name}
			continue
		}
		position := slices.IndexFunc(entry.sources, func(parameter ParameterDescriptor) bool { return parameter.Key == name })
		if position < 0 {
			return nil, fmt.Errorf("%w: indicator %q reads unsupported field %q", ErrInvalidResult, selection.Type, name)
		}
		source, err := r.source(entry.sources[position], selection.Parameters)
		if err != nil {
			return nil, err
		}
		if source.nested == nil && !slices.Contains(CandleFields, source.field) {
			return nil, fmt.Errorf("%w: indicator %q reads %s, which names no candle field", ErrInvalidResult, selection.Type, name)
		}
		source.name = name
		sources[index] = source
	}
	return sources, nil
}

// source validates the value of the source parameter in parameters, its
// default when left out: one of its choices, or a nested source, the
// non-sparse output of an indicator clients can configure that reads only
// candle fields itself.
func (r *Registry) source(parameter ParameterDescriptor, parameters Parameters) (inputSource, error) {
	raw, exists := parameters[parameter.Key]
	if !exists {
		raw = parameter.Default
	}
	number, nested, err := ParseSource(raw)
	if err != nil {
		return inputSource{}, fmt.Errorf("%w: %s %w", ErrInvalidRequest, parameter.Key, err)
	}
	if nested == nil {
		position := slices.IndexFunc(parameter.Choices, func(choice Choice) bool { return float64(choice.Value) == number })
		if position < 0 {
			return inputSource{}, fmt.Errorf("%w: %s must be one of its choices or an indicator output", ErrInvalidRequest, parameter.Key)
		}
		choice := parameter.Choices[position]
		return inputSource{field: choice.Name, value: choice.Value}, nil
	}
	entry, err := r.entry(nested.Selection.Type)
	if err != nil || entry.descriptor.Internal {
		return inputSource{}, fmt.Errorf("%w: %s reads unknown indicator %q", ErrInvalidRequest, parameter.Key, nested.Selection.Type)
	}
	if sparse, declared := entry.sparse[nested.Output]; !declared || sparse {
		return inputSource{}, fmt.Errorf("%w: %s reads %q of %s, which is not an output with a value on every candle", ErrInvalidRequest, parameter.Key, nested.Output, nested.Selection.Type)
	}
	selection, err := r.Normalize(nested.Selection)
	if err != nil {
		return inputSource{}, fmt.Errorf("%w: %s: %w", ErrInvalidRequest, parameter.Key, err)
	}
	for _, inner := range entry.sources {
		if _, deeper := selection.Parameters[inner.Key].(map[string]any); deeper {
			return inputSource{}, fmt.Errorf("%w: %s reads %s, whose own %s must be a candle field", ErrInvalidRequest, parameter.Key, nested.Selection.Type, inner.Key)
		}
	}
	source := Source{Selection: selection, Output: nested.Output}
	return inputSource{nested: &source, value: source.Value()}, nil
}

// Outputs returns the named series produced by the selected module.
func (r *Registry) Outputs(indicatorType Type) ([]string, error) {
	entry, err := r.entry(indicatorType)
	if err != nil {
		return nil, err
	}
	return slices.Clone(entry.outputs), nil
}

// Normalize returns the selection with canonical parameters, so selections
// with equal settings compare and deduplicate equally.
func (r *Registry) Normalize(selection Selection) (Selection, error) {
	entry, err := r.entry(selection.Type)
	if err != nil {
		return Selection{}, err
	}
	parameters, err := entry.implementation.Normalize(selection.Parameters)
	if err != nil {
		return Selection{}, err
	}
	if len(entry.sources) > 0 {
		// The implementation may have returned the parameters it was given.
		parameters = maps.Clone(parameters)
		if parameters == nil {
			parameters = Parameters{}
		}
		for _, parameter := range entry.sources {
			source, err := r.source(parameter, selection.Parameters)
			if err != nil {
				return Selection{}, err
			}
			parameters[parameter.Key] = source.value
		}
	}
	return Selection{Type: selection.Type, Parameters: parameters}, nil
}

// Describe returns the self-description of the selected module.
func (r *Registry) Describe(indicatorType Type) (Descriptor, error) {
	entry, err := r.entry(indicatorType)
	if err != nil {
		return Descriptor{}, err
	}
	return entry.implementation.Describe(), nil
}

// Descriptors describes every module clients can configure, ordered by
// type. Internal modules are left out.
func (r *Registry) Descriptors() []Descriptor {
	if r == nil {
		return nil
	}
	result := make([]Descriptor, 0, len(r.implementations))
	for _, entry := range r.implementations {
		if entry.descriptor.Internal {
			continue
		}
		// Describe returns fresh slices, so callers cannot change the
		// registered inputs.
		result = append(result, entry.implementation.Describe())
	}
	slices.SortFunc(result, func(left, right Descriptor) int { return strings.Compare(string(left.Type), string(right.Type)) })
	return result
}

// Calculate dispatches a request and validates the implementation's result.
// The inputs are the candle fields Fields names. A nested source is
// calculated first over the same inputs and its values become the source
// series, so the outputs start that many values later.
func (r *Registry) Calculate(request Request) (Result, error) {
	entry, err := r.entry(request.Type)
	if err != nil {
		return Result{}, err
	}
	sources, err := r.inputSources(entry, Selection{Type: request.Type, Parameters: request.Parameters})
	if err != nil {
		return Result{}, err
	}
	inputs := make(Inputs, len(sources))
	// starts are the positions of the first values of the nested series,
	// and offset the latest of them, where every series has a value.
	starts, offset := map[string]int{}, 0
	for _, source := range sources {
		if source.nested == nil {
			if values, exists := request.Inputs[source.field]; exists {
				inputs[source.name] = values
			}
			continue
		}
		nested, err := r.Calculate(Request{Type: source.nested.Selection.Type, Parameters: source.nested.Selection.Parameters, Inputs: request.Inputs})
		if err != nil {
			return Result{}, err
		}
		series := nested.Outputs[source.nested.Output]
		inputs[source.name], starts[source.name] = series.Values, series.Offset
		offset = max(offset, series.Offset)
	}
	if offset > 0 {
		for name, values := range inputs {
			inputs[name] = values[min(offset-starts[name], len(values)):]
		}
	}

	result, err := entry.implementation.Calculate(request.Parameters, inputs)
	if err != nil {
		return Result{}, err
	}
	if err := validateResult(result, entry.outputs, entry.sparse); err != nil {
		return Result{}, fmt.Errorf("%w: indicator %q: %v", ErrInvalidResult, request.Type, err)
	}
	if offset > 0 {
		for name, series := range result.Outputs {
			series.Offset += offset
			result.Outputs[name] = series
		}
	}

	return result, nil
}

func (r *Registry) entry(indicatorType Type) (registered, error) {
	if r == nil {
		return registered{}, fmt.Errorf("%w: %q", ErrUnknownType, indicatorType)
	}
	entry, exists := r.implementations[indicatorType]
	if !exists {
		return registered{}, fmt.Errorf("%w: %q", ErrUnknownType, indicatorType)
	}
	return entry, nil
}

func validateOutputs(outputs []string) error {
	if len(outputs) == 0 {
		return errors.New("no declared outputs")
	}
	seen := map[string]bool{}
	for _, name := range outputs {
		if strings.TrimSpace(name) == "" || seen[name] {
			return fmt.Errorf("output %q is empty or duplicated", name)
		}
		seen[name] = true
	}
	return nil
}

// validateResult checks the common contract, including that the result
// contains exactly the declared outputs clients are told to expect. Only the
// outputs in sparse may hold empty values.
func validateResult(result Result, outputs []string, sparse map[string]bool) error {
	if len(result.Outputs) != len(outputs) {
		return fmt.Errorf("returned %d output series, declared %d", len(result.Outputs), len(outputs))
	}
	for _, name := range outputs {
		if _, ok := result.Outputs[name]; !ok {
			return fmt.Errorf("declared output %q is missing", name)
		}
	}

	for name, series := range result.Outputs {
		if strings.TrimSpace(name) == "" {
			return errors.New("output name is empty")
		}
		if series.Offset < 0 {
			return fmt.Errorf("output %q has negative offset %d", name, series.Offset)
		}
		if series.Values == nil {
			return fmt.Errorf("output %q has nil values", name)
		}
		for index, value := range series.Values {
			if !numeric.Finite(value) && !(sparse[name] && Empty(value)) {
				return fmt.Errorf("output %q value %d is not finite", name, index)
			}
		}
	}

	return nil
}
