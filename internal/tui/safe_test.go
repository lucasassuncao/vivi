package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// payloads are the sequences a hostile or careless value could carry. Each one
// does something to the terminal rather than putting a glyph on it.
var payloads = map[string]string{
	"clipboard hijack": "\x1b]52;c;aGVsbG8=\x07",
	"clear screen":     "\x1b[2J",
	"window title":     "\x1b]0;pwned\x07",
	"cursor home":      "\x1b[H",
	"carriage return":  "\rOVERWRITTEN",
	// Written as an escape, not as the character: a source file with a raw
	// RIGHT-TO-LEFT OVERRIDE in it has the same reading problem this test is
	// about, and staticcheck says so (ST1018).
	"bidi override": "\u202e",
	"nul":           "\x00",
}

// assertNoPayload fails if any payload survived into a rendered frame.
func assertNoPayload(t *testing.T, where, view string) {
	t.Helper()
	for name, p := range payloads {
		if strings.Contains(view, p) {
			t.Errorf("%s: the %s payload reached the screen", where, name)
		}
	}
}

func allPayloads() string {
	var b strings.Builder
	for _, p := range payloads {
		b.WriteString(p)
	}
	return b.String()
}

// A revealed value is the one place a secret's own bytes are drawn, so it is
// the one place this matters most.
func TestRevealedValueCannotDriveTheTerminal(t *testing.T) {
	h := newHarness(t)
	h.server.PutVersions("kv", "hostile", []map[string]any{{
		"payload":       allPayloads(),
		allPayloads():   "value under a hostile key",
		"clipboard":     payloads["clipboard hijack"],
		"bidirectional": "capabilities = [\u202e\"sudo\"\u202c]",
	}}, time.Now())
	h.run(h.m.Init())
	h.open("kv/hostile")
	h.press("enter")

	// Reveal every field in turn: only the selected one is drawn unmasked.
	for range h.m.secretsTab.FieldKeys() {
		h.press("r")
		assertNoPayload(t, "revealed value", h.view())
		h.press("r")
		h.press("down")
	}
}

// The masked view draws the keys, which are server-supplied too.
func TestSecretKeysCannotDriveTheTerminal(t *testing.T) {
	h := newHarness(t)
	h.server.PutVersions("kv", "hostile", []map[string]any{{
		allPayloads(): "x",
	}}, time.Now())
	h.run(h.m.Init())
	h.open("kv/hostile")

	assertNoPayload(t, "masked field list", h.view())
}

// A policy is where a bidirectional override would pay off best: the reason to
// open one is to believe what it says about who may read what.
func TestPolicyDocumentCannotDriveTheTerminal(t *testing.T) {
	h := newHarness(t)
	h.server.SetPolicy("hostile", "path \"kv/*\" {\n  capabilities = [\u202e\"sudo\"\u202c]\n}\n"+allPayloads())
	h.run(h.m.Init())

	h.press("2")
	for range h.m.policiesTab.Names() {
		assertNoPayload(t, "policy document", h.view())
		h.press("down")
	}
}

// A policy is legitimately multi-line, and neutralising it must not flatten it
// into one row: a document nobody can read is its own kind of failure.
func TestPolicyKeepsItsLineBreaks(t *testing.T) {
	lines := ui.SanitizeLines("path \"kv/*\" {\n  capabilities = [\"read\"]\n}")
	if len(lines) != 3 {
		t.Fatalf("a three-line policy became %d lines: %q", len(lines), lines)
	}
	if strings.Contains(strings.Join(lines, "\n"), `\n`) {
		t.Errorf("line breaks were escaped instead of kept: %q", lines)
	}
}

// A Windows-authored document must not read as one "\r" per line.
func TestSanitizeLinesTreatsCRLFAsALineEnding(t *testing.T) {
	lines := ui.SanitizeLines("a\r\nb\r\n")
	for _, line := range lines {
		if strings.Contains(line, `\r`) {
			t.Errorf("a CRLF line ending was drawn as content: %q", lines)
		}
	}
}

// Node names come from the server as well, and the tree draws them before
// anything has been selected.
func TestTreeNodeNamesCannotDriveTheTerminal(t *testing.T) {
	h := newHarness(t)
	h.server.PutVersions("kv", "safe/"+allPayloads(), []map[string]any{{"a": "b"}}, time.Now())
	h.run(h.m.Init())
	h.open("kv/safe")

	assertNoPayload(t, "tree", h.view())
}

// The banner is the one line that survives whatever pane is on screen, and what
// it reports is a path or an operation the server named.
func TestTheFooterBannerCannotDriveTheTerminal(t *testing.T) {
	h := newHarness(t)
	hostile := "safe/" + allPayloads()
	h.server.PutVersions("kv", hostile, []map[string]any{{"a": "b"}}, time.Now())
	h.run(h.m.Init())

	h.open("kv/" + hostile)
	h.press("enter")
	h.press("d", "y") // the footer reports the delete by path

	if h.m.banner == "" {
		t.Fatal("the delete reported nothing, so this proves nothing about the banner")
	}
	assertNoPayload(t, "banner", h.view())
}

// The display name and the policy names in the token panel are chosen by
// whoever configured the auth method, not by the person reading them.
func TestTheTokenPanelCannotDriveTheTerminal(t *testing.T) {
	h := newHarness(t)
	// One payload per row rather than all of them at once: a line long enough
	// to be truncated loses its escapes on the way, which proves nothing. It
	// goes through the server because opening the panel re-reads the token.
	token := &vault.TokenInfo{
		DisplayName: payloads["clear screen"],
		Accessor:    payloads["clipboard hijack"],
		EntityID:    payloads["window title"],
		TTL:         time.Hour,
	}
	for _, p := range payloads {
		token.Policies = append(token.Policies, p)
	}
	h.server.SetToken(token)

	h.press("i")
	if h.mode() != "token" {
		t.Fatalf("i should open the token panel, mode=%s", h.mode())
	}

	assertNoPayload(t, "token panel", h.view())
}

// Control characters are shown, not dropped: "a\x00b" and "ab" are different
// secrets and must not look alike.
func TestSanitizeShowsWhatItNeutralises(t *testing.T) {
	got := ui.Sanitize("a\x00b")
	if got == "ab" {
		t.Fatal("a control character was dropped, so two different values now look the same")
	}
	if !strings.Contains(got, "a") || !strings.Contains(got, "b") {
		t.Errorf("the printable text did not survive: %q", got)
	}
}

// Text with nothing to neutralise comes back identical, which is what keeps the
// common case free.
func TestSanitizeLeavesOrdinaryTextAlone(t *testing.T) {
	for _, s := range []string{"", "hunter2", "kv/app/prod/db", "ação ünïcode 日本語", "a b c"} {
		if got := ui.Sanitize(s); got != s {
			t.Errorf("sanitize(%q) = %q, want it unchanged", s, got)
		}
	}
}

// The editor round-trip is proved beside the editor, in the secrets package:
// it needs the editor itself, and the rule it proves is the same one.

// Every tab draws something the server chose. This walks all four at a size
// where both panes are up, as a backstop for a render path the tests above miss.
func TestNoTabDrawsAPayload(t *testing.T) {
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	h.server.PutVersions("kv", "hostile", []map[string]any{{allPayloads(): allPayloads()}}, time.Now())
	h.server.SetPolicy("hostile", allPayloads())
	h.run(h.m.Init())

	for _, tabKey := range []string{"1", "2", "3", "4"} {
		h.press(tabKey)
		assertNoPayload(t, "tab "+tabKey, h.view())
	}
}

// The fuzz that proves sanitize itself is safe to draw lives beside sanitize,
// in ui: it reads unsafeToDraw, which is that package's own business.
