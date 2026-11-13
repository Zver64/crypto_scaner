package strategy

import (
	"slices"
	"testing"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/candle"
	"crypto-scanner/internal/indicator/pivot"
	indicatortalib "crypto-scanner/internal/indicator/talib"
)

// Every function name of the language resolves to exactly one indicator
// output, candle field, or function over the whole registry, since an
// indicator with several outputs is called as its type and the output.
func TestFunctionNamesAreUnique(t *testing.T) {
	modules := indicatortalib.New()
	rsi := slices.IndexFunc(modules, func(module indicator.Implementation) bool {
		return module.Describe().Type == indicatortalib.RSIType
	})
	if rsi < 0 {
		t.Fatal("no RSI module")
	}
	registry, err := indicator.NewRegistry(append(modules, pivot.New(), pivot.NewDivergence(modules[rsi]), candle.New())...)
	if err != nil {
		t.Fatal(err)
	}
	owners := map[string]string{}
	add := func(name, owner string) {
		if previous, found := owners[name]; found {
			t.Errorf("%s names both %s and %s", name, previous, owner)
		}
		owners[name] = owner
	}
	for _, function := range []string{"Abs", "Mod", "Min", "Max", "Prev", "Percentile", "Of", "CrossesAbove", "CrossesBelow"} {
		add(function, "function "+function)
	}
	for _, field := range CandleFields() {
		add(field, "candle field "+field)
	}
	for _, variable := range PositionVariables() {
		add(variable.Name, "position variable "+variable.Name)
	}
	for _, descriptor := range registry.Descriptors() {
		if len(descriptor.Outputs) > 1 {
			add(string(descriptor.Type), "indicator "+string(descriptor.Type))
		}
		for _, output := range descriptor.Outputs {
			add(indicatorFunction(descriptor.Type, len(descriptor.Outputs), output.Name), "output "+output.Name+" of "+string(descriptor.Type))
		}
	}
}
