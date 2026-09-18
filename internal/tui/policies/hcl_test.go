package policies

import (
	"strings"
	"testing"

	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

func TestHCLHighlightingPreservesTheText(t *testing.T) {
	// No colour profile to force any more: a v2 style renders full-fidelity
	// escapes wherever it runs, and the terminal's own profile is applied when
	// the frame is written out. So the escapes are here to strip even with no
	// TTY in sight.
	st := ui.NewStyles(ui.Colors{}, true)
	doc := "# a comment\npath \"kv/data/app/*\" {\n  capabilities = [\"read\", \"list\"]\n  max_ttl = 3600\n}\n"

	highlighted := highlight(st, doc)

	// Colour must not change what the policy says: strip the escapes and the
	// document has to come back byte for byte.
	if got := stripANSI(highlighted); got != doc {
		t.Fatalf("highlighting altered the document:\nwant %q\ngot  %q", doc, got)
	}
	if highlighted == doc {
		t.Fatal("nothing was highlighted at all")
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // the "m" itself
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
