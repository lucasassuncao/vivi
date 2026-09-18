package vault_test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// localVaultAddr reports whether addr points at a Vault on this machine. The
// suite creates and deletes paths, and the distance to a real server is one
// forgotten export. Matching is exact: "127.0.0.1.evil.com" is not localhost.
func localVaultAddr(addr string) bool {
	u, err := url.Parse(addr)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}

// These run against the throwaway Vault from docker-compose, the only way to
// check the request shapes the fake cannot verify. Start it with `make
// vault-up` and run them with `make test-integration`.
func integrationClient(t *testing.T) (vault.Client, vault.Config) {
	t.Helper()

	if os.Getenv("VIVI_INTEGRATION") == "" {
		t.Skip("set VIVI_INTEGRATION=1 and run docker compose up -d")
	}

	cfg := vault.ConfigFromEnv()
	if !localVaultAddr(cfg.Address) {
		t.Fatalf("refusing to run the integration suite against %q: it creates and "+
			"deletes paths, so it only runs against a localhost Vault", cfg.Address)
	}
	client, err := vault.New(cfg)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return client, cfg
}

func testCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestIntegrationMountsAreClassified(t *testing.T) {
	client, _ := integrationClient(t)

	mounts, err := client.Mounts(testCtx(t))
	if err != nil {
		t.Fatalf("mounts: %v", err)
	}

	want := map[string]int{
		"kv/":     vault.KV2,
		"legacy/": vault.KV1,
		"pki/":    vault.KVUnknown,
	}
	found := map[string]int{}
	for _, m := range mounts {
		found[m.Path] = m.KVVersion
	}

	for path, version := range want {
		got, ok := found[path]
		if !ok {
			t.Errorf("mount %s missing; run the seeder", path)
			continue
		}
		if got != version {
			t.Errorf("mount %s: got KV version %d, want %d", path, got, version)
		}
	}
}

func TestIntegrationListWalksAVersionedMount(t *testing.T) {
	client, _ := integrationClient(t)
	ctx := testCtx(t)

	// A v2 mount lists through metadata/, not data/. Getting this wrong yields
	// an empty tree with no error, which is why it is worth checking for real.
	entries, err := client.List(ctx, "kv", "")
	if err != nil {
		t.Fatalf("list kv/: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("kv/ listed empty; the v2 listing path is wrong")
	}

	var sawAppDir bool
	for _, e := range entries {
		if e.Name == "app" && e.IsDir {
			sawAppDir = true
		}
	}
	if !sawAppDir {
		t.Fatalf("expected an app/ directory, got %+v", entries)
	}

	deep, err := client.List(ctx, "kv", "app/prod")
	if err != nil {
		t.Fatalf("list kv/app/prod: %v", err)
	}
	if len(deep) < 3 {
		t.Fatalf("expected the prod secrets, got %+v", deep)
	}
}

// The history is built here rather than read from the seeded kv/app/prod/db:
// that path is shared with anyone running vivi by hand, and one extra write
// turns absolute version numbers into a failure nobody caused.
func TestIntegrationReadsVersionsAndTheirStates(t *testing.T) {
	client, _ := integrationClient(t)
	ctx := testCtx(t)
	path := scratchPath(t, client, "history")

	for _, password := range []string{"first", "second", "third", "fourth"} {
		data := map[string]any{"password": password + "-password"}
		if _, err := client.Write(ctx, "kv", path, data, vault.CASDisabled); err != nil {
			t.Fatalf("seeding %q: %v", password, err)
		}
	}
	if err := client.Delete(ctx, "kv", path, []int{2}); err != nil {
		t.Fatalf("delete v2: %v", err)
	}
	if err := client.Destroy(ctx, "kv", path, []int{1}); err != nil {
		t.Fatalf("destroy v1: %v", err)
	}

	versions, meta, err := client.Versions(ctx, "kv", path)
	if err != nil {
		t.Fatalf("versions: %v", err)
	}
	if len(versions) != 4 || meta.CurrentVersion != 4 {
		t.Fatalf("expected four versions with v4 current, got %d and current=%d",
			len(versions), meta.CurrentVersion)
	}
	if versions[0].Version != 4 {
		t.Fatalf("versions must come newest first, got %d", versions[0].Version)
	}

	byVersion := map[int]vault.VersionInfo{}
	for _, v := range versions {
		byVersion[v.Version] = v
	}
	if !byVersion[2].Deleted() {
		t.Error("v2 should be soft-deleted")
	}
	if !byVersion[1].Destroyed {
		t.Error("v1 should be destroyed")
	}
	if byVersion[1].Deleted() {
		t.Error("a destroyed version must not also report as merely deleted")
	}

	current, err := client.Read(ctx, "kv", path, 0)
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	if current.Version != 4 || current.Data["password"] != "fourth-password" {
		t.Fatalf("unexpected current version: %+v", current)
	}

	old, err := client.Read(ctx, "kv", path, 3)
	if err != nil {
		t.Fatalf("read v3: %v", err)
	}
	if old.Data["password"] != "third-password" {
		t.Fatalf("historical read returned the wrong version: %+v", old.Data)
	}

	// A destroyed version has no data, and the facade must call that not-found
	// rather than surfacing a raw SDK error.
	if _, err := client.Read(ctx, "kv", path, 1); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("reading a destroyed version: got %v, want ErrNotFound", err)
	}
}

func TestIntegrationCheckAndSetRejectsAStaleWrite(t *testing.T) {
	client, _ := integrationClient(t)
	ctx := testCtx(t)

	path := "integration/cas-" + time.Now().Format("150405.000")

	if _, err := client.Write(ctx, "kv", path, map[string]any{"n": "1"}, 0); err != nil {
		t.Fatalf("create with cas=0: %v", err)
	}
	// Creating the same path again with cas=0 must fail: it already exists.
	if _, err := client.Write(ctx, "kv", path, map[string]any{"n": "2"}, 0); !errors.Is(err, vault.ErrCAS) {
		t.Fatalf("recreating: got %v, want ErrCAS", err)
	}
	if _, err := client.Write(ctx, "kv", path, map[string]any{"n": "2"}, 1); err != nil {
		t.Fatalf("write with the right cas: %v", err)
	}
	// Writing against the version we already superseded is the concurrent-edit
	// case the editor depends on.
	if _, err := client.Write(ctx, "kv", path, map[string]any{"n": "3"}, 1); !errors.Is(err, vault.ErrCAS) {
		t.Fatalf("stale write: got %v, want ErrCAS", err)
	}

	if err := client.DeleteMetadata(ctx, "kv", path); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}

func TestIntegrationKV1HasNoHistory(t *testing.T) {
	client, _ := integrationClient(t)
	ctx := testCtx(t)

	sec, err := client.Read(ctx, "legacy", "old/app", 0)
	if err != nil {
		t.Fatalf("read v1 secret: %v", err)
	}
	if sec.Version != 0 || sec.Data["key"] != "no-history-here" {
		t.Fatalf("unexpected v1 secret: %+v", sec)
	}

	if _, _, err := client.Versions(ctx, "legacy", "old/app"); !errors.Is(err, vault.ErrUnsupported) {
		t.Fatalf("versions on v1: got %v, want ErrUnsupported", err)
	}
}

func TestIntegrationPoliciesAndAuthAreReadable(t *testing.T) {
	client, _ := integrationClient(t)
	ctx := testCtx(t)

	names, err := client.Policies(ctx)
	if err != nil {
		t.Fatalf("policies: %v", err)
	}
	var sawSeeded bool
	for _, n := range names {
		if n == "app-admin" {
			sawSeeded = true
		}
	}
	if !sawSeeded {
		t.Fatalf("expected the seeded policies, got %v", names)
	}

	hcl, err := client.Policy(ctx, "app-admin")
	if err != nil || hcl == "" {
		t.Fatalf("policy body: %q, %v", hcl, err)
	}

	methods, err := client.AuthMethods(ctx)
	if err != nil {
		t.Fatalf("auth methods: %v", err)
	}

	// The role endpoints differ per backend, and this is the only place that
	// mapping is checked against a real server.
	for _, meth := range methods {
		if meth.Type != "approle" {
			continue
		}
		roles, err := client.AuthRoles(ctx, meth)
		if err != nil {
			t.Fatalf("approle roles: %v", err)
		}
		if len(roles) != 2 {
			t.Fatalf("expected two approle roles, got %+v", roles)
		}
		for _, r := range roles {
			if len(r.Policies) == 0 {
				t.Errorf("role %s came back without its policies", r.Name)
			}
		}
	}
}

func TestIntegrationTokenDescribesItself(t *testing.T) {
	client, cfg := integrationClient(t)

	info, err := vault.Preflight(testCtx(t), cfg, client)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if len(info.Policies) == 0 {
		t.Fatal("the token should report its policies")
	}
}

// TokenInfo maps lookup-self by string key, so a renamed field is a zero value
// and not an error. ttl drives the header countdown and renewable decides what
// RenewToken reports, which is why both are checked against the real answer.
func TestIntegrationTokenInfoReadsTheRealLookup(t *testing.T) {
	client, cfg := integrationClient(t)
	ctx := testCtx(t)

	root, err := client.TokenInfo(ctx)
	if err != nil {
		t.Fatalf("root token info: %v", err)
	}
	if root.Accessor == "" {
		t.Error("root token came back without an accessor")
	}
	if !slices.Contains(root.Policies, "root") {
		t.Errorf("root token policies: got %v, want root among them", root.Policies)
	}
	if root.Renewable || root.TTL != 0 || !root.ExpiresAt.IsZero() {
		t.Errorf("a root token never expires and cannot be renewed, got %+v", root)
	}

	// A minted token is the other shape: it counts down, it can be renewed, and
	// it carries the name and the policies it was created with.
	var policy string
	cfg.Token, policy = limitedToken(t, cfg, `path "kv/data/app/*" { capabilities = ["read"] }`)
	limited, err := vault.New(cfg)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	info, err := limited.TokenInfo(ctx)
	if err != nil {
		t.Fatalf("limited token info: %v", err)
	}
	if info.DisplayName != "token-vivi-integration" {
		t.Errorf("display name: got %q", info.DisplayName)
	}
	if !slices.Contains(info.Policies, policy) {
		t.Errorf("policies: got %v, want %s among them", info.Policies, policy)
	}
	if !info.Renewable {
		t.Error("a minted token is renewable, and RenewToken relies on that field")
	}
	if info.TTL <= 0 || info.TTL > 10*time.Minute {
		t.Errorf("ttl: got %v, want within the 10m it was created with", info.TTL)
	}
	if !info.ExpiresAt.After(time.Now()) {
		t.Errorf("expires at %v is not in the future", info.ExpiresAt)
	}
}

// limitedToken mints a token carrying only the given policy, using the root
// token the suite runs as. Policy and token are both removed when the test ends,
// so no run depends on what the seeder printed.
func limitedToken(t *testing.T, cfg vault.Config, hcl string) (token, policy string) {
	t.Helper()

	apiCfg := api.DefaultConfig()
	if apiCfg.Error != nil {
		t.Fatalf("read Vault environment: %v", apiCfg.Error)
	}
	apiCfg.Address = cfg.Address
	root, err := api.NewClient(apiCfg)
	if err != nil {
		t.Fatalf("root client: %v", err)
	}
	root.SetToken(cfg.Token)

	ctx := testCtx(t)
	policy = "vivi-integration-" + time.Now().Format("150405.000")
	if err := root.Sys().PutPolicyWithContext(ctx, policy, hcl); err != nil {
		t.Fatalf("write policy %s: %v", policy, err)
	}
	t.Cleanup(func() {
		clean, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = root.Sys().DeletePolicyWithContext(clean, policy)
	})

	sec, err := root.Auth().Token().CreateWithContext(ctx, &api.TokenCreateRequest{
		Policies:    []string{policy},
		TTL:         "10m",
		DisplayName: "vivi-integration",
	})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	token = sec.Auth.ClientToken
	t.Cleanup(func() {
		clean, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = root.Auth().Token().RevokeTreeWithContext(clean, token)
	})
	return token, policy
}

// The policy behind the denial tests: enough to walk kv/app, and a subtree that
// is denied outright, the shape the tree renders as a locked node.
const deniedSubtreePolicy = `
path "sys/mounts"               { capabilities = ["read"] }
path "kv/metadata/app/*"        { capabilities = ["read", "list"] }
path "kv/data/app/*"            { capabilities = ["read"] }
path "kv/metadata/restricted/*" { capabilities = ["deny"] }
path "kv/data/restricted/*"     { capabilities = ["deny"] }
`

// A real denial takes a token that is denied something, and a root token never
// is. The test mints its own rather than waiting for one in the environment: a
// test that skips everywhere checks nothing.
func TestIntegrationDeniedPathIsForbidden(t *testing.T) {
	_, cfg := integrationClient(t)
	cfg.Token, _ = limitedToken(t, cfg, deniedSubtreePolicy)
	client, err := vault.New(cfg)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	ctx := testCtx(t)

	// The tree renders this as a locked node. If the facade reported it as
	// not-found instead, the subtree would silently look empty.
	if _, err := client.List(ctx, "kv", "restricted"); !errors.Is(err, vault.ErrForbidden) {
		t.Fatalf("listing a denied path: got %v, want ErrForbidden", err)
	}
	if _, err := client.Read(ctx, "kv", "restricted/root-credentials", 0); !errors.Is(err, vault.ErrForbidden) {
		t.Fatalf("reading a denied path: got %v, want ErrForbidden", err)
	}

	// The rest of the Vault stays reachable with the same token.
	if _, err := client.List(ctx, "kv", "app/prod"); err != nil {
		t.Fatalf("a denial must not affect other paths: %v", err)
	}
}

// The integration suite writes and deletes real paths. It must only ever be
// able to do that to the throwaway container on this machine.
func TestLocalVaultAddrGuard(t *testing.T) {
	local := []string{
		"http://127.0.0.1:8200",
		"http://localhost:8200",
		"https://localhost:8200",
		"http://[::1]:8200",
	}
	for _, addr := range local {
		if !localVaultAddr(addr) {
			t.Errorf("%s should be accepted as local", addr)
		}
	}

	remote := []string{
		"https://vault.corp:8200",
		"https://vault.internal.example.com",
		"http://10.0.0.5:8200",
		"http://127.0.0.1.evil.com:8200",
		"",
		"::::",
	}
	for _, addr := range remote {
		if localVaultAddr(addr) {
			t.Errorf("%s must be refused", addr)
		}
	}
}

// scratchPath is a kv/ path this test owns, removed whatever the test does to
// it. The cleanup builds its own context because the test's has almost
// certainly expired by the time it runs.
func scratchPath(t *testing.T, client vault.Client, name string) string {
	t.Helper()
	path := "integration/" + name
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = client.DeleteMetadata(ctx, "kv", path)
	})
	return path
}

// Capabilities is the one line the access panel presents as the server's own
// verdict rather than as an inference from policy documents. Until now nothing
// had ever sent the request.
func TestIntegrationCapabilitiesComeFromTheServer(t *testing.T) {
	client, _ := integrationClient(t)

	caps, err := client.Capabilities(testCtx(t), "kv/data/app/prod/db")
	if err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	if len(caps) == 0 {
		t.Fatal("the server named no capabilities at all: sys/capabilities-self was not reached")
	}

	// The suite runs as root, and Vault answers for a root token with the single
	// word "root" rather than with the crud list.
	var sawRoot bool
	for _, c := range caps {
		if c == "root" {
			sawRoot = true
		}
	}
	if !sawRoot {
		t.Fatalf("root token got %v, want root among them", caps)
	}
	if !sort.StringsAreSorted(caps) {
		t.Errorf("capabilities came back unsorted: %v", caps)
	}
}

// Health is documented to fill in the parts of ServerInfo only the server can
// answer, and to become what Server() reports from then on. The second half is
// the one a test has to pin: it is a write to shared state on the client.
func TestIntegrationHealthDescribesTheServerAndSticks(t *testing.T) {
	client, _ := integrationClient(t)

	info, err := client.Health(testCtx(t))
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if info.Version == "" {
		t.Error("health returned no version, so the header would say less than it could")
	}
	if info.Sealed {
		t.Error("the throwaway server reports itself sealed")
	}
	if got := client.Server(); got.Version != info.Version {
		t.Errorf("Server() reports version %q after Health() returned %q", got.Version, info.Version)
	}
}

// The interface says a non-renewable token fails with ErrUnsupported and
// fake.Server answers that way, but classify produces that kind for no status
// code. A root token is never renewable, so no provisioning is needed here.
func TestIntegrationRenewRefusesANonRenewableToken(t *testing.T) {
	client, _ := integrationClient(t)

	if _, err := client.RenewToken(testCtx(t)); !errors.Is(err, vault.ErrUnsupported) {
		t.Fatalf("renewing a root token: got %v, want ErrUnsupported", err)
	}
}

// Three different things Vault does to one version, with three request shapes.
// Destroy is the operation vivi gates hardest - app.GateFor makes the user type
// the secret's name - and its request had never been sent.
func TestIntegrationUndeleteAndDestroyMoveAVersionBetweenStates(t *testing.T) {
	client, _ := integrationClient(t)
	ctx := testCtx(t)
	path := scratchPath(t, client, "versions")

	for _, password := range []string{"one", "two"} {
		if _, err := client.Write(ctx, "kv", path, map[string]any{"password": password}, vault.CASDisabled); err != nil {
			t.Fatalf("seeding %q: %v", password, err)
		}
	}

	stateOfV1 := func(after string) vault.VersionInfo {
		t.Helper()
		versions, _, err := client.Versions(ctx, "kv", path)
		if err != nil {
			t.Fatalf("versions after %s: %v", after, err)
		}
		for _, v := range versions {
			if v.Version == 1 {
				return v
			}
		}
		t.Fatalf("v1 left the history after %s", after)
		return vault.VersionInfo{}
	}

	if err := client.Delete(ctx, "kv", path, []int{1}); err != nil {
		t.Fatalf("delete v1: %v", err)
	}
	if v := stateOfV1("delete"); !v.Deleted() {
		t.Fatalf("v1 should be soft-deleted, got %+v", v)
	}

	if err := client.Undelete(ctx, "kv", path, []int{1}); err != nil {
		t.Fatalf("undelete v1: %v", err)
	}
	if v := stateOfV1("undelete"); !v.Readable() {
		t.Fatalf("v1 should be readable again, got %+v", v)
	}

	if err := client.Destroy(ctx, "kv", path, []int{1}); err != nil {
		t.Fatalf("destroy v1: %v", err)
	}
	v := stateOfV1("destroy")
	if !v.Destroyed {
		t.Fatalf("v1 should be destroyed, got %+v", v)
	}
	if v.Deleted() {
		t.Error("a destroyed version must not also report as merely deleted")
	}
	if _, err := client.Read(ctx, "kv", path, 1); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("reading a destroyed version: got %v, want ErrNotFound", err)
	}
}

// cas=0 means "must not already exist" on every mount. A v1 mount has no
// check-and-set, so the client performs the guard itself with a read - and
// without it a create silently overwrites the only copy a v1 mount keeps.
func TestIntegrationKV1CreateRefusesToOverwrite(t *testing.T) {
	client, _ := integrationClient(t)
	ctx := testCtx(t)

	// Not scratchPath: DeleteMetadata is a v2 endpoint, and this is the v1 mount.
	path := "integration/create-guard"
	t.Cleanup(func() {
		clean, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = client.Delete(clean, "legacy", path, nil)
	})

	if _, err := client.Write(ctx, "legacy", path, map[string]any{"key": "first"}, 0); err != nil {
		t.Fatalf("creating on a v1 mount: %v", err)
	}

	if _, err := client.Write(ctx, "legacy", path, map[string]any{"key": "second"}, 0); !errors.Is(err, vault.ErrCAS) {
		t.Fatalf("recreating an existing v1 secret: got %v, want ErrCAS", err)
	}

	sec, err := client.Read(ctx, "legacy", path, 0)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if sec.Data["key"] != "first" {
		t.Fatalf("the refused write landed anyway: %+v", sec.Data)
	}
}
