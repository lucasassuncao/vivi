package secrets

import (
	"github.com/lucasassuncao/bezel/legend"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// What "y" copies, named for what the focus makes it: the cursor is on a secret
// in the list and on a field in the pane, so the two bars say different words
// and the contrast is the whole rule.
var (
	copySecretLegend = legend.New("y", "copy secret")
	copyFieldLegend  = legend.New("y", "copy field")

	// "d" takes what the cursor is on, which is a field in the pane and the
	// secret in the list. A key that took the secret from beside "reveal field"
	// and "copy field" read as taking the field, and did not.
	paneDeleteLegend = legend.New("d", "delete field", ui.CapWrite)

	// M sits beside d so the reversible delete and the final one read as a pair.
	destroySecretLegend = legend.New("M", "destroy secret", ui.CapWrite)
)

// listDeleteLegend says "soft" only where undelete exists: on v1, d is final.
func (m *Model) listDeleteLegend() legend.Entry {
	if m.SelectedVersioned() {
		return legend.New("d", "soft delete", ui.CapWrite)
	}
	return legend.New("d", "delete", ui.CapWrite)
}

// "r" is a toggle, so the bar names what the next press does rather than what
// the key is called. It used to conjugate on the selected row, which said it
// only for the field and only in the pane; here it says it for both scopes.

// revealSecretLegend reads "hide" only once every field is on screen.
func (m *Model) revealSecretLegend() legend.Entry {
	if m.allFieldsRevealed() {
		return legend.New("r", "hide secret")
	}
	return legend.New("r", "reveal secret")
}

// revealFieldLegend follows the one field under the cursor.
func (m *Model) revealFieldLegend() legend.Entry {
	if m.currentFieldRevealed() {
		return legend.New("r", "hide field")
	}
	return legend.New("r", "reveal field")
}

// openLegend describes what right and enter do to the node under the cursor.
// Both expand a folder and both step into an already-loaded secret, so they
// share a slot; on a narrow terminal the verb changes because there is one pane.
func (m *Model) openLegend() []legend.Entry {
	n := m.tree.current()
	if n == nil || n.kind != kindSecret {
		return []legend.Entry{legend.New("→/enter", "open folder")}
	}
	if m.width < ui.NarrowLimit {
		return []legend.Entry{legend.New("→/enter", "open secret")}
	}
	return []legend.Entry{legend.New("→/enter", "focus detail")}
}

// editorKeys is what the editor's table offers. The footer and the line drawn
// under the table are the same list, from here: as two lists they drifted, and
// the line went on naming keys the bar had already stopped offering.
func editorKeys() []legend.Entry {
	return []legend.Entry{
		legend.New("enter", "edit this value"), legend.New("a", "add a key", ui.CapWrite),
		legend.New("x", "remove", ui.CapWrite), legend.New("ctrl+s", "save", ui.CapWrite), legend.New("esc", "cancel"),
	}
}

// modeLegend is the legend of the screen this tab has up, or nil when the
// panes are what the user is looking at. A modal never changes it: the field
// form and every confirmation draw their keys inside their box.
func (m *Model) modeLegend() []legend.Entry { return m.legendOf(m.currentMode()) }

func (m *Model) legendOf(md mode) []legend.Entry {
	switch md := md.(type) {
	case editing:
		return editorKeys()
	case confirming:
		return m.legendOf(md.under)
	case choosingVersion:
		// One pair per key, not "b/u": a slash reads as "the same thing". The
		// irreversible one goes last, so a narrow terminal drops it first.
		return ui.ListLegend("move", legend.New("space", "mark"),
			legend.New("enter", "read"), legend.New("esc", "back"), legend.New("d", "diff"),
			legend.New("b", "rollback", ui.CapWrite), legend.New("u", "undelete", ui.CapWrite),
			legend.New("D", "destroy version", ui.CapWrite),
		)
	case filtering:
		return []legend.Entry{legend.New("type", "to filter"), legend.New("enter", "apply"), legend.New("esc", "clear")}
	}
	return nil
}
