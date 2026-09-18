package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEmptyActivityTabExplainsItself(t *testing.T) {
	h := newHarness(t)
	h.press("4")

	view := h.view()
	if !strings.Contains(view, "no writes in this session") {
		t.Fatal("an empty Activity tab should say so")
	}
	if !strings.Contains(view, "recorded here") {
		t.Fatal("an empty state should hint at what fills it")
	}
}

func TestActivityRecordsWritesWithoutSecretValues(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")
	h.press("e", "enter")
	h.typeText("-logged")
	h.press("ctrl+s", "ctrl+s", "y") // apply the field, then save the secret

	entries := h.m.activityTab.Entries()
	if len(entries) == 0 {
		t.Fatal("a write must be recorded in the activity log")
	}
	entry := entries[len(entries)-1]
	if entry.Op != "write" || !entry.OK() {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if strings.Contains(entry.Path+entry.Detail, "logged") {
		t.Fatal("the activity log must never carry secret values")
	}
}

// The Activity tab records what this session wrote; there is no path on a server
// behind it. A panel with nothing in it would be a border spending rows.
func TestCopyPanelIsAbsentWhereThereIsNothingToAddress(t *testing.T) {
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	h.press("4")

	if strings.Contains(h.view(), copyPanelTitle) {
		t.Errorf("the Activity tab has nothing to copy, so it should have no reference panel:\n%s", h.view())
	}
}
