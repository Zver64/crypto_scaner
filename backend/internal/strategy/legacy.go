package strategy

// This file rewrites rules of the syntax before indicator calls, which named
// variables after table column titles, such as
// crosses_above(h_close, prev(h_max_12_high, 1)), into calls, such as
// CrossesAbove(close("1h"), Prev(max("1h", 12, high), 1)). It serves the
// one-time rewrite of the stored rules at startup and goes once that has run
// in production.

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
)

// legacyFunctions maps the functions of the old syntax to the new ones.
var legacyFunctions = map[string]string{
	"prev": "Prev", "percentile": "Percentile", "of": "Of", "crosses_above": "CrossesAbove", "crosses_below": "CrossesBelow",
	"abs": "Abs", "mod": "Mod", "min": "Min", "max": "Max",
}

// rewriteLegacyStrategy rewrites the rules of item, stored in the old
// syntax. A rule that cannot be rewritten stays as it is and is logged.
func (service *Service) rewriteLegacyStrategy(ctx context.Context, item Strategy) Strategy {
	for _, rule := range []struct {
		name   string
		source *string
	}{
		{"entry rule", &item.Expression}, {"exit rule", &item.ExitExpression},
		{"take profit", &item.TakeProfitExpression}, {"stop loss", &item.StopLossExpression},
	} {
		if strings.TrimSpace(*rule.source) == "" {
			continue
		}
		rewritten, err := service.rewriteLegacy(*rule.source)
		if err != nil {
			service.logger.WarnContext(ctx, "strategy rule left in the old syntax", "strategy_id", item.ID, "strategy", item.Name, "rule", rule.name, "error", err)
			continue
		}
		*rule.source = rewritten
	}
	return item
}

// rewriteLegacy rewrites source from the old syntax: each variable name
// becomes its call and each function its capitalized name; the rest stays
// as written.
func (service *Service) rewriteLegacy(source string) (string, error) {
	env, err := cel.NewEnv(cel.ClearMacros())
	if err != nil {
		return "", err
	}
	parsed, issues := env.Parse(source)
	if issues.Err() != nil {
		return "", errors.New(strings.Join(issueMessages(issues), "; "))
	}
	text := common.NewTextSource(source)
	runes := []rune(source)
	info := parsed.NativeRep().SourceInfo()
	var replacements []replacement
	var problems []string
	offset := func(id int64) int {
		at, _ := text.LocationOffset(info.GetStartLocation(id))
		return int(at)
	}
	ast.PreOrderVisit(parsed.NativeRep().Expr(), ast.NewExprVisitor(func(node ast.Expr) {
		switch node.Kind() {
		case ast.IdentKind:
			name := node.AsIdent()
			if slices.ContainsFunc(PositionVariables(), func(variable Variable) bool { return variable.Name == name }) {
				return
			}
			spelled, err := service.legacyVariable(name)
			if err != nil {
				problems = append(problems, err.Error())
				return
			}
			start := offset(node.ID())
			replacements = append(replacements, replacement{start: start, end: start + len([]rune(name)), text: spelled})
		case ast.CallKind:
			call := node.AsCall()
			if _, operator := operators.FindReverse(call.FunctionName()); operator && !call.IsMemberFunction() {
				return
			}
			renamed, ok := legacyFunctions[call.FunctionName()]
			if !ok || call.IsMemberFunction() {
				problems = append(problems, "unknown function "+call.FunctionName())
				return
			}
			start, _ := callSpan(runes, offset(node.ID()))
			replacements = append(replacements, replacement{start: start, end: start + len([]rune(call.FunctionName())), text: renamed})
		case ast.SelectKind:
			problems = append(problems, "unsupported field "+node.AsSelect().FieldName())
		}
	}))
	if len(problems) > 0 {
		return "", errors.New(strings.Join(problems, "; "))
	}
	return replace(source, replacements), nil
}

// legacyVariable spells the call of a variable name of the old syntax,
// such as close("1h") for h_close or bbands_upperband("1h", 20, 2.5) for
// h_bbands_20_2p5_upperband.
func (service *Service) legacyVariable(name string) (string, error) {
	for _, interval := range market.CandleIntervals() {
		for _, field := range candleFields {
			if name == scannerindicator.IntervalPrefix(interval)+"_"+field.name {
				return fmt.Sprintf("%s(%q)", field.name, interval), nil
			}
		}
	}
	var spelled []string
	for _, match := range service.legacyIndicatorsNamed(name) {
		arguments, err := callArguments(service.registry, match.entry.Selection)
		if err != nil {
			return "", err
		}
		call := spellCall(indicatorFunction(match.entry.Selection.Type, len(match.entry.Outputs), match.output), match.interval, arguments)
		if !slices.Contains(spelled, call) {
			spelled = append(spelled, call)
		}
	}
	switch len(spelled) {
	case 0:
		return "", fmt.Errorf("unknown name %s", name)
	case 1:
		return spelled[0], nil
	}
	return "", fmt.Errorf("%s may be %s", name, strings.Join(spelled, " or "))
}

// legacyMatch is an output of an indicator on an interval whose old
// variable name is the one looked up.
type legacyMatch struct {
	entry    scannerindicator.Entry
	interval market.CandleInterval
	output   string
}

// spelledNumber is a parameter value as old variable names spell it, such
// as "_20" or "_2p5".
var spelledNumber = regexp.MustCompile(`^_([0-9]+(?:p[0-9]+)?)(?:_|$)`)

// legacyIndicatorsNamed lists the outputs whose old variable would be name.
// Names spell the interval, the type, and the parameter values of the
// title; every way of reading the values is tried and kept only when the
// title it gives spells name back.
func (service *Service) legacyIndicatorsNamed(name string) []legacyMatch {
	var found []legacyMatch
	for _, interval := range market.CandleIntervals() {
		rest, ok := strings.CutPrefix(name, scannerindicator.IntervalPrefix(interval)+"_")
		if !ok {
			continue
		}
		for _, descriptor := range service.registry.Descriptors() {
			tail, ok := strings.CutPrefix(rest, string(descriptor.Type))
			if !ok || tail != "" && tail[0] != '_' {
				continue
			}
			for _, parameters := range parameterReadings(descriptor.Parameters, tail, indicator.Parameters{}) {
				entry, err := service.indicators.Preview(indicator.Selection{Type: descriptor.Type, Parameters: parameters})
				if err != nil {
					continue
				}
				for _, output := range entry.Outputs {
					if legacyOutputName(entry, interval, output) == name {
						found = append(found, legacyMatch{entry: entry, interval: interval, output: output})
					}
				}
			}
		}
	}
	return found
}

var invalidNameCharacters = regexp.MustCompile(`[^A-Za-z0-9_]`)

// legacyOutputName is the old variable name of one output of entry on
// interval: the table column title, followed by the output name when the
// indicator has several. Decimal points became "p" and other characters
// identifiers cannot hold "_", so "h-bbands-20-2.5" was h_bbands_20_2p5.
func legacyOutputName(entry scannerindicator.Entry, interval market.CandleInterval, output string) string {
	spell := func(title string) string {
		return invalidNameCharacters.ReplaceAllString(strings.ReplaceAll(title, ".", "p"), "_")
	}
	name := spell(entry.ColumnTitle(interval))
	if len(entry.Outputs) > 1 {
		name += "_" + spell(output)
	}
	return name
}

// parameterReadings lists the parameter values tail may spell, parameter by
// parameter in order: each one is either spelled or left at its default. The
// rest of tail may name an output, so every reading is listed.
func parameterReadings(parameters []indicator.ParameterDescriptor, tail string, chosen indicator.Parameters) []indicator.Parameters {
	if len(parameters) == 0 {
		return []indicator.Parameters{maps.Clone(chosen)}
	}
	parameter, others := parameters[0], parameters[1:]
	readings := parameterReadings(others, tail, chosen)
	for _, spelling := range spellings(parameter, tail) {
		chosen[parameter.Key] = spelling.value
		readings = append(readings, parameterReadings(others, spelling.rest, chosen)...)
		delete(chosen, parameter.Key)
	}
	return readings
}

type spelling struct {
	value float64
	rest  string
}

// spellings lists the values of parameter that tail starts with: a choice
// name, such as "_volume", or a number, such as "_2p5".
func spellings(parameter indicator.ParameterDescriptor, tail string) []spelling {
	var result []spelling
	for _, choice := range parameter.Choices {
		if choice.Name == "" {
			continue
		}
		if rest, ok := strings.CutPrefix(tail, "_"+choice.Name); ok && (rest == "" || rest[0] == '_') {
			result = append(result, spelling{value: float64(choice.Value), rest: rest})
		}
	}
	if match := spelledNumber.FindStringSubmatch(tail); match != nil {
		if value, err := strconv.ParseFloat(strings.Replace(match[1], "p", ".", 1), 64); err == nil {
			result = append(result, spelling{value: value, rest: tail[len(match[1])+1:]})
		}
	}
	return result
}
