package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
	"github.com/lucasassuncao/vivi/internal/vault/fake"
)

// The model itself: how it is built and what bounds the requests it issues.
// What it does with the answers is in update_test.go; what each tab does with
// a keystroke is in the file named after that tab.

// Every request is bounded by the program's own context, so quitting cancels
// what is still in flight instead of leaving it to run out its own timeout
// against a server that is no longer being waited on.
func TestQuittingCancelsRequestsInFlight(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	srv := fake.New()
	m := New(Options{Client: srv, Token: &vault.TokenInfo{}, Ctx: ctx})

	// The command captures the context when it is built, and reads it when it
	// runs: a request made before the cancel still sees it.
	cmd := m.request(func(reqCtx context.Context, _ vault.Client) ui.RequestAnswer {
		return ctxErrMsg{err: reqCtx.Err()}
	})

	cancel()

	if got, _ := resolveCmd(cmd).(ctxErrMsg); !errors.Is(got.err, context.Canceled) {
		t.Errorf("a request outliving the program: err = %v, want context.Canceled", got.err)
	}
}

// ctxErrMsg hands a request's view of its own context back to the test. A
// message type and not a bare error because request takes nothing else: the
// suite is not exempt from the rule the compiler enforces on the app.
type ctxErrMsg struct{ err error }

func (ctxErrMsg) AnswersRequest() {}

// Without a context the browser still works: the zero Options is a valid one.
func TestModelWithoutAContextStillRuns(t *testing.T) {
	srv := fake.New()
	m := New(Options{Client: srv, Token: &vault.TokenInfo{}})

	cmd := m.request(func(reqCtx context.Context, _ vault.Client) ui.RequestAnswer {
		return ctxErrMsg{err: reqCtx.Err()}
	})
	if got, _ := resolveCmd(cmd).(ctxErrMsg); got.err != nil {
		t.Errorf("a request with no program context failed: %v", got.err)
	}
}

// The predicates over the mode used to be string comparisons, so isMode
// ("version") where the mode is "versions" compiled, answered false forever,
// and silently disabled whatever it guarded.
func TestModePredicatesFollowTheMode(t *testing.T) {
	h := newHarness(t)

	if !modeIs[browsing](h.m) {
		t.Fatalf("a fresh model is not browsing: %s", h.m.currentMode().Name())
	}

	// The tab's own modes are the tab's to report, and the shell's machine is
	// unmoved by them: ":" opened over the version list leaves it open.
	h.open("kv/app/prod/db")
	h.press("v")
	if got := h.m.secretsTab.State().Mode; got != "versions" {
		t.Errorf(`"v" did not open the version list: %s`, got)
	}

	h.press(":")
	if !modeIs[commanding](h.m) {
		t.Errorf(`":" did not open the command line: %s`, h.m.currentMode().Name())
	}
	if got := h.m.secretsTab.State().Mode; got != "versions" {
		t.Errorf("the command line closed the version list behind it: %s", got)
	}
	if !onVersionList(h.m) {
		t.Error("a command typed over the version list is not judged against it")
	}
}
