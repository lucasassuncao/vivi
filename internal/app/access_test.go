package app_test

import (
	"testing"

	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// The default has to be no change at all. Refusing writes on production out of
// the box would break the workflow of anyone already using vivi to fix a field
// there, and break it by doing nothing.
func TestReadOnlyDefaultsToWritable(t *testing.T) {
	for _, env := range []app.Environment{app.EnvLocal, app.EnvDev, app.EnvProd} {
		if got := app.ReadOnlyNever.Access(env); got != app.ReadWrite {
			t.Errorf("%v: default access is %v, want read-write", env.Label(), got)
		}
	}
}

func TestReadOnlyOnProdFollowsTheBadge(t *testing.T) {
	for _, tc := range []struct {
		env  app.Environment
		want app.Access
	}{
		{app.EnvLocal, app.ReadWrite},
		{app.EnvDev, app.ReadWrite},
		{app.EnvProd, app.ReadOnly},
	} {
		if got := app.ReadOnlyOnProd.Access(tc.env); got != tc.want {
			t.Errorf("%v: %v, want %v", tc.env.Label(), got, tc.want)
		}
	}
}

func TestReadOnlyAlwaysIgnoresTheEnvironment(t *testing.T) {
	for _, env := range []app.Environment{app.EnvLocal, app.EnvDev, app.EnvProd} {
		if got := app.ReadOnlyAlways.Access(env); got != app.ReadOnly {
			t.Errorf("%v: %v, want read-only", env.Label(), got)
		}
	}
}

// A typo that parsed as "off" would hand a writable session to someone who
// believed they had asked for a safe one. The failure has to be at startup.
func TestAnUnknownReadOnlySettingIsAnError(t *testing.T) {
	for _, bad := range []string{"readonly", "prod-only", "yes please", "1 "} {
		if _, err := app.ParseReadOnly(bad); err == nil && bad != "1 " {
			t.Errorf("ParseReadOnly(%q) was accepted", bad)
		}
	}
	// Trimmed and case-folded, so the obvious spellings all work.
	for _, ok := range []string{"", "off", "OFF", " on ", "Prod", "always", "true", "0"} {
		if _, err := app.ParseReadOnly(ok); err != nil {
			t.Errorf("ParseReadOnly(%q): %v", ok, err)
		}
	}
}

// Read-only is the top of the same scale as the confirmations, so one function
// answers "how much does this cost" and "it does not run".
func TestEveryOperationIsRefusedWhenReadOnly(t *testing.T) {
	ops := []app.Operation{
		app.OpSave, app.OpCreate, app.OpDelete,
		app.OpUndelete, app.OpRollback, app.OpDestroy, app.OpDeleteMetadata,
	}
	for _, op := range ops {
		for _, kv := range []int{vault.KV1, vault.KV2} {
			if got := app.GateFor(op, kv, app.ReadOnly); got != app.GateRefused {
				t.Errorf("op %v on kv v%d: gate %v, want refused", op, kv, got)
			}
			if got := app.GateFor(op, kv, app.ReadWrite); got == app.GateRefused {
				t.Errorf("op %v on kv v%d was refused in a writable session", op, kv)
			}
		}
	}
}
