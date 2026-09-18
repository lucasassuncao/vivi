package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
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

// render is the whole screen, and nothing but the order it is assembled in.
// Everything it calls lives with the component that owns it; what stays here is
// the one decision that is nobody's: which of them is on screen right now.
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

	header := m.renderHeader()
	tabs := m.renderTabs()
	bodyHeight := m.bodyHeight()

	// Each mode hands its renderer what it needs; what is checked is that it
	// brought anything at all, since the panes underneath are always a true
	// answer. A missing pointer here fails on every frame from then on.
	var body string
	switch mode := m.mode.(type) {
	case showingHelp:
		body = m.float(m.renderHelp(bodyHeight), bodyHeight)
	case showingToken:
		body = m.float(m.renderToken(bodyHeight), bodyHeight)
	case commanding:
		if mode.line == nil {
			break
		}
		body = m.renderCmdline(mode.line, bodyHeight)
	}
	// Then whatever the tab itself has up. The shell centres it over the panes,
	// which is the half of the drawing a tab cannot do: the panes are here.
	if body == "" {
		if over := m.currentTab().Overlay(m.contextIn(bodyHeight)); over != "" {
			body = m.float(over, bodyHeight)
		}
	}
	if body == "" {
		body = m.renderPanes(bodyHeight)
	}

	// The footer is rendered last because it reports on the body: the scroll
	// position it shows comes from the viewport the body just filled.
	footer := m.renderFooter()

	// The final clip is the guarantee: whatever any section decided, the screen
	// is never taller than the terminal. On a window too small to hold header,
	// tabs, body and footer at once, the bottom is what gives.
	return clipHeight(strings.Join([]string{header, "", tabs, "", body, footer}, "\n"), m.height)
}

// tooSmall is the whole screen when the terminal cannot hold the app. It names
// the size needed, the only part the user can act on, and cuts the lines first:
// a notice about a short screen must not be the thing overflowing it.
func (m *Model) tooSmall() string {
	lines := []string{
		ui.Truncate(m.st.Warn.Render("terminal too small"), m.width),
		ui.Truncate(m.st.Dim.Render(fmt.Sprintf("need %d×%d", minTerminalWidth, minTerminalHeight)), m.width),
	}
	return clipHeight(
		lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, strings.Join(lines, "\n")),
		m.height,
	)
}
