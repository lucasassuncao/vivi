package app

import "github.com/lucasassuncao/vivi/internal/vault"

// Operation is something vivi can do that changes the Vault. Reads are absent
// on purpose: nothing that only reads earns a gate.
type Operation int

const (
	OpSave Operation = iota
	OpCreate
	OpDelete
	OpUndelete
	OpRollback
	OpDestroy
	OpDeleteMetadata
)

// Gate is how much friction an operation earns before it runs.
type Gate int

const (
	// GateNone runs immediately. Nothing is lost, so nothing is asked.
	GateNone Gate = iota
	// GateConfirm asks yes or no.
	GateConfirm
	// GateTyped requires the secret's name to be reproduced by hand.
	GateTyped
	// GateRefused is an operation this session may not perform at all. It is
	// the top of the same scale, so a caller cannot ask "how much does this
	// cost" without also learning "it does not run".
	GateRefused
)

// GateFor grades an operation on reversibility alone: nothing lost asks
// nothing, recoverable asks yes/no, gone for good wants the name typed. An
// early vivi gated v1 delete with a yes/no - data loss as a UI decision.
func GateFor(op Operation, kvVersion int, access Access) Gate {
	// Answered before looking at the operation: Operation only names writes,
	// so renewing the token stays allowed in a read-only session - a session
	// that cannot renew ends in the middle of the reading it was opened for.
	if !access.CanWrite() {
		return GateRefused
	}

	switch op {
	case OpUndelete, OpRollback:
		return GateNone

	case OpDestroy, OpDeleteMetadata:
		return GateTyped

	case OpDelete:
		if kvVersion == vault.KV1 {
			return GateTyped
		}
		return GateConfirm

	default:
		// Save and create. A v1 write overwrites the only copy, but the gate
		// stays yes/no; saying so is Warns' job.
		return GateConfirm
	}
}

// Warns reports whether an operation destroys something this mount cannot
// recover, and so needs saying beyond the gate. The gate decides how hard it is
// to say yes; this decides whether the user hears the whole truth first.
func Warns(op Operation, kvVersion int) bool {
	if kvVersion != vault.KV1 {
		return false
	}
	switch op {
	case OpSave, OpDelete:
		return true
	default:
		return false
	}
}
