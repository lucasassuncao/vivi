package tui

import (
	"strings"
	"testing"
)

// The reference lines are copied by a click, not from the palette: one
// command per line crowded it with names that change with the cursor.
func TestTheReferenceLinesAreNotInThePalette(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	for _, c := range h.m.available() {
		if strings.HasPrefix(c.name, "copy-") {
			t.Errorf("the palette offers :%s", c.name)
		}
	}
}

// The panel is what replaced the Y and C keys, so what those keys used to put on
// the clipboard has to be readable on screen instead. What goes in the rows is
// the tab's and is proved there; that they reach the screen is proved here.
func TestCopyPanelPrintsWhatTheKeysUsedToCopy(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	view := h.view()
	for _, want := range []string{
		"Command Reference",                  // the panel is there
		"kv/app/prod/db",                     // what Y copied
		"vault kv get -mount=kv app/prod/db", // what C copied
		"vault kv list -mount=kv app/prod",   // the listing beside it
		"policy-path kv/data/app/prod/db",    // the row people came for
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the reference panel does not show %q:\n%s", want, view)
		}
	}
}
