package secrets

import (
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

func (h *harness) currentVersionReadable() bool {
	v := h.m.currentVersionInfo()
	return v != nil && v.Readable()
}
