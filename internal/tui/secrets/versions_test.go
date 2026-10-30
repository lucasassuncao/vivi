package secrets

import (
	"strings"
	"testing"
)

func TestDestroyRequiresTypingThePath(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")
	h.press("V")

	h.press("D")
	if h.modal() == nil || !h.modal().danger {
		t.Fatal("destroy must open the strong confirmation")
	}

	// A stray "y" must not be enough for an irreversible operation.
	h.press("y")
	if h.modal() == nil {
		t.Fatal("y closed a typed confirmation")
	}

	h.press("enter") // not satisfied yet
	if h.modal() == nil {
		t.Fatal("enter confirmed before the path was typed")
	}

	h.press("esc")
	if h.modal() != nil {
		t.Fatal("esc must cancel")
	}
	for _, v := range h.versions("app/prod/db") {
		if v.Version == 4 && v.Destroyed {
			t.Fatal("the current version was destroyed by a cancelled modal")
		}
	}
}

func TestDestroyRunsOnceThePathIsTyped(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")
	h.press("V", "D")

	h.typeText("db")
	h.press("enter")

	var destroyed bool
	for _, v := range h.versions("app/prod/db") {
		if v.Version == 4 && v.Destroyed {
			destroyed = true
		}
	}
	if !destroyed {
		t.Fatal("destroy did not run after the path was typed correctly")
	}
}

func TestSoftDeleteAndUndeleteRoundTrip(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/cache")

	h.press("d") // delete asks y/n, not a typed path: it is reversible
	if h.modal() == nil || h.modal().danger {
		t.Fatal("a soft-delete should use the light confirmation")
	}
	h.press("y")

	versions := h.versions("app/prod/cache")
	if !versions[0].Deleted() {
		t.Fatal("version was not soft-deleted")
	}

	h.press("V", "u")
	if h.versions("app/prod/cache")[0].Deleted() {
		t.Fatal("undelete did not restore the version")
	}
}

func TestRollbackWritesANewVersionAndDestroysNothing(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")
	h.press("V")

	h.press("down", "down") // v4, v3 -> land on v2... move to a readable one
	for h.currentVersionReadable() == false && h.m.versionCursor > 0 {
		h.press("up")
	}

	before := len(h.versions("app/prod/db"))
	h.press("b")
	if h.modal() == nil || h.modal().danger {
		t.Fatal("rollback destroys nothing and should use the light confirmation")
	}
	h.press("y")

	after := h.versions("app/prod/db")
	if len(after) != before+1 {
		t.Fatalf("rollback should append a version: %d -> %d", before, len(after))
	}
}

// A failed rollback is a write with no editor behind it. As an enum beside a
// nullable field, the failure handler could set "editing" with a nil editor - a
// mode that swallowed every key. The payload rides inside now.
func TestFailedRollbackDoesNotLandInAnEmptyEditor(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("V") // version list

	// v1 is destroyed in the fixture: reading it to roll back fails.
	for h.m.versionCursor < len(h.m.versions)-1 {
		h.press("down")
	}
	if v := h.m.currentVersionInfo(); v == nil || !v.Destroyed {
		t.Skip("fixture changed: the oldest version is no longer destroyed")
	}

	h.press("b") // rollback; the modal refuses a version with no data

	if h.mode() == "edit" && h.editor() == nil {
		t.Fatal("landed in the editing mode with no editor")
	}
	if h.editor() != nil && h.mode() != "edit" {
		t.Fatal("an editor exists outside the editing mode")
	}
	_ = h.view() // must not panic whatever it decided
}

// The version list keeps its own cursor and leaving it does not reset it, so a
// browse-mode "d" read that cursor and soft-deleted whichever row the list had
// been resting on, under a modal naming the secret and not the version.
func TestDeleteFromTheTreeIgnoresAClosedVersionListCursor(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	h.press("V")    // the version list, newest first
	h.press("down") // rest on the one below the current version
	older := h.m.currentVersionInfo()
	if older == nil || older.Version == 4 {
		t.Fatalf("fixture changed: expected an older version under the cursor, got %+v", older)
	}
	h.press("esc")

	h.press("d")
	h.press("y")

	for _, v := range h.versions("app/prod/db") {
		if v.Version == older.Version && v.Deleted() {
			t.Fatalf("d deleted v%d, the row a closed list was resting on", v.Version)
		}
		if v.Version == 4 && !v.Deleted() {
			t.Fatal("d must delete the version on screen, which is the current one")
		}
	}
}

// M takes the secret under the tree cursor, and only there: the version list
// and the pane both leave it alone.
func TestDestroySecretRunsFromTheTree(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	h.press("V", "M")
	if h.modal() != nil {
		t.Fatal("M still opens a modal from the version list")
	}
	h.press("esc")

	h.press("M")
	if h.modal() == nil || !h.modal().danger {
		t.Fatal("M in the tree must open the strong confirmation")
	}
	h.typeText("db")
	h.press("enter")

	if _, _, err := h.server.Versions(t.Context(), "kv", "app/prod/db"); err == nil {
		t.Fatal("the secret still has a history after M")
	}
}

// v1 keeps no metadata, so M has nothing to remove there.
func TestDestroySecretIsNotOfferedOnKV1(t *testing.T) {
	h := newHarness(t)
	h.open("legacy/old/app")

	h.press("M")
	if h.modal() != nil {
		t.Fatal("M opened a modal on a KV v1 secret")
	}
	_, keys := h.m.Legend(h.ctx)
	for _, e := range keys {
		if e.Help.Key == "M" {
			t.Fatal("the legend offers M on a KV v1 secret")
		}
		if e.Help.Key == "d" && e.Help.Desc != "delete" {
			t.Fatalf("a KV v1 delete is final, but the legend says %q", e.Help.Desc)
		}
	}
}

func (h *harness) currentVersionReadable() bool {
	v := h.m.currentVersionInfo()
	return v != nil && v.Readable()
}

// The boxes are for marking: idle under the detail they read as something to
// click, so they show only while the version list is open.
func TestTheVersionBoxesShowOnlyWhileMarking(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	if pane := stripANSI(h.m.renderVersions(100)); strings.Contains(pane, "[ ]") {
		t.Errorf("the boxes show with the list closed:\n%s", pane)
	}
	h.press("v")
	if pane := stripANSI(h.m.renderVersions(100)); !strings.Contains(pane, "[ ] v") {
		t.Errorf("the open list has no boxes to mark:\n%s", pane)
	}
	h.press("space")
	if pane := stripANSI(h.m.renderVersions(100)); !strings.Contains(pane, "[×] v") {
		t.Errorf("a marked version is not ticked:\n%s", pane)
	}
}

// After M the cursor goes up to the folder, which the pane shows at once: left
// where it was it fell on a sibling nobody read, and the pane said loading for
// good. The toast says what happened, not the endpoint's name.
func TestDestroyingASecretLeavesTheCursorOnItsFolder(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	h.press("M")
	h.typeText("db")
	h.press("enter")

	if got := h.m.CursorPath(); got != "kv/app/prod" {
		t.Errorf("the cursor is on %q, want the folder the secret was in", got)
	}
	if strings.Contains(h.view(), "loading") {
		t.Errorf("the pane is waiting on something:\n%s", h.view())
	}
	if got := h.banner(); got != "destroyed kv/app/prod/db and its history" {
		t.Errorf("the toast says %q", got)
	}
}

// Every operation reads as what it did, and a failure as what it could not do.
func TestOperationsReadAsWhatTheyDid(t *testing.T) {
	for _, tc := range []struct {
		op          string
		kv1         bool
		done, doing string
	}{
		{"delete", false, "soft-deleted a/b", "soft-delete a/b"},
		{"delete", true, "deleted a/b for good", "delete a/b"},
		{"undelete", false, "restored a/b", "restore a/b"},
		{"destroy", false, "destroyed versions of a/b", "destroy versions of a/b"},
		{"delete-metadata", false, "destroyed a/b and its history", "destroy a/b and its history"},
	} {
		if done, doing := opWords(tc.op, "a/b", tc.kv1); done != tc.done || doing != tc.doing {
			t.Errorf("%s (kv1 %v) reads %q / %q", tc.op, tc.kv1, done, doing)
		}
	}
}
