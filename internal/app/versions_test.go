package app

import (
	"testing"

	"github.com/lucasassuncao/vivi/internal/vault"
)

// Marking is optional: the version list has to work for someone who never
// learns that space marks a row.
func TestVersionTargetsFallBackToTheCursor(t *testing.T) {
	cursor := &vault.VersionInfo{Version: 4}

	if got := TargetVersions(nil, cursor); len(got) != 1 || got[0] != 4 {
		t.Errorf("with nothing marked the cursor is the target: %v", got)
	}

	marked := map[int]bool{3: true, 1: true, 2: false}
	got := TargetVersions(marked, cursor)
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Errorf("marked versions win and come out sorted: %v", got)
	}

	if got := TargetVersions(map[int]bool{2: false}, nil); len(got) != 0 {
		t.Errorf("unmarking everything with no cursor targets nothing: %v", got)
	}
}

// A list of bare numbers does not say which one is live, and destroying the
// current version is a different act from destroying history.
func TestDescribeCallsOutTheCurrentVersion(t *testing.T) {
	got := DescribeVersions([]int{2, 4}, 4)
	if got != "v2, v4 (current)" {
		t.Errorf("describe = %q", got)
	}
	if got := DescribeVersions(nil, 4); got != "-" {
		t.Errorf("nothing selected should read as %q, got %q", "-", got)
	}
}

func TestLastSegmentIsWhatAConfirmationAsksFor(t *testing.T) {
	for path, want := range map[string]string{
		"kv/app/prod/db": "db",
		"db":             "db",
		"kv/app/prod/":   "prod",
		"":               "",
	} {
		if got := LastSegment(path); got != want {
			t.Errorf("LastSegment(%q) = %q, want %q", path, got, want)
		}
	}
}
