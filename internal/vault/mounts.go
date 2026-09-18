package vault

import (
	"context"
	"slices"
	"strings"
)

// Mounts reads the mount table and remembers each mount's KV version.
func (a *apiClient) Mounts(ctx context.Context) ([]Mount, error) {
	table, err := a.c.Sys().ListMountsWithContext(ctx)
	if err != nil {
		return nil, wrap("list mounts", "sys/mounts", err)
	}

	mounts := make([]Mount, 0, len(table))
	kinds := make(map[string]int, len(table))

	for path, out := range table {
		if out == nil {
			// The mount table is the server's answer, not ours. A nil entry
			// would be strange, and reading through it would take the whole
			// browser down before it has drawn a single row.
			continue
		}
		m := Mount{
			Path:        path,
			Type:        out.Type,
			Description: out.Description,
			Accessor:    out.Accessor,
			KVVersion:   kvVersionOf(out.Type, out.Options),
		}
		mounts = append(mounts, m)
		kinds[normalizeMount(path)] = m.KVVersion
	}

	slices.SortFunc(mounts, func(a, b Mount) int { return strings.Compare(a.Path, b.Path) })

	a.mu.Lock()
	a.kinds = kinds
	a.mu.Unlock()

	return mounts, nil
}

// kvVersionOf decides how a mount must be addressed. v2 reports type "kv" with
// options.version == "2", v1 the same type with it absent or "1". cubbyhole
// speaks the v1 shape and is browsable, just per-token.
func kvVersionOf(mountType string, options map[string]string) int {
	switch mountType {
	case "kv", "generic":
		if options["version"] == "2" {
			return KV2
		}
		return KV1
	case "cubbyhole":
		return KV1
	default:
		return KVUnknown
	}
}
