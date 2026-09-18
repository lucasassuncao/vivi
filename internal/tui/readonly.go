package tui

import "github.com/lucasassuncao/vivi/internal/app"

// A read-only session refuses a write in four places: the header badge, the
// palette, the key handlers, and the gate every write passes through. Only the
// gate is load-bearing - the other three are about being understood; the gate
// is about being right, and it lives with the tab that writes.

// canWrite reports whether this session may change anything.
func (m *Model) canWrite() bool { return m.access.CanWrite() }

// Refusing a write out loud is the tab that writes; what is left here is the
// standing explanation above and below it.

// writable is the scope shared by every command in the palette that writes.
func writable(m *Model) bool { return m.canWrite() }

// readOnlyBadge is the permanent notice in the header, drawn in the calm
// palette: a refused write is the app working. The loud colours are spent on a
// production address and a token about to expire.
func (m *Model) readOnlyBadge() string {
	return m.st.Badge.Render(" READ-ONLY ")
}

// readOnlyReason is the sentence the help panel shows under its banner. It
// names the setting rather than the state, so someone who did not type the flag
// today can still find out what did.
func (m *Model) readOnlyReason() string {
	switch m.readOnly {
	case app.ReadOnlyAlways:
		return "--read-only was given, or VIVI_READ_ONLY asked for it"
	case app.ReadOnlyOnProd:
		return "read-only on production, and this server is not a recognised sandbox"
	default:
		return "the session was started without write access"
	}
}
