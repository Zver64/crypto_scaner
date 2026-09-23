package alerts

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeTarget(t *testing.T) {
	t.Parallel()
	for input, want := range map[string]string{"1": "1", "1.0": "1", "0.0100": "0.01", "999.12000": "999.12", strings.Repeat("9", 20): strings.Repeat("9", 20)} {
		input, want := input, want
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeTarget(input)
			if err != nil || got != want {
				t.Fatalf("NormalizeTarget(%q)=(%q,%v), want %q", input, got, err, want)
			}
		})
	}
	for _, input := range []string{"0", "0.0", "-1", "+1", "1e3", "01", ".1", "1.", strings.Repeat("9", 21), strings.Repeat("9", 20) + "." + strings.Repeat("1", 19), strings.Repeat("9", 38) + ".1", strings.Repeat("9", 39)} {
		input := input
		t.Run("invalid_"+input, func(t *testing.T) {
			t.Parallel()
			if _, err := NormalizeTarget(input); !errors.Is(err, ErrInvalidTarget) {
				t.Fatalf("NormalizeTarget(%q) error=%v", input, err)
			}
		})
	}
}

func TestCompareUsesExactDecimals(t *testing.T) {
	t.Parallel()
	cmp, err := Compare("9007199254740993", "9007199254740992")
	if err != nil || cmp <= 0 {
		t.Fatalf("Compare()=(%d,%v)", cmp, err)
	}
	cmp, err = Compare("1.0", "1.00")
	if err != nil || cmp != 0 {
		t.Fatalf("equal Compare()=(%d,%v)", cmp, err)
	}
}
