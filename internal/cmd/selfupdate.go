package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/lucasassuncao/vivi/internal/updater"
	"github.com/spf13/cobra"
)

// DefaultRepo is where self-update looks for releases. The release build sets
// it via ldflags; the value here is what a "go install" build has, and it is
// this repository because that is where such a build came from.
var DefaultRepo = "lucasassuncao/vivi"

// SelfUpdateCmd returns the self-update command.
func SelfUpdateCmd(currentVersion string) *cobra.Command {
	var (
		repo            string
		version         string
		list            bool
		prerelease      bool
		allowUnverified bool
		limit           int
	)

	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Update vivi to a GitHub release",
		Long: `Downloads a release of vivi from GitHub and replaces the current binary.
The old binary is kept alongside the new one with a .old suffix until the next run.

With no flags, installs the latest stable release.
Use --list to see available versions, --version to pick a specific one,
and --prerelease to include rc/beta/alpha releases.`,
		Example: `  vivi self-update
  vivi self-update --list
  vivi self-update --list --prerelease
  vivi self-update --version v1.2.0
  vivi self-update --repo lucasassuncao/vivi`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if repo == "" {
				return errors.New("no repository configured: use --repo or build with -ldflags \"-X 'github.com/lucasassuncao/vivi/internal/cmd.DefaultRepo=owner/repo'\"")
			}
			// The context is the one Execute put a signal handler on, so Ctrl-C
			// ends a download instead of waiting out its five-minute ceiling.
			if list {
				return runSelfUpdateList(cmd.Context(), cmd.OutOrStdout(), repo, prerelease, limit, currentVersion)
			}
			return updater.SelfUpdate(cmd.Context(), updater.UpdateOptions{
				Repo:              repo,
				CurrentVersion:    currentVersion,
				Version:           version,
				IncludePrerelease: prerelease,
				AllowUnverified:   allowUnverified,
				Out:               cmd.OutOrStdout(),
				Err:               cmd.ErrOrStderr(),
			})
		},
	}

	cmd.Flags().StringVar(&repo, "repo", DefaultRepo, `GitHub repository in "owner/repo" format`)
	cmd.Flags().StringVar(&version, "version", "", "Install this specific release tag (e.g. v1.2.0) instead of the latest")
	cmd.Flags().BoolVar(&list, "list", false, "List available releases and exit")
	cmd.Flags().BoolVar(&prerelease, "prerelease", false, "Include prereleases (rc/beta/alpha) in --list, or as the latest target when no --version is given")
	cmd.Flags().BoolVar(&allowUnverified, "allow-unverified", false, "Install even when the release publishes no checksum to verify the download against")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of releases to show with --list (max 100)")

	return cmd
}

// listReleases is a variable for the same reason cleanOldBinary is: the fetch
// has its own tests behind a fake server, and this command's job is what it
// does with the answer.
var listReleases = updater.ListReleases

func runSelfUpdateList(ctx context.Context, out io.Writer, repo string, includePrerelease bool, limit int, currentVersion string) error {
	releases, err := listReleases(ctx, repo, includePrerelease, limit)
	if err != nil {
		return err
	}
	return formatReleases(out, releases, currentVersion)
}

// formatReleases renders the --list table, separate from the fetch so it can be
// tested without a network. "installed" uses the same normalisation SelfUpdate
// compares with, or the list and the update disagree.
func formatReleases(out io.Writer, releases []updater.Release, currentVersion string) error {
	if len(releases) == 0 {
		_, err := fmt.Fprintln(out, "No releases found.")
		return err
	}

	current := strings.TrimPrefix(currentVersion, "v")
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	// "latest" is what GitHub means by it: the newest stable release, which
	// with --prerelease is not necessarily the first row.
	latestMarked := false
	for _, r := range releases {
		tags := make([]string, 0, 3)
		if !r.Prerelease && !latestMarked {
			tags = append(tags, "latest")
			latestMarked = true
		}
		if r.Prerelease {
			tags = append(tags, "prerelease")
		}
		if strings.TrimPrefix(r.Tag, "v") == current {
			tags = append(tags, "installed")
		}
		label := ""
		if len(tags) > 0 {
			label = "(" + strings.Join(tags, ", ") + ")"
		}
		published := ""
		if !r.PublishedAt.IsZero() {
			published = r.PublishedAt.Format(time.DateOnly)
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", r.Tag, label, published)
	}
	return tw.Flush()
}
