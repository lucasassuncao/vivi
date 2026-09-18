package vault

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/vault/api"
)

// Error kinds. Callers test with errors.Is; the *Error wrapper carries the
// detail worth showing.
var (
	// ErrNotFound is a path with nothing at it.
	ErrNotFound = errors.New("not found")

	// ErrForbidden is the token lacking a policy for this path. Vault answers
	// 403, but also 404 for some paths a token may not read: a denied path must
	// render as denied, or an audit shows a Vault smaller than it is.
	ErrForbidden = errors.New("permission denied")

	// ErrTokenInvalid is a token that expired or was revoked mid-session, as
	// opposed to a token that is merely missing a policy.
	ErrTokenInvalid = errors.New("token is invalid or has expired")

	// ErrSealed is a server that cannot serve anything until it is unsealed.
	ErrSealed = errors.New("vault is sealed")

	// ErrUnsupported is an operation the mount cannot do, such as asking a
	// KV v1 mount for version history.
	ErrUnsupported = errors.New("operation not supported by this mount")

	// ErrCAS is a write rejected because the secret changed since it was read.
	ErrCAS = errors.New("secret was modified since it was read")
)

// Error is a failed Vault operation, keeping the kind for errors.Is and the
// server's own message for display.
type Error struct {
	Op     string
	Path   string
	Kind   error
	Detail string
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(e.Op)
	if e.Path != "" {
		b.WriteString(" ")
		b.WriteString(e.Path)
	}
	b.WriteString(": ")
	if e.Kind != nil {
		b.WriteString(e.Kind.Error())
	} else {
		b.WriteString("failed")
	}
	if e.Detail != "" && (e.Kind == nil || e.Detail != e.Kind.Error()) {
		b.WriteString(" (")
		b.WriteString(e.Detail)
		b.WriteString(")")
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.Kind }

// wrap turns an SDK error into an *Error with a kind vivi can branch on.
// Returns nil for a nil error so call sites can wrap unconditionally.
func wrap(op, path string, err error) error {
	if err == nil {
		return nil
	}
	detail, kind := classify(err)
	return &Error{Op: op, Path: path, Kind: kind, Detail: detail}
}

// classify maps an SDK error to one of the sentinel kinds. 403 is the subtle
// one: Vault sends it both for "your policy does not cover this" and "your
// token is invalid", and only the message separates them.
func classify(err error) (detail string, kind error) {
	if errors.Is(err, api.ErrSecretNotFound) {
		return "", ErrNotFound
	}

	var respErr *api.ResponseError
	if !errors.As(err, &respErr) {
		return err.Error(), nil
	}

	detail = strings.Join(respErr.Errors, "; ")
	lower := strings.ToLower(detail)

	switch respErr.StatusCode {
	case http.StatusNotFound:
		return detail, ErrNotFound
	case http.StatusForbidden:
		if strings.Contains(lower, "invalid token") ||
			strings.Contains(lower, "token expired") ||
			strings.Contains(lower, "token is expired") ||
			strings.Contains(lower, "bad token") {
			return detail, ErrTokenInvalid
		}
		return detail, ErrForbidden
	case http.StatusBadRequest:
		// A rejected check-and-set arrives as 400 naming the expected version.
		// Matching "cas" as a bare substring also caught the mount that requires
		// the parameter, offering a reload that fixes nothing.
		if isCASRejection(lower) {
			return detail, ErrCAS
		}
		return detail, nil
	case http.StatusServiceUnavailable:
		if strings.Contains(lower, "sealed") {
			return detail, ErrSealed
		}
		return detail, nil
	default:
		if detail == "" {
			detail = fmt.Sprintf("HTTP %d", respErr.StatusCode)
		}
		return detail, nil
	}
}

// isCASRejection tells a 400 refusing a write because the version moved from
// one refusing it for want of the parameter: "did not match the current
// version" against "required for this call", which is a configuration problem.
func isCASRejection(lower string) bool {
	if strings.Contains(lower, "did not match the current version") {
		return true
	}
	return strings.Contains(lower, "check-and-set") &&
		!strings.Contains(lower, "required")
}
