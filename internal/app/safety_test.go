package app

import (
	"testing"

	"github.com/lucasassuncao/vivi/internal/vault"
)

// The scale is reversibility and nothing else. These cases are the policy: if
// one of them changes, the change was a decision about data loss and should
// read like one.
func TestGateScalesWithWhatCannotBeUndone(t *testing.T) {
	tests := []struct {
		name string
		op   Operation
		kv   int
		want Gate
	}{
		{"rollback deletes nothing", OpRollback, vault.KV2, GateNone},
		{"undelete restores", OpUndelete, vault.KV2, GateNone},

		{"a v2 save keeps the old version", OpSave, vault.KV2, GateConfirm},
		{"a v2 delete is reversible", OpDelete, vault.KV2, GateConfirm},

		// The one that matters: the same key, the same word, two different acts.
		{"a v1 delete removes the only copy", OpDelete, vault.KV1, GateTyped},

		{"destroy erases the data", OpDestroy, vault.KV2, GateTyped},
		{"delete-metadata erases the history", OpDeleteMetadata, vault.KV2, GateTyped},
	}

	for _, tc := range tests {
		if got := GateFor(tc.op, tc.kv, ReadWrite); got != tc.want {
			t.Errorf("%s: gate %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A v1 save passes an ordinary yes/no gate and still has to admit that the
// previous value is not coming back. The gate decides how hard it is to say
// yes; this decides whether the user knows what they are saying yes to.
func TestWarnsCoversTheV1WritesTheGateDoesNot(t *testing.T) {
	if !Warns(OpSave, vault.KV1) {
		t.Error("a v1 save overwrites the only copy and must say so")
	}
	if !Warns(OpDelete, vault.KV1) {
		t.Error("a v1 delete cannot be undone and must say so")
	}
	if Warns(OpSave, vault.KV2) {
		t.Error("a v2 save is versioned; warning about it is noise")
	}
	if Warns(OpRollback, vault.KV1) {
		t.Error("a rollback destroys nothing, on any mount")
	}
}
