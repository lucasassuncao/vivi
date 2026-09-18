package secrets

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The command is the point: you open the app because you remember neither the
// path nor the command, and you leave with both.
func TestReadCommandForASecret(t *testing.T) {
	n := &node{kind: kindSecret, name: "db", mount: "kv", path: "app/prod/db"}

	if got, want := readCommand(n, 0, ""), "vault kv get -mount=kv app/prod/db"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestListCommandForADirectory(t *testing.T) {
	n := &node{kind: kindDir, name: "prod", mount: "kv", path: "app/prod"}

	if got, want := listCommand(n), "vault kv list -mount=kv app/prod"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A mount has no path under it, and "vault kv list -mount=kv " with a trailing
// space is a command that looks like a bug when pasted.
func TestListCommandForAMount(t *testing.T) {
	n := &node{kind: kindMount, name: "kv", mount: "kv"}

	if got, want := listCommand(n), "vault kv list -mount=kv"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The two tests about the generated ":copy-..." commands live with the palette
// that generates them, in the shell: what this tab owes them is the rows.

// What the reference panel draws is asserted in the shell, which draws it. What
// this tab owes it is the rows, and those are proved here.

// The rows are what replaced the Y and C keys, so what those keys used to put on
// the clipboard has to be in them. If it is not, the keys went for nothing.
func TestTheRowsCarryWhatTheKeysUsedToCopy(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	var lines []string
	for _, r := range h.copyRows() {
		lines = append(lines, r.Label+" "+r.Text)
	}
	joined := strings.Join(lines, "\n")

	for _, want := range []string{
		"kv/app/prod/db",                     // what Y copied
		"vault kv get -mount=kv app/prod/db", // what C copied
		"vault kv list -mount=kv app/prod",   // the listing beside it
		"policy-path kv/data/app/prod/db",    // the row people came for
		"vault kv metadata get -mount=kv app/prod/db",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the rows do not offer %q:\n%s", want, joined)
		}
	}
}

// -field is the row worth having: it prints the value alone, with no JSON around
// it, so it pipes. It has to follow the field cursor or it names the wrong key.
func TestTheRowsFollowTheFieldCursor(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter") // into the detail pane, where the field cursor lives

	rows := func() string {
		var b strings.Builder
		for _, r := range h.copyRows() {
			b.WriteString(r.Text + "\n")
		}
		return b.String()
	}

	keys := h.m.sortedKeys()
	if len(keys) < 2 {
		t.Fatalf("this secret needs at least two fields to prove the point, got %v", keys)
	}
	if !strings.Contains(rows(), "-field="+keys[0]) {
		t.Fatalf("the rows do not name the selected field %q:\n%s", keys[0], rows())
	}

	h.press("down")
	if !strings.Contains(rows(), "-field="+keys[1]) {
		t.Errorf("the rows did not follow the cursor to %q:\n%s", keys[1], rows())
	}
}

// A pinned version is only worth printing while a historical one is on screen.
// On the current version it names what the plain read already returns, and it
// would go stale the moment anybody writes.
func TestTheRowsPinTheVersionOnlyWhenHistorical(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	rows := func() string {
		var b strings.Builder
		for _, r := range h.copyRows() {
			b.WriteString(r.Text + "\n")
		}
		return b.String()
	}

	if strings.Contains(rows(), "-version=") {
		t.Errorf("the current version should not be pinned in the command:\n%s", rows())
	}

	h.press("V")
	h.press("down") // v3: the newest one that is neither deleted nor destroyed
	h.press("enter")

	if !strings.Contains(rows(), "-version=3") {
		t.Errorf("a historical version should be pinned in the command:\n%s", rows())
	}
}

// The rows address the tree cursor, and a secret being created has no row in the
// tree - so they went on naming whichever sibling the cursor was left on.
func TestTheRowsFollowTheEditor(t *testing.T) {
	h := openEditor(t)

	var b strings.Builder
	for _, r := range h.copyRows() {
		b.WriteString(r.Label + " " + r.Text + "\n")
	}
	rows := b.String()

	if !strings.Contains(rows, "path kv/app/prod/newsecret") {
		t.Errorf("the rows do not address the secret being created:\n%s", rows)
	}
	// A read of something that does not exist yet would fail, and a put would
	// have to spell the values out. Neither belongs here.
	if strings.Contains(rows, "vault kv get -mount=kv app/prod/newsecret") {
		t.Errorf("the rows offer a read of a secret that is not saved yet:\n%s", rows)
	}
	if strings.Contains(rows, "abc") {
		t.Errorf("a value being edited reached the rows:\n%s", rows)
	}
}

// The suite used to skip these without a clipboard and use the real one with -
// so `make test` clobbered the developer's, and the assertion was really about
// how fast clip.exe starts. Model.clip is the seam: every harness has its own.

// "y" follows the focus, because the focus is already what the arrows mean: on
// the list the cursor is on a secret, in the pane it is on a field. Copying one
// field from the list was the old behaviour and gave no way to take the rest.
func TestCopyFromTheListTakesTheWholeSecret(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.ctx.Focus = ui.FocusList

	h.press("y")

	got := h.copied()
	var fields map[string]string
	if err := json.Unmarshal([]byte(got), &fields); err != nil {
		t.Fatalf("the clipboard does not hold JSON: %v\n%s", err, got)
	}
	for _, want := range h.m.sortedKeys() {
		if _, ok := fields[want]; !ok {
			t.Errorf("%q is missing from the copied secret:\n%s", want, got)
		}
	}
	if len(fields) != len(h.m.sortedKeys()) {
		t.Errorf("copied %d fields, the secret has %d", len(fields), len(h.m.sortedKeys()))
	}
}

// From the detail pane the cursor is on one field, and that is what moves.
func TestCopyFromTheDetailPaneTakesOneField(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.ctx.Focus = ui.FocusDetail
	h.m.fieldCursor = 0

	h.press("y")

	key := h.m.sortedKeys()[0]
	if got, want := h.copied(), valueToString(h.m.secret.Data[key]); got != want {
		t.Errorf("copied %q, want the value of %q (%q)", got, key, want)
	}
}
