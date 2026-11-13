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

// candleFields are the candle fields expressions read as calls of their
// name, such as close("1h"), with the output each reads.
var candleFields = []struct{ name, output string }{
	{"open", "open"}, {"high", "high"}, {"low", "low"}, {"close", "close"},
	{"volume", "volume"}, {"quote_volume", "quote_asset_volume"}, {"trades", "trade_count"},
}

// CandleFields names the candle fields expressions read, such as close.
func CandleFields() []string {
	names := make([]string, len(candleFields))
	for i, field := range candleFields {
		names[i] = field.name
	}
	return names
}

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
	// output is the output read, which count marks as a count of candles;
	// candle marks a candle field call.
	output        string
	count, candle bool
	// start and end delimit the call in source, in code points, end after
	// its closing parenthesis.
	start, end int
}

// variable is the variable the call reads.
func (c call) variable() Variable {
	return Variable{Name: c.spelling, Target: c.target, Output: c.output, Count: c.count}
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

// resolveCandleCall resolves the call of the candle field name with args:
// the period only.
func resolveCandleCall(name, output string, args []ast.Expr) (call, string) {
	interval, problem := callInterval(name, args)
	if problem != "" {
		return call{}, problem
	}
	if len(args) > 1 {
		return call{}, fmt.Sprintf("%s takes only the period", name)
	}
	return call{target: CandleTarget(interval), spelling: fmt.Sprintf("%s(%q)", name, interval), output: output, candle: true}, ""
}

// resolveCall resolves the call of output of the indicator descriptor
// describes with args: the period, then the parameters in descriptor order,
// of which trailing ones may be left out. problem explains a call that does
// not resolve.
func resolveCall(registry *indicator.Registry, descriptor indicator.Descriptor, output indicator.OutputDescriptor, args []ast.Expr) (resolved call, problem string) {
	name := indicatorFunction(descriptor.Type, len(descriptor.Outputs), output.Name)
	interval, problem := callInterval(name, args)
	if problem != "" {
		return call{}, problem
	}
	if len(args)-1 > len(descriptor.Parameters) {
		return call{}, fmt.Sprintf("%s takes the period and at most %d parameters", name, len(descriptor.Parameters))
	}
	parameters := indicator.Parameters{}
	for i, arg := range args[1:] {
		parameter := descriptor.Parameters[i]
		value, ok := parameterValue(parameter, arg)
		if !ok {
			if namedChoice(parameter) {
				return call{}, fmt.Sprintf("the %s of %s is one of %s", parameter.Key, name, strings.Join(choiceNames(parameter), ", "))
			}
			return call{}, fmt.Sprintf("the %s of %s is a number", parameter.Key, name)
		}
		parameters[parameter.Key] = value
	}
	selection, err := registry.Normalize(indicator.Selection{Type: descriptor.Type, Parameters: parameters})
	if err != nil {
		return call{}, fmt.Sprintf("%s: %v", name, err)
	}
	arguments, err := callArguments(registry, selection)
	if err != nil {
		return call{}, fmt.Sprintf("%s: %v", name, err)
	}
	target := closedindicator.Target{Interval: interval, Selection: selection}
	return call{target: target, spelling: spellCall(name, interval, arguments), output: output.Name, count: output.Count}, ""
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

// parameterValue reads a parameter argument: the name of a named choice,
// such as volume, or a number for any other parameter.
func parameterValue(parameter indicator.ParameterDescriptor, arg ast.Expr) (float64, bool) {
	if namedChoice(parameter) {
		if arg.Kind() != ast.IdentKind {
			return 0, false
		}
		index := slices.IndexFunc(parameter.Choices, func(choice indicator.Choice) bool { return choice.Name == arg.AsIdent() })
		if index < 0 {
			return 0, false
		}
		return float64(parameter.Choices[index].Value), true
	}
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
// normalized, as calls write them: in descriptor order up to the last one
// that differs from its default, named choices by their name, such as
// ["20", "volume"] or ["20", "2.5"].
func callArguments(registry *indicator.Registry, selection indicator.Selection) ([]string, error) {
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
		arguments[i] = spellParameter(parameter, selection.Parameters[parameter.Key])
		if arguments[i] != spellParameter(parameter, defaults.Parameters[parameter.Key]) {
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

// spellParameter writes a parameter value: the name of a named choice, or
// the number without trailing zeros.
func spellParameter(parameter indicator.ParameterDescriptor, value any) string {
	number, err := indicator.ParseNumber(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	if namedChoice(parameter) {
		for _, choice := range parameter.Choices {
			if float64(choice.Value) == number {
				return choice.Name
			}
		}
	}
	return strconv.FormatFloat(number, 'f', -1, 64)
}

// namedChoice reports whether parameter chooses among named values, such as
// candle fields, which calls write by name.
func namedChoice(parameter indicator.ParameterDescriptor) bool {
	return parameter.Kind == indicator.ParameterChoice && len(parameter.Choices) > 0 && parameter.Choices[0].Name != ""
}

func choiceNames(parameter indicator.ParameterDescriptor) []string {
	names := make([]string, len(parameter.Choices))
	for i, choice := range parameter.Choices {
		names[i] = choice.Name
	}
	return names
}

// callSpan finds the call whose opening parenthesis is at open in source:
// from the start of its name to after its closing parenthesis. Calls hold
// only literals and names, so the first closing parenthesis outside quotes
// ends it.
func callSpan(source []rune, open int) (start, end int) {
	start = open
	for start > 0 && isSpace(source[start-1]) {
		start--
	}
	for start > 0 && isNameRune(source[start-1]) {
		start--
	}
	var quote rune
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
		case character == ')':
			return start, end + 1
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
