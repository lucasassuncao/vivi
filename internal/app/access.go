package app

import (
	"fmt"
	"strings"
)

// Access is how much of the Vault a session may change. It belongs to the
// session, not the operation: the same destroy is gated or refused depending
// only on how vivi was started and which server it is talking to.
type Access int

const (
	// ReadWrite is the ordinary session: every operation is available, gated
	// by how much of it cannot be undone.
	ReadWrite Access = iota

	// ReadOnly refuses every operation that changes the Vault. It covers the
	// session you knew in advance was only for looking - the one you were not
	// being careful in.
	ReadOnly
)

// CanWrite reports whether the session may perform operations at all.
func (a Access) CanWrite() bool { return a == ReadWrite }

func (a Access) String() string {
	if a == ReadOnly {
		return "read-only"
	}
	return "read-write"
}

// ReadOnlyPolicy is when a session refuses to write. It is chosen before the
// session knows which server it is pointed at, so it is a policy rather than
// an Access: it becomes one only when resolved against the address.
type ReadOnlyPolicy int

const (
	// ReadOnlyNever is the default: nothing changes for a session that does
	// not ask, so existing workflows that fix fields on production keep working.
	ReadOnlyNever ReadOnlyPolicy = iota

	// ReadOnlyOnProd refuses writes on a server that is not recognisably a
	// sandbox. This is the setting worth putting in a shell profile: it makes
	// the app act on the environment badge instead of only showing it.
	ReadOnlyOnProd

	// ReadOnlyAlways refuses writes wherever it is pointed.
	ReadOnlyAlways
)

// Access resolves the policy against the server actually connected to.
func (p ReadOnlyPolicy) Access(env Environment) Access {
	switch p {
	case ReadOnlyAlways:
		return ReadOnly
	case ReadOnlyOnProd:
		if env == EnvProd {
			return ReadOnly
		}
		return ReadWrite
	default:
		return ReadWrite
	}
}

// Reason explains, in one clause, why a session came out read-only. The
// interface shows it when a key refuses, since the user may not have typed
// the flag themselves.
func (p ReadOnlyPolicy) Reason() string {
	switch p {
	case ReadOnlyAlways:
		return "read-only was asked for"
	case ReadOnlyOnProd:
		return "this server is not a recognised sandbox"
	default:
		return ""
	}
}

// ParseReadOnly reads the flag or the environment variable. Spellings are
// generous, but an unrecognised value is an error and never a default: a typo
// parsing as "off" hands a writable session to someone who asked for a safe one.
func ParseReadOnly(s string) (ReadOnlyPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "off", "false", "no", "0", "never":
		return ReadOnlyNever, nil
	case "prod", "production":
		return ReadOnlyOnProd, nil
	case "on", "true", "yes", "1", "always":
		return ReadOnlyAlways, nil
	default:
		return ReadOnlyNever, fmt.Errorf(
			"unknown read-only setting %q: use on, off, or prod", s)
	}
}
