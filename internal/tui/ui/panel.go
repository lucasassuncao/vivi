package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// FitPanel forces content into the box the panel has. Clipping by line count
// is not enough: a too-wide line wraps inside lipgloss into two rows, pushing
// the border down. Cut to the content width first, so the row count is honest.
func FitPanel(content string, width, height int) string {
	// Same reason clipHeight refuses one: a negative height reaches the slice as
	// a bound, and lines[:-1] is a panic rather than an empty panel.
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = Truncate(line, width)
	}
	return strings.Join(lines, "\n")
}

// NarrowLimit is where two panes stop being readable and the detail collapses.
// At 44 the split was legal and useless: kv pads a label to fourteen, so every
// value was truncated to nothing. 72 leaves the detail 42 columns.
//
// Shared because the layout decides the split by it and a tab's legend changes
// its wording by it: one pane and two are different sentences.
const NarrowLimit = 72

// PanelContentWidth is how many columns a panel of this outer width leaves for
// content: two for the borders, two for the padding.
func PanelContentWidth(outer int) int {
	return max(1, outer-4)
}

// PanelContentHeight is how many rows a panel of this outer height leaves: one
// for the title edge, one for the bottom edge.
func PanelContentHeight(outer int) int {
	return max(1, outer-2)
}

// The furniture inside a panel, shared by every pane that has any: the block
// headings, the message a pane shows when it has nothing to list, and the
// scrolling every list does the same way.

// Heading is a block heading inside a pane. It was a rule of box-drawing dashes
// to the full width, which at 250 columns cost one word two hundred dashes.
// Typography costs no columns, and matches the tree's group labels.
func (s Styles) Heading(name string, width int) string {
	return s.Section.Render(Truncate(strings.ToUpper(name), width))
}

// HeadingBadged is a heading wearing a badge. Beside the label rather than
// pinned to the far edge: on a wide pane the two ended up a screen apart, and a
// badge that says what the block below it is has to be read with it.
func (s Styles) HeadingBadged(name, badge string, width int) string {
	label := strings.ToUpper(name)
	if lipgloss.Width(label)+1+lipgloss.Width(badge) > width {
		// No room for the badge: the heading alone is the honest fallback.
		return s.Heading(name, width)
	}
	return s.Section.Render(label) + " " + badge
}

// EmptyState centres a message in the pane instead of leaving a blank rectangle
// with a sentence in its corner. Wrapped before it is placed: left alone, a long
// hint wraps inside the panel and drops its border below the neighbour's.
func (s Styles) EmptyState(width, height int, message, hint string) string {
	wrap := lipgloss.NewStyle().Width(max(1, width)).Align(lipgloss.Center)

	body := wrap.Render(s.Dim.Render(message))
	if hint != "" {
		body += "\n\n" + wrap.Render(s.Help.Render(hint))
	}
	if width < 4 || height < 3 {
		return body
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, body)
}

// ScrollStart keeps the cursor inside the window without jumping around.
func ScrollStart(cursor, total, height int) int {
	if total <= height || height <= 0 {
		return 0
	}
	return min(max(cursor-height/2, 0), total-height)
}
