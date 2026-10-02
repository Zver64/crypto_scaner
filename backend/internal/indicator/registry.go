package indicator

import (
	"errors"
	"fmt"
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
		for position, output := range descriptor.Outputs {
			outputs[position] = output.Name
		}
		if err := validateOutputs(outputs); err != nil {
			return nil, fmt.Errorf("%w: %q: %v", ErrEmptyRegistration, indicatorType, err)
		}
		byType[indicatorType] = registered{implementation: implementation, descriptor: descriptor, outputs: outputs}
	}

	return &Registry{implementations: byType}, nil
}

// Lookback reports the number of preceding input values required by the
// selected implementation. A negative value is an invalid implementation
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

	return lookback, nil
}

// Fields returns the candle fields the selection reads.
func (r *Registry) Fields(selection Selection) ([]string, error) {
	entry, err := r.entry(selection.Type)
	if err != nil {
		return nil, err
	}
	fields, err := entry.implementation.Fields(selection.Parameters)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		if !slices.Contains(CandleFields, field) {
			return nil, fmt.Errorf("%w: indicator %q reads unsupported field %q", ErrInvalidResult, selection.Type, field)
		}
	}
	return fields, nil
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
func (r *Registry) Calculate(request Request) (Result, error) {
	entry, err := r.entry(request.Type)
	if err != nil {
		return Result{}, err
	}

	result, err := entry.implementation.Calculate(request.Parameters, request.Inputs)
	if err != nil {
		return Result{}, err
	}
	if err := validateResult(result, entry.outputs); err != nil {
		return Result{}, fmt.Errorf("%w: indicator %q: %v", ErrInvalidResult, request.Type, err)
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
// contains exactly the declared outputs clients are told to expect.
func validateResult(result Result, outputs []string) error {
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
			if !numeric.Finite(value) {
				return fmt.Errorf("output %q value %d is not finite", name, index)
			}
		}
	}

	return nil
}
