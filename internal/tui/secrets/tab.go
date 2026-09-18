package secrets

import (
	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// What the shell asks of this tab. Every method adopts the session's view of
// itself first and drains what the call collected last, so the code underneath
// reads exactly as it read when all of this was one package.

// Load fetches the mounts, once. Opening the tree lists the mounts and nothing
// else; a path is read the first time it is expanded.
func (m *Model) Load() tea.Cmd {
	if m.mountsAsked {
		return nil
	}
	m.mountsAsked = true
	return m.loadMounts()
}

// Reload drops what the cursor is on and fetches it again: a secret is re-read,
// anything else has its listing thrown away and asked for afresh.
func (m *Model) Reload() tea.Cmd {
	n := m.tree.current()
	if n == nil {
		return nil
	}
	if n.kind == kindSecret {
		return m.drain(m.selectSecret(n, 0))
	}
	return m.drain(m.reload(n))
}

// Key routes a keystroke. The tab's own modal answers first, exactly as the one
// switch in the shell used to: a handler never asks whether what it operates on
// exists, only whether the mode carries anything.
func (m *Model) Key(msg tea.KeyPressMsg, ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	m.ensureState()
	return m.drain(m.key(msg))
}

func (m *Model) key(msg tea.KeyPressMsg) tea.Cmd {
	switch mode := m.currentMode().(type) {
	case confirming:
		if mode.confirmation == nil {
			break
		}
		return m.keyConfirm(mode.confirmation, msg)
	case editing:
		if mode.editor == nil {
			break
		}
		return m.keyEdit(mode.editor, msg)
	case filtering:
		return m.keyFilter(msg)
	case choosingVersion:
		return m.keyVersions(msg)
	case showingDiff:
		return m.keyDiff(mode.diff, msg)
	}
	return m.keySecrets(msg)
}

// KeyDetail walks the fields of the secret on screen. The one under the cursor
// is what r and y act on, so the arrows move the selection rather than the
// document; with nothing to select the shell scrolls instead.
func (m *Model) KeyDetail(msg tea.KeyPressMsg, ctx ui.Context) (tea.Cmd, bool) {
	m.adopt(ctx)

	keys := m.sortedKeys()
	if len(keys) == 0 {
		return nil, false
	}

	switch msg.String() {
	case "down":
		m.moveField(1)
	case "up":
		m.moveField(-1)
	case "home":
		m.fieldCursor = 0
		m.followFieldCursor()
	case "end":
		m.fieldCursor = len(keys) - 1
		m.followFieldCursor()
	default:
		return nil, false
	}
	return m.drain(nil), true
}

// Captures reports that a modal or an input here is taking every keystroke, so
// the shell's own shortcuts stand aside. It is the same question the single
// mode switch used to answer by falling through or not.
func (m *Model) Captures() bool { return !modeIs[browsing](m) }

// Resize re-lays the one widget that is sized to the terminal rather than
// measured at render: a text area has to know its width to wrap and its height
// to scroll, so it cannot wait for the next frame to find out.
func (m *Model) Resize(ctx ui.Context) {
	m.adopt(ctx)
	if e := m.currentEditor(); e != nil && e.form != nil {
		e.form.resize(m.width, m.bodyHeight())
	}
}

// Blur is the tab leaving the front. The version list and the filter are this
// tab's transient state and the jumps run from inside them: one of them left
// open behind another tab answers keys the user can no longer see.
func (m *Model) Blur() { m.mode = browsing{} }

func (m *Model) Data(msg tea.Msg) tea.Cmd {
	m.ensureState()
	return m.drain(m.handleData(msg))
}

func (m *Model) Titles() (left, right string) {
	if e := m.currentEditor(); e != nil {
		// The title said "editing" while the body said "creating", which is the
		// one distinction that changes what the save does.
		if e.creating {
			return "Secrets", "creating " + e.path
		}
		return "Secrets", "editing " + e.path
	}
	// shownNode, not the cursor: while a secret is being read the pane is still
	// showing the previous one, and a title running ahead of the body would put
	// one secret's name over another's fields.
	if n := m.shownNode(); n != nil {
		return "Secrets", n.fullPath()
	}
	return "Secrets", "Detail"
}

func (m *Model) RenderList(width, height int, ctx ui.Context) string {
	m.adopt(ctx)
	return m.renderTree(width, height)
}

func (m *Model) RenderDetail(width int, ctx ui.Context) string {
	m.adopt(ctx)
	return m.renderSecretDetail(width)
}

// Overlay is what this tab draws over the panes: a confirmation, the editor's
// form, or a diff. The shell centres it, because the panes underneath are the
// shell's and this tab cannot see them.
func (m *Model) Overlay(ctx ui.Context) string {
	m.adopt(ctx)

	switch mode := m.currentMode().(type) {
	case confirming:
		if mode.confirmation == nil {
			return ""
		}
		return m.renderModal(mode.confirmation, m.height)
	case editing:
		// The table is a pane, drawn by this tab; only the form floats.
		if mode.editor == nil || mode.editor.form == nil {
			return ""
		}
		return m.renderFieldForm(mode.editor, mode.editor.form, m.height)
	case showingDiff:
		if mode.diff == nil {
			return ""
		}
		return m.renderDiff(mode.diff, m.height)
	}
	return ""
}

func (m *Model) Legend(ctx ui.Context) (status string, keys []ui.LegendEntry) {
	m.adopt(ctx)

	// The count describes what the left pane is listing. The tree has none: it
	// is lazy, so a node count reports what was expanded rather than what is
	// there. The filter still reports itself - the user put it there.
	switch {
	case m.tree.filter != "":
		status = "filter: " + m.tree.filter
	case modeIs[filtering](m):
		status = m.filterInput.View()
	}

	if hs := m.modeLegend(); hs != nil {
		return status, hs
	}
	if ctx.Focus == ui.FocusDetail {
		// The actions belong here too, since the shell lets them fall through.
		// r and y name what they act on, and the words differ from the list's:
		// here the cursor is on a field, there it is on the secret.
		head := []ui.LegendEntry{ui.Entry("esc/←", "back to list"), ui.Entry("pgup/pgdn", "scroll")}
		return status, ui.ListLegend("fields",
			append(head, m.actions(m.revealFieldLegend(), copyFieldLegend, paneDeleteLegend, true)...)...)
	}
	head := append(m.openLegend(), ui.Entry("/", "filter"))
	return status, ui.ListLegend("move",
		append(head, m.actions(m.revealSecretLegend(), copySecretLegend, listDeleteLegend, false)...)...)
}

// actions are the keys that act on what the cursor is on, each shown only where
// it would do something. They are scoped on exactly the predicates the command
// palette scopes its own entries on, because a legend and a palette that
// disagree are two lists and only one of them is checked.
func (m *Model) actions(reveal, take, remove ui.LegendEntry, inPane bool) []ui.LegendEntry {
	var out []ui.LegendEntry
	if m.HasField() {
		out = append(out, reveal, take)
	}
	if m.HasSecret() {
		out = append(out, ui.WriteEntry("e", "edit"))
	}
	// Creating lands in the folder the tree cursor is in, so it belongs to the
	// list. Offered beside the field keys it read as adding a field.
	if !inPane && m.CanCreateHere() {
		out = append(out, ui.WriteEntry("a", "create"))
	}
	if m.HasVersions() {
		out = append(out, ui.Entry("v", "versions"))
	}
	if (inPane && m.HasField()) || (!inPane && m.SelectedSecret()) {
		out = append(out, remove)
	}
	return out
}

func (m *Model) Help(ctx ui.Context) ui.HelpSection {
	m.adopt(ctx)

	if modeIs[choosingVersion](m) {
		return ui.HelpSection{Title: "Version list", Rows: [][2]string{
			{"↑ / ↓", "move"},
			{"space", "mark a version"},
			{"enter", "read the marked version"},
			{"d", "diff two marked versions"},
			// Each line answers what the key itself does not: what it touches,
			// and what is left behind. "destroy" and "delete all" sound
			// interchangeable, and only one takes the secret out of the listing.
			{"b", "rollback: old content back as a new version"},
			{"u", "undelete: reverse d, the same version returns"},
			{"D", "destroy: version data erased, secret still listed"},
			{"M", "delete secret: every version and the metadata, unlisted"},
			{"esc", "back"},
		}}
	}
	if ctx.Focus == ui.FocusDetail {
		return ui.HelpSection{Title: "Detail pane", Rows: [][2]string{
			{"↑ / ↓", "move between fields, or scroll"},
			{"pgup / pgdn", "half a page"},
			{"r", "reveal the selected field, or hide it again"},
			{"y", "copy the selected field, without showing it"},
			{"e", "edit the secret   ·   x removes a field in there too"},
			{"v", "version list"},
			{"d", "remove the selected field: a new version without it"},
			{"esc / ←", "back to the list, where d takes the whole secret"},
		}}
	}
	return ui.HelpSection{Title: "Secrets", Rows: [][2]string{
		{"↑ / ↓", "move"},
		{"→ / enter", "open a folder; on a secret, step into the pane"},
		{"←", "close a folder, or go to the parent"},
		{"/", "filter what is already loaded"},
		{"r", "reveal every field of the secret, or hide them again"},
		{"y", "copy the whole secret as JSON, without showing it"},
		{"e", "edit   ·   a create secret"},
		{"v", "version list"},
		{"d", "delete"},
	}}
}
