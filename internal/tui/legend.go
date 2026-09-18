package tui

import (
	"fmt"
	"strings"

	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The footer under the panes: what the left pane is listing, and what the
// keyboard can do here. A column saved is an action that survives a narrow
// terminal, so the legend is priority-ordered and drops entries whole.

func (m *Model) renderFooter() string {
	// The count describes what the left pane is listing. The tree has none: it
	// is lazy, so a node count reports what was expanded rather than what is
	// there. The filter still reports itself - the user put it there.
	status, _ := m.currentTab().Legend(m.uiContext())

	// When the detail pane holds more than it can show, say where in it we are:
	// otherwise a truncated policy looks like the whole policy.
	if m.detail.Height() > 0 && m.detail.TotalLineCount() > m.detail.Height() {
		status += fmt.Sprintf("  ·  detail %d%%", int(m.detail.ScrollPercent()*100))
	}

	if m.banner != "" {
		if m.bannerErr {
			status = m.st.ErrBanner.Render(m.banner)
		} else {
			status = m.st.OKBanner.Render(m.banner)
		}
	}

	// The command line outranks the banner, the way the filter input outranks
	// the filter above: a line being typed cannot be covered by a message about
	// something that already happened, or the caret vanishes mid-word.
	if c, ok := m.mode.(commanding); ok && c.line != nil {
		status = c.line.input.View()
	}

	return ui.Truncate(status, m.width) + "\n" + m.renderLegend(m.width)
}

// ui.LegendEntry is one key and what it does. A key may be several, when they do
// one thing (→ and enter both step inward) or read positionally as a family:
// "[y/Y/C] copy value/path/command" pairs off in order.

// renderLegend draws [key] action pairs; the bracketed key is what makes the
// line scannable. Entries are dropped whole - a cut pair leaves a dangling
// bracket - and never the first, which legend() makes "?" for every context.
func (m *Model) renderLegend(width int) string {
	return strings.Join(m.legendLines(width), "\n")
}

// legendLines is the legend laid out, one string per footer row. Two lines
// because the widest legend wants 143 columns and nobody has them; below about
// seventy the mark at the end says how many keys are missing.
func (m *Model) legendLines(width int) []string {
	hs := m.legend()
	if len(hs) == 0 {
		return nil
	}

	pairs := make([]string, 0, len(hs))
	for _, h := range hs {
		pairs = append(pairs, m.st.LegendKey.Render("["+h.Key+"]")+" "+m.st.LegendText.Render(h.Action))
	}

	mark := func(dropped int) string {
		return m.st.LegendText.Render(fmt.Sprintf("  +%d in [?]", dropped))
	}
	return ui.PackToWidth(pairs, m.st.LegendText.Render("  "), mark, width, legendMaxLines)
}

// legendMaxLines is what the footer will spend on the legend. Three would fit
// every legend at every width this app agrees to draw, and would take a third
// row from the panes on a short terminal to print keys nobody asked to see.
const legendMaxLines = 2

// legend is the bar itself, in priority order: "?" first and never dropped,
// then movement, then common actions, then the destructive and rare ones. Those
// last are droppable precisely because "?" reaches them in one keystroke.
func (m *Model) legend() []ui.LegendEntry {
	hs := m.contextLegend()
	if m.canWrite() {
		return hs
	}
	// The legend is priority-ordered and space-constrained, so what cannot run
	// is dropped rather than explained - the header badge is the standing
	// explanation, and "?" keeps the full list.
	kept := make([]ui.LegendEntry, 0, len(hs))
	for _, h := range hs {
		if !h.Writes {
			kept = append(kept, h)
		}
	}
	return kept
}

// contextLegend is the legend for where the user is, before read-only has had its
// say. A modal answers for itself; otherwise the tab and the focus decide.
func (m *Model) contextLegend() []ui.LegendEntry {
	if hs := m.modeLegend(); hs != nil {
		return hs
	}
	return m.tabLegend()
}

// modeLegend is the legend of the shell's own modal, or nil when what is in
// front of the user belongs to a tab.
func (m *Model) modeLegend() []ui.LegendEntry {
	if modeIs[commanding](m) {
		return []ui.LegendEntry{
			ui.Entry("enter", "run"), ui.Entry("tab", "complete"),
			ui.Entry("↑/↓", "choose"), ui.Entry("esc", "cancel"),
		}
	}
	return nil
}

// tabLegend is the legend of a tab. Focus splits every tab in two: the same
// arrows do different things on either side. One branch ahead of the tabs used
// to hand a policy document the secrets legend - five keys, none of which does
// anything on a policy.
func (m *Model) tabLegend() []ui.LegendEntry {
	_, keys := m.currentTab().Legend(m.uiContext())
	return keys
}
