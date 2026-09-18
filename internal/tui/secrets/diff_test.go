package secrets

import (
	"strings"
	"testing"

	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// openDiff compares the two newest versions, the only two the fake leaves
// readable: v1 is destroyed and v2 soft-deleted. v3 → v4 exercises all three
// row kinds - password changes, host is added, username stays put.
func openDiff(t *testing.T) *harness {
	t.Helper()

	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("v")
	// The list runs newest first, so the cursor opens on v4 and the row below
	// it is v3.
	h.press(" ")
	h.press("down", " ")
	h.run(h.m.startDiff())

	if h.mode() != "diff" {
		t.Fatalf("precondition: mode = %q, want the diff open", h.mode())
	}
	return h
}

// The diff used to render every value as dots with no key to lift them, so it
// could report that a field changed and never what to. The values were built
// and thrown away at render time; this is the test that they reach the screen.
func TestDiffRevealsEveryRowAtOnce(t *testing.T) {
	h := openDiff(t)

	view := h.view()
	if !strings.Contains(view, ui.MaskedValue) {
		t.Error("the diff should open masked")
	}
	for _, secret := range []string{"hunter2-v3", "hunter2-v4", "db.prod.internal"} {
		if strings.Contains(view, secret) {
			t.Errorf("%q is on screen before anything was revealed", secret)
		}
	}

	// A diff is read as a whole: the question is what changed between two
	// versions, so one press answers it rather than one press per row. A
	// changed row shows both sides, since half a pair answers nothing.
	h.press("r")
	view = h.view()
	for _, want := range []string{"db.prod.internal", "hunter2-v3", "hunter2-v4"} {
		if !strings.Contains(view, want) {
			t.Errorf("r should have revealed every row, %q is still masked", want)
		}
	}
	// Not "nothing on screen is masked": the detail pane behind the modal keeps
	// its own dots, and it is right to.

	// And puts them all back, which is what the footer offers once they show.
	h.press("r")
	view = h.view()
	for _, secret := range []string{"hunter2-v3", "hunter2-v4", "db.prod.internal"} {
		if strings.Contains(view, secret) {
			t.Errorf("r did not hide %q again", secret)
		}
	}
}

// Reveals are scoped to the modal the same way the detail pane scopes them to
// the field: closing has to put the dots back, or a diff opened once leaves the
// value unmasked for every diff after it.
func TestClosingTheDiffDropsItsReveals(t *testing.T) {
	h := openDiff(t)

	h.press("down", "r")
	if !strings.Contains(h.view(), "hunter2-v4") {
		t.Fatal("precondition: the row should be revealed")
	}

	h.press("esc")
	if h.mode() != "browse" {
		t.Fatalf("esc should close the diff, mode = %q", h.mode())
	}
	if len(h.m.diffRevealed) != 0 {
		t.Errorf("the reveals outlived the modal: %v", h.m.diffRevealed)
	}

	h.run(h.m.startDiff())
	if strings.Contains(h.view(), "hunter2-v4") {
		t.Error("the second diff opened with a value already revealed")
	}
}

// The cursor is clamped at both ends rather than wrapping, and r on a diff with
// no rows to point at has to be a no-op rather than a panic.
func TestDiffCursorStaysInRange(t *testing.T) {
	h := openDiff(t)

	h.press("up", "up", "up")
	if h.m.diffCursor != 0 {
		t.Errorf("cursor went above the first row: %d", h.m.diffCursor)
	}

	h.press("end", "down", "down")
	rows := len(h.m.mode.(showingDiff).diff.rows)
	if h.m.diffCursor != rows-1 {
		t.Errorf("cursor went past the last row: %d, want %d", h.m.diffCursor, rows-1)
	}

	empty := &diffResult{pathLabel: "kv/app", a: 1, b: 2}
	h.m.mode = showingDiff{diff: empty}
	h.m.diffCursor = 0
	h.press("r", "down", "end")
	h.view()
}

// A box is as wide as its widest line, and one revealed value can be a
// certificate on a single line. On a wide terminal that sized the modal to the
// value and the comparison became a curtain.
func TestALongValueDoesNotStretchTheDiffAcrossTheTerminal(t *testing.T) {
	h := openDiff(t)
	h.resize(250, 40)

	h.m.mode = showingDiff{diff: &diffResult{
		pathLabel: "kv/app/prod/cache", a: 1, b: 2,
		rows: []diffRow{{kind: "added", key: "pem", bValue: strings.Repeat("A", 1200)}},
	}}
	h.press("r") // reveal it, which is when the value reaches the screen

	for _, line := range strings.Split(stripANSI(h.m.Overlay(h.ctx)), "\n") {
		if w := lineWidth(line); w > ui.ModalMaxInner+8 {
			t.Fatalf("the modal is %d columns wide on a 250-column terminal: %q", w, line)
		}
	}
}
