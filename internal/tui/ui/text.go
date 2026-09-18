package ui

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// The text every panel is built out of: the mask, the label rows, and the two
// measurements that have to agree with what is drawn. Nothing here belongs to
// one tab - a single component's helper lives with that component.

// MaskedValue is what a secret looks like until it is explicitly revealed.
const MaskedValue = "•••••••••"

// KV is one labelled row. Both halves are sanitized here rather than at the
// call sites: every caller is passing something the server chose, and the ones
// passing a literal lose nothing by it.
func (s Styles) KV(key, value string, width int) string {
	return Truncate("  "+s.Key.Render(fmt.Sprintf("%-14s ", Sanitize(key)))+Sanitize(value), width) + "\n"
}

// Pad fills a line out to a width, measuring what lands on screen. The inverse
// of Truncate, and used with it wherever a column has to line up.
func Pad(line string, width int) string {
	if gap := width - lipgloss.Width(line); gap > 0 {
		return line + strings.Repeat(" ", gap)
	}
	return line
}

func OrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func SortedMapKeys(m map[string]any) []string {
	return slices.Sorted(maps.Keys(m))
}

// Truncate cuts a line to a width, measuring what lands on screen and not what
// is in the string, which carries escape sequences occupying no columns. It
// never wraps, so the layout can count lines and get the terminal's answer.
func Truncate(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

// ShortDuration renders a lifetime the way a header has room for: the two
// largest units that say anything, truncated rather than rounded, so a countdown
// never shows more time than is left.
func ShortDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	switch {
	case d >= 24*time.Hour:
		days := d / (24 * time.Hour)
		if h := (d % (24 * time.Hour)) / time.Hour; h > 0 {
			return fmt.Sprintf("%dd%dh", days, h)
		}
		return fmt.Sprintf("%dd", days)
	case d >= time.Hour:
		hours := d / time.Hour
		if mins := (d % time.Hour) / time.Minute; mins > 0 {
			return fmt.Sprintf("%dh%dm", hours, mins)
		}
		return fmt.Sprintf("%dh", hours)
	case d >= time.Minute:
		return fmt.Sprintf("%dm", d/time.Minute)
	default:
		return fmt.Sprintf("%ds", d/time.Second)
	}
}

// PackToWidth joins parts over up to maxLines, dropping whole ones off the end
// until they fit. ellipsis takes the
// dropped count so the mark can say how many are missing, measured at its
// widest: a reservation growing after the budget is how a line runs over.
func PackToWidth(parts []string, separator string, ellipsis func(dropped int) string, width, maxLines int) []string {
	if len(parts) == 0 {
		return nil
	}
	if width <= 0 || maxLines < 1 {
		return []string{strings.Join(parts, separator)}
	}

	lines, placed := packGreedy(parts, separator, width, maxLines, 0)
	if placed < len(parts) {
		lines, placed = packGreedy(parts, separator, width, maxLines,
			lipgloss.Width(ellipsis(len(parts))))
		lines[len(lines)-1] += ellipsis(len(parts) - placed)
	}

	// The first part on a line is kept whatever it costs, the same rule
	// The first part always stays, so a line can still be wider than the terminal on its
	// own and still has to be cut.
	for i := range lines {
		lines[i] = Truncate(lines[i], width)
	}
	return lines
}

// packGreedy fills lines left to right, reporting how many parts it placed.
// reserveOnLast is held back from the budget of the final line, which is where
// the ellipsis goes.
func packGreedy(parts []string, separator string, width, maxLines, reserveOnLast int) (lines []string, placed int) {
	sepWidth := lipgloss.Width(separator)

	var current string
	currentWidth := 0

	for _, part := range parts {
		partWidth := lipgloss.Width(part)
		cost := partWidth
		if current != "" {
			cost += sepWidth
		}

		budget := width
		if len(lines)+1 == maxLines {
			budget -= reserveOnLast
		}

		if current != "" && currentWidth+cost > budget {
			if len(lines)+1 == maxLines {
				break // out of lines: the rest is what the ellipsis stands for
			}
			lines = append(lines, current)
			current, currentWidth = "", 0
			cost = partWidth
		}

		if current == "" {
			current = part
		} else {
			current += separator + part
		}
		currentWidth += cost
		placed++
	}

	if current != "" || len(lines) == 0 {
		lines = append(lines, current)
	}
	return lines, placed
}

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
