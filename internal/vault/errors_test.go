package vault

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/vault/api"
)

func respErr(status int, messages ...string) error {
	return &api.ResponseError{StatusCode: status, Errors: messages, URL: "https://vault.test/v1/kv/data/x"}
}

func TestClassifySeparatesDeniedFromMissing(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"missing path", respErr(404), ErrNotFound},
		{"sdk secret-not-found", fmt.Errorf("wrapped: %w", api.ErrSecretNotFound), ErrNotFound},
		{"denied by policy", respErr(403, "1 error occurred: permission denied"), ErrForbidden},
		{"sealed", respErr(503, "Vault is sealed"), ErrSealed},
		{"cas rejected", respErr(400, "check-and-set parameter did not match the current version"), ErrCAS},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := wrap("read", "kv/x", tc.err)
			if !errors.Is(got, tc.want) {
				t.Fatalf("got %v, want kind %v", got, tc.want)
			}
		})
	}
}

// An expired token and a missing policy both arrive as 403. Confusing them
// sends the user hunting through policies when they only needed to log in
// again, so the message is what separates them.
func TestClassifyTellsAnExpiredTokenFromAMissingPolicy(t *testing.T) {
	expired := wrap("read", "kv/x", respErr(403, "permission denied: invalid token"))
	if !errors.Is(expired, ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid, got %v", expired)
	}
	if errors.Is(expired, ErrForbidden) {
		t.Fatal("an invalid token must not be reported as a policy problem")
	}

	denied := wrap("read", "kv/x", respErr(403, "permission denied"))
	if !errors.Is(denied, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", denied)
	}
}

func TestErrorKeepsTheServerMessage(t *testing.T) {
	err := wrap("read", "kv/app/db", respErr(403, "permission denied"))

	var ve *Error
	if !errors.As(err, &ve) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if ve.Op != "read" || ve.Path != "kv/app/db" {
		t.Fatalf("lost the operation context: %+v", ve)
	}
	if ve.Detail == "" {
		t.Fatal("the server's own message should survive for display")
	}
}

func TestWrapPassesNilThrough(t *testing.T) {
	if err := wrap("read", "kv/x", nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestUnknownErrorsKeepTheirText(t *testing.T) {
	err := wrap("read", "kv/x", errors.New("dial tcp: connection refused"))
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrForbidden) {
		t.Fatal("a transport failure must not be mistaken for a Vault answer")
	}
	if got := err.Error(); got == "" {
		t.Fatal("the underlying message should be visible")
	}
}

// Vault answers 400 both for a rejected check-and-set and for a mount demanding
// the parameter. Matching "cas" as a bare substring conflated them, so the
// second arrived as a version conflict offering a reload that fixes nothing.
func TestCASRejectionIsNotConfusedWithCASRequired(t *testing.T) {
	cases := []struct {
		name    string
		detail  string
		wantCAS bool
	}{
		{"rejected write", "check-and-set parameter did not match the current version", true},
		{"rejected write, terse", "did not match the current version", true},
		{"mount requires the parameter", "check-and-set parameter required for this call", false},
		{"unrelated message", "casing is wrong somewhere", false},
		{"missing field", "missing required field", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, kind := classify(respErr(400, tc.detail))
			if got := errors.Is(kind, ErrCAS); got != tc.wantCAS {
				t.Errorf("classify(%q) CAS = %v, want %v", tc.detail, got, tc.wantCAS)
			}
		})
	}
}
