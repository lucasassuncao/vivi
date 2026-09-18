package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// field is one key-value pair while editing. orig keeps the value exactly as
// Vault returned it, so an untouched number or object round-trips unchanged
// instead of being flattened into text by the act of opening the editor.
type field struct {
	key     string
	value   string
	orig    any
	changed bool
	added   bool
}

// editor is the in-panel field editor.
type editor struct {
	fields []field
	cursor int

	// form is the field being typed, while one is. Every value goes through
	// it, new or not: it is the one widget that keeps a pasted value's lines.
	form *fieldForm

	// baseVersion is the version the edit started from; it becomes the
	// check-and-set parameter so a concurrent write cannot be clobbered.
	baseVersion int

	// creating marks a brand-new secret, which is written with cas=0 so it
	// fails if the path was taken in the meantime.
	creating bool

	// removedKeys records deletions, which leave no field behind to carry a
	// flag but still have to appear in the confirmation.
	removedKeys []string
	removed     bool

	path string
}

func newEditor(sec *vault.Secret, path string, baseVersion int, creating bool) *editor {
	e := &editor{baseVersion: baseVersion, creating: creating, path: path}
	if sec != nil {
		for k, v := range sec.Data {
			e.fields = append(e.fields, field{key: k, value: valueToString(v), orig: v})
		}
	}
	slices.SortFunc(e.fields, func(a, b field) int { return strings.Compare(a.key, b.key) })
	return e
}

// valueToString is ui.ValueToString under the name its callers here use.
func valueToString(v any) string { return ui.ValueToString(v) }

// retype converts edited text back to the type Vault sent, or valueToString's
// flattening is permanent: a touched number returns as a string, which Vault
// accepts and the application reading the secret does not. No original: string.
func retype(orig any, s string) (any, error) {
	switch orig.(type) {
	case nil, string:
		return s, nil
	case bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("expected true or false, got %q", s)
		}
		return b, nil
	case json.Number:
		// Kept as digits rather than parsed: a float64 would round any integer
		// past 2^53 on the way back to the server.
		if !isJSONNumber(s) {
			return nil, fmt.Errorf("expected a number, got %q", s)
		}
		return json.Number(strings.TrimSpace(s)), nil
	case float64, int, int64:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("expected a number, got %q", s)
		}
		return f, nil
	default:
		var v any
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return nil, fmt.Errorf("expected valid JSON: %s", jsonProblem(s, err))
		}
		return v, nil
	}
}

// jsonProblem places a decoding error on its line, which is what the form's
// numbered lines are for. The decoder only knows a byte offset.
func jsonProblem(s string, err error) string {
	var syntax *json.SyntaxError
	if !errors.As(err, &syntax) {
		return err.Error()
	}
	at := min(int(syntax.Offset), len(s))
	line := 1 + strings.Count(s[:at], "\n")
	return fmt.Sprintf("line %d: %s", line, syntax.Error())
}

// isJSONNumber reports whether s is one JSON number token and nothing else.
func isJSONNumber(s string) bool {
	// Valid first: the decoder stops after one value and would let "0}" through.
	if !json.Valid([]byte(s)) {
		return false
	}
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return false
	}
	_, ok := v.(json.Number)
	return ok
}

// typeName is the word the editor shows for what retype will make of a value.
// It is read off orig, which is the one source of that decision.
func typeName(orig any) string {
	switch orig.(type) {
	case nil, string:
		return "string"
	case json.Number, float64, int, int64:
		return "number"
	case bool:
		return "bool"
	default:
		return "json"
	}
}

// typeCycle is the order the form offers a new field's types in. Each entry is
// the zero value retype keys on, so choosing a type is choosing an orig.
var typeCycle = []any{nil, json.Number(""), false, map[string]any{}}

// data assembles what will be written: an untouched field exactly as Vault sent
// it, a touched one as its original type. The error is not advisory, because
// the corruption it prevents is invisible until something else reads it.
func (e *editor) data() (map[string]any, error) {
	out := make(map[string]any, len(e.fields))
	for _, f := range e.fields {
		if !f.changed {
			out[f.key] = f.orig
			continue
		}
		v, err := retype(f.orig, f.value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.key, err)
		}
		out[f.key] = v
	}
	return out, nil
}

// dirty reports whether anything would change.
func (e *editor) dirty() bool {
	if e.creating {
		return true
	}
	for _, f := range e.fields {
		if f.changed || f.added {
			return true
		}
	}
	return e.removed
}

// changedKeys lists what the confirmation modal shows.
func (e *editor) changedKeys() []string {
	var out []string
	for _, f := range e.fields {
		switch {
		case f.added:
			// The type is a decision made in this editor, so the modal repeats
			// it: "+ port (number)" is the last look before it is written.
			if t := typeName(f.orig); t != "string" {
				out = append(out, "+ "+f.key+" ("+t+")")
			} else {
				out = append(out, "+ "+f.key)
			}
		case f.changed:
			out = append(out, "~ "+f.key)
		}
	}
	for _, k := range e.removedKeys {
		out = append(out, "- "+k)
	}
	slices.Sort(out)
	return out
}

func (e *editor) current() *field {
	if e.cursor < 0 || e.cursor >= len(e.fields) {
		return nil
	}
	return &e.fields[e.cursor]
}

func (e *editor) move(delta int) {
	if len(e.fields) == 0 {
		return
	}
	e.cursor = clampIndex(e.cursor+delta, len(e.fields)-1)
}

// add appends a field typed in this edit, with the type the form chose, and
// puts the cursor on it. The form has already refused an empty or taken key.
func (e *editor) add(key, value string, orig any) {
	// A key removed and then added back is a change to that key, not one
	// removal plus one addition: the modal used to list both.
	e.removedKeys = slices.DeleteFunc(e.removedKeys, func(k string) bool { return k == key })
	e.removed = len(e.removedKeys) > 0
	e.fields = append(e.fields, field{key: key, value: value, orig: orig, added: true, changed: true})
	e.cursor = len(e.fields) - 1
}

// setValue is what the form hands back for an existing field. Typing the same
// value again is not a change.
func (e *editor) setValue(index int, value string) {
	f := &e.fields[index]
	if value != f.value {
		f.value = value
		f.changed = true
	}
}

func (e *editor) hasKey(key string) bool {
	for _, f := range e.fields {
		if f.key == key {
			return true
		}
	}
	return false
}

// remove drops the selected field.
func (e *editor) remove() {
	f := e.current()
	if f == nil {
		return
	}
	// A key added in this edit was never on the server: dropping it again is
	// not a removal the modal should list, nor a reason to call the edit dirty.
	if !f.added {
		e.removedKeys = append(e.removedKeys, f.key)
		e.removed = true
	}
	e.fields = append(e.fields[:e.cursor], e.fields[e.cursor+1:]...)
	if e.cursor >= len(e.fields) {
		e.cursor = max(0, len(e.fields)-1)
	}
}

// Rendering. The editor takes over the Secrets detail pane, so it draws the
// same table the pane draws read-only, with one column made writable and a
// badge saying which of the three states the keyboard is in.

// editorKeyWidth is the width of the key column, which the header and every row
// share so the two line up. It is the same width the detail pane gives its keys,
// because the editor is the same table with one column made writable.
const editorKeyWidth = 16

func (m *Model) renderEditor(e *editor, width int) string {
	var b strings.Builder

	// What this write will do, in words. "cas=%d" names the Vault parameter
	// rather than the consequence, and is only meaningful to someone who
	// already knows check-and-set - not the person this app is for.
	intent := fmt.Sprintf("based on v%d - the save fails if someone else writes first", e.baseVersion)
	if e.creating {
		intent = "new secret - the save fails if the path already exists"
	}
	b.WriteString(m.st.Dim.Render(ui.Truncate(intent, width)))
	b.WriteString("\n\n")

	// The badge names what the keyboard is doing, in the place the mask badge
	// sits on the read-only pane. Without it, typing a value and looking at one
	// differ by a caret that is easy to miss and invisible in a screenshot.
	b.WriteString(m.st.HeadingBadged("data", m.editorBadge(e), width))
	b.WriteString("\n")
	b.WriteString(m.st.Help.Render(ui.Truncate(
		fmt.Sprintf("    %-*s %s", editorKeyWidth, "KEY", "VALUE"), width)))
	b.WriteString("\n")

	for i, f := range e.fields {
		b.WriteString(ui.Truncate(m.editorRow(e, f, i), width))
		b.WriteString("\n")
	}

	if len(e.fields) == 0 {
		b.WriteString(m.st.Dim.Render("  (no keys yet - press a to add one)\n"))
	}

	// The form floats over this pane with hints of its own; the table's would
	// show past the form's edge, naming keys that are letters in there.
	if e.form == nil {
		b.WriteString("\n")
		b.WriteString(ui.Truncate(m.st.HintLine(editorKeys()), width))
	}
	return b.String()
}

// editorRow is one key and its value, in the state that row is currently in.
func (m *Model) editorRow(e *editor, f field, i int) string {
	cursor := "  "
	if i == e.cursor {
		cursor = m.st.Cursor.Render("› ")
	}
	// Two marks in two colours: a key that did not exist and one whose value
	// moved are different facts, and the modal lists them under different signs.
	flag := " "
	switch {
	case f.added:
		flag = m.st.OKBanner.Render("+")
	case f.changed:
		flag = m.st.Changed.Render("~")
	}

	key := m.st.Key.Render(fmt.Sprintf("%-*s", editorKeyWidth, ui.Sanitize(f.key)))
	row := fmt.Sprintf("%s%s %s %s", cursor, flag, key, m.editorValue(f))

	// The type rides on the row for every field that is not a plain string, and
	// on every added field whatever its type: it was a choice made in this edit.
	if t := typeName(f.orig); f.added || t != "string" {
		row += "  " + m.st.Dim.Render(t)
	}
	return row
}

// editorValue is the value column, which used to draw two states alike: an
// empty field looked like the mask, and dots mean "a value is here and hidden".
// A value over several lines says how many, never the lines themselves.
func (m *Model) editorValue(f field) string {
	if f.value == "" {
		return m.st.Dim.Render("(empty)")
	}
	lines := strings.Count(f.value, "\n") + 1
	if !m.revealed[f.key] {
		if lines > 1 {
			return m.st.Masked.Render(ui.MaskedValue) + m.st.Dim.Render(fmt.Sprintf("  %d lines", lines))
		}
		return m.st.Masked.Render(ui.MaskedValue)
	}
	if lines > 1 {
		first, _, _ := strings.Cut(f.value, "\n")
		return ui.Sanitize(first) + m.st.Dim.Render(fmt.Sprintf("  ⏎ +%d", lines-1))
	}
	return ui.Sanitize(f.value)
}

// editorBadge says which of the editor's two states the keyboard is in: on the
// table, or in the form typing one field.
func (m *Model) editorBadge(e *editor) string {
	if e.form != nil {
		return m.st.BadgeWarn.Render(" EDITING A FIELD ")
	}
	return m.st.Badge.Render(" NAVIGATING ")
}

// Opening, driving and saving.

// beginEdit opens the field editor on the current secret.
func (m *Model) beginEdit() {
	if m.sel == nil || m.secret == nil {
		return
	}
	if m.blocks(app.OpSave, m.sel.kvVersion, "edit") {
		return
	}
	base := 0
	if m.meta != nil {
		base = m.meta.CurrentVersion
	}
	if !app.CanEdit(m.secretVersion, base) {
		m.notify("historical version: return to the current one to edit (rollback is 'b')")
		return
	}
	m.mode = editing{editor: newEditor(m.secret, m.sel.fullPath(), base, false)}
}

// beginCreate starts a new secret under the selected directory or mount, or a
// sibling when the selection is itself a secret: the moment you want a second
// secret beside this one is exactly when the cursor is on the first.
func (m *Model) beginCreate() {
	n := m.createParent()
	if n == nil {
		return
	}
	if m.blocks(app.OpCreate, n.kvVersion, "create") {
		return
	}

	in := ui.NewInput()
	in.Prompt = ""
	in.Placeholder = "name"
	// Unset, the widget draws one rune of the placeholder and stops.
	in.SetWidth(48)
	in.Focus()

	// The folder the name lands in, with one slash to join on: a mount's own
	// path already ends in one, a directory's does not.
	folder := strings.TrimSuffix(n.fullPath(), "/") + "/"

	m.mode = confirming{confirmation: &confirmation{
		title: "New secret in " + folder,
		// What enter does and what it does not: nothing is written here. The
		// check-and-set is the editor's to explain, at the save, where it bites.
		lines: []string{
			"A slash in the name makes folders: team/api/db.",
			"Next comes the editor for its fields; nothing is written",
			"until you save them there.",
		},
		input:        in,
		inputLabel:   "name",
		previewLabel: "path",
		preview: func(typed string) string {
			typed = strings.Trim(strings.TrimSpace(typed), "/")
			if typed == "" {
				return m.st.Dim.Render(folder + "…")
			}
			return ui.Sanitize(folder + typed)
		},
		confirmLabel: m.st.HintLine([]ui.LegendEntry{
			ui.Entry("enter", "open the editor"), ui.Entry("esc", "cancel"),
		}),
		// The name is free text, so the modal captures keystrokes even though
		// it is not one of the dangerous ones.
		capture: true,
		// The typed name is filled in when the modal is accepted, which is the
		// one moment the input still exists.
		onConfirm: confirmedCreateMsg{parent: n},
	}}
}

// createParent is the folder a new secret would go in: the selected node, or
// its parent when the selection is itself a secret. nil when there is nowhere
// to write, which is what the ":create" scope reads.
func (m *Model) createParent() *node {
	n := m.tree.current()
	if n == nil {
		return nil
	}
	if n.kind == kindSecret {
		n = n.parent
	}
	if n == nil || !n.isKV() {
		return nil
	}
	return n
}

// createSecret opens the editor on a secret that does not exist yet.
func (m *Model) createSecret(parent *node, name string) {
	// The same trim the modal's path row applied, so what was previewed is
	// what is made: "/db/" is db, not a path with empty segments.
	name = strings.Trim(strings.TrimSpace(name), "/")
	if parent == nil || name == "" {
		return
	}
	child := newChild(parent, kindSecret, name)
	m.sel = child
	m.secret = &vault.Secret{Mount: child.mount, Path: child.path, Data: map[string]any{}}
	m.mode = editing{editor: newEditor(m.secret, child.fullPath(), 0, true)}
}

// keyEdit routes keys while the field editor is open. The editor comes from the
// mode, so there is no case where it is missing.
func (m *Model) keyEdit(e *editor, msg tea.KeyPressMsg) tea.Cmd {
	if e.form != nil {
		cmd, done := m.keyForm(e, e.form, msg)
		if done {
			e.form = nil
		}
		return cmd
	}

	switch msg.String() {
	case "down":
		e.move(1)
	case "up":
		e.move(-1)
	case "enter":
		if f := e.current(); f != nil {
			e.form = newEditForm(m.st, e.cursor, *f, m.width, m.bodyHeight())
			return textarea.Blink
		}
	case "a":
		e.form = newAddForm(m.st, m.width, m.bodyHeight())
		return textinput.Blink
	case "x":
		e.remove()
	case "ctrl+s":
		m.confirmSave(e)
	case "esc":
		if e.dirty() {
			m.mode = confirming{confirmation: newConfirm(
				"Discard changes?",
				append([]string{"The following changes will be lost:"}, e.changedKeys()...),
				confirmedDiscardMsg{creating: e.creating})}
			return nil
		}
		m.mode = browsing{}
	}
	return nil
}

// confirmSave opens the modal for a write, saying plainly what a v1 mount does
// differently: it overwrites, and the old value is not recoverable.
func (m *Model) confirmSave(e *editor) {
	n := m.sel
	if n == nil {
		return
	}
	if m.blocks(app.OpSave, n.kvVersion, "save") {
		return
	}
	if !e.dirty() {
		m.notify("nothing changed")
		return
	}

	// Built before the confirmation is armed: a value that no longer fits its
	// original type must stop the save here, not be discovered after the user
	// has confirmed it.
	data, err := e.data()
	if err != nil {
		m.notify("save: " + err.Error())
		return
	}

	lines := []string{n.fullPath(), ""}
	lines = append(lines, e.changedKeys()...)
	lines = append(lines, "")

	// The check-and-set value is app's decision; what follows only says out loud
	// what that decision means, in the words this modal has room for.
	cas := app.CAS(e.creating, n.kvVersion, e.baseVersion)

	op := app.OpSave
	if e.creating {
		op = app.OpCreate
	}

	switch {
	case e.creating:
		lines = append(lines, "Creates the secret. Fails if the path already exists.")
	case app.Warns(op, n.kvVersion):
		lines = append(lines,
			m.st.Warn.Render("KV v1 mount: this overwrites the current value."),
			m.st.Warn.Render("There is no history, and no way to recover the previous value."))
	default:
		lines = append(lines, fmt.Sprintf("Writes a new version based on v%d.", e.baseVersion))
	}

	c := newConfirm("Save secret", lines,
		confirmedSaveMsg{node: n, data: data, cas: cas, editor: e})
	// Cancelling a save is not discarding one: the editor is what the modal
	// replaced, so it is where declining goes back to.
	c.onCancel = restoreModeMsg{mode: editing{editor: e}}
	m.mode = confirming{confirmation: c}
}

// casConflictModal explains a rejected write and offers the only two honest
// options: discard and reload, or keep editing. vivi never overwrites on the
// user's behalf.
func (m *Model) casConflictModal(n *node, e *editor) *confirmation {
	c := newConfirm("Version conflict", []string{
		"This secret changed on the server since you opened it.",
		"Nothing was written; your changes are still here.",
		"",
		"[y] reload from the server and discard my changes",
		"[n/esc] keep editing",
	}, confirmedReloadMsg{node: n})
	c.onCancel = restoreModeMsg{mode: editing{editor: e}}
	return c
}
