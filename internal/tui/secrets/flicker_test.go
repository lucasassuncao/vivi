package secrets

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Moving between sibling secrets fires three reads that answer one at a time,
// and what the screen shows meanwhile is this file's subject. It used to empty
// to "loading…" and repaint three times per move, led by a blank frame.
//
// The header used to carry a spinner here too, which moved the address four
// columns every time a read started. It is gone: nothing on screen animates.

func TestThePaneShowsOneWholeSecretInEveryFrame(t *testing.T) {
	h := newHarness(t)
	h.resize(120, 40)
	h.open("kv/app/prod/cache")

	from := h.m.sel.fullPath()
	fromFields := strings.Join(h.m.sortedKeys(), " ")

	// Down to the sibling, then hand over the answers one by one, looking at the
	// screen after each.
	cmd := h.m.Key(keyMsg("down"), h.ctx)

	frames := []string{h.view()}
	for _, msg := range h.collect(cmd) {
		h.m.Data(msg)
		frames = append(frames, h.view())
	}

	to := h.m.sel.fullPath()
	toFields := strings.Join(h.m.sortedKeys(), " ")
	if from == to {
		t.Fatalf("precondition: the cursor did not move off %s", from)
	}

	for i, frame := range frames {
		view := ansi.Strip(frame)
		if strings.Contains(view, "loading…") {
			t.Errorf("frame %d blanks the pane while reading:\n%s", i, view)
		}

		// The title names one of the two, and the fields on screen have to be
		// that one's. Anything else is half a secret.
		title, fields := from, fromFields
		if strings.Contains(view, to) && !strings.Contains(view, from) {
			title, fields = to, toFields
		}
		for _, key := range strings.Fields(fields) {
			if !strings.Contains(view, key) {
				t.Errorf("frame %d shows %s but not its field %q:\n%s", i, title, key, view)
			}
		}
	}
}

// Nothing is remembered between visits: vivi holds no copy of a value, so what
// is on screen is always something the server said a moment ago. A cache would
// get somebody else's write wrong with no way to notice.
func TestEveryVisitReadsTheSecretAgain(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/cache")
	first := h.server.Calls["read"]

	h.press("down") // to the sibling
	h.press("up")   // and back

	if h.m.sel.fullPath() != "kv/app/prod/cache" {
		t.Fatalf("precondition: the pane is on %s, want it back on cache", h.m.sel.fullPath())
	}
	if got := h.server.Calls["read"]; got <= first {
		t.Errorf("coming back served a remembered copy: still %d reads", got)
	}
	if h.m.loading != nil {
		t.Error("the read settled but something is still in flight")
	}
}
