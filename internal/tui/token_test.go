package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/lucasassuncao/vivi/internal/vault"
)

func TestTokenPanelShowsWhoWeAre(t *testing.T) {
	h := newHarness(t)
	h.press("i")

	view := h.view()
	for _, want := range []string{"Token", "userpass-lucas", "app-admin"} {
		if !strings.Contains(view, want) {
			t.Fatalf("token panel missing %q", want)
		}
	}
}

// A session that expires mid-task and forces a restart is exactly the friction
// that ends with the web UI open, which is the one thing this app exists to
// avoid. Renewal has to be reachable from where the expiry is visible.
func TestRenewFromTheTokenPanel(t *testing.T) {
	h := newHarness(t)

	h.press("i")
	if !strings.Contains(h.view(), "renew") {
		t.Fatal("the token panel does not offer renewal")
	}

	before := h.m.token.ExpiresAt
	h.press("n")

	if !h.m.token.ExpiresAt.After(before) {
		t.Errorf("renew did not move the expiry: %v then %v", before, h.m.token.ExpiresAt)
	}
	if !strings.Contains(h.m.banner(), "token renewed") {
		t.Error("renewal gave no feedback")
	}
}

// The panel stays a panel: the keys that closed it before still close it.
func TestTokenPanelStillCloses(t *testing.T) {
	h := newHarness(t)

	h.press("i")
	if h.mode() != "token" {
		t.Fatalf("i did not open the token panel: mode = %q", h.mode())
	}
	h.press("esc")
	if h.mode() != "browse" {
		t.Errorf("esc did not close the token panel: mode = %q", h.mode())
	}
}

// A token that cannot be renewed says so instead of failing against the server.
// It is replaced on the fake and not the model, because "i" re-reads it:
// mutating m.token here would be undone before the key is pressed.
func TestRenewRefusedForANonRenewableToken(t *testing.T) {
	h := newHarness(t)

	h.server.SetToken(&vault.TokenInfo{
		DisplayName: "static",
		TTL:         time.Hour,
		Renewable:   false,
	})

	h.press("i")
	if strings.Contains(h.view(), "renew this token") {
		t.Error("the panel offers renewal for a token that cannot be renewed")
	}

	h.press("n")
	if !strings.Contains(h.m.banner(), "not renewable") {
		t.Error("a non-renewable token got no explanation")
	}
}

func TestShortDurationKeepsTheTwoLargestUnits(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0: "0s", 42 * time.Second: "42s", 5*time.Minute + 30*time.Second: "5m",
		3*time.Hour + 12*time.Minute: "3h12m", 2 * time.Hour: "2h",
		50*time.Hour + 59*time.Minute: "2d2h", 48 * time.Hour: "2d",
	} {
		if got := shortDuration(d); got != want {
			t.Errorf("shortDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
