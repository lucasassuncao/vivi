package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// at is the screen cell where text first starts on the drawn screen, at or
// below row from: the place a user would put the pointer to click it.
func (h *harness) at(text string, from int) (x, y int) {
	h.t.Helper()
	for y, line := range strings.Split(h.view(), "\n") {
		if i := strings.Index(line, text); y >= from && i >= 0 {
			return ansi.StringWidth(line[:i]), y
		}
	}
	h.t.Fatalf("%q is not on screen:\n%s", text, h.view())
	return 0, 0
}

// click is a left click on the first cell of text, and what it set going.
func (h *harness) click(text string, from int) {
	h.t.Helper()
	x, y := h.at(text, from)
	_, cmd := h.m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	h.run(cmd)
}

// The screen asks the terminal for the mouse, or none of the rest reaches it.
func TestTheViewTurnsTheMouseOn(t *testing.T) {
	h := newHarness(t)
	if h.m.View().MouseMode != tea.MouseModeCellMotion {
		t.Error("the view does not ask for mouse reporting")
	}
}

// A tab on the strip is the same as its number key.
func TestClickingATabSwitchesToIt(t *testing.T) {
	h := newHarness(t)
	h.click("Policies", len(h.m.headerLines()))
	if h.m.tab != tabPolicies {
		t.Errorf("clicking Policies left the tab at %v", h.m.tab)
	}
}

// A row takes the cursor; the row already under it does what enter does.
func TestClickingATreeRowSelectsItThenOpensIt(t *testing.T) {
	h := newHarness(t)
	body := h.m.sh.Body().Y

	h.click("▸ legacy/", body)
	if got := h.m.secretsTab.CursorPath(); got != "legacy/" {
		t.Fatalf("the click left the cursor on %q", got)
	}
	before := len(h.visiblePaths())
	h.click("▸ legacy/", body)
	if len(h.visiblePaths()) <= before {
		t.Errorf("a second click on the selected folder did not open it: %v", h.visiblePaths())
	}
}

// In the detail pane a click selects the field, which r and y then act on, and
// gives the pane the keys.
func TestClickingAFieldSelectsIt(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	h.click("password", h.m.sh.Body().Y)
	if h.m.focus != ui.FocusDetail {
		t.Error("clicking the detail did not give it the keys")
	}
	if !strings.Contains(h.view(), "-field=password") {
		t.Errorf("the clicked field is not the selected one:\n%s", h.view())
	}
}

// Mouse reporting takes the terminal's own selection, so a Command Reference
// row copies itself when clicked, as its :copy command does.
func TestClickingAReferenceRowCopiesIt(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	h.click("policy-path", h.m.sh.Body().Y)
	if got := h.copied(); got != "kv/data/app/prod/db" {
		t.Errorf("clicking the policy-path row copied %q", got)
	}
}

// The wheel scrolls the pane under the pointer, and moves the list's cursor
// as the arrows do.
func TestTheWheelScrollsThePaneUnderIt(t *testing.T) {
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	h.open("kv/app/prod/db")

	x, y := h.at("DATA", h.m.sh.Body().Y)
	h.m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown})
	if h.m.detail.YOffset() == 0 {
		t.Error("the wheel did not scroll the detail")
	}

	before := h.m.secretsTab.CursorPath()
	list := h.m.sh.Rect(paneList)
	_, cmd := h.m.Update(tea.MouseWheelMsg{X: list.X + 2, Y: list.Y + 2, Button: tea.MouseWheelDown})
	h.run(cmd)
	if h.m.secretsTab.CursorPath() == before {
		t.Errorf("the wheel over the list left the cursor on %q", before)
	}
}

// A modal of the tab's takes the keyboard, and the mouse stands aside with it.
func TestTheMouseStandsAsideForAModal(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("d")
	mode := h.mode()

	h.click("Policies", len(h.m.headerLines()))
	if h.m.tab != tabSecrets || h.mode() != mode {
		t.Errorf("a click under the confirmation acted: tab %v, mode %q", h.m.tab, h.mode())
	}
}

// vivi has the mouse, so the terminal selects only with shift held, and the
// panel worth selecting from says so on its edge.
func TestTheReferenceSaysToHoldShift(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	if !strings.Contains(h.view(), copyPanelHint) {
		t.Errorf("the Command Reference does not say how to select:\n%s", h.view())
	}
}

// --no-mouse leaves the mouse to the terminal, and with it nothing says to
// hold shift: the terminal selects as it always does.
func TestNoMouseLeavesTheMouseToTheTerminal(t *testing.T) {
	h := newHarness(t)
	h.m.noMouse = true
	h.open("kv/app/prod/db")

	if h.m.View().MouseMode != tea.MouseModeNone {
		t.Error("the view still asks for mouse reporting")
	}
	if strings.Contains(h.view(), copyPanelHint) {
		t.Errorf("the Command Reference still says to hold shift:\n%s", h.view())
	}
}
