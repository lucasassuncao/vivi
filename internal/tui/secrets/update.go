package secrets

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// What the model does with an answer: every request is a tea.Cmd that comes
// back as a message, and this is where the server's answers and the user's
// confirmations are folded in. The requests themselves are in cmds.go.

// handleData folds a completed request into the model.
func (m *Model) handleData(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case mountsMsg:
		if msg.err != nil {
			return m.fail("load mounts", msg.err)
		}
		m.tree.setMounts(msg.mounts)
		// A ":goto" typed before the mounts arrived starts here.
		return m.advanceGoto()

	case childrenMsg:
		// Every one of these answers a listing that named its node, so a nil
		// means request and answer came apart: nothing to fold in and nothing to
		// say. The alternative is a dereference that ends the session.
		if msg.node == nil {
			return nil
		}
		msg.node.loading = false
		if msg.err != nil {
			if errors.Is(msg.err, vault.ErrForbidden) {
				m.tree.markDenied(msg.node)
				return m.advanceGoto()
			}
			// A missing path is the one error the tree draws as an empty
			// result rather than as a failure.
			if errors.Is(msg.err, vault.ErrNotFound) {
				m.tree.setChildren(msg.node, nil)
				return m.advanceGoto()
			}
			m.pendingGoto = ""
			return m.fail("list "+msg.node.fullPath(), msg.err)
		}
		m.tree.setChildren(msg.node, msg.entries)
		// This listing may be the level a ":goto" was waiting on. advanceGoto
		// is a no-op when no walk is in flight.
		return m.advanceGoto()

	case secretMsg:
		if m.staleAnswer(msg.node) {
			return nil
		}
		m.loading.secret, m.loading.err = msg.secret, msg.err
		m.loading.readVersion, m.loading.gotRead = msg.version, true
		m.settleSecret()
		return nil

	case capabilitiesMsg:
		if m.staleAnswer(msg.node) {
			return nil
		}
		m.loading.caps, m.loading.capsErr, m.loading.gotCaps = msg.caps, msg.err, true
		m.settleSecret()
		return nil

	case policyDocMsg:
		// A policy the token names but cannot read is normal and is not a
		// failure worth a banner: the reverse lookup simply cannot account for
		// that one, and says so where it lists what it found.
		if msg.err == nil {
			m.policyDocs[msg.name] = msg.hcl
		}
		return nil

	case versionsMsg:
		if m.staleAnswer(msg.node) {
			return nil
		}
		m.loading.gotHistory = true
		if msg.err == nil {
			// A v1 mount has no history; that is expected, not a failure, and
			// the zero values already say it.
			m.loading.versions, m.loading.meta = msg.versions, msg.meta
		}
		m.settleSecret()
		return nil

	case writeMsg:
		return m.afterWrite(msg)

	case opMsg:
		m.record(msg.op, msg.path, "", msg.err)
		if msg.err != nil {
			return m.fail(msg.op+" "+msg.path, msg.err)
		}
		m.notify(fmt.Sprintf("%s: %s", msg.op, msg.path))
		m.marked = map[int]bool{}
		return m.refreshSelection(msg.node, msg.op)

	case diffMsg:
		return m.afterDiff(msg)
	}

	return m.handleConfirmed(msg)
}

// afterDiff opens the comparison, or says why there is none to open.
func (m *Model) afterDiff(msg diffMsg) tea.Cmd {
	// A comparison with no error and no result is not a diff to show. The
	// renderer reads straight through the pointer, so entering the mode
	// empty fails not here but at the next frame, and every frame after.
	if msg.err != nil || msg.result == nil {
		m.mode = browsing{}
		if msg.err != nil {
			return m.fail("compare versions", msg.err)
		}
		return m.failWith("compare versions: the server returned nothing to compare")
	}
	// A fresh comparison starts at the top, masked. A carried-over cursor
	// would point into a different set of keys, and a carried-over reveal
	// would unmask a value nobody asked about here.
	m.diffCursor, m.diffRevealed = 0, map[string]bool{}
	m.mode = showingDiff{diff: msg.result}
	return nil
}

// handleConfirmed applies what the user agreed to in a modal. These arrive as
// messages rather than running inside the modal that asked, so a destroy is as
// traceable through the loop as a keystroke or a server answer.
func (m *Model) handleConfirmed(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case confirmedCreateMsg:
		m.createSecret(msg.parent, msg.name)
		return nil

	case confirmedDiscardMsg:
		m.mode = browsing{}
		if !msg.creating {
			return nil
		}
		// The create selected a node the tree never had. Cancelling puts the
		// pane back on the cursor, or "e" reopens a path nobody made.
		return m.onCursorMoved()

	case confirmedSaveMsg:
		return m.writeSecret(msg.node, msg.data, msg.cas, msg.editor)

	case confirmedReloadMsg:
		m.mode = browsing{}
		return m.selectSecret(msg.node, 0)

	case restoreModeMsg:
		// The way back has to lead somewhere. A restore carrying no mode would
		// leave m.mode nil, and the crash would surface at the next keystroke
		// rather than here, in a handler that has nothing to do with it.
		if msg.mode == nil {
			m.mode = browsing{}
			return nil
		}
		m.mode = msg.mode
		return nil

	case confirmedDeleteMsg:
		return m.deleteVersions(msg.node, msg.versions)

	case confirmedDestroyMsg:
		return m.destroyVersions(msg.node, msg.versions)

	case confirmedDeleteMetadataMsg:
		return m.deleteMetadata(msg.node)

	case confirmedRollbackMsg:
		return m.rollback(msg.node, msg.toVersion, msg.base)
	}

	// Not one of this tab's answers. The nested inputs get it next: the filter
	// and the editor's fields are Bubbles models with state of their own.
	return m.routeToInput(msg)
}

// afterWrite reports a finished write, giving a check-and-set rejection its own
// treatment: the edit is kept so nothing typed is lost, and the user chooses
// between reloading and continuing.
func (m *Model) afterWrite(msg writeMsg) tea.Cmd {
	path := ""
	if msg.node != nil {
		path = msg.node.fullPath()
	}
	m.record("write", path, "", msg.err)

	if msg.err != nil {
		return m.afterFailedWrite(msg, path)
	}

	m.mode = browsing{}
	if msg.secret != nil && msg.secret.Version > 0 {
		m.notify(fmt.Sprintf("saved %s as v%d", path, msg.secret.Version))
	} else {
		m.notify("saved " + path)
	}

	cmds := []tea.Cmd{m.selectSecret(msg.node, 0)}

	// A secret that was just created does not exist in its parent's listing
	// yet, so the tree would not show it until something else reloaded that
	// path. Refresh the parent so the new node appears where it belongs.
	if n := msg.node; n != nil && n.parent != nil && !n.parent.hasChild(n.name) {
		cmds = append(cmds, m.reload(n.parent))
	}

	return tea.Batch(cmds...)
}

// afterFailedWrite decides where a rejected write lands: a check-and-set
// conflict opens the reload-or-keep modal, and everything else returns to the
// editor with the edit intact.
func (m *Model) afterFailedWrite(msg writeMsg, path string) tea.Cmd {
	if errors.Is(msg.err, vault.ErrCAS) && msg.editor != nil {
		// A rejected create is not a conflict: the name was simply taken.
		// Offering to reload and discard would load somebody else's secret into
		// a half-typed new one, so this returns to the editor instead.
		if msg.editor.creating {
			m.mode = editing{editor: msg.editor}
			return m.failWith(path + " already exists - press esc and create it under another name")
		}
		m.mode = confirming{confirmation: m.casConflictModal(msg.node, msg.editor)}
		return nil
	}
	// Only a save has an editor: a rollback is a write with nothing behind it,
	// and sending that to the editing mode used to leave a mode with no editor.
	if msg.editor != nil {
		m.mode = editing{editor: msg.editor}
	} else {
		m.mode = browsing{}
	}
	return m.fail("save "+path, msg.err)
}

// refreshSelection reloads what an operation invalidated.
func (m *Model) refreshSelection(n *node, op string) tea.Cmd {
	if n == nil {
		return nil
	}
	if op == "delete-metadata" || (op == "delete" && n.kvVersion == vault.KV1) {
		// The secret is gone: reload the parent listing instead of the secret.
		if n.parent == nil {
			return nil
		}
		m.loading = nil
		m.sel, m.secret, m.versions, m.meta = nil, nil, nil, nil
		return m.reload(n.parent)
	}
	return m.selectSecret(n, 0)
}

// reload drops a node's listing and fetches it again. expand toggles, so the
// node is collapsed first; left expanded, the call folded the listing away.
func (m *Model) reload(n *node) tea.Cmd {
	m.tree.invalidate(n)
	n.expanded = false
	return m.expand(n)
}

// staleAnswer reports an answer for a load the cursor has moved past: it would
// describe the wrong secret, so the caller drops it. Same guard for all three.
func (m *Model) staleAnswer(n *node) bool {
	return m.loading == nil || n == nil || n != m.loading.node
}
