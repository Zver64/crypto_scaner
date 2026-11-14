package indicator

import (
	"errors"
	"fmt"

	"crypto-scanner/internal/platform/numeric"
)

// ParseNumber reads a parameter value supplied as any Go number. Modules wrap
// its error in ErrInvalidRequest.
func ParseNumber(raw any) (float64, error) {
	var value float64
	switch typed := raw.(type) {
	case int:
		value = float64(typed)
	case int32:
		value = float64(typed)
	case int64:
		value = float64(typed)
	case float64:
		value = typed
	default:
		return 0, errors.New("must be a number")
	}
	if !numeric.Finite(value) {
		return 0, errors.New("must be finite")
	}
	return value, nil
}

// Source is an output of another indicator read as the source of an
// indicator: the nested value of a ParameterSource parameter.
type Source struct {
	Selection Selection
	Output    string
}

// Value is the source as a parameter value, which JSON writes as
// {"type", "parameters", "output"}.
func (source Source) Value() map[string]any {
	return map[string]any{
		"type":       string(source.Selection.Type),
		"parameters": map[string]any(source.Selection.Parameters),
		"output":     source.Output,
	}
}

// ParseSource reads the value of a ParameterSource parameter: a number, the
// value of one of its choices, or a nested source, as Value builds it or
// JSON decodes it. nested is nil for a number.
func ParseSource(raw any) (choice float64, nested *Source, err error) {
	value, ok := raw.(map[string]any)
	if !ok {
		choice, err = ParseNumber(raw)
		if err != nil {
			return 0, nil, errors.New("must be a number or an indicator output")
		}
		return choice, nil, nil
	}
	source := Source{}
	for key, item := range value {
		ok := true
		switch key {
		case "type":
			var name string
			name, ok = item.(string)
			source.Selection.Type = Type(name)
		case "parameters":
			switch parameters := item.(type) {
			case nil:
			case map[string]any:
				source.Selection.Parameters = parameters
			case Parameters:
				source.Selection.Parameters = parameters
			default:
				ok = false
			}
		case "output":
			source.Output, ok = item.(string)
		default:
			ok = false
		}
		if !ok {
			return 0, nil, fmt.Errorf("has an unsupported %q", key)
		}
	}
	return 0, &source, nil
}

// ReadInputs returns the named input series in names order, checking that
// each is present, finite, and as long as the others. Modules wrap its error
// in ErrInvalidRequest.
func ReadInputs(inputs Inputs, names []string) ([][]float64, error) {
	result := make([][]float64, len(names))
	for index, name := range names {
		values, exists := inputs[name]
		if !exists {
			return nil, fmt.Errorf("%s input is required", name)
		}
		if index > 0 && len(values) != len(result[0]) {
			return nil, fmt.Errorf("%s has %d values, want %d", name, len(values), len(result[0]))
		}
		for position, value := range values {
			if !numeric.Finite(value) {
				return nil, fmt.Errorf("%s value %d is not finite", name, position)
			}
		}
		result[index] = values
	}
	return result, nil
}
