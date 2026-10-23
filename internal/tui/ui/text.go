package ui

import (
	"maps"
	"slices"
	"time"

	"github.com/lucasassuncao/bezel/input"
	"github.com/lucasassuncao/bezel/theme"
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

// NewInput builds a text input in th, whose CaretBlink Shell sets from
// BlinkSpeed. The call sites set their own prompt and placeholder.
func NewInput(th theme.Resolved) input.Model { return input.New(th) }

// SortedMapKeys is a map's keys in order, for a listing that must not shuffle.
func SortedMapKeys(m map[string]any) []string {
	return slices.Sorted(maps.Keys(m))
}
