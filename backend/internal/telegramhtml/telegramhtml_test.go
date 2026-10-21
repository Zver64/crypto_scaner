package telegramhtml

import (
	"strings"
	"testing"
)

// A title with line breaks stays on one line, so a cut near it never leaves
// its tag open.
func TestTruncateKeepsMultiLineTitleBalanced(t *testing.T) {
	text := strings.Repeat("x", MaxLength-20) + "\n" + Title("Buy\nsignal\nname") + "\nbody"
	got := Truncate(text, "\n…")
	if strings.Count(got, "<b>") != strings.Count(got, "</b>") {
		t.Fatalf("Truncate() left a tag open: %q", got[len(got)-20:])
	}
}
