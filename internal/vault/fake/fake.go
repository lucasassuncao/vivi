// Package fake is the second driven adapter: an in-memory Vault satisfying
// vault.Client, so the suite runs without a server. It models Vault's behaviour
// where vivi must get it right - v1, soft-deleted versus destroyed - and no more.
package fake

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lucasassuncao/vivi/internal/vault"
)

// Server is an in-memory Vault. The zero value is not usable; call New or NewEmpty.
type Server struct {
	mu sync.Mutex

	mounts   []vault.Mount
	secrets  map[string]*record // keyed by "mount/path"
	policies map[string]string
	methods  []vault.AuthMethod
	roles    map[string][]vault.Role // keyed by auth method path
	token    *vault.TokenInfo
	server   vault.ServerInfo

	// Denied makes any operation whose path starts with one of these prefixes
	// fail with vault.ErrForbidden, the way a token missing a policy would.
	// Prefixes are "mount/path" with no leading slash.
	Denied []string

	// Calls counts operations by name, so tests can assert that the tree loads
	// lazily instead of walking everything up front.
	Calls map[string]int
}

// record is the stored history of one secret.
type record struct {
	versions []version
	metadata vault.Metadata
}

type version struct {
	data      map[string]any
	created   time.Time
	deletedAt time.Time
	destroyed bool
}

var _ vault.Client = (*Server)(nil)

// NewEmpty returns a server with two mounts and no secrets.
func NewEmpty() *Server {
	return &Server{
		mounts: []vault.Mount{
			{Path: "kv/", Type: "kv", KVVersion: vault.KV2, Description: "versioned secrets"},
			{Path: "legacy/", Type: "kv", KVVersion: vault.KV1, Description: "unversioned secrets"},
			// A mount vivi lists but cannot browse. The dev-mode Vault the README
			// documents has one, and without it the non-KV path is untested.
			{Path: "pki/", Type: "pki", KVVersion: vault.KVUnknown, Description: "certificates"},
		},
		secrets:  map[string]*record{},
		policies: map[string]string{},
		roles:    map[string][]vault.Role{},
		token: &vault.TokenInfo{
			Accessor:    "hmac-accessor",
			DisplayName: "token",
			Policies:    []string{"default"},
			TTL:         time.Hour,
		},
		server: vault.ServerInfo{Address: "https://vault.test:8200", Version: "1.23.0"},
		Calls:  map[string]int{},
	}
}

// New returns a server populated with fixtures covering the cases vivi has to
// render: a v2 mount with nested paths, a v1 mount, a secret with deleted and
// destroyed versions, a denied subtree, policies, and an auth method with roles.
func New() *Server {
	s := NewEmpty()
	now := time.Date(2026, 8, 1, 14, 22, 0, 0, time.UTC)

	s.PutVersions("kv", "app/prod/db", []map[string]any{
		{"username": "svc_app", "password": "hunter2-v1"},
		{"username": "svc_app", "password": "hunter2-v2"},
		{"username": "svc_app", "password": "hunter2-v3"},
		{"username": "svc_app", "password": "hunter2-v4", "host": "db.prod.internal"},
	}, now)
	s.DeleteVersion("kv", "app/prod/db", 2)
	s.DestroyVersion("kv", "app/prod/db", 1)

	s.PutVersions("kv", "app/prod/cache", []map[string]any{
		{"url": "redis://cache.prod:6379"},
	}, now)
	s.PutVersions("kv", "app/stage/db", []map[string]any{
		{"username": "svc_stage", "password": "stage-pw"},
	}, now)
	s.PutVersions("kv", "platform/ci/token", []map[string]any{
		{"token": "ci-token"},
	}, now)
	s.PutVersions("legacy", "old/app", []map[string]any{
		{"key": "no-history-here"},
	}, now)

	s.Denied = []string{"kv/restricted"}
	s.PutVersions("kv", "restricted/root-key", []map[string]any{{"key": "unreachable"}}, now)

	s.policies["default"] = "# default policy\n"
	s.policies["app-prod-db"] = "path \"kv/data/app/prod/db\" {\n  capabilities = [\"read\"]\n}\n"
	s.policies["app-admin"] = "path \"kv/data/app/*\" {\n  capabilities = [\"create\", \"read\", \"update\", \"delete\"]\n}\n"

	s.methods = []vault.AuthMethod{
		{Path: "approle/", Type: "approle", Description: "machine auth"},
		{Path: "userpass/", Type: "userpass", Description: "humans"},
	}
	s.roles["approle/"] = []vault.Role{
		{Name: "db-writer", Policies: []string{"app-admin"}, Data: map[string]any{"token_ttl": "1h"}},
		{Name: "ci", Policies: []string{"app-prod-db"}, Data: map[string]any{"token_ttl": "20m"}},
	}
	s.roles["userpass/"] = []vault.Role{
		{Name: "lucas", Policies: []string{"app-admin", "default"}},
	}

	s.token = &vault.TokenInfo{
		Accessor:    "hmac-9f2c",
		DisplayName: "userpass-lucas",
		Policies:    []string{"app-admin", "default"},
		TTL:         7*time.Hour + 52*time.Minute,
		Renewable:   true,
		EntityID:    "entity-1",
		// ExpiresAt is deliberately left zero: a fixed timestamp would drift
		// into the past and make the header count down to nothing in tests. A
		// test that wants an expiring token sets it with SetToken.
	}
	return s
}

// PutVersions seeds a secret with a full history, oldest first.
func (s *Server) PutVersions(mount, path string, datas []map[string]any, start time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := &record{}
	for i, d := range datas {
		rec.versions = append(rec.versions, version{
			data:    d,
			created: start.Add(time.Duration(i) * time.Hour),
		})
	}
	// An empty history has no last version to date the record from, and the
	// arithmetic below would put the update an hour before the creation.
	updated := start
	if len(datas) > 0 {
		updated = start.Add(time.Duration(len(datas)-1) * time.Hour)
	}
	rec.metadata = vault.Metadata{
		CurrentVersion: len(datas),
		OldestVersion:  1,
		MaxVersions:    10,
		CreatedTime:    start,
		UpdatedTime:    updated,
	}
	s.secrets[key(mount, path)] = rec
}

// DeleteVersion soft-deletes one version of a seeded secret.
func (s *Server) DeleteVersion(mount, path string, v int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.secrets[key(mount, path)]; ok && v >= 1 && v <= len(rec.versions) {
		rec.versions[v-1].deletedAt = time.Now()
	}
}

// DestroyVersion permanently destroys one version of a seeded secret.
func (s *Server) DestroyVersion(mount, path string, v int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.secrets[key(mount, path)]; ok && v >= 1 && v <= len(rec.versions) {
		rec.versions[v-1].destroyed = true
		rec.versions[v-1].data = nil
	}
}

// SetToken replaces the token this server reports.
func (s *Server) SetToken(info *vault.TokenInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = info
}

// SetPolicy adds or replaces a policy document, so a test can decide what the
// Policies tab is asked to render.
func (s *Server) SetPolicy(name, hcl string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[name] = hcl
}

func key(mount, path string) string {
	return strings.Trim(mount, "/") + "/" + strings.Trim(path, "/")
}

func (s *Server) count(op string) {
	if s.Calls == nil {
		s.Calls = map[string]int{}
	}
	s.Calls[op]++
}

// denied reports whether a path is inside a subtree the token may not reach.
func (s *Server) denied(mount, path string) bool {
	full := key(mount, path)
	for _, prefix := range s.Denied {
		if full == prefix || strings.HasPrefix(full, strings.TrimSuffix(prefix, "/")+"/") {
			return true
		}
	}
	return false
}

func (s *Server) mount(name string) (vault.Mount, bool) {
	name = strings.Trim(name, "/")
	for _, m := range s.mounts {
		if strings.Trim(m.Path, "/") == name {
			return m, true
		}
	}
	return vault.Mount{}, false
}

func forbidden(op, mount, path string) error {
	return &vault.Error{Op: op, Path: key(mount, path), Kind: vault.ErrForbidden, Detail: "permission denied"}
}

func notFound(op, mount, path string) error {
	return &vault.Error{Op: op, Path: key(mount, path), Kind: vault.ErrNotFound}
}

func unsupported(op, mount, path string) error {
	return &vault.Error{Op: op, Path: key(mount, path), Kind: vault.ErrUnsupported}
}

// Server implements vault.Client.

func (s *Server) Server() vault.ServerInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.server
}

func (s *Server) Mounts(_ context.Context) ([]vault.Mount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("mounts")
	out := make([]vault.Mount, len(s.mounts))
	copy(out, s.mounts)
	return out, nil
}

func (s *Server) List(_ context.Context, mount, path string) ([]vault.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("list")

	if _, ok := s.mount(mount); !ok {
		return nil, notFound("list", mount, path)
	}
	if s.denied(mount, path) {
		return nil, forbidden("list", mount, path)
	}

	prefix := key(mount, path)
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	seen := map[string]bool{}
	var entries []vault.Entry
	for full := range s.secrets {
		if !strings.HasPrefix(full, prefix) {
			continue
		}
		rest := strings.TrimPrefix(full, prefix)
		name, _, isDir := strings.Cut(rest, "/")
		if name == "" || seen[name+boolKey(isDir)] {
			continue
		}
		seen[name+boolKey(isDir)] = true
		entries = append(entries, vault.Entry{Name: name, IsDir: isDir})
	}

	slices.SortFunc(entries, vault.CompareEntries)
	return entries, nil
}

func boolKey(b bool) string {
	if b {
		return "/"
	}
	return ""
}

func (s *Server) Read(_ context.Context, mount, path string, v int) (*vault.Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("read")

	m, ok := s.mount(mount)
	if !ok {
		return nil, notFound("read", mount, path)
	}
	if s.denied(mount, path) {
		return nil, forbidden("read", mount, path)
	}
	rec, ok := s.secrets[key(mount, path)]
	if !ok {
		return nil, notFound("read", mount, path)
	}

	if m.KVVersion == vault.KV1 {
		if v != 0 {
			return nil, unsupported("read version", mount, path)
		}
		// A record with no versions has nothing to be the latest - what
		// PutVersions leaves on an empty history - and indexing its last element
		// panics in a fake whose job is to let tests reach odd shapes safely.
		if len(rec.versions) == 0 {
			return nil, notFound("read", mount, path)
		}
		latest := rec.versions[len(rec.versions)-1]
		return &vault.Secret{Mount: mount, Path: path, Data: clone(latest.data)}, nil
	}

	if v == 0 {
		v = rec.metadata.CurrentVersion
	}
	if v < 1 || v > len(rec.versions) {
		return nil, notFound("read", mount, path)
	}

	ver := rec.versions[v-1]
	// A deleted or destroyed version still exists in the history but has no
	// data to return, which Vault reports as a missing secret.
	if ver.destroyed || !ver.deletedAt.IsZero() {
		return nil, notFound("read", mount, path)
	}

	return &vault.Secret{
		Mount:       mount,
		Path:        path,
		Data:        clone(ver.data),
		Version:     v,
		CreatedTime: ver.created,
	}, nil
}

func (s *Server) Write(_ context.Context, mount, path string, data map[string]any, cas int) (*vault.Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("write")

	m, ok := s.mount(mount)
	if !ok {
		return nil, notFound("write", mount, path)
	}
	if s.denied(mount, path) {
		return nil, forbidden("write", mount, path)
	}

	k := key(mount, path)
	rec, exists := s.secrets[k]

	if m.KVVersion == vault.KV1 {
		// No check-and-set, so no history to protect - but cas=0 still means
		// "must not already exist", the same as on a versioned mount. The real
		// client reads before writing to honour that; the fake just looks.
		if cas == 0 && exists {
			return nil, &vault.Error{
				Op:     "write",
				Path:   k,
				Kind:   vault.ErrCAS,
				Detail: "the path already exists, and a KV v1 mount cannot create without overwriting it",
			}
		}
		// No history: the previous value is simply gone.
		s.secrets[k] = &record{versions: []version{{data: clone(data), created: time.Now()}}}
		return &vault.Secret{Mount: mount, Path: path, Data: clone(data)}, nil
	}

	current := 0
	if exists {
		current = rec.metadata.CurrentVersion
	}

	if cas != vault.CASDisabled && cas != current {
		return nil, &vault.Error{
			Op:     "write",
			Path:   k,
			Kind:   vault.ErrCAS,
			Detail: "check-and-set parameter did not match the current version",
		}
	}

	if !exists {
		rec = &record{metadata: vault.Metadata{MaxVersions: 10, OldestVersion: 1, CreatedTime: time.Now()}}
		s.secrets[k] = rec
	}

	now := time.Now()
	rec.versions = append(rec.versions, version{data: clone(data), created: now})
	rec.metadata.CurrentVersion = len(rec.versions)
	rec.metadata.UpdatedTime = now

	return &vault.Secret{
		Mount:       mount,
		Path:        path,
		Data:        clone(data),
		Version:     rec.metadata.CurrentVersion,
		CreatedTime: now,
	}, nil
}

func (s *Server) Versions(_ context.Context, mount, path string) ([]vault.VersionInfo, *vault.Metadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("versions")

	m, ok := s.mount(mount)
	if !ok {
		return nil, nil, notFound("list versions", mount, path)
	}
	if m.KVVersion != vault.KV2 {
		return nil, nil, unsupported("list versions", mount, path)
	}
	if s.denied(mount, path) {
		return nil, nil, forbidden("list versions", mount, path)
	}
	rec, ok := s.secrets[key(mount, path)]
	if !ok {
		return nil, nil, notFound("list versions", mount, path)
	}

	out := make([]vault.VersionInfo, 0, len(rec.versions))
	for i := len(rec.versions) - 1; i >= 0; i-- {
		v := rec.versions[i]
		out = append(out, vault.VersionInfo{
			Version:      i + 1,
			CreatedTime:  v.created,
			DeletionTime: v.deletedAt,
			Destroyed:    v.destroyed,
		})
	}

	md := rec.metadata
	return out, &md, nil
}

func (s *Server) Delete(_ context.Context, mount, path string, versions []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("delete")

	m, rec, err := s.lookup("delete", mount, path)
	if err != nil {
		return err
	}

	if m.KVVersion == vault.KV1 {
		delete(s.secrets, key(mount, path))
		return nil
	}

	if len(versions) == 0 {
		versions = []int{rec.metadata.CurrentVersion}
	}
	for _, v := range versions {
		if v >= 1 && v <= len(rec.versions) && !rec.versions[v-1].destroyed {
			rec.versions[v-1].deletedAt = time.Now()
		}
	}
	return nil
}

func (s *Server) Undelete(_ context.Context, mount, path string, versions []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("undelete")

	m, rec, err := s.lookup("undelete", mount, path)
	if err != nil {
		return err
	}
	if m.KVVersion != vault.KV2 {
		return unsupported("undelete", mount, path)
	}

	for _, v := range versions {
		// A destroyed version cannot come back; undelete silently does nothing
		// for it, as the server does.
		if v >= 1 && v <= len(rec.versions) && !rec.versions[v-1].destroyed {
			rec.versions[v-1].deletedAt = time.Time{}
		}
	}
	return nil
}

func (s *Server) Destroy(_ context.Context, mount, path string, versions []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("destroy")

	m, rec, err := s.lookup("destroy", mount, path)
	if err != nil {
		return err
	}
	if m.KVVersion != vault.KV2 {
		return unsupported("destroy", mount, path)
	}

	for _, v := range versions {
		if v >= 1 && v <= len(rec.versions) {
			rec.versions[v-1].destroyed = true
			rec.versions[v-1].data = nil
		}
	}
	return nil
}

func (s *Server) DeleteMetadata(_ context.Context, mount, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("delete_metadata")

	m, _, err := s.lookup("delete metadata", mount, path)
	if err != nil {
		return err
	}
	if m.KVVersion != vault.KV2 {
		return unsupported("delete metadata", mount, path)
	}

	delete(s.secrets, key(mount, path))
	return nil
}

// lookup resolves a mount and secret, applying the denial rules. Callers hold
// the lock.
func (s *Server) lookup(op, mount, path string) (vault.Mount, *record, error) {
	m, ok := s.mount(mount)
	if !ok {
		return vault.Mount{}, nil, notFound(op, mount, path)
	}
	if s.denied(mount, path) {
		return vault.Mount{}, nil, forbidden(op, mount, path)
	}
	rec, ok := s.secrets[key(mount, path)]
	if !ok {
		return vault.Mount{}, nil, notFound(op, mount, path)
	}
	return m, rec, nil
}

func (s *Server) Policies(_ context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("policies")

	names := make([]string, 0, len(s.policies))
	for name := range s.policies {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

func (s *Server) Policy(_ context.Context, name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("policy")

	hcl, ok := s.policies[name]
	if !ok {
		return "", &vault.Error{Op: "read policy", Path: name, Kind: vault.ErrNotFound}
	}
	return hcl, nil
}

func (s *Server) AuthMethods(_ context.Context) ([]vault.AuthMethod, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("auth_methods")

	out := make([]vault.AuthMethod, len(s.methods))
	copy(out, s.methods)
	return out, nil
}

func (s *Server) AuthRoles(_ context.Context, method vault.AuthMethod) ([]vault.Role, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("auth_roles")

	roles := s.roles[method.Path]
	out := make([]vault.Role, len(roles))
	copy(out, roles)
	return out, nil
}

func (s *Server) TokenInfo(_ context.Context) (*vault.TokenInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("token_info")

	if s.token == nil {
		return nil, &vault.Error{
			Op:   "lookup token",
			Path: "auth/token/lookup-self",
			Kind: vault.ErrTokenInvalid,
		}
	}
	info := *s.token
	return &info, nil
}

// RenewToken pushes the expiry out by the token's own TTL, which is enough for
// the TUI to assert that renewal moved it.
func (s *Server) RenewToken(_ context.Context) (*vault.TokenInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("renew_token")

	if s.token == nil {
		return nil, &vault.Error{
			Op:   "renew token",
			Path: "auth/token/renew-self",
			Kind: vault.ErrTokenInvalid,
		}
	}
	if !s.token.Renewable {
		return nil, &vault.Error{
			Op:   "renew token",
			Path: "auth/token/renew-self",
			Kind: vault.ErrUnsupported,
		}
	}

	s.token.ExpiresAt = time.Now().Add(s.token.TTL)
	info := *s.token
	return &info, nil
}

func clone(m map[string]any) map[string]any {
	return maps.Clone(m)
}

// Health answers from whatever SetServer was given, since the fake has no node
// to be a standby of. It exists so the interface is satisfied and so a test can
// put the browser in front of a sealed or standby server on purpose.
func (s *Server) Health(_ context.Context) (vault.ServerInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("health")
	return s.server, nil
}

// SetServer replaces what this server reports about itself.
func (s *Server) SetServer(info vault.ServerInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.server = info
}

// Capabilities answers from the denial rules the fake already models, so a test
// gets the same "denied" verdict here that it gets from a read.
func (s *Server) Capabilities(_ context.Context, path string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count("capabilities")

	// The raw path carries the v2 data/ or metadata/ segment; the denial
	// prefixes are written in browsing terms, so it comes back out here.
	browsing := strings.Replace(path, "/data/", "/", 1)
	browsing = strings.Replace(browsing, "/metadata/", "/", 1)

	mount, rest, _ := strings.Cut(browsing, "/")
	if s.denied(mount, rest) {
		return []string{"deny"}, nil
	}
	return []string{"create", "delete", "list", "read", "update"}, nil
}
