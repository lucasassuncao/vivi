package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lucasassuncao/vivi/internal/vault"
)

// TargetVersions is what a version operation applies to: everything the user
// marked, or the one under the cursor when nothing is marked. The fallback
// keeps the version list usable without ever learning that marking exists.
func TargetVersions(marked map[int]bool, cursor *vault.VersionInfo) []int {
	out := make([]int, 0, len(marked))
	for v, on := range marked {
		if on {
			out = append(out, v)
		}
	}
	if len(out) > 0 {
		slices.Sort(out)
		return out
	}

	if cursor == nil {
		return nil
	}
	return []int{cursor.Version}
}

// DescribeVersions names a set of versions for a confirmation modal. The
// current one is called out because destroying it is a different act from
// destroying history.
func DescribeVersions(versions []int, current int) string {
	if len(versions) == 0 {
		return "-"
	}

	parts := make([]string, 0, len(versions))
	for _, v := range versions {
		label := fmt.Sprintf("v%d", v)
		if v == current {
			label += " (current)"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, ", ")
}

// LastSegment is the name at the end of a path, which is what a typed
// confirmation asks for: a moment's attention on which secret this is, not a
// copy-paste exercise.
func LastSegment(path string) string {
	path = strings.TrimSuffix(path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}
