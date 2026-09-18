package secrets

import "github.com/lucasassuncao/vivi/internal/tui/ui"

// What a confirmation asks and how it is laid out. The box around it is
// ui.ModalBox, and the shell floats the result over the panes.

func (m *Model) renderModal(c *confirmation, height int) string {
	title := m.st.ModalTitle.Render(c.title)
	if c.danger {
		title = m.st.Danger.Render(c.title)
	}

	lines := []string{title, ""}
	// A labelled input leads, with its preview under it, and the lines explain
	// below; the bare input follows the lines, the way a typed confirmation
	// reads: the warning first, then the place to answer it.
	if c.inputLabel != "" {
		lines = append(lines, m.formRow(c.inputLabel, c.input.View()))
		if c.preview != nil {
			lines = append(lines, m.formRow(c.previewLabel, c.preview(c.input.Value())))
		}
		lines = append(lines, "")
	}
	lines = append(lines, c.lines...)

	if (c.needsInput() || c.capture) && c.inputLabel == "" {
		lines = append(lines, "", c.input.View())
	}
	lines = append(lines, "", m.modalHint(c))

	return m.st.ModalBox(lines, height, m.width)
}

// modalHint is the last line: what accepts, what cancels. A typed confirmation
// that is not yet satisfied says so instead, since enter does nothing there.
func (m *Model) modalHint(c *confirmation) string {
	switch {
	case c.needsInput() && !c.satisfied():
		return m.st.Dim.Render("type the exact text to unlock") + "   " +
			m.st.HintLine([]ui.LegendEntry{ui.Entry("esc", "cancel")})
	case c.confirmLabel != "":
		return c.confirmLabel
	case c.needsInput() || c.capture:
		return m.st.HintLine([]ui.LegendEntry{ui.Entry("enter", "confirm"), ui.Entry("esc", "cancel")})
	default:
		return m.st.HintLine([]ui.LegendEntry{ui.Entry("y", "confirm"), ui.Entry("n/esc", "cancel")})
	}
}
