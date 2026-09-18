package app

import (
	"testing"

	"github.com/lucasassuncao/vivi/internal/vault"
)

// Each of the three cases loses data quietly when it is wrong.
func TestCASSaysWhatEachWriteIsAllowedToOverwrite(t *testing.T) {
	if got := CAS(true, vault.KV2, 7); got != 0 {
		t.Errorf("a create must claim the path is empty: cas %d, want 0", got)
	}
	if got := CAS(false, vault.KV1, 7); got != vault.CASDisabled {
		t.Errorf("v1 has no check-and-set: cas %d, want CASDisabled", got)
	}
	if got := CAS(false, vault.KV2, 7); got != 7 {
		t.Errorf("a v2 write must name the version it read: cas %d, want 7", got)
	}
}

// Saving an edit made on a historical version would write it back as the newest
// one - a rollback wearing an edit's clothes, reverting every field while the
// user believes they are fixing one.
func TestEditingIsRefusedOnAHistoricalVersion(t *testing.T) {
	if CanEdit(3, 4) {
		t.Error("editing v3 while v4 is current would revert the secret")
	}
	if !CanEdit(4, 4) {
		t.Error("the current version is editable")
	}
	if !CanEdit(0, 4) {
		t.Error("version 0 means whatever is current, which is editable")
	}
	if !CanEdit(3, 0) {
		t.Error("a mount with no history has nothing to be historical about")
	}
}
