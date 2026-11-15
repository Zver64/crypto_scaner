package chart

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"crypto-scanner/internal/indicator"
)

// layout is where an indicator is drawn and what its shapes are titled.
type layout struct {
	// selection is normalized.
	selection  indicator.Selection
	descriptor indicator.Descriptor
	placement  Placement
	// pane is the key of the pane indicators of one type and units share;
	// empty for an indicator drawn in a pane of its own or, with sourcePane,
	// in the pane the source has of its own.
	pane       string
	sourcePane bool
	// source is the output an indicator of another indicator reads, the
	// first of several; nil for an indicator of candle fields.
	source *indicator.Source
	// title names the indicator, such as "RSI 14".
	title string
}

// layoutOf places an indicator by what it reads. An overlay fits the price
// scale only over prices and the volume scale only over volumes; quote
// volumes, trade counts, and everything else get a pane. An indicator of
// another indicator that draws in the units of its input is drawn where that
// source is, whatever fields lie beneath; any other keeps its own pane. Its
// title names the source.
func layoutOf(registry *indicator.Registry, selection indicator.Selection) (layout, error) {
	selection, err := registry.Normalize(selection)
	if err != nil {
		return layout{}, err
	}
	descriptor, err := registry.Describe(selection.Type)
	if err != nil {
		return layout{}, err
	}
	fields, err := registry.Fields(selection)
	if err != nil {
		return layout{}, err
	}
	defaults, err := registry.Normalize(indicator.Selection{Type: selection.Type})
	if err != nil {
		return layout{}, err
	}
	result := layout{selection: selection, descriptor: descriptor, placement: PlacementPane}
	if descriptor.Overlay {
		switch {
		case !slices.ContainsFunc(fields, func(field string) bool { return !indicator.PriceField(field) }):
			result.placement = PlacementOverlay
		case !slices.ContainsFunc(fields, func(field string) bool { return field != "volume" }):
			result.placement = PlacementVolume
		}
	}
	if result.placement == PlacementPane && !slices.ContainsFunc(descriptor.Outputs, func(output indicator.OutputDescriptor) bool {
		return output.Style == indicator.OutputHistogram
	}) {
		result.pane = sharedPane(selection.Type, fields)
	}
	titled := selection.Parameters
	for i := len(descriptor.Parameters) - 1; i >= 0; i-- {
		parameter := descriptor.Parameters[i]
		if parameter.Kind != indicator.ParameterSource {
			continue
		}
		_, nested, err := indicator.ParseSource(selection.Parameters[parameter.Key])
		if err != nil {
			return layout{}, fmt.Errorf("%s %w", parameter.Key, err)
		}
		if nested == nil {
			continue
		}
		source, err := layoutOf(registry, nested.Selection)
		if err != nil {
			return layout{}, err
		}
		// The first nested source decides.
		result.source = nested
		if descriptor.Overlay {
			result.placement, result.pane = source.placement, source.pane
			result.sourcePane = source.placement == PlacementPane && source.pane == ""
		}
		name := source.title
		if len(source.descriptor.Outputs) > 1 {
			name += " " + nested.Output
		}
		titled = maps.Clone(titled)
		titled[parameter.Key] = name
	}
	result.title = lineTitle(descriptor, titled, defaults.Parameters)
	return result, nil
}

// sharedPane keys the pane that indicators of one type share when they read
// fields of the same units, such as every RSI or every ATR, whatever their
// parameters. Indicators with a histogram, such as MACD, do not read well next
// to each other and get panes of their own.
func sharedPane(indicatorType indicator.Type, fields []string) string {
	units := make([]string, len(fields))
	for i, field := range fields {
		units[i] = field
		if indicator.PriceField(field) {
			units[i] = "price"
		}
	}
	slices.Sort(units)
	return string(indicatorType) + ":" + strings.Join(slices.Compact(units), ",")
}

// lineTitle is the upper-case type followed by every parameter value, such as
// "RSI 14" or "MACD 12 26 9". Named choices, such as the candle field, appear
// only once one of them differs from the default, as in "SMA 20 volume".
func lineTitle(descriptor indicator.Descriptor, parameters, defaults indicator.Parameters) string {
	parts := []string{strings.ToUpper(string(descriptor.Type))}
	named := NamedChoicesChanged(descriptor, parameters, defaults)
	for _, parameter := range descriptor.Parameters {
		if !NamedChoice(parameter) || named {
			parts = append(parts, ParameterValue(parameter, parameters[parameter.Key]))
		}
	}
	return strings.Join(parts, " ")
}

// NamedChoicesChanged reports whether a named choice differs from its
// default.
func NamedChoicesChanged(descriptor indicator.Descriptor, parameters, defaults indicator.Parameters) bool {
	return slices.ContainsFunc(descriptor.Parameters, func(parameter indicator.ParameterDescriptor) bool {
		return NamedChoice(parameter) && ParameterValue(parameter, parameters[parameter.Key]) != ParameterValue(parameter, defaults[parameter.Key])
	})
}

// ParameterValue formats a value as titles show it, naming named choices
// such as "volume", "price", or "ema"; a nested source is already a name.
// Choices without names stay numbers.
func ParameterValue(parameter indicator.ParameterDescriptor, value any) string {
	if NamedChoice(parameter) {
		for _, choice := range parameter.Choices {
			if fmt.Sprint(choice.Value) == fmt.Sprint(value) {
				return choice.Name
			}
		}
	}
	return fmt.Sprint(value)
}

// NamedChoice reports a parameter whose choices have names: a source, or a
// choice such as the pivots of divergence.
func NamedChoice(parameter indicator.ParameterDescriptor) bool {
	return parameter.Kind == indicator.ParameterSource ||
		parameter.Kind == indicator.ParameterChoice && slices.ContainsFunc(parameter.Choices, func(choice indicator.Choice) bool { return choice.Name != "" })
}

// sameSelection reports whether two normalized selections are equal.
func sameSelection(left, right indicator.Selection) bool {
	return selectionKey(left) == selectionKey(right)
}

func selectionKey(selection indicator.Selection) string {
	var key strings.Builder
	key.WriteString(string(selection.Type))
	for _, name := range slices.Sorted(maps.Keys(selection.Parameters)) {
		fmt.Fprintf(&key, "|%s=%v", name, selection.Parameters[name])
	}
	return key.String()
}
