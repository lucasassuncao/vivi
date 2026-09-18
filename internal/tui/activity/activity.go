// Package activity is the Activity tab: what this session wrote, in order. The
// one tab with no server behind it, which is why R says so rather than
// refetching and why the entries are built here rather than loaded.
package activity

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// Entry is one write vivi performed this session. The log exists for a sequence
// that stops halfway - four destroys failing on the third - and records what
// happened, never what was written: no secret value reaches this struct.
type Entry struct {
	At     time.Time
	Op     string
	Path   string
	Detail string
	Err    error
}

// OK reports whether the operation succeeded, which is the only thing the log
// says about it beyond what it was and when.
func (e Entry) OK() bool { return e.Err == nil }

func (e Entry) result() string {
	if e.Err == nil {
		return "ok"
	}
	return "failed"
}

// Model is the tab's whole state. Two fields, and neither is anyone else's.
type Model struct {
	entries []Entry
	cursor  int
}

func New() *Model { return &Model{} }

// Entries and Cursor are what this tab will say about itself from outside: how
// much it has recorded and which row is selected. Read-only, and the slice is
// the tab's own - a caller that appends to it is writing another tab's state.
func (m *Model) Entries() []Entry { return m.entries }
func (m *Model) Cursor() int      { return m.cursor }

// Record appends an entry, newest last.
func (m *Model) Record(op, path, detail string, err error) {
	m.entries = append(m.entries, Entry{
		At:     time.Now(),
		Op:     op,
		Path:   path,
		Detail: detail,
		Err:    err,
	})
	if m.cursor == len(m.entries)-2 || len(m.entries) == 1 {
		m.cursor = len(m.entries) - 1
	}
}

// Load answers nothing: there is no server behind this tab. Reload says so out
// loud, which is the whole reason R does not silently do nothing here.
func (m *Model) Load() tea.Cmd { return nil }

func (m *Model) Reload() tea.Cmd {
	// The log is this session's own record of what it wrote. There is nothing
	// on the server to fetch it from.
	return ui.Emit(ui.BannerMsg{Text: "the activity log is local to this session"})
}

// Data answers nothing for the same reason: no request means no reply.
func (m *Model) Data(tea.Msg) tea.Cmd { return nil }

func (m *Model) Titles() (left, right string) { return "Activity", "Operation" }

func (m *Model) RenderList(width, height int, ctx ui.Context) string {
	st := ctx.Styles
	if len(m.entries) == 0 {
		return st.EmptyState(width, height,
			"no writes in this session",
			"edits, deletes and rollbacks are recorded here as you make them")
	}
	start := ui.ScrollStart(m.cursor, len(m.entries), height)
	var b strings.Builder
	for i := start; i < len(m.entries) && i < start+height; i++ {
		e := m.entries[i]
		cursor := "  "
		if i == m.cursor {
			cursor = st.Cursor.Render("● ")
		}
		status := st.OKBanner.Render("✓ ok")
		if !e.OK() {
			status = st.ErrBanner.Render("✗ failed")
		}
		line := fmt.Sprintf("%s%s %-16s %s", cursor, e.At.Format("15:04:05"), e.Op, status)
		b.WriteString(ui.Truncate(line, width))
		b.WriteString("\n")
	}
	return b.String()
}

func (m *Model) RenderDetail(width int, ctx ui.Context) string {
	st := ctx.Styles
	if m.cursor >= len(m.entries) {
		return st.Dim.Render("-")
	}
	e := m.entries[m.cursor]
	var b strings.Builder
	b.WriteString(st.KV("path", e.Path, width))
	b.WriteString(st.KV("time", e.At.Format("15:04:05"), width))
	b.WriteString(st.KV("result", e.result(), width))
	if e.Detail != "" {
		b.WriteString(st.KV("detail", e.Detail, width))
	}
	if e.Err != nil {
		b.WriteString("\n")
		b.WriteString(st.ErrBanner.Render(ui.Truncate(ui.Sanitize(e.Err.Error()), width)))
	}
	return b.String()
}

func (m *Model) Key(msg tea.KeyPressMsg, _ ui.Context) tea.Cmd {
	switch msg.String() {
	case "down":
		return m.moveTo(m.cursor + 1)
	case "up":
		return m.moveTo(m.cursor - 1)
	case "home":
		return m.moveTo(0)
	case "end":
		return m.moveTo(len(m.entries) - 1)
	case "right", "enter":
		return ui.Emit(ui.FocusMsg{Focus: ui.FocusDetail})
	}
	return nil
}

// moveTo clamps the cursor and asks for the pane it just changed to go back to
// its first line. The pane is the parent's, so this is a request and not a call.
func (m *Model) moveTo(i int) tea.Cmd {
	m.cursor = min(max(i, 0), max(0, len(m.entries)-1))
	return ui.Emit(ui.ScrollTopMsg{})
}

func (m *Model) Legend(ctx ui.Context) (status string, keys []ui.LegendEntry) {
	status = fmt.Sprintf("%d writes this session", len(m.entries))
	if ctx.Focus == ui.FocusDetail {
		return status, ui.ScrollLegend()
	}
	return status, ui.ListLegend("move", ui.Entry("→/enter", "detail"))
}

func (m *Model) Help(ctx ui.Context) ui.HelpSection {
	if ctx.Focus == ui.FocusDetail {
		return ui.HelpSection{Title: "Operation detail", Rows: ui.DocumentKeys()}
	}
	return ui.HelpSection{Title: "Activity", Rows: [][2]string{
		{"↑ / ↓", "move"},
		{"→ / enter", "detail"},
	}}
}

// This tab has nothing that floats and nothing that takes the keyboard, so it
// draws no overlay, never captures, and has no transient state to drop.
func (m *Model) Overlay(ui.Context) string { return "" }

// KeyDetail takes nothing: the pane holds a description, not a list, so the
// arrows scroll it.
func (m *Model) KeyDetail(tea.KeyPressMsg, ui.Context) (tea.Cmd, bool) { return nil, false }

func (m *Model) Captures() bool { return false }
func (m *Model) Blur()          {}

// CopyRows is empty on purpose. The activity log is this session's own record.
// There is no path on a server behind it, so there is nothing here to address.
func (m *Model) CopyRows(ui.Context) []ui.CopyRow { return nil }
