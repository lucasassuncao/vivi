package tui

import (
	"strings"
	"testing"

	"github.com/lucasassuncao/bezel/bezeltest"
)

func TestPoliciesAndAuthTabsLoadOnDemand(t *testing.T) {
	h := newHarness(t)
	if h.server.Calls["policies"] != 0 {
		t.Fatal("policies loaded before the tab was opened")
	}

	h.press("2")
	if h.server.Calls["policies"] != 1 {
		t.Fatal("opening the Policies tab should load them once")
	}
	if !strings.Contains(h.view(), "app-prod-db") {
		t.Fatal("policy list not rendered")
	}

	h.press("3")
	if h.server.Calls["auth_methods"] != 1 {
		t.Fatal("opening the Auth tab should load the methods")
	}
	if !strings.Contains(h.view(), "approle/") {
		t.Fatal("auth methods not rendered")
	}
}

// The legend and the "?" panel print "R reload" on these tabs, and the two read
// once and never again without it. The key reached only the secrets tree, so
// what they promised was answered by ":reload" alone.
func TestReloadKeyRefetchesEveryTab(t *testing.T) {
	h := newHarness(t)

	h.press("2")
	h.press("R")
	if got := h.server.Calls["policies"]; got != 2 {
		t.Errorf("R on the Policies tab: %d fetches, want 2", got)
	}

	h.press("3")
	h.press("R")
	if got := h.server.Calls["auth_methods"]; got != 2 {
		t.Errorf("R on the Auth tab: %d fetches, want 2", got)
	}
}

// "p" on a role jumps to the policy it grants, and can fire before the Policies
// tab has opened. The jump used to be dropped then: the tab opened on whichever
// policy sorted first, which reads as the jump landing somewhere strange.
func TestJumpToPolicyWaitsForTheListItNeeds(t *testing.T) {
	h := newHarness(t)
	if h.m.policiesTab.Names() != nil {
		t.Fatal("precondition: the policy list should not be loaded yet")
	}

	h.run(h.m.jumpToPolicy("app-prod-db"))

	if h.m.tab != tabPolicies {
		t.Fatalf("the jump should switch tabs, got %v", h.m.tab)
	}
	if got := h.m.policiesTab.Names()[h.m.policiesTab.Cursor()]; got != "app-prod-db" {
		t.Errorf("landed on %q, want app-prod-db (list: %v)", got, h.m.policiesTab.Names())
	}
	if !strings.Contains(h.m.policiesTab.Doc(), "app/prod/db") {
		t.Errorf("the pane shows another policy's document: %q", h.m.policiesTab.Doc())
	}
}

// A jump to something the server does not have says so, rather than silently
// leaving the cursor wherever it was.
func TestJumpToAnUnknownPolicySaysSo(t *testing.T) {
	h := newHarness(t)
	h.run(h.m.jumpToPolicy("no-such-policy"))

	if !strings.Contains(h.m.banner(), "not in the list") {
		t.Errorf("expected the jump to report the miss, banner=%q", h.m.banner())
	}
}

// The shrinking-list case moved to the policies package with the clamp it
// pins: the answer it feeds in is that tab's own message.

// Reloading the Policies tab drops the list and asks for it again. The cursor
// has to come with it, or the answer arrives against a cursor that outlived the
// list it was pointing into.
func TestReloadingPoliciesResetsTheCursor(t *testing.T) {
	h := newHarness(t)
	h.runCommand("tab policies")
	h.press("down", "down")

	if h.m.policiesTab.Cursor() == 0 {
		t.Fatal("the cursor did not move, so the test proves nothing")
	}
	h.runCommand("reload")

	if h.m.policiesTab.Cursor() >= len(h.m.policiesTab.Names()) {
		t.Errorf("after a reload the cursor is off the end: %d of %d",
			h.m.policiesTab.Cursor(), len(h.m.policiesTab.Names()))
	}
}

// R on the Policies and Auth tabs was a key that did nothing, on the two tabs
// most likely to be stale: both are read once when the tab first opens.
func TestReloadRefetchesWhicheverTabIsShowing(t *testing.T) {
	h := newHarness(t)
	h.runCommand("tab policies")

	before := h.server.Calls["policies"]
	h.runCommand("reload")
	if h.server.Calls["policies"] <= before {
		t.Errorf("reload on the Policies tab did not refetch: %d calls, was %d",
			h.server.Calls["policies"], before)
	}
}

// Copying the policy was a key with no name anywhere. The secrets tab's line
// is ":copy-policy-path", so the bare name stays free for this one.
func TestThePolicyTabCanCopyByName(t *testing.T) {
	h := newHarness(t)
	h.runCommand("tab policies")

	if !offers(h, "copy-policy") {
		t.Fatal("the policies tab offers no copy command")
	}
	// ":copy" is the secrets tab's and refused here. Its siblings on this tab
	// start with it, so the palette stays open on them rather than guessing.
	h.runCommand("copy")
	if h.mode() != "command" || !strings.Contains(h.view(), "copy is not available here") {
		t.Errorf(`"copy" ran or closed with its siblings on the tab: mode %q`, h.mode())
	}
	h.press("esc")

	// The exact name still wins, which is the first rule of resolution.
	h.runCommand("copy-policy")
	if h.mode() != "browse" || h.copied() == "" {
		t.Errorf(`"copy-policy" did not run: mode %q, banner %q`, h.mode(), h.m.banner())
	}
}

// The panel title follows the cursor at once while the document arrives as a
// message, so between the two the pane paired one policy's name with another's
// rules, which is the one thing a policy reader cannot afford to misread.
func TestMovingThePolicyCursorDropsThePreviousDocument(t *testing.T) {
	h := newHarness(t)
	h.runCommand("tab policies")

	previous := h.m.policiesTab.Doc()
	if previous == "" {
		t.Fatal("the first policy never loaded, so this proves nothing")
	}

	// Update and not press: what is being tested is the frame drawn before the
	// answer, which the harness would otherwise settle away.
	_, cmd := h.m.Update(bezeltest.Key("down"))
	if h.m.policiesTab.Doc() == previous {
		t.Fatal("the previous document is still in the pane, under the new name")
	}
	if strings.Contains(h.view(), "kv/data/app/prod/db") {
		t.Fatalf("the previous policy is still on screen:\n%s", h.view())
	}

	h.run(cmd)
	if h.m.policiesTab.Doc() == "" {
		t.Fatal("the policy under the cursor never arrived")
	}
}
