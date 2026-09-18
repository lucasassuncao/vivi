package secrets

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// The version list, and the four operations that only exist there. It opens
// over the Secrets tab, which is why the keys have their own handler: inside
// this list "d" is diff and not delete, and that reversal needs one place.

// openVersions shows the history of the selected secret.
func (m *Model) openVersions() {
	if len(m.versions) == 0 {
		return
	}
	m.mode = choosingVersion{}
	m.versionCursor = 0
}

// keyVersions routes keys while the version list has focus.
func (m *Model) keyVersions(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "v", "V":
		m.mode = browsing{}
		return nil
	case ":":
		return ui.Emit(ui.OpenCmdlineMsg{})
	case "down":
		m.versionCursor = clampIndex(m.versionCursor+1, max(0, len(m.versions)-1))
	case "up":
		m.versionCursor = clampIndex(m.versionCursor-1, max(0, len(m.versions)-1))
	case "home":
		m.versionCursor = 0
	case "end":
		m.versionCursor = max(0, len(m.versions)-1)
	case "space":
		// "space" and not " ": v2 names the key rather than the character it
		// types, which is what the legend and the help panel have always called
		// it. In v1 this read " " and the rename broke marking silently.
		if v := m.currentVersionInfo(); v != nil {
			m.marked[v.Version] = !m.marked[v.Version]
		}
	case "enter":
		return m.readSelectedVersion()
	case "d":
		return m.startDiff()
	case "u":
		return m.confirmUndelete()
	case "D":
		m.confirmDestroy()
	case "b":
		m.confirmRollback()
	case "M":
		m.confirmDeleteMetadata()
	}
	return nil
}

func (m *Model) readSelectedVersion() tea.Cmd {
	v := m.currentVersionInfo()
	if v == nil || !v.Readable() {
		return nil
	}
	m.mode = browsing{}
	return m.selectSecret(m.sel, v.Version)
}

func (m *Model) currentVersionInfo() *vault.VersionInfo {
	if m.versionCursor < 0 || m.versionCursor >= len(m.versions) {
		return nil
	}
	return &m.versions[m.versionCursor]
}

func (m *Model) currentVersionNumber() int {
	if m.meta != nil {
		return m.meta.CurrentVersion
	}
	return 0
}

// targetVersions asks app which versions the next operation applies to.
func (m *Model) targetVersions() []int {
	return app.TargetVersions(m.marked, m.currentVersionInfo())
}

// Four operations, none a variant of another. D and M are the confusable pair
// and both have no undo: LIST reads the metadata, so a destroyed version stays
// listed and readable as a name, and only M takes the path out.
func (m *Model) confirmUndelete() tea.Cmd {
	n, target := m.sel, m.targetVersions()
	if n == nil || len(target) == 0 {
		return nil
	}
	if m.blocks(app.OpUndelete, n.kvVersion, "undelete") {
		return nil
	}
	// Undelete restores; it destroys nothing, so it does not ask.
	return m.undeleteVersions(n, target)
}

func (m *Model) confirmDestroy() {
	n, target := m.sel, m.targetVersions()
	if n == nil || len(target) == 0 {
		return
	}
	if m.blocks(app.OpDestroy, n.kvVersion, "destroy") {
		return
	}
	m.mode = confirming{confirmation: newDangerConfirm(
		"Destroy versions",
		[]string{
			n.fullPath(),
			"",
			"Versions: " + app.DescribeVersions(target, m.currentVersionNumber()),
			"",
			m.st.Danger.Render("This erases the data permanently."),
			"Vault has no undo for destroy: undelete will not bring it back.",
			"",
			"The secret stays in the listing with an empty version.",
			"Delete all (M) is what removes the path itself.",
			"",
			"Type " + m.st.Danger.Render(app.LastSegment(n.path)) + " to confirm:",
		},
		app.LastSegment(n.path),
		confirmedDestroyMsg{node: n, versions: target})}
}

func (m *Model) confirmDeleteMetadata() {
	n := m.sel
	if n == nil {
		return
	}
	if m.blocks(app.OpDeleteMetadata, n.kvVersion, "delete metadata") {
		return
	}
	m.mode = confirming{confirmation: newDangerConfirm(
		"Delete the secret and its whole history",
		[]string{
			n.fullPath(),
			"",
			fmt.Sprintf("Destroys %d version(s) and the secret's metadata.", len(m.versions)),
			m.st.Danger.Render("Irreversible: none of this can be recovered."),
			"",
			"The path disappears from the listing. Destroy (D) leaves it.",
			"",
			"Type " + m.st.Danger.Render(app.LastSegment(n.path)) + " to confirm:",
		},
		app.LastSegment(n.path),
		confirmedDeleteMetadataMsg{node: n})}
}

// confirmRollback writes the old content as a new version. Nothing is lost,
// which is why this is a y/n and not a typed confirmation.
func (m *Model) confirmRollback() {
	n, v := m.sel, m.currentVersionInfo()
	if n == nil || v == nil {
		return
	}
	if m.blocks(app.OpRollback, n.kvVersion, "rollback") {
		return
	}
	if !v.Readable() {
		m.notify("that version has no data to restore")
		return
	}

	base := m.currentVersionNumber()
	m.mode = confirming{confirmation: newConfirm(
		"Rollback",
		[]string{
			n.fullPath(),
			"",
			fmt.Sprintf("Writes the contents of v%d as a new version (v%d).", v.Version, base+1),
			"No version is deleted.",
		},
		confirmedRollbackMsg{node: n, toVersion: v.Version, base: base})}
}
