package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

func TestFocusMovesIntoTheDetailPaneAndBack(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	if h.m.focus != ui.FocusList {
		t.Fatal("focus should start on the list")
	}

	// Right means "further in", and on a secret there is nothing to expand, so
	// further in is the pane already showing it.
	h.press("right")
	if h.m.focus != ui.FocusDetail {
		t.Fatal("right on a secret should step into the detail pane")
	}
	h.press("esc")

	h.press("enter")
	if h.m.focus != ui.FocusDetail {
		t.Fatal("enter on a secret should focus the detail pane")
	}

	// With the pane focused, the arrows drive the pane instead of the tree.
	cursorBefore := h.m.secretsTab.State().TreeCursor
	h.press("down", "down")
	if h.m.secretsTab.State().TreeCursor != cursorBefore {
		t.Fatal("the tree cursor must not move while the detail pane has focus")
	}

	// Left is the mirror of the right that got here: it leaves the pane. What it
	// must never do is reach the tree behind it and collapse a node the cursor
	// is not on, which is what falling through would have done.
	h.press("left")
	if h.m.focus != ui.FocusList {
		t.Fatal("left should step back out to the list")
	}
	if h.m.secretsTab.State().TreeCursor != cursorBefore {
		t.Fatal("left reached the tree behind the focused pane")
	}

	h.press("enter")
	h.press("esc")
	if h.m.focus != ui.FocusList {
		t.Fatal("esc should return focus to the list")
	}
	h.press("down")
	if h.m.secretsTab.State().TreeCursor == cursorBefore {
		t.Fatal("the tree should move again once focus is back")
	}
}

func TestEscapeLeavesTheDetailPane(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")

	if h.m.focus != ui.FocusDetail {
		t.Fatal("precondition: the pane should be focused")
	}
	h.press("esc")
	if h.m.focus != ui.FocusList {
		t.Fatal("esc should return focus to the list")
	}
}

// Navigation is arrow keys only. Letters are reserved for actions, so a stray
// vim habit must not move anything: pressing one where a cursor lives has to be
// a no-op, not a hidden second way to navigate.
func TestLettersDoNotNavigate(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	// "j", "k", "g" and "G" are the ones muscle memory reaches for.
	for _, key := range []string{"j", "k", "g", "G"} {
		before := h.m.secretsTab.State().TreeCursor
		h.press(key)
		if now := h.m.secretsTab.State().TreeCursor; now != before {
			t.Fatalf("%q moved the tree cursor from %d to %d", key, before, now)
		}
	}

	// The same inside the detail pane, where they would scroll.
	h.press("enter")
	for _, key := range []string{"j", "k"} {
		before := h.m.detail.YOffset()
		h.press(key)
		if h.m.detail.YOffset() != before {
			t.Fatalf("%q scrolled the detail pane", key)
		}
	}

	// "h" and "l" must not move focus either.
	h.press("h")
	if h.m.focus != ui.FocusDetail {
		t.Fatal("h changed the focused pane")
	}
	h.press("esc")
	h.press("l")
	if h.m.focus != ui.FocusList {
		t.Fatal("l changed the focused pane")
	}
}

// Recorded from an ABNT2 keyboard on Windows: "?" and "/" arrive with Alt set,
// each preceded by the NUL half of the Ctrl+Alt pair Windows makes of AltGr.
func TestAltGrRunesReachTheKeyTable(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"?", "?"},
		{"/", "/"},
	} {
		msg := tea.KeyPressMsg{Code: []rune(tc.name)[0], Text: tc.name, Mod: tea.ModAlt}
		got, ok := normalizeKey(msg)
		if !ok || got.String() != tc.want {
			t.Errorf("AltGr %q normalised to %q (ok=%v), want %q", tc.name, got.String(), ok, tc.want)
		}
	}

	nul := tea.KeyPressMsg{Code: 0, Text: "\x00", Mod: tea.ModAlt}
	if _, ok := normalizeKey(nul); ok {
		t.Error("the NUL half of an AltGr sequence should not reach the key table")
	}

	plain := tea.KeyPressMsg{Code: 'a', Text: "a"}
	if got, ok := normalizeKey(plain); !ok || got.String() != "a" {
		t.Errorf("an ordinary key changed: %q", got.String())
	}
}

// KEYS.md lists home and end under "anywhere", and the help panel repeats it.
// Three of the four tabs never handled them, so the key did nothing on lists
// long enough to need it.
func TestHomeAndEndMoveEveryTabsList(t *testing.T) {
	cursors := map[string]struct {
		open   func(*harness)
		cursor func(*harness) int
		size   func(*harness) int
	}{
		"policies": {
			open:   func(h *harness) { h.press("2") },
			cursor: func(h *harness) int { return h.m.policiesTab.Cursor() },
			size:   func(h *harness) int { return len(h.m.policiesTab.Names()) },
		},
		"auth": {
			open:   func(h *harness) { h.press("3") },
			cursor: func(h *harness) int { return h.m.authTab.MethodCursor() },
			size:   func(h *harness) int { return len(h.m.authTab.Methods()) },
		},
		"activity": {
			open: func(h *harness) {
				// The tab is empty until the session writes something.
				h.open("kv/app/prod/db")
				h.press("enter")
				h.press("e", "enter")
				h.typeText("-one")
				h.press("ctrl+s", "ctrl+s", "y")
				h.press("e", "enter")
				h.typeText("-two")
				h.press("ctrl+s", "ctrl+s", "y")
				h.press("4")
			},
			cursor: func(h *harness) int { return h.m.activityTab.Cursor() },
			size:   func(h *harness) int { return len(h.m.activityTab.Entries()) },
		},
	}

	for name, tab := range cursors {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			tab.open(h)

			if n := tab.size(h); n < 2 {
				t.Fatalf("the %s list holds %d entries, too few to move through", name, n)
			}

			h.press("end")
			if got, want := tab.cursor(h), tab.size(h)-1; got != want {
				t.Errorf("end left the cursor at %d, want %d", got, want)
			}

			h.press("home")
			if got := tab.cursor(h); got != 0 {
				t.Errorf("home left the cursor at %d, want 0", got)
			}
		})
	}
}
