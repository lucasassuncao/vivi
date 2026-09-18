package vault

import (
	"testing"
	"time"
)

func TestVersionInfoDistinguishesDeletedFromDestroyed(t *testing.T) {
	when := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	deleted := VersionInfo{Version: 2, DeletionTime: when}
	destroyed := VersionInfo{Version: 1, Destroyed: true, DeletionTime: when}
	live := VersionInfo{Version: 3}

	if !deleted.Deleted() || deleted.Readable() {
		t.Fatal("a soft-deleted version is deleted and unreadable")
	}
	// A destroyed version is not merely deleted: undelete cannot bring it back,
	// and the panel must not offer to.
	if destroyed.Deleted() {
		t.Fatal("a destroyed version must not report itself as merely deleted")
	}
	if !live.Readable() {
		t.Fatal("a live version should be readable")
	}
}
