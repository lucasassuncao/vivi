package cmd

import (
	"io"
	"os"
	"testing"
)

// A release build injects the version; a "go install" one does not, and used to
// report "dev" while the module version sat in the binary's own build info.
func TestVersionFallsBackToTheModuleVersion(t *testing.T) {
	cases := []struct {
		name      string
		injected  string
		buildInfo string
		want      string
	}{
		{"ldflags win", "1.2.3", "v0.9.0", "1.2.3"},
		{"go install reads the module version", "dev", "v0.9.0", "v0.9.0"},
		{"a local build stays dev", "dev", "(devel)", "dev"},
		{"no build info stays dev", "dev", "", "dev"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveVersion(tc.injected, tc.buildInfo); got != tc.want {
				t.Errorf("resolveVersion(%q, %q) = %q, want %q", tc.injected, tc.buildInfo, got, tc.want)
			}
		})
	}
}

// self-update's help says the .old binary is kept "until the next run", and
// nothing ran it. --list-themes is the cheapest path through Execute that
// returns before requireTerminal, so this asserts on startup, not the browser.
func TestStartupClearsAStaleOldBinary(t *testing.T) {
	var called bool
	original := cleanOldBinary
	t.Cleanup(func() { cleanOldBinary = original })
	cleanOldBinary = func(io.Writer) { called = true }

	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })
	os.Args = []string{"vivi", "--list-themes"}

	if code := Execute(); code != 0 {
		t.Fatalf("--list-themes exited %d", code)
	}
	if !called {
		t.Error("startup did not clear a leftover .old binary")
	}
}
