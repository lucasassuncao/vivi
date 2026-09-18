package tui

import "testing"

func TestAuthTabJumpsToTheGrantedPolicy(t *testing.T) {
	h := newHarness(t)
	h.press("2") // load policies first so the jump can resolve
	h.press("3")
	h.press("right") // focus roles
	h.press("p")

	if h.m.tab != tabPolicies {
		t.Fatal("p should jump to the Policies tab")
	}
	if h.m.policiesTab.Names()[h.m.policiesTab.Cursor()] != "app-admin" {
		t.Fatalf("jumped to the wrong policy: %s", h.m.policiesTab.Names()[h.m.policiesTab.Cursor()])
	}
}

// The link the Auth tab shows had a key and no name.
func TestFollowingARolePolicyHasACommand(t *testing.T) {
	h := newHarness(t)
	h.runCommand("tab auth")
	h.press("right") // into the role list

	if !offers(h, "role-policy") {
		t.Fatal("the auth tab offers no way to follow the role's policy by name")
	}
	h.runCommand("role-policy")
	if h.m.tab != tabPolicies {
		t.Errorf("role-policy did not follow the link: on tab %v", h.m.tab)
	}
}

// The shrinking-list case moved to the auth package with the clamp it pins.
