package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// A TUI cannot printf: stdout is the surface it draws on. So when VIVI_DEBUG
// names a file, cmd opens it and every key that arrives is written there, which
// is the only way to see what the terminal actually sent.

// debugKey records what a keystroke arrived as. String() is what the key table
// matches on; the code, the text and the modifiers are what it was built from,
// and a layout that sends something unexpected shows the difference here.
func (m *Model) debugKey(msg tea.KeyPressMsg) {
	if m.debug == nil {
		return
	}
	fmt.Fprintf(m.debug, "key  string=%q  code=%v  text=%q  mod=%v\n",
		msg.String(), msg.Code, msg.Text, msg.Mod)
}
