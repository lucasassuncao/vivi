package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// Where a keystroke goes: the mode decides first, then the tab. Each handler
// lives with its own component - keySecrets in secrets.go, keyEdit in edit.go -
// so this file is only the routing, plus the keys that mean one thing anywhere.

// normalizeKey undoes what Windows makes of AltGr. A layout that puts "?" and
// "/" behind it - ABNT2 among them - reports Ctrl+Alt, so the rune arrives with
// Alt set and a NUL beside it, and "alt+?" matches nothing in the table below.
//
// Stripping Alt is safe because vivi binds no Alt combination at all: an
// Alt-modified rune can only be a layout saying the rune. The NUL is the other
// half of the pair and is dropped, or it would be typed into a filter.
func normalizeKey(msg tea.KeyPressMsg) (tea.KeyPressMsg, bool) {
	if msg.Text == "" || !msg.Mod.Contains(tea.ModAlt) {
		return msg, true
	}
	text := strings.ReplaceAll(msg.Text, "\x00", "")
	if text == "" {
		return msg, false
	}
	msg.Text, msg.Mod = text, msg.Mod&^tea.ModAlt
	// Code is what String() matches on, and it carries the NUL of the pair as
	// readily as Text did.
	msg.Code = []rune(text)[0]
	return msg, true
}

// handleKey routes a keystroke to whatever is in focus.
func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	m.debugKey(msg)

	// Before anything reads it, including the editor and the filter input.
	msg, ok := normalizeKey(msg)
	if !ok {
		return nil
	}

	if msg.String() == "ctrl+c" {
		return tea.Quit
	}

	// The shell's own modals answer first. The mode carries what its handler
	// needs, so a handler never asks whether what it operates on exists - only
	// whether the mode carries anything. A payload-less mode answers no key,
	// and quitting would be the way out.
	switch mode := m.mode.(type) {
	case commanding:
		if mode.line == nil {
			break
		}
		return m.keyCommand(mode.line, msg)
	case showingToken:
		return m.keyToken(msg)
	case showingHelp:
		if dismisses(msg) {
			m.mode = browsing{}
		}
		return nil
	}

	// Then the tab, if it has something of its own taking every keystroke: a
	// confirmation, a form, a filter. The shell's shortcuts stand aside, or "q"
	// would quit out of a half-typed value.
	if t := m.currentTab(); t.Captures() {
		return t.Key(msg, m.uiContext())
	}

	return m.keyBrowse(msg)
}

// dismisses is the set of keys that close a panel with nothing else to do.
func dismisses(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "esc", "q", "?", "i":
		return true
	}
	return false
}

// keyToken drives the token panel, the one modal with an action of its own:
// it is the only place that knows whether the token can be renewed.
func (m *Model) keyToken(msg tea.KeyPressMsg) tea.Cmd {
	if msg.String() == "n" {
		if m.token == nil || !m.token.Renewable {
			m.notify("this token is not renewable")
			return nil
		}
		return m.renewToken()
	}
	if dismisses(msg) {
		m.mode = browsing{}
	}
	return nil
}

// switchTab shows a tab from the top of its list, fetching its data on the
// first visit. The number keys and ":tab" both land here.
func (m *Model) switchTab(t tab) tea.Cmd {
	m.setTab(t)
	m.detail.GotoTop()
	m.focus = ui.FocusList
	return m.ensureTabLoaded()
}

func (m *Model) keyBrowse(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "q":
		return tea.Quit
	case ":":
		// The commands are the same actions these keys run, listed by name and
		// with the key printed beside each. See cmdline.
		return m.openCmdline()
	case "?":
		m.openHelp()
		return nil
	case "i":
		return m.openToken()
	case "R":
		// Here and not in the tab handlers: reloadCurrent already answers for
		// every tab, and only the secrets one ever called it, so the "R reload"
		// the legend prints on Policies and Auth was a key nothing read.
		return m.reloadCurrent()
	case "tab":
		m.nextTab(1)
		return m.ensureTabLoaded()
	case "shift+tab":
		m.nextTab(-1)
		return m.ensureTabLoaded()
	case "1", "2", "3", "4":
		return m.switchTab(tab(int(msg.String()[0] - '1')))
	}

	// With the detail pane focused, the movement keys drive the document
	// instead of the list. Anything else falls through, so r, y, V and the tab
	// keys keep working without having to leave the pane first.
	if m.focus == ui.FocusDetail {
		if cmd, handled := m.keyDetail(msg); handled {
			return cmd
		}
	}

	return m.currentTab().Key(msg, m.uiContext())
}

// keyDetail drives the focused detail pane, reporting whether it consumed the
// key. On a secret the arrows walk fields, because the one under the cursor is
// what r and y act on; elsewhere nothing is selectable, so they scroll.
func (m *Model) keyDetail(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	// A pane holding a list of its own answers the arrows itself: the row under
	// the cursor is what the other keys act on, so scrolling past it would
	// leave them aimed at something off screen.
	if cmd, took := m.currentTab().KeyDetail(msg, m.uiContext()); took {
		return cmd, true
	}

	switch msg.String() {
	case "down":
		m.detail.ScrollDown(1)
	case "up":
		m.detail.ScrollUp(1)
	case "home":
		m.detail.GotoTop()
	case "end":
		m.detail.GotoBottom()
	case "pgdown":
		m.detail.HalfPageDown()
	case "pgup":
		m.detail.HalfPageUp()
	case "left", "esc":
		// Left mirrors the right that got here, and is consumed either way:
		// falling through would collapse a node in the tree behind the pane,
		// where the cursor is not, and the user would not see what moved.
		m.focus = ui.FocusList
	default:
		return nil, false
	}
	return nil, true
}

// scrollToLine brings one line of the detail pane into view, for the tab that
// has a selection inside a document: a selection scrolled out of sight is worse
// than no selection, because the keys still act on it.
func (m *Model) scrollToLine(line int) {
	switch {
	case line < m.detail.YOffset():
		m.detail.SetYOffset(line)
	case m.detail.Height() > 0 && line >= m.detail.YOffset()+m.detail.Height():
		m.detail.SetYOffset(line - m.detail.Height() + 1)
	}
}
