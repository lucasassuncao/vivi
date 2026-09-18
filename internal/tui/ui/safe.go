package ui

import (
	"fmt"
	"strings"
)

// Sanitize renders server-supplied text so drawing it can only draw: OSC 52
// sets the clipboard, CSI forges a confirmation modal, bidi (CVE-2021-42574)
// reorders a line. Rendering only - the clipboard still sees the real bytes.
func Sanitize(s string) string {
	if !strings.ContainsFunc(s, unsafeToDraw) {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case !unsafeToDraw(r):
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == 0x1b:
			b.WriteString(`\e`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			fmt.Fprintf(&b, `\u%04x`, r)
		}
	}
	return b.String()
}

// SanitizeLines handles legitimately multi-line text - a policy, a role's
// configuration - keeping the breaks and neutralising the rest. A trailing
// carriage return is CRLF and not content, so a Windows document reads clean.
func SanitizeLines(s string) []string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = Sanitize(strings.TrimSuffix(line, "\r"))
	}
	return lines
}

// unsafeToDraw reports whether a rune changes the terminal's behaviour or the
// reading order of the line, rather than putting a glyph on the screen.
func unsafeToDraw(r rune) bool {
	switch {
	case r < 0x20, r == 0x7f:
		// C0 controls and DEL: escapes, carriage returns, NUL.
		return true
	case r >= 0x80 && r <= 0x9f:
		// C1 controls. A UTF-8 terminal will not act on these, but not every
		// terminal is in UTF-8 mode, and the cost of covering them is nothing.
		return true
	case r == 0x200e, r == 0x200f:
		// LEFT-TO-RIGHT and RIGHT-TO-LEFT MARK.
		return true
	case r >= 0x202a && r <= 0x202e:
		// The embedding and override pair, and POP DIRECTIONAL FORMATTING.
		return true
	case r >= 0x2066 && r <= 0x2069:
		// The isolates, which do the same job with tidier scoping.
		return true
	default:
		return false
	}
}
