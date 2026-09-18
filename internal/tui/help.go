package tui

import (
	"fmt"
	"strings"

	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The "?" panel: the full key list for where the user actually is. Load-bearing
// and not convenient - the legend may drop the destructive actions on a narrow
// terminal precisely because this reaches them in one keystroke.

// openHelp puts the panel up. It closes on esc, q, ? or i, which handleKey
// answers for every modal that has nothing else to do.
func (m *Model) openHelp() { m.mode = showingHelp{} }

// ui.HelpSection is one titled block of the "?" panel.

// commandSection is the colon commands available now, generated from the table
// so a new one cannot be added without appearing here. Only the keyless ones:
// the rest are keystrokes the section above already names.
func (m *Model) commandSection() ui.HelpSection {
	rows := make([][2]string, 0, 6)
	for _, c := range m.available() {
		if c.key == "" {
			rows = append(rows, [2]string{c.label(), c.title})
		}
	}
	rows = append(rows, [2]string{":", "every command, with its key"})

	return ui.HelpSection{Title: "Commands", Rows: rows}
}

// renderHelp is the full key list for where the user actually is, contextual
// for the reason the legend truncates: a list spanning every tab is one nobody
// finishes. It ends with the meta keys, which act on the app and not the node.
func (m *Model) renderHelp(bodyHeight int) string {
	meta := ui.HelpSection{Title: "Anywhere", Rows: [][2]string{
		{"tab, 1-4", "switch tab"},
		{"i", "token information"},
		{"R", "reload what this tab shows"},
		{"home / end", "top / bottom"},
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
	sections = append(sections, meta, m.commandSection())

	var b strings.Builder
	for i, s := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(m.st.ModalTitle.Render(s.Title))
		b.WriteString("\n\n")
		for _, r := range s.Rows {
			fmt.Fprintf(&b, "  %-16s %s\n", r[0], r[1])
		}
	}
	b.WriteString("\n")
	b.WriteString(m.st.Dim.Render("esc closes"))
	return m.st.ModalBox(strings.Split(b.String(), "\n"), bodyHeight, m.width)
}

// contextHelp is the section for where the user is, which every tab answers for
// itself: what it does, and how that splits by focus and by whatever it has up.
func (m *Model) contextHelp() ui.HelpSection {
	return m.currentTab().Help(m.uiContext())
}
