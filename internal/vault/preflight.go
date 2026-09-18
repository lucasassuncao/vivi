package vault

import (
	"context"
	"errors"
	"fmt"
)

// PreflightError is a startup check that failed, with the remedy attached. vivi
// refuses the alternate screen without a working connection: a TUI opening onto
// an empty tree cannot tell a broken token from an empty Vault.
type PreflightError struct {
	Check  string
	Reason string
	Hint   string
	Err    error
}

func (e *PreflightError) Error() string {
	msg := fmt.Sprintf("%s: %s", e.Check, e.Reason)
	if e.Hint != "" {
		msg += "\n  " + e.Hint
	}
	// "cannot reach" is where everything unclassified lands, so the cause is
	// what separates an unknown CA from a dead name from a proxy refusing.
	if e.Err != nil {
		msg += "\n  " + e.Err.Error()
	}
	return msg
}

func (e *PreflightError) Unwrap() error { return e.Err }

// preflightClient is the two roles the checks touch: a Client passes, and a
// test needs to fake no more than this.
type preflightClient interface {
	Connection
	TokenHolder
}

// Preflight verifies, in order, that there is an address, a token, a server
// that answers, and a token the server still accepts. It returns the token's
// own description on success, since that is the first thing the header shows.
func Preflight(ctx context.Context, cfg Config, c preflightClient) (*TokenInfo, error) {
	if cfg.Address == "" {
		return nil, &PreflightError{
			Check:  "VAULT_ADDR",
			Reason: "not set",
			Hint:   `export VAULT_ADDR=https://your-vault:8200`,
		}
	}
	if cfg.Token == "" {
		return nil, &PreflightError{
			Check:  "VAULT_TOKEN",
			Reason: "not set, and no token cached in ~/.vault-token",
			Hint:   `run "vault login" or export VAULT_TOKEN=...`,
		}
	}

	info, err := c.TokenInfo(ctx)
	if err != nil {
		switch {
		case errors.Is(err, ErrTokenInvalid), errors.Is(err, ErrForbidden):
			return nil, &PreflightError{
				Check:  "token",
				Reason: "rejected by " + cfg.Address,
				Hint:   `the token is expired or revoked; run "vault login" for a new one`,
				Err:    err,
			}
		case errors.Is(err, ErrSealed):
			return nil, &PreflightError{
				Check:  "server",
				Reason: cfg.Address + " is sealed",
				Hint:   "an operator has to unseal it before anything can be read",
				Err:    err,
			}
		default:
			return nil, &PreflightError{
				Check:  "connection",
				Reason: "cannot reach " + cfg.Address,
				Hint:   "check the address, your network, and TLS trust (VAULT_CACERT)",
				Err:    err,
			}
		}
	}

	// Health is asked last and its failure is not fatal: it is unauthenticated
	// on a real Vault, but a proxy can refuse it, and a version string is not
	// worth turning a working session away over.
	_, _ = c.Health(ctx)

	return info, nil
}
