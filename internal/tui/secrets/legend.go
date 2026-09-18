package secrets

import "github.com/lucasassuncao/vivi/internal/tui/ui"

// What "y" copies, named for what the focus makes it: the cursor is on a secret
// in the list and on a field in the pane, so the two bars say different words
// and the contrast is the whole rule.
var (
	copySecretLegend = ui.Entry("y", "copy secret")
	copyFieldLegend  = ui.Entry("y", "copy field")

	// "d" takes what the cursor is on, which is a field in the pane and the
	// secret in the list. A key that took the secret from beside "reveal field"
	// and "copy field" read as taking the field, and did not.
	listDeleteLegend = ui.WriteEntry("d", "delete")
	paneDeleteLegend = ui.WriteEntry("d", "delete field")
)

// "r" is a toggle, so the bar names what the next press does rather than what
// the key is called. It used to conjugate on the selected row, which said it
// only for the field and only in the pane; here it says it for both scopes.

// revealSecretLegend reads "hide" only once every field is on screen.
func (m *Model) revealSecretLegend() ui.LegendEntry {
	if m.allFieldsRevealed() {
		return ui.Entry("r", "hide secret")
	}
	return ui.Entry("r", "reveal secret")
}

// revealFieldLegend follows the one field under the cursor.
func (m *Model) revealFieldLegend() ui.LegendEntry {
	if m.currentFieldRevealed() {
		return ui.Entry("r", "hide field")
	}
	return ui.Entry("r", "reveal field")
}

// openLegend describes what right and enter do to the node under the cursor.
// Both expand a folder and both step into an already-loaded secret, so they
// share a slot; on a narrow terminal the verb changes because there is one pane.
func (m *Model) openLegend() []ui.LegendEntry {
	n := m.tree.current()
	if n == nil || n.kind != kindSecret {
		return []ui.LegendEntry{ui.Entry("→/enter", "open folder")}
	}
	if m.width < ui.NarrowLimit {
		return []ui.LegendEntry{ui.Entry("→/enter", "open secret")}
	}
	return []ui.LegendEntry{ui.Entry("→/enter", "focus detail")}
}

// editorKeys is what the editor's table offers. The footer and the line drawn
// under the table are the same list, from here: as two lists they drifted, and
// the line went on naming keys the bar had already stopped offering.
func editorKeys() []ui.LegendEntry {
	return []ui.LegendEntry{
		ui.Entry("enter", "edit this value"), ui.WriteEntry("a", "add a key"),
		ui.WriteEntry("x", "remove"), ui.WriteEntry("ctrl+s", "save"), ui.Entry("esc", "cancel"),
	}
}

// modeLegend is the legend of the modal this tab has up, or nil when the panes
// are what the user is looking at.
func (m *Model) modeLegend() []ui.LegendEntry {
	switch {
	case modeIs[editing](m):
		// While the form is open, "a add" and "x remove" are not keys, they are
		// letters going into the value; the form carries its own hints.
		if e := m.currentEditor(); e != nil && e.form != nil {
			return []ui.LegendEntry{ui.Entry("tab", "next field"), ui.Entry("esc", "cancel")}
		}
		return editorKeys()
	case modeIs[confirming](m):
		return []ui.LegendEntry{ui.Entry("y", "confirm"), ui.Entry("esc", "cancel")}
	case modeIs[choosingVersion](m):
		// One pair per key, not "b/u" and "D/M": a slash reads as "the same
		// thing", and D and M differ on whether the secret survives. The two
		// irreversible ones go last, so a narrow terminal drops them first.
		return ui.ListLegend("move", ui.Entry("space", "mark"),
			ui.Entry("enter", "read"), ui.Entry("esc", "back"), ui.Entry("d", "diff"),
			ui.WriteEntry("b", "rollback"), ui.WriteEntry("u", "undelete"),
			ui.WriteEntry("D", "destroy version"), ui.WriteEntry("M", "delete secret"),
		)
	case modeIs[filtering](m):
		return []ui.LegendEntry{ui.Entry("type", "to filter"), ui.Entry("enter", "apply"), ui.Entry("esc", "clear")}
	}
	return nil
}
