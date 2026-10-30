package ui

import "github.com/lucasassuncao/bezel/legend"

// The shapes a legend and a help section are built from. Here rather than in
// the parent because every tab answers for its own keys, and a tab cannot name
// a helper that lives in the package importing it.

// CapWrite marks a key a read-only session refuses; the shell drops it.
const CapWrite legend.Capability = "write"

// WriteKey is a legend entry a read-only session drops: keys and desc that
// need CapWrite.
func WriteKey(keys, desc string) legend.Entry {
	e := legend.New(keys, desc)
	e.Needs = CapWrite
	return e
}

// ListLegend opens every list's legend the same way: the tabs, the arrows and
// moves, the other ways of moving here, then the command line, then the rest.
func ListLegend(arrows string, moves []legend.Entry, rest ...legend.Entry) []legend.Entry {
	out := append([]legend.Entry{legend.New("tab", "change tab"), legend.New("↑/↓", arrows)}, moves...)
	return append(append(out, legend.New(":", "commands")), rest...)
}

// Actions puts entries on the legend's second row: the first is how to get
// around, the second what can be done where the cursor is.
func Actions(entries ...legend.Entry) []legend.Entry {
	out := make([]legend.Entry, len(entries))
	for i, e := range entries {
		e.Group = 1
		out[i] = e
	}
	return out
}

// ScrollLegend is the legend for a focused pane holding a document rather than
// a list of fields. Secrets has its own: there the arrows walk fields. Each
// caller appends what its own tab can do.
func ScrollLegend() []legend.Entry {
	return ListLegend("scroll",
		[]legend.Entry{legend.New("esc/←", "back to list"), legend.New("pgup/pgdn", "half a page")})
}

// HintLine draws entries as one line of "[key] action". The footer packs the
// same pairs across two rows because it has the whole width to fill; a modal
// has one line and its own border, so it joins them and stops.
func (s Styles) HintLine(entries []legend.Entry) string {
	return legend.HintLine(entries, s.Hint())
}

// Hint is the style a hint row is drawn in, for a bezel widget drawing its own.
func (s Styles) Hint() legend.Style { return legend.Style{Key: s.LegendKey, Text: s.LegendText} }

// DocumentKeys is what the "?" panel says about a focused document pane.
func DocumentKeys(extra ...[2]string) [][2]string {
	return append([][2]string{
		{"↑ / ↓", "scroll"},
		{"pgup / pgdn", "half a page"},
		{"esc / ←", "back to the list"},
	}, extra...)
}
