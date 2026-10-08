package main

import "testing"

func TestParseAndFormatUSD(t *testing.T) {
	for _, test := range []struct {
		text, formatted string
	}{
		{"150M", "$150M"}, {"1.5b", "$1.5B"}, {"$2T", "$2T"}, {"2500k", "$2.5M"}, {"500", "$500"}, {" 2000000 ", "$2M"}, {"999999", "$1M"},
	} {
		usd, err := parseUSD(test.text)
		if err != nil || usd == nil || formatUSD(*usd) != test.formatted {
			t.Errorf("parseUSD(%q) = %v, %v; want %s", test.text, usd, err, test.formatted)
		}
	}
	if usd, err := parseUSD(""); usd != nil || err != nil {
		t.Errorf("parseUSD(\"\") = %v, %v; want an open bound", usd, err)
	}
	for _, text := range []string{"0", "-5M", "M", "lots", "1e400", "NaN"} {
		if _, err := parseUSD(text); err == nil {
			t.Errorf("parseUSD(%q) accepted", text)
		}
	}
}
