package tui

import (
	"fmt"
	"github.com/lucasassuncao/bezel/draw"

	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The reference panel is the bottom of the right column: the address of what
// the cursor is on and the commands reaching it, printed so they are selectable
// with the mouse. No secret value is here; "y" is still the only way one moves.
//
// What goes in it is each tab's answer. What is left here is the frame around
// the answer; the clipboard write itself is bezel's shell.Copy.

// copyPanelTitle names the panel, and is what the layout looks for when
// deciding whether there is a second panel at all. It read "Copy" until that
// promised an action no key here performs: nothing is copied for you.
const copyPanelTitle = "Command Reference"

// copyRows is what the panel has to show for the tab in front of the user, or
// nothing at all - which is a real answer, and the one that collapses the right
// column back to a single panel.
func (m *Model) copyRows() []ui.CopyRow {
	return m.currentTab().CopyRows(m.uiContext())
}

// copyLines renders the panel, one screen line per entry. Lines and not a block
// because the layout has to know how tall the panel wants to be first, and
// every line here is truncated and never wrapped: what is counted is drawn.
func (m *Model) copyLines(width int) []string {
	rows := m.copyRows()
	if len(rows) == 0 {
		return nil
	}

	labelWidth := 0
	for _, r := range rows {
		if len(r.Label) > labelWidth {
			labelWidth = len(r.Label)
		}
	}

	// No section heading: the panel's own title already says what this is, and a
	// rule repeating it under the border spends a row saying it twice.
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		// The path and the mount inside a command are both server-supplied, so
		// the whole line goes out under the same rules as any other content the
		// panels draw.
		label := m.st.Key.Render(fmt.Sprintf("%-*s ", labelWidth, r.Label))
		lines = append(lines, draw.Cut("  "+label+draw.Sanitize(r.Text), width))
	}
	return lines
}
