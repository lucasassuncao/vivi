package ui

import (
	"strings"
	"testing"
)

// Whatever bytes a value carries, what comes out of sanitize is safe to draw:
// no rune the terminal would act on, and nothing lost that was safe to begin
// with. Invalid UTF-8 is included, because a secret is not obliged to be text.
func FuzzSanitizeIsSafeToDraw(f *testing.F) {
	rtlOverride := string(rune(0x202e))
	for _, s := range []string{"", "plain", "a\x1b[31mb", rtlOverride, "tab\there", "\xff\xfe", "caf" + string(rune(0xe9))} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := Sanitize(s)
		if strings.ContainsFunc(out, unsafeToDraw) {
			t.Fatalf("sanitize(%q) = %q still holds a rune the terminal acts on", s, out)
		}
		if !strings.ContainsFunc(s, unsafeToDraw) && out != s {
			t.Fatalf("sanitize(%q) = %q changed a value that was safe", s, out)
		}
		if Sanitize(out) != out {
			t.Fatalf("sanitize is not idempotent on %q", s)
		}
	})
}
