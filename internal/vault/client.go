package vault

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hashicorp/vault/api"
)

// Client is everything vivi needs from a Vault server. The TUI depends on this
// interface and never on *api.Client, so the whole interface layer can be
// exercised against fake.Server. It is the union of the five roles below, each
// small enough to be taken on its own by a caller that needs only that much.
type Client interface {
	Connection
	SecretReader
	SecretWriter
	Auditor
	TokenHolder
}

// Connection is the server itself: what it is and what it serves.
type Connection interface {
	// Server describes the connection itself, for the header.
	Server() ServerInfo

	// Health fills in what only the server can say about itself - its version,
	// whether it is sealed, and whether this node is a standby - and returns the
	// ServerInfo that Server() will report from then on.
	Health(ctx context.Context) (ServerInfo, error)

	// Mounts lists the secrets engines. Non-KV mounts are included so the tree
	// can show them as unbrowsable rather than hide them.
	Mounts(ctx context.Context) ([]Mount, error)
}

// SecretReader walks and reads KV mounts without changing anything.
type SecretReader interface {
	// List returns the children of a path within a mount. path is relative to
	// the mount and may be empty for the mount root.
	List(ctx context.Context, mount, path string) ([]Entry, error)

	// Read returns a secret. version 0 means the current version; a non-zero
	// version is only valid on KV v2.
	Read(ctx context.Context, mount, path string, version int) (*Secret, error)

	// Versions returns the history newest first, plus the secret's metadata:
	// one request answers both, and the detail panel shows them together.
	// KV v1 returns ErrUnsupported.
	Versions(ctx context.Context, mount, path string) ([]VersionInfo, *Metadata, error)
}

// SecretWriter is every call that changes a KV mount. A read-only session
// never reaches one of these.
type SecretWriter interface {
	// Write stores data as a new version. cas is the version the caller believes
	// is current, failing with ErrCAS if the server moved past it; 0 creates a
	// secret that must not exist, CASDisabled overwrites (KV v1's only mode).
	Write(ctx context.Context, mount, path string, data map[string]any, cas int) (*Secret, error)

	// Delete soft-deletes versions on KV v2; on KV v1, versions is ignored and
	// the secret itself is removed, which cannot be undone.
	Delete(ctx context.Context, mount, path string, versions []int) error

	// Undelete restores soft-deleted versions. KV v2 only.
	Undelete(ctx context.Context, mount, path string, versions []int) error

	// Destroy permanently removes the data of specific versions. KV v2 only,
	// and irreversible.
	Destroy(ctx context.Context, mount, path string, versions []int) error

	// DeleteMetadata removes the secret and every version of it. KV v2 only,
	// and irreversible.
	DeleteMetadata(ctx context.Context, mount, path string) error
}

// Auditor answers who may do what: the server's own verdict on a path, and
// the policies and auth methods that verdict is built from.
type Auditor interface {
	// Capabilities reports what the current token may do at a raw API path,
	// which is the only answer that comes from the server rather than from
	// reading policy documents and inferring.
	Capabilities(ctx context.Context, path string) ([]string, error)

	// Policies lists policy names.
	Policies(ctx context.Context) ([]string, error)

	// Policy returns a policy's HCL document.
	Policy(ctx context.Context, name string) (string, error)

	// AuthMethods lists the enabled auth backends.
	AuthMethods(ctx context.Context) ([]AuthMethod, error)

	// AuthRoles lists the roles configured under one auth method. Methods
	// without a role concept return an empty slice and no error.
	AuthRoles(ctx context.Context, method AuthMethod) ([]Role, error)
}

// TokenHolder is the token the session runs with.
type TokenHolder interface {
	// TokenInfo describes the token vivi is using.
	TokenInfo(ctx context.Context) (*TokenInfo, error)

	// RenewToken extends the current token's lease and returns what the token
	// looks like afterwards. It fails with ErrUnsupported when the token is not
	// renewable.
	RenewToken(ctx context.Context) (*TokenInfo, error)
}

// CASDisabled asks Write to overwrite whatever is there, skipping the
// check-and-set guard.
const CASDisabled = -1

// apiClient is the real implementation, over hashicorp/vault/api.
type apiClient struct {
	c    *api.Client
	info ServerInfo

	// mu guards kinds, a cache of mount path to KV version. A cache and not a
	// parameter because every secret call needs the version to pick its request
	// shape, and threading it through would put that concern into the views.
	mu    sync.RWMutex
	kinds map[string]int
}

// Config configures a real client. Everything comes from the environment: vivi
// deliberately has no config file of its own and behaves like the vault CLI.
type Config struct {
	Address   string
	Token     string
	Namespace string
}

// ConfigFromEnv reads VAULT_ADDR, VAULT_TOKEN and VAULT_NAMESPACE, falling back
// to the token cached by `vault login` in ~/.vault-token.
func ConfigFromEnv() Config {
	cfg := Config{
		Address:   os.Getenv("VAULT_ADDR"),
		Token:     os.Getenv("VAULT_TOKEN"),
		Namespace: os.Getenv("VAULT_NAMESPACE"),
	}
	if cfg.Token == "" {
		cfg.Token = tokenFromHelper()
	}
	return cfg
}

func tokenFromHelper() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	// Fixed name inside the user's own home directory: the very file the vault
	// CLI writes on `vault login`, and nothing here comes from user input.
	b, err := os.ReadFile(filepath.Join(home, ".vault-token")) // #nosec G304
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// New builds a client against a real server. It does not talk to the server;
// reachability is checked by Preflight so the caller controls when that cost is
// paid and how the failure is reported.
func New(cfg Config) (Client, error) {
	apiCfg := api.DefaultConfig()
	if apiCfg.Error != nil {
		return nil, fmt.Errorf("read Vault environment: %w", apiCfg.Error)
	}
	if cfg.Address != "" {
		apiCfg.Address = cfg.Address
	}

	c, err := api.NewClient(apiCfg)
	if err != nil {
		return nil, fmt.Errorf("create Vault client: %w", err)
	}
	c.SetToken(cfg.Token)
	if cfg.Namespace != "" {
		c.SetNamespace(cfg.Namespace)
	}

	return &apiClient{
		c: c,
		info: ServerInfo{
			Address:   c.Address(),
			Namespace: cfg.Namespace,
		},
		kinds: map[string]int{},
	}, nil
}

func (a *apiClient) Server() ServerInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.info
}

// kvVersion returns the KV version of a mount, loading the mount table once if
// this mount has not been seen yet.
func (a *apiClient) kvVersion(ctx context.Context, mount string) (int, error) {
	key := normalizeMount(mount)

	a.mu.RLock()
	v, ok := a.kinds[key]
	a.mu.RUnlock()
	if ok {
		return v, nil
	}

	if _, err := a.Mounts(ctx); err != nil {
		return KVUnknown, err
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
	if v, ok = a.kinds[key]; ok {
		return v, nil
	}
	return KVUnknown, &Error{Op: "resolve mount", Path: mount, Kind: ErrNotFound}
}

// normalizeMount reduces a mount to a bare name with no surrounding slashes,
// so "kv", "kv/" and "/kv/" are one key.
func normalizeMount(mount string) string {
	return strings.Trim(mount, "/")
}

// JoinPath builds a Vault path from parts, dropping empty ones so a mount root
// makes no double slash. Exported because app builds paths too, and two copies
// would be two chances to disagree about what a mount root joins to.
func JoinPath(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, "/")
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "/")
}
