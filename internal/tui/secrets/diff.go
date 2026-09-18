package secrets

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// diffResult is a comparison between two versions of a secret. Values start
// masked and r reveals every row at once; the rows carry both sides in
// full, so what is withheld is a rendering decision and not a gap in the fetch.
type diffResult struct {
	pathLabel string
	a, b      int
	rows      []diffRow
}

type diffRow struct {
	key string
	// kind is "same", "changed", "added" (only in b) or "removed" (only in a).
	kind   string
	aValue string
	bValue string
}

type diffMsg struct {
	result *diffResult
	err    error
}

// startDiff compares the two marked versions.
func (m *Model) startDiff() tea.Cmd {
	if m.sel == nil {
		return nil
	}
	marked := m.targetVersions()
	if len(marked) != 2 {
		m.notify("mark exactly two versions with space to compare them")
		return nil
	}

	n := m.sel
	a, b := marked[0], marked[1]

	return m.onNode(n, func(ctx context.Context, c vault.Client, r nodeRef) ui.RequestAnswer {
		secA, err := c.Read(ctx, r.mount, r.path, a)
		if err != nil {
			return diffMsg{err: err}
		}
		secB, err := c.Read(ctx, r.mount, r.path, b)
		if err != nil {
			return diffMsg{err: err}
		}
		// "No error" and "a secret to read" are different promises, and Client
		// only makes the first. On a command goroutine a nil dereference is not
		// a failed diff but the end of the session, so this becomes an error.
		if secA == nil || secB == nil {
			missing := a
			if secA != nil {
				missing = b
			}
			return diffMsg{err: missingVersionErr(r.fullPath, missing)}
		}

		return diffMsg{result: buildDiff(r.fullPath, a, b, secA.Data, secB.Data)}
	})
}

// missingVersionErr names the side of a comparison that came back empty, since
// "compare versions: not found" on its own leaves the user guessing which of the
// two they marked has gone.
func missingVersionErr(path string, missing int) error {
	return &vault.Error{
		Op:     "read version",
		Path:   path,
		Kind:   vault.ErrNotFound,
		Detail: fmt.Sprintf("v%d has no data to compare", missing),
	}
}

func buildDiff(path string, a, b int, dataA, dataB map[string]any) *diffResult {
	keys := map[string]bool{}
	for k := range dataA {
		keys[k] = true
	}
	for k := range dataB {
		keys[k] = true
	}

	names := make([]string, 0, len(keys))
	for k := range keys {
		names = append(names, k)
	}
	slices.Sort(names)

	res := &diffResult{pathLabel: path, a: a, b: b}
	for _, k := range names {
		va, okA := dataA[k]
		vb, okB := dataB[k]
		row := diffRow{key: k, aValue: valueToString(va), bValue: valueToString(vb)}
		switch {
		case okA && !okB:
			row.kind = "removed"
		case !okA && okB:
			row.kind = "added"
		case row.aValue != row.bValue:
			row.kind = "changed"
		default:
			row.kind = "same"
		}
		res.rows = append(res.rows, row)
	}
	return res
}

// rollback writes an old version's content as a new version. It is implemented
// as a read plus a write with check-and-set rather than the SDK's Rollback so
// it goes through the same conflict handling as every other write.
func (m *Model) rollback(n *node, toVersion, base int) tea.Cmd {
	return m.mutate(n, func(ctx context.Context, c vault.Client, r nodeRef) ui.RequestAnswer {
		old, err := c.Read(ctx, r.mount, r.path, toVersion)
		if err != nil {
			return writeMsg{node: r.node, err: err}
		}
		// Nothing to write back. Reading through the empty answer here would
		// crash a command goroutine; worse, writing old.Data from a nil secret
		// would put an empty secret over the live one and call it a rollback.
		if old == nil {
			return writeMsg{node: r.node, err: &vault.Error{
				Op:     "rollback",
				Path:   r.fullPath,
				Kind:   vault.ErrNotFound,
				Detail: fmt.Sprintf("v%d has no data to restore", toVersion),
			}}
		}
		sec, err := c.Write(ctx, r.mount, r.path, old.Data, base)
		return writeMsg{node: r.node, secret: sec, cas: base, err: err}
	})
}

// Rendering.

// renderDiff draws the comparison, one line per key, masked until r. The reveal
// is per row, since one toggle would raise every secret at once; the cursor is
// drawn outside the row's style, as a dim cursor on a dim row is not a cursor.
func (m *Model) renderDiff(d *diffResult, bodyHeight int) string {
	var b strings.Builder
	b.WriteString(m.st.ModalTitle.Render(fmt.Sprintf("diff  v%d → v%d", d.a, d.b)))
	b.WriteString("\n")
	b.WriteString(m.st.Dim.Render(d.pathLabel))
	b.WriteString("\n\n")

	anyRevealed := false
	for i, row := range d.rows {
		marker := "  "
		if i == m.diffCursor {
			marker = m.st.Cursor.Render("› ")
		}
		if m.diffRevealed[row.key] {
			anyRevealed = true
		}
		key := ui.Sanitize(row.key)
		was, now := m.diffSide(row.key, row.aValue), m.diffSide(row.key, row.bValue)

		var rest string
		switch row.kind {
		case "added":
			rest = m.st.OKBanner.Render(fmt.Sprintf("+ %-16s %s", key, now))
		case "removed":
			rest = m.st.Danger.Render(fmt.Sprintf("- %-16s %s", key, was))
		case "changed":
			rest = m.st.Changed.Render(fmt.Sprintf("~ %-16s %s → %s", key, was, now))
		default:
			rest = m.st.Dim.Render(fmt.Sprintf("  %-16s %s", key, now))
		}
		b.WriteString(marker)
		b.WriteString(rest)
		b.WriteString("\n")
	}

	// The footer names what the key would do next rather than what it did, the
	// same way the detail pane's per-row hint does: offering to reveal what is
	// already on screen reads as a second, unrelated action. Drawn as bracketed
	// pairs because that is what a key looks like everywhere else in the app.
	reveal, closes := ui.Entry("r", "reveal fields"), ui.Entry("esc", "closes")
	if anyRevealed {
		reveal, closes = ui.Entry("r", "hide fields"), ui.Entry("esc", "closes and re-masks")
	}
	b.WriteString("\n")
	b.WriteString(m.st.HintLine([]ui.LegendEntry{ui.Entry("↑/↓", "move"), reveal, closes}))
	return m.st.ModalBox(strings.Split(b.String(), "\n"), bodyHeight, m.width)
}

// diffSide is one half of a row: the real value once revealed, dots until then.
// Sanitized here and not in valueToString, which also feeds the editor and the
// clipboard - only what gets painted is made safe to paint. modalBox clips.
func (m *Model) diffSide(key, value string) string {
	if !m.diffRevealed[key] {
		return m.st.Masked.Render(ui.MaskedValue)
	}
	if value == "" {
		return m.st.Dim.Render("(empty)")
	}
	return ui.Sanitize(value)
}

// keyDiff routes keys while the diff modal is open. It used to answer four,
// all of which closed it: the comparison could say a key had changed and never
// what to, which is the work a diff exists to remove.
func (m *Model) keyDiff(d *diffResult, msg tea.KeyPressMsg) tea.Cmd {
	// A mode with no payload answers no key, and every arm below would be
	// reading through the nil. Falling back to browsing gives the keystroke
	// somewhere to land, the same way handleKey does for the other modes.
	if d == nil {
		m.mode = browsing{}
		return nil
	}
	last := max(0, len(d.rows)-1)

	switch msg.String() {
	case "esc", "q", "?", "i":
		// Reveals last exactly as long as the modal does. The detail pane drops
		// its own on navigation for the same reason: a value put on screen
		// deliberately should not still be there after moving on.
		m.diffRevealed = map[string]bool{}
		m.mode = browsing{}
	case "down":
		m.diffCursor = clampIndex(m.diffCursor+1, last)
	case "up":
		m.diffCursor = clampIndex(m.diffCursor-1, last)
	case "home":
		m.diffCursor = 0
	case "end":
		m.diffCursor = last
	case "r":
		// Every row at once. A diff is read as a whole - the question is what
		// changed between two versions, not what one key holds - so revealing
		// it a row at a time was asking for the same keystroke per line.
		keys := make([]string, 0, len(d.rows))
		for i := range d.rows {
			keys = append(keys, d.rows[i].key)
		}
		toggleAll(m.diffRevealed, keys)
	}
	return nil
}
