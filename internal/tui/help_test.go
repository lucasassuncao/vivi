package tui

import (
	"strings"
	"testing"
)

// The ? panel is the route to what the footer drops, so it has to be about the
// tab you are on. One list spanning every tab is a list nobody reads.
func TestHelpIsContextualPerTab(t *testing.T) {
	h := newHarness(t)

	secrets := h.m.renderHelp(40)
	if !strings.Contains(secrets, "reveal") {
		t.Error("the secrets help does not mention reveal")
	}
	if strings.Contains(secrets, "diff two marked versions") {
		t.Error("the secrets help leaks the version-list keys")
	}

	h.press("2") // Policies
	policies := h.m.renderHelp(40)
	if strings.Contains(policies, "reveal") {
		t.Error("the policies help leaks the secrets keys")
	}
	if !strings.Contains(policies, "copy the HCL") {
		t.Error("the policies help does not mention copying the HCL")
	}
}

// Meta keys live nowhere else: the footer never shows them, by design.
func TestHelpCarriesTheKeysTheFooterDrops(t *testing.T) {
	h := newHarness(t)

	help := h.m.renderHelp(40)
	for _, want := range []string{"token information", "reload what this tab shows", "switch tab"} {
		if !strings.Contains(help, want) {
			t.Errorf("the help panel does not carry %q", want)
		}
	}
}
