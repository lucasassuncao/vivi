package app

import (
	"net/url"
	"strings"
	"time"
)

// Environment is how dangerous the connected server is assumed to be.
type Environment int

const (
	EnvLocal Environment = iota
	EnvDev
	EnvProd
)

// Label is the badge text for an environment, and the only place these three
// words are spelled.
func (e Environment) Label() string {
	switch e {
	case EnvLocal:
		return "LOCAL"
	case EnvDev:
		return "DEV"
	default:
		return "PROD"
	}
}

// ClassifyEnvironment guesses what kind of server an address points at. The
// default is PROD on purpose: a wrong PROD badge costs a moment of care, a
// wrong LOCAL badge invites a destroy on a server mistaken for a sandbox.
func ClassifyEnvironment(address string) Environment {
	host := strings.ToLower(address)
	if u, err := url.Parse(address); err == nil && u.Host != "" {
		host = strings.ToLower(u.Hostname())
	}

	switch {
	case host == "localhost" || host == "127.0.0.1" || host == "::1" ||
		strings.HasSuffix(host, ".localhost") || strings.HasPrefix(host, "host.docker.internal"):
		return EnvLocal
	case hasLabel(host, "dev", "stage", "staging", "stg", "test", "qa", "sandbox", "sbx", "lab"):
		return EnvDev
	default:
		return EnvProd
	}
}

// hasLabel reports whether any of the words is a whole label of the host name,
// or a word inside one. A plain substring search would badge "vault.qatar.corp"
// and "prod-devices.corp" as sandboxes, which is the wrong direction to err in.
func hasLabel(host string, words ...string) bool {
	for _, label := range strings.Split(host, ".") {
		for _, part := range strings.FieldsFunc(label, func(r rune) bool {
			return r == '-' || r == '_'
		}) {
			for _, w := range words {
				if part == w {
					return true
				}
			}
		}
	}
	return false
}

// TTLUrgency grades how close the token is to expiring, so the header can warn
// before a session dies mid edit rather than after.
type TTLUrgency int

const (
	TTLFine TTLUrgency = iota
	TTLSoon
	TTLCritical
)

const (
	TTLSoonThreshold     = time.Hour
	TTLCriticalThreshold = 15 * time.Minute
)

// ClassifyTTL grades a remaining lifetime. Zero means "never" (such as root),
// not "now".
func ClassifyTTL(ttl time.Duration) TTLUrgency {
	switch {
	case ttl <= 0:
		return TTLFine
	case ttl <= TTLCriticalThreshold:
		return TTLCritical
	case ttl <= TTLSoonThreshold:
		return TTLSoon
	default:
		return TTLFine
	}
}
