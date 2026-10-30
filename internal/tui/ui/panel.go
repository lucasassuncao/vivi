package ui

import (
	"fmt"
	"strings"

	"github.com/lucasassuncao/bezel/draw"
	"github.com/lucasassuncao/bezel/layout"
)

// NarrowLimit is where two panes stop being readable and the detail collapses.
// At 44 the split was legal and useless: kv pads a label to fourteen, so every
// value was truncated to nothing. 72 leaves the detail 42 columns.
//
// Shared because the layout decides the split by it and a tab's legend changes
// its wording by it: one pane and two are different sentences.
const NarrowLimit = 72

// EmptyState centres a message in the pane instead of leaving a blank rectangle
// with a sentence in its corner: bezel's, in this theme's dim ink.
func (s Styles) EmptyState(width, height int, message, hint string) string {
	return draw.EmptyState(layout.Rect{W: width, H: height}, message, hint, draw.ChromeStyle{Dim: s.Dim})
}

// Heading is bezel's, drawn with this theme's section ink.
func (s Styles) Heading(name string, width int) string {
	return draw.Heading(name, s.Section, width)
}

// HeadingBadged is a heading wearing a badge beside it, or the heading alone
// when the two do not fit: a badge that says what the block is has to be read
// with it.
func (s Styles) HeadingBadged(name, badge string, width int) string {
	label := strings.ToUpper(name)
	if draw.Width(label)+1+draw.Width(badge) > width {
		return s.Heading(name, width)
	}
	return s.Section.Render(label) + " " + badge
}

// KV is one labelled row: the key padded to 14, the value after it, the whole
// line cut to width. Both halves are sanitized: either may be a server's text.
func (s Styles) KV(key, value string, width int) string {
	return draw.Cut("  "+s.Key.Render(fmt.Sprintf("%-14s ", draw.Sanitize(key)))+draw.Sanitize(value), width) + "\n"
}
