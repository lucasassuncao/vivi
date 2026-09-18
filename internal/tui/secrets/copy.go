package secrets

import (
	"encoding/json"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// What "y" copies here, and what the copy panel offers. The clipboard itself is
// the shell's - it is a field there so a test can replace it - so everything
// below ends in a request rather than a write.

func addrRow(label, text string) ui.CopyRow { return ui.CopyRow{Label: label, Text: text} }

func cmdRow(label, text string) ui.CopyRow {
	return ui.CopyRow{Label: label, Text: text, IsCommand: true}
}

// CopyRows addresses the node under the cursor. Built from the node and not the
// loaded secret, so the commands survive a read in flight or a read that failed
// - a denied path is exactly when someone wants the command.
func (m *Model) CopyRows(ctx ui.Context) []ui.CopyRow {
	m.adopt(ctx)

	n := m.shownNode()

	// While the editor is open the panel above shows m.sel, which is not always
	// the tree cursor's row: a secret being created has no row yet, so this
	// would go on addressing whichever sibling the cursor was left on.
	editing := m.currentEditor()
	if editing != nil && m.sel != nil {
		n = m.sel
	}
	if n == nil {
		return nil
	}

	rows := []ui.CopyRow{addrRow("path", n.fullPath())}

	// A secret being created does not exist yet: a read would fail and a put
	// would spell the values out, which no line here does. The path and the
	// folder it lands in are the honest answer until it is saved.
	if editing != nil && editing.creating {
		if n.parent != nil {
			rows = append(rows, cmdRow("list", listCommand(n.parent)))
		}
		return rows
	}

	if n.kind != kindSecret {
		if n.isKV() {
			rows = append(rows, cmdRow("list", listCommand(n)))
		}
		return rows
	}

	rows = append(rows, cmdRow("read", readCommand(n, 0, "")))
	// The path a policy has to name, which on a v2 mount is not the path above
	// it. This is the row people came here to copy without knowing it.
	rows = append(rows, addrRow("policy-path", app.PolicyPath(n.mount, n.path, n.kvVersion)))

	// The selected field gets the command worth having: -field prints the value
	// alone, no JSON around it, so it pipes. It does not flicker between
	// siblings because the panel addresses the node the pane is showing.
	if key := m.selectedFieldKey(); key != "" {
		rows = append(rows, cmdRow("field", readCommand(n, 0, key)))
	}

	// Only when a historical version is on screen. On the current one the
	// -version flag names what the plain read already returns, and a command
	// that pins today's version would go stale the moment someone writes.
	if m.showingHistorical() {
		rows = append(rows, cmdRow("version", readCommand(n, m.secretVersion, "")))
	}

	if n.kvVersion == vault.KV2 {
		rows = append(rows, cmdRow("metadata",
			fmt.Sprintf("vault kv metadata get -mount=%s %s", n.mount, n.path)))
	}
	if n.parent != nil {
		rows = append(rows, cmdRow("list", listCommand(n.parent)))
	}
	return rows
}

// readCommand is `vault kv get`, optionally pinned to a version and narrowed to
// one field. version 0 and an empty field mean "leave that flag off".
func readCommand(n *node, version int, field string) string {
	cmd := "vault kv get -mount=" + n.mount
	if version > 0 {
		cmd += fmt.Sprintf(" -version=%d", version)
	}
	if field != "" {
		cmd += " -field=" + field
	}
	return cmd + " " + n.path
}

// listCommand is `vault kv list`. A mount root has an empty path, and the
// trailing space that leaves behind would be copied along with the command.
func listCommand(n *node) string {
	cmd := "vault kv list -mount=" + n.mount
	if n.path != "" {
		cmd += " " + n.path
	}
	return cmd
}

// selectedFieldKey is the key the field cursor is on, or "" when there is no
// secret loaded or nothing in it.
func (m *Model) selectedFieldKey() string {
	keys := m.sortedKeys()
	if m.fieldCursor < 0 || m.fieldCursor >= len(keys) {
		return ""
	}
	return keys[m.fieldCursor]
}

// showingHistorical reports that the pane holds a version other than the
// current one, which is the only case where pinning a version in the command
// says something the plain read does not.
func (m *Model) showingHistorical() bool {
	if m.secret == nil || m.secretVersion == 0 || m.meta == nil {
		return false
	}
	return m.secretVersion != m.meta.CurrentVersion
}

// copyCurrentValue is what "y" copies, which is decided the same way the arrows
// mean: in the list the cursor is on a secret, so it copies the secret; in the
// detail pane the cursor is on a field, so it copies the field.
func (m *Model) copyCurrentValue() tea.Cmd {
	if m.secret == nil {
		return nil
	}
	if m.focus != ui.FocusDetail {
		return m.copyWholeSecret()
	}

	keys := m.sortedKeys()
	if m.fieldCursor >= len(keys) {
		return nil
	}
	key := keys[m.fieldCursor]
	return m.copy("copy", "copied: "+key, ui.ValueToString(m.secret.Data[key]))
}

// copyWholeSecret copies every field as JSON. Not "key=value" lines: a value
// may itself contain newlines - a certificate does - and the pairs would stop
// being separable. JSON is also what `vault kv put @file.json` reads back.
func (m *Model) copyWholeSecret() tea.Cmd {
	keys := m.sortedKeys()
	if len(keys) == 0 {
		return nil
	}

	fields := make(map[string]string, len(keys))
	for _, k := range keys {
		fields[k] = ui.ValueToString(m.secret.Data[k])
	}
	// Marshal sorts the keys, so the same secret always copies the same bytes.
	out, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return m.fail("copy", err)
	}

	what := fmt.Sprintf("copied: %d fields", len(keys))
	if n := m.sel; n != nil {
		what = fmt.Sprintf("copied: %s (%d fields)", n.fullPath(), len(keys))
	}
	return m.copy("copy", what, string(out))
}

// copy hands the text to the shell, which owns the clipboard.
func (m *Model) copy(op, what, text string) tea.Cmd {
	return ui.Emit(ui.CopyMsg{Op: op, What: what, Text: text})
}
