// Package policies is the Policies tab: the names, the document, and the jump
// reaching one by name. Read-only by design - writing policies is a non-goal -
// so no editor and no confirmation. The colouring lives in hcl.go, a lexer and
// not a view.
package policies

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// Model is the tab's state. Nothing here is anyone else's, and the reverse
// lookup the Secrets pane does over the same documents is its own: the two
// want different things from the same endpoint, so they ask separately.
type Model struct {
	client vault.Auditor
	req    ui.Requester

	names  []string
	cursor int
	hcl    string
	// reading is a document still in flight, which the pane draws as such: an
	// empty body and "(empty policy)" are different answers.
	reading bool
	// pending is a jump waiting for the list it needs. "p" on a role can fire
	// before this tab has ever been opened.
	pending string
}

func New(client vault.Auditor, req ui.Requester) *Model {
	return &Model{client: client, req: req}
}

// What this tab says about itself from outside. Read-only, and the slice is the
// tab's own: a caller that appends to it is writing another tab's state.
func (m *Model) Names() []string { return m.names }
func (m *Model) Cursor() int     { return m.cursor }
func (m *Model) Doc() string     { return m.hcl }

// Pending is the jump still waiting for the list it needs, or "".
func (m *Model) Pending() string { return m.pending }

// Restore puts the tab in a state it would otherwise have to be walked into.
// It exists for the tests that need a tab already holding a list and a document
// with no server behind it; nothing in the running app calls it.
func (m *Model) Restore(names []string, cursor int, hcl string) {
	m.names, m.cursor, m.hcl = names, cursor, hcl
}

// listMsg and docMsg are this tab's own answers. They satisfy ui.RequestAnswer
// so the parent's in-flight count settles wherever the call was made.
type listMsg struct {
	names []string
	err   error
}

type docMsg struct {
	name string
	hcl  string
	err  error
}

func (listMsg) AnswersRequest() {}
func (docMsg) AnswersRequest()  {}

// Load asks for the list on the first visit only. nil means never loaded; an
// empty answer is stored as an empty slice, or it would be asked for again on
// every visit.
func (m *Model) Load() tea.Cmd {
	if m.names == nil {
		return m.loadList()
	}
	return nil
}

// Reload drops the cursor with the list it points into. Keeping it would leave
// it addressing a position in a list that no longer exists.
func (m *Model) Reload() tea.Cmd {
	m.names, m.hcl, m.cursor = nil, "", 0
	return m.loadList()
}

func (m *Model) loadList() tea.Cmd {
	client := m.client
	return m.req.Request(func(ctx context.Context) tea.Msg {
		names, err := client.Policies(ctx)
		return listMsg{names: names, err: err}
	})
}

func (m *Model) loadDoc(name string) tea.Cmd {
	// The panel title follows the cursor at once, so the body cannot keep the
	// last document while this one is read: that pairs one policy's name with
	// another's rules, and nothing on screen says which is which.
	m.hcl, m.reading = "", true
	client := m.client
	return m.req.Request(func(ctx context.Context) tea.Msg {
		hcl, err := client.Policy(ctx, name)
		return docMsg{name: name, hcl: hcl, err: err}
	})
}

func (m *Model) Data(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case listMsg:
		return m.afterList(msg)

	case docMsg:
		// A late answer would put one policy's HCL under another's name, and a
		// reader cannot tell. Same guard the secret read uses.
		if len(m.names) == 0 || m.cursor >= len(m.names) || msg.name != m.names[m.cursor] {
			return nil
		}
		m.reading = false
		if msg.err != nil {
			m.hcl = ""
			return ui.Emit(ui.BannerMsg{Text: "read policy " + msg.name, Cause: msg.err})
		}
		m.hcl = msg.hcl
		return nil
	}
	return nil
}

func (m *Model) afterList(msg listMsg) tea.Cmd {
	if msg.err != nil {
		return ui.Emit(ui.BannerMsg{Text: "list policies", Cause: msg.err})
	}
	m.names = msg.names
	if m.names == nil {
		m.names = []string{}
	}
	if len(m.names) == 0 {
		m.pending = ""
		return nil
	}
	// A jump that arrived before the list did gets its target now.
	var missed tea.Cmd
	if want := m.pending; want != "" {
		m.pending = ""
		if !m.selectByName(want) {
			missed = ui.Emit(ui.BannerMsg{Text: "policy " + want + " is not in the list"})
		}
	}
	// The cursor outlives the list it points into: policies are provisioned
	// elsewhere, so a refresh can answer with fewer. "The list is not empty"
	// is a different question from "the cursor is still on it".
	m.cursor = min(max(m.cursor, 0), len(m.names)-1)
	return tea.Batch(missed, m.loadDoc(m.names[m.cursor]))
}

// Jump puts the cursor on a policy by name, fetching the list first if this tab
// has never been opened. The Auth tab reaches it through the parent.
func (m *Model) Jump(name string) tea.Cmd {
	if m.selectByName(name) {
		return m.loadDoc(name)
	}
	if m.names == nil {
		m.pending = name
		return m.loadList()
	}
	return ui.Emit(ui.BannerMsg{Text: "policy " + name + " is not in the list"})
}

// selectByName puts the cursor on a policy, reporting whether it is there.
func (m *Model) selectByName(name string) bool {
	for i, p := range m.names {
		if p == name {
			m.cursor = i
			return true
		}
	}
	return false
}

func (m *Model) Titles() (left, right string) {
	if m.cursor < len(m.names) {
		return "Policies", m.names[m.cursor]
	}
	return "Policies", "Policy"
}

func (m *Model) RenderList(width, height int, ctx ui.Context) string {
	st := ctx.Styles
	if len(m.names) == 0 {
		return st.EmptyState(width, height,
			"no policies this token can read",
			"listing policies needs read on sys/policy")
	}
	start := ui.ScrollStart(m.cursor, len(m.names), height)
	var b strings.Builder
	for i := start; i < len(m.names) && i < start+height; i++ {
		cursor := "  "
		if i == m.cursor {
			cursor = st.Cursor.Render("● ")
		}
		b.WriteString(ui.Truncate(cursor+ui.Sanitize(m.names[i]), width))
		b.WriteString("\n")
	}
	return b.String()
}

func (m *Model) RenderDetail(width int, ctx ui.Context) string {
	st := ctx.Styles
	switch {
	case m.cursor >= len(m.names):
		return st.Dim.Render("-")
	case m.reading:
		return st.Dim.Render("reading…")
	case m.hcl == "":
		return st.Dim.Render("(empty policy)")
	}

	// The name is the panel title, so the body is the document. Line breaks
	// survive and every other control character does not: a policy is where a
	// bidirectional override would pay off best, since it is read to be believed.
	var b strings.Builder
	safe := strings.Join(ui.SanitizeLines(m.hcl), "\n")
	for _, line := range strings.Split(highlight(st, safe), "\n") {
		b.WriteString(ui.Truncate(line, width))
		b.WriteString("\n")
	}
	return b.String()
}

func (m *Model) Key(msg tea.KeyPressMsg, _ ui.Context) tea.Cmd {
	switch msg.String() {
	case "down":
		return m.move(1)
	case "up":
		return m.move(-1)
	case "home":
		return m.move(-m.cursor)
	case "end":
		return m.move(len(m.names) - 1 - m.cursor)
	case "right", "enter":
		return ui.Emit(ui.FocusMsg{Focus: ui.FocusDetail})
	case "y":
		return m.CopyDoc()
	}
	return nil
}

// move steps the cursor and reads the policy it lands on. At either end nothing
// moves and nothing is asked for.
func (m *Model) move(delta int) tea.Cmd {
	next := m.cursor + delta
	if next < 0 || next >= len(m.names) {
		return nil
	}
	m.cursor = next
	return tea.Batch(ui.Emit(ui.ScrollTopMsg{}), m.loadDoc(m.names[next]))
}

// HasDoc reports whether there is a document open, which is what the ":copy
// policy" command is scoped on.
func (m *Model) HasDoc() bool { return m.hcl != "" }

// CopyDoc puts the open policy document on the clipboard. A policy is not a
// secret - it says who may read one - so nothing is masked and there is nothing
// to reveal first.
func (m *Model) CopyDoc() tea.Cmd {
	if m.hcl == "" {
		return nil
	}
	return ui.Emit(ui.CopyMsg{Op: "copy policy", What: "policy copied", Text: m.hcl})
}

func (m *Model) Legend(ctx ui.Context) (status string, keys []ui.LegendEntry) {
	status = fmt.Sprintf("%d policies", len(m.names))
	if ctx.Focus == ui.FocusDetail {
		return status, append(ui.ScrollLegend(), ui.Entry("y", "copy HCL"), ui.Entry("R", "reload"))
	}
	return status, ui.ListLegend("move",
		ui.Entry("→/enter", "read"), ui.Entry("y", "copy HCL"), ui.Entry("R", "reload"))
}

func (m *Model) Help(ctx ui.Context) ui.HelpSection {
	if ctx.Focus == ui.FocusDetail {
		return ui.HelpSection{Title: "Policy document", Rows: ui.DocumentKeys(
			[2]string{"y", "copy the HCL"},
			[2]string{"R", "reload the list"},
		)}
	}
	return ui.HelpSection{Title: "Policies (read-only)", Rows: [][2]string{
		{"↑ / ↓", "move"},
		{"→ / enter", "read the policy"},
		{"y", "copy the HCL"},
		{"R", "reload the list"},
	}}
}

// Read-only by design: nothing floats, nothing takes the keyboard, and there is
// no transient state to drop on the way out.
func (m *Model) Overlay(ui.Context) string { return "" }

// KeyDetail takes nothing: the pane holds a document, so the arrows scroll it.
func (m *Model) KeyDetail(tea.KeyPressMsg, ui.Context) (tea.Cmd, bool) { return nil, false }

func (m *Model) Captures() bool { return false }
func (m *Model) Blur()          {}

func (m *Model) CopyRows(ui.Context) []ui.CopyRow {
	if m.cursor < 0 || m.cursor >= len(m.names) {
		return nil
	}
	name := m.names[m.cursor]
	return []ui.CopyRow{
		{Label: "name", Text: name},
		{Label: "read", Text: "vault policy read " + name, IsCommand: true},
	}
}
