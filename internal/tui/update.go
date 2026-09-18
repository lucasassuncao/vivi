package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// What the shell does with an answer. Its own are the token and the clipboard;
// everything else belongs to a tab, and is offered to all four.

func (m *Model) handleData(msg tea.Msg) tea.Cmd {
	// One request, one answer: the count is settled here rather than at the top
	// of every branch below, the same way Request raises it in one place rather
	// than at each of the dozen call sites.
	if _, ok := msg.(ui.RequestAnswer); ok {
		m.pending--
	}

	switch msg := msg.(type) {
	case copiedMsg:
		// The clipboard is not a request against the server, so it is not
		// counted in m.pending: it answers in a moment or it answers with the
		// timeout, and either way the loop kept running.
		if msg.err != nil {
			return m.fail(msg.op, msg.err)
		}
		m.notify(msg.what)
		return nil

	case tokenMsg:
		if msg.err != nil {
			return m.fail("look up token", msg.err)
		}
		m.token = msg.info
		return nil

	case renewMsg:
		if msg.err != nil {
			return m.fail("renew token", msg.err)
		}
		m.token = msg.info
		m.notify("token renewed")
		return nil
	}

	// The tabs, and the shell's own nested input: the command line is a Bubbles
	// model with a caret that blinks on a message of its own, and a blink that
	// never arrives is a caret that stalls.
	return tea.Batch(m.routeToTabs(msg), m.routeToInput(msg))
}

// routeToTabs offers a message to every tab. All four and not the first that
// answers: the answer types are each package's own and unexported, so a tab can
// only match what it asked for, and one returning nothing is not the same as
// one not recognising it. Every tab is offered it and not only the open one,
// because a jump can be answered while another tab is in front of the user.
func (m *Model) routeToTabs(msg tea.Msg) tea.Cmd {
	return tea.Batch(
		m.secretsTab.Data(msg),
		m.policiesTab.Data(msg),
		m.authTab.Data(msg),
		m.activityTab.Data(msg),
	)
}

// jumpToPolicy switches to the Policies tab and asks it to reach a policy by
// name. Whoever wanted it - the Auth tab, or a typed command - never names the
// tab that answers, which is what keeps a tab from importing a tab.
func (m *Model) jumpToPolicy(name string) tea.Cmd {
	m.setTab(tabPolicies)
	return m.policiesTab.Jump(name)
}
