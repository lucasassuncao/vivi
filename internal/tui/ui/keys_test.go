package ui

import (
	"testing"

	"github.com/lucasassuncao/bezel/bezeltest"
)

// A Mac terminal keeps Home, End, PgUp and PgDn for its own scrollback, so
// there the letters reach the handlers in their place. Elsewhere every key
// stays what it is: Windows and Linux keyboards have the real ones.
func TestOnAMacTheLettersStandInForTheKeysItsTerminalKeeps(t *testing.T) {
	for _, tc := range []struct{ goos, pressed, want string }{
		{"darwin", "g", "home"},
		{"darwin", "G", "end"},
		{"darwin", "ctrl+u", "pgup"},
		{"darwin", "ctrl+d", "pgdown"},
		{"darwin", "home", "home"},
		{"darwin", "u", "u"},
		{"darwin", "d", "d"},
		{"windows", "g", "g"},
		{"windows", "G", "G"},
		{"windows", "ctrl+d", "ctrl+d"},
		{"linux", "ctrl+u", "ctrl+u"},
	} {
		got := KeysFor(tc.goos).StandIn(bezeltest.Key(tc.pressed)).String()
		if got != tc.want {
			t.Errorf("on %s %q arrived as %q, want %q", tc.goos, tc.pressed, got, tc.want)
		}
	}
}
