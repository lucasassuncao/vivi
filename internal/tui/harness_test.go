package tui

import (
	"strings"
	"sync"

	"charm.land/bubbles/v2/cursor"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lucasassuncao/vivi/internal/vault/fake"
)

// harness drives the model the way bubbletea would, but synchronously: every
// command a key produces is run and its message fed back, so a test can assert
// on the state after the server has answered.
type harness struct {
	t      *testing.T
	m      *Model
	server *fake.Server
	clip   *fakeClipboard
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	srv := fake.New()
	token, err := srv.TokenInfo(t.Context())
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	m := New(Options{Client: srv, Token: token})
	clip := &fakeClipboard{}
	m.clip = clip.write
	h := &harness{t: t, m: m, server: srv, clip: clip}

	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	h.run(m.Init())
	return h
}

// settleTimeout is how long a command gets to answer before the harness calls
// it hung. At 50ms with a silent give-up, every keystroke arming a timer was
// abandoned half-done - 1102 a run. Reaching this now is a real hang.
const settleTimeout = 2 * time.Second

// run executes a command and feeds every resulting message back into the model,
// following batches.
func (h *harness) run(cmd tea.Cmd) {
	h.t.Helper()
	if cmd == nil {
		return
	}

	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()

	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(settleTimeout):
		h.t.Fatalf("a command did not answer within %v: something is waiting on a clock "+
			"the tests have not shrunk, or on something that never arrives", settleTimeout)
	}

	switch msg := msg.(type) {
	case nil:
		return
	case tea.BatchMsg:
		for _, c := range msg {
			h.run(c)
		}
		return
	case bannerExpiredMsg:
		return
	case clockMsg:
		// A timer, like the two above: feeding it back would re-arm it and the
		// harness would follow the chain for as long as the test ran.
		return
	case cursor.BlinkMsg:
		// The caret is the one timer re-arming through the model rather than a
		// command of ours, so the two would loop until the binary is killed.
		// BlinkMsg is exported and loops; initialBlinkMsg arrives once.
		return
	}

	_, next := h.m.Update(msg)
	h.run(next)
}

// press sends a keystroke and settles whatever it started.
func (h *harness) press(keys ...string) {
	h.t.Helper()
	for _, k := range keys {
		_, cmd := h.m.Update(keyMsg(k))
		h.run(cmd)
	}
}

// typeText sends each character as its own keystroke, the way a terminal does.
func (h *harness) typeText(s string) {
	h.t.Helper()
	for _, r := range s {
		h.press(string(r))
	}
}

// paste delivers text the way a bracketed paste arrives: one message carrying
// the whole thing, newlines included. Its own type in v2, where v1 carried it
// as a key message with a flag on it.
func (h *harness) paste(s string) {
	h.t.Helper()
	_, cmd := h.m.Update(tea.PasteMsg{Content: s})
	h.run(cmd)
}

func keyMsg(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case " ", "space":
		// The text matters as much as the code: a text input inserts what the
		// message carries, so a space with no text types nothing. A terminal
		// always sends both.
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	default:
		// A printable keystroke carries both: the code is what the key table
		// matches on, the text is what a widget inserts.
		return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
	}
}

// focus puts the cursor on a visible node by full path, loading whatever
// landing there would load: moving the cursor is what reads a secret, so a test
// that jumped straight to one would otherwise be looking at an empty pane.
func (h *harness) focus(path string) {
	h.t.Helper()
	cmd, found := h.m.secretsTab.FocusPath(path, h.m.uiContext())
	if !found {
		h.t.Fatalf("node %q is not visible; visible: %v", path, h.visiblePaths())
	}
	h.run(cmd)
}

// open expands every ancestor and lands the cursor on the target.
func (h *harness) open(path string) {
	h.t.Helper()

	segments := strings.Split(strings.Trim(path, "/"), "/")
	prefix := ""
	for i, seg := range segments {
		if i == 0 {
			prefix = seg
		} else {
			prefix += "/" + seg
		}
		h.focus(prefix)
		if i < len(segments)-1 {
			h.press("right")
		}
	}
	h.focus(path)
}

func (h *harness) visiblePaths() []string { return h.m.secretsTab.VisiblePaths() }

// view is the screen as text. Stripped, because a v2 style renders its escapes
// whatever it is drawing into: a test looking for a word would otherwise find a
// colour change in the middle of it. What the escapes say is styles_test's.
func (h *harness) view() string { return stripANSI(h.m.render()) }

// mode is the name of what is in front of the user. There are two machines now:
// the shell's own, for the command line and the two panels, and the tab's. The
// shell's answers first, because what it has up is drawn over the tab.
func (h *harness) mode() string {
	if name := h.m.mode.Name(); name != "browse" {
		return name
	}
	return h.m.secretsTab.State().Mode
}

// modalTitle is what the open confirmation asks, or "". The confirmation itself
// belongs to the tab that raised it, and is inspected in that tab's own tests.
func (h *harness) modalTitle() string { return h.m.secretsTab.ModalTitle() }

// Reading the server back is what the tab's own tests do, beside the writes
// they are checking. Nothing here writes any more.

// typeCommand opens the command line and types a whole line into it, one
// keystroke at a time, the way a terminal delivers it.
func (h *harness) typeCommand(line string) {
	h.t.Helper()
	h.press(":")
	if h.mode() != "command" {
		h.t.Fatalf(`":" did not open the command line: mode is %q`, h.mode())
	}
	h.typeText(line)
}

// runCommand types a line and runs it.
func (h *harness) runCommand(line string) {
	h.t.Helper()
	h.typeCommand(line)
	h.press("enter")
}

// offers reports whether a command is currently on the menu.
func offers(h *harness, name string) bool {
	for _, c := range h.m.available() {
		if c.name == name {
			return true
		}
	}
	return false
}

// stripANSI removes escape sequences so a rendered string can be compared with
// its source.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // the "m" itself
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// lineWidth measures a rendered line the way a terminal does, ignoring the
// ANSI escapes lipgloss adds for colour.
func lineWidth(line string) int {
	return lipgloss.Width(line)
}

// resolveCmd runs a command and returns the message it produces, following a
// batch to the one message the caller is asking about.
func resolveCmd(cmd tea.Cmd) tea.Msg {
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return msg
	}
	for _, c := range batch {
		inner := c()
		if inner == nil {
			continue
		}
		return inner
	}
	return nil
}

// Watching the frames between two answers is the Secrets tab's own test, and
// its harness has the collector for it.

// fakeClipboard is what the model writes to instead of the machine's own. Every
// harness has one, so nothing touches the real clipboard and nothing a test
// copies reaches another test, or whoever is running the suite.
type fakeClipboard struct {
	mu      sync.Mutex
	written []string
}

func (c *fakeClipboard) write(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.written = append(c.written, text)
	return nil
}

// copied is the last thing put on the clipboard, or "" if nothing was.
func (h *harness) copied() string {
	h.clip.mu.Lock()
	defer h.clip.mu.Unlock()
	if len(h.clip.written) == 0 {
		return ""
	}
	return h.clip.written[len(h.clip.written)-1]
}
