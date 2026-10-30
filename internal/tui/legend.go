package tui

import (
	"strings"

	"github.com/lucasassuncao/bezel/legend"
	"github.com/lucasassuncao/bezel/shell"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The footer under the panes is the shell's: it asks the tab in front for its
// status and keys, packs the keys to the width and drops from the end. What
// stays here is the translation - a tab speaks legend.Entry - and the
// read-only rule: what cannot run is dropped rather than explained, since the
// header badge is the standing explanation and "?" keeps the full list.

// tabAdapter is one tab as the shell sees it: a name for the strip and a
// legend for the footer.
type tabAdapter struct {
	m *Model
	t tab
}

func (a tabAdapter) Name() string { return tabNames[a.t] }

// Status is the tab's own status. How much of the detail is below the pane
// is the pane's to say, on its last row.
func (a tabAdapter) Status(shell.Context) string {
	status, _ := a.m.tabFor(a.t).Legend(a.m.uiContext())
	return status
}

// Actions is the tab's keys as the shell prints them. vivi routes every key
// itself, so each is display only. "?" closes the first row wherever keyBrowse
// gets it, and stays out of the palette, which has vivi's own :help.
func (a tabAdapter) Actions(shell.Context) []shell.Action {
	m := a.m
	keys := m.screenKeys(a.t)
	helps := shell.When(func(shell.Context) bool { return m.helpKey() != "" })
	var out []shell.Action
	for _, e := range keys {
		out = append(out, shell.Custom(e.Help.Key, e.Help.Desc, nil, shell.Needs(e.Needs), shell.Group(e.Group), shell.DisplayOnly()))
	}
	// Group 0 like the moves, and after them: the legend sorts by group, stably.
	return append(out, shell.Help(shell.DisplayOnly(), shell.Named(""), helps))
}

// legend is the footer's keys for where the user is, after the read-only
// filter: what the shell will pack. Kept for the tests that pin the rule.
func (m *Model) legend() []legend.Entry {
	return legend.Filter(m.screenKeys(m.tab), m.can)
}

// screenKeys is a tab's keys and, closing the first row before help, the
// Command Reference's: it acts on the screen, not on what the cursor is on.
func (m *Model) screenKeys(t tab) []legend.Entry {
	_, keys := m.tabFor(t).Legend(m.uiContext())
	if m.tabFor(t).ScreenCaptures() || !m.referenceFits() {
		return keys
	}
	desc := "hide reference"
	if m.refHidden {
		desc = "show reference"
	}
	return append(keys, legend.New("c", desc))
}

// legendMaxLines is what the footer will spend on the legend. Three since d and
// M both sit in the tree: two lines no longer held the secrets legend at 88.
const legendMaxLines = 3

// legendLines is the legend packed to a width the way the shell packs it,
// for the tests that pin how it folds.
func (m *Model) legendLines(width int) []string {
	helpKey := m.helpKey()
	keys := m.legend()
	if helpKey != "" {
		keys = append(keys, legend.New(helpKey, "help")) // where the shell puts it
	}
	return legend.Pack(keys, width, legendMaxLines, helpKey, m.st.Shell().Legend)
}

// renderLegend is legendLines joined, for the tests that read one line.
func (m *Model) renderLegend(width int) string { return strings.Join(m.legendLines(width), "\n") }

// can is the read-only rule: a write key needs a session that may write.
func (m *Model) can(c legend.Capability) bool { return c != ui.CapWrite || m.canWrite() }
