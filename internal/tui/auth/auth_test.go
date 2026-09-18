package auth

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// stubRequester stands in for the parent where a test only needs the call to be
// built and never run.
type stubRequester struct{}

func (stubRequester) Request(func(context.Context) tea.Msg) tea.Cmd { return nil }

// Roles are keyed by method path, so a late answer lands under the right key.
// That is why rolesMsg needs no stale guard, and this pins it: flattening the
// map into a single slice would reintroduce the bug silently.
func TestRolesAreStoredByMethodPath(t *testing.T) {
	m := New(nil, stubRequester{})

	approle := vault.AuthMethod{Path: "approle/", Type: "approle"}
	userpass := vault.AuthMethod{Path: "userpass/", Type: "userpass"}

	m.Data(rolesMsg{method: approle, roles: []vault.Role{{Name: "ci"}}})
	m.Data(rolesMsg{method: userpass, roles: []vault.Role{{Name: "lucas"}}})

	if got := m.roles["approle/"]; len(got) != 1 || got[0].Name != "ci" {
		t.Errorf("approle roles landed wrong: %+v", got)
	}
	if got := m.roles["userpass/"]; len(got) != 1 || got[0].Name != "lucas" {
		t.Errorf("userpass roles landed wrong: %+v", got)
	}
}

// The same shape as the policy list: auth methods are provisioned elsewhere, so
// a refresh can answer with fewer than the cursor is pointing at.
func TestAShorterAuthMethodListDoesNotCrashTheRefresh(t *testing.T) {
	m := New(nil, stubRequester{})
	m.methods = []vault.AuthMethod{{Path: "a/"}, {Path: "b/"}, {Path: "c/"}, {Path: "d/"}}
	m.methodCursor = 3

	m.Data(methodsMsg{methods: []vault.AuthMethod{{Path: "userpass/"}}})

	if m.MethodCursor() >= len(m.Methods()) {
		t.Errorf("the cursor was left off the end of the list: %d of %d",
			m.MethodCursor(), len(m.Methods()))
	}
}
