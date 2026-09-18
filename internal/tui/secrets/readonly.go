package secrets

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// A read-only session refuses a write in four places: the header badge, the
// palette, the key handlers, and mutate. Only mutate is load-bearing - the
// other three are about being understood; this one is about being right.

// canWrite reports whether this session may change anything.
func (m *Model) canWrite() bool { return m.access.CanWrite() }

// refuse reports a write a read-only session will not perform. It names the
// reason and not the state: VIVI_READ_ONLY=prod in a shell profile is a
// decision made weeks ago, and the user is not hunting a flag typed today.
func (m *Model) refuse(what string) tea.Cmd {
	msg := what + ": this session is read-only"
	if reason := m.readOnly.Reason(); reason != "" {
		msg += " - " + reason
	}
	return m.failWith(msg)
}

// mutate is onNode for the calls that change the Vault. Every write goes
// through here, which makes read-only a property of the session and not a set
// of disabled keys. Silent: the key handler that got here has already said so.
func (m *Model) mutate(n *node, call func(context.Context, vault.Client, nodeRef) ui.RequestAnswer) tea.Cmd {
	if !m.canWrite() {
		return nil
	}
	return m.onNode(n, call)
}

// gateFor asks app how much friction an operation earns in this session, which
// is where read-only joins the same scale as the confirmations.
func (m *Model) gateFor(op app.Operation, kvVersion int) app.Gate {
	return app.GateFor(op, kvVersion, m.access)
}

// blocks reports that an operation cannot run here and says so, for the key
// handlers. It returns true when the caller should stop.
func (m *Model) blocks(op app.Operation, kvVersion int, what string) bool {
	if m.gateFor(op, kvVersion) != app.GateRefused {
		return false
	}
	m.refuse(what)
	return true
}

// The badge in the header and the sentence in the help panel are the shell's:
// both are standing notices about the session rather than about a write.
