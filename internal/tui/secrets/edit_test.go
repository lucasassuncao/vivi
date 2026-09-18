package secrets

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// The editor's own model: what it round-trips, what it refuses, and what the
// pane it takes over draws while it is open.

// Opening the editor must not change a value nobody touched. The check used to
// be "the original is not nil", which sent a JSON null back as an empty string:
// two different values, and the difference is the caller's to keep.
func TestUntouchedFieldsRoundTripUnchanged(t *testing.T) {
	sec := &vault.Secret{Data: map[string]any{
		"text":    "hello",
		"number":  float64(3),
		"boolean": true,
		"nothing": nil,
		"nested":  map[string]any{"a": "b"},
	}}

	e := newEditor(sec, "kv/x", 1, false)
	got, err := e.data()
	if err != nil {
		t.Fatalf("data: %v", err)
	}

	if len(got) != len(sec.Data) {
		t.Fatalf("field count changed: %v", got)
	}
	for k, want := range sec.Data {
		// A nested value round-trips by identity: it is handed back as the very
		// value Vault sent, never flattened into its rendering.
		if wantMap, isMap := want.(map[string]any); isMap {
			gotMap, ok := got[k].(map[string]any)
			if !ok {
				t.Errorf("%s: nested value became %#v", k, got[k])
				continue
			}
			if len(gotMap) != len(wantMap) {
				t.Errorf("%s: nested value changed: %#v", k, gotMap)
			}
			continue
		}
		if got[k] != want {
			t.Errorf("%s: got %#v, want %#v", k, got[k], want)
		}
	}
	if v, ok := got["nothing"]; !ok || v != nil {
		t.Errorf("a null field became %#v", v)
	}
}

// A field the user did edit is written as the text they typed.
func TestEditedFieldsAreWrittenAsTyped(t *testing.T) {
	sec := &vault.Secret{Data: map[string]any{"a": "old", "keep": float64(7)}}
	e := newEditor(sec, "kv/x", 1, false)
	e.setValue(0, "new")

	got, err := e.data()
	if err != nil {
		t.Fatalf("data: %v", err)
	}
	if got["a"] != "new" {
		t.Errorf("edited field: got %#v, want \"new\"", got["a"])
	}
	if got["keep"] != float64(7) {
		t.Errorf("untouched neighbour changed: got %#v", got["keep"])
	}
}

// Editing a field must not change its type. Vault stores JSON, so a number that
// comes back as a string is a different value, and the application reading the
// secret is the one that finds out.
func TestEditedValuesKeepTheirType(t *testing.T) {
	sec := &vault.Secret{Data: map[string]any{
		"port":    float64(8080),
		"enabled": true,
		"limits":  map[string]any{"cpu": "1"},
		"name":    "api",
	}}

	e := newEditor(sec, "kv/app", 1, false)
	setFieldValue(t, e, "port", "9090")
	setFieldValue(t, e, "enabled", "false")
	setFieldValue(t, e, "limits", `{"cpu":"2"}`)
	setFieldValue(t, e, "name", "web")

	got, err := e.data()
	if err != nil {
		t.Fatalf("data: %v", err)
	}

	if v, ok := got["port"].(float64); !ok || v != 9090 {
		t.Errorf("port became %#v, want float64(9090)", got["port"])
	}
	if v, ok := got["enabled"].(bool); !ok || v != false {
		t.Errorf("enabled became %#v, want bool(false)", got["enabled"])
	}
	nested, ok := got["limits"].(map[string]any)
	if !ok {
		t.Fatalf("limits became %#v, want map[string]any", got["limits"])
	}
	if nested["cpu"] != "2" {
		t.Errorf("limits.cpu = %#v, want \"2\"", nested["cpu"])
	}
	if got["name"] != "web" {
		t.Errorf("name = %#v, want \"web\"", got["name"])
	}
}

// A value that no longer fits its original type is refused, with the key named.
// Writing it as a string instead would be the corruption this test exists to
// prevent, quietly relabelled as success.
func TestEditRefusesAValueThatNoLongerParses(t *testing.T) {
	sec := &vault.Secret{Data: map[string]any{"port": float64(8080)}}

	e := newEditor(sec, "kv/app", 1, false)
	setFieldValue(t, e, "port", "not-a-number")

	got, err := e.data()
	if err == nil {
		t.Fatalf("a non-numeric value was accepted for a number field: %#v", got)
	}
	if !strings.Contains(err.Error(), "port") {
		t.Errorf("the error does not name the field: %v", err)
	}
}

// Structured values are edited as JSON, so text that is no longer JSON must be
// refused rather than written as a string: that is how an object silently
// stopped being an object.
func TestEditRefusesBrokenJSON(t *testing.T) {
	sec := &vault.Secret{Data: map[string]any{"limits": map[string]any{"cpu": "1"}}}

	e := newEditor(sec, "kv/app", 1, false)
	setFieldValue(t, e, "limits", `{"cpu": `)

	if _, err := e.data(); err == nil {
		t.Fatal("broken JSON was accepted for a structured field")
	}
}

// A brand-new field has no original type, so it is a string, and adding one
// must not be blocked by the type check.
func TestAddedFieldsAreStrings(t *testing.T) {
	e := newEditor(&vault.Secret{Data: map[string]any{}}, "kv/app", 0, true)
	e.fields = append(e.fields, field{key: "token", value: "abc", added: true, changed: true})

	got, err := e.data()
	if err != nil {
		t.Fatalf("data: %v", err)
	}
	if got["token"] != "abc" {
		t.Errorf("token = %#v, want \"abc\"", got["token"])
	}
}

// setFieldValue edits one field the way the editor's own input would.
func setFieldValue(t *testing.T, e *editor, key, value string) {
	t.Helper()
	for i := range e.fields {
		if e.fields[i].key == key {
			e.fields[i].value = value
			e.fields[i].changed = true
			return
		}
	}
	t.Fatalf("no field named %q", key)
}

// What the editor draws.

// openEditor opens the editor on a new secret with one filled field and one empty
// one, which is the shape every test below needs.
func openEditor(t *testing.T) *harness {
	t.Helper()

	h := newHarness(t)
	h.resize(110, 40)
	h.open("kv/app/prod/db")

	h.press("a")
	h.typeText("newsecret")
	h.press("enter")

	h.addField("filled", "abc")
	h.addField("blank", "") // and leave this one with no value

	if !modeIs[editing](h.m) {
		t.Fatalf("precondition: the editor should be open, mode is %q", h.mode())
	}
	return h
}

// addField walks the form the way a user does: a, the name, enter past the
// type (a string), enter to the value, the value, ctrl+s to add it.
func (h *harness) addField(key, value string) {
	h.t.Helper()
	h.press("a")
	h.typeText(key)
	h.press("enter", "enter")
	h.typeText(value)
	h.press("ctrl+s")
	if h.form() != nil {
		h.t.Fatalf("adding %s=%q left the form open: %q", key, value, h.form().err)
	}
}

// form is the open field form, or nil.
func (h *harness) form() *fieldForm {
	if e := h.editor(); e != nil {
		return e.form
	}
	return nil
}

// The two columns had no heading, so nothing on screen said which half was the
// key and which was the value - and enter, which moves to the value, had nothing
// to name.
func TestTheEditorNamesItsColumns(t *testing.T) {
	h := openEditor(t)

	view := h.view()
	key := strings.Index(view, "KEY")
	value := strings.Index(view, "VALUE")
	switch {
	case key < 0:
		t.Fatalf("the editor does not name its key column:\n%s", view)
	case value < 0:
		t.Fatalf("the editor does not name its value column:\n%s", view)
	case value < key:
		t.Errorf("the value column is headed to the left of the key column:\n%s", view)
	}
}

// The mask means "a value is here and not being shown". A field with nothing in
// it was drawn the same way, so a key you had filled in and one you had not
// were indistinguishable - the only question worth answering while writing.
func TestAnEmptyValueDoesNotLookLikeAHiddenOne(t *testing.T) {
	h := openEditor(t)
	view := h.view()

	var filled, blank string
	for _, line := range strings.Split(view, "\n") {
		switch {
		case strings.Contains(line, "filled"):
			filled = line
		case strings.Contains(line, "blank"):
			blank = line
		}
	}
	if filled == "" || blank == "" {
		t.Fatalf("both fields should be on screen:\n%s", view)
	}

	if !strings.Contains(filled, ui.MaskedValue) {
		t.Errorf("a field with a value should still be masked: %q", filled)
	}
	if strings.Contains(blank, ui.MaskedValue) {
		t.Errorf("a field with no value must not be drawn as a hidden one: %q", blank)
	}
	if !strings.Contains(blank, "empty") {
		t.Errorf("a field with no value should say so: %q", blank)
	}
}

// Typing and navigating differed by a caret, which is easy to miss. The badge
// on the table and the form's own title and hint are what make the state
// readable: which field, and what enter does right now.
func TestTheEditorSaysWhatTheKeyboardIsDoing(t *testing.T) {
	h := openEditor(t)

	if !strings.Contains(h.view(), "NAVIGATING") {
		t.Errorf("the editor does not say it is navigating:\n%s", h.view())
	}

	h.press("enter") // the form opens on the selected field, "blank"
	editing := h.view()
	if !strings.Contains(editing, "Edit blank in kv/app/prod/newsecret") {
		t.Errorf("the form does not name the field and the secret:\n%s", editing)
	}
	if !strings.Contains(editing, "[ctrl+s] apply") {
		t.Errorf("the form does not say how the value is accepted:\n%s", editing)
	}

	h.press("esc")
	h.press("a") // a new field, starting on its name
	adding := h.view()
	if !strings.Contains(adding, "Add field to kv/app/prod/newsecret") {
		t.Errorf("the form does not say a field is being added:\n%s", adding)
	}
	// enter here does not finish the field, it moves on to the type, and that
	// is the step the user could not see coming.
	if !strings.Contains(adding, "[enter] next") {
		t.Errorf("the form does not say where enter goes next:\n%s", adding)
	}
}

// Two carets, each claiming to be where the keyboard is, is worse than none:
// whatever row tab lands on, exactly one widget is focused.
func TestOnlyOneWidgetOfTheFormIsFocused(t *testing.T) {
	h := openEditor(t)
	h.press("a")

	for _, step := range []string{"start", "tab", "tab", "tab", "shift+tab"} {
		if step != "start" {
			h.press(step)
		}
		f := h.form()
		if f == nil {
			t.Fatalf("after %s the form is gone", step)
		}
		focused := 0
		if f.key.Focused() {
			focused++
		}
		if f.value.Focused() {
			focused++
		}
		want := 1
		if f.focus == focusType {
			want = 0 // the selector has no caret
		}
		if focused != want {
			t.Errorf("after %s, %d widgets are focused on row %d, want %d", step, focused, f.focus, want)
		}
	}
}

// "cas=0" names the Vault parameter rather than what the save will do, and it is
// only meaningful to someone who already knows what check-and-set is - who is
// not the person this app is for.
func TestTheEditorExplainsTheWriteInWords(t *testing.T) {
	h := openEditor(t)

	view := h.view()
	if strings.Contains(view, "cas=") {
		t.Errorf("the editor still shows the raw parameter:\n%s", view)
	}
	if !strings.Contains(view, "the save fails if the path already exists") {
		t.Errorf("the editor does not say what creating will do:\n%s", view)
	}
	// The title used to say "editing" while the body said "creating", and only
	// one of those describes what ctrl+s is about to do.
	if !strings.Contains(view, "creating kv/app/prod/newsecret") {
		t.Errorf("the panel title does not say the secret is being created:\n%s", view)
	}
	if strings.Contains(view, "editing kv/app/prod/newsecret") {
		t.Errorf("the panel title contradicts the body:\n%s", view)
	}
}

// Writing through the editor, end to end.

func TestEditSaveWritesANewVersion(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")

	h.press("e")
	if h.mode() != "edit" {
		t.Fatal("e should open the editor")
	}

	// Change the first field, apply it, save, confirm.
	h.press("enter")
	h.typeText("-changed")
	h.press("ctrl+s")
	h.press("ctrl+s")

	if h.mode() != "confirm" {
		t.Fatal("a write must be confirmed first")
	}
	h.press("y")

	versions := h.versions("app/prod/db")
	if len(versions) != 5 {
		t.Fatalf("expected a fifth version, got %d", len(versions))
	}
	if got := h.secretData("app/prod/db")["host"]; got != "db.prod.internal-changed" {
		t.Fatalf("edit did not reach the server: host=%v", got)
	}
}

// Creating a secret runs its action after the modal has been cleared, which is
// where a closure reaching back through the model for its own input panicked.
func TestCreateSecretFromADirectory(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.focus("kv/app/prod")

	h.press("a")
	if h.modal() == nil {
		t.Fatal("a should ask for the new secret's name")
	}

	h.typeText("newsecret")
	h.press("enter")

	if h.mode() != "edit" || h.editor() == nil {
		t.Fatalf("confirming the name should open the editor, mode=%v", h.mode())
	}
	if !h.editor().creating {
		t.Fatal("the editor should know it is creating, so the write uses cas=0")
	}

	h.addField("token", "brand-new")
	h.press("ctrl+s", "y")

	data := h.secretData("app/prod/newsecret")
	if data["token"] != "brand-new" {
		t.Fatalf("the new secret did not reach the server: %+v", data)
	}

	// The tree has to show it without the user reloading anything.
	if !strings.Contains(strings.Join(h.visiblePaths(), " "), "kv/app/prod/newsecret") {
		t.Fatalf("the new secret is missing from the tree: %v", h.visiblePaths())
	}
}

// The create modal used to say "cas=0" and "relative to this path" over a bare
// "> " prompt, and its hint promised "create" when enter only opens the editor.
// It names the field, shows the full path as it is typed, and says what comes
// next.
func TestTheCreateModalShowsThePathItWillMake(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.focus("kv/app/prod")
	h.press("a")

	view := stripANSI(h.view())
	t.Logf("\n%s", view)
	for _, want := range []string{"New secret in kv/app/prod/", "name   name", "path   kv/app/prod/…", "[enter] open the editor"} {
		if !strings.Contains(view, want) {
			t.Errorf("the create modal does not show %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "cas=0") {
		t.Errorf("the create modal still names the Vault parameter:\n%s", view)
	}

	h.typeText("team/api")
	if !strings.Contains(stripANSI(h.view()), "path   kv/app/prod/team/api") {
		t.Errorf("the path row does not follow what is typed:\n%s", h.view())
	}
	h.press("enter")
	if e := h.editor(); e == nil || e.path != "kv/app/prod/team/api" {
		t.Fatalf("enter should open the editor on the previewed path, got %+v", e)
	}
}

func TestCreateRefusesAPathThatAlreadyExists(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.focus("kv/app/prod")

	h.press("a")
	h.typeText("db")
	h.press("enter")

	h.addField("k", "v")
	h.press("ctrl+s", "y")

	// cas=0 means "must not exist", so the server rejects it. Not the conflict a
	// version-aware modal is for - nothing moved, the name was taken - so it
	// reads as what it is and the edit stays where it was typed.
	if h.mode() != "edit" {
		t.Fatalf("a refused create should stay in the editor, got %s", h.mode())
	}
	if !strings.Contains(h.banner(), "already exists") {
		t.Fatalf("the refusal must say the path was taken, got %q", h.banner())
	}
	if got := h.secretData("app/prod/db")["username"]; got != "svc_app" {
		t.Fatalf("the existing secret was overwritten: %v", got)
	}
}

func TestSaveIsRejectedWhenTheSecretMovedUnderneath(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")
	h.press("e", "enter")
	h.typeText("-mine")
	h.press("ctrl+s")

	// Someone else writes while the editor is open.
	if _, err := h.server.Write(t.Context(), "kv", "app/prod/db",
		map[string]any{"username": "someone-else"}, 4); err != nil {
		t.Fatalf("concurrent write: %v", err)
	}

	h.press("ctrl+s")
	h.press("y") // confirm the save; the server rejects it

	if h.modal() == nil || !strings.Contains(h.modal().title, "conflict") {
		t.Fatal("a check-and-set rejection must be reported, not swallowed")
	}

	// The modal promises the changes are still there, and escaping it has to
	// make good on that: the editor comes back with what was typed.
	h.press("esc")
	e := h.editor()
	if e == nil {
		t.Fatal("the edit must survive a rejected write")
	}
	if !e.dirty() {
		t.Fatal("the editor came back empty: the typed change was lost")
	}

	// The server keeps the other writer's version: nothing was overwritten.
	if got := h.secretData("app/prod/db")["username"]; got != "someone-else" {
		t.Fatalf("vivi overwrote a concurrent write: username=%v", got)
	}
}

// The save modal offers "[n/esc] cancel", and cancelling a save is not the same
// as discarding one: esc in the editor asks before dropping changes, so the
// modal above it must not drop them without asking at all.
func TestCancellingTheSaveModalKeepsTheEdit(t *testing.T) {
	for _, key := range []string{"n", "esc"} {
		t.Run(key, func(t *testing.T) {
			h := newHarness(t)
			h.open("kv/app/prod/db")
			h.press("enter")
			h.press("e", "enter")
			h.typeText("-mine")
			h.press("ctrl+s") // apply the field
			h.press("ctrl+s") // open the save modal

			if h.modal() == nil {
				t.Fatalf("ctrl+s should open the save modal, mode=%s", h.mode())
			}
			h.press(key)

			e := h.editor()
			if e == nil {
				t.Fatalf("cancelling the save dropped the editor, mode=%s", h.mode())
			}
			if !e.dirty() {
				t.Fatal("the editor came back empty: the typed change was lost")
			}
		})
	}
}

// The conflict modal says the changes are still there and SAFETY.md repeats it.
// n is offered beside esc, so it has to keep the same promise.
func TestDecliningTheVersionConflictKeepsTheEdit(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")
	h.press("e", "enter")
	h.typeText("-mine")
	h.press("ctrl+s")

	if _, err := h.server.Write(t.Context(), "kv", "app/prod/db",
		map[string]any{"username": "someone-else"}, 4); err != nil {
		t.Fatalf("concurrent write: %v", err)
	}

	h.press("ctrl+s")
	h.press("y") // confirm the save; the server rejects it

	if h.modal() == nil || !strings.Contains(h.modal().title, "conflict") {
		t.Fatal("a check-and-set rejection must be reported, not swallowed")
	}

	h.press("n")
	e := h.editor()
	if e == nil {
		t.Fatalf("declining the reload dropped the editor, mode=%s", h.mode())
	}
	if !e.dirty() {
		t.Fatal("the editor came back empty: the typed change was lost")
	}
}

// A create selects a node the tree never had, so cancelling one has to take it
// back out. Left there, the pane draws an empty secret under the cursor's title
// and "e" reopens the editor on a path nobody made.
func TestCancellingACreateTakesThePhantomOutOfThePane(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")

	h.focus("kv/app/prod")
	h.press("a")
	h.typeText("ghost")
	h.press("enter")
	if h.editor() == nil {
		t.Fatalf("the create modal should have opened the editor, mode=%s", h.mode())
	}
	h.addField("k", "v")

	// A create counts as dirty from the first keystroke, so leaving one always
	// goes through the discard modal.
	h.press("esc", "y")

	if h.mode() != "browse" {
		t.Fatalf("cancelling a create should return to the tree, mode=%s", h.mode())
	}
	if h.m.sel != nil && h.m.sel.fullPath() == "kv/app/prod/ghost" {
		t.Fatal("the cancelled path is still selected: e and d would act on it")
	}
	if strings.Contains(h.view(), "ghost") {
		t.Fatal("the cancelled path is still on screen")
	}
}

// cas=0 means "must not already exist". A v1 mount has no check-and-set and the
// modal promised it anyway: the write went through and the old secret was gone.
// The client reads before writing on v1 now, so the promise is kept.
func TestCreateNeverOverwritesAnExistingV1Secret(t *testing.T) {
	h := newHarness(t)

	h.focus("legacy/")
	h.press("a")
	h.typeText("old/app") // the path the fixture already holds
	h.press("enter")
	if h.editor() == nil {
		t.Fatalf("the create modal should have opened the editor, mode=%s", h.mode())
	}
	h.addField("brand", "new")
	h.press("ctrl+s")
	h.press("y")

	after := h.secretDataIn("legacy", "old/app")
	if _, ok := after["key"]; !ok {
		t.Fatalf("the existing secret was overwritten: %v", after)
	}
	if _, ok := after["brand"]; ok {
		t.Fatalf("the new data reached a path that was taken: %v", after)
	}

	// Back in the editor, nothing typed lost, and told why.
	if h.mode() != "edit" {
		t.Errorf("a refused create should return to the editor, got %s", h.mode())
	}
	if !strings.Contains(h.banner(), "already exists") {
		t.Errorf("the refusal must say the path was taken, got %q", h.banner())
	}
}

// The confirmation modal lists what will change. Removing a key and adding it
// back is one change to that key, and removing a key added a moment ago is no
// change at all; the editor used to list "+ k" beside "- k" for both.
func TestChangedKeysCollapseARemoveAndAnAdd(t *testing.T) {
	e := newEditor(&vault.Secret{Data: map[string]any{"host": "db", "port": "5432"}}, "app/db", 3, false)

	// Remove "host" (the cursor starts on it, fields are sorted), then add it back.
	e.remove()
	e.add("host", "", nil)

	got := strings.Join(e.changedKeys(), " ")
	if got != "+ host" {
		t.Errorf("remove then add back listed %q, want %q", got, "+ host")
	}
	if !e.dirty() {
		t.Error("a key added back is still a change: its value is now empty")
	}

	// Add "new", then remove it again: nothing on the server changed.
	e = newEditor(&vault.Secret{Data: map[string]any{"host": "db"}}, "app/db", 3, false)
	e.add("new", "", nil)
	e.remove()

	if got := e.changedKeys(); len(got) != 0 {
		t.Errorf("adding and removing the same key listed %v, want nothing", got)
	}
	if e.dirty() {
		t.Error("nothing changed, so the editor must not be dirty")
	}
}

// A new key is a string unless the user says otherwise, and saying otherwise
// is a choice with a word on screen, never a guess from the digits: "007"
// stays a string until the form's type reads "number", and is then refused.
func TestANewKeyTakesTheTypeItIsGiven(t *testing.T) {
	e := newEditor(&vault.Secret{Data: map[string]any{"host": "db"}}, "app/db", 3, false)
	m := &Model{}

	f := newAddForm(m.st, 100, 30)
	f.key.SetValue("port")
	f.value.SetValue("007")
	if !m.applyForm(e, f) {
		t.Fatalf("a string is never refused, got %q", f.err)
	}
	data, err := e.data()
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := data["port"].(string); !ok || v != "007" {
		t.Fatalf("untyped new key must stay a string, got %T %v", data["port"], data["port"])
	}

	// "007" is not a JSON number, and the answer is a refusal, never a 7. The
	// form stays open, on the value, with the reason beside it.
	e = newEditor(&vault.Secret{Data: map[string]any{"host": "db"}}, "app/db", 3, false)
	f = newAddForm(m.st, 100, 30)
	f.key.SetValue("port")
	f.typ = typeIndex(json.Number(""))
	f.value.SetValue("007")
	if m.applyForm(e, f) {
		t.Fatal(`"007" as a number must be refused`)
	}
	if !strings.Contains(f.err, "expected a number") || f.focus != focusValue {
		t.Fatalf("the refusal must name the type and land on the value, got %q on row %d", f.err, f.focus)
	}
	f.value.SetValue("8080")
	if !m.applyForm(e, f) {
		t.Fatalf("8080 as a number was refused: %q", f.err)
	}
	data, err = e.data()
	if err != nil {
		t.Fatal(err)
	}
	if n, ok := data["port"].(json.Number); !ok || n.String() != "8080" {
		t.Fatalf("the key is a number, got %T %v", data["port"], data["port"])
	}
	if got := strings.Join(e.changedKeys(), " "); got != "+ port (number)" {
		t.Errorf("the modal must repeat the chosen type, got %q", got)
	}

	// The other two types refuse with their own wording, so the reason reads as
	// the type it is about.
	f = newAddForm(m.st, 100, 30)
	f.key.SetValue("flag")
	f.typ = typeIndex(false)
	f.value.SetValue("8080")
	if m.applyForm(e, f) || !strings.Contains(f.err, "true or false") {
		t.Errorf(`"8080" as a bool must be refused with the bool wording, got %q`, f.err)
	}
	f.typ = typeIndex(map[string]any{})
	f.value.SetValue("not json")
	if m.applyForm(e, f) || !strings.Contains(f.err, "JSON") {
		t.Errorf(`"not json" as json must be refused with the JSON wording, got %q`, f.err)
	}
}

// The form refuses a field with no name and one that is already there: adding
// "host" beside "host" would be two rows for one key.
func TestTheFormRefusesAnEmptyOrTakenKey(t *testing.T) {
	e := newEditor(&vault.Secret{Data: map[string]any{"host": "db"}}, "app/db", 3, false)
	m := &Model{}

	f := newAddForm(m.st, 100, 30)
	f.value.SetValue("x")
	if m.applyForm(e, f) || !strings.Contains(f.err, "name") || f.focus != focusKey {
		t.Errorf("an unnamed field must be refused on the key row, got %q on row %d", f.err, f.focus)
	}
	f.key.SetValue("host")
	if m.applyForm(e, f) || !strings.Contains(f.err, "already exists") {
		t.Errorf("a taken key must be refused, got %q", f.err)
	}
	if len(e.fields) != 1 {
		t.Errorf("a refused form must add nothing, fields are %v", e.fields)
	}
}

// Through the keyboard: a names the key, → picks number, and the value lands
// on the server as that type.
func TestChoosingNumberInTheFormWritesANumber(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter", "e")

	h.press("a")
	h.typeText("port")
	h.press("enter") // on to the type
	h.press("right") // string → number
	if !strings.Contains(h.view(), "● number") {
		t.Fatalf("the selector must mark number:\n%s", h.view())
	}
	h.press("enter") // on to the value
	h.typeText("8080")
	if !strings.Contains(stripANSI(h.view()), `"port": 8080`) {
		t.Fatalf("the form must preview the number it will write:\n%s", h.view())
	}
	h.press("ctrl+s")
	if h.form() != nil {
		t.Fatalf("ctrl+s must add the field, form still open: %q", h.form().err)
	}
	if !strings.Contains(h.view(), "number") {
		t.Fatalf("an added key must show its type on the row:\n%s", h.view())
	}

	h.press("ctrl+s")
	if h.mode() != "confirm" {
		t.Fatal("a write must be confirmed first")
	}
	if !strings.Contains(strings.Join(h.modal().lines, "\n"), "+ port (number)") {
		t.Fatalf("the modal must repeat the type:\n%s", strings.Join(h.modal().lines, "\n"))
	}
	h.press("y")

	sec, err := h.server.Read(h.t.Context(), "kv", "app/prod/db", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n, ok := sec.Data["port"].(json.Number); !ok || n.String() != "8080" {
		t.Fatalf("the server got %T %v, want json.Number 8080", sec.Data["port"], sec.Data["port"])
	}
}

// A key from the server has the type the server gave it. The form shows it and
// offers neither the key nor the type: the type is a fact about the secret, not
// a choice in this editor, so tab has nowhere to go but the value.
func TestAnExistingKeyKeepsTheServersType(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter", "e")
	// The fixture is all strings; a number the server sent is what this needs.
	h.editor().fields = append(h.editor().fields, field{key: "port", value: "5432", orig: json.Number("5432")})
	h.focusField("port")

	h.press("enter")
	f := h.form()
	if f == nil {
		t.Fatal("enter on a row must open the form")
	}
	if !f.fixed || typeName(f.orig()) != "number" {
		t.Fatalf("the form must carry the server's type, got fixed=%v %s", f.fixed, typeName(f.orig()))
	}
	h.press("tab", "right")
	if f.focus != focusValue || typeName(f.orig()) != "number" {
		t.Errorf("tab and → must not reach the type of a server field, got row %d %s", f.focus, typeName(f.orig()))
	}
	if !strings.Contains(h.view(), "(the server's, fixed)") {
		t.Errorf("the form does not say why the type is not on offer:\n%s", h.view())
	}
}

// focusField puts the editor's cursor on a key.
func (h *harness) focusField(key string) {
	h.t.Helper()
	e := h.editor()
	if e == nil {
		h.t.Fatal("the editor is not open")
	}
	for i, f := range e.fields {
		if f.key == key {
			e.cursor = i
			return
		}
	}
	h.t.Fatalf("no field %q in the editor", key)
}

// The reason the form exists: the row's one-line input replaced every newline
// in a pasted certificate with a space, and nothing said so. The text area
// keeps them, the row says how many there are, and the server gets them all.
func TestAPastedCertificateKeepsItsLines(t *testing.T) {
	const pem = "-----BEGIN CERTIFICATE-----\nMIIDdzCCAl+gAwIBAgIEAgAAuTANBgkqhkiG9w0BAQUFADBaM\n-----END CERTIFICATE-----"

	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter", "e")

	h.press("a")
	h.typeText("tls_cert")
	h.press("enter", "enter")
	h.paste(pem)

	if !strings.Contains(stripANSI(h.view()), "3 lines") {
		t.Fatalf("the form does not count the pasted lines:\n%s", h.view())
	}
	// A value over lines is added with ctrl+s: enter is a line break there.
	h.press("enter")
	if h.form() == nil {
		t.Fatal("enter in a multi-line value must break the line, not add the field")
	}
	h.press("backspace", "ctrl+s")
	if h.form() != nil {
		t.Fatalf("ctrl+s must add the field, form still open: %q", h.form().err)
	}
	if !strings.Contains(stripANSI(h.view()), "3 lines") {
		t.Errorf("the row does not say the value spans lines:\n%s", h.view())
	}

	h.press("ctrl+s", "y")
	if got := h.secretData("app/prod/db")["tls_cert"]; got != pem {
		t.Fatalf("the certificate lost its lines on the way to the server:\n%q", got)
	}
}

// On Windows a paste arrives as keystrokes, newlines as enter. With enter
// accepting a one-line value, the first pasted line closed the form and the
// rest fell on the table, where "a" opens a form and "x" removes a field: an
// .env pasted into the value deleted keys. Now everything lands in the value.
func TestAPasteArrivingAsKeystrokesStaysInTheValue(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter", "e")
	before := len(h.editor().fields)

	h.press("a")
	h.typeText("env")
	h.press("enter", "enter")
	for i, line := range []string{"# exports, with an x and an a in them", "VAULT_ADDR=http://127.0.0.1:8200", "VAULT_TOKEN=root"} {
		if i > 0 {
			h.press("enter")
		}
		h.typeText(line)
	}

	f := h.form()
	if f == nil {
		t.Fatal("the paste closed the form")
	}
	if got := f.value.LineCount(); got != 3 {
		t.Errorf("the value holds %d lines, want the 3 pasted", got)
	}
	if got := len(h.editor().fields); got != before {
		t.Errorf("the paste reached the table: %d fields, had %d", got, before)
	}
}

// The type row reads arrows only, so pasted text cannot walk the selector.
func TestPastedLettersDoNotChangeTheType(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter", "e", "a")
	h.typeText("k")
	h.press("enter") // on the type row
	h.typeText("hello world, all the letters that used to cycle it")
	if got := typeName(h.form().orig()); got != "string" {
		t.Errorf("letters on the type row changed it to %s", got)
	}
	h.press("right")
	if got := typeName(h.form().orig()); got != "number" {
		t.Errorf("→ must still choose the next type, got %s", got)
	}
}

// The form is a box, not a curtain: on a wide terminal it keeps its own width,
// and the value is drawn as an area whatever it holds, with a numbered gutter
// and four rows, so a one-line value does not turn it back into an input.
func TestTheFormHasItsOwnWidthAndDrawsAnArea(t *testing.T) {
	h := newHarness(t)
	h.resize(160, 40)
	h.open("kv/app/prod/db")
	h.press("enter", "e", "enter") // the form on "host", a one-line value

	view := stripANSI(h.view())
	t.Logf("\n%s", view)
	var box []string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "│") && strings.Contains(line, "Edit host") {
			box = append(box, line)
		}
	}
	if len(box) != 1 {
		t.Fatalf("expected the form's title row once, got %d:\n%s", len(box), view)
	}
	// The form's own borders are the "│" on either side of its title; the pane
	// borders further out share the glyph, so measure from the title outwards.
	title := box[0]
	at := strings.Index(title, "Edit host")
	from := strings.LastIndex(title[:at], "│")
	to := at + strings.Index(title[at:], "│")
	if width := lineWidth(title[from:to]); width > formInnerWidth+6 {
		t.Errorf("the form spans %d columns on a 160-column terminal, want about %d", width, formInnerWidth+6)
	}

	if !strings.Contains(view, "  1 │ db.prod.internal") {
		t.Errorf("the value row has no gutter:\n%s", view)
	}
	for _, n := range []string{"  2 │ ~", "  3 │ ~", "  4 │ ~"} {
		if !strings.Contains(view, n) {
			t.Errorf("the area does not show its empty row %q:\n%s", n, view)
		}
	}
}

// The tall-value case is asserted in the shell: half of what it proves is that
// the whole screen still fits the terminal, and the screen is the shell's.

// A JSON value is laid out over lines as a matter of course, so enter breaks
// the line from the first keystroke and the error names the line it is on.
func TestJSONIsTypedOverLines(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter", "e")

	h.press("a")
	h.typeText("limits")
	h.press("enter", "right", "right", "right", "enter") // string → number → bool → json
	h.typeText("{")
	h.press("enter")
	h.typeText(`"rps": 100,`)
	h.press("enter")
	h.typeText("}")

	f := h.form()
	if f == nil || f.value.LineCount() != 3 {
		t.Fatalf("enter must break the line in a json value, got %d lines", f.value.LineCount())
	}
	// The trailing comma is on line 2, and the decoder trips over the brace on
	// line 3: the refusal names the line, which the numbered rows make findable.
	if !strings.Contains(stripANSI(h.view()), "✗ expected valid JSON: line 3:") {
		t.Fatalf("a JSON error must say which line it is on:\n%s", h.view())
	}
	h.press("backspace", "backspace") // the brace and the line break, back to the comma
	h.press("backspace")              // the comma
	h.press("enter")
	h.typeText("}")
	if !strings.Contains(stripANSI(h.view()), `"limits": {"rps":100}`) {
		t.Fatalf("the form must preview the object compactly:\n%s", h.view())
	}

	h.press("ctrl+s")
	if h.form() != nil {
		t.Fatalf("valid json must be added, form still open: %q", h.form().err)
	}
	h.press("ctrl+s", "y")
	got, ok := h.secretData("app/prod/db")["limits"].(map[string]any)
	if !ok || got["rps"] != float64(100) {
		t.Fatalf("the server got %#v, want an object with rps 100", h.secretData("app/prod/db")["limits"])
	}
}

// The SDK hands numbers over as json.Number. A touched one used to go through
// float64 on the way back, which rounds every integer past 2^53: an id typed as
// 9007199254740993 was saved as 9007199254740992, and nothing said so.
func TestEditedNumbersKeepTheirDigits(t *testing.T) {
	orig := json.Number("42")

	if got := valueToString(json.Number("9007199254740993")); got != "9007199254740993" {
		t.Errorf("valueToString = %q", got)
	}

	v, err := retype(orig, "9007199254740993")
	if err != nil {
		t.Fatalf("retype: %v", err)
	}
	if n, ok := v.(json.Number); !ok || n.String() != "9007199254740993" {
		t.Errorf("retype = %T %v, want json.Number with the same digits", v, v)
	}

	for _, bad := range []string{"abc", "", "4 2", "1,5", "[1]", "\"7\""} {
		if _, err := retype(orig, bad); err == nil || !strings.Contains(err.Error(), "expected a number") {
			t.Errorf("retype(%q): err = %v, want a number error", bad, err)
		}
	}
}

// Whatever is typed into a numeric field, retype either returns the digits it
// was given or refuses; it never returns a value that would print differently.
func FuzzRetypeNumber(f *testing.F) {
	// "0}" is the fuzzer's own find: the decoder stops after one value and
	// json.Decoder.More reads a closing brace as "no more", not as garbage.
	for _, s := range []string{"0", "-1", "3.14", "1e10", "9007199254740993", "abc", "", " 7 ", "0}"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		v, err := retype(json.Number("1"), s)
		if err != nil {
			return
		}
		n, ok := v.(json.Number)
		if !ok {
			t.Fatalf("retype(%q) = %T, want json.Number", s, v)
		}
		if n.String() != strings.TrimSpace(s) {
			t.Fatalf("retype(%q) = %q, digits changed", s, n)
		}
		if !json.Valid([]byte(n)) {
			t.Fatalf("retype(%q) accepted something the encoder would refuse", s)
		}
	})
}

// The editor and the clipboard are not rendering: both must see the real bytes,
// or a save would write the escaped text back into the secret.
func TestSanitizeIsNotAppliedToTheEditedValue(t *testing.T) {
	raw := "line1\nline2"
	sec := &vault.Secret{Data: map[string]any{"multi": raw}}

	e := newEditor(sec, "kv/x", 1, false)
	got, err := e.data()
	if err != nil {
		t.Fatalf("data: %v", err)
	}
	if got["multi"] != raw {
		t.Errorf("the editor round-tripped %q as %q", raw, got["multi"])
	}
}
