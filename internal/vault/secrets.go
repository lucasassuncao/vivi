package vault

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/hashicorp/vault/api"
)

// List returns the children of a path inside a mount. An empty result is not an
// error: Vault answers a listing of nothing with a nil secret, and a directory
// that exists and holds nothing is a normal state.
func (a *apiClient) List(ctx context.Context, mount, path string) ([]Entry, error) {
	version, err := a.kvVersion(ctx, mount)
	if err != nil {
		return nil, err
	}

	var reqPath string
	if version == KV2 {
		// A v2 mount lists through its metadata endpoint; the data endpoint
		// only answers reads.
		reqPath = JoinPath(mount, "metadata", path)
	} else {
		reqPath = JoinPath(mount, path)
	}

	sec, err := a.c.Logical().ListWithContext(ctx, reqPath)
	if err != nil {
		return nil, wrap("list", reqPath, err)
	}

	keys := listedKeys(sec)
	if keys == nil {
		return nil, nil
	}
	entries := make([]Entry, 0, len(keys))
	for _, name := range keys {
		entries = append(entries, Entry{
			Name:  strings.TrimSuffix(name, "/"),
			IsDir: strings.HasSuffix(name, "/"),
		})
	}

	slices.SortFunc(entries, CompareEntries)
	return entries, nil
}

// CompareEntries orders a listing by name, a directory ahead of the secret
// that shares its name. Exported so fake.Server lists in the same order.
func CompareEntries(a, b Entry) int {
	if c := strings.Compare(a.Name, b.Name); c != 0 {
		return c
	}
	switch {
	case a.IsDir == b.IsDir:
		return 0
	case a.IsDir:
		return -1
	default:
		return 1
	}
}

// Read returns one version of a secret.
func (a *apiClient) Read(ctx context.Context, mount, path string, version int) (*Secret, error) {
	kvVersion, err := a.kvVersion(ctx, mount)
	if err != nil {
		return nil, err
	}

	if kvVersion == KV1 {
		if version != 0 {
			return nil, &Error{Op: "read version", Path: JoinPath(mount, path), Kind: ErrUnsupported}
		}
		sec, err := a.kv1(mount).Get(ctx, path)
		if err != nil {
			return nil, wrap("read", JoinPath(mount, path), err)
		}
		if sec == nil || sec.Data == nil {
			return nil, &Error{Op: "read", Path: JoinPath(mount, path), Kind: ErrNotFound}
		}
		return &Secret{Mount: mount, Path: path, Data: sec.Data}, nil
	}

	var sec *api.KVSecret
	if version == 0 {
		sec, err = a.kv2(mount).Get(ctx, path)
	} else {
		sec, err = a.kv2(mount).GetVersion(ctx, path, version)
	}
	if err != nil {
		return nil, wrap("read", JoinPath(mount, path), err)
	}

	// A deleted or destroyed version is not an error to the server: 200 with a
	// null body. Left alone it renders as a secret with no keys, reading as
	// "this secret is empty" rather than "this version is gone".
	if sec == nil || sec.Data == nil {
		return nil, &Error{
			Op:     "read",
			Path:   JoinPath(mount, path),
			Kind:   ErrNotFound,
			Detail: "this version has no data: it was deleted or destroyed",
		}
	}
	return newSecret(mount, path, sec.Data, sec.VersionMetadata), nil
}

// newSecret is a v2 answer as the interface promises it: the version fields
// come from the metadata, which a write may answer without.
func newSecret(mount, path string, data map[string]any, md *api.KVVersionMetadata) *Secret {
	out := &Secret{Mount: mount, Path: path, Data: data}
	if md != nil {
		out.Version = md.Version
		out.CreatedTime = md.CreatedTime
	}
	return out
}

// kv1 and kv2 are the SDK's engine clients for a mount, named the one way the
// SDK accepts.
func (a *apiClient) kv1(mount string) *api.KVv1 { return a.c.KVv1(normalizeMount(mount)) }
func (a *apiClient) kv2(mount string) *api.KVv2 { return a.c.KVv2(normalizeMount(mount)) }

// listedKeys pulls the names out of a LIST answer, or nil when there is no list
// in it: Vault answers a listing of nothing with a nil secret.
func listedKeys(sec *api.Secret) []string {
	if sec == nil || sec.Data == nil {
		return nil
	}
	raw, ok := sec.Data["keys"].([]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(raw))
	for _, k := range raw {
		if name, ok := k.(string); ok {
			keys = append(keys, name)
		}
	}
	return keys
}

// Write stores data as a new version and returns what the server recorded.
func (a *apiClient) Write(ctx context.Context, mount, path string, data map[string]any, cas int) (*Secret, error) {
	kvVersion, err := a.kvVersion(ctx, mount)
	if err != nil {
		return nil, err
	}

	if kvVersion == KV1 {
		// v1 has no check-and-set and no history: a write overwrites. cas == 0
		// still means "must not already exist", so read first and refuse. Not
		// atomic - nothing here can be - but a race is smaller than a clobber.
		if cas == 0 {
			if err := a.mustNotExist(ctx, mount, path); err != nil {
				return nil, err
			}
		}
		if err := a.kv1(mount).Put(ctx, path, data); err != nil {
			return nil, wrap("write", JoinPath(mount, path), err)
		}
		return &Secret{Mount: mount, Path: path, Data: data}, nil
	}

	var opts []api.KVOption
	if cas != CASDisabled {
		opts = append(opts, api.WithCheckAndSet(cas))
	}

	sec, err := a.kv2(mount).Put(ctx, path, data, opts...)
	if err != nil {
		return nil, wrap("write", JoinPath(mount, path), err)
	}
	if sec == nil {
		return &Secret{Mount: mount, Path: path, Data: data}, nil
	}
	return newSecret(mount, path, data, sec.VersionMetadata), nil
}

// mustNotExist reports ErrCAS when a v1 path already holds a secret, giving
// cas=0 the meaning it has on a versioned mount. Callers treat ErrCAS as "the
// server moved underneath you", which is exactly what happened.
func (a *apiClient) mustNotExist(ctx context.Context, mount, path string) error {
	sec, err := a.kv1(mount).Get(ctx, path)
	if err != nil {
		// Not found is the answer we want; anything else is the caller's to see.
		if errors.Is(err, api.ErrSecretNotFound) {
			return nil
		}
		return wrap("write", JoinPath(mount, path), err)
	}
	if sec == nil || sec.Data == nil {
		return nil
	}
	return &Error{
		Op:     "write",
		Path:   JoinPath(mount, path),
		Kind:   ErrCAS,
		Detail: "the path already exists, and a KV v1 mount cannot create without overwriting it",
	}
}

// Versions returns the history newest first, together with the secret's
// metadata. The two travel together because they come from one request and the
// detail panel shows them side by side.
func (a *apiClient) Versions(ctx context.Context, mount, path string) ([]VersionInfo, *Metadata, error) {
	if err := a.requireV2(ctx, mount, "list versions", path); err != nil {
		return nil, nil, err
	}

	md, err := a.kv2(mount).GetMetadata(ctx, path)
	if err != nil {
		return nil, nil, wrap("list versions", JoinPath(mount, path), err)
	}
	// The SDK's mapstructure extraction can return a nil document with no error
	// beside it. Reading through that would take the browser down over an
	// ordinary question about history.
	if md == nil {
		return nil, nil, &Error{
			Op:     "list versions",
			Path:   JoinPath(mount, path),
			Kind:   ErrNotFound,
			Detail: "the server returned no metadata for this secret",
		}
	}

	versions := make([]VersionInfo, 0, len(md.Versions))
	for _, v := range md.Versions {
		versions = append(versions, VersionInfo{
			Version:      v.Version,
			CreatedTime:  v.CreatedTime,
			DeletionTime: v.DeletionTime,
			Destroyed:    v.Destroyed,
		})
	}
	slices.SortFunc(versions, func(a, b VersionInfo) int { return cmp.Compare(b.Version, a.Version) })

	return versions, &Metadata{
		CASRequired:        md.CASRequired,
		MaxVersions:        md.MaxVersions,
		CurrentVersion:     md.CurrentVersion,
		OldestVersion:      md.OldestVersion,
		CreatedTime:        md.CreatedTime,
		UpdatedTime:        md.UpdatedTime,
		DeleteVersionAfter: md.DeleteVersionAfter,
		CustomMetadata:     md.CustomMetadata,
	}, nil
}

// Delete soft-deletes versions on a v2 mount, or removes the secret outright on
// a v1 mount, where nothing can bring it back.
func (a *apiClient) Delete(ctx context.Context, mount, path string, versions []int) error {
	kvVersion, err := a.kvVersion(ctx, mount)
	if err != nil {
		return err
	}

	if kvVersion == KV1 {
		return wrap("delete", JoinPath(mount, path), a.kv1(mount).Delete(ctx, path))
	}
	if len(versions) == 0 {
		return wrap("delete", JoinPath(mount, path), a.kv2(mount).Delete(ctx, path))
	}
	return wrap("delete versions", JoinPath(mount, path), a.kv2(mount).DeleteVersions(ctx, path, versions))
}

// Undelete restores soft-deleted versions.
func (a *apiClient) Undelete(ctx context.Context, mount, path string, versions []int) error {
	if err := a.requireV2(ctx, mount, "undelete", path); err != nil {
		return err
	}
	return wrap("undelete", JoinPath(mount, path), a.kv2(mount).Undelete(ctx, path, versions))
}

// Destroy permanently removes the data of specific versions. There is no undo.
func (a *apiClient) Destroy(ctx context.Context, mount, path string, versions []int) error {
	if err := a.requireV2(ctx, mount, "destroy", path); err != nil {
		return err
	}
	return wrap("destroy", JoinPath(mount, path), a.kv2(mount).Destroy(ctx, path, versions))
}

// DeleteMetadata removes the secret and every version of it. There is no undo.
func (a *apiClient) DeleteMetadata(ctx context.Context, mount, path string) error {
	if err := a.requireV2(ctx, mount, "delete metadata", path); err != nil {
		return err
	}
	return wrap("delete metadata", JoinPath(mount, path), a.kv2(mount).DeleteMetadata(ctx, path))
}

func (a *apiClient) requireV2(ctx context.Context, mount, op, path string) error {
	kvVersion, err := a.kvVersion(ctx, mount)
	if err != nil {
		return err
	}
	if kvVersion != KV2 {
		return &Error{Op: op, Path: JoinPath(mount, path), Kind: ErrUnsupported}
	}
	return nil
}
