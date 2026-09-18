// Package auth is the Auth tab: methods, their roles, and the link from a role
// to the policy it grants. The left pane holds two cursors and not two panes -
// roles are drawn indented - so onRoles decides what the arrows mean here, not
// focus.
package auth

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

type Model struct {
	client vault.Auditor
	req    ui.Requester

	methods      []vault.AuthMethod
	methodCursor int
	roles        map[string][]vault.Role
	roleCursor   int
	onRoles      bool
}

func New(client vault.Auditor, req ui.Requester) *Model {
	return &Model{client: client, req: req, roles: map[string][]vault.Role{}}
}

// What this tab says about itself from outside.
func (m *Model) Methods() []vault.AuthMethod { return m.methods }
func (m *Model) MethodCursor() int           { return m.methodCursor }
func (m *Model) RoleCursor() int             { return m.roleCursor }
func (m *Model) OnRoles() bool               { return m.onRoles }

// CurrentRoles are the roles of the method under the cursor.
func (m *Model) CurrentRoles() []vault.Role {
	if m.methodCursor >= len(m.methods) {
		return nil
	}
	return m.roles[m.methods[m.methodCursor].Path]
}

type methodsMsg struct {
	methods []vault.AuthMethod
	err     error
}

type rolesMsg struct {
	method vault.AuthMethod
	roles  []vault.Role
	err    error
}

func (methodsMsg) AnswersRequest() {}
func (rolesMsg) AnswersRequest()   {}

func (m *Model) Load() tea.Cmd {
	if m.methods == nil {
		return m.loadMethods()
	}
	return nil
}

// Reload drops the methods and their roles together: a role list keyed on a
// method that is no longer there describes nothing.
func (m *Model) Reload() tea.Cmd {
	m.methods, m.methodCursor, m.roleCursor, m.onRoles = nil, 0, 0, false
	m.roles = map[string][]vault.Role{}
	return m.loadMethods()
}

func (m *Model) loadMethods() tea.Cmd {
	client := m.client
	return m.req.Request(func(ctx context.Context) tea.Msg {
		methods, err := client.AuthMethods(ctx)
		return methodsMsg{methods: methods, err: err}
	})
}

func (m *Model) loadRoles(method vault.AuthMethod) tea.Cmd {
	client := m.client
	return m.req.Request(func(ctx context.Context) tea.Msg {
		roles, err := client.AuthRoles(ctx, method)
		return rolesMsg{method: method, roles: roles, err: err}
	})
}

func (m *Model) Data(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case methodsMsg:
		if msg.err != nil {
			return ui.Emit(ui.BannerMsg{Text: "list auth methods", Cause: msg.err})
		}
		m.methods = msg.methods
		if len(m.methods) == 0 {
			return nil
		}
		// The list can come back shorter than the cursor that was pointing into
		// the old one.
		m.methodCursor = min(max(m.methodCursor, 0), len(m.methods)-1)
		return m.loadRoles(m.methods[m.methodCursor])

	case rolesMsg:
		// No staleness guard needed: the answer keys on the method path, so a
		// late one lands under the right method by construction.
		if msg.err != nil {
			return ui.Emit(ui.BannerMsg{Text: "list roles of " + msg.method.Path, Cause: msg.err})
		}
		m.roles[msg.method.Path] = msg.roles
		return nil
	}
	return nil
}

func (m *Model) Titles() (left, right string) {
	roles := m.CurrentRoles()
	switch {
	case m.onRoles && m.roleCursor < len(roles):
		return "Auth methods", m.methods[m.methodCursor].Path + roles[m.roleCursor].Name
	case m.methodCursor < len(m.methods):
		return "Auth methods", m.methods[m.methodCursor].Path
	}
	return "Auth methods", "Auth"
}

func (m *Model) RenderList(width, _ int, ctx ui.Context) string {
	st := ctx.Styles
	if len(m.methods) == 0 {
		return st.Dim.Render("no auth methods this token can read")
	}
	var b strings.Builder
	for i, meth := range m.methods {
		cursor := "  "
		if i == m.methodCursor && !m.onRoles {
			cursor = st.Cursor.Render("● ")
		}
		b.WriteString(ui.Truncate(cursor+st.Mount.Render(ui.Sanitize(meth.Path))+st.Dim.Render("  "+ui.Sanitize(meth.Type)), width))
		b.WriteString("\n")

		if i != m.methodCursor {
			continue
		}
		for j, role := range m.roles[meth.Path] {
			rc := "    "
			if m.onRoles && j == m.roleCursor {
				rc = st.Cursor.Render("  › ")
			}
			line := rc + ui.Sanitize(role.Name)
			if role.ReadErr != nil {
				line += st.Warn.Render("  unreadable")
			}
			b.WriteString(ui.Truncate(line, width))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m *Model) RenderDetail(width int, ctx ui.Context) string {
	st := ctx.Styles
	if m.methodCursor >= len(m.methods) {
		return st.Dim.Render("-")
	}
	meth := m.methods[m.methodCursor]
	roles := m.CurrentRoles()

	var b strings.Builder
	if !m.onRoles || m.roleCursor >= len(roles) {
		b.WriteString(st.KV("type", meth.Type, width))
		b.WriteString(st.KV("accessor", meth.Accessor, width))
		if meth.Description != "" {
			b.WriteString(st.KV("description", meth.Description, width))
		}
		b.WriteString(st.KV("roles", fmt.Sprint(len(roles)), width))
		return b.String()
	}

	role := roles[m.roleCursor]
	b.WriteString(st.Heading("granted policies", width))
	b.WriteString("\n")
	switch {
	case role.ReadErr != nil:
		b.WriteString(ui.Truncate(st.Warn.Render("  (could not be read: "+ui.Humanize(role.ReadErr)+")"), width))
		b.WriteString("\n")
	case len(role.Policies) == 0:
		b.WriteString(st.Dim.Render("  (none)\n"))
	}
	for _, p := range role.Policies {
		b.WriteString(ui.Truncate("  "+ui.Sanitize(p), width))
		b.WriteString("\n")
	}
	b.WriteString(st.Dim.Render("\n  p jumps to that policy on the Policies tab\n"))

	if len(role.Data) > 0 {
		b.WriteString("\n")
		b.WriteString(st.Heading("configuration", width))
		b.WriteString("\n")
		for _, k := range ui.SortedMapKeys(role.Data) {
			b.WriteString(st.KV(k, ui.ValueToString(role.Data[k]), width))
		}
	}
	return b.String()
}

func (m *Model) Key(msg tea.KeyPressMsg, _ ui.Context) tea.Cmd {
	// focus rides to the end because every arm here finishes the same way: the
	// cursors are clamped and the pane goes back to its first line.
	var focus bool

	switch msg.String() {
	case "down":
		if m.onRoles {
			m.roleCursor++
		} else if cmd := m.moveMethod(1); cmd != nil {
			return cmd
		}
	case "up":
		if m.onRoles {
			m.roleCursor = max(0, m.roleCursor-1)
		} else if cmd := m.moveMethod(-1); cmd != nil {
			return cmd
		}
	case "home", "end":
		// Both columns move the same way the arrows do, on whichever one the
		// keyboard is in.
		last := 0
		if msg.String() == "end" {
			if m.onRoles {
				last = len(m.CurrentRoles()) - 1
			} else {
				last = len(m.methods) - 1
			}
		}
		if m.onRoles {
			m.roleCursor = max(0, last)
		} else if cmd := m.moveMethod(max(0, last) - m.methodCursor); cmd != nil {
			return cmd
		}
	case "right", "enter":
		// Right walks inward: methods to roles, then into the pane describing
		// the selected role.
		if m.onRoles {
			focus = true
		} else {
			m.onRoles = true
		}
	case "left":
		m.onRoles = false
	case "p":
		if cmd := m.JumpToSelectedRolePolicy(); cmd != nil {
			return cmd
		}
	}
	m.clampCursors()
	if focus {
		return ui.Emit(ui.FocusMsg{Focus: ui.FocusDetail}, ui.ScrollTopMsg{})
	}
	return ui.Emit(ui.ScrollTopMsg{})
}

// moveMethod steps the method cursor and lists the roles of the one it lands
// on, starting at the first. At either end nothing moves and it answers nil.
func (m *Model) moveMethod(delta int) tea.Cmd {
	next := m.methodCursor + delta
	if next < 0 || next >= len(m.methods) {
		return nil
	}
	m.methodCursor, m.roleCursor = next, 0
	return m.loadRoles(m.methods[next])
}

// JumpToSelectedRolePolicy follows the link this tab shows: auditing means
// asking "and what does that policy actually allow" constantly, and retyping
// the name is the friction that stops people from asking. The parent routes it,
// because a tab does not name another tab.
func (m *Model) JumpToSelectedRolePolicy() tea.Cmd {
	name := m.selectedRolePolicy()
	if name == "" {
		return nil
	}
	return ui.Emit(ui.JumpToPolicyMsg{Name: name})
}

// HasRolePolicy reports whether "p" has anywhere to go, which is what the
// command palette scopes the entry on.
func (m *Model) HasRolePolicy() bool { return m.selectedRolePolicy() != "" }

func (m *Model) selectedRolePolicy() string {
	roles := m.CurrentRoles()
	if m.roleCursor >= len(roles) {
		return ""
	}
	pols := roles[m.roleCursor].Policies
	if len(pols) == 0 {
		return ""
	}
	return strings.TrimSpace(pols[0])
}

func (m *Model) clampCursors() {
	m.methodCursor = min(max(m.methodCursor, 0), max(0, len(m.methods)-1))
	m.roleCursor = min(max(m.roleCursor, 0), max(0, len(m.CurrentRoles())-1))
}

func (m *Model) Legend(ctx ui.Context) (status string, keys []ui.LegendEntry) {
	status = fmt.Sprintf("%d auth methods  ·  %d roles", len(m.methods), len(m.CurrentRoles()))
	if ctx.Focus == ui.FocusDetail {
		return status, append(ui.ScrollLegend(), ui.Entry("p", "go to policy"), ui.Entry("R", "reload"))
	}
	return status, ui.ListLegend("move",
		ui.Entry("→/enter", "roles, then detail"), ui.Entry("p", "go to policy"), ui.Entry("R", "reload"))
}

func (m *Model) Help(ctx ui.Context) ui.HelpSection {
	if ctx.Focus == ui.FocusDetail {
		return ui.HelpSection{Title: "Role detail", Rows: ui.DocumentKeys(
			[2]string{"p", "go to the policy this role grants"},
			[2]string{"R", "reload the methods"},
		)}
	}
	return ui.HelpSection{Title: "Auth (read-only)", Rows: [][2]string{
		{"↑ / ↓", "move"},
		{"→ / enter", "roles, then the role detail"},
		{"←", "back up"},
		{"p", "go to the policy this role grants"},
		{"R", "reload the methods"},
	}}
}

// Read-only by design: nothing floats, nothing takes the keyboard, and there is
// no transient state to drop on the way out.
func (m *Model) Overlay(ui.Context) string { return "" }

// KeyDetail takes nothing: the role's pane holds a description, so the arrows
// scroll it.
func (m *Model) KeyDetail(tea.KeyPressMsg, ui.Context) (tea.Cmd, bool) { return nil, false }

func (m *Model) Captures() bool { return false }
func (m *Model) Blur()          {}

func (m *Model) CopyRows(ui.Context) []ui.CopyRow {
	if m.methodCursor < 0 || m.methodCursor >= len(m.methods) {
		return nil
	}
	meth := m.methods[m.methodCursor]

	roles := m.CurrentRoles()
	if m.onRoles && m.roleCursor >= 0 && m.roleCursor < len(roles) {
		role := roles[m.roleCursor]
		rows := []ui.CopyRow{{Label: "path", Text: "auth/" + meth.Path + role.Name}}
		// A backend vivi cannot enumerate has no read endpoint to name either,
		// and guessing one would print a command that fails.
		if p := vault.RoleReadPath(meth, role.Name); p != "" {
			rows = append(rows, ui.CopyRow{Label: "read", Text: "vault read " + p, IsCommand: true})
		}
		return rows
	}

	return []ui.CopyRow{
		{Label: "path", Text: "auth/" + meth.Path},
		{Label: "list", Text: "vault auth list", IsCommand: true},
	}
}
