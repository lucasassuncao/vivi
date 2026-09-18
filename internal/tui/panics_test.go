package tui

import (
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The empty-message and nil-map tests went with the tab whose messages and maps
// they are. What is left here is the shell's own: the modes it owns, the panel
// it draws, the clippers, and the crash report.

// A mode is a state plus what it is about, travelling together so no handler
// checks. Nothing enforces that, and an empty payload crashed whichever
// renderer drew it next - on every frame after, which cannot be recovered.
func TestModesWithNoPayloadDoNotEndTheSession(t *testing.T) {
	modes := []mode{
		nil,
		commanding{},
		showingHelp{},
		showingToken{},
	}

	for _, md := range modes {
		h := newHarness(t)
		h.open("kv/app/prod/db")
		h.m.mode = md

		h.view()
		h.press("down", "enter", "esc", "y", "?")
		h.view()

		if h.m.mode == nil {
			t.Errorf("%T left the app with no mode", md)
		}
	}
}

// The cmdline cursor indexes a list the model regenerates every keystroke, and
// an answer landing while the panel is open can drop commands. A cursor past
// the end put start past end, and cands[start:end] panics.
func TestAStaleCommandCursorDoesNotCrashThePanel(t *testing.T) {
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	h.open("kv/app/prod/db")
	h.press(":")

	line, ok := h.m.mode.(commanding)
	if !ok || line.line == nil {
		t.Fatal("precondition: the command line should be open")
	}
	// Further past the end than the window is wide, which is what turned a
	// stale index into an inverted slice.
	line.line.cursor = len(commands) + cmdlineRows

	h.view()
	h.press("tab", "enter")
	h.view()
}

// Every panel floors its own height, so nothing reaches these two with a
// negative one today. They are the last step before a slice bound, though, and
// lines[:-1] does not clip: it ends the session.
func TestClippersRefuseNegativeHeights(t *testing.T) {
	for _, height := range []int{-3, -1, 0} {
		if got := clipHeight("a\nb\nc", height); got != "" {
			t.Errorf("clipHeight(height=%d) = %q, want empty", height, got)
		}
		if got := ui.FitPanel("a\nb\nc", 10, height); got != "" {
			t.Errorf("fitPanel(height=%d) = %q, want empty", height, got)
		}
	}
}

// Every test above covers a crash already found; this covers the ones not. Every
// key the app binds plus a few it does not, resizing mid-run because a cursor
// set on two panes and read on one is where the guards earn it. Seeds are fixed.
func TestRandomInputNeverEndsTheSession(t *testing.T) {
	if testing.Short() {
		t.Skip("soak test")
	}

	keys := []string{
		"up", "down", "left", "right", "enter", "esc", "tab", "shift+tab",
		"1", "2", "3", "4", "?", "i", ":", "/", "space", "home", "end",
		"pgup", "pgdown", "backspace", "ctrl+s",
		"r", "y", "Y", "C", "e", "a", "x", "d", "D", "M", "v", "V", "b", "u", "p", "R",
		// Unbound, and still routed: a key with no handler must fall through
		// rather than reach a branch that assumed it was one of the above.
		"k", "s", "g", "o", "n", "t", "w", "z", "0", "9", "-", ".", "*",
	}
	sizes := []tea.WindowSizeMsg{
		{Width: 120, Height: 40}, {Width: 80, Height: 24}, {Width: 60, Height: 20},
		{Width: 30, Height: 10}, {Width: 200, Height: 60}, {Width: 25, Height: 8},
		{Width: 1, Height: 1}, {Width: narrowLimit, Height: 12},
	}

	for seed := int64(0); seed < 10; seed++ {
		rng := rand.New(rand.NewPCG(uint64(seed), 0))
		h := newHarness(t)
		h.open("kv/app/prod/db")

		for step := 0; step < 200; step++ {
			if rng.IntN(25) == 0 {
				h.m.Update(sizes[rng.IntN(len(sizes))])
			}
			h.press(keys[rng.IntN(len(keys))])
			// Drawing is half of what can crash, and it is the half that fails
			// on every frame from then on rather than once.
			h.view()
		}
	}
}

// The crash barrier does not stop a panic - that would leave the model half
// updated. It writes down the work in flight, because the editor is the only
// place an unsaved edit exists, then gets out of Bubble Tea's way.
func TestACrashRecordsTheUnsavedEditAndStillDies(t *testing.T) {
	const value = "sup3r-s3cret-value"

	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter", "e")

	// Type a new value into the first field, the way a user does: enter opens
	// it, the text replaces it, ctrl+s applies it to the editor.
	h.press("enter")
	h.typeText(value)
	h.press("ctrl+s")

	if h.mode() != "edit" {
		t.Fatalf("precondition: the editor should be open, mode is %q", h.mode())
	}

	var caught any
	func() {
		defer func() { caught = recover() }()
		defer h.m.guardPanic()
		panic("boom")
	}()

	if caught == nil {
		t.Fatal("the barrier swallowed the panic instead of re-raising it")
	}

	path := crashPathFrom(t, caught)
	report, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	t.Cleanup(func() { os.Remove(path) })

	note := string(report)
	if !strings.Contains(note, "boom") {
		t.Errorf("the report does not carry the panic:\n%s", note)
	}
	if !strings.Contains(note, "UNSAVED") || !strings.Contains(note, "kv/app/prod/db") {
		t.Errorf("the report does not name the unsaved edit:\n%s", note)
	}
	// The keys render sorted, so the first field is "host".
	if !strings.Contains(note, "~ host") {
		t.Errorf("the report does not name the changed key:\n%s", note)
	}
	// The whole point of writing keys rather than fields.
	if strings.Contains(note, value) {
		t.Errorf("the report put a secret value on disk:\n%s", note)
	}
}

// crashPathFrom pulls the report's path out of the value the barrier re-raised,
// checking on the way that the original panic travelled with it unchanged.
func crashPathFrom(t *testing.T, caught any) string {
	t.Helper()
	cp, ok := caught.(crashPanic)
	if !ok {
		t.Fatalf("the barrier re-raised %T, so it never wrote a report", caught)
	}
	if cp.value != "boom" {
		t.Errorf("the original panic was %v, want boom", cp.value)
	}
	if !strings.Contains(cp.Error(), "crash report written to "+cp.path) {
		t.Errorf("the terminal message does not name the report: %q", cp.Error())
	}
	return cp.path
}

// The report is built from a model that has just proved it is not the shape the
// code assumed. A panic while describing it skips the deferred call that hands
// the terminal back - the one failure this barrier exists to prevent.
func TestTheCrashReportSurvivesABrokenModel(t *testing.T) {
	if note := safeNote(&Model{}); note == "" {
		t.Error("a zero model should still produce a report")
	}
}
