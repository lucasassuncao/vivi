package secrets

import (
	"strings"
	"testing"
	"time"

	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
	"github.com/lucasassuncao/vivi/internal/vault/fake"

	"charm.land/lipgloss/v2"
)

// The cursor may never rest on a header or a spacer: every key that acts on "the
// current node" would find nothing there. These rows are built by hand so the
// rules can be checked before grouping produces any.
func TestCursorSkipsRowsThatAreNotNodes(t *testing.T) {
	a := &node{kind: kindMount, name: "a", mount: "a"}
	b := &node{kind: kindMount, name: "b", mount: "b"}

	tr := &tree{visible: []row{
		{kind: rowHeader, header: "FIRST"},
		{kind: rowNode, node: a},
		{kind: rowSpacer},
		{kind: rowHeader, header: "SECOND"},
		{kind: rowNode, node: b},
	}}

	tr.moveTo(0)
	if tr.current() != a {
		t.Fatal("home should skip the leading header and land on the first node")
	}

	tr.move(1)
	if tr.current() != b {
		t.Fatal("down should cross the spacer and the header in one step")
	}

	tr.move(1)
	if tr.current() != b {
		t.Fatal("down at the end must stay on the last node")
	}

	tr.moveTo(len(tr.visible) - 1)
	if tr.current() != b {
		t.Fatal("end should land on the last node")
	}

	tr.move(-5)
	if tr.current() != a {
		t.Fatal("up past the top must stop on the first node")
	}
}

// One section separates nothing from nothing. With a single group the header is
// pure furniture, and the engine goes back onto the mount's own row.
func TestSingleGroupRendersNoHeader(t *testing.T) {
	tr := &tree{}
	tr.setMounts([]vault.Mount{
		{Path: "kv/", Type: "kv", KVVersion: vault.KV2},
		{Path: "other/", Type: "kv", KVVersion: vault.KV2},
	})

	if tr.grouped {
		t.Fatal("a single section must not be grouped")
	}
	for i, r := range tr.visible {
		if r.kind != rowNode {
			t.Fatalf("row %d is furniture in a single-section tree: %+v", i, r)
		}
	}

	tr.setMounts([]vault.Mount{
		{Path: "kv/", Type: "kv", KVVersion: vault.KV2},
		{Path: "legacy/", Type: "kv", KVVersion: vault.KV1},
	})
	if !tr.grouped {
		t.Fatal("two sections must be grouped")
	}
}

func TestOpeningListsMountsAndNothingElse(t *testing.T) {
	h := newHarness(t)

	if got := len(h.visiblePaths()); got != 3 {
		t.Fatalf("expected the three mounts, got %d: %v", got, h.visiblePaths())
	}
	// Lazy loading is the whole reason a large Vault opens instantly.
	if n := h.server.Calls["list"]; n != 0 {
		t.Fatalf("expected no listing before an expansion, got %d", n)
	}
}

func TestExpandingListsOnceAndCachesUntilReload(t *testing.T) {
	h := newHarness(t)

	h.focus("kv/")
	h.press("right")
	if h.server.Calls["list"] != 1 {
		t.Fatalf("expected one listing, got %d", h.server.Calls["list"])
	}

	h.press("left")  // collapse
	h.press("right") // expand again, from cache
	if h.server.Calls["list"] != 1 {
		t.Fatalf("re-expanding must not hit the server, got %d listings", h.server.Calls["list"])
	}

	h.focus("kv/")
	// "R" is the shell's key and it calls this, which is what the tab owns.
	h.run(h.m.Reload())
	if h.server.Calls["list"] != 2 {
		t.Fatalf("R must refetch, got %d listings", h.server.Calls["list"])
	}
}

func TestDeniedPathStaysVisible(t *testing.T) {
	h := newHarness(t)

	h.focus("kv/")
	h.press("right")
	n := h.focus("kv/restricted")
	h.press("right")

	if !n.denied {
		t.Fatal("a path the token cannot list must be marked denied")
	}
	if !strings.Contains(h.view(), "[denied]") {
		t.Fatal("the tree must say the path is denied instead of hiding it")
	}
	// The node is still there: dropping it would understate the Vault.
	if !strings.Contains(strings.Join(h.visiblePaths(), " "), "kv/restricted") {
		t.Fatal("denied node disappeared from the tree")
	}
}

// Right on a directory still expands it: focus only makes sense once the pane
// has something to show.
func TestRightOnADirectoryExpandsInsteadOfFocusing(t *testing.T) {
	h := newHarness(t)
	h.focus("kv/")
	h.press("right")

	if h.ctx.Focus != ui.FocusList {
		t.Fatal("expanding a mount must not steal focus")
	}
	if !strings.Contains(strings.Join(h.visiblePaths(), " "), "kv/app") {
		t.Fatalf("right should have expanded the mount: %v", h.visiblePaths())
	}
}

// A mount that lists as empty has to say so, in the tree and the panel: a dev
// Vault ships one (secret/), and an open arrow at blank space reads as a failed
// load. The three states the panel rendered alike are checked apart here.
func TestEmptyMountSaysSoInsteadOfLookingUnloaded(t *testing.T) {
	srv := fake.NewEmpty()
	h := &harness{t: t, m: New(srv, requester{t: t}), server: srv, ctx: newContext()}
	h.resize(100, 24)
	h.run(h.m.Load())

	h.focus("kv/")
	if view := h.view(); !strings.Contains(view, "not loaded") {
		t.Errorf("an unopened mount should say it is not loaded:\n%s", view)
	}

	h.press("right")
	if n := h.m.tree.current(); !n.emptyDir {
		t.Fatal("precondition: kv/ should have loaded with no children")
	}

	view := h.view()
	if !strings.Contains(view, "(empty)") {
		t.Errorf("the tree row should be labelled empty:\n%s", view)
	}
	if !strings.Contains(view, "this mount is empty") {
		t.Errorf("the detail panel should say the mount is empty:\n%s", view)
	}
}

// The engine is the section key: it is decided in setMounts and nothing the user
// does can move a mount from one section to another.
func TestMountsAreGroupedByEngine(t *testing.T) {
	h := newHarness(t)
	view := h.view()

	v2 := strings.Index(view, "KV V2")
	v1 := strings.Index(view, "KV V1")
	nb := strings.Index(view, "NOT BROWSABLE")

	if v2 < 0 || v1 < 0 || nb < 0 {
		t.Fatalf("all three sections should be labelled (v2=%d v1=%d nb=%d):\n%s", v2, v1, nb, view)
	}
	if v2 > v1 || v1 > nb {
		t.Fatalf("sections out of order (v2=%d v1=%d nb=%d):\n%s", v2, v1, nb, view)
	}
}

// The engine label moved from the row to the section header, but it still has to
// be on screen: KV v1 and v2 differ in whether history exists and whether a
// delete can be undone.
func TestMountsAreLabelledWithTheirEngine(t *testing.T) {
	h := newHarness(t)
	view := h.view()

	if !strings.Contains(view, "KV V2") || !strings.Contains(view, "KV V1") {
		t.Fatalf("mounts should be sectioned by KV version:\n%s", view)
	}
	if strings.Contains(view, "  v2") {
		t.Fatalf("a grouped row must not repeat what its header says:\n%s", view)
	}
}

func TestCursorNeverLandsOnAGroupHeader(t *testing.T) {
	h := newHarness(t)
	steps := len(h.m.tree.visible) + 2

	for i := 0; i < steps; i++ {
		if h.m.tree.current() == nil {
			t.Fatalf("cursor landed on a non-node row going down, step %d", i)
		}
		h.press("down")
	}
	for i := 0; i < steps; i++ {
		if h.m.tree.current() == nil {
			t.Fatalf("cursor landed on a non-node row going up, step %d", i)
		}
		h.press("up")
	}

	h.press("home")
	if h.m.tree.current() == nil {
		t.Fatal("home landed on a non-node row")
	}
	h.press("end")
	if h.m.tree.current() == nil {
		t.Fatal("end landed on a non-node row")
	}
}

// A section whose mounts the filter hid must lose its header too, or the pane
// grows labels pointing at nothing. "k" is a subsequence of "kv" and of "pki",
// but not of "legacy", so it empties exactly one of the three sections.
func TestFilterHidesTheHeaderOfAnEmptiedGroup(t *testing.T) {
	h := newHarness(t)

	if !strings.Contains(h.view(), "KV V1") {
		t.Fatalf("precondition: legacy/ should have a section:\n%s", h.view())
	}

	h.press("/")
	h.typeText("k")
	h.press("enter")

	view := h.view()
	if strings.Contains(view, "KV V1") {
		t.Fatalf("the emptied section must lose its header:\n%s", view)
	}
	if !strings.Contains(view, "KV V2") || !strings.Contains(view, "NOT BROWSABLE") {
		t.Fatalf("the surviving sections keep theirs:\n%s", view)
	}
}

func TestNonKVMountIsShownButNotBrowsable(t *testing.T) {
	h := newHarness(t)

	pki := h.mountNamed("pki")
	if pki.kvVersion != vault.KVUnknown {
		t.Fatalf("precondition: pki/ should not be a KV mount, got version %d", pki.kvVersion)
	}
	if pki.expandable() {
		t.Fatal("a non-KV mount must not be expandable")
	}
	// The explanation moved to the section header; the engine type stays where
	// it distinguishes one mount from another, since pki and transit share a
	// heading. Dropped only when it repeats the name: "pki/  pki" said it twice.
	view := h.view()
	if !strings.Contains(view, "NOT BROWSABLE") {
		t.Fatalf("the tree should say why the mount cannot be opened:\n%s", view)
	}
	if strings.Contains(view, "pki/  pki") {
		t.Errorf("the row repeats the mount name as its type:\n%s", view)
	}
	if !strings.Contains(view, "pki/") {
		t.Errorf("the mount itself must still be listed:\n%s", view)
	}
	// A type that does differ from the name still survives; the fake seeds no
	// such mount, so TestNotBrowsableStillSaysSo pins that half directly.
}

// "pki/  pki" says nothing twice under a heading that already says the mount
// cannot be opened.
func TestMountTypeIsDroppedWhenItRepeatsTheName(t *testing.T) {
	cases := []struct {
		name, mountType string
		redundant       bool
	}{
		{"pki", "pki", true},
		{"identity", "identity", true},
		{"agent-registry", "agent_registry", true},
		{"sys", "system", false},
		{"secrets", "kv", false},
	}

	for _, c := range cases {
		if got := redundantType(c.name, c.mountType); got != c.redundant {
			t.Errorf("redundantType(%q, %q) = %v, want %v",
				c.name, c.mountType, got, c.redundant)
		}
	}
}

// A non-browsable mount still explains itself when the type carries information.
func TestNotBrowsableStillSaysSo(t *testing.T) {
	pki := &node{kind: kindMount, name: "pki", mount: "pki", mountType: "pki", kvVersion: 0}
	sys := &node{kind: kindMount, name: "sys", mount: "sys", mountType: "system", kvVersion: 0}

	if got := mountSuffix(pki, true); got != "  (not browsable)" {
		t.Errorf("pki ungrouped: got %q", got)
	}
	if got := mountSuffix(pki, false); got != "" {
		t.Errorf("pki grouped: got %q, want nothing", got)
	}
	if got := mountSuffix(sys, false); got != "  system" {
		t.Errorf("sys grouped: got %q, want the type", got)
	}
}

// A loading node once drew an animated marker a column wider than the arrow it
// replaced, so the name slid sideways while the listing was in flight. Nothing
// animates now; this keeps any future marker the same width as the arrow.
func TestALoadingRowIsTheSameWidthAsAnIdleOne(t *testing.T) {
	h := newHarness(t)
	var n *node
	for _, r := range h.m.tree.visible {
		if r.node != nil {
			n = r.node
			break
		}
	}
	if n == nil {
		t.Fatal("no node rows in the tree")
	}

	idle := lipgloss.Width(h.m.renderTreeRow(n, false, 60, true))
	n.loading = true
	if busy := lipgloss.Width(h.m.renderTreeRow(n, false, 60, true)); busy != idle {
		t.Errorf("a loading row is %d columns and an idle one %d", busy, idle)
	}
}

// withTwinNames puts a secret beside the directory of the same name.
func withTwinNames(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	// kv/app/ already exists in the fixtures, holding prod/ and stage/. This
	// puts a secret beside it under the same name.
	h.server.PutVersions("kv", "app", []map[string]any{
		{"note": "the secret called app"},
	}, time.Date(2026, 8, 1, 14, 22, 0, 0, time.UTC))
	return h
}

// A directory and a secret can share a name - kv/app/ beside kv/app - and the
// trailing slash is all that tells them apart. The two middle rows differ in
// the slash and nothing else, landing on the same path with different kinds.
func TestGotoLandsWhereTheTrailingSlashSays(t *testing.T) {
	tests := []struct {
		name  string
		twins bool
		path  string
		want  string
		kind  nodeKind
	}{
		{"walking through a directory that shares a secret's name", true, "kv/app/prod/db", "kv/app/prod/db", kindSecret},
		{"on the secret when no slash is given", true, "kv/app", "kv/app", kindSecret},
		{"on the directory when the slash is given", true, "kv/app/", "kv/app", kindDir},
		// A name that exists as only one of the two is not worth failing over:
		// the path is unambiguous in practice whatever the slash says.
		{"on the secret when only it exists, slash or not", false, "kv/app/prod/db/", "kv/app/prod/db", kindSecret},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			if tt.twins {
				h = withTwinNames(t)
			}
			h.run(h.m.Goto(tt.path))

			n := h.m.tree.current()
			if n == nil {
				t.Fatal("the walk landed nowhere")
			}
			if n.fullPath() != tt.want {
				t.Errorf("landed on %q, want %q", n.fullPath(), tt.want)
			}
			if n.kind != tt.kind {
				t.Errorf("landed on a %v, want a %v", n.kind, tt.kind)
			}
		})
	}
}

// A directory and a secret can share a name. A secret created next to a
// directory of its name was treated as already listed, so the tree never
// showed it.
func TestADirectoryIsNotMistakenForASecretOfTheSameName(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	kv := h.mountNamed("kv")
	if !kv.loaded {
		t.Fatal("the mount should be listed by now")
	}
	if kv.hasChild("app") {
		t.Error("the directory app/ was mistaken for a secret called app")
	}
}
