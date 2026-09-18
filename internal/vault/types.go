// Package vault is the port and its SDK adapter: Client is the interface the
// rest of vivi speaks, apiClient the one implementation over hashicorp/vault/api.
// Resolved here so they never leak into the views: the KV version, and the 404.
package vault

import "time"

// KV engine versions. KVUnknown covers mounts that are not KV at all.
const (
	KVUnknown = 0
	KV1       = 1
	KV2       = 2
)

// Mount is a secrets engine mounted on the server.
type Mount struct {
	// Path always ends in "/", the way sys/mounts reports it.
	Path        string
	Type        string
	Description string
	Accessor    string
	// KVVersion is KV1, KV2, or KVUnknown for non-KV engines.
	KVVersion int
}

// Entry is one child of a path listing.
type Entry struct {
	// Name has no trailing slash, even for directories.
	Name string
	// IsDir is true when Vault reported the child with a trailing slash. A path
	// can be both a secret and a directory: such an entry appears twice in a
	// listing, and the tree keeps both.
	IsDir bool
}

// Secret is one version of a key-value secret.
type Secret struct {
	Mount string
	// Path is relative to the mount, without a leading slash.
	Path string
	Data map[string]any

	// Version is 0 for KV v1 mounts, which have no versioning.
	Version     int
	CreatedTime time.Time

	// Metadata is nil for KV v1 mounts.
	Metadata *Metadata
}

// Metadata is the per-secret metadata a KV v2 mount keeps alongside the
// versions.
type Metadata struct {
	CASRequired        bool
	MaxVersions        int
	CurrentVersion     int
	OldestVersion      int
	CreatedTime        time.Time
	UpdatedTime        time.Time
	DeleteVersionAfter time.Duration
	CustomMetadata     map[string]any
}

// VersionInfo describes one version in a secret's history.
type VersionInfo struct {
	Version      int
	CreatedTime  time.Time
	DeletionTime time.Time
	Destroyed    bool
}

// Deleted reports a soft-deleted version: the data is gone but undelete can
// still bring it back. A destroyed version is not merely deleted.
func (v VersionInfo) Deleted() bool {
	return !v.Destroyed && !v.DeletionTime.IsZero()
}

// Readable reports whether the version still has data to show.
func (v VersionInfo) Readable() bool { return !v.Destroyed && v.DeletionTime.IsZero() }

// TokenInfo is what the current token can tell about itself. It answers "with
// which badge am I operating", which matters because vivi authenticates only
// from the environment.
type TokenInfo struct {
	Accessor    string
	DisplayName string
	Policies    []string
	TTL         time.Duration
	Renewable   bool
	EntityID    string
	Namespace   string
	// ExpiresAt is zero for tokens that never expire, such as root.
	ExpiresAt time.Time
}

// AuthMethod is an enabled authentication backend.
type AuthMethod struct {
	// Path always ends in "/".
	Path        string
	Type        string
	Description string
	Accessor    string
}

// Role is one named role or entity under an auth method.
type Role struct {
	Name     string
	Policies []string
	// Data is the raw role configuration, shown as-is in the detail panel.
	Data map[string]any
	// ReadErr is why Policies and Data are empty, when they are empty because
	// the role could not be read rather than because it grants nothing.
	ReadErr error
}

// ServerInfo is the connection context shown in the header.
type ServerInfo struct {
	Address   string
	Namespace string
	Version   string
	Sealed    bool
	Standby   bool
}
