package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lucasassuncao/vivi/internal/updater"
)

// The --list table is the half that has ever been wrong and the half that needs
// no network: "installed" has to use the same normalisation SelfUpdate compares
// with, or the list says one thing and the update does another.
func TestFormatReleasesMarksTheInstalledRelease(t *testing.T) {
	releases := []updater.Release{
		{Tag: "v2.0.0", PublishedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)},
		{Tag: "v1.9.0-rc1", Prerelease: true},
		{Tag: "v1.8.0"},
	}

	// The running version carries its "v" in some builds and not others, so the
	// comparison has to see through that: a table marking nothing sends the user
	// to reinstall what they have, and one marking the wrong row is worse.
	var out strings.Builder
	if err := formatReleases(&out, releases, "1.8.0"); err != nil {
		t.Fatalf("format: %v", err)
	}

	// Asserted per line, because the column widths are tabwriter's to choose
	// and a test that pinned the spacing would fail on a longer tag.
	byTag := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			byTag[fields[0]] = line
		}
	}

	for tag, want := range map[string]string{
		"v2.0.0":     "(latest)",
		"v1.9.0-rc1": "(prerelease)",
		"v1.8.0":     "(installed)",
	} {
		line, ok := byTag[tag]
		if !ok {
			t.Errorf("%s is not in the table:\n%s", tag, out.String())
			continue
		}
		if !strings.Contains(line, want) {
			t.Errorf("%s is labelled %q, want %q", tag, line, want)
		}
	}

	if strings.Contains(byTag["v2.0.0"], "installed") {
		t.Errorf("the wrong release is marked installed: %q", byTag["v2.0.0"])
	}
	if !strings.Contains(byTag["v2.0.0"], "2026-09-01") {
		t.Errorf("the publication date is missing: %q", byTag["v2.0.0"])
	}
}

// With --prerelease the newest row is often a release candidate. "latest" is
// GitHub's word for the newest stable one, and it used to be pinned to row
// zero, so a list led by an rc marked nothing as latest at all.
func TestFormatReleasesMarksLatestBelowAPrerelease(t *testing.T) {
	releases := []updater.Release{
		{Tag: "v2.1.0-rc1", Prerelease: true},
		{Tag: "v2.0.0"},
		{Tag: "v1.9.0"},
	}
	var out strings.Builder
	if err := formatReleases(&out, releases, "1.9.0"); err != nil {
		t.Fatalf("format: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("want three rows, got:\n%s", out.String())
	}
	if strings.Contains(lines[0], "latest") {
		t.Errorf("an rc must not be marked latest: %q", lines[0])
	}
	if !strings.Contains(lines[1], "(latest)") {
		t.Errorf("the newest stable release must be marked latest: %q", lines[1])
	}
	if strings.Contains(lines[2], "latest") {
		t.Errorf("only one row is latest: %q", lines[2])
	}
}

func TestFormatReleasesSaysWhenThereAreNone(t *testing.T) {
	var out strings.Builder
	if err := formatReleases(&out, nil, "1.0.0"); err != nil {
		t.Fatalf("format: %v", err)
	}
	if !strings.Contains(out.String(), "No releases") {
		t.Errorf("an empty list said nothing:\n%s", out.String())
	}
}

// --list passes its flags through to the fetch unchanged and prints what comes
// back; a fetch that fails is the command's own failure, with nothing printed.
func TestSelfUpdateListFetchesWithItsFlagsAndPrints(t *testing.T) {
	original := listReleases
	t.Cleanup(func() { listReleases = original })

	var gotRepo string
	var gotPrerelease bool
	var gotLimit int
	listReleases = func(_ context.Context, repo string, prerelease bool, limit int) ([]updater.Release, error) {
		gotRepo, gotPrerelease, gotLimit = repo, prerelease, limit
		return []updater.Release{{Tag: "v1.1.0"}, {Tag: "v1.0.0"}}, nil
	}

	var out strings.Builder
	if err := runSelfUpdateList(context.Background(), &out, "owner/repo", true, 7, "1.0.0"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if gotRepo != "owner/repo" || !gotPrerelease || gotLimit != 7 {
		t.Errorf("fetched with repo=%q prerelease=%v limit=%d", gotRepo, gotPrerelease, gotLimit)
	}
	if !strings.Contains(out.String(), "v1.1.0") || !strings.Contains(out.String(), "installed") {
		t.Errorf("the table is missing rows or the installed mark:\n%s", out.String())
	}

	listReleases = func(context.Context, string, bool, int) ([]updater.Release, error) {
		return nil, errors.New("github api returned 503")
	}
	out.Reset()
	err := runSelfUpdateList(context.Background(), &out, "owner/repo", false, 20, "1.0.0")
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("a failed fetch must surface as the command's error, got %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("nothing should be printed when the fetch failed, got %q", out.String())
	}
}

// The README offers "go install" first, and that build carries no ldflags: the
// repository has to be known without them, or the first thing a new user runs
// answers "no repository configured".
func TestSelfUpdateKnowsItsRepositoryWithoutLdflags(t *testing.T) {
	if DefaultRepo == "" {
		t.Fatal("DefaultRepo is empty, so a go install build cannot self-update")
	}

	cmd := SelfUpdateCmd("dev")
	flag := cmd.Flags().Lookup("repo")
	if flag == nil {
		t.Fatal("self-update has no --repo flag")
	}
	if flag.DefValue != DefaultRepo {
		t.Errorf("--repo defaults to %q, want %q", flag.DefValue, DefaultRepo)
	}
}
