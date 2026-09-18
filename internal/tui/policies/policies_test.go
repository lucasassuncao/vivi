package policies

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// stubRequester stands in for the parent where a test only needs the call to be
// built and never run: afterList asks for the document of whatever it lands on.
type stubRequester struct{}

func (stubRequester) Request(func(context.Context) tea.Msg) tea.Cmd { return nil }

// A policy read that arrives after the cursor has moved must be discarded. The
// alternative is one policy's HCL rendered under another policy's name, which
// in an auditing tool is worse than showing nothing at all.
func TestStalePolicyAnswerIsDropped(t *testing.T) {
	m := New(nil, nil)
	m.Restore([]string{"admin", "readonly"}, 1, // the cursor is on "readonly"
		"path \"kv/*\" { capabilities = [\"read\"] }")
	before := m.Doc()

	m.Data(docMsg{name: "admin", hcl: "path \"*\" { capabilities = [\"sudo\"] }"})

	if m.Doc() != before {
		t.Fatalf("a stale answer for %q overwrote the pane: %q", "admin", m.Doc())
	}
}

// The answer for the policy actually under the cursor still lands.
func TestFreshPolicyAnswerIsApplied(t *testing.T) {
	m := New(nil, nil)
	m.Restore([]string{"admin", "readonly"}, 1, "")

	m.Data(docMsg{name: "readonly", hcl: "path \"kv/*\" { capabilities = [\"read\"] }"})

	if m.Doc() == "" {
		t.Fatal("the answer for the selected policy was dropped")
	}
}

// A list coming back shorter than the cursor points at turns a refresh into a
// crash, and policies are provisioned elsewhere so it can shrink. The handler
// checked the list was not empty and then indexed it - two different questions.
func TestAShorterPolicyListDoesNotCrashTheRefresh(t *testing.T) {
	m := New(nil, stubRequester{})
	// Sitting on the third policy when the list comes back with two.
	m.Restore([]string{"a", "b", "c"}, 2, "")

	m.Data(listMsg{names: []string{"app-admin", "default"}})

	if m.Cursor() >= len(m.Names()) {
		t.Errorf("the cursor was left off the end of the list: %d of %d",
			m.Cursor(), len(m.Names()))
	}
}

// A stale error is noise too: it names a policy the user has already left, and
// it would clear the HCL of the one they are actually looking at.
func TestStalePolicyErrorIsDropped(t *testing.T) {
	m := New(nil, nil)
	m.Restore([]string{"admin", "readonly"}, 1,
		"path \"kv/*\" { capabilities = [\"read\"] }")

	cmd := m.Data(docMsg{name: "admin", err: errors.New("read policy failed")})

	if cmd != nil {
		t.Error("a stale error produced a footer message")
	}
	if m.Doc() == "" {
		t.Error("a stale error cleared the HCL of the selected policy")
	}
}
