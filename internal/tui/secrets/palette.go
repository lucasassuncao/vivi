package secrets

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// What the command palette needs to know and to run. Every entry here is a
// question the palette used to answer by reading this tab's fields, and every
// action is one it used to call by name. The table in the shell is unchanged;
// what changed is that it now asks rather than reaches.

// IsBrowsing reports that the panes are what the user is looking at, with
// nothing of this tab's own over them.
func (m *Model) IsBrowsing() bool { return modeIs[browsing](m) }

// OnVersionList reports that the version list is open. It stays true while the
// command line is up, because the two are separate machines now: ":" opened
// over the list leaves the list where it was.
func (m *Model) OnVersionList() bool { return modeIs[choosingVersion](m) }

// HasField reports that there is a selected field for reveal and copy to act
// on, which is the guard those two apply anyway.
func (m *Model) HasField() bool { return m.fieldCursor < len(m.sortedKeys()) }

// SelectedSecret reports a secret and not a folder under the cursor.
func (m *Model) SelectedSecret() bool {
	n := m.tree.current()
	return n != nil && n.kind == kindSecret
}

// HasSecret reports a secret loaded in the pane, which is what edit needs.
func (m *Model) HasSecret() bool { return m.secret != nil }

// CanCreateHere reports a folder or mount the new secret would land in.
func (m *Model) CanCreateHere() bool { return m.createParent() != nil }

// HasVersions reports a history to open.
func (m *Model) HasVersions() bool { return len(m.versions) > 0 }

// HasSelection reports a secret the whole-history delete would address.
func (m *Model) HasSelection() bool { return m.sel != nil }

// ReadableVersion reports that the marked version still has data behind it.
func (m *Model) ReadableVersion() bool {
	v := m.currentVersionInfo()
	return v != nil && v.Readable()
}

// CurrentVersion is the version under the cursor in the list, or nil.
func (m *Model) CurrentVersion() *vault.VersionInfo { return m.currentVersionInfo() }

// The actions. Each adopts the context and drains, the same as every other way
// in, because a command and its keystroke run the same code.

func (m *Model) RevealCurrentField(ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	m.revealCurrentField()
	return m.drain(nil)
}

func (m *Model) CopyCurrentValue(ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	return m.drain(m.copyCurrentValue())
}

func (m *Model) BeginEdit(ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	m.beginEdit()
	return m.drain(nil)
}

func (m *Model) BeginCreate(ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	m.beginCreate()
	return m.drain(nil)
}

func (m *Model) OpenVersions(ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	m.openVersions()
	return m.drain(nil)
}

func (m *Model) ConfirmDeleteCurrent(ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	m.confirmDeleteCurrent()
	return m.drain(nil)
}

func (m *Model) ConfirmDeleteField(ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	m.confirmDeleteField()
	return m.drain(nil)
}

func (m *Model) ReadSelectedVersion(ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	return m.drain(m.readSelectedVersion())
}

func (m *Model) StartDiff(ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	return m.drain(m.startDiff())
}

func (m *Model) ConfirmRollback(ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	m.confirmRollback()
	return m.drain(nil)
}

func (m *Model) ConfirmUndelete(ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	return m.drain(m.confirmUndelete())
}

func (m *Model) ConfirmDestroy(ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	m.confirmDestroy()
	return m.drain(nil)
}

func (m *Model) ConfirmDeleteMetadata(ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	m.confirmDeleteMetadata()
	return m.drain(nil)
}

// Filter applies a pattern typed as ":filter", which is the same thing "/"
// leaves behind when it is accepted.
func (m *Model) Filter(pattern string, ctx ui.Context) tea.Cmd {
	m.adopt(ctx)
	m.tree.filter = pattern
	m.filterInput.SetValue(pattern)
	m.tree.reflow()
	return m.drain(m.onCursorMoved())
}

// ReadVersion reads a version named by number, which is what ":version 3" is
// for: the list is the discoverable way and this is the direct one.
func (m *Model) ReadVersion(arg string, ctx ui.Context) tea.Cmd {
	m.adopt(ctx)

	n, err := strconv.Atoi(strings.TrimPrefix(arg, "v"))
	if err != nil {
		m.notify("not a version number: " + arg)
		return m.drain(nil)
	}
	for i, v := range m.versions {
		if v.Version != n {
			continue
		}
		if !v.Readable() {
			m.notify(fmt.Sprintf("v%d has no data to show", n))
			return m.drain(nil)
		}
		m.versionCursor = i
		m.mode = browsing{}
		return m.drain(m.selectSecret(m.sel, n))
	}
	m.notify(fmt.Sprintf("no v%d on this secret", n))
	return m.drain(nil)
}
