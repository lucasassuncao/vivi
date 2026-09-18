package secrets

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The one place a value is typed. The editor's table used to open a single-line
// input on the row, which silently flattened a pasted certificate into one line:
// the input widget replaces every newline. A field is typed in this modal now,
// whether it is new or from the server, through a text area that keeps them.

// formFocus is which row of the form the keyboard is on.
type formFocus int

const (
	focusKey formFocus = iota
	focusType
	focusValue
)

// fieldForm is a value being typed for one field of the editor: the key and the
// type, both fixed when the field came from the server, and the value.
type fieldForm struct {
	key   textinput.Model
	value textarea.Model
	focus formFocus

	// typ indexes typeCycle, which is how choosing a type is choosing the orig
	// retype keys on. fixed marks a field from the server: its key and type are
	// facts about the secret, so only the value takes focus.
	typ   int
	fixed bool

	// index is the field being edited, or -1 for one being added.
	index int

	// err is what the value fails against its type, live, so the refusal lands
	// beside the value rather than in a banner at save time.
	err string
	// tried marks an accept that was refused, which is when an empty value is
	// worth complaining about: before that it is merely not typed yet.
	tried bool
}

// formLabelWidth is the label column: "  value  ", which the text area's rows
// after the first are indented to.
const formLabelWidth = 9

// Form sizing. The form has a width of its own rather than the terminal's:
// wide enough that a PEM line, 64 characters, fits beside the gutter without
// wrapping, and no wider, so on a big screen it is a box and not a curtain.
const (
	formInnerWidth = 80
	// formMinRows is the least the value area shows. Four rows even for a
	// one-line value, so the area reads as one: a single row is an input.
	formMinRows = 4
	// gutterWidth is "%3d │ ": three digits, the rule, a space.
	gutterWidth = 6
)

// formInner is the form's content width on this terminal.
func formInner(width int) int { return min(formInnerWidth, ui.ModalInner(width)) }

// newAddForm starts a field that does not exist yet, a string until told
// otherwise.
func newAddForm(st ui.Styles, width, bodyHeight int) *fieldForm {
	key := ui.NewInput()
	key.Prompt = ""
	key.Placeholder = "name"
	key.Focus()

	f := &fieldForm{key: key, value: newValueArea(st), index: -1}
	f.resize(width, bodyHeight)
	return f
}

// newEditForm opens an existing field on its value. The key and type are shown
// and not offered: the type is the server's, and guessing at it is how a
// digit-only password becomes a number.
func newEditForm(st ui.Styles, index int, fld field, width, bodyHeight int) *fieldForm {
	key := ui.NewInput()
	key.Prompt = ""
	key.SetValue(fld.key)

	f := &fieldForm{key: key, value: newValueArea(st), index: index, fixed: true, focus: focusValue}
	f.typ = typeIndex(fld.orig)
	f.value.SetValue(fld.value)
	f.value.CursorEnd()
	f.value.Focus()
	f.resize(width, bodyHeight)
	f.validate()
	return f
}

// newValueArea is a text area with the widget's own limits taken off: the
// default caps a value at 400 characters and 99 lines, and a certificate is
// several thousand characters over fifty. Drawn the way yedit draws its YAML
// editor: a numbered gutter on every row, and "~" on the rows past the end.
func newValueArea(st ui.Styles) textarea.Model {
	ta := textarea.New()
	ta.CharLimit = 0
	ta.MaxHeight = 0
	ta.ShowLineNumbers = false
	ta.EndOfBufferCharacter = '~'
	ta.SetPromptFunc(gutterWidth, func(info textarea.PromptInfo) string {
		return st.Dim.Render(fmt.Sprintf("%3d │ ", info.LineNumber+1))
	})
	// The widget's own colours are fixed 256-colour numbers, not the theme's.
	// The line under the cursor is left unpainted: highlighted, it read as a
	// selection, and there is nothing to select.
	plain := lipgloss.NewStyle()
	s := ta.Styles()
	s.Cursor.BlinkSpeed = ui.BlinkSpeed
	s.Focused.CursorLine = plain
	s.Focused.Base = plain
	s.Focused.EndOfBuffer = st.Dim
	s.Blurred.CursorLine = plain
	s.Blurred.Base = plain
	s.Blurred.EndOfBuffer = st.Dim
	s.Blurred.Text = plain
	ta.SetStyles(s)
	return ta
}

// typeIndex is where an orig sits in typeCycle, so the selector can show it.
func typeIndex(orig any) int {
	name := typeName(orig)
	for i, t := range typeCycle {
		if typeName(t) == name {
			return i
		}
	}
	return 0
}

// orig is the zero value retype keys on for the chosen type.
func (f *fieldForm) orig() any { return typeCycle[f.typ] }

// resize fits the form to the terminal. The text area is as tall as its lines,
// never less than formMinRows and never more than what the modal leaves of the
// body; past that it scrolls.
func (f *fieldForm) resize(width, bodyHeight int) {
	inner := formInner(width)
	f.key.SetWidth(max(1, inner-formLabelWidth-1))
	f.value.SetWidth(max(1, inner-formLabelWidth))

	// The rows around the text area: border, title, key, type, blanks, the
	// writes line and the hint. Whatever the body has left is the area's cap.
	const chrome = 11
	capHeight := max(1, bodyHeight-chrome)
	f.value.SetHeight(min(capHeight, max(formMinRows, f.value.LineCount())))
}

// validate holds the value against its type and records the answer. A string
// is never wrong; everything else is retype's decision, the same one the save
// makes.
func (f *fieldForm) validate() {
	if _, err := retype(f.orig(), f.value.Value()); err != nil {
		f.err = err.Error()
		return
	}
	f.err = ""
}

// keyProblem is why the key cannot be accepted, or "". Only for a field being
// added: an existing key is not up for discussion.
func (f *fieldForm) keyProblem(e *editor) string {
	if f.fixed {
		return ""
	}
	key := f.key.Value()
	switch {
	case key == "":
		return "the field needs a name"
	case e.hasKey(key):
		return key + " already exists"
	}
	return ""
}

// showErr reports whether the writes line should carry the refusal. An empty
// value is not yet a mistake unless the user tried to accept it.
func (f *fieldForm) showErr() bool {
	return f.err != "" && (f.tried || f.value.Value() != "")
}

// keyForm drives the form. It returns done when the form is finished with,
// either applied or dismissed, so the editor can drop it.
func (m *Model) keyForm(e *editor, f *fieldForm, msg tea.KeyPressMsg) (cmd tea.Cmd, done bool) {
	switch msg.String() {
	case "esc":
		return nil, true
	case "ctrl+s":
		return nil, m.applyForm(e, f)
	case "tab", "shift+tab":
		if f.fixed {
			return nil, false
		}
		delta := 1
		if msg.String() == "shift+tab" {
			delta = -1
		}
		f.setFocus(formFocus((int(f.focus) + delta + 3) % 3))
		return textarea.Blink, false
	}

	switch f.focus {
	case focusKey:
		if msg.String() == "enter" {
			f.setFocus(focusType)
			return nil, false
		}
		f.key, cmd = f.key.Update(msg)
		return cmd, false

	case focusType:
		// Arrows only. Letters used to cycle the selector too, and a paste that
		// arrives as keystrokes - every paste, on Windows - walked it to json.
		switch msg.String() {
		case "enter":
			f.setFocus(focusValue)
			return textarea.Blink, false
		case "left":
			f.typ = (f.typ + len(typeCycle) - 1) % len(typeCycle)
		case "right":
			f.typ = (f.typ + 1) % len(typeCycle)
		}
		f.resize(m.width, m.bodyHeight())
		f.validate()
		return nil, false
	}

	// The value. enter is always a line break here and only ctrl+s accepts: a
	// paste the terminal delivers as keystrokes then lands whole in the area.
	// With enter accepting, the first pasted line closed the form and the rest
	// fell on the table, where "x" removes a field.
	f.value, cmd = f.value.Update(msg)
	f.resize(m.width, m.bodyHeight())
	f.validate()
	return cmd, false
}

// route hands a message that is not a key to the focused widget: the caret's
// blink timer, and the paste the text area asks the terminal for.
func (f *fieldForm) route(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch f.focus {
	case focusKey:
		f.key, cmd = f.key.Update(msg)
	case focusValue:
		f.value, cmd = f.value.Update(msg)
	}
	return cmd
}

// setFocus moves the keyboard to a row, so exactly one widget shows a caret.
func (f *fieldForm) setFocus(to formFocus) {
	f.focus = to
	f.key.Blur()
	f.value.Blur()
	switch to {
	case focusKey:
		f.key.Focus()
	case focusValue:
		f.value.Focus()
	}
}

// applyForm puts the form's answer into the editor, or refuses and says why.
// It reports whether the form is done.
func (m *Model) applyForm(e *editor, f *fieldForm) bool {
	f.tried = true
	if problem := f.keyProblem(e); problem != "" {
		f.err = problem
		f.setFocus(focusKey)
		return false
	}
	f.validate()
	if f.err != "" {
		f.setFocus(focusValue)
		return false
	}

	value := f.value.Value()
	if f.index >= 0 && f.index < len(e.fields) {
		e.setValue(f.index, value)
		return true
	}
	e.add(f.key.Value(), value, f.orig())
	return true
}

// Rendering.

// renderFieldForm draws the form as a modal over the editor's table.
func (m *Model) renderFieldForm(e *editor, f *fieldForm, height int) string {
	title := "Add field to " + e.path
	if f.fixed {
		title = "Edit " + ui.Sanitize(f.key.Value()) + " in " + e.path
	}

	inner := formInner(m.width)
	lines := []string{m.st.ModalTitle.Render(title), ""}
	lines = append(lines, m.formRow("key", m.formKey(f)))
	lines = append(lines, m.formRow("type", m.formType(f)))
	lines = append(lines, m.formValue(f)...)
	preview := m.formPreview(f, inner-formLabelWidth)
	lines = append(lines, "", m.formRow("writes", preview), "", m.formHint(f))

	// Padded to one width: a box whose border followed its longest line would
	// change width as the user typed.
	for i, line := range lines {
		lines[i] = ui.Pad(line, inner)
	}
	// The box only. The shell floats it over the panes, because the panes are
	// the shell's and a tab cannot draw over what it cannot see.
	return m.st.ModalBoxWidth(lines, height, inner, m.width)
}

// formRow is one labelled line, the label in the same column on every row.
func (m *Model) formRow(label, content string) string {
	return m.st.Key.Render(fmt.Sprintf("  %-*s", formLabelWidth-2, label)) + content
}

func (m *Model) formKey(f *fieldForm) string {
	if f.fixed {
		return ui.Sanitize(f.key.Value()) + m.st.Dim.Render("   (from the server)")
	}
	return f.key.View()
}

// formType is the selector: every type named, the chosen one marked. Marked
// with a glyph and not colour alone, so the choice survives a monochrome
// terminal; the accent says which row the arrows are driving.
func (m *Model) formType(f *fieldForm) string {
	if f.fixed {
		return typeName(f.orig()) + m.st.Dim.Render("   (the server's, fixed)")
	}
	parts := make([]string, 0, len(typeCycle))
	for i, t := range typeCycle {
		name := typeName(t)
		switch {
		case i == f.typ && f.focus == focusType:
			parts = append(parts, m.st.Editing.Render("● "+name))
		case i == f.typ:
			parts = append(parts, "● "+name)
		default:
			parts = append(parts, m.st.Dim.Render("  "+name))
		}
	}
	return strings.Join(parts, "  ")
}

// formValue is the text area, its first row carrying the label and the rest
// indented under it.
func (m *Model) formValue(f *fieldForm) []string {
	rows := strings.Split(f.value.View(), "\n")
	out := make([]string, 0, len(rows))
	for i, row := range rows {
		if i == 0 {
			out = append(out, m.formRow("value", row))
			continue
		}
		out = append(out, strings.Repeat(" ", formLabelWidth)+row)
	}
	return out
}

// formPreview is what the save will write for this field, as JSON, which is
// the shortest possible explanation of why the type matters: "30" and 30 are
// two different values. A refusal takes the line instead. width is the room
// the line has; a value over lines keeps its count in view whatever it costs
// the JSON, since the count is what says the lines survived.
func (m *Model) formPreview(f *fieldForm, width int) string {
	if f.showErr() {
		return m.st.Danger.Render("✗ " + f.err)
	}
	value := f.value.Value()
	typed, err := retype(f.orig(), value)
	if err != nil {
		return m.st.Dim.Render("…")
	}
	encoded, err := json.Marshal(typed)
	if err != nil {
		return m.st.Dim.Render("…")
	}
	key := f.key.Value()
	if key == "" {
		key = "…"
	}
	line := ui.Sanitize(fmt.Sprintf("%q: %s", key, encoded))
	n := f.value.LineCount()
	if n <= 1 {
		return line
	}
	suffix := fmt.Sprintf("   %d lines · %s", n, byteCount(len(value)))
	return ui.Truncate(line, max(1, width-lipgloss.Width(suffix))) + m.st.Dim.Render(suffix)
}

// byteCount is a size the modal has room for.
func byteCount(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// formHint says what the keys do on the row the keyboard is on. ctrl+s is the
// one way to accept, named on every row: it works from all three.
func (m *Model) formHint(f *fieldForm) string {
	verb := "add"
	if f.fixed {
		verb = "apply"
	}

	var keys []ui.LegendEntry
	switch {
	case f.focus == focusKey:
		keys = []ui.LegendEntry{ui.Entry("enter", "next"), ui.Entry("tab", "move")}
	case f.focus == focusType:
		keys = []ui.LegendEntry{ui.Entry("←/→", "choose"), ui.Entry("enter", "next"), ui.Entry("tab", "move")}
	case f.fixed:
		keys = []ui.LegendEntry{ui.Entry("enter", "new line")}
	default:
		keys = []ui.LegendEntry{ui.Entry("enter", "new line"), ui.Entry("tab", "move")}
	}
	return m.st.HintLine(append(keys, ui.Entry("ctrl+s", verb), ui.Entry("esc", "cancel")))
}
