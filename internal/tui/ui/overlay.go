package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Composite draws fg over bg at (x, y), replacing the cells it covers and
// keeping the rest of each bg line, escapes included. This is what stops a
// modal blanking the screen: an empty canvas reads as a black rectangle.
func Composite(fg, bg string, x, y int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	for i, fgLine := range fgLines {
		row := y + i
		if row < 0 || row >= len(bgLines) {
			continue
		}
		bgLine := bgLines[row]

		left := ansi.Truncate(bgLine, x, "")
		if pad := x - ansi.StringWidth(left); pad > 0 {
			// The bg line stops before the box starts: fill the gap so the box
			// lands on the column it was placed at.
			left += strings.Repeat(" ", pad)
		}
		right := ansi.TruncateLeft(bgLine, x+ansi.StringWidth(fgLine), "")

		bgLines[row] = left + fgLine + right
	}

	return strings.Join(bgLines, "\n")
}

// CompositeCenter centres fg over bg.
func CompositeCenter(fg, bg string) string {
	fgW, fgH := BlockSize(fg)
	bgW, bgH := BlockSize(bg)

	return Composite(fg, bg, max(0, (bgW-fgW)/2), max(0, (bgH-fgH)/2))
}

// BlockSize measures a rendered block: its widest line, and how many lines it
// has.
func BlockSize(s string) (width, height int) {
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		if w := ansi.StringWidth(line); w > width {
			width = w
		}
	}
	return width, len(lines)
}

// PadHeight grows a block to the given number of lines. The background has to
// be at least as tall as what is drawn over it, or Composite would have nowhere
// to put the bottom of the box.
func PadHeight(content string, height int) string {
	if missing := height - strings.Count(content, "\n") - 1; missing > 0 {
		return content + strings.Repeat("\n", missing)
	}
	return content
}
