// Package secrets is the Secrets tab: the tree on the left, the detail pane on
// the right, and everything that opens over them - the editor, the version
// list, the diff and the confirmations. It is the one tab that writes, which is
// why the read-only gate lives here rather than in the parent.
package secrets

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// The Secrets tab: the detail pane, and the keys that drive it. The left pane
// lives in tree.go with the model it draws, and the editor that takes this pane
// over while a secret is written lives in edit.go for the same reason.

// fieldsStartLine is where the first data field lands in the pane's content:
// version label, blank line, "data" rule. A constant because View may not write
// to the model, and pinned by TestFieldsStartWhereTheCursorThinksTheyDo.
const fieldsStartLine = 3

// renderSecretDetail is the right pane on the Secrets tab. It is never blank:
// a directory or a mount describes itself when no secret is selected.
func (m *Model) renderSecretDetail(width int) string {
	n := m.shownNode()
	if n == nil {
		return m.st.Dim.Render("-")
	}

	if e := m.currentEditor(); e != nil {
		return m.renderEditor(e, width)
	}

	switch n.kind {
	case kindMount:
		return m.renderMountDetail(n, width)
	case kindDir:
		return m.renderDirDetail(n, width)
	}

	// The path is the panel's title now, so the body opens on the version line.
	var b strings.Builder

	if m.detailErr != nil {
		b.WriteString(m.st.ErrBanner.Render(humanize(m.detailErr)))
		if errors.Is(m.detailErr, vault.ErrNotFound) && len(m.versions) > 0 {
			b.WriteString("\n")
			b.WriteString(m.st.Dim.Render("this version was deleted or destroyed"))
		}
		b.WriteString("\n\n")
		b.WriteString(m.renderVersions(width))
		return b.String()
	}

	if m.secret == nil {
		return b.String() + m.st.Dim.Render("loading…")
	}

	b.WriteString(m.st.Dim.Render(m.versionLabel()))
	b.WriteString("\n\n")
	b.WriteString(m.st.HeadingBadged("data", m.maskBadge(), width))
	b.WriteString("\n")

	keys := m.sortedKeys()
	if len(keys) == 0 {
		b.WriteString(m.st.Dim.Render("  (no keys)\n"))
	}
	for i, k := range keys {
		selected := i == m.fieldCursor

		cursor := "  "
		if selected {
			cursor = m.st.Cursor.Render("› ")
		}

		// The glyph marks the exception, a value on screen: masking is already
		// said by the badge and the dots, and a marker on every row marks
		// nothing. The columns are still spent, so the keys stay aligned.
		var glyph, value string
		if m.revealed[k] {
			glyph = m.st.Warn.Render("◉ ")
			// Sanitized here and not in valueToString, which also feeds the
			// editor and the clipboard: those two must see the real bytes.
			value = ui.Sanitize(valueToString(m.secret.Data[k]))
		} else {
			glyph = "  "
			value = m.st.Masked.Render(ui.MaskedValue)
		}

		line := cursor + glyph + m.st.Key.Render(fmt.Sprintf("%-16s ", ui.Sanitize(k))) + value
		b.WriteString(ui.Truncate(line, width))
		b.WriteString("\n")
	}

	b.WriteString(m.renderVersions(width))
	b.WriteString(m.renderMetadata(width))
	b.WriteString(m.renderAccess(n, width))
	return b.String()
}

// maskBadge states, in one glance, whether anything on this screen is currently
// legible. It matters most when someone is about to share their terminal.
func (m *Model) maskBadge() string {
	shown := 0
	for _, k := range m.sortedKeys() {
		if m.revealed[k] {
			shown++
		}
	}
	if shown == 0 {
		return m.st.Badge.Render("(MASKED)")
	}
	return m.st.BadgeWarn.Render(fmt.Sprintf("(REVEALED: %d)", shown))
}

func (m *Model) versionLabel() string {
	if m.secret == nil || m.secret.Version == 0 {
		return "no versioning (KV v1)"
	}
	// "of N" needs an N. A refused or failed history leaves versions empty while
	// the secret read fine, and "v1 of 0" states a contradiction: the version on
	// screen is one of them.
	label := fmt.Sprintf("v%d", m.secret.Version)
	if total := len(m.versions); total > 0 {
		label = fmt.Sprintf("v%d of %d", m.secret.Version, total)
	}
	if m.meta != nil && m.secret.Version != m.meta.CurrentVersion {
		label += "  -  historical (read-only)"
	}
	if !m.secret.CreatedTime.IsZero() {
		label += "   created " + m.secret.CreatedTime.Format("2006-01-02 15:04")
	}
	return label
}

func (m *Model) renderVersions(width int) string {
	if len(m.versions) == 0 {
		return ""
	}

	// Sections are separated by a blank line: three rules stacked against their
	// rows read as one wall of text, and the version list is where the eye has
	// to land quickly.
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(m.st.Heading("versions", width))
	b.WriteString("\n")

	for i, v := range m.versions {
		cursor := "  "
		if modeIs[choosingVersion](m) && i == m.versionCursor {
			cursor = m.st.Cursor.Render("› ")
		}
		mark := " "
		if m.marked[v.Version] {
			mark = "×"
		}

		state := ""
		switch {
		case v.Destroyed:
			state = m.st.Danger.Render("✗ destroyed")
		case v.Deleted():
			state = m.st.Warn.Render("⌫ deleted")
		case m.meta != nil && v.Version == m.meta.CurrentVersion:
			// Every version action defaults to this one, so it gets a badge
			// rather than a word in a different colour.
			state = m.st.Badge.Render(" ✓ current ")
		}

		line := fmt.Sprintf("%s[%s] v%-3d %s  %s",
			cursor, mark, v.Version, v.CreatedTime.Format("2006-01-02 15:04"), state)
		b.WriteString(ui.Truncate(line, width))
		b.WriteString("\n")
	}
	return b.String()
}

func (m *Model) renderMetadata(width int) string {
	if m.meta == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")
	// "metadata", which is what this is: one GetMetadata answers it, and every
	// row is a property of the secret rather than of the version on screen.
	b.WriteString(m.st.Heading("metadata", width))
	b.WriteString("\n")
	// Not the API's field names and raw values, which is the wrong way round for
	// a panel describing the secret's state. "max_versions 0" is the worst: a
	// zero there means unlimited, and it reads as none.
	rows := [][2]string{
		{"current version", fmt.Sprint(m.meta.CurrentVersion)},
		{"oldest kept", oldestKept(m.meta.OldestVersion)},
		{"keeps", versionLimit(m.meta.MaxVersions)},
		{"check-and-set", requiredOrNot(m.meta.CASRequired)},
		{"updated", m.meta.UpdatedTime.Format("2006-01-02 15:04")},
	}
	for _, r := range rows {
		b.WriteString(ui.Truncate("  "+m.st.Key.Render(fmt.Sprintf("%-16s ", r[0]))+r[1], width))
		b.WriteString("\n")
	}
	return b.String()
}

// oldestKept says what version 0 means: nothing has aged out yet, so the whole
// history is still there.
func oldestKept(v int) string {
	if v == 0 {
		return "v1 (nothing pruned yet)"
	}
	return fmt.Sprintf("v%d", v)
}

// versionLimit spells out Vault's zero, which means unlimited and reads as none.
func versionLimit(n int) string {
	if n == 0 {
		return "every version"
	}
	return fmt.Sprintf("the last %d versions", n)
}

// requiredOrNot answers the question the field is actually asking, instead of
// printing the boolean behind it.
func requiredOrNot(b bool) string {
	if b {
		return "required on every write"
	}
	return "not required"
}

func (m *Model) renderMountDetail(n *node, width int) string {
	// The path is in the panel title; the body carries what the title cannot.
	var b strings.Builder
	b.WriteString(m.st.KV("type", n.mountType, width))
	b.WriteString(m.st.KV("engine", kvLabel(n.kvVersion), width))
	if n.desc != "" {
		b.WriteString(m.st.KV("description", n.desc, width))
	}
	// What the listing found, told apart the way the directory panel tells it
	// apart: "empty", "not loaded" and "denied" all rendered as "0 children"
	// before, and the three mean very different things.
	if !n.isKV() {
		return b.String() + "\n" + m.st.Dim.Render("vivi browses KV mounts only")
	}
	return b.String() + m.renderListing(n, "mount", width)
}

func (m *Model) renderDirDetail(n *node, width int) string {
	var b strings.Builder
	b.WriteString(m.st.KV("mount", n.mount+"/", width))
	b.WriteString(m.st.KV("engine", kvLabel(n.kvVersion), width))
	return b.String() + m.renderListing(n, "path", width)
}

// renderListing says what listing a node found, or why there is nothing to
// say: "empty", "not loaded" and "denied" are three different answers.
func (m *Model) renderListing(n *node, what string, width int) string {
	switch {
	case n.denied:
		return "\n" + m.st.Denied.Render("the token cannot list this "+what)
	case !n.loaded:
		return "\n" + m.st.Dim.Render("not loaded - open it with →")
	case len(n.children) == 0:
		return "\n" + m.st.Dim.Render("this "+what+" is empty")
	default:
		return m.st.KV("children", fmt.Sprint(len(n.children)), width)
	}
}

func kvLabel(v int) string {
	switch v {
	case vault.KV2:
		return "KV v2 (versioned)"
	case vault.KV1:
		return "KV v1 (no history)"
	default:
		return "non-KV"
	}
}

// renderAccess answers the two questions an auditing browser is opened to ask:
// what may I do here, and which policy is responsible. Different in kind - the
// capabilities line is the server's verdict, the grants a partial explanation.
func (m *Model) renderAccess(n *node, width int) string {
	if n == nil || n.kind != kindSecret {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(m.st.Heading("access", width))
	b.WriteString("\n")

	rows := [][2]string{{"you may", m.capabilityLabel()}}

	// The policy path is the reason this section pays for itself. On a v2 mount
	// it is not the path in the panel title, and a policy naming the title
	// grants nothing at all - silently.
	rows = append(rows, [2]string{"policy path", app.PolicyPath(n.mount, n.path, n.kvVersion)})
	if p := app.MetadataPolicyPath(n.mount, n.path, n.kvVersion); p != "" {
		rows = append(rows, [2]string{"metadata path", p})
	}
	for _, r := range rows {
		b.WriteString(ui.Truncate("  "+m.st.Key.Render(fmt.Sprintf("%-16s ", r[0]))+ui.Sanitize(r[1]), width))
		b.WriteString("\n")
	}

	b.WriteString(m.renderGrants(n, width))
	return b.String()
}

// capabilityLabel is the server's verdict, or why there isn't one.
func (m *Model) capabilityLabel() string {
	switch {
	case m.capsErr != nil:
		return m.st.Dim.Render("unknown (" + humanize(m.capsErr) + ")")
	case len(m.capabilities) == 0:
		return m.st.Dim.Render("…")
	case len(m.capabilities) == 1 && m.capabilities[0] == "deny":
		// Vault's own word for "nothing", and it reads as a capability rather
		// than as the absence of every capability.
		return m.st.Danger.Render("nothing on this path")
	default:
		return strings.Join(m.capabilities, ", ")
	}
}

// isRootToken is the server's own verdict: sys/capabilities-self answers a root
// token with the single word "root" instead of the crud list.
func (m *Model) isRootToken() bool {
	return len(m.capabilities) == 1 && m.capabilities[0] == "root"
}

// renderGrants lists the policies that reach this path, and the ones that could
// not be read at all. The second list is not decoration: an explanation with a
// hole in it that does not admit to the hole reads as a complete one.
func (m *Model) renderGrants(n *node, width int) string {
	if len(m.policyDocs) == 0 {
		return ""
	}

	grants, unreadable := app.GrantsFor(m.policyDocs, app.PolicyPath(n.mount, n.path, n.kvVersion))

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(ui.Truncate("  "+m.st.Key.Render("granted by"), width))
	b.WriteString("\n")

	// "root" is a policy with no document, so no rule will ever name the path.
	// Saying "none grants it" beside a "you may root" line reads as a denial.
	if m.isRootToken() {
		b.WriteString(m.st.Dim.Render("    (root bypasses policy: no rule needs to name it)\n"))
		return b.String()
	}
	if len(grants) == 0 {
		b.WriteString(m.st.Dim.Render("    (none of this token's readable policies names it)\n"))
	}

	for _, g := range grants {
		// A glob is worth marking. "kv/data/app/*" reaching this secret is a
		// different fact from a rule naming it, and it is the one that also
		// reaches everything beside it.
		kind := m.st.Warn.Render("glob")
		if g.Exact {
			kind = m.st.Dim.Render("exact")
		}
		line := fmt.Sprintf("    %-18s %-28s %s  %s",
			ui.Sanitize(g.Policy), ui.Sanitize(g.Rule.Path), kind,
			m.st.Dim.Render(strings.Join(g.Rule.Capabilities, ", ")))
		b.WriteString(ui.Truncate(line, width))
		b.WriteString("\n")
	}

	if len(unreadable) > 0 {
		b.WriteString(ui.Truncate(m.st.Warn.Render(fmt.Sprintf(
			"    (%d could not be parsed: %s)",
			len(unreadable), ui.Sanitize(strings.Join(unreadable, ", ")))), width))
		b.WriteString("\n")
	}

	return b.String()
}

// Keys.

func (m *Model) keySecrets(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "down":
		m.tree.move(1)
		return m.onCursorMoved()
	case "up":
		m.tree.move(-1)
		return m.onCursorMoved()
	case "home":
		m.tree.moveTo(0)
		return m.onCursorMoved()
	case "end":
		m.tree.moveTo(len(m.tree.visible) - 1)
		return m.onCursorMoved()
	case "right", "enter", " ":
		// Both mean "further in": expanding a folder, or moving into the pane
		// already showing a secret's contents. Left mirrors both. Stopping dead
		// on a secret left the obvious key doing nothing where it mattered most.
		if n := m.tree.current(); n != nil && n.kind == kindSecret {
			m.focusDetailPane()
			return nil
		}
		return m.expandCurrent()
	case "left":
		m.tree.collapse()
		return m.onCursorMoved()
	case "/":
		m.mode = filtering{}
		m.filterInput.SetValue(m.tree.filter)
		m.filterInput.Focus()
		return textinput.Blink
	case "r":
		m.revealCurrentField()
		return nil
	case "y":
		// Y and C used to copy the path and the vault command. The copy panel
		// prints both now, so the keys were spending two letters on something
		// the screen answers. Each line the panel shows has a command of its
		// own now, generated from it: :copy-path and the rest.
		return m.copyCurrentValue()
	case "v", "V":
		// Lowercase is the binding; uppercase is an unadvertised alias. Every
		// other focus-only key is lowercase, and V sat among R, D and M as if it
		// were irreversible, when opening a list destroys nothing.
		m.openVersions()
		return nil
	case "e":
		m.beginEdit()
		return nil
	case "a":
		m.beginCreate()
		return nil
	case "d":
		// The pane's cursor is on a field, so that is what "d" takes there. The
		// whole secret goes from the list, where the cursor is on the secret.
		if m.focus == ui.FocusDetail {
			m.confirmDeleteField()
		} else {
			m.confirmDeleteCurrent()
		}
		return nil
	}
	return nil
}

func (m *Model) keyFilter(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.tree.filter = ""
		m.filterInput.SetValue("")
		m.filterInput.Blur()
		m.mode = browsing{}
		m.tree.reflow()
		return m.onCursorMoved()
	case "enter":
		m.filterInput.Blur()
		m.mode = browsing{}
		return m.onCursorMoved()
	}

	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.tree.filter = m.filterInput.Value()
	m.tree.reflow()
	return cmd
}

// Selection.

// secretLoad gathers every answer the detail pane needs about one node so the
// pane can change to it in a single frame. Rendered as they landed, the three
// requests cost a blank frame and three repaints per cursor move.
type secretLoad struct {
	node    *node
	version int // the version asked for; 0 means whichever is current

	secret      *vault.Secret
	readVersion int
	err         error
	gotRead     bool

	versions    []vault.VersionInfo
	meta        *vault.Metadata
	wantHistory bool
	gotHistory  bool

	caps    []string
	capsErr error
	gotCaps bool
}

// settled reports that every answer this load is waiting for has arrived. An
// error counts as an answer: the pane can draw a failure, and waiting past one
// would hold the last secret on screen until the timeout.
func (l *secretLoad) settled() bool {
	return l.gotRead && l.gotCaps && (!l.wantHistory || l.gotHistory)
}

// onCursorMoved loads whatever the newly selected row needs.
func (m *Model) onCursorMoved() tea.Cmd {
	n := m.tree.current()
	if n == nil || n.kind != kindSecret {
		// A mount or a folder is drawn from the node itself, so there is
		// nothing to wait for and nothing to hold: the pane changes now.
		m.loading = nil
		m.sel, m.secret, m.versions, m.meta, m.detailErr = n, nil, nil, nil, nil
		m.scrollTop()
		return nil
	}
	if n == m.sel && m.secret != nil {
		// Back on the row the pane is already showing - which happens when the
		// cursor leaves and returns inside one load - so whatever was in flight
		// is no longer wanted.
		m.loading = nil
		return nil
	}
	return m.selectSecret(n, 0)
}

// selectSecret reads a secret and, on a versioned mount, its history. Nothing
// on screen changes here: the pane goes on showing the last secret it has until
// commitSecret swaps this one in whole.
func (m *Model) selectSecret(n *node, version int) tea.Cmd {
	if n == nil || n.kind != kindSecret {
		return nil
	}

	load := &secretLoad{node: n, version: version, wantHistory: n.kvVersion == vault.KV2}
	load.gotHistory = !load.wantHistory
	m.loading = load

	cmds := []tea.Cmd{m.loadSecret(n, version), m.loadCapabilities(n), m.loadTokenPolicies()}
	if load.wantHistory {
		cmds = append(cmds, m.loadVersions(n))
	}
	return tea.Batch(cmds...)
}

// settleSecret puts a finished load on screen. Called wherever an answer lands,
// so the swap happens on whichever of the three arrives last.
func (m *Model) settleSecret() {
	if m.loading != nil && m.loading.settled() {
		m.commitSecret()
	}
}

// commitSecret moves a load onto the screen, all of it in one assignment. The
// only place the pane's state changes, which is what makes it atomic: no frame
// has one secret's fields beside another's version list.
func (m *Model) commitSecret() {
	l := m.loading
	if l == nil {
		return
	}

	m.sel = l.node
	m.secret, m.detailErr, m.secretVersion = l.secret, l.err, l.readVersion
	m.versions, m.meta = l.versions, l.meta
	m.capabilities, m.capsErr, m.capsNode = l.caps, l.capsErr, l.node

	m.fieldCursor = 0
	m.revealed = map[string]bool{}
	m.marked = map[int]bool{}
	m.scrollTop()

	m.loading = nil
}

// shownNode is the node the detail pane is drawing: the one under the cursor,
// except while a secret is being read. The title and the command panel both ask
// here, so the pane never names one secret over another's fields.
func (m *Model) shownNode() *node {
	if m.loading != nil && m.sel != nil {
		return m.sel
	}
	return m.tree.current()
}

// expandCurrent opens the selected node, listing it on first open.
func (m *Model) expandCurrent() tea.Cmd {
	n := m.tree.current()
	if n == nil {
		return nil
	}
	if n.kind == kindSecret {
		return m.selectSecret(n, 0)
	}
	return m.expand(n)
}

func (m *Model) expand(n *node) tea.Cmd {
	if n == nil {
		return nil
	}
	if !n.expandable() {
		if !n.isKV() {
			m.notify(n.mountType + " is not a static secrets mount")
		}
		return nil
	}
	if n.expanded {
		n.expanded = false
		m.tree.reflow()
		return nil
	}
	return m.openNode(n)
}

// The fields inside the selected secret.

// sortedKeys gives the detail panel and the field cursor one stable order.
func (m *Model) sortedKeys() []string {
	if m.secret == nil {
		return nil
	}
	keys := make([]string, 0, len(m.secret.Data))
	for k := range m.secret.Data {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func (m *Model) moveField(delta int) {
	keys := m.sortedKeys()
	if len(keys) == 0 {
		return
	}
	m.fieldCursor = clampIndex(m.fieldCursor+delta, len(keys)-1)
	m.followFieldCursor()
}

// revealCurrentField unmasks one value. The reveal is scoped to this field and
// this secret: selecting another secret clears the map, so nothing stays
// exposed on screen after the user has moved on.
func (m *Model) revealCurrentField() {
	keys := m.sortedKeys()
	if len(keys) == 0 {
		return
	}
	// The focus decides the scope, the way it does for "y": in the list the
	// cursor is on a secret, in the pane it is on a field.
	if m.focus != ui.FocusDetail {
		toggleAll(m.revealed, keys)
		return
	}
	if m.fieldCursor >= len(keys) {
		return
	}
	// Hidden is an absent key, the same shape toggleAll leaves behind, so
	// anything that iterates the map sees one form of "not revealed".
	key := keys[m.fieldCursor]
	if m.revealed[key] {
		delete(m.revealed, key)
		return
	}
	m.revealed[key] = true
}

// allFieldsRevealed is the condition toggleAll hides on, and so the condition
// the legend says "hide" on. One function short of two, or the bar offers to
// hide a secret that the next press would finish revealing.
func (m *Model) allFieldsRevealed() bool {
	keys := m.sortedKeys()
	if len(keys) == 0 {
		return false
	}
	for _, k := range keys {
		if !m.revealed[k] {
			return false
		}
	}
	return true
}

// currentFieldRevealed reports on the one field the pane's cursor is on.
func (m *Model) currentFieldRevealed() bool {
	keys := m.sortedKeys()
	if m.fieldCursor >= len(keys) {
		return false
	}
	return m.revealed[keys[m.fieldCursor]]
}

// toggleAll reveals every key, or hides every key once they all already are.
// Anything still masked means the press was asking to see it, not to put it
// back - a half-revealed secret should take one keystroke to finish, not two.
func toggleAll(set map[string]bool, keys []string) {
	for _, k := range keys {
		if !set[k] {
			for _, k := range keys {
				set[k] = true
			}
			return
		}
	}
	for _, k := range keys {
		delete(set, k)
	}
}

// confirmDeleteCurrent picks the right gate for what "d" means on this node.
// confirmDeleteField removes one key from the secret. It is a write and not a
// delete: the secret stays and a new version is created without the field, so
// the friction and the wording are the save's, not the delete's.
func (m *Model) confirmDeleteField() {
	n, keys := m.sel, m.sortedKeys()
	if n == nil || m.secret == nil || m.fieldCursor < 0 || m.fieldCursor >= len(keys) {
		return
	}
	if m.blocks(app.OpSave, n.kvVersion, "remove a field") {
		return
	}
	// The same base and the same refusal as the editor, because this is the
	// editor's write without the editor: a historical version is read-only.
	base := 0
	if m.meta != nil {
		base = m.meta.CurrentVersion
	}
	if !app.CanEdit(m.secretVersion, base) {
		m.notify("historical version: return to the current one to remove a field")
		return
	}
	key := keys[m.fieldCursor]

	data := make(map[string]any, len(keys)-1)
	for _, k := range keys {
		if k != key {
			data[k] = m.secret.Data[k]
		}
	}

	lines := []string{n.fullPath(), "", m.st.Danger.Render("- " + ui.Sanitize(key)), ""}
	if app.Warns(app.OpSave, n.kvVersion) {
		lines = append(lines,
			m.st.Warn.Render("KV v1 mount: this overwrites the current value."),
			m.st.Warn.Render("There is no history, and no way to recover the field."))
	} else {
		lines = append(lines,
			fmt.Sprintf("Writes a new version based on v%d, without this field.", base),
			"The field stays readable in the versions before it.")
	}

	m.mode = confirming{confirmation: newConfirm("Remove field", lines,
		confirmedSaveMsg{node: n, data: data, cas: app.CAS(false, n.kvVersion, base)})}
}

func (m *Model) confirmDeleteCurrent() {
	n := m.tree.current()
	if n == nil || n.kind != kindSecret {
		return
	}
	if m.blocks(app.OpDelete, n.kvVersion, "delete") {
		return
	}

	// "Delete" is one word for two acts, and the mount decides which: on v2 a
	// soft delete undelete reverses, on v1 the only copy there is. app grades
	// it, so the two modals below cannot drift from the policy.
	if m.gateFor(app.OpDelete, n.kvVersion) == app.GateTyped {
		m.mode = confirming{confirmation: newDangerConfirm(
			"Delete secret (KV v1)",
			[]string{
				n.fullPath(),
				"",
				m.st.Danger.Render("v1 mounts have no soft-delete and no history."),
				"The secret is gone, and Vault cannot undo it.",
				"",
				"Type " + m.st.Danger.Render(app.LastSegment(n.path)) + " to confirm:",
			},
			app.LastSegment(n.path),
			confirmedDeleteMsg{node: n})}
		return
	}

	target, described := m.browseDeleteTarget(n)
	m.mode = confirming{confirmation: newConfirm(
		"Delete version",
		[]string{
			n.fullPath(),
			"",
			"Versions: " + described,
			"",
			"Soft-delete: the data disappears from reads, and undelete (u) brings it back.",
		},
		confirmedDeleteMsg{node: n, versions: target})}
}

// browseDeleteTarget is the version "d" deletes outside the version list, and
// how to name it. The marks and the list cursor belong to the secret the pane
// is showing and outlive the list being closed, so neither decides this: what
// is on screen for this node does, and nil means whatever Vault has current.
func (m *Model) browseDeleteTarget(n *node) (target []int, described string) {
	if n != m.sel {
		return nil, "the current version"
	}
	if m.secretVersion != 0 {
		return []int{m.secretVersion}, app.DescribeVersions([]int{m.secretVersion}, m.currentVersionNumber())
	}
	// The pane is on the current version. Metadata names it when the token can
	// read it, and when it cannot the modal says which version it means anyway.
	if cur := m.currentVersionNumber(); cur > 0 {
		return nil, app.DescribeVersions([]int{cur}, cur)
	}
	return nil, "the current version"
}
