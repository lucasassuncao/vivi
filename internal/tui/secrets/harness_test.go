package secrets

import (
	"context"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/cursor"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
	"github.com/lucasassuncao/vivi/internal/vault/fake"
)

// harness drives the tab the way the shell would, but synchronously: every
// command a key produces is run and its message fed back, so a test can assert
// on the state after the server has answered.
//
// It also stands in for the shell, which is what makes it useful here: the
// requests it wraps, the banners it would print, the focus it would move. Those
// are collected rather than acted on, and a test reads them back.
type harness struct {
	t      *testing.T
	m      *Model
	server *fake.Server
	ctx    ui.Context

	banners []ui.BannerMsg
	copies  []ui.CopyMsg
	records []ui.RecordedMsg
}

// requester is the shell's half of a call: the timeout and the context.
type requester struct{ t *testing.T }

func (r requester) Request(call func(context.Context) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(r.t.Context(), settleTimeout)
		defer cancel()
		return call(ctx)
	}
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	srv := fake.New()
	h := &harness{
		t:      t,
		m:      New(srv, requester{t: t}),
		server: srv,
		ctx:    newContext(),
	}

	// The shell looks the token up and hands the tab the policies it names,
	// which is what the reverse lookup in the access section reads.
	if token, err := srv.TokenInfo(t.Context()); err == nil && token != nil {
		h.ctx.TokenPolicies = token.Policies
	}

	h.run(h.m.Load())
	return h
}

// readOnlyHarness is newHarness with the session refusing writes. Read-only is
// a property of the session, so here it is a property of the context.
func readOnlyHarness(t *testing.T, policy app.ReadOnlyPolicy) *harness {
	t.Helper()

	h := newHarness(t)
	h.ctx.Access, h.ctx.ReadOnly = app.ReadOnly, policy
	return h
}

// newContext is the session as this tab sees it: an ordinary writable one, on
// a terminal wide enough for two panes.
func newContext() ui.Context {
	return ui.Context{
		Styles: ui.NewStyles(ui.Colors{}, true),
		Focus:  ui.FocusList,
		Width:  120,
		Height: 40 - chromeRows,
	}
}

// settleTimeout is how long a command gets to answer before the harness calls
// it hung. Reaching it is a real hang, not a slow test.
const settleTimeout = 2 * time.Second

// run executes a command and feeds every resulting message back, following
// batches. What the tab asks of the shell is collected instead.
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
		h.t.Fatalf("a command did not answer within %v", settleTimeout)
	}

	switch msg := msg.(type) {
	case nil:
		return
	case tea.BatchMsg:
		for _, c := range msg {
			h.run(c)
		}
		return
	case cursor.BlinkMsg:
		// The caret re-arms through the model rather than a command, so feeding
		// it back would loop until the binary is killed.
		return

	// What the tab asks of the shell. The shell is this harness now.
	case ui.BannerMsg:
		h.banners = append(h.banners, msg)
		return
	case ui.CopyMsg:
		h.copies = append(h.copies, msg)
		return
	case ui.RecordedMsg:
		h.records = append(h.records, msg)
		return
	case ui.FocusMsg:
		h.ctx.Focus = msg.Focus
		return
	case ui.ScrollTopMsg, ui.ScrollToLineMsg, ui.ShowSecretsMsg, ui.OpenCmdlineMsg:
		// The detail viewport and the shell's panels are not here. What the tab
		// wanted of them is not what these tests are about.
		return
	}

	h.run(h.m.Data(msg))
}

// press sends a keystroke and settles whatever it started, routing it the way
// the shell does: with the detail pane focused the tab gets first refusal, and
// only what it declines falls through to the ordinary handler.
func (h *harness) press(keys ...string) {
	h.t.Helper()
	for _, k := range keys {
		msg := keyMsg(k)

		// A modal in the tab takes every key, exactly as the shell's routing
		// stands aside for it. Only with the panes in front does the focused
		// pane get its turn first.
		if !h.m.Captures() && h.ctx.Focus == ui.FocusDetail {
			if cmd, took := h.m.KeyDetail(msg, h.ctx); took {
				h.run(cmd)
				continue
			}
			// esc and left leave the pane, which is the shell's doing.
			if k == "esc" || k == "left" {
				h.ctx.Focus = ui.FocusList
				continue
			}
		}
		h.run(h.m.Key(msg, h.ctx))
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
// the whole thing, newlines included.
func (h *harness) paste(s string) {
	h.t.Helper()
	h.run(h.m.Data(tea.PasteMsg{Content: s}))
}

func keyMsg(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case " ", "space":
		// The text matters as much as the code: a text input inserts what the
		// message carries, so a space with no text types nothing.
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
func (h *harness) focus(path string) *node {
	h.t.Helper()
	h.m.adopt(h.ctx)
	for i, r := range h.m.tree.visible {
		if r.kind != rowNode {
			continue
		}
		n := r.node
		if n.fullPath() == path || (n.kind != kindSecret && n.fullPath() == strings.TrimSuffix(path, "/")+"/") {
			h.m.tree.cursor = i
			h.run(h.m.onCursorMoved())
			return n
		}
	}
	h.t.Fatalf("node %q is not visible; visible: %v", path, h.visiblePaths())
	return nil
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

// mountNamed returns a mount from the forest. It reads the roots rather than
// the visible rows, so it keeps working however the visible list is shaped.
func (h *harness) mountNamed(name string) *node {
	h.t.Helper()
	for _, n := range h.m.tree.roots {
		if n.name == name {
			return n
		}
	}
	h.t.Fatalf("mount %q is not in the tree", name)
	return nil
}

func (h *harness) visiblePaths() []string {
	out := make([]string, 0, len(h.m.tree.visible))
	for _, r := range h.m.tree.visible {
		if r.kind == rowNode {
			out = append(out, r.node.fullPath())
		}
	}
	return out
}

// view is the tab's two panes and whatever it has over them, as text. Stripped,
// because a v2 style renders its escapes whatever it is drawing into: a test
// looking for a word would otherwise find a colour change in the middle of it.
func (h *harness) view() string {
	var b strings.Builder
	if over := h.m.Overlay(h.ctx); over != "" {
		b.WriteString(over)
		b.WriteString("\n")
	}
	// The titles the shell sets into the panel edges. They are the tab's words,
	// so a test looking for "creating kv/..." is looking at this.
	left, right := h.m.Titles()
	b.WriteString(left + "\n")
	b.WriteString(h.m.RenderList(48, h.ctx.Height, h.ctx))
	b.WriteString("\n" + right + "\n")
	b.WriteString(h.m.RenderDetail(h.ctx.Width-52, h.ctx))
	return stripANSI(b.String())
}

// mode is the name of the current mode, which is what assertions care about
// when they are not reaching for its payload.
func (h *harness) mode() string { return h.m.currentMode().Name() }

// modal is the open confirmation, or nil when the tab is not in one. The
// payload lives inside the mode, so a test cannot look at a modal that is not
// on screen - which is the point of it living there.
func (h *harness) modal() *confirmation {
	if c, ok := h.m.currentMode().(confirming); ok {
		return c.confirmation
	}
	return nil
}

// editor is the open field editor, or nil.
func (h *harness) editor() *editor { return h.m.currentEditor() }

// banner is the last thing the tab asked the footer to say.
func (h *harness) banner() string {
	if len(h.banners) == 0 {
		return ""
	}
	last := h.banners[len(h.banners)-1]
	if last.Cause != nil {
		return last.Text + ": " + ui.Humanize(last.Cause)
	}
	return last.Text
}

// bannerErr reports whether the last thing the tab said was a failure.
func (h *harness) bannerErr() bool {
	if len(h.banners) == 0 {
		return false
	}
	last := h.banners[len(h.banners)-1]
	return last.Err || last.Cause != nil
}

// copyRows is the Command Reference the tab would fill.
func (h *harness) copyRows() []ui.CopyRow { return h.m.CopyRows(h.ctx) }

// collect runs a command and returns every message it produced, batches
// flattened, without feeding any of them back.
func (h *harness) collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}

	var out []tea.Msg
	for _, c := range batch {
		out = append(out, h.collect(c)...)
	}
	return out
}

// copied is the last text the tab asked to put on the clipboard.
func (h *harness) copied() string {
	if len(h.copies) == 0 {
		return ""
	}
	return h.copies[len(h.copies)-1].Text
}

// chromeRows is what the shell spends above and below the panes: the two header
// lines, the panel borders and the footer. The tab is handed the body that is
// left, and the tests speak in terminal sizes, so the harness takes it off here.
const chromeRows = 6

// resize is the window size message the shell would fold in, which is what
// re-lays the one widget measured against the terminal rather than at render.
func (h *harness) resize(width, height int) {
	h.ctx.Width, h.ctx.Height = width, max(1, height-chromeRows)
	h.m.Resize(h.ctx)
}

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
// escapes that occupy no columns.
func lineWidth(line string) int { return lipgloss.Width(stripANSI(line)) }

func m2lines(s string) []string { return strings.Split(s, "\n") }

func (h *harness) secretData(path string) map[string]any {
	h.t.Helper()
	sec, err := h.server.Read(h.t.Context(), "kv", path, 0)
	if err != nil {
		h.t.Fatalf("read kv/%s: %v", path, err)
	}
	return sec.Data
}

func (h *harness) versions(path string) []vault.VersionInfo {
	h.t.Helper()
	versions, _, err := h.server.Versions(h.t.Context(), "kv", path)
	if err != nil {
		h.t.Fatalf("versions kv/%s: %v", path, err)
	}
	return versions
}

// secretDataIn reads from a named mount, for the v1 fixtures.
func (h *harness) secretDataIn(mount, path string) map[string]any {
	h.t.Helper()
	sec, err := h.server.Read(h.t.Context(), mount, path, 0)
	if err != nil {
		h.t.Fatalf("read %s/%s: %v", mount, path, err)
	}
	return sec.Data
}
