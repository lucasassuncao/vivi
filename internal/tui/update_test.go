package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/lucasassuncao/bezel/bezeltest"
	"github.com/lucasassuncao/bezel/component/input"
	"github.com/lucasassuncao/bezel/component/textbox"
)

// What the loop does with an answer, and in particular with a late one: every
// request here is issued against a cursor that may have moved before the reply
// arrives.

// The three tests that pinned the stale-answer guard now live beside the guard,
// in the policies package: the state they set up is that tab's own.

// The stale-capabilities guard moved to the secrets package with the answer it
// drops: the message is that tab's own.

// The role-keying test moved to the auth package with the map it pins.

// pending counts what is in flight, so a drifting count misreports the session
// busy or permanently idle. It was incremented by hand at every call site and
// two forgot; it lives in the command builder now, beside its decrement.
func TestPendingCountNeverDrifts(t *testing.T) {
	h := newHarness(t)
	if h.m.pending != 0 {
		t.Fatalf("pending after init = %d, want 0", h.m.pending)
	}

	// Every path that issues a request, including the two that were missing it.
	for _, key := range []string{"2", "3", "4", "1", "right", "down", "enter", "i", "R", "tab"} {
		h.press(key)
		if h.m.pending < 0 {
			t.Fatalf("pending went negative (%d) after %q", h.m.pending, key)
		}
	}

	if h.m.pending != 0 {
		t.Errorf("pending = %d once everything answered, want 0", h.m.pending)
	}
}

// Nothing animates, so something still has to bring the render around or
// the token countdown freezes at whatever the last key left it. A number that
// does not move is worse than no number, because it gets believed.
func TestTheClockKeepsTicking(t *testing.T) {
	restore := clockInterval
	clockInterval = time.Millisecond
	t.Cleanup(func() { clockInterval = restore })

	h := newHarness(t)

	_, cmd := h.m.Update(clockMsg{})
	if cmd == nil {
		t.Fatal("the clock stopped: the token countdown would freeze")
	}
	if _, ok := cmd().(clockMsg); !ok {
		t.Error("the clock must re-arm itself")
	}
}

// The filter and the modals are nested components with messages of their own.
// Their blink timer answers with a message of its own, and if the update loop
// drops it the timer is never rearmed: the caret stops moving while the user types.
func TestNestedInputsKeepTheirCursorBlinking(t *testing.T) {
	h := newHarness(t)
	// rearms reports whether the blink a key armed comes back to its field.
	rearms := func(key string) bool {
		_, cmd := h.m.Update(bezeltest.Key(key))
		blink := blinkIn(cmd)
		if blink == nil {
			return false
		}
		_, next := h.m.Update(blink)
		return next != nil
	}

	if !rearms("/") {
		t.Error("the filter input never received the blink message")
	}
	h.press("esc")

	h.focus("kv/")
	h.press("a") // the create modal captures free text
	if !rearms("x") {
		t.Error("the modal input never received the blink message")
	}
	h.press("esc")

	h.open("kv/app/prod/db")
	h.press("e")          // editor
	if !rearms("enter") { // start typing a value
		t.Error("the editor input never received the blink message")
	}
}

// blinkIn runs cmd and finds the caret blink it carries, batched or not.
func blinkIn(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			if b := blinkIn(c); b != nil {
				return b
			}
		}
	case input.BlinkMsg, textbox.BlinkMsg:
		return msg
	}
	return nil
}
