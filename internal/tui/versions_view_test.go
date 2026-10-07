package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Only the list in front of the user wears a cursor: opening the version list
// takes it from the fields.
func TestTheVersionListTakesTheCursorFromTheFields(t *testing.T) {
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	h.open("kv/app/prod/db")
	h.press("V")
	if n := strings.Count(stripANSI(h.m.detailView()), "› "); n != 1 {
		t.Errorf("the pane shows %d cursors, want 1:\n%s", n, stripANSI(h.m.detailView()))
	}
}

// On a short terminal the pane follows the version cursor down the list.
func TestTheDetailPaneFollowsTheVersionCursor(t *testing.T) {
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: minTerminalHeight})
	h.open("kv/app/prod/db")
	h.press("V")
	h.press("end")
	if !strings.Contains(stripANSI(h.m.detailView()), "› [") {
		t.Errorf("the version cursor scrolled out of the pane:\n%s", stripANSI(h.m.detailView()))
	}
	h.press("home")
	if !strings.Contains(stripANSI(h.m.detailView()), "› [") {
		t.Errorf("the version cursor scrolled out of the pane going back up:\n%s", stripANSI(h.m.detailView()))
	}
}
