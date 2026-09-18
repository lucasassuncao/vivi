package app

import (
	"testing"
	"time"
)

// An address that is not obviously local or a sandbox is treated as production.
// Guessing wrong toward PROD costs a moment of care; guessing wrong toward
// LOCAL invites someone to destroy a version on a server they misread.
func TestEnvironmentClassification(t *testing.T) {
	tests := []struct {
		address string
		want    Environment
	}{
		{"http://127.0.0.1:8200", EnvLocal},
		{"http://localhost:8200", EnvLocal},
		{"https://vault.dev.corp:8200", EnvDev},
		{"https://vault-staging.corp:8200", EnvDev},
		{"https://vault-qa.corp:8200", EnvDev},
		{"https://vault.corp:8200", EnvProd},
		{"https://secrets.internal", EnvProd},
		{"", EnvProd},

		// These all contain a sandbox word as a substring and none of them is
		// one; a guess has to fall towards care.
		{"https://vault.qatar.corp:8200", EnvProd},
		{"https://prod-devices.corp:8200", EnvProd},
		{"https://vault.stgermain.example:8200", EnvProd},
		{"https://latest-vault.corp:8200", EnvProd},
		{"https://collaborate.corp:8200", EnvProd},
		{"https://sbxpress.corp:8200", EnvProd},

		// And the shapes that really are sandboxes still read as one.
		{"https://vault.test.corp:8200", EnvDev},
		{"https://vault-dev-01.corp:8200", EnvDev},
		{"https://qa.vault.corp:8200", EnvDev},
		{"https://vault_sandbox.corp:8200", EnvDev},
		{"https://vault-sbx.corp:8200", EnvDev},
	}

	for _, tc := range tests {
		if got := ClassifyEnvironment(tc.address); got != tc.want {
			t.Errorf("%q: got %s, want %s", tc.address, got.Label(), tc.want.Label())
		}
	}
}

func TestTokenTTLWarnsAsItRunsOut(t *testing.T) {
	tests := []struct {
		name string
		ttl  time.Duration
		want TTLUrgency
	}{
		{"plenty", 4 * time.Hour, TTLFine},
		{"under an hour", 40 * time.Minute, TTLSoon},
		{"minutes left", 5 * time.Minute, TTLCritical},
	}

	for _, tc := range tests {
		if got := ClassifyTTL(tc.ttl); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
