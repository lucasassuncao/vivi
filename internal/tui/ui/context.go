package ui

import "github.com/lucasassuncao/vivi/internal/app"

// The vocabulary the parent and a tab both have to speak. It lives here and not
// in either of them because the parent imports every tab, so anything a tab has
// to name cannot come from the parent without a cycle.

// Focus is which of the two panes the keyboard drives. Only one pane holds it
// at a time and the border says which.
type Focus int

const (
	FocusList Focus = iota
	FocusDetail
)

// Mode is what the interface is doing, with whatever that takes riding inside
// it: "editing with no editor" is a state that cannot be written down. As an
// enum beside nullable fields it could, and a failed rollback did exactly that.
//
// The interface lives here and not in the parent because four of the modes
// belong to the Secrets tab and will be declared in its package, which the
// parent imports: a type satisfying an interface of the package that imports it
// is a cycle.
type Mode interface{ Name() string }

// Context is the session state a tab reads and does not own. Passed by value on
// every call, never held: a tab that stored it would be reading last frame's
// focus, and a tab that could write it would be the parent again.
type Context struct {
	Styles Styles
	Focus  Focus

	// Width and Height are the body the tab is drawing into, borders already
	// taken off. A tab measures nothing about the terminal itself.
	Width, Height int

	// TokenPolicies are the policies this session's token names, which the
	// Secrets pane reads backwards to say which of them grants the path in
	// front of the user. Empty until the token lookup answers.
	TokenPolicies []string

	// Access and ReadOnly are fixed for the life of the session. A tab needs
	// them to refuse a write where it is typed rather than after it is sent,
	// and to say why rather than only that.
	Access   app.Access
	ReadOnly app.ReadOnlyPolicy
}

// LegendEntry is one key and what it does, for the footer under the panes.
type LegendEntry struct {
	Key, Action string

	// Writes marks an entry a read-only session refuses, so legend can drop it.
	// It rides on the entry because a lookup table keyed on key strings drifted:
	// it advertised four refused keys and dropped "d diff", which works.
	Writes bool
}

// HelpSection is one titled block of the "?" panel.
type HelpSection struct {
	Title string
	Rows  [][2]string
}

// CopyRow is one line the copy panel offers.
type CopyRow struct {
	Label, Text string
	// IsCommand separates a line holding a CLI command from one holding an
	// address, which is the whole of the difference between their two names.
	IsCommand bool
}

// CommandName is what the line is called in the palette. A command line says
// so: ":copy-list-command" copies "vault kv list ...", where ":copy-path"
// copies the path itself.
func (r CopyRow) CommandName() string {
	if r.IsCommand {
		return "copy-" + r.Label + "-command"
	}
	return "copy-" + r.Label
}
