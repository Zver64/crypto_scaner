package strategy

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/candle"
	"crypto-scanner/internal/market"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/types"
)

// CandleTarget is the closed-candle calculation of the candle fields of
// interval.
func CandleTarget(interval market.CandleInterval) closedindicator.Target {
	return closedindicator.Target{Interval: interval, Selection: indicator.Selection{Type: candle.Type, Parameters: indicator.Parameters{}}}
}

// call is an indicator or candle field call of a source, such as
// bbands_upperband("1h", 20, 2.5) or close("1d").
type call struct {
	target closedindicator.Target
	// spelling is the canonical call.
	spelling string
	// output is the output read; candle marks a candle field call.
	output string
	candle bool
	// source marks a call written as the source of another call, which the
	// expression does not read by itself.
	source bool
	// start and end delimit the call in source, in code points, end after
	// its closing parenthesis.
	start, end int
}

// variable is the variable the call reads.
func (c call) variable() Variable {
	return Variable{Name: c.spelling, Target: c.target, Output: c.output}
}

// indicatorFunction names the function that reads output of an indicator
// of indicatorType with outputs outputs: the type, or for an indicator with
// several outputs the type and the output, such as bbands_upperband.
func indicatorFunction(indicatorType indicator.Type, outputs int, output string) string {
	if outputs > 1 {
		return string(indicatorType) + "_" + output
	}
	return string(indicatorType)
}

// outputFunctions lists the functions that read the outputs of the
// indicator descriptor describes.
func outputFunctions(descriptor indicator.Descriptor) []string {
	names := make([]string, len(descriptor.Outputs))
	for i, output := range descriptor.Outputs {
		names[i] = indicatorFunction(descriptor.Type, len(descriptor.Outputs), output.Name)
	}
	return names
}

// resolveCandleCall resolves the call of the candle field name, such as
// close("1h"), with args: the period only.
func resolveCandleCall(name string, args []ast.Expr) (call, string) {
	interval, problem := callInterval(name, args)
	if problem != "" {
		return call{}, problem
	}
	if len(args) > 1 {
		return call{}, fmt.Sprintf("%s takes only the period", name)
	}
	return call{target: CandleTarget(interval), spelling: fmt.Sprintf("%s(%q)", name, interval), output: name, candle: true}, ""
}

// resolveCall resolves the call of output of the indicator descriptor
// describes with args: the period, then the parameters in descriptor order,
// of which trailing ones may be left out. A source may be an earlier call of
// the language, which is marked as one. problem explains a call that does
// not resolve.
func (language *language) resolveCall(descriptor indicator.Descriptor, output indicator.OutputDescriptor, args []ast.Expr) (resolved call, problem string) {
	registry := language.registry
	name := indicatorFunction(descriptor.Type, len(descriptor.Outputs), output.Name)
	interval, problem := callInterval(name, args)
	if problem != "" {
		return call{}, problem
	}
	if len(args)-1 > len(descriptor.Parameters) {
		return call{}, fmt.Sprintf("%s takes the period and at most %d parameters", name, len(descriptor.Parameters))
	}
	parameters := indicator.Parameters{}
	var sources []int
	for i, arg := range args[1:] {
		parameter := descriptor.Parameters[i]
		if parameter.Kind == indicator.ParameterSource {
			value, source, ok := language.sourceValue(parameter, arg, interval)
			if !ok {
				return call{}, fmt.Sprintf("the %s of %s is a candle field (%s) or an indicator call of the same period", parameter.Key, name, strings.Join(choiceNames(parameter), ", "))
			}
			if source >= 0 {
				sources = append(sources, source)
			}
			parameters[parameter.Key] = value
			continue
		}
		value, ok := numberValue(arg)
		if !ok {
			return call{}, fmt.Sprintf("the %s of %s is a number", parameter.Key, name)
		}
		parameters[parameter.Key] = value
	}
	selection, err := registry.Normalize(indicator.Selection{Type: descriptor.Type, Parameters: parameters})
	if err != nil {
		return call{}, fmt.Sprintf("%s: %v", name, err)
	}
	arguments, err := callArguments(registry, selection, interval)
	if err != nil {
		return call{}, fmt.Sprintf("%s: %v", name, err)
	}
	for _, source := range sources {
		language.calls[source].source = true
	}
	target := closedindicator.Target{Interval: interval, Selection: selection}
	return call{target: target, spelling: spellCall(name, interval, arguments), output: output.Name}, ""
}

// callInterval reads the period, the first argument of a call of name.
func callInterval(name string, args []ast.Expr) (market.CandleInterval, string) {
	if len(args) == 0 {
		return "", fmt.Sprintf(`%s takes the period first, such as %s("1h")`, name, name)
	}
	var interval market.CandleInterval
	if args[0].Kind() == ast.LiteralKind {
		if value, ok := args[0].AsLiteral().(types.String); ok {
			interval = market.CandleInterval(value)
		}
	}
	if !interval.Valid() {
		return "", fmt.Sprintf(`the period of %s is "1h", "1d", "1w", or "1M"`, name)
	}
	return interval, ""
}

// sourceValue reads the argument of a source parameter of a call on
// interval: the name of a candle field, such as volume, or an indicator call
// of the same interval, which CEL has already resolved into the identifier
// of that call. call is its index among the calls, -1 for a candle field.
func (language *language) sourceValue(parameter indicator.ParameterDescriptor, arg ast.Expr, interval market.CandleInterval) (value any, call int, ok bool) {
	if arg.Kind() != ast.IdentKind {
		return nil, -1, false
	}
	if index, isCall := strings.CutPrefix(arg.AsIdent(), callMarker); isCall {
		number, err := strconv.Atoi(index)
		if err != nil || number >= len(language.calls) {
			return nil, -1, false
		}
		inner := language.calls[number]
		if inner.candle || inner.target.Interval != interval {
			return nil, -1, false
		}
		return indicator.Source{Selection: inner.target.Selection, Output: inner.output}.Value(), number, true
	}
	index := slices.IndexFunc(parameter.Choices, func(choice indicator.Choice) bool { return choice.Name != "" && choice.Name == arg.AsIdent() })
	if index < 0 {
		return nil, -1, false
	}
	return parameter.Choices[index].Value, -1, true
}

// choiceNames lists the names calls write for the choices of parameter; a
// choice without one is only the default.
func choiceNames(parameter indicator.ParameterDescriptor) []string {
	var names []string
	for _, choice := range parameter.Choices {
		if choice.Name != "" {
			names = append(names, choice.Name)
		}
	}
	return names
}

// numberValue reads the argument of a parameter that is a number.
func numberValue(arg ast.Expr) (float64, bool) {
	if arg.Kind() != ast.LiteralKind {
		return 0, false
	}
	switch literal := arg.AsLiteral().(type) {
	case types.Int:
		return float64(literal), true
	case types.Uint:
		return float64(literal), true
	case types.Double:
		return float64(literal), true
	}
	return 0, false
}

// callArguments spells the parameters of selection, which must be
// normalized, as calls on interval write them: in descriptor order up to the
// last one that differs from its default, candle fields by their name and a
// nested source as its own canonical call, such as ["20", "volume"],
// ["20", "2.5"], or ["9", `rsi("1h")`].
func callArguments(registry *indicator.Registry, selection indicator.Selection, interval market.CandleInterval) ([]string, error) {
	descriptor, err := registry.Describe(selection.Type)
	if err != nil {
		return nil, err
	}
	defaults, err := registry.Normalize(indicator.Selection{Type: selection.Type})
	if err != nil {
		return nil, err
	}
	arguments := make([]string, len(descriptor.Parameters))
	last := -1
	for i, parameter := range descriptor.Parameters {
		if arguments[i], err = spellParameter(registry, parameter, selection.Parameters[parameter.Key], interval); err != nil {
			return nil, err
		}
		standard, err := spellParameter(registry, parameter, defaults.Parameters[parameter.Key], interval)
		if err != nil {
			return nil, err
		}
		if arguments[i] != standard {
			last = i
		}
	}
	return arguments[:last+1], nil
}

// spellCall writes the canonical call of the indicator function name, such
// as bbands_upperband("1h", 20, 2.5).
func spellCall(name string, interval market.CandleInterval, arguments []string) string {
	return name + "(" + strings.Join(append([]string{strconv.Quote(string(interval))}, arguments...), ", ") + ")"
}

// spellParameter writes a parameter value: the name of a candle field, the
// canonical call on interval of a nested source, or the number without
// trailing zeros.
func spellParameter(registry *indicator.Registry, parameter indicator.ParameterDescriptor, value any, interval market.CandleInterval) (string, error) {
	if parameter.Kind == indicator.ParameterSource {
		number, nested, err := indicator.ParseSource(value)
		if err != nil {
			return "", fmt.Errorf("%s %w", parameter.Key, err)
		}
		if nested != nil {
			descriptor, err := registry.Describe(nested.Selection.Type)
			if err != nil {
				return "", err
			}
			arguments, err := callArguments(registry, nested.Selection, interval)
			if err != nil {
				return "", err
			}
			return spellCall(indicatorFunction(descriptor.Type, len(descriptor.Outputs), nested.Output), interval, arguments), nil
		}
		for _, choice := range parameter.Choices {
			if float64(choice.Value) == number {
				return choice.Name, nil
			}
		}
	}
	number, err := indicator.ParseNumber(value)
	if err != nil {
		return fmt.Sprint(value), nil
	}
	return strconv.FormatFloat(number, 'f', -1, 64), nil
}

// nestedSource reports whether selection, which must be normalized, reads
// another indicator as a source.
func nestedSource(selection indicator.Selection) bool {
	for _, value := range selection.Parameters {
		if _, nested := value.(map[string]any); nested {
			return true
		}
	}
	return false
}

// callSpan finds the call whose opening parenthesis is at open in source:
// from the start of its name to after its closing parenthesis, the one that
// matches it outside quotes: a source may be a call itself.
func callSpan(source []rune, open int) (start, end int) {
	start = open
	for start > 0 && isSpace(source[start-1]) {
		start--
	}
	for start > 0 && isNameRune(source[start-1]) {
		start--
	}
	var quote rune
	depth := 1
	for end = open + 1; end < len(source); end++ {
		switch character := source[end]; {
		case quote != 0 && character == '\\':
			end++
		case quote != 0:
			if character == quote {
				quote = 0
			}
		case character == '"' || character == '\'':
			quote = character
		case character == '(':
			depth++
		case character == ')':
			if depth--; depth == 0 {
				return start, end + 1
			}
		}
	}
	return start, len(source)
}

func isSpace(character rune) bool {
	return character == ' ' || character == '\t' || character == '\n' || character == '\r'
}

func isNameRune(character rune) bool {
	return character == '_' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9'
}

// replacement replaces source between start and end, in code points.
type replacement struct {
	start, end int
	text       string
}

// replace applies replacements, which must not overlap, to source.
func replace(source string, replacements []replacement) string {
	runes := []rune(source)
	slices.SortFunc(replacements, func(a, b replacement) int { return a.start - b.start })
	var result strings.Builder
	at := 0
	for _, item := range replacements {
		if item.start < at {
			continue
		}
		result.WriteString(string(runes[at:item.start]))
		result.WriteString(item.text)
		at = item.end
	}
	result.WriteString(string(runes[at:]))
	return result.String()
}
