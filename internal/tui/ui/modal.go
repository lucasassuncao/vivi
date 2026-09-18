package ui

import (
	"fmt"
	"strings"
)

// The box a floating panel is drawn in. Only the box: what a modal asks and
// which message it dispatches belongs to whichever component raised it.

// ModalMaxInner stops a modal spanning a wide terminal. One long value - a
// certificate on a single line - sized the box to itself, because a box is as
// wide as its widest line. Past this a line stops being scannable anyway.
const ModalMaxInner = 120

// ModalInner is the width a modal's content gets when it takes the terminal:
// border and padding on both sides, and never more than it can be read across.
func ModalInner(width int) int { return min(max(10, width-8), ModalMaxInner) }

// ModalBox frames content and clips it to the terminal: an overflowing modal
// wraps into the panes and becomes unreadable exactly when it matters most.
// bodyHeight is passed in, so rendering stays a pure function of the state.
func (s Styles) ModalBox(lines []string, bodyHeight, termWidth int) string {
	return s.ModalBoxWidth(lines, bodyHeight, ModalInner(termWidth), termWidth)
}

// ModalBoxWidth is ModalBox at a width of the caller's choosing.
func (s Styles) ModalBoxWidth(lines []string, bodyHeight, inner, termWidth int) string {
	clipped := make([]string, 0, len(lines))
	for _, line := range lines {
		for _, part := range strings.Split(line, "\n") {
			clipped = append(clipped, Truncate(part, inner))
		}
	}

	// A modal taller than the body pushes its own border off the screen, the
	// same way an unclipped pane does.
	if maxLines := bodyHeight - 2; maxLines > 2 && len(clipped) > maxLines {
		cut := len(clipped) - maxLines + 1
		clipped = append(clipped[:maxLines-1],
			s.Dim.Render(fmt.Sprintf("… %d more lines, resize to see them", cut)))
	}

	return s.Modal.MaxWidth(termWidth).Render(strings.Join(clipped, "\n"))
}
