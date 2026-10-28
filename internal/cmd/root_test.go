package cmd

import (
	"io"
	"os"
	"testing"

	"github.com/lucasassuncao/bezel/animation"
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

// The mouse is on unless --no-mouse says otherwise.
func TestTheMouseIsOnUnlessNoMouse(t *testing.T) {
	f := NewRootCmd().Flags().Lookup("no-mouse")
	if f == nil || f.DefValue != "false" {
		t.Fatalf("--no-mouse is missing or not off by default: %+v", f)
	}
}

// Without --no-motion a BEZEL_NO_MOTION from the shell is cleared: the flag
// is the only switch.
func TestWithoutNoMotionTheEnvironmentHasNoSay(t *testing.T) {
	t.Setenv(noMotionEnvVar, "1")
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })
	os.Args = []string{"vivi", "--list-themes"}

	if code := Execute(); code != 0 {
		t.Fatalf("--list-themes exited %d", code)
	}
	if _, set := os.LookupEnv(noMotionEnvVar); set {
		t.Errorf("%s from the shell survived a run without --no-motion", noMotionEnvVar)
	}
}

// --no-motion turns bezel's animations off before anything animates.
func TestNoMotionTurnsTheAnimationsOff(t *testing.T) {
	t.Setenv(noMotionEnvVar, "")
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })
	os.Args = []string{"vivi", "--no-motion", "--list-themes"}

	if code := Execute(); code != 0 {
		t.Fatalf("--no-motion --list-themes exited %d", code)
	}
	if !animation.Still() {
		t.Error("--no-motion left the animations on")
	}
}
