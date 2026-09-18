package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The frame around a pane, which is the parent's and not a tab's: a component
// says what goes inside, and never how wide the box around it is. The furniture
// inside the box lives in ui, where every tab can reach it.

// titledPanel draws a rounded panel with its title set into the top edge. width
// and height are outer dimensions, borders included; the focused panel draws a
// heavier border and reverses its title, so focus survives a monochrome terminal.
func (m *Model) titledPanel(title string, width, height int, active bool, content string) string {
	// The title is server-supplied text for every pane but the four fixed ones -
	// a path, a policy or a role name - so it is drawn under the same rules.
	// Here covers both panes; the fixed labels pass through untouched.
	title = ui.Sanitize(title)

	width, height = max(width, 4), max(height, 3)

	// Focus is carried by the border's characters and not only its colour. Under
	// NO_COLOR the two panels rendered byte for byte the same, and nothing said
	// which the arrows drove. A thicker box survives: a glyph is not a colour.
	border, titleStyle, edge := m.st.BorderOff, m.st.TitleIdle, lipgloss.RoundedBorder()
	if active {
		border, titleStyle, edge = m.st.BorderOn, m.st.PaneTitle, lipgloss.ThickBorder()
	}

	innerW := width - 2

	segment := titleStyle.Render(" " + title + " ")
	// A title wider than the panel would push the top edge past the panel's
	// right column and break the side by side join.
	if maxTitle := innerW - 1; maxTitle > 0 && lipgloss.Width(segment) > maxTitle {
		segment = ui.Truncate(segment, maxTitle)
	}

	fill := max(innerW-1-lipgloss.Width(segment), 0)
	// The top edge is drawn by hand because the title sits inside it, so it has
	// to be built from the same border set the body below uses or the two halves
	// of the box would not meet.
	top := border.Render(edge.TopLeft+edge.Top) + segment +
		border.Render(strings.Repeat(edge.Top, fill)+edge.TopRight)

	// The body carries every edge but the top, which the title line drew. Width
	// and height are the block's own in v2, borders and padding counted in, so
	// they are the panel's outer width and everything below its title line. In
	// v1 they described the content instead and read innerW and height-2.
	body := lipgloss.NewStyle().
		Border(edge).
		BorderTop(false).
		BorderForeground(m.borderColor(active)).
		Padding(0, 1).
		Width(width).
		Height(height - 1).
		MaxWidth(width).
		Render(ui.FitPanel(content, innerW-2, height-2))

	return lipgloss.JoinVertical(lipgloss.Left, top, body)
}

func (m *Model) borderColor(active bool) color.Color {
	if active {
		return m.st.Pal.Accent
	}
	return m.st.Pal.Border
}
