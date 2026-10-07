package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lucasassuncao/bezel/draw"
	"github.com/lucasassuncao/bezel/layout"
	"github.com/lucasassuncao/bezel/shell"
)

// The smallest terminal vivi will draw into. Below either number the bottom of
// the screen gives, taking the legend and with it the only thing that says "?"
// reaches the key list - so the app names what it needs instead of clipping.
const (
	minTerminalWidth  = 30
	minTerminalHeight = 10
)

// View is what bubbletea draws, and in v2 it is the screen plus the terminal
// state that goes with it: vivi asks for the alternate screen here rather than
// at NewProgram, which is where the option used to live.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

// render is the whole screen: the shell draws the chrome, the panes and its
// own overlays; what stays here is the tab's overlay, which the shell cannot
// see because it belongs to the tab.
func (m *Model) render() string {
	// A panic while drawing is as fatal as one while updating, and it is the
	// one that repeats: it fires on every frame from then on. Same barrier.
	defer m.guardPanic()

	// Before the first WindowSizeMsg there is no terminal to draw into. <= and
	// not ==, because a width the layout cannot work with is the same situation
	// as not knowing it yet, and the arithmetic below assumes a positive one.
	if m.width <= 0 || m.height <= 0 {
		return "loading…"
	}
	if m.width < minTerminalWidth || m.height < minTerminalHeight {
		return m.tooSmall()
	}

	out := m.sh.View(m.panes())
	// A tab's modal - a confirmation, a form, a diff - floats over the panes
	// but under the shell's own panels, which answer for themselves.
	if m.browsing() {
		if over := m.currentTab().Overlay(m.contextIn(m.bodyHeight())); over != "" {
			body := m.sh.Body()
			over = draw.FitBlock(over, body.W, body.H)
			w, h := draw.BlockSize(over)
			out = draw.Composite(over, out, body.X+max(0, (body.W-w)/2), body.Y+max(0, (body.H-h)/2))
		}
	}
	return out
}

// panes is what goes inside the shell's panels this frame. The reference panel
// is only handed over when the layout gave it rows.
func (m *Model) panes() map[string]shell.Pane {
	left, right := m.currentTab().Titles()
	panes := map[string]shell.Pane{
		paneList: {Title: draw.Sanitize(left), Body: func(r layout.Rect) string {
			return m.renderList(r.W, r.H)
		}},
		paneDetail: {Title: draw.Sanitize(right), Body: func(layout.Rect) string { return m.detailView() }},
	}
	if m.sh.Rect(paneCopy) != (layout.Rect{}) {
		panes[paneCopy] = shell.Pane{Title: copyPanelTitle, Body: func(r layout.Rect) string {
			return strings.Join(m.copyLines(r.W), "\n")
		}}
	}
	return panes
}

// tooSmall is the whole screen when the terminal cannot hold the app. It names
// the size needed, the only part the user can act on, and cuts the lines first:
// a notice about a short screen must not be the thing overflowing it.
func (m *Model) tooSmall() string {
	lines := []string{
		draw.Cut(m.st.Warn.Render("terminal too small"), m.width),
		draw.Cut(m.st.Dim.Render(fmt.Sprintf("need %d×%d", minTerminalWidth, minTerminalHeight)), m.width),
	}
	return draw.FitBlock(
		lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, strings.Join(lines, "\n")),
		m.width, m.height,
	)
}

// renderList is the left pane of the active tab, filled while rendering.
func (m *Model) renderList(width, height int) string {
	return m.currentTab().RenderList(width, height, m.uiContext())
}

// renderDetail is the right pane of the active tab, filled from syncDetail.
func (m *Model) renderDetail(width int) string {
	return m.currentTab().RenderDetail(width, m.uiContext())
}

// detailView is the detail viewport, with "↓ N more lines" on the row
// syncDetail kept for it while anything is below the pane.
func (m *Model) detailView() string {
	rest := m.detail.TotalLineCount() - m.detail.YOffset() - m.detail.Height()
	if m.detail.Height() <= 0 || rest <= 0 {
		return m.detail.View()
	}
	return m.detail.View() + "\n" + m.st.Dim.Render(moreLines(rest))
}

// moreLines is the count under a pane, singular for one.
func moreLines(n int) string {
	if n == 1 {
		return "  ↓ 1 more line"
	}
	return fmt.Sprintf("  ↓ %d more lines", n)
}
