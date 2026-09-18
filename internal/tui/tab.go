package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/activity"
	"github.com/lucasassuncao/vivi/internal/tui/auth"
	"github.com/lucasassuncao/vivi/internal/tui/policies"
	"github.com/lucasassuncao/vivi/internal/tui/secrets"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// Tab is one tab: what it carries, what it draws, and what the keyboard can do
// in it. Every method here was a `switch m.tab` arm before it was a method, so
// the interface is not a new idea - it is the one the switches were already
// keeping, written down.
type Tab interface {
	// Load fetches what the tab needs on its first visit; Reload drops what it
	// has and asks again.
	Load() tea.Cmd
	Reload() tea.Cmd

	// Key is a keystroke the tab owns. Data is a reply to a request it made.
	Key(msg tea.KeyPressMsg, ctx ui.Context) tea.Cmd
	Data(msg tea.Msg) tea.Cmd

	// KeyDetail is the tab's chance at a keystroke while the detail pane holds
	// focus, reporting whether it took it. A pane with a list of its own walks
	// that list; one holding a document lets the shell scroll it.
	KeyDetail(msg tea.KeyPressMsg, ctx ui.Context) (tea.Cmd, bool)

	// Titles names both panes: what the list is showing, and what the detail is.
	Titles() (left, right string)

	RenderList(width, height int, ctx ui.Context) string
	RenderDetail(width int, ctx ui.Context) string

	// Legend answers the footer's two questions at once: what the list holds,
	// and what the keyboard can do here.
	Legend(ctx ui.Context) (status string, keys []ui.LegendEntry)
	Help(ctx ui.Context) ui.HelpSection
	CopyRows(ctx ui.Context) []ui.CopyRow

	// Overlay is what the tab draws over the panes - a confirmation, a form, a
	// diff - or "" for nothing. The shell centres it, because the panes are the
	// shell's and a tab cannot see them.
	Overlay(ctx ui.Context) string

	// Captures reports that a modal or an input in the tab is taking every
	// keystroke, so the shell's own shortcuts stand aside.
	Captures() bool

	// Blur is the tab leaving the front, its transient state with it.
	Blur()
}

// Compile-time proof that each component still answers the whole interface. A
// tab that drifts out of it should fail here and not at the call site.
var (
	_ Tab = (*secrets.Model)(nil)
	_ Tab = (*activity.Model)(nil)
	_ Tab = (*auth.Model)(nil)
	_ Tab = (*policies.Model)(nil)
)

// tabFor is the component behind a tab. This switch is the last one over the
// tab in the shell, and it is the dispatch table itself rather than a decision
// about what a tab can do.
func (m *Model) tabFor(t tab) Tab {
	switch t {
	case tabPolicies:
		return m.policiesTab
	case tabAuth:
		return m.authTab
	case tabActivity:
		return m.activityTab
	default:
		return m.secretsTab
	}
}

// currentTab is the component in front of the user.
func (m *Model) currentTab() Tab { return m.tabFor(m.tab) }
