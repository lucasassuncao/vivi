package ui

import tea "charm.land/bubbletea/v2"

// Keys is what the keyboard in front of vivi can send for moving around. The
// zero value is a PC keyboard, which has Home, End, PgUp and PgDn.
type Keys struct{ mac bool }

// KeysFor is the keyboard of the system vivi runs on, as runtime.GOOS names it.
func KeysFor(goos string) Keys { return Keys{mac: goos == "darwin"} }

// macStandIns are the keys a Mac terminal keeps for its own scrollback, under
// the ones that reach vivi in their place: the letters less and vim move with.
var macStandIns = map[string]rune{
	"g":      tea.KeyHome,
	"G":      tea.KeyEnd,
	"ctrl+u": tea.KeyPgUp,
	"ctrl+d": tea.KeyPgDown,
}

// StandIn is msg as a movement handler reads it: on a Mac, g arrives as home,
// G as end, ctrl+u as pgup and ctrl+d as pgdown. Never for a handler that
// takes typed text, or a filter would lose its g.
func (k Keys) StandIn(msg tea.KeyPressMsg) tea.KeyPressMsg {
	if code, ok := macStandIns[msg.String()]; k.mac && ok {
		return tea.KeyPressMsg{Code: code}
	}
	return msg
}

// Pages is how the legend names the keys for half a page up and down.
func (k Keys) Pages() string { return k.pick("pgup/pgdn", "ctrl+u/d") }

// PagesHelp is how the help panel names the keys for half a page.
func (k Keys) PagesHelp() string { return k.pick("pgup / pgdn", "ctrl+u / ctrl+d") }

// EndsHelp is how the help panel names the keys for the top and the bottom.
func (k Keys) EndsHelp() string { return k.pick("home / end", "g / G") }

func (k Keys) pick(pc, mac string) string {
	if k.mac {
		return mac
	}
	return pc
}
