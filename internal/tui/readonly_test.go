package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/vault/fake"
)

// The shell's half of read-only: the standing badge, what the palette offers,
// and the fact that a refusal clears itself like any other footer message. The
// gate a write actually passes through is the Secrets tab's, and is tested
// there, because that is the tab that writes.

// readOnlyHarness is newHarness with the session refusing writes.
func readOnlyHarness(t *testing.T, policy app.ReadOnlyPolicy) *harness {
	t.Helper()

	srv := fake.New()
	token, err := srv.TokenInfo(t.Context())
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	m := New(Options{Client: srv, Token: token, Access: app.ReadOnly, ReadOnly: policy})
	clip := &fakeClipboard{}
	m.clip = clip.write
	h := &harness{t: t, m: m, server: srv, clip: clip}

	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 44})
	h.run(m.Init())
	return h
}

// The palette lists what can run. In a read-only session the writes cannot, and
// the standing explanation is the header badge rather than an entry each.
func TestThePaletteHidesWritesInAReadOnlySession(t *testing.T) {
	h := readOnlyHarness(t, app.ReadOnlyAlways)
	h.open("kv/app/prod/db")

	writes := map[string]bool{
		"edit": true, "create": true, "delete": true,
		"rollback": true, "undelete": true, "destroy": true, "delete-metadata": true,
	}
	for _, c := range h.m.available() {
		if writes[c.name] {
			t.Errorf("the palette still offers %q in a read-only session", c.name)
		}
	}

	// And the reads are all still there, which is the point of the mode.
	var reads int
	for _, c := range h.m.available() {
		if c.name == "copy" || c.name == "reveal" || c.name == "versions" {
			reads++
		}
	}
	if reads != 3 {
		t.Errorf("read-only removed reads too: %d of 3 survived", reads)
	}
}

// Without the badge nothing below it is comprehensible: the keys would simply
// have stopped working.
func TestTheHeaderSaysTheSessionIsReadOnly(t *testing.T) {
	h := readOnlyHarness(t, app.ReadOnlyAlways)
	if !strings.Contains(h.m.renderHeader(), "READ-ONLY") {
		t.Errorf("the header does not say the session is read-only:\n%s", h.m.renderHeader())
	}

	rw := newHarness(t)
	if strings.Contains(rw.m.renderHeader(), "READ-ONLY") {
		t.Errorf("an ordinary session should not be badged:\n%s", rw.m.renderHeader())
	}
}

// The legend drops what cannot run; the help panel keeps it and scopes it. Both
// halves matter, and they are easy to get backwards.
func TestTheLegendDropsWritesButTheHelpExplainsThem(t *testing.T) {
	h := readOnlyHarness(t, app.ReadOnlyOnProd)
	h.open("kv/app/prod/db")

	for _, e := range h.m.legend() {
		if e.Writes {
			t.Errorf("the legend still offers %q in a read-only session", e.Key)
		}
	}

	h.press("?")
	help := h.view()
	if !strings.Contains(help, "Read-only session") {
		t.Errorf("the help panel does not say the session is read-only:\n%s", help)
	}
	if !strings.Contains(help, "sandbox") {
		t.Errorf("the help panel does not say why it is read-only:\n%s", help)
	}
	// It is the complete reference: the write keys stay listed under the banner.
	if !strings.Contains(help, "version list") {
		t.Errorf("the help panel stopped being a complete reference:\n%s", help)
	}
}

// A refusal is a transient message like any other, and it has to clear itself
// like one: the key handlers dropped the command that expires it, so the notice
// sat on the footer until something else happened to replace it.
func TestTheRefusalBannerExpires(t *testing.T) {
	h := readOnlyHarness(t, app.ReadOnlyAlways)
	h.open("kv/app/prod/db")
	h.press("enter")

	// press settles the refusal, which arrives as a message from the tab and is
	// what arms the timer.
	h.press("d")
	if h.m.banner == "" {
		t.Fatal("a refused delete said nothing at all")
	}

	// The harness drops timer messages rather than chase them. This is the one
	// test about the timer, so it delivers them itself.
	for _, msg := range deliver(expireBanner(h.m.bannerID)) {
		h.m.Update(msg)
	}
	if h.m.banner != "" {
		t.Fatalf("the refusal never expired: %q", h.m.banner)
	}
}

// deliver runs a command down to the messages it produces, batches included.
func deliver(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}

	var out []tea.Msg
	for _, c := range batch {
		out = append(out, deliver(c)...)
	}
	return out
}

// Renewing is not an Operation and must not be refused: a session that cannot
// write is still a session, and it expires like any other.
func TestRenewingIsNotRefusedInAReadOnlySession(t *testing.T) {
	h := readOnlyHarness(t, app.ReadOnlyAlways)
	h.run(h.m.renewToken())

	if h.server.Calls["renew_token"] == 0 {
		t.Error("a read-only session refused to renew its own token")
	}
}
