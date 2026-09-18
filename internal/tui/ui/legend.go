package ui

import "strings"

// The shapes a legend and a help section are built from. Here rather than in
// the parent because every tab answers for its own keys, and a tab cannot name
// a helper that lives in the package importing it.

func Entry(key, action string) LegendEntry { return LegendEntry{Key: key, Action: action} }

// WriteEntry marks a key a read-only session refuses.
func WriteEntry(key, action string) LegendEntry {
	return LegendEntry{Key: key, Action: action, Writes: true}
}

// ListLegend opens every list's legend the same way, with the keys that mean
// one thing anywhere ahead of what the arrows do here.
func ListLegend(arrows string, rest ...LegendEntry) []LegendEntry {
	return append([]LegendEntry{
		Entry("?", "help"), Entry("tab", "change tab"), Entry("↑/↓", arrows), Entry(":", "commands"),
	}, rest...)
}

// ScrollLegend is the legend for a focused pane holding a document rather than
// a list of fields. Secrets has its own: there the arrows walk fields. Each
// caller appends what its own tab can do.
func ScrollLegend() []LegendEntry {
	return []LegendEntry{
		Entry("?", "help"), Entry("tab", "change tab"), Entry("↑/↓", "scroll"), Entry(":", "commands"),
		Entry("esc/←", "back to list"), Entry("pgup/pgdn", "half a page"),
	}
}

// HintLine draws entries as one line of "[key] action". The footer packs the
// same pairs across two rows because it has the whole width to fill; a modal
// has one line and its own border, so it joins them and stops.
func (s Styles) HintLine(entries []LegendEntry) string {
	pairs := make([]string, 0, len(entries))
	for _, e := range entries {
		pairs = append(pairs, s.LegendKey.Render("["+e.Key+"]")+" "+s.LegendText.Render(e.Action))
	}
	return strings.Join(pairs, s.LegendText.Render(" · "))
}

// DocumentKeys is what the "?" panel says about a focused document pane.
func DocumentKeys(extra ...[2]string) [][2]string {
	return append([][2]string{
		{"↑ / ↓", "scroll"},
		{"pgup / pgdn", "half a page"},
		{"esc / ←", "back to the list"},
	}, extra...)
}
