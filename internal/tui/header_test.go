package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/vault"
)

func TestHeaderCarriesTheEnvironmentBadge(t *testing.T) {
	h := newHarness(t)

	h.m.server.Address = "https://vault.corp:8200"
	if !strings.Contains(h.m.renderHeader(), "PROD") {
		t.Fatalf("an unrecognised address should warn:\n%s", h.m.renderHeader())
	}

	h.m.server.Address = "http://127.0.0.1:8200"
	if !strings.Contains(h.m.renderHeader(), "LOCAL") {
		t.Fatal("a localhost address should read as LOCAL")
	}
}

// The header must count down rather than repeat the value captured at startup:
// a number that never moves is worse than none, because it gets believed.
func TestHeaderCountsDownFromTheExpiry(t *testing.T) {
	h := newHarness(t)
	// The extra 30s is not slack for a slow machine: shortDuration truncates,
	// so an exact 9m expiry renders as "8m" everywhere the clock is finer than
	// a millisecond.
	h.m.token = &vault.TokenInfo{TTL: time.Hour, ExpiresAt: time.Now().Add(9*time.Minute + 30*time.Second)}

	header := h.m.renderHeader()
	if !strings.Contains(header, "9m") {
		t.Fatalf("expected the remaining time, not the original TTL:\n%s", header)
	}

	h.m.token = &vault.TokenInfo{TTL: time.Hour, ExpiresAt: time.Now().Add(-time.Minute)}
	if !strings.Contains(h.m.renderHeader(), "expired") {
		t.Fatal("an expired token must say so")
	}
}

func TestHeaderShowsWhereWeAreConnected(t *testing.T) {
	h := newHarness(t)
	header := h.m.renderHeader()

	if !strings.Contains(header, "vault.test") {
		t.Fatal("the header must show the server being operated on")
	}
	if !strings.Contains(header, "7h52m") {
		t.Fatal("the header must show how long the token still lives")
	}
}

// ServerInfo declared Version, Sealed and Standby and nothing ever filled them
// in. A browser that cannot say it is reading from a standby leaves the user to
// explain stale data to themselves.
func TestTheHeaderReportsWhatTheServerSaysAboutItself(t *testing.T) {
	srv := newHarness(t).server
	srv.SetServer(vault.ServerInfo{
		Address: "https://vault.corp:8200", Version: "1.21.3", Standby: true,
	})

	token, err := srv.TokenInfo(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{Client: srv, Token: token})
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})

	header := stripANSI(m.renderHeader())
	if !strings.Contains(header, "STANDBY") {
		t.Errorf("the header does not say the node is a standby:\n%s", header)
	}
	if !strings.Contains(header, "v1.21.3") {
		t.Errorf("the header does not name the server version:\n%s", header)
	}
}

func TestASealedServerIsBadged(t *testing.T) {
	srv := newHarness(t).server
	srv.SetServer(vault.ServerInfo{Address: "https://vault.corp:8200", Sealed: true})

	token, err := srv.TokenInfo(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{Client: srv, Token: token})
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})

	if !strings.Contains(stripANSI(m.renderHeader()), "SEALED") {
		t.Errorf("a sealed server should be badged:\n%s", stripANSI(m.renderHeader()))
	}
}

// The header is priority-ordered because a joined line loses its right end. It
// The one-line header ended with the countdown behind a thirty-column address,
// so eighty columns dropped the session's remaining time and kept a constant
// address. Three lines end that trade: the address owns one, and nothing it
// pushes out is on another line.
func TestTheHeaderKeepsEveryFactAtEightyColumns(t *testing.T) {
	h := newHarness(t)
	h.m.server.Address = "https://vault.production.internal.example.com:8200"
	h.m.token = &vault.TokenInfo{TTL: time.Hour, ExpiresAt: time.Now().Add(9*time.Minute + 30*time.Second)}
	h.m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	header := stripANSI(h.m.renderHeader())
	for _, want := range []string{"9m", "PROD", "vault.production.internal.example.com"} {
		if !strings.Contains(header, want) {
			t.Errorf("eighty columns dropped %q:\n%s", want, header)
		}
	}
	if lineWidth(h.m.renderHeader()) > 80 {
		t.Errorf("the header is %d columns wide in an eighty-column terminal:\n%s",
			lineWidth(h.m.renderHeader()), header)
	}
}
