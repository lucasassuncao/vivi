package tui

import (
	"strings"
	"testing"
)

// The palette generates one command per line the Command Reference is showing.
// The rows are the tab's; turning them into commands is the shell's, and these
// two are about the turning.

// One command per line, each copying that line. The banner is what the user
// sees and the clipboard is what they get: asserting only on the banner let a
// command report a copy it never made.
func TestEveryReferenceLineHasACommandThatCopiesIt(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	rows := h.m.copyRows()
	if len(rows) < 4 {
		t.Fatalf("a loaded secret should offer several reference lines, got %d", len(rows))
	}

	for _, row := range rows {
		name := row.CommandName()
		h.runCommand(name)

		if got := h.m.banner; !strings.Contains(got, "copied: "+row.Text) {
			t.Errorf(":%s did not report what it copied: banner is %q", name, got)
		}
		if got := h.copied(); got != row.Text {
			t.Errorf(":%s put %q on the clipboard, want %q", name, got, row.Text)
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

// A folder has no read, no field and no metadata, so it offers no command that
// would copy an empty line. The panel already made that judgement.
func TestAFolderOffersOnlyThePathAndTheListing(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("esc", "left") // up to the folder

	names := map[string]bool{}
	for _, c := range h.m.available() {
		names[c.name] = true
	}
	for _, want := range []string{"copy-path", "copy-list-command"} {
		if !names[want] {
			t.Errorf("a folder should offer :%s", want)
		}
	}
	for _, unwanted := range []string{
		"copy-read-command", "copy-field-command", "copy-metadata-command", "copy-policy-path",
	} {
		if names[unwanted] {
			t.Errorf("a folder offers :%s, which addresses nothing here", unwanted)
		}
	}
}
