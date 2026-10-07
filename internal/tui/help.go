package tui

import (
	"github.com/lucasassuncao/bezel/layout"
	"github.com/lucasassuncao/bezel/legend"
	"github.com/lucasassuncao/bezel/overlay"

	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The "?" panel: the full key list for where the user actually is. Load-bearing
// and not convenient - the legend may drop the destructive actions on a narrow
// terminal precisely because this reaches them in one keystroke.

// openHelp puts the panel up. It closes on esc, q, ? or i, which handleKey
// answers for every modal that has nothing else to do.
func (m *Model) openHelp() {
	m.closeOverlays()
	m.sh = m.sh.Push(helpOverlay{m})
}

// ui.HelpSection is one titled block of the "?" panel.

// renderHelp is the full key list for where the user actually is, drawn by
// bezel's Help so every app's "?" panel looks alike. Contextual
// for the reason the legend truncates: a list spanning every tab is one nobody
// finishes. It ends with the meta keys, which act on the app and not the node.
func (m *Model) renderHelp(bodyHeight int) string {
	meta := ui.HelpSection{Title: "Anywhere", Rows: [][2]string{
		{"tab, 1-4", "switch tab"},
		{"i", "token information"},
		{"R", "reload what this tab shows"},
		{"home / end", "top / bottom"},
		{":", "every command, with its key"},
		{"?", "this panel   ·   q quit"},
	}}

	sections := []ui.HelpSection{m.contextHelp()}
	// The write keys stay listed below. This panel is the complete reference,
	// and hiding half of it would leave someone wondering whether vivi can edit
	// at all; the banner scopes everything under it instead.
	if !m.canWrite() {
		sections = append([]ui.HelpSection{{Title: "Read-only session", Rows: [][2]string{
			{"", "every write is refused: edit, create, delete, rollback, destroy"},
			{"", m.readOnlyReason()},
			{"", "reading, copying and the version list all still work"},
		}}}, sections...)
	}
	sections = append(sections, meta)

	out := make([]overlay.HelpSection, len(sections))
	for i, s := range sections {
		entries := make([]legend.Entry, len(s.Rows))
		for j, r := range s.Rows {
			entries[j] = legend.New(r[0], r[1])
		}
		out[i] = overlay.HelpSection{Name: s.Title, Entries: entries}
	}
	return overlay.NewHelp("", out, m.st.Modal, m.st.Shell().Legend).View(layout.Rect{W: m.width, H: bodyHeight})
}

// contextHelp is the section for where the user is, which every tab answers for
// itself: what it does, and how that splits by focus and by whatever it has up.
func (m *Model) contextHelp() ui.HelpSection {
	return m.currentTab().Help(m.uiContext())
}
