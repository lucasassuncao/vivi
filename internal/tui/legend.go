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
// itself, so each is display only; "?" is pinned wherever keyBrowse gets it,
// and stays out of the palette, which has vivi's own :help.
func (a tabAdapter) Actions(shell.Context) []shell.Action {
	m := a.m
	_, keys := m.tabFor(a.t).Legend(m.uiContext())
	helps := shell.When(func(shell.Context) bool { return m.helpKey() != "" })
	out := []shell.Action{shell.Help(shell.DisplayOnly(), shell.Named(""), helps)}
	for _, e := range keys {
		h := e.Help()
		out = append(out, shell.Custom(h.Key, h.Desc, nil, shell.WithKey(e.Keys()...), shell.Needs(e.Needs), shell.DisplayOnly()))
	}
	return out
}

// legend is the footer's keys for where the user is, after the read-only
// filter: what the shell will pack. Kept for the tests that pin the rule.
func (m *Model) legend() []legend.Entry {
	_, hs := m.currentTab().Legend(m.uiContext())
	return legend.Filter(hs, m.can)
}

// legendMaxLines is what the footer will spend on the legend. Three since d and
// M both sit in the tree: two lines no longer held the secrets legend at 88.
const legendMaxLines = 3

// legendLines is the legend packed to a width the way the shell packs it,
// for the tests that pin how it folds.
func (m *Model) legendLines(width int) []string {
	st := m.st.Shell().Legend
	st.HelpKey = m.helpKey()
	return legend.Pack(m.legend(), width, legendMaxLines, st)
}

// renderLegend is legendLines joined, for the tests that read one line.
func (m *Model) renderLegend(width int) string { return strings.Join(m.legendLines(width), "\n") }

// can is the read-only rule: a write key needs a session that may write.
func (m *Model) can(c legend.Capability) bool { return c != ui.CapWrite || m.canWrite() }
