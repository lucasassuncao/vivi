package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// The detail's title is the shown secret's path as a breadcrumb, and a click
// on a part goes up to that folder, as :goto would.
func TestTheDetailTitleIsAPathThatGoesUp(t *testing.T) {
	h := newHarness(t)
	h.runCommand("goto kv/app/prod/db")
	if !strings.Contains(h.view(), "kv › app › prod › db") {
		t.Fatalf("the title is no breadcrumb:\n%s", h.view())
	}

	x, y := h.at("app › prod", 0)
	_, cmd := h.m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	h.run(cmd)
	if got := h.m.secretsTab.CursorPath(); got != "kv/app" {
		t.Errorf("a click on app left the cursor on %q", got)
	}
	if !strings.Contains(h.view(), "kv › app ") {
		t.Errorf("the title does not follow the cursor up:\n%s", h.view())
	}
}

// A click on the edge past the title, or on another tab's plain title, does
// nothing to the cursor.
func TestAClickBesideTheTitleGoesNowhere(t *testing.T) {
	h := newHarness(t)
	h.runCommand("goto kv/app/prod/db")
	before := h.m.secretsTab.CursorPath()
	_, y := h.at("kv › app › prod › db", 0)
	r := h.m.sh.Rect(paneDetail)
	_, cmd := h.m.Update(tea.MouseClickMsg{X: r.X + r.W - 3, Y: y, Button: tea.MouseLeft})
	h.run(cmd)
	if got := h.m.secretsTab.CursorPath(); got != before {
		t.Errorf("a click past the title moved the cursor to %q", got)
	}
}
