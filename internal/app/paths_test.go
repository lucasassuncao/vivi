package app_test

import (
	"testing"

	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// The v2 data/ segment is the whole reason this exists: a policy naming the
// browsing path grants nothing, and it grants nothing silently.
func TestPolicyPathCarriesTheKVSegments(t *testing.T) {
	for _, tc := range []struct {
		kv                 int
		wantData, wantMeta string
	}{
		{vault.KV2, "kv/data/app/prod/db", "kv/metadata/app/prod/db"},
		{vault.KV1, "kv/app/prod/db", ""},
		{vault.KVUnknown, "kv/app/prod/db", ""},
	} {
		if got := app.PolicyPath("kv", "app/prod/db", tc.kv); got != tc.wantData {
			t.Errorf("kv v%d: policy path %q, want %q", tc.kv, got, tc.wantData)
		}
		if got := app.MetadataPolicyPath("kv", "app/prod/db", tc.kv); got != tc.wantMeta {
			t.Errorf("kv v%d: metadata path %q, want %q", tc.kv, got, tc.wantMeta)
		}
	}
	// A mount root must not produce a double slash.
	if got := app.PolicyPath("kv/", "", vault.KV2); got != "kv/data" {
		t.Errorf("mount root: %q", got)
	}
}
