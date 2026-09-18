package vault

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// stubClient answers only what Preflight asks. The embedded interface is nil,
// so anything else Preflight starts calling panics rather than failing to
// compile - a deliberate trade for a stub this small.
type stubClient struct {
	Client
	info   *TokenInfo
	err    error
	health ServerInfo
}

func (s stubClient) TokenInfo(context.Context) (*TokenInfo, error) { return s.info, s.err }

// Health is asked after the token check and its failure is not fatal, so the
// stub answers rather than refusing: what is being tested is that Preflight
// still returns the token.
func (s stubClient) Health(context.Context) (ServerInfo, error) { return s.health, nil }

func TestPreflightRejectsAMissingAddress(t *testing.T) {
	_, err := Preflight(context.Background(), Config{Token: "t"}, stubClient{})

	var pe *PreflightError
	if !errors.As(err, &pe) || pe.Check != "VAULT_ADDR" {
		t.Fatalf("expected a VAULT_ADDR failure, got %v", err)
	}
	if !strings.Contains(pe.Error(), "export VAULT_ADDR") {
		t.Fatal("the failure must say how to fix it")
	}
}

func TestPreflightRejectsAMissingToken(t *testing.T) {
	_, err := Preflight(context.Background(), Config{Address: "https://v"}, stubClient{})

	var pe *PreflightError
	if !errors.As(err, &pe) || pe.Check != "VAULT_TOKEN" {
		t.Fatalf("expected a VAULT_TOKEN failure, got %v", err)
	}
	if !strings.Contains(pe.Error(), "vault login") {
		t.Fatal("the failure should point at vault login")
	}
}

func TestPreflightExplainsARejectedToken(t *testing.T) {
	cfg := Config{Address: "https://v", Token: "expired"}
	_, err := Preflight(context.Background(), cfg, stubClient{err: &Error{Kind: ErrTokenInvalid}})

	var pe *PreflightError
	if !errors.As(err, &pe) || pe.Check != "token" {
		t.Fatalf("expected a token failure, got %v", err)
	}
	if !strings.Contains(pe.Error(), "expired or revoked") {
		t.Fatalf("the reason should name the cause: %v", pe)
	}
}

func TestPreflightExplainsASealedServer(t *testing.T) {
	cfg := Config{Address: "https://v", Token: "t"}
	_, err := Preflight(context.Background(), cfg, stubClient{err: &Error{Kind: ErrSealed}})

	var pe *PreflightError
	if !errors.As(err, &pe) || pe.Check != "server" {
		t.Fatalf("expected a server failure, got %v", err)
	}
	if !strings.Contains(pe.Error(), "sealed") {
		t.Fatalf("the reason should say the Vault is sealed: %v", pe)
	}
}

// Anything that is not a Vault answer is a connection problem, and saying so
// beats printing a transport error the user cannot act on.
func TestPreflightTreatsAnythingElseAsUnreachable(t *testing.T) {
	cfg := Config{Address: "https://v", Token: "t"}
	_, err := Preflight(context.Background(), cfg, stubClient{err: errors.New("dial tcp: refused")})

	var pe *PreflightError
	if !errors.As(err, &pe) || pe.Check != "connection" {
		t.Fatalf("expected a connection failure, got %v", err)
	}
	if !strings.Contains(pe.Error(), "VAULT_CACERT") {
		t.Fatal("the hint should mention TLS trust, a common cause")
	}
}

// "cannot reach" is the bucket everything unclassified falls into, so it is the
// one that has to carry the cause: an unknown CA, a proxy 502 and a dead DNS
// name are one message otherwise, and none of them is the address being wrong.
func TestPreflightPrintsTheUnderlyingCause(t *testing.T) {
	cfg := Config{Address: "https://v", Token: "t"}
	cause := errors.New("x509: certificate signed by unknown authority")

	_, err := Preflight(context.Background(), cfg, stubClient{err: cause})

	var pe *PreflightError
	if !errors.As(err, &pe) || pe.Check != "connection" {
		t.Fatalf("expected a connection failure, got %v", err)
	}
	if !strings.Contains(pe.Error(), "x509") {
		t.Fatalf("the cause is the only thing that tells TLS from DNS: %q", pe.Error())
	}
}

func TestPreflightReturnsTheTokenOnSuccess(t *testing.T) {
	want := &TokenInfo{DisplayName: "userpass-lucas"}
	cfg := Config{Address: "https://v", Token: "t"}

	got, err := Preflight(context.Background(), cfg, stubClient{info: want})
	if err != nil {
		t.Fatalf("unexpected failure: %v", err)
	}
	if got != want {
		t.Fatal("Preflight should hand the token to the caller for the header")
	}
}
