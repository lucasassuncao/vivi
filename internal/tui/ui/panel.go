package ui

import (
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

// Heading, HeadingBadged and KV are bezel's, drawn with this theme's inks.
func (s Styles) Heading(name string, width int) string {
	return draw.Heading(name, s.Section, width)
}

func (s Styles) HeadingBadged(name, badge string, width int) string {
	return draw.HeadingBadged(name, badge, s.Section, width)
}

func (s Styles) KV(key, value string, width int) string { return draw.KV(key, value, s.Key, width) }
