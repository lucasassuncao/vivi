package secrets

import (
	"strings"
	"testing"

	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// An answer for a path the cursor has left describes another secret, and this
// one would describe it as an access verdict.
func TestAStaleCapabilitiesAnswerIsDropped(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	h.m.capabilities = []string{"read"}
	h.run(h.m.Data(capabilitiesMsg{node: &node{name: "elsewhere"}, caps: []string{"root"}}))

	if len(h.m.capabilities) != 1 || h.m.capabilities[0] != "read" {
		t.Errorf("a stale answer overwrote the current one: %v", h.m.capabilities)
	}
}

// "d" takes what the cursor is on, and in the pane that is a field. It used to
// take the whole secret from beside "reveal field" and "copy field", which is
// the one place the word and the act disagreed.
func TestDeleteInTheDetailPaneTakesTheFieldAndNotTheSecret(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")

	keys := h.m.sortedKeys()
	if len(keys) < 2 {
		t.Fatalf("this secret needs two fields to tell the two acts apart: %v", keys)
	}
	gone := keys[0]

	h.press("d")
	if h.modal() == nil {
		t.Fatal("d in the pane did not ask before writing")
	}
	h.press("y")

	// The secret is still there, one field lighter, in a new version.
	data := h.secretData("app/prod/db")
	if _, still := data[gone]; still {
		t.Errorf("%q survived the removal: %v", gone, data)
	}
	if len(data) != len(keys)-1 {
		t.Errorf("removing one field left %d of %d", len(data), len(keys)-1)
	}
	for _, v := range h.versions("app/prod/db") {
		if v.Version == 4 && v.Deleted() {
			t.Fatal("d in the pane deleted the secret's version, not the field")
		}
	}
}

// And the bar says so, because that is where the user reads it first.
func TestTheDetailPaneSaysDeleteTakesTheField(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")

	_, keys := h.m.Legend(h.ctx)
	for _, e := range keys {
		if e.Key != "d" {
			continue
		}
		if e.Action != "delete field" {
			t.Errorf("the pane offers [d] %q, want \"delete field\"", e.Action)
		}
		return
	}
	t.Fatal("the pane stopped offering [d] at all")
}

// "r" is a toggle, so the bar has to name what the next press does. Saying
// "reveal" over a value already on screen offers an action that is not there,
// and the row hint that used to conjugate this is gone from the pane.
func TestRevealLegendNamesTheNextPress(t *testing.T) {
	action := func(h *harness) string {
		_, keys := h.m.Legend(h.ctx)
		for _, e := range keys {
			if e.Key == "r" {
				return e.Action
			}
		}
		return ""
	}

	h := newHarness(t)
	h.open("kv/app/prod/db")

	// The list scope follows the whole secret, and hides only once every field
	// is up - the same condition toggleAll hides on.
	h.ctx.Focus = ui.FocusList
	if got := action(h); got != "reveal secret" {
		t.Errorf("masked, the list offers %q, want \"reveal secret\"", got)
	}
	h.press("r")
	if got := action(h); got != "hide secret" {
		t.Errorf("revealed, the list offers %q, want \"hide secret\"", got)
	}

	// One field masked again is enough: the next press finishes revealing.
	keys := h.m.sortedKeys()
	h.m.revealed[keys[0]] = false
	if got := action(h); got != "reveal secret" {
		t.Errorf("half revealed, the list offers %q, want \"reveal secret\"", got)
	}

	// The pane scope follows the one field under the cursor.
	h.ctx.Focus = ui.FocusDetail
	h.m.fieldCursor = 0
	if got := action(h); got != "reveal field" {
		t.Errorf("masked, the pane offers %q, want \"reveal field\"", got)
	}
	h.m.revealed[keys[0]] = true
	if got := action(h); got != "hide field" {
		t.Errorf("revealed, the pane offers %q, want \"hide field\"", got)
	}
}

// A policy naming the browsing path of a v2 secret grants nothing, silently.
// vivi already resolved the mount's KV version to read it at all, so it is one
// of the few tools able to state the right path without guessing.
func TestTheAccessSectionNamesTheRealPolicyPath(t *testing.T) {
	h := newHarness(t)
	h.resize(120, 60)
	h.open("kv/app/prod/db")

	pane := stripANSI(h.m.renderAccess(h.m.tree.current(), 90))
	for _, want := range []string{
		"ACCESS",
		"kv/data/app/prod/db",     // where a policy has to grant the data
		"kv/metadata/app/prod/db", // and the history
	} {
		if !strings.Contains(pane, want) {
			t.Errorf("the access section does not show %q:\n%s", want, pane)
		}
	}

	// A v1 mount has no data/ segment and no metadata to grant.
	h.open("legacy/old/app")
	v1 := stripANSI(h.m.renderAccess(h.m.tree.current(), 90))
	if strings.Contains(v1, "/data/") || strings.Contains(v1, "metadata path") {
		t.Errorf("a v1 mount was given v2 paths:\n%s", v1)
	}
}

// The capabilities line is the server's own verdict, and it is the only part of
// this section that is a proof rather than an inference.
func TestTheAccessSectionReportsTheServersVerdict(t *testing.T) {
	h := newHarness(t)
	h.resize(120, 60)
	h.open("kv/app/prod/db")

	if h.server.Calls["capabilities"] == 0 {
		t.Fatal("selecting a secret did not ask the server what the token may do")
	}
	pane := stripANSI(h.m.renderAccess(h.m.tree.current(), 90))
	if !strings.Contains(pane, "you may") || !strings.Contains(pane, "read") {
		t.Errorf("the access section does not report the capabilities:\n%s", pane)
	}
}

// Vault's word for "nothing" is a capability called deny, which reads as a
// permission rather than as the absence of every permission.
func TestDenialReadsAsNothingRatherThanAsACapability(t *testing.T) {
	h := newHarness(t)
	h.m.capabilities = []string{"deny"}

	if got := stripANSI(h.m.capabilityLabel()); !strings.Contains(got, "nothing") {
		t.Errorf("capabilityLabel for a denied path = %q", got)
	}
}

// The reverse lookup: which of this token's policies reaches this path, and
// whether it does so by naming it or by covering it with a glob.
func TestTheAccessSectionNamesThePolicyThatGrantsThePath(t *testing.T) {
	h := newHarness(t)
	h.resize(120, 60)
	h.open("kv/app/prod/db")

	if len(h.m.policyDocs) == 0 {
		t.Fatal("the token's own policy documents were never fetched")
	}

	pane := stripANSI(h.m.renderAccess(h.m.tree.current(), 110))
	if !strings.Contains(pane, "granted by") {
		t.Fatalf("the access section does not list what grants the path:\n%s", pane)
	}
	// app-admin is on the fake token and covers kv/data/app/* with a glob.
	if !strings.Contains(pane, "app-admin") {
		t.Errorf("the granting policy is not named:\n%s", pane)
	}
	if !strings.Contains(pane, "glob") {
		t.Errorf("a glob reaching the path should be marked as one:\n%s", pane)
	}
}

// A path none of the readable policies names is a real answer and a common one:
// a policy the token may not read back is invisible here and still enforced.
func TestAPathNoReadablePolicyNamesSaysSo(t *testing.T) {
	h := newHarness(t)
	h.resize(120, 60)
	h.open("kv/platform/ci/token")

	pane := stripANSI(h.m.renderAccess(h.m.tree.current(), 110))
	if !strings.Contains(pane, "none of this token") {
		t.Errorf("an unexplained path should say the lookup found nothing:\n%s", pane)
	}
}

func TestDataSectionSaysWhetherAnythingIsRevealed(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")

	if !strings.Contains(h.view(), "MASKED") {
		t.Fatal("the data section should be badged MASKED while everything is hidden")
	}

	h.press("r")
	view := h.view()
	if !strings.Contains(view, "REVEALED") {
		t.Fatal("revealing a field should change the badge")
	}
	if strings.Contains(view, "MASKED") {
		t.Fatal("the badge should not claim MASKED while a value is on screen")
	}
}

// The badge reports what the block below it holds, so it is read with the
// heading. Pinned to the far edge it ended up a screen away from the word it
// qualifies on a wide pane, which is where it was.
func TestMaskBadgeSitsBesideItsHeading(t *testing.T) {
	h := newHarness(t)
	h.resize(120, 40)
	h.open("kv/app/prod/db")

	for _, line := range strings.Split(h.view(), "\n") {
		if !strings.Contains(line, "DATA") {
			continue
		}
		data := strings.Index(line, "DATA")
		badge := strings.Index(line, "(MASKED)")
		if badge < 0 {
			t.Fatalf("the data heading lost its badge: %q", line)
		}
		// "DATA" plus one space: anything more and it is drifting off again.
		if gap := badge - (data + len("DATA")); gap != 1 {
			t.Errorf("the badge is %d columns from the heading, want 1: %q", gap, line)
		}
		return
	}
	t.Fatal("no data heading on screen")
}

func TestSecretValuesAreMaskedUntilRevealed(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	if strings.Contains(h.view(), "hunter2-v4") {
		t.Fatal("secret value rendered without an explicit reveal")
	}
	if !strings.Contains(h.view(), ui.MaskedValue) {
		t.Fatal("expected masked values")
	}

	// Keys are shown sorted (host, password, username), so the cursor starts
	// on "host"; step once to "password" and reveal it.
	h.press("enter", "down", "r")
	if !strings.Contains(h.view(), "hunter2-v4") {
		t.Fatal("r must reveal the selected field")
	}
}

func TestRevealIsDiscardedWhenMovingToAnotherSecret(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter", "down", "r")

	if !strings.Contains(h.view(), "hunter2-v4") {
		t.Fatal("precondition: value should be revealed")
	}

	h.focus("kv/app/prod/cache")
	h.press("enter")
	h.focus("kv/app/prod/db")
	h.press("enter")

	if strings.Contains(h.view(), "hunter2-v4") {
		t.Fatal("a reveal must not survive navigating away and back")
	}
}

func TestArrowsMoveBetweenFieldsInTheDetailPane(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")

	// Keys render sorted: host, password, username.
	if h.m.fieldCursor != 0 {
		t.Fatalf("field cursor should start at the first key, got %d", h.m.fieldCursor)
	}

	h.press("down", "down")
	if h.m.fieldCursor != 2 {
		t.Fatalf("down should walk the fields, got %d", h.m.fieldCursor)
	}
	h.press("end")
	if h.m.fieldCursor != 2 {
		t.Fatalf("end should land on the last field, got %d", h.m.fieldCursor)
	}
	h.press("home")
	if h.m.fieldCursor != 0 {
		t.Fatalf("home should land on the first field, got %d", h.m.fieldCursor)
	}

	// Past the end it stops rather than wrapping into the scroll.
	h.press("down", "down", "down", "down")
	if h.m.fieldCursor != 2 {
		t.Fatalf("the field cursor ran past the last key: %d", h.m.fieldCursor)
	}
}

// fieldsStartLine is a constant the field cursor scrolls by, so it has to agree
// with where the fields actually land.
func TestFieldsStartWhereTheCursorThinksTheyDo(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	pane := m2lines(h.m.renderSecretDetail(60))
	keys := h.m.sortedKeys()
	if len(keys) == 0 {
		t.Fatal("precondition: the secret should have fields")
	}

	if fieldsStartLine >= len(pane) {
		t.Fatalf("fieldsStartLine = %d but the pane has %d lines", fieldsStartLine, len(pane))
	}
	if !strings.Contains(pane[fieldsStartLine], keys[0]) {
		t.Errorf("line %d is %q, expected the first field %q\nfull pane:\n%s",
			fieldsStartLine, pane[fieldsStartLine], keys[0], strings.Join(pane, "\n"))
	}
}

// The actions reach the focused detail pane, and the footer says so. Requiring a
// trip back to the list before editing the field under the cursor would be
// friction; advertising only the movement keys hides the actions just as well.
func TestActionsWorkAndAreAdvertisedInTheDetailPane(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")

	if h.ctx.Focus != ui.FocusDetail {
		t.Fatal("precondition: the detail pane should be focused")
	}

	for _, tc := range []struct {
		key  string
		want string
	}{
		{"e", "edit"},
		{"d", "confirm"},
		{"v", "versions"},
		{"V", "versions"}, // the old key still opens it
	} {
		h.press(tc.key)
		if h.mode() != tc.want {
			t.Errorf("%q from the detail pane: mode = %q, want %q", tc.key, h.mode(), tc.want)
		}
		h.press("esc")
		if h.mode() == "confirm" {
			h.press("esc") // a discard prompt needs a second one
		}
	}

	h.focus("kv/app/prod/db")
	h.press("enter")
	_, keys := h.m.Legend(h.ctx)
	advertised := map[string]bool{}
	for _, entry := range keys {
		advertised[entry.Key] = true
	}
	// One key each. The copy family used to ride a single pair here, "y/Y/C",
	// read positionally as value, path and command; the last two are printed in
	// the copy panel now and no longer have keys to advertise.
	// "y" is here as well as on the row: it copies a field from the pane and the
	// whole secret from the list, and the footer is where those two words sit
	// side by side across a focus change.
	for _, key := range []string{"e", "d", "v", "r", "y"} {
		if !advertised[key] {
			t.Errorf("the footer does not advertise %q while the pane has focus", key)
		}
	}

	// And the words have to be the ones that distinguish the pane from the list.
	want := map[string]string{"r": "reveal field", "y": "copy field"}
	for _, e := range keys {
		if w, ok := want[e.Key]; ok && e.Action != w {
			t.Errorf("the pane advertises %q as %q, want %q", e.Key, e.Action, w)
		}
	}

	// The rows used to carry "r reveal · y copy" themselves. The bar says it
	// now, for both keys and in both focuses, so the row says it no more.
	for _, hint := range []string{"r reveal", "r hide", "y copy"} {
		if strings.Contains(h.view(), hint) {
			t.Errorf("the selected row still carries the inline hint %q", hint)
		}
	}
}

// Vault's zero for max_versions means unlimited. Printing the zero told the
// reader the opposite of the truth.
func TestMetadataSpellsOutVaultsZeroes(t *testing.T) {
	if got := versionLimit(0); !strings.Contains(got, "every") {
		t.Errorf("versionLimit(0) = %q, want it to say every version is kept", got)
	}
	if got := versionLimit(5); !strings.Contains(got, "5") {
		t.Errorf("versionLimit(5) = %q, want the number", got)
	}
	if got := oldestKept(0); !strings.Contains(got, "v1") {
		t.Errorf("oldestKept(0) = %q, want it to name the oldest surviving version", got)
	}
	if got := oldestKept(3); got != "v3" {
		t.Errorf("oldestKept(3) = %q, want \"v3\"", got)
	}
	if requiredOrNot(true) == requiredOrNot(false) {
		t.Error("check-and-set reads the same whether it is required or not")
	}
}

// The detail pane mixes scopes on purpose - the data is the version's, the
// history and metadata the secret's - so each block is named for what it
// describes and moves, or does not, with the version selector.
func TestTheMetadataBlockDescribesTheSecretNotTheVersion(t *testing.T) {
	h := newHarness(t)
	h.resize(120, 60)
	h.open("kv/app/prod/db")

	current := h.view()
	if !strings.Contains(current, "METADATA") {
		t.Fatalf("the detail pane has no METADATA section:\n%s", current)
	}
	// The header is the only place the version on screen is named now.
	if !strings.Contains(current, "v4 of 4") {
		t.Errorf("the header does not name the version on screen:\n%s", current)
	}
	if strings.Contains(current, "THIS VERSION") {
		t.Errorf("the block that restated the header is still there:\n%s", current)
	}

	h.press("v")
	h.press("down") // v3, the newest that is neither deleted nor destroyed
	h.press("enter")
	historical := h.view()

	// The header follows the selection...
	if !strings.Contains(historical, "v3 of 4") {
		t.Errorf("the header did not follow the selection:\n%s", historical)
	}
	if !strings.Contains(historical, "historical (read-only)") {
		t.Errorf("a historical version should say so:\n%s", historical)
	}
	// ...and the metadata does not: it is the same secret either way, which is
	// the half that was being read as a bug in the version selector.
	if !strings.Contains(historical, "current version  4") {
		t.Errorf("the metadata should still name the secret's current version:\n%s", historical)
	}
}

func TestV1SecretShowsNoVersionHistory(t *testing.T) {
	h := newHarness(t)
	h.open("legacy/old/app")
	h.press("enter")

	if len(h.m.versions) != 0 {
		t.Fatal("a v1 mount has no versions to show")
	}
	view := h.view()
	if strings.Contains(view, "── versions") {
		t.Fatal("the versions section must be hidden on v1")
	}
	if !strings.Contains(view, "no versioning") {
		t.Fatal("the panel should say the mount is unversioned")
	}
}

// Delete is one word for two acts, and which one it is depends on the mount:
// on v1 there is no undo, so it gets the gate destroy gets.
func TestV1DeleteUsesTheStrongConfirmation(t *testing.T) {
	h := newHarness(t)
	h.open("legacy/old/app")

	// From the list, where the cursor is on the secret: in the pane "d" takes
	// the field the cursor is on instead.
	h.press("d")
	if h.modal() == nil || !h.modal().danger {
		t.Fatal("KV v1 has no undo for a delete; it must use the strong gate")
	}
	if !strings.Contains(strings.Join(h.modal().lines, " "), "v1") {
		t.Fatal("the modal should explain why v1 is different")
	}
}

// Once a v1 secret is gone its folder is reloaded, and the folder has to stay
// open with the secret missing from it. expand toggles, and the reload used to
// hit it while still expanded: the folder folded shut instead of refreshing.
func TestDeletingAV1SecretRefreshesItsFolderInPlace(t *testing.T) {
	h := newHarness(t)
	h.open("legacy/old/app")

	h.press("d")
	h.typeText("app")
	h.press("enter")

	if h.modal() != nil {
		t.Fatal("the delete did not run")
	}
	paths := strings.Join(h.visiblePaths(), " ")
	if strings.Contains(paths, "legacy/old/app") {
		t.Fatalf("the deleted secret is still in the tree: %s", paths)
	}
	folder, rest := h.m.tree.find("legacy/old/")
	if folder == nil || rest != "" || !folder.expanded {
		t.Fatalf("the folder must stay open after its listing is refreshed, got %+v", folder)
	}
}

func TestFilterModeShowsAnInputInThePane(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("/")

	if !strings.Contains(h.view(), "/") {
		t.Fatal("filter mode should show its input in the pane")
	}
	h.typeText("cache")
	if !strings.Contains(h.view(), "cache") {
		t.Fatal("the pattern being typed should be visible where the results are")
	}
}

func TestFilterNeverTouchesTheServer(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	before := h.server.Calls["list"]

	h.press("/")
	h.typeText("cache")
	h.press("enter")

	if h.server.Calls["list"] != before {
		t.Fatal("the filter must work on what is already loaded")
	}
	paths := strings.Join(h.visiblePaths(), " ")
	if !strings.Contains(paths, "kv/app/prod/cache") {
		t.Fatalf("filter dropped the match: %v", h.visiblePaths())
	}
	if strings.Contains(paths, "kv/app/prod/db") {
		t.Fatalf("filter kept a non-match: %v", h.visiblePaths())
	}
}

func TestFilterKeepsAncestorsOfAMatch(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	h.press("/")
	h.typeText("cache")

	paths := strings.Join(h.visiblePaths(), " ")
	for _, ancestor := range []string{"kv/", "kv/app", "kv/app/prod"} {
		if !strings.Contains(paths, ancestor) {
			t.Fatalf("ancestor %q was filtered out, leaving the match unreachable: %v",
				ancestor, h.visiblePaths())
		}
	}
}

// The access section is two claims side by side: the server's verdict, and the
// policies explaining it. A policy vivi could not parse makes the second one
// incomplete, and the panel has to say which half is missing.
func TestTheAccessPanelNamesAPolicyItCouldNotRead(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	h.m.policyDocs = map[string]string{
		"reader": `path "kv/data/app/prod/db" { capabilities = ["read"] }`,
		"broken": `path "unclosed" {`,
	}

	view := h.m.renderGrants(h.m.sel, 120)
	if !strings.Contains(view, "reader") {
		t.Errorf("the readable grant is missing:\n%s", view)
	}
	if !strings.Contains(view, "broken") {
		t.Errorf("the panel did not name the policy it could not read:\n%s", view)
	}
}
