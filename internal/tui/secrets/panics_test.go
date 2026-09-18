package secrets

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// Nothing type-checks that the payload a message promises is in it, and every
// handler below once reached straight through the pointer: an empty message did
// not make a wrong screen, it ended the session with the edit inside it.
func TestEmptyMessagesDoNotEndTheSession(t *testing.T) {
	msgs := []tea.Msg{
		childrenMsg{},                // an answer with no node to fold it into
		secretMsg{},                  //
		versionsMsg{},                //
		writeMsg{},                   //
		opMsg{op: "delete"},          //
		diffMsg{},                    // neither a result nor an error
		restoreModeMsg{},             // a way back that leads nowhere
		confirmedDeleteMsg{},         // a confirmed operation with nothing to act on
		confirmedDestroyMsg{},        //
		confirmedDeleteMetadataMsg{}, //
		confirmedRollbackMsg{},       //
		confirmedSaveMsg{},           //
		confirmedReloadMsg{},         //
		confirmedCreateMsg{},         //
	}

	for _, msg := range msgs {
		h := newHarness(t)
		h.open("kv/app/prod/db")
		h.run(h.m.Data(msg))
		h.view() // panics on failure, which is the assertion

		if h.m.mode == nil {
			t.Errorf("%T left the tab with no mode, which the next keystroke would crash on", msg)
		}
		// The tab has to still answer keys, not merely still exist.
		h.press("down", "up", "esc")
		h.view()
	}
}

// A client that answers a read with neither a secret nor an error is not a case
// the code writes, and is exactly the shape a proxy or a stub produces.
type emptyReader struct{ vault.Client }

func (emptyReader) Read(context.Context, string, string, int) (*vault.Secret, error) {
	return nil, nil
}

func TestAClientThatReadsNothingIsReportedRatherThanFatal(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.m.client = emptyReader{h.server}

	h.press("V")
	h.press(" ")    // mark the version under the cursor
	h.press("down") //
	h.press(" ")    // and the one below it, which is what diff needs
	h.m.adopt(h.ctx)
	h.run(h.m.startDiff())

	h.view()
	if modeIs[showingDiff](h.m) {
		t.Error("a comparison with nothing to compare should not open the diff panel")
	}
	if !h.bannerErr() {
		t.Errorf("the failure should be reported in the footer, got %q", h.banner())
	}

	// The rollback takes the same shape: read, then write what was read.
	h.press("esc")
	h.run(h.m.rollback(h.m.sel, 1, 1))
	h.view()
	if !h.bannerErr() {
		t.Errorf("a rollback with nothing to restore should be reported, got %q", h.banner())
	}
}

// A write to a nil map is a panic, not a no-op, and these are replaced whole in
// several places - so the invariant is "never nil when a key arrives".
func TestNilMapsAreRestoredBeforeAKeyIsHandled(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("enter")

	h.m.revealed, h.m.marked = nil, nil

	h.press("r") // writes to revealed
	h.press("esc")
	h.press("V")
	h.press(" ") // writes to marked
	h.press("esc")
	h.view()
}

// A secret loaded while its history did not is a real state: a failed
// versionsMsg clears m.versions and m.meta and leaves m.secret. Three sections
// read those fields, and each must answer for itself.
func TestASecretWhoseHistoryFailedStillDraws(t *testing.T) {
	h := newHarness(t)
	h.resize(120, 40)
	h.open("kv/app/prod/db")

	if h.m.secret == nil {
		t.Fatal("precondition: the secret should have loaded")
	}
	// An answer is only taken while its load is open, so the read and the
	// capabilities are staged as arrived and the history is what is missing.
	// Settling waits for all three, so a failing history is always the last.
	n := h.m.sel
	h.m.loading = &secretLoad{
		node: n, wantHistory: true,
		secret: h.m.secret, readVersion: h.m.secretVersion, gotRead: true,
		caps: h.m.capabilities, gotCaps: true,
	}

	h.run(h.m.Data(versionsMsg{node: n, err: vault.ErrForbidden}))

	if h.m.versions != nil || h.m.meta != nil {
		t.Fatal("precondition: a failed history should have cleared both fields")
	}
	view := h.view()
	if strings.Contains(view, "VERSIONS") {
		t.Errorf("there is no history to head a section with:\n%s", view)
	}
	// The header carries the version on its own now. Without a history to count
	// it must not offer a total: the secret read fine and is being shown, so
	// "v4 of 0" would deny the version on screen.
	if !strings.Contains(view, "v4") || strings.Contains(view, "v4 of 0") {
		t.Errorf("the header should name the version without inventing a total:\n%s", view)
	}
	if strings.Contains(view, "METADATA") {
		t.Errorf("there is no metadata to head a section with:\n%s", view)
	}
}
