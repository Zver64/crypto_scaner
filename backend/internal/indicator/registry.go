package indicator

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
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
	implementations map[Type]Implementation
}

// NewRegistry creates a calculator from concrete implementations. At least one
// non-nil implementation is required, and each non-empty type may occur once.
func NewRegistry(implementations ...Implementation) (*Registry, error) {
	if len(implementations) == 0 {
		return nil, ErrEmptyRegistration
	}

	registered := make(map[Type]Implementation, len(implementations))
	for index, implementation := range implementations {
		if isNilImplementation(implementation) {
			return nil, fmt.Errorf("%w at index %d", ErrEmptyRegistration, index)
		}

		indicatorType := implementation.Type()
		if strings.TrimSpace(string(indicatorType)) == "" {
			return nil, fmt.Errorf("%w at index %d: type is empty", ErrEmptyRegistration, index)
		}
		if _, exists := registered[indicatorType]; exists {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateRegistration, indicatorType)
		}
		registered[indicatorType] = implementation
	}

	return &Registry{implementations: registered}, nil
}

// Lookback reports the number of preceding input values required by the
// selected implementation. A negative value is an invalid implementation
// result.
func (r *Registry) Lookback(indicatorType Type, parameters Parameters) (int, error) {
	implementation, err := r.implementation(indicatorType)
	if err != nil {
		return 0, err
	}

	lookback, err := implementation.Lookback(parameters)
	if err != nil {
		return 0, err
	}
	if lookback < 0 {
		return 0, fmt.Errorf("%w: indicator %q returned negative lookback %d", ErrInvalidResult, indicatorType, lookback)
	}

	return lookback, nil
}

// Inputs returns the named candle fields required by the selected module.
func (r *Registry) Inputs(indicatorType Type) ([]string, error) {
	implementation, err := r.implementation(indicatorType)
	if err != nil {
		return nil, err
	}
	return append([]string(nil), implementation.Inputs()...), nil
}

// Calculate dispatches a request and validates the implementation's result.
func (r *Registry) Calculate(request Request) (Result, error) {
	implementation, err := r.implementation(request.Type)
	if err != nil {
		return Result{}, err
	}

	result, err := implementation.Calculate(request.Parameters, request.Inputs)
	if err != nil {
		return Result{}, err
	}
	if err := validateResult(result); err != nil {
		return Result{}, fmt.Errorf("%w: indicator %q: %v", ErrInvalidResult, request.Type, err)
	}

	return result, nil
}

func (r *Registry) implementation(indicatorType Type) (Implementation, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: %q", ErrUnknownType, indicatorType)
	}
	implementation, exists := r.implementations[indicatorType]
	if !exists {
		return nil, fmt.Errorf("%w: %q", ErrUnknownType, indicatorType)
	}
	return implementation, nil
}

func validateResult(result Result) error {
	if len(result.Outputs) == 0 {
		return errors.New("no output series")
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
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("output %q value %d is not finite", name, index)
			}
		}
	}

	return nil
}

func isNilImplementation(implementation Implementation) bool {
	if implementation == nil {
		return true
	}

	value := reflect.ValueOf(implementation)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
