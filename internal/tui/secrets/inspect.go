package secrets

import (
	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// What this tab will say about itself to a caller outside it. The shell does
// not use any of it; the tests that drive the whole app do, because the tree is
// how you get anywhere in vivi and it lives here now.

// State is what this tab looks like from outside: enough for a caller driving
// the whole app to say what the tab did, and nothing it could write through.
type State struct {
	Mode        string
	CursorPath  string
	SelPath     string
	Filter      string
	FilterValue string

	SecretLoaded  bool
	SecretVersion int
	FieldCursor   int
	VersionCursor int
	Versions      int
	Revealed      int
	Marked        int
	TreeRows      int
	TreeCursor    int
	PendingGoto   string
}

// FieldKeys are the secret's keys in the order the pane draws them.
func (m *Model) FieldKeys() []string { return m.sortedKeys() }

// SecretValue is one field of the secret on screen, rendered the way the pane
// would render it. There is no secret loaded, or no such field, and it is "".
func (m *Model) SecretValue(key string) string {
	if m.secret == nil {
		return ""
	}
	return ui.ValueToString(m.secret.Data[key])
}

// RevealedKeys are the fields currently shown in the clear.
func (m *Model) RevealedKeys() []string {
	out := make([]string, 0, len(m.revealed))
	for k, on := range m.revealed {
		if on {
			out = append(out, k)
		}
	}
	return out
}

func (m *Model) State() State {
	s := State{
		Mode:          m.currentMode().Name(),
		CursorPath:    m.CursorPath(),
		Filter:        m.tree.filter,
		FilterValue:   m.filterInput.Value(),
		SecretLoaded:  m.secret != nil,
		SecretVersion: m.secretVersion,
		FieldCursor:   m.fieldCursor,
		VersionCursor: m.versionCursor,
		Versions:      len(m.versions),
		Revealed:      len(m.revealed),
		Marked:        len(m.marked),
		TreeRows:      len(m.tree.visible),
		TreeCursor:    m.tree.cursor,
		PendingGoto:   m.pendingGoto,
	}
	if m.sel != nil {
		s.SelPath = m.sel.fullPath()
	}
	return s
}

// VisiblePaths is every node currently drawn in the tree, top to bottom.
func (m *Model) VisiblePaths() []string {
	out := make([]string, 0, len(m.tree.visible))
	for _, r := range m.tree.visible {
		if r.kind == rowNode {
			out = append(out, r.node.fullPath())
		}
	}
	return out
}

// CursorPath is the node under the cursor, or "".
func (m *Model) CursorPath() string {
	if n := m.tree.current(); n != nil {
		return n.fullPath()
	}
	return ""
}

// MountPath is a mount's path by name, or "". It reads the roots rather than
// the visible rows, so it keeps working however the visible list is shaped.
func (m *Model) MountPath(name string) string {
	for _, n := range m.tree.roots {
		if n.name == name {
			return n.fullPath()
		}
	}
	return ""
}

// FocusPath puts the cursor on a visible node by full path and loads whatever
// landing there would load, reporting whether the path was on screen at all.
// Moving the cursor is what reads a secret, so a caller that jumped straight to
// one would otherwise be looking at an empty pane.
func (m *Model) FocusPath(path string, ctx ui.Context) (tea.Cmd, bool) {
	m.adopt(ctx)

	for i, r := range m.tree.visible {
		if r.kind != rowNode {
			continue
		}
		n := r.node
		if n.fullPath() == path || (n.kind != kindSecret && n.fullPath() == trimSlash(path)+"/") {
			m.tree.cursor = i
			return m.drain(m.onCursorMoved()), true
		}
	}
	return nil, false
}

// ModalTitle is what the open confirmation asks, or "".
func (m *Model) ModalTitle() string {
	if c, ok := m.currentMode().(confirming); ok && c.confirmation != nil {
		return c.confirmation.title
	}
	return ""
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
