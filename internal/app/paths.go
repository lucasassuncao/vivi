package app

import "github.com/lucasassuncao/vivi/internal/vault"

// PolicyPath is what a policy must name to grant access to a secret's data.
func PolicyPath(mount, path string, kvVersion int) string {
	if kvVersion == vault.KV2 {
		return vault.JoinPath(mount, "data", path)
	}
	return vault.JoinPath(mount, path)
}

// MetadataPolicyPath is what a policy must name to grant the version history
// and delete-metadata. It returns "" on a v1 mount, which keeps no metadata.
func MetadataPolicyPath(mount, path string, kvVersion int) string {
	if kvVersion != vault.KV2 {
		return ""
	}
	return vault.JoinPath(mount, "metadata", path)
}
