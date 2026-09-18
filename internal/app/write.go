package app

import "github.com/lucasassuncao/vivi/internal/vault"

// CAS is the check-and-set value a write must carry, and each case loses data
// quietly when wrong: creating means "must not exist", a v1 mount is
// unconditional and the modal says so, and v2 carries the version opened on.
func CAS(creating bool, kvVersion, baseVersion int) int {
	switch {
	case creating:
		return 0
	case kvVersion == vault.KV1:
		return vault.CASDisabled
	default:
		return baseVersion
	}
}

// CanEdit reports whether the version on screen may be edited in place. Saving
// an edit of a historical one writes it back as newest - a rollback in an
// edit's clothes - and rollback has its own gesture. showing 0 means current.
func CanEdit(showing, current int) bool {
	return showing == 0 || current == 0 || showing == current
}
