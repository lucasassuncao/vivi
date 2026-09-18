package ui

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lucasassuncao/vivi/internal/vault"
)

// Reading a server's answers out loud: what a value looks like on screen, and
// what a failure is called. Both are presentation and both are shared, which is
// why this package knows the vault errors by name and nothing else about them.

// ValueToString renders a Vault value for editing. Structured values become
// JSON so they stay editable rather than turning into Go syntax.
func ValueToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	case json.Number:
		// What the Vault SDK hands over for every number: the digits as written,
		// which is the only form that survives an integer past 2^53.
		return t.String()
	case bool, float64, int, int64:
		return fmt.Sprint(t)
	default:
		if b, err := json.Marshal(t); err == nil {
			return string(b)
		}
		return fmt.Sprint(t)
	}
}

// Humanize turns a Vault error into something worth reading in a footer.
func Humanize(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, vault.ErrForbidden):
		return "permission denied"
	case errors.Is(err, vault.ErrTokenInvalid):
		return "the token expired or was revoked - quit and authenticate again"
	case errors.Is(err, vault.ErrNotFound):
		return "not found"
	case errors.Is(err, vault.ErrUnsupported):
		return "this mount does not support the operation"
	case errors.Is(err, vault.ErrCAS):
		return "the secret changed since it was read"
	case errors.Is(err, vault.ErrSealed):
		return "the Vault is sealed"
	default:
		// The only branch carrying text vivi did not write: the server phrased
		// this one, so it is drawn under the same rules as any other content
		// from the server.
		return Sanitize(err.Error())
	}
}
