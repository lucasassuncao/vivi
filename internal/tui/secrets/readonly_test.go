package secrets

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/app"
)

// The read-only guards that belong to this tab: the gate every write passes
// through, and the keys that refuse out loud. The header badge, the palette and
// the banner's expiry are the shell's, and are tested there.

// The layer that matters. Every other guard is about being understood; this one
// is about being right, and it is the one a message arriving from anywhere - a
// stale modal, a replayed confirmation, a future refactor - has to pass.
func TestNoWriteReachesTheServerInAReadOnlySession(t *testing.T) {
	h := readOnlyHarness(t, app.ReadOnlyAlways)
	h.open("kv/app/prod/db")

	before := map[string]int{}
	for _, op := range []string{"write", "delete", "undelete", "destroy", "delete_metadata"} {
		before[op] = h.server.Calls[op]
	}

	// Straight past the keys and the modals: these are the messages a
	// confirmation dispatches once the user has already agreed.
	n := h.m.sel
	for _, msg := range []tea.Msg{
		confirmedSaveMsg{node: n, data: map[string]any{"x": "y"}, cas: 4},
		confirmedDeleteMsg{node: n, versions: []int{4}},
		confirmedDestroyMsg{node: n, versions: []int{4}},
		confirmedDeleteMetadataMsg{node: n},
		confirmedRollbackMsg{node: n, toVersion: 3, base: 4},
	} {
		h.run(h.m.Data(msg))
	}
	h.m.adopt(h.ctx)
	h.run(h.m.undeleteVersions(n, []int{2}))

	for op, was := range before {
		if now := h.server.Calls[op]; now != was {
			t.Errorf("a read-only session reached the server: %s went from %d to %d", op, was, now)
		}
	}
}

// A key that silently does nothing is indistinguishable from a key that is
// broken, and "d" not opening a modal is exactly when someone needs telling.
func TestWriteKeysRefuseOutLoud(t *testing.T) {
	for _, tc := range []struct{ key, wants string }{
		{"e", "edit"},
		{"a", "create"},
		{"d", "delete"},
	} {
		h := readOnlyHarness(t, app.ReadOnlyAlways)
		h.open("kv/app/prod/db")
		h.press(tc.key)

		if h.mode() != "browse" {
			t.Errorf("%q opened %q in a read-only session", tc.key, h.mode())
		}
		if !strings.Contains(h.banner(), tc.wants) {
			t.Errorf("%q did not say why it refused, banner is %q", tc.key, h.banner())
		}
		if !strings.Contains(h.banner(), "read-only") {
			t.Errorf("%q refused without naming the reason: %q", tc.key, h.banner())
		}
	}
}

// The version list has its own four, and they are the destructive ones.
func TestVersionKeysRefuseInAReadOnlySession(t *testing.T) {
	for _, key := range []string{"b", "u", "D", "M"} {
		h := readOnlyHarness(t, app.ReadOnlyAlways)
		h.open("kv/app/prod/db")
		h.press("V")
		h.press(key)

		if h.mode() != "versions" {
			t.Errorf("%q left the version list for %q", key, h.mode())
		}
		if !strings.Contains(h.banner(), "read-only") {
			t.Errorf("%q did not refuse out loud, banner is %q", key, h.banner())
		}
	}
}

// The legend drops what cannot run. The help panel keeps it and scopes it, and
// that half is the shell's because the banner above it is.
func TestTheLegendDropsWritesInAReadOnlySession(t *testing.T) {
	h := readOnlyHarness(t, app.ReadOnlyOnProd)
	h.open("kv/app/prod/db")

	// The tab reports every key it has; dropping the ones a read-only session
	// refuses is the shell's pass, so this checks the marking rather than the
	// dropping: an entry that forgets to say it writes cannot be dropped.
	h.press("V")
	_, keys := h.m.Legend(h.ctx)

	writes := map[string]bool{}
	for _, e := range keys {
		writes[e.Key] = e.Writes
	}
	for _, key := range []string{"b", "u", "D", "M"} {
		if !writes[key] {
			t.Errorf("the version legend does not mark %q as a write, so nothing will drop it", key)
		}
	}
	if _, offered := writes["d"]; !offered {
		t.Error("the version legend dropped [d] diff, which is a read and still works")
	}
	if writes["d"] {
		t.Error("[d] diff is marked as a write, so a read-only session would lose a read")
	}
}
