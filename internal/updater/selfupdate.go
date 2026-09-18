// Package updater implements the self-update mechanism for vivi.
package updater

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// apiClient handles short metadata requests against the GitHub API.
// downloadClient covers the larger binary fetch and uses a longer ceiling.
// Both have timeouts so a slow or hung server cannot stall the CLI indefinitely.
var (
	apiClient      = &http.Client{Timeout: 30 * time.Second}
	downloadClient = &http.Client{Timeout: 5 * time.Minute}
)

// osExecutable is a variable so tests can override os.Executable.
var osExecutable = os.Executable

// Release is the public, presentation-friendly view of a GitHub release
// returned by ListReleases. It hides API-specific fields.
type Release struct {
	Tag         string
	Prerelease  bool
	PublishedAt time.Time
}

// UpdateOptions selects a release and decides how strict the install is. A
// struct and not a parameter list because SelfUpdate(repo, "", v, "", false,
// true) gives no reader a way to tell which guard was just turned off.
type UpdateOptions struct {
	// Repo is "owner/repo", e.g. "lucasassuncao/vivi".
	Repo string
	// CurrentVersion is the running binary's version ("1.0.0" or "v1.0.0").
	// The update is skipped when it matches the resolved tag.
	CurrentVersion string
	// Version selects the release to install. Empty means latest.
	Version string
	// IncludePrerelease only affects the empty-Version path: when true, the most
	// recent release wins even if it is a prerelease. An explicit Version is
	// always honoured, prerelease or not.
	IncludePrerelease bool
	// AllowUnverified installs a binary whose checksum could not be found or
	// fetched. Off by default; see verifyDownloadedAsset for why.
	AllowUnverified bool

	// Out is where progress goes and Err where warnings go. Both default to the
	// process's own streams when nil; the CLI passes cobra's, which is what
	// makes the output redirectable by the user and assertable in a test.
	Out io.Writer
	Err io.Writer
}

func (o UpdateOptions) out() io.Writer {
	if o.Out != nil {
		return o.Out
	}
	return os.Stdout
}

func (o UpdateOptions) errOut() io.Writer {
	if o.Err != nil {
		return o.Err
	}
	return os.Stderr
}

// SelfUpdate downloads a release from GitHub and replaces the current binary,
// keeping the old one as <name>.old until the next run. The download is
// verified against the published checksum unless opts.AllowUnverified says so.
func SelfUpdate(ctx context.Context, opts UpdateOptions) error {
	if opts.Repo == "" {
		return errors.New("--repo is required (e.g. --repo lucasassuncao/vivi)")
	}

	// Clean up any leftover .old binary from a previous update.
	CleanOldBinary(opts.errOut())

	rel, err := resolveRelease(ctx, opts.out(), opts.Repo, opts.Version, opts.IncludePrerelease)
	if err != nil {
		return err
	}

	// Normalise both versions to a bare "X.Y.Z" form before comparing.
	if normalizeVersion(opts.CurrentVersion) == normalizeVersion(rel.TagName) {
		fmt.Fprintf(opts.out(), "Already on %s.\n", rel.TagName)
		return nil
	}

	asset := selectAsset(rel.Assets)
	if asset == nil {
		return fmt.Errorf("no compatible binary found in release %s", rel.TagName)
	}

	fmt.Fprintf(opts.out(), "Found %s → %s (%.1f MB)\n", rel.TagName, asset.Name, float64(asset.Size)/1e6)

	exePath, err := osExecutable()
	if err != nil {
		return fmt.Errorf("cannot determine current executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	fmt.Fprintln(opts.out(), "Downloading new binary...")
	tmpPath := exePath + ".new"
	if err := download(ctx, asset.BrowserDownloadURL, tmpPath, asset.Size); err != nil {
		return err
	}

	if err := verifyDownloadedAsset(ctx, opts.out(), opts.errOut(), rel.Assets, asset, tmpPath, opts.AllowUnverified); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	// On Windows we cannot delete the running binary, but we can rename it.
	// A stale .old that will not go (another vivi still running from it, an
	// antivirus holding it) would also refuse the rename onto it, so the backup
	// takes a fresh name instead; CleanOldBinary sweeps every .old* later.
	oldPath := exePath + ".old"
	if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(opts.errOut(), "warning: could not remove stale binary %s: %v\n", oldPath, err)
		oldPath = fmt.Sprintf("%s.old.%d", exePath, time.Now().UnixNano())
	}

	fmt.Fprintln(opts.out(), "Replacing binary...")
	if err := os.Rename(exePath, oldPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming current binary: %w", err)
	}
	if err := os.Rename(tmpPath, exePath); err != nil {
		// Rollback: try to restore the previous binary.
		if rbErr := os.Rename(oldPath, exePath); rbErr != nil {
			fmt.Fprintf(opts.errOut(),
				"CRITICAL: install failed and rollback failed too. Original binary is at %s, downloaded binary at %s. Restore manually. Rollback error: %v\n",
				oldPath, tmpPath, rbErr)
		} else {
			_ = os.Remove(tmpPath)
		}
		return fmt.Errorf("installing new binary: %w", err)
	}

	fmt.Fprintf(opts.out(), "✓ Installed %s  (old binary saved as %s)\n", rel.TagName, filepath.Base(oldPath))
	return nil
}

// ListReleases returns up to `limit` recent releases for the repo, newest first.
// Drafts are always excluded. When includePrerelease is false, prereleases are
// also excluded. limit <= 0 defaults to 20; the GitHub per-page cap is 100.
func ListReleases(ctx context.Context, repo string, includePrerelease bool, limit int) ([]Release, error) {
	if repo == "" {
		return nil, errors.New("repo is required (e.g. lucasassuncao/vivi)")
	}
	if limit <= 0 {
		limit = 20
	}

	// Over-fetch a little so filtering prereleases still gives us a usable list.
	perPage := limit
	if !includePrerelease {
		perPage *= 2
	}
	perPage = min(perPage, 100)

	raw, err := fetchReleases(ctx, repo, perPage)
	if err != nil {
		return nil, err
	}

	out := make([]Release, 0, len(raw))
	for i := range raw {
		if raw[i].Draft {
			continue
		}
		if raw[i].Prerelease && !includePrerelease {
			continue
		}
		out = append(out, Release{
			Tag:         raw[i].TagName,
			Prerelease:  raw[i].Prerelease,
			PublishedAt: raw[i].PublishedAt,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// resolveRelease maps the (version, includePrerelease) inputs to a concrete release.
func resolveRelease(ctx context.Context, out io.Writer, repo, version string, includePrerelease bool) (*ghRelease, error) {
	if version != "" {
		fmt.Fprintf(out, "Fetching release %s from %s...\n", version, repo)
		rel, err := fetchReleaseByTag(ctx, repo, version)
		// A bare "1.2.0" is tried as "v1.2.0" too; the first error is the one
		// reported when neither exists.
		if err != nil && !strings.HasPrefix(version, "v") {
			if alt, altErr := fetchReleaseByTag(ctx, repo, "v"+version); altErr == nil {
				return alt, nil
			}
		}
		return rel, err
	}

	if includePrerelease {
		fmt.Fprintf(out, "Checking most recent release of %s (including prereleases)...\n", repo)
		all, err := fetchReleases(ctx, repo, 10)
		if err != nil {
			return nil, err
		}
		for i := range all {
			if !all[i].Draft {
				return &all[i], nil
			}
		}
		return nil, fmt.Errorf("no releases found for %s", repo)
	}

	fmt.Fprintf(out, "Checking latest release of %s...\n", repo)
	return fetchLatestRelease(ctx, repo)
}

// CleanOldBinary removes a <exe>.old left by a previous self-update. cmd.Execute
// calls it at startup, which is what "kept until the next run" means in
// self-update's help; SelfUpdate calls it before creating a new one.
func CleanOldBinary(errOut io.Writer) {
	exe, err := osExecutable()
	if err != nil {
		return
	}
	// .old, and the .old.<stamp> names SelfUpdate falls back to when .old itself
	// would not go. A directory listing rather than a glob: the executable's own
	// path may carry glob metacharacters.
	dir, base := filepath.Split(exe)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), base+".old") {
			continue
		}
		old := filepath.Join(dir, e.Name())
		if err := os.Remove(old); err != nil {
			fmt.Fprintf(errOut, "warning: could not remove old binary %s: %v\n", old, err)
		}
	}
}

// normalizeVersion strips a leading "v" so that "v1.2.3" and "1.2.3" compare equal.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func newGitHubRequest(ctx context.Context, method, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "github.com/lucasassuncao/vivi")
	return req, nil
}

func fetchLatestRelease(ctx context.Context, repo string) (*ghRelease, error) {
	return fetchRelease(ctx, fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo))
}

func fetchReleaseByTag(ctx context.Context, repo, tag string) (*ghRelease, error) {
	return fetchRelease(ctx, fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, url.PathEscape(tag)))
}

func fetchRelease(ctx context.Context, rawURL string) (*ghRelease, error) {
	var rel ghRelease
	if err := getJSON(ctx, rawURL, &rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func fetchReleases(ctx context.Context, repo string, perPage int) ([]ghRelease, error) {
	u := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=%d", repo, perPage)
	var rels []ghRelease
	if err := getJSON(ctx, u, &rels); err != nil {
		return nil, err
	}
	return rels, nil
}

func getJSON(ctx context.Context, rawURL string, out any) error {
	req, err := newGitHubRequest(ctx, http.MethodGet, rawURL)
	if err != nil {
		return err
	}
	resp, err := apiClient.Do(req)
	if err != nil {
		return fmt.Errorf("github api: %w", err)
	}
	defer func() {
		// Drained before closing so the connection goes back to the pool
		// instead of being torn down after every non-200.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found (404): %s", rawURL)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github api returned %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// Scoring weights used by selectAsset to rank release assets.
const (
	scoreOS   = 5
	scoreArch = 3
	scoreExt  = 2
)

var osAliases = map[string][]string{
	"windows": {"windows", "win64", "win32", "win"},
	"linux":   {"linux"},
	"darwin":  {"darwin", "macos", "mac", "osx"},
}

var archAliases = map[string][]string{
	"amd64": {"amd64", "x86_64", "x64"},
	"arm64": {"arm64", "aarch64"},
	"386":   {"i386", "x86", "386"},
	"arm":   {"armv7", "armhf", "arm"},
}

// notABinary marks the assets that can never be installed as is. Archives are
// among them: the updater writes the asset over the binary, and a tarball named
// for this platform would score as the right one.
var notABinary = []string{".sha256", ".sha512", ".sig", ".asc", "checksums", ".txt",
	".tar.gz", ".tgz", ".tar.xz", ".tar.bz2", ".zip", ".deb", ".rpm", ".apk"}

// selectAsset picks the best asset for the current platform.
func selectAsset(assets []ghAsset) *ghAsset {
	osPatterns := osAliases[runtime.GOOS]
	archPatterns := archAliases[runtime.GOARCH]

	var best *ghAsset
	bestScore := -1

	for i := range assets {
		lower := strings.ToLower(assets[i].Name)
		if containsAnyOf(lower, notABinary) {
			continue
		}

		// The asset needs positive evidence it is for this machine. Architecture
		// alone is not it: "vivi_amd64" is as likely Linux as Windows, and
		// picking wrong writes an unrunnable file over the running binary.
		osMatch := matchesPlatform(lower, osPatterns)
		exeMatch := runtime.GOOS == "windows" && filepath.Ext(lower) == ".exe"
		if !osMatch && !exeMatch {
			continue
		}

		// Among the assets that are for this platform, the score picks the best.
		score := 0
		if osMatch {
			score += scoreOS
		}
		if matchesPlatform(lower, archPatterns) {
			score += scoreArch
		}
		if exeMatch {
			score += scoreExt
		}

		if score > bestScore {
			bestScore = score
			best = &assets[i]
		}
	}

	return best
}

// verifyDownloadedAsset checks downloadedPath against the published checksum
// and fails closed: a missing manifest was once a warning the update ignored,
// making "no checksum" the cheapest way to defeat the check.
func verifyDownloadedAsset(ctx context.Context, out, errOut io.Writer, assets []ghAsset, asset *ghAsset, downloadedPath string, allowUnverified bool) error {
	expected, err := findAssetChecksum(ctx, assets, asset)
	if err != nil {
		return unverified(errOut, allowUnverified,
			fmt.Sprintf("could not fetch checksum manifest: %v", err),
			fmt.Errorf("could not fetch the checksum manifest for this release, so the binary was not installed (%s): %w", unverifiedHint, err))
	}
	if expected == "" {
		return unverified(errOut, allowUnverified,
			"no checksum manifest found for this release",
			fmt.Errorf("this release publishes no checksum, so the download cannot be verified and the binary was not installed (%s)", unverifiedHint))
	}
	got, err := sha256File(downloadedPath)
	if err != nil {
		return fmt.Errorf("hashing downloaded binary: %w", err)
	}
	if !strings.EqualFold(got, expected) {
		return fmt.Errorf("checksum mismatch: downloaded binary does not match published checksum (expected %s, got %s)", expected, got)
	}
	fmt.Fprintln(out, "Checksum verified ✓")
	return nil
}

// unverifiedHint is the way past a download nothing vouches for, said inside
// the error rather than on a line of its own: an error is one line, because
// whoever wraps it prints it inside a sentence.
const unverifiedHint = "pass --allow-unverified to install it anyway"

// unverified is the answer to a download nothing vouches for: the failure, or
// two warnings when --allow-unverified turned the check off.
func unverified(errOut io.Writer, allowed bool, warning string, failure error) error {
	if !allowed {
		return failure
	}
	fmt.Fprintln(errOut, "warning: "+warning)
	fmt.Fprintln(errOut, "warning: installing an unverified binary because --allow-unverified was given")
	return nil
}

// findAssetChecksum locates the checksum manifest among assets and returns the
// expected SHA-256 hex digest for asset. Returns "" (no error) when no manifest
// is found; an error only for a manifest that was found but could not be read.
func findAssetChecksum(ctx context.Context, assets []ghAsset, asset *ghAsset) (string, error) {
	for i := range assets {
		// A digest file names the one asset it covers, and a release may ship
		// several. Reading a bare hex line out of the wrong one compares against
		// another platform's binary and looks like a corrupt download.
		perAsset := isPerAssetDigest(assets[i].Name, asset.Name)
		if !perAsset && !isManifest(assets[i].Name) {
			continue
		}

		body, err := fetchAssetBody(ctx, assets[i].BrowserDownloadURL)
		if err != nil {
			return "", err
		}
		if sum := findChecksumInManifest(body, asset.Name, perAsset); sum != "" {
			return sum, nil
		}
	}
	return "", nil
}

// isPerAssetDigest reports whether name is the digest file for asset: the asset
// name with a digest suffix, which is how goreleaser and friends name them.
func isPerAssetDigest(name, assetName string) bool {
	lower, target := strings.ToLower(name), strings.ToLower(assetName)
	for _, ext := range []string{".sha256", ".sha256sum", ".sha512"} {
		if lower == target+ext {
			return true
		}
	}
	return false
}

// isManifest reports whether name is a whole-release checksum list, which is
// keyed by filename and can be searched for any asset.
func isManifest(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "checksum") || strings.Contains(lower, "sha256sums")
}

// findChecksumInManifest scans a manifest for fileName. A line is either
// "<hex>  <filename>" (sha256sum(1), "*" for binary mode) or a bare digest,
// accepted whatever fileName is because such a file names only one asset.
func findChecksumInManifest(body []byte, fileName string, bare bool) string {
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		switch len(fields) {
		case 0:
			continue
		case 1:
			// A lone digest says nothing about which file it covers, so it is
			// only trustworthy when the file it came from is named after the
			// asset being verified.
			if bare && isHexSHA256(fields[0]) {
				return fields[0]
			}
		default:
			name := strings.TrimPrefix(fields[len(fields)-1], "*")
			if name == fileName && isHexSHA256(fields[0]) {
				return fields[0]
			}
		}
	}
	return ""
}

func isHexSHA256(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

// fetchAssetBody downloads a small release asset (a checksum manifest) fully
// into memory. Capped at 1 MiB, far more than any plain-text manifest needs.
func fetchAssetBody(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "github.com/lucasassuncao/vivi")
	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching checksum manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksum manifest returned HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// sha256File computes the SHA-256 hex digest of the file at path.
func sha256File(path string) (string, error) {
	f, err := os.Open(filepath.Clean(path)) //#nosec G304 -- path is the updater's own ".new" temp download
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// maxDownloadOverhead caps how much we read beyond the asset's advertised size.
const maxDownloadOverhead = 1 << 20 // 1 MiB

// download fetches url into destPath. expectedSize is the asset size reported by
// the release metadata; the response body is capped at expectedSize +
// maxDownloadOverhead to prevent a misconfigured or hostile server from filling the disk.
func download(ctx context.Context, rawURL, destPath string, expectedSize int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "github.com/lucasassuncao/vivi")

	resp, err := downloadClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	limit := expectedSize + maxDownloadOverhead
	if expectedSize <= 0 {
		limit = 256 << 20 // 256 MiB hard cap when no size is advertised
	}

	// The file is closed before any Remove: on Windows an open file cannot be
	// removed, and a failed download used to leave its .new behind.
	written, err := writeFile(destPath, io.LimitReader(resp.Body, limit+1)) // +1 so we can detect overflow
	switch {
	case err != nil:
		err = fmt.Errorf("writing binary: %w", err)
	case written > limit:
		err = fmt.Errorf("download exceeded expected size (%d bytes, cap %d)", written, limit)
	case expectedSize > 0 && written < expectedSize:
		err = fmt.Errorf("download truncated: got %d bytes, expected %d", written, expectedSize)
	}
	if err != nil {
		_ = os.Remove(destPath)
		return err
	}
	return nil
}

// writeFile streams r into path, syncs, and closes; a failure anywhere is
// reported after the file is closed, so the caller may remove it.
func writeFile(path string, r io.Reader) (written int64, retErr error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) //#nosec G302 G304 -- binary must be executable; path is controlled by the updater
	if err != nil {
		return 0, fmt.Errorf("creating temp binary: %w", err)
	}
	defer func() {
		if err := f.Close(); retErr == nil {
			retErr = err
		}
	}()

	if written, err = io.Copy(f, r); err != nil {
		return written, err
	}
	if err := f.Sync(); err != nil {
		return written, fmt.Errorf("syncing binary: %w", err)
	}
	return written, nil
}

// containsAnyOf reports whether s contains any of the needles.
func containsAnyOf(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

// matchesPlatform is containsAnyOf for the os and arch aliases, which are words
// in a file name rather than substrings of one: "darwin" ends in "win", and a
// plain Contains read a macOS build as a Windows one.
func matchesPlatform(s string, aliases []string) bool {
	for _, a := range aliases {
		for i := 0; ; {
			at := strings.Index(s[i:], a)
			if at < 0 {
				break
			}
			at += i
			if isWholeWord(s, at, len(a)) {
				return true
			}
			i = at + 1
		}
	}
	return false
}

// isWholeWord reports whether the match at [at, at+n) is bounded by separators
// on both sides. A release name separates its parts with punctuation, so
// anything alphanumeric next to the match means this is another word.
func isWholeWord(s string, at, n int) bool {
	before := at == 0 || !isAlphanumeric(rune(s[at-1]))
	end := at + n
	after := end == len(s) || !isAlphanumeric(rune(s[end]))
	return before && after
}

func isAlphanumeric(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}
