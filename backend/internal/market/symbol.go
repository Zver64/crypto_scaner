package market

import "strings"

// NormalizeSymbol returns the canonical exchange symbol form.
func NormalizeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}
