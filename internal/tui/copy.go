package tui

import (
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The reference panel is the bottom of the right column: the address of what
// the cursor is on and the commands reaching it, printed so they are selectable
// with the mouse. No secret value is here; "y" is still the only way one moves.
//
// What goes in it is each tab's answer. What is left here is the frame around
// the answer, and the clipboard, which is the shell's because it is a field a
// test replaces.

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
		lines = append(lines, ui.Truncate("  "+label+ui.Sanitize(r.Text), width))
	}
	return lines
}

// copiedMsg reports a finished clipboard write.
//
// op names the operation if it failed; what is the banner if it did not.
type copiedMsg struct {
	op   string
	what string
	err  error
}

// clipboardTimeout bounds a write that is never coming back.
const clipboardTimeout = 5 * time.Second

// errClipboardTimeout is the write that outlived the timeout, worded for a
// footer: the cause is almost always a missing or wedged helper rather than
// anything about the value.
var errClipboardTimeout = errors.New("the clipboard did not answer - is xclip, wl-copy or pbcopy installed and working?")

// copyToClipboard hands the write to Bubble Tea: atotto/clipboard shells out,
// and a wedged helper inside a key handler freezes the program. The timeout
// turns that into an answer; the buffered channel catches a late write.
//
// The value arrives as a plain string, read on the loop that owns it: a command
// runs elsewhere and must not reach back into any model.
func (m *Model) copyToClipboard(op, what, text string) tea.Cmd {
	write := m.clip
	return func() tea.Msg {
		done := make(chan error, 1)
		go func() { done <- write(text) }()

		select {
		case err := <-done:
			return copiedMsg{op: op, what: what, err: err}
		case <-time.After(clipboardTimeout):
			return copiedMsg{op: op, what: what, err: errClipboardTimeout}
		}
	}
}
