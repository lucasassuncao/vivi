package tui

import (
	"slices"
	"strings"
)

// available is what can run right now, in alphabetical order. The palette is
// looked up by name and narrowed by prefix, so the name is what the eye follows
// and a family shares a place: the generated copies land beside ":copy" without
// being put there, and the list does not move under a cursor when the set
// changes with it.
func (m *Model) available() []command {
	out := make([]command, 0, len(commands))
	for _, c := range commands {
		if c.scope(m) {
			out = append(out, c)
		}
	}
	out = append(out, m.copyCommands()...)

	slices.SortFunc(out, func(a, b command) int { return strings.Compare(a.name, b.name) })
	return out
}
