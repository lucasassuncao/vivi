package secrets

import (
	"github.com/lucasassuncao/bezel/layout"
	"github.com/lucasassuncao/bezel/legend"
)

// What a confirmation asks and how it is laid out. The box around it is
// ui.ModalBox, and the shell floats the result over the panes. A modal that
// takes text is bezel's overlay.Prompt, which draws itself.

func (m *Model) renderModal(c *confirmation, height int) string {
	if c.prompt != nil {
		return c.prompt.View(layout.Rect{W: m.width, H: height})
	}

	title := m.st.ModalTitle.Render(c.title)
	if c.danger {
		title = m.st.Danger.Render(c.title)
	}
	lines := append([]string{title, ""}, c.lines...)
	lines = append(lines, "", m.st.HintLine([]legend.Entry{legend.New("y", "confirm"), legend.New("n/esc", "cancel")}))
	return m.st.ModalBox(lines, height, m.width)
}
