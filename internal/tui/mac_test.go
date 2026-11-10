package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// A Mac's terminal scrolls its own window on PgUp and PgDn, so there the pane
// takes ctrl+d and ctrl+u for half a page.
func TestOnAMacCtrlDAndCtrlUScrollTheDetailPane(t *testing.T) {
	h := macHarness(t)
	long := strings.Repeat("path \"sys/whatever\" {\n  capabilities = [\"read\"]\n}\n\n", 30)
	h.m.policiesTab.Restore([]string{"default"}, 0, long)
	h.m.tab = tabPolicies
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	h.view()

	h.press("right")
	before := h.m.detail.YOffset()
	h.press("ctrl+d")
	if h.m.detail.YOffset() <= before {
		t.Fatal("ctrl+d should scroll the detail pane down")
	}
	h.press("ctrl+u")
	if got := h.m.detail.YOffset(); got != before {
		t.Fatalf("ctrl+u should scroll back to offset %d, got %d", before, got)
	}
}

// The footer offers what a Mac can press: PgUp and PgDn never reach vivi there,
// so naming them is naming keys that do nothing.
func TestOnAMacTheFooterOffersCtrlUAndCtrlDForTheDocument(t *testing.T) {
	for _, tc := range []struct {
		name string
		walk func(*harness)
		want string
	}{
		{"a secret's fields", func(h *harness) { h.open("kv/app/prod/db"); h.press("enter") }, "[ctrl+u/d] scroll"},
		{"a policy document", func(h *harness) { h.press("2", "enter") }, "[ctrl+u/d] half a page"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := macHarness(t)
			tc.walk(h)
			footer := stripANSI(strings.Join(h.m.legendLines(200), "\n"))
			if !strings.Contains(footer, tc.want) {
				t.Errorf("the footer does not offer %q:\n%s", tc.want, footer)
			}
			if strings.Contains(footer, "pgup") {
				t.Errorf("the footer still offers pgup on a Mac:\n%s", footer)
			}
		})
	}
}

// The ? panel is the full reference, so on a Mac it names the keys that work
// there, and only those.
func TestOnAMacTheHelpNamesTheKeysAMacCanPress(t *testing.T) {
	for _, tc := range []struct {
		name      string
		walk      func(*harness)
		want, not string
	}{
		{"anywhere", func(*harness) {}, "g / G", "home / end"},
		{"a secret's fields", func(h *harness) { h.open("kv/app/prod/db"); h.press("enter") }, "ctrl+u / ctrl+d", "pgup"},
		{"a policy document", func(h *harness) { h.press("2", "enter") }, "ctrl+u / ctrl+d", "pgup"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := macHarness(t)
			tc.walk(h)
			help := stripANSI(h.m.renderHelp(40))
			if !strings.Contains(help, tc.want) {
				t.Errorf("the help does not name %q:\n%s", tc.want, help)
			}
			if strings.Contains(help, tc.not) {
				t.Errorf("the help still names %q on a Mac:\n%s", tc.not, help)
			}
		})
	}
}
