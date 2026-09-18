package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"
)

// What a panic costs, and the one part worth catching. Bubble Tea restores the
// terminal already, so nothing here stops a panic: it writes down the work in
// flight, which is what nobody else can reconstruct.

// crashFileMode keeps the report to its owner. It names paths inside a Vault,
// which is not a secret but is not public either.
const crashFileMode = 0o600

// guardPanic is deferred by Update and View. It writes the report and re-raises
// the panic, so the program still ends through Bubble Tea's own handler and the
// terminal is restored exactly as it would have been.
func (m *Model) guardPanic() {
	r := recover()
	if r == nil {
		return
	}

	// Captured before the re-raise: this is the stack of the original panic.
	// The one Bubble Tea prints afterwards is this function's, which says where
	// the panic was re-thrown and not where it came from.
	report := fmt.Sprintf("vivi crash %s\n\npanic: %v\n\n%s\n%s",
		time.Now().Format(time.RFC3339), r, safeNote(m), debug.Stack())

	if path := writeCrashNote(report); path != "" {
		// Carried in the panic value rather than printed here. This runs before
		// Bubble Tea's own recover, so the terminal is still in the alternate
		// screen and anything written to it now is wiped by the restore.
		panic(crashPanic{value: r, path: path})
	}
	panic(r)
}

// crashPanic is what guardPanic re-raises once the report is on disk: the
// original value, kept as it was, and where the report went. Bubble Tea prints
// it through Error, so the terminal reads the same as before.
type crashPanic struct {
	value any
	path  string
}

func (c crashPanic) Error() string {
	return fmt.Sprintf("%v\n\nvivi: crash report written to %s", c.value, c.path)
}

// Unwrap exposes the original panic when it was an error, so errors.Is and
// errors.As see through the report to it.
func (c crashPanic) Unwrap() error {
	err, _ := c.value.(error)
	return err
}

// safeNote describes what was in flight without trusting the model, which is by
// definition in an unknown state. A panic here would skip every remaining
// defer - including the one that restores the terminal - so it degrades instead.
func safeNote(m *Model) (note string) {
	defer func() {
		if second := recover(); second != nil {
			note = fmt.Sprintf("(the report could not be built: %v)\n", second)
		}
	}()
	return m.crashNote()
}

// crashNote is where the user was and what they had not saved. Keys, never
// values: a crash report is a file on disk, and the path plus the key names are
// what it takes to redo the work. The values the user still has.
func (m *Model) crashNote() string {
	var b strings.Builder

	fmt.Fprintf(&b, "tab: %s\n", tabNames[min(max(int(m.tab), 0), len(tabNames)-1)])
	fmt.Fprintf(&b, "size: %dx%d\n", m.width, m.height)
	if m.mode != nil {
		fmt.Fprintf(&b, "mode: %s\n", m.mode.Name())
	}
	// The rest is the Secrets tab's: where the cursor was, and what was typed
	// into an editor and not yet saved. It is the only tab with unsaved work.
	b.WriteString(m.secretsTab.CrashNote())

	return b.String()
}

// writeCrashNote saves the report and returns where, or "" if it could not be
// written - which is not worth reporting, because the only place left to report
// it is the terminal this is in the middle of giving back.
func writeCrashNote(report string) string {
	name := filepath.Join(os.TempDir(),
		fmt.Sprintf("vivi-crash-%s.txt", time.Now().Format("20060102-150405")))
	if err := os.WriteFile(name, []byte(report), crashFileMode); err != nil {
		return ""
	}
	return name
}
