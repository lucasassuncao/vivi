package secrets

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// Walking to a path. The shell parses ":goto" and hands the argument here,
// because a path is this tab's vocabulary and the command line's only job was
// to read it off the line.

// Goto walks to a full path, expanding each level as its listing arrives - the
// reason the cmdline parses arguments at all. The walk is stored and not
// looped, because every listing is a request answered by a message.
func (m *Model) Goto(path string) tea.Cmd {
	// The leading slash is noise; the trailing one is not. "secret/app" and
	// "secret/app/" are two different things in Vault, and the slash is all
	// that tells find which was meant.
	path = strings.TrimPrefix(strings.TrimSpace(path), "/")
	if strings.Trim(path, "/") == "" {
		m.notify("goto needs a path, as in kv/app/prod/db")
		return nil
	}
	// A path is an address, so it works from wherever the user is. Switching to
	// this tab is the shell's to do, so it is asked for rather than done.
	m.outbox = append(m.outbox,
		ui.ShowSecretsMsg{},
		ui.FocusMsg{Focus: ui.FocusList},
	)

	m.pendingGoto = path
	return m.drain(m.advanceGoto())
}

// advanceGoto takes the walk one level further, and is re-entered by every
// listing that arrives while one is in flight.
func (m *Model) advanceGoto() tea.Cmd {
	want := m.pendingGoto
	if want == "" {
		return nil
	}

	n, rest := m.tree.find(want)
	switch {
	case n == nil:
		m.pendingGoto = ""
		return m.failWith("no mount called " + firstSegment(want))

	case rest == "":
		m.pendingGoto = ""
		m.tree.revealNode(n)
		return m.onCursorMoved()

	case n.loading:
		return nil // its listing is already on the way; that message re-enters here

	case n.denied:
		m.pendingGoto = ""
		return m.failWith("cannot list " + n.fullPath() + ": permission denied")

	case n.kind == kindSecret, !n.expandable(), n.loaded:
		// Nothing below this node, or it is listed and the next segment was
		// not in it.
		m.pendingGoto = ""
		return m.failWith(want + " is not a path in this vault")
	}

	return m.openNode(n)
}

// openNode expands a node without toggling it. expand() is a toggle, right for
// a keystroke and wrong for a walk: crossing an already-open level would close
// it, and the walk would arrive with its own trail collapsed behind it.
func (m *Model) openNode(n *node) tea.Cmd {
	n.expanded = true
	if n.loaded {
		m.tree.reflow()
		return nil
	}
	n.loading = true
	m.tree.reflow()
	return m.loadChildren(n)
}

func firstSegment(path string) string {
	seg, _, _ := strings.Cut(path, "/")
	return seg
}

// routeToInput hands a message to whichever nested Bubbles model is taking
// keystrokes: the filter, a confirmation's input, or a field in the editor.
func (m *Model) routeToInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch current := m.currentMode().(type) {
	case filtering:
		m.filterInput, cmd = m.filterInput.Update(msg)
	case confirming:
		if c := current.confirmation; c != nil && (c.needsInput() || c.capture) {
			c.input, cmd = c.input.Update(msg)
		}
	case editing:
		if e := current.editor; e != nil && e.form != nil {
			cmd = e.form.route(msg)
		}
	}
	return cmd
}

// focusDetailPane moves focus to the detail. It used to refuse below a narrow
// terminal, which also meant a narrow terminal could browse the tree and open
// nothing. The shell draws the focused pane, so focus is never invisible.
func (m *Model) focusDetailPane() {
	m.outbox = append(m.outbox, ui.FocusMsg{Focus: ui.FocusDetail})
}

// followFieldCursor scrolls the pane so the selected field stays on screen. A
// selection that has scrolled out of view is worse than no selection, because r
// and y still act on it. The viewport is the shell's, so this is a request.
func (m *Model) followFieldCursor() {
	m.outbox = append(m.outbox, ui.ScrollToLineMsg{Line: fieldsStartLine + m.fieldCursor})
}
