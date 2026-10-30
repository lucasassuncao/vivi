package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/bezel"
	"github.com/lucasassuncao/bezel/draw"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The mouse does what a key already does, so nothing here is a new action: a
// click on a tab switches to it, on a row moves the cursor there, on the row
// under the cursor does what enter does. The wheel scrolls the pane under it.

// wheelLines is how far one notch of the wheel scrolls the detail.
const wheelLines = 3

// mouse routes a click or a wheel turn. The shell's panels answer for their own
// clicks; a modal of the tab's takes the keyboard, so the mouse stands aside.
func (m *Model) mouse(msg tea.MouseMsg) tea.Cmd {
	if m.showingCommands() || m.sh.TopOverlay() != nil {
		var r bezel.Result
		m.sh, r = m.sh.Update(msg)
		return r.Cmd
	}
	if m.currentTab().Captures() {
		return nil
	}
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			return m.click(msg)
		}
	case tea.MouseWheelMsg:
		return m.wheel(msg)
	}
	return nil
}

// click is a left click on the tab strip or on a pane.
func (m *Model) click(msg tea.MouseClickMsg) tea.Cmd {
	at := msg.Mouse()
	if at.Y == len(m.headerLines()) {
		if i := draw.TabAt(m.width, tabNames, int(m.tab), m.st.Shell().Chrome, at.X); i >= 0 {
			return m.switchTab(tab(i))
		}
		return nil
	}
	pane, ok := m.sh.PaneAt(at.X, at.Y)
	if !ok {
		return nil
	}
	in := m.sh.MouseIn(pane, msg).Mouse()
	switch pane {
	case paneList:
		m.focus = ui.FocusList
		return m.currentTab().ClickList(in.Y, draw.InnerRect(m.sh.Rect(paneList)).H, m.uiContext())
	case paneDetail:
		m.focus = ui.FocusDetail
		return m.currentTab().ClickDetail(in.Y+m.detail.YOffset(), m.uiContext())
	case paneCopy:
		return m.copyRowAt(in.Y)
	}
	return nil
}

// copyRowAt copies the Command Reference row clicked, as its :copy command
// does. Mouse reporting takes the terminal's own selection, which this repays.
func (m *Model) copyRowAt(y int) tea.Cmd {
	rows := m.copyRows()
	if y < 0 || y >= len(rows) {
		return nil
	}
	r := rows[y]
	return m.sh.Copy(r.Text, "copied: "+r.Text, "copy "+r.Label)
}

// wheel scrolls the pane under the pointer: the list moves its cursor, as the
// arrows do, and the detail scrolls its document.
func (m *Model) wheel(msg tea.MouseWheelMsg) tea.Cmd {
	at := msg.Mouse()
	var up bool
	switch at.Button {
	case tea.MouseWheelUp:
		up = true
	case tea.MouseWheelDown:
	default:
		return nil
	}
	pane, _ := m.sh.PaneAt(at.X, at.Y)
	switch pane {
	case paneList:
		key := tea.KeyPressMsg{Code: tea.KeyDown}
		if up {
			key = tea.KeyPressMsg{Code: tea.KeyUp}
		}
		ctx := m.uiContext()
		ctx.Focus = ui.FocusList
		return m.currentTab().Key(key, ctx)
	case paneDetail:
		if up {
			m.detail.ScrollUp(wheelLines)
		} else {
			m.detail.ScrollDown(wheelLines)
		}
	}
	return nil
}
