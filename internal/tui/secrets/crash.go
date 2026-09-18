package secrets

import (
	"fmt"
	"strings"
)

// CrashNote is this tab's half of the report: where the user was, and what they
// had not saved. Keys, never values - a crash report is a file on disk, and the
// path plus the key names are what it takes to redo the work. The values the
// user still has.
func (m *Model) CrashNote() string {
	var b strings.Builder

	if n := m.tree.current(); n != nil {
		fmt.Fprintf(&b, "selected: %s\n", n.fullPath())
	}

	e := m.currentEditor()
	if e == nil {
		return b.String()
	}

	what := "editing"
	if e.creating {
		what = "creating"
	}
	fmt.Fprintf(&b, "\nUNSAVED - %s %s, from v%d\n", what, e.path, e.baseVersion)
	for _, f := range e.fields {
		switch {
		case f.added:
			fmt.Fprintf(&b, "  + %s\n", f.key)
		case f.changed:
			fmt.Fprintf(&b, "  ~ %s\n", f.key)
		}
	}
	for _, k := range e.removedKeys {
		fmt.Fprintf(&b, "  - %s\n", k)
	}
	b.WriteString("\nThe values are deliberately not recorded.\n")

	return b.String()
}
