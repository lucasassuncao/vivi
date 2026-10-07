package ui

import (
	"maps"
	"slices"
	"time"

	"charm.land/bubbles/v2/textinput"
)

// The text every panel is built out of: the mask, the label rows, and the two
// measurements that have to agree with what is drawn. Nothing here belongs to
// one tab - a single component's helper lives with that component.

// MaskedValue is what a secret looks like until it is explicitly revealed.
const MaskedValue = "•••••••••"

// BlinkSpeed is how often a focused text input flips its caret. Every keystroke
// typed into one answers with a command that waits this long. A variable so
// tests can shrink it: a timer does not answer inside the harness's wait.
var BlinkSpeed = 530 * time.Millisecond

// NewInput builds a text input with this app's caret. The four call sites set
// their own prompt and placeholder and share the caret, which is the part a
// test needs: the inputs are built while the app runs, so there is nowhere else.
func NewInput() textinput.Model {
	in := textinput.New()
	s := in.Styles()
	s.Cursor.BlinkSpeed = BlinkSpeed
	in.SetStyles(s)
	return in
}

// SortedMapKeys is a map's keys in order, for a listing that must not shuffle.
func SortedMapKeys(m map[string]any) []string {
	return slices.Sorted(maps.Keys(m))
}
