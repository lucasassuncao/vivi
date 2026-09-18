package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serveFakeRelease(t *testing.T, statusCode int, body any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

type redirectTransport struct{ target string }

func (r redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Host = r.target
	req2.URL.Scheme = "http"
	return http.DefaultTransport.RoundTrip(req2)
}

func withTestServer(t *testing.T, srv *httptest.Server, fn func()) {
	t.Helper()
	rt := redirectTransport{target: srv.Listener.Addr().String()}
	origAPI := apiClient
	origDownload := downloadClient
	apiClient = &http.Client{Transport: rt}
	downloadClient = &http.Client{Transport: rt}
	t.Cleanup(func() {
		apiClient = origAPI
		downloadClient = origDownload
	})
	fn()
}

func TestNormalizeVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"v0.0.1", "0.0.1"},
		{"", ""},
		{"vv1.0.0", "v1.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, normalizeVersion(tt.input))
		})
	}
}

func TestSelectAsset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		assets   []ghAsset
		wantName string // empty = nil expected
	}{
		{"nil assets", nil, ""},
		{"empty assets", []ghAsset{}, ""},
		{
			"skips checksum/sig files",
			[]ghAsset{
				{Name: "vivi_checksums.txt"},
				{Name: "vivi_linux_amd64.sha256"},
				{Name: "vivi_linux_amd64.sig"},
			},
			"",
		},
		{
			// An archive named for this platform scores like the binary it holds,
			// and the updater does not unpack: written over vivi it is unrunnable.
			"skips archives and packages",
			[]ghAsset{
				{Name: "vivi_1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"},
				{Name: "vivi_1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".zip"},
				{Name: "vivi_1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".deb"},
			},
			"",
		},
		{
			// One build per operating system, as a release carries them. Named
			// literally rather than from runtime.GOOS, so the case still offers
			// the other platforms whichever one the suite is running on.
			"prefers current os over other os",
			[]ghAsset{
				{Name: "vivi_linux_amd64"},
				{Name: "vivi_darwin_amd64"},
				{Name: "vivi_windows_amd64.exe"},
			},
			func() string {
				if runtime.GOOS == "windows" {
					return "vivi_windows_amd64.exe"
				}
				return "vivi_" + runtime.GOOS + "_amd64"
			}(),
		},
		{
			// Nothing here says which platform it is for. This used to return
			// the first asset, which self-update then wrote over the running
			// binary; refusing is the only safe answer.
			"refuses assets that name no platform",
			[]ghAsset{
				{Name: "vivi_unknown_v1"},
				{Name: "vivi_unknown_v2"},
			},
			"",
		},
		{
			"picks the matching arch among assets for this os",
			[]ghAsset{
				{Name: "vivi_" + runtime.GOOS + "_arm64"},
				{Name: "vivi_" + runtime.GOOS + "_amd64"},
			},
			"vivi_" + runtime.GOOS + "_" + runtime.GOARCH,
		},
		{
			// Assets built for another operating system, whatever their
			// architecture. Installing one leaves an unrunnable binary.
			"refuses another platform's build",
			[]ghAsset{
				{Name: "vivi_plan9_arm64"},
				{Name: "vivi_plan9_amd64"},
			},
			"",
		},
		{
			// On Windows the extension is evidence enough on its own; anywhere
			// else an ".exe" is somebody else's binary.
			"exe counts as a platform match on windows only",
			[]ghAsset{
				{Name: "vivi_arm64"},
				{Name: "vivi_amd64.exe"},
			},
			func() string {
				if runtime.GOOS == "windows" {
					return "vivi_amd64.exe"
				}
				return ""
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := selectAsset(tt.assets)
			if tt.wantName == "" {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.wantName, got.Name)
			}
		})
	}
}

// "darwin" ends in "win", and the aliases were matched as plain substrings. A
// release missing its Windows build would have installed the macOS one over
// vivi.exe instead of saying there was nothing to install.
func TestPlatformAliasesDoNotMatchInsideAnotherWord(t *testing.T) {
	t.Parallel()

	if matchesPlatform("vivi_1.0.0_darwin_x86_64", osAliases["windows"]) {
		t.Error(`the "win" alias matched a darwin asset`)
	}
	for _, name := range []string{
		"vivi_1.0.0_win_x86_64", "vivi_1.0.0_Windows_x86_64.exe", "vivi-win64.exe", "vivi.win.exe",
	} {
		if !matchesPlatform(strings.ToLower(name), osAliases["windows"]) {
			t.Errorf("a genuine Windows asset stopped matching: %s", name)
		}
	}
	if !matchesPlatform("vivi_1.0.0_darwin_arm64", archAliases["arm64"]) {
		t.Error("an arm64 asset stopped matching")
	}
}

func TestSelectAssetRefusesADarwinBuildOnWindows(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("the substring this guards against only bites on windows")
	}

	got := selectAsset([]ghAsset{{Name: "vivi_1.0.0_Darwin_x86_64"}})
	assert.Nil(t, got, "a darwin asset was taken for a windows one")
}

func TestIsHexSHA256(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"valid lowercase", "8bc3c68d94a3e4de6ea921270c169243da6fd46dedbff8f9608541e7390f4c4b", true},
		{"valid uppercase", "8BC3C68D94A3E4DE6EA921270C169243DA6FD46DEDBFF8F9608541E7390F4C4B", true},
		{"too short", "abc123", false},
		{"too long", "8bc3c68d94a3e4de6ea921270c169243da6fd46dedbff8f9608541e7390f4c4b00", false},
		{"non-hex characters", strings.Repeat("g", 64), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isHexSHA256(tt.in))
		})
	}
}

func TestFindChecksumInManifest(t *testing.T) {
	t.Parallel()
	const hash = "8bc3c68d94a3e4de6ea921270c169243da6fd46dedbff8f9608541e7390f4c4b"

	tests := []struct {
		name     string
		body     string
		fileName string
		bare     bool
		want     string
	}{
		{
			name:     "sha256sum format with matching filename",
			body:     hash + "  vivi_linux_amd64\n" + strings.Repeat("0", 64) + "  other_file\n",
			fileName: "vivi_linux_amd64",
			want:     hash,
		},
		{
			name:     "sha256sum format with binary-mode asterisk",
			body:     hash + " *vivi_linux_amd64\n",
			fileName: "vivi_linux_amd64",
			want:     hash,
		},
		{
			name:     "no matching filename",
			body:     hash + "  some_other_asset\n",
			fileName: "vivi_linux_amd64",
			want:     "",
		},
		{
			name:     "bare digest, from the asset's own digest file",
			body:     hash + "\n",
			fileName: "vivi_linux_amd64",
			bare:     true,
			want:     hash,
		},
		{
			// The same bytes out of another platform's digest file. Accepting
			// this compared the download against the wrong binary.
			name:     "bare digest, from somebody else's digest file",
			body:     hash + "\n",
			fileName: "vivi_linux_amd64",
			bare:     false,
			want:     "",
		},
		{
			name:     "empty manifest",
			body:     "",
			fileName: "vivi_linux_amd64",
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, findChecksumInManifest([]byte(tt.body), tt.fileName, tt.bare))
		})
	}
}

func TestFetchLatestRelease(t *testing.T) {
	tests := []struct {
		name    string
		server  func() *httptest.Server
		wantErr string
		check   func(t *testing.T, rel *ghRelease)
	}{
		{
			name: "200 ok returns release",
			server: func() *httptest.Server {
				return serveFakeRelease(t, http.StatusOK, ghRelease{
					TagName: "v2.5.3",
					Assets:  []ghAsset{{Name: "vivi_linux_amd64"}},
				})
			},
			check: func(t *testing.T, rel *ghRelease) {
				assert.Equal(t, "v2.5.3", rel.TagName)
				assert.Len(t, rel.Assets, 1)
			},
		},
		{
			name:    "404 returns not found error",
			server:  func() *httptest.Server { return serveFakeRelease(t, http.StatusNotFound, nil) },
			wantErr: "not found (404)",
		},
		{
			name:    "500 returns status error",
			server:  func() *httptest.Server { return serveFakeRelease(t, http.StatusInternalServerError, nil) },
			wantErr: "500",
		},
		{
			name: "invalid json returns decode error",
			server: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("not json"))
				}))
			},
			wantErr: "decoding response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := tt.server()
			defer srv.Close()

			withTestServer(t, srv, func() {
				rel, err := fetchLatestRelease(context.Background(), "owner/repo")
				if tt.wantErr != "" {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tt.wantErr)
					return
				}
				require.NoError(t, err)
				if tt.check != nil {
					tt.check(t, rel)
				}
			})
		})
	}
}

// serveReleaseAPI answers the three GitHub endpoints the updater reads, from
// one list. Tags are looked up by exact name, so a test can check the "v"
// fallback; the list echoes the per_page it was asked for through `seen`.
func serveReleaseAPI(t *testing.T, releases []ghRelease, seen *[]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/releases", func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			*seen = append(*seen, r.URL.Query().Get("per_page"))
		}
		_ = json.NewEncoder(w).Encode(releases)
	})
	mux.HandleFunc("/repos/owner/repo/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		for i := range releases {
			if !releases[i].Draft && !releases[i].Prerelease {
				_ = json.NewEncoder(w).Encode(releases[i])
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/repos/owner/repo/releases/tags/", func(w http.ResponseWriter, r *http.Request) {
		tag := strings.TrimPrefix(r.URL.Path, "/repos/owner/repo/releases/tags/")
		for i := range releases {
			if releases[i].TagName == tag {
				_ = json.NewEncoder(w).Encode(releases[i])
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})
	return httptest.NewServer(mux)
}

// ListReleases is what --list prints. Drafts never show, prereleases only on
// request, and the limit is honoured after filtering, not before: a page of
// twenty with fifteen prereleases must still list every stable one asked for.
func TestListReleases(t *testing.T) {
	releases := []ghRelease{
		{TagName: "v3.0.0-rc1", Prerelease: true},
		{TagName: "v2.1.0-draft", Draft: true},
		{TagName: "v2.0.0"},
		{TagName: "v1.9.0-beta", Prerelease: true},
		{TagName: "v1.8.0"},
		{TagName: "v1.7.0"},
	}
	tags := func(rels []Release) []string {
		out := make([]string, 0, len(rels))
		for _, r := range rels {
			out = append(out, r.Tag)
		}
		return out
	}

	t.Run("stable only, drafts hidden", func(t *testing.T) {
		var seen []string
		srv := serveReleaseAPI(t, releases, &seen)
		defer srv.Close()
		withTestServer(t, srv, func() {
			got, err := ListReleases(context.Background(), "owner/repo", false, 20)
			require.NoError(t, err)
			assert.Equal(t, []string{"v2.0.0", "v1.8.0", "v1.7.0"}, tags(got))
			// Over-fetched to make room for the prereleases it will drop.
			assert.Equal(t, []string{"40"}, seen)
		})
	})

	t.Run("prereleases on request, drafts still hidden", func(t *testing.T) {
		var seen []string
		srv := serveReleaseAPI(t, releases, &seen)
		defer srv.Close()
		withTestServer(t, srv, func() {
			got, err := ListReleases(context.Background(), "owner/repo", true, 20)
			require.NoError(t, err)
			assert.Equal(t, []string{"v3.0.0-rc1", "v2.0.0", "v1.9.0-beta", "v1.8.0", "v1.7.0"}, tags(got))
			assert.Equal(t, []string{"20"}, seen)
			for _, r := range got {
				if r.Tag == "v3.0.0-rc1" {
					assert.True(t, r.Prerelease, "the flag must travel with the release")
				}
			}
		})
	})

	t.Run("limit counts what is shown", func(t *testing.T) {
		srv := serveReleaseAPI(t, releases, nil)
		defer srv.Close()
		withTestServer(t, srv, func() {
			got, err := ListReleases(context.Background(), "owner/repo", false, 2)
			require.NoError(t, err)
			assert.Equal(t, []string{"v2.0.0", "v1.8.0"}, tags(got))
		})
	})

	t.Run("limit defaults and caps", func(t *testing.T) {
		var seen []string
		srv := serveReleaseAPI(t, releases, &seen)
		defer srv.Close()
		withTestServer(t, srv, func() {
			_, err := ListReleases(context.Background(), "owner/repo", false, 0)
			require.NoError(t, err)
			_, err = ListReleases(context.Background(), "owner/repo", false, 90)
			require.NoError(t, err)
			// 0 becomes 20 and is doubled; 90 doubled is 180, capped at the API's 100.
			assert.Equal(t, []string{"40", "100"}, seen)
		})
	})

	t.Run("no repo is refused before any request", func(t *testing.T) {
		_, err := ListReleases(context.Background(), "", false, 20)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "repo is required")
	})

	t.Run("server errors surface", func(t *testing.T) {
		srv := serveFakeRelease(t, http.StatusInternalServerError, nil)
		defer srv.Close()
		withTestServer(t, srv, func() {
			_, err := ListReleases(context.Background(), "owner/repo", false, 20)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "500")
		})
	})
}

// resolveRelease maps --version and --prerelease onto one release. The "v"
// fallback is the case worth pinning: "1.2.0" must find the tag "v1.2.0", and
// a tag that exists under neither spelling must report the one the user typed.
func TestResolveRelease(t *testing.T) {
	releases := []ghRelease{
		{TagName: "v2.0.0-draft", Draft: true},
		{TagName: "v2.0.0-rc1", Prerelease: true},
		{TagName: "v1.2.0"},
	}

	resolve := func(t *testing.T, rels []ghRelease, version string, prerelease bool) (*ghRelease, string, error) {
		t.Helper()
		srv := serveReleaseAPI(t, rels, nil)
		defer srv.Close()
		var (
			rel *ghRelease
			err error
			out strings.Builder
		)
		withTestServer(t, srv, func() {
			rel, err = resolveRelease(context.Background(), &out, "owner/repo", version, prerelease)
		})
		return rel, out.String(), err
	}

	t.Run("an exact tag", func(t *testing.T) {
		rel, out, err := resolve(t, releases, "v1.2.0", false)
		require.NoError(t, err)
		assert.Equal(t, "v1.2.0", rel.TagName)
		assert.Contains(t, out, "Fetching release v1.2.0")
	})

	t.Run("a tag typed without its v", func(t *testing.T) {
		rel, _, err := resolve(t, releases, "1.2.0", false)
		require.NoError(t, err)
		assert.Equal(t, "v1.2.0", rel.TagName)
	})

	t.Run("a tag that exists under neither spelling", func(t *testing.T) {
		_, _, err := resolve(t, releases, "9.9.9", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tags/9.9.9", "the error names what the user typed")
	})

	t.Run("an explicit prerelease is honoured without the flag", func(t *testing.T) {
		rel, _, err := resolve(t, releases, "v2.0.0-rc1", false)
		require.NoError(t, err)
		assert.Equal(t, "v2.0.0-rc1", rel.TagName)
	})

	t.Run("latest skips prereleases", func(t *testing.T) {
		rel, out, err := resolve(t, releases, "", false)
		require.NoError(t, err)
		assert.Equal(t, "v1.2.0", rel.TagName)
		assert.Contains(t, out, "Checking latest release")
	})

	t.Run("latest with prereleases skips drafts only", func(t *testing.T) {
		rel, out, err := resolve(t, releases, "", true)
		require.NoError(t, err)
		assert.Equal(t, "v2.0.0-rc1", rel.TagName)
		assert.Contains(t, out, "including prereleases")
	})

	t.Run("nothing but drafts is nothing", func(t *testing.T) {
		_, _, err := resolve(t, []ghRelease{{TagName: "v0.1.0", Draft: true}}, "", true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no releases found")
	})
}

func TestDownload(t *testing.T) {
	content := []byte("fake binary content")

	tests := []struct {
		name         string
		server       func() *httptest.Server
		destFn       func(dir string) string
		expectedSize int64
		wantErr      string
		check        func(t *testing.T, dest string)
	}{
		{
			name: "writes content to dest",
			server: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(content)
				}))
			},
			destFn:       func(dir string) string { return filepath.Join(dir, "binary") },
			expectedSize: int64(len(content)),
			check: func(t *testing.T, dest string) {
				got, err := os.ReadFile(dest)
				require.NoError(t, err)
				assert.Equal(t, content, got)
			},
		},
		{
			name: "http error returns status error",
			server: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusForbidden)
				}))
			},
			destFn:       func(dir string) string { return filepath.Join(dir, "binary") },
			expectedSize: 0,
			wantErr:      "403",
		},
		{
			name: "invalid dest path returns error",
			server: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("data"))
				}))
			},
			destFn:       func(_ string) string { return "/nonexistent/dir/binary" },
			expectedSize: 0,
			wantErr:      "creating temp binary",
		},
		{
			name: "truncated download returns error",
			server: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("short"))
				}))
			},
			destFn:       func(dir string) string { return filepath.Join(dir, "binary") },
			expectedSize: 1000,
			wantErr:      "download truncated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := tt.server()
			defer srv.Close()

			dest := tt.destFn(t.TempDir())
			err := download(context.Background(), srv.URL, dest, tt.expectedSize)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				// A failed download must not leave its file behind. On Windows a
				// Remove while the file is still open fails, so this is a real check.
				_, statErr := os.Stat(dest)
				assert.True(t, os.IsNotExist(statErr), "partial download left at %s", dest)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, dest)
			}
		})
	}
}

func TestCleanOldBinary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		createOld bool
	}{
		{"removes .old file when present", true},
		{"noop when .old file absent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exe, err := os.Executable()
			require.NoError(t, err)
			old := exe + ".old"

			_ = os.Remove(old)
			if tt.createOld {
				f, err := os.Create(old)
				require.NoError(t, err)
				f.Close()
			}

			assert.NotPanics(t, func() { CleanOldBinary(io.Discard) })
			_, statErr := os.Stat(old)
			assert.True(t, os.IsNotExist(statErr), ".old file should not exist after CleanOldBinary")
		})
	}
}

func TestSelfUpdate(t *testing.T) {
	tests := []struct {
		name           string
		repo           string
		currentVersion string
		release        *ghRelease
		wantErr        string
	}{
		{
			name:    "empty repo returns error",
			repo:    "",
			wantErr: "--repo is required",
		},
		{
			name:           "already up to date",
			repo:           "owner/repo",
			currentVersion: "v2.5.3",
			release:        &ghRelease{TagName: "v2.5.3", Assets: []ghAsset{{Name: "asset"}}},
		},
		{
			name:           "no compatible asset",
			repo:           "owner/repo",
			currentVersion: "v1.0.0",
			release:        &ghRelease{TagName: "v9.9.9", Assets: []ghAsset{{Name: "checksums.txt"}}},
			wantErr:        "no compatible binary found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.release != nil {
				srv := serveFakeRelease(t, http.StatusOK, tt.release)
				defer srv.Close()
				withTestServer(t, srv, func() {
					err := SelfUpdate(context.Background(), UpdateOptions{Repo: tt.repo, CurrentVersion: tt.currentVersion, Out: io.Discard, Err: io.Discard})
					if tt.wantErr != "" {
						require.Error(t, err)
						assert.Contains(t, err.Error(), tt.wantErr)
					} else {
						assert.NoError(t, err)
					}
				})
				return
			}
			err := SelfUpdate(context.Background(), UpdateOptions{Repo: tt.repo, CurrentVersion: tt.currentVersion, Out: io.Discard, Err: io.Discard})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestSelfUpdate_DownloadsAndReplacesBinary(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "vivi")
	require.NoError(t, os.WriteFile(fakeBin, []byte("old"), 0o755))

	newContent := []byte("new binary content")
	sum := sha256.Sum256(newContent)
	manifest := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), platformAsset())

	// The release publishes a checksum, because a release without one is no
	// longer installable without --allow-unverified. This test now covers the
	// verification too, rather than passing because it was skipped.
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(newContent)
			return
		case "/checksums":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(manifest))
			return
		}
		rel := ghRelease{
			TagName: "v9.9.9",
			Assets: []ghAsset{{
				Name:               platformAsset(),
				BrowserDownloadURL: srv.URL + "/download",
				Size:               int64(len(newContent)),
			}, {
				Name:               "checksums.txt",
				BrowserDownloadURL: srv.URL + "/checksums",
			}},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(rel)
	}))
	defer srv.Close()

	origExecutable := osExecutable
	osExecutable = func() (string, error) { return fakeBin, nil }
	t.Cleanup(func() { osExecutable = origExecutable })

	withTestServer(t, srv, func() {
		require.NoError(t, SelfUpdate(context.Background(), UpdateOptions{Repo: "owner/repo", CurrentVersion: "v1.0.0", Out: io.Discard, Err: io.Discard}))
	})

	got, err := os.ReadFile(fakeBin)
	require.NoError(t, err)
	assert.Equal(t, newContent, got)

	_, err = os.Stat(fakeBin + ".old")
	assert.NoError(t, err, ".old binary should exist")
}

// serveInstallableRelease publishes one verified binary for this platform.
func serveInstallableRelease(t *testing.T, content []byte) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(content)
	manifest := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), platformAsset())
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download":
			_, _ = w.Write(content)
		case "/checksums":
			_, _ = w.Write([]byte(manifest))
		default:
			_ = json.NewEncoder(w).Encode(ghRelease{
				TagName: "v9.9.9",
				Assets: []ghAsset{
					{Name: platformAsset(), BrowserDownloadURL: srv.URL + "/download", Size: int64(len(content))},
					{Name: "checksums.txt", BrowserDownloadURL: srv.URL + "/checksums"},
				},
			})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// A stale .old that cannot be removed (on Windows, a file somebody still has
// open) used to sink the whole update: the rename onto it failed after the
// download had already been verified. The backup takes a fresh name instead,
// and the sweep at the next start removes every .old* it finds.
func TestSelfUpdate_BacksUpUnderAFreshNameWhenOldIsHeld(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "vivi")
	require.NoError(t, os.WriteFile(fakeBin, []byte("old"), 0o755))

	held, err := os.OpenFile(fakeBin+".old", os.O_CREATE|os.O_WRONLY, 0o755)
	require.NoError(t, err)
	t.Cleanup(func() { _ = held.Close() })

	origExecutable := osExecutable
	osExecutable = func() (string, error) { return fakeBin, nil }
	t.Cleanup(func() { osExecutable = origExecutable })

	newContent := []byte("new binary content")
	withTestServer(t, serveInstallableRelease(t, newContent), func() {
		var out strings.Builder
		require.NoError(t, SelfUpdate(context.Background(), UpdateOptions{
			Repo: "owner/repo", CurrentVersion: "v1.0.0", Out: &out, Err: io.Discard,
		}))
		assert.Contains(t, out.String(), "old binary saved as vivi.old")
	})

	got, err := os.ReadFile(fakeBin)
	require.NoError(t, err)
	assert.Equal(t, newContent, got)

	// Wherever the backup landed, it holds what was running before.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var backups []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "vivi.old") {
			backups = append(backups, e.Name())
		}
	}
	var sawOld bool
	for _, name := range backups {
		if b, _ := os.ReadFile(filepath.Join(dir, name)); string(b) == "old" {
			sawOld = true
		}
	}
	assert.True(t, sawOld, "the previous binary must survive as a backup, got %v", backups)

	// Once nothing holds it, the sweep clears every backup name.
	require.NoError(t, held.Close())
	CleanOldBinary(io.Discard)
	entries, err = os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.HasPrefix(e.Name(), "vivi.old"), "%s survived the sweep", e.Name())
	}
}

// TestSelfUpdate_VerifiesMatchingChecksum is a regression test for the
// self-update checksum verification: a release that publishes a checksum
// manifest matching the downloaded binary must install successfully.
func TestSelfUpdate_VerifiesMatchingChecksum(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "vivi")
	require.NoError(t, os.WriteFile(fakeBin, []byte("old"), 0o755))

	newContent := []byte("new binary content")
	const correctSum = "8bc3c68d94a3e4de6ea921270c169243da6fd46dedbff8f9608541e7390f4c4b"

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(newContent)
		case "/checksums.txt":
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s  %s\n", correctSum, platformAsset())
		default:
			rel := ghRelease{
				TagName: "v9.9.9",
				Assets: []ghAsset{
					{Name: platformAsset(), BrowserDownloadURL: srv.URL + "/download", Size: int64(len(newContent))},
					{Name: "checksums.txt", BrowserDownloadURL: srv.URL + "/checksums.txt"},
				},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(rel)
		}
	}))
	defer srv.Close()

	origExecutable := osExecutable
	osExecutable = func() (string, error) { return fakeBin, nil }
	t.Cleanup(func() { osExecutable = origExecutable })

	withTestServer(t, srv, func() {
		require.NoError(t, SelfUpdate(context.Background(), UpdateOptions{Repo: "owner/repo", CurrentVersion: "v1.0.0", Out: io.Discard, Err: io.Discard}))
	})

	got, err := os.ReadFile(fakeBin)
	require.NoError(t, err)
	assert.Equal(t, newContent, got)
}

// TestSelfUpdate_ChecksumMismatchAbortsInstall is a regression test: when the
// downloaded binary does not match a checksum manifest that was found, the
// install must abort and leave the original binary untouched.
func TestSelfUpdate_ChecksumMismatchAbortsInstall(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "vivi")
	require.NoError(t, os.WriteFile(fakeBin, []byte("old"), 0o755))

	newContent := []byte("new binary content")
	wrongSum := strings.Repeat("0", 64)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(newContent)
		case "/checksums.txt":
			_, _ = fmt.Fprintf(w, "%s  %s\n", wrongSum, platformAsset())
		default:
			rel := ghRelease{
				TagName: "v9.9.9",
				Assets: []ghAsset{
					{Name: platformAsset(), BrowserDownloadURL: srv.URL + "/download", Size: int64(len(newContent))},
					{Name: "checksums.txt", BrowserDownloadURL: srv.URL + "/checksums.txt"},
				},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(rel)
		}
	}))
	defer srv.Close()

	origExecutable := osExecutable
	osExecutable = func() (string, error) { return fakeBin, nil }
	t.Cleanup(func() { osExecutable = origExecutable })

	withTestServer(t, srv, func() {
		err := SelfUpdate(context.Background(), UpdateOptions{Repo: "owner/repo", CurrentVersion: "v1.0.0", Out: io.Discard, Err: io.Discard})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "checksum mismatch")
	})

	got, err := os.ReadFile(fakeBin)
	require.NoError(t, err)
	assert.Equal(t, []byte("old"), got, "original binary must be untouched after a checksum mismatch")

	_, err = os.Stat(fakeBin + ".old")
	assert.True(t, os.IsNotExist(err), "no .old file should be created when install aborts before replacing the binary")
	_, err = os.Stat(fakeBin + ".new")
	assert.True(t, os.IsNotExist(err), "the failed download must be cleaned up")
}

// platformAsset names a release asset for the machine running the test.
// selectAsset refuses assets that do not say which platform they are for, so a
// fixture hard-coding one operating system only exercises the code on that one.
func platformAsset() string {
	return fmt.Sprintf("vivi_%s_%s", runtime.GOOS, runtime.GOARCH)
}

// A digest file names the one asset it covers, and a release may publish one
// per platform. Reading a bare hex line out of the wrong one compares against
// another platform's binary, failing a download that was perfectly good.
func TestPerAssetDigestBelongsToItsOwnAsset(t *testing.T) {
	t.Parallel()
	const hash = "8bc3c68d94a3e4de6ea921270c169243da6fd46dedbff8f9608541e7390f4c4b"

	assert.True(t, isPerAssetDigest("vivi_linux_amd64.sha256", "vivi_linux_amd64"),
		"a digest named after the asset covers it")
	assert.False(t, isPerAssetDigest("vivi_windows_amd64.exe.sha256", "vivi_linux_amd64"),
		"another platform's digest does not")

	assert.Equal(t, hash, findChecksumInManifest([]byte(hash+"\n"), "vivi_linux_amd64", true),
		"a lone digest is trusted from the asset's own file")
	assert.Empty(t, findChecksumInManifest([]byte(hash+"\n"), "vivi_linux_amd64", false),
		"a lone digest from anywhere else names no file and must be ignored")
}

// A release with no checksum used to install anyway, behind a warning nobody
// reads - making "publish no checksum" the cheapest way to defeat the check, on
// the one command that overwrites the binary the user is running.
func TestSelfUpdateRefusesAnUnverifiableRelease(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "vivi")
	require.NoError(t, os.WriteFile(fakeBin, []byte("old"), 0o755))

	newContent := []byte("new binary content")

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(newContent)
			return
		}
		// No checksum asset anywhere in this release.
		rel := ghRelease{
			TagName: "v9.9.9",
			Assets: []ghAsset{{
				Name:               platformAsset(),
				BrowserDownloadURL: srv.URL + "/download",
				Size:               int64(len(newContent)),
			}},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(rel)
	}))
	defer srv.Close()

	origExecutable := osExecutable
	osExecutable = func() (string, error) { return fakeBin, nil }
	t.Cleanup(func() { osExecutable = origExecutable })

	withTestServer(t, srv, func() {
		err := SelfUpdate(context.Background(), UpdateOptions{Repo: "owner/repo", CurrentVersion: "v1.0.0", Out: io.Discard, Err: io.Discard})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no checksum")
		assert.Contains(t, err.Error(), "--allow-unverified")
	})

	// The refusal has to leave the running binary alone, and clean up after the
	// download it threw away.
	got, err := os.ReadFile(fakeBin)
	require.NoError(t, err)
	assert.Equal(t, []byte("old"), got, "the binary was replaced despite the refusal")

	_, err = os.Stat(fakeBin + ".new")
	assert.True(t, os.IsNotExist(err), "the rejected download was left on disk")

	// And the same release installs once the risk is accepted explicitly.
	withTestServer(t, srv, func() {
		require.NoError(t, SelfUpdate(context.Background(), UpdateOptions{
			Repo: "owner/repo", CurrentVersion: "v1.0.0", AllowUnverified: true,
		}))
	})
	got, err = os.ReadFile(fakeBin)
	require.NoError(t, err)
	assert.Equal(t, newContent, got)
}

// Execute wires signal.NotifyContext, so Ctrl-C cancels everything else the CLI
// does. The updater built every request with http.NewRequest and never saw it:
// a download had a five-minute ceiling and no way out of it.
func TestSelfUpdateStopsWhenTheContextIsCancelled(t *testing.T) {
	// The server never has to answer: a cancelled context fails the very first
	// request, which is the point,  the old code would have reached the server
	// and then waited on downloadClient's five-minute ceiling.
	srv := serveFakeRelease(t, http.StatusOK, ghRelease{TagName: "v9.9.9"})
	t.Cleanup(srv.Close)

	withTestServer(t, srv, func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := SelfUpdate(ctx, UpdateOptions{
			Repo:           "owner/repo",
			CurrentVersion: "1.0.0",
			Out:            io.Discard,
			Err:            io.Discard,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

// The updater printed to the process's own stdout, which is why nothing asserted
// on what it said. It takes the streams from its caller now, and the CLI passes
// cobra's.
func TestSelfUpdateWritesThroughTheStreamsItIsGiven(t *testing.T) {
	srv := serveFakeRelease(t, http.StatusOK, ghRelease{TagName: "v1.0.0"})
	t.Cleanup(srv.Close)

	var out, errOut strings.Builder
	withTestServer(t, srv, func() {
		err := SelfUpdate(context.Background(), UpdateOptions{
			Repo:           "owner/repo",
			CurrentVersion: "1.0.0",
			Out:            &out,
			Err:            &errOut,
		})
		require.NoError(t, err)
	})

	assert.Contains(t, out.String(), "Already on v1.0.0")
	assert.Empty(t, errOut.String())
}
