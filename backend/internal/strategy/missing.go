package strategy

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
)

// spelledNumber is a parameter value as variable names spell it, such as
// "_20" or "_2p5".
var spelledNumber = regexp.MustCompile(`^_([0-9]+(?:p[0-9]+)?)(?:_|$)`)

// missingIndicators resolves names that no variable has into the indicators
// that would add them, once per indicator and in the order of names. A name
// that no indicator, or several, would add stays unresolved.
func (service *Service) missingIndicators(names []string) []scannerindicator.Entry {
	var missing []scannerindicator.Entry
	for _, name := range names {
		found := service.indicatorsNamed(name)
		if len(found) != 1 || slices.ContainsFunc(missing, func(entry scannerindicator.Entry) bool { return entry.Target().Equal(found[0].Target()) }) {
			continue
		}
		missing = append(missing, found[0])
	}
	return missing
}

// indicatorsNamed lists the distinct indicators with an output whose variable
// would be name. Names spell the interval, the type, and the parameter
// values of the title; every way of reading the values is tried and kept
// only when the title it gives spells name back.
func (service *Service) indicatorsNamed(name string) []scannerindicator.Entry {
	var found []scannerindicator.Entry
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
				entry, err := service.indicators.Preview(scannerindicator.Indicator{
					Interval:  interval,
					Selection: indicator.Selection{Type: descriptor.Type, Parameters: parameters},
				})
				if err != nil || !slices.ContainsFunc(entry.Outputs, func(output string) bool { return outputName(entry, output) == name }) {
					continue
				}
				if !slices.ContainsFunc(found, func(other scannerindicator.Entry) bool { return other.Target().Equal(entry.Target()) }) {
					found = append(found, entry)
				}
			}
		}
	}
	return found
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
