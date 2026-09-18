package tui

import "testing"

// A request raises the pending count and its answer lowers it. The compiler
// guards the marker now; this guards the invariant the marker is for, over one
// secret's three reads.
func TestPendingReturnsToZeroAfterASecretIsLoaded(t *testing.T) {
	h := newHarness(t)

	h.open("kv/app/prod/db")
	if h.m.pending != 0 {
		t.Fatalf("%d requests are still counted as in flight after the secret settled",
			h.m.pending)
	}

	h.press("v")
	h.press("esc")
	h.press("R")
	if h.m.pending != 0 {
		t.Fatalf("%d requests are still counted as in flight after a reload", h.m.pending)
	}
}
