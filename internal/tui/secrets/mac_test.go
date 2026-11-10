package secrets

import (
	"testing"

	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// macHarness is newHarness on a Mac, whose terminal keeps Home, End, PgUp and
// PgDn for its own scrollback.
func macHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.ctx.Keys = ui.KeysFor("darwin")
	return h
}

// On a Mac g and G do what home and end do on a PC keyboard. The tree skips its
// section rows, so its ends are read off what home and end reach there.
func TestOnAMacGAndShiftGReachTheEndsOfTheTree(t *testing.T) {
	pc, mac := newHarness(t), macHarness(t)

	pc.press("end")
	mac.press("G")
	if mac.m.tree.Cursor() != pc.m.tree.Cursor() {
		t.Errorf("G left the cursor at %d, end at %d", mac.m.tree.Cursor(), pc.m.tree.Cursor())
	}
	bottom := pc.m.tree.Cursor()

	pc.press("home")
	mac.press("g")
	if mac.m.tree.Cursor() != pc.m.tree.Cursor() {
		t.Errorf("g left the cursor at %d, home at %d", mac.m.tree.Cursor(), pc.m.tree.Cursor())
	}
	if pc.m.tree.Cursor() == bottom {
		t.Fatal("precondition: the tree is one row long, so its ends prove nothing")
	}
}

// The field list, the version list and the diff each keep a cursor of their
// own, and each answers g and G on a Mac.
func TestOnAMacGAndShiftGReachTheEndsOfEveryListInTheTab(t *testing.T) {
	for _, tc := range []struct {
		name   string
		open   func(*harness)
		cursor func(*harness) int
		last   func(*harness) int
	}{
		{
			"the fields of a secret",
			func(h *harness) { h.open("kv/app/prod/db"); h.press("enter") },
			func(h *harness) int { return h.m.fieldCursor },
			func(*harness) int { return 2 }, // host, password, username
		},
		{
			"the versions",
			func(h *harness) { h.open("kv/app/prod/db"); h.press("v") },
			func(h *harness) int { return h.m.versionCursor },
			func(*harness) int { return 3 }, // the fixture holds four
		},
		{
			"the diff",
			func(h *harness) { *h = *openDiff(h.t); h.ctx.Keys = ui.KeysFor("darwin") },
			func(h *harness) int { return h.m.diffCursor },
			func(h *harness) int { return len(h.m.mode.(showingDiff).diff.rows) - 1 },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := macHarness(t)
			tc.open(h)
			if tc.last(h) < 1 {
				t.Fatalf("precondition: the list is one row long, so its ends prove nothing")
			}

			h.press("G")
			if got, want := tc.cursor(h), tc.last(h); got != want {
				t.Errorf("G left the cursor at %d, want %d", got, want)
			}
			h.press("g")
			if got := tc.cursor(h); got != 0 {
				t.Errorf("g left the cursor at %d, want 0", got)
			}
		})
	}
}
