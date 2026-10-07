package tui

import (
	"github.com/lucasassuncao/bezel/draw"
	"github.com/lucasassuncao/bezel/layout"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// How the terminal is divided, and nothing about what goes in the divisions.
// The shell resolves the tree; what is decided here is the tree's shape for
// this frame: which pane survives a narrow terminal, and whether the right
// column has room for the reference panel under the detail.

// Left pane sizing. The floor keeps paths readable; the ceiling stops an
// ultrawide terminal from handing half the screen to a column of short names
// while the document that needs the room is squeezed beside it.
const (
	minLeftWidth  = 26
	maxLeftWidth  = 52
	leftWidthPart = 3 // a third of the terminal, before the clamps
)

// narrowLimit is where two panes stop being readable and the detail collapses.
// At 44 the split was legal and useless: kv pads a label to fourteen, so every
// value was truncated to nothing. 72 leaves the detail 42 columns.
const narrowLimit = ui.NarrowLimit

// Right column sizing. The column holds the detail panel and, under it, the
// copy panel.
const (
	// minPanelOuter is the smallest a panel can be and still be one: a title
	// edge, a row of content, a bottom edge.
	minPanelOuter = 3

	// copyPanelPart is the share of the right column the copy panel takes: a
	// fifth of it, borders included.
	copyPanelPart = 5

	// copyPanelCeilingPart is the most of the column the panel may ever hold,
	// which is what stops the floor below from turning a reference panel into
	// the larger half of the column on a terminal too short for both.
	copyPanelCeilingPart = 3

	// copyPanelRows is the most the panel ever says - path, read, field,
	// version, metadata, list - and the point below which a share stops being a
	// smaller panel and starts being one with the commands cut off.
	copyPanelRows = 6

	// copyPanelCols is the same bound the other way: 57 columns, the width of
	// the longest line there is ("field vault kv get -mount=kv -field=host
	// app/prod/db"). Below it the panel spends six rows on ellipses.
	copyPanelCols = 57
)

// Leaf names, shared by the layout, the panes and the focus.
const (
	paneList   = "list"
	paneDetail = "detail"
	paneCopy   = "copy"
)

// splitHeights divides the right column between the detail and the copy panel.
// The copy panel takes a fixed share, not its own content height: sized to its
// rows, the detail above resized on every cursor move. rows == 0 gives it none.
func splitHeights(total, rows int) (detail, copyPanel int) {
	if rows <= 0 || total < 2*minPanelOuter {
		return total, 0
	}

	// A fifth, bounded at both ends: the floor keeps the commands from being cut
	// off, the ceiling keeps a reference panel from outgrowing what it
	// annotates. A third and not a half, because a half never bound at 80x24.
	copyPanel = max(minPanelOuter, total/copyPanelPart)
	copyPanel = max(copyPanel, copyPanelRows+2) // the title edge and the bottom edge
	copyPanel = min(copyPanel, max(minPanelOuter, total/copyPanelCeilingPart))

	return total - copyPanel, copyPanel
}

// leftColumn is the list pane's constraints.
func leftColumn() layout.Leaf {
	return layout.Fixed(paneList, layout.Ratio(1, leftWidthPart), layout.Min(minLeftWidth), layout.Max(maxLeftWidth))
}

// layout is this frame's tree. Below narrowLimit only the focused pane is
// drawn: stepping into a secret replaces the list and esc puts it back.
func (m *Model) layout() layout.Node {
	keep := paneList
	if m.focus == ui.FocusDetail {
		keep = paneDetail
	}
	right := layout.Node(layout.Fill(paneDetail))
	if h := m.copyPanelHeight(); h > 0 {
		right = layout.Rows(layout.Fill(paneDetail), layout.Fixed(paneCopy, layout.Lines(h)).Info())
	}
	return layout.Columns(leftColumn(), right).Collapse(narrowLimit, keep)
}

// copyPanelHeight is the outer height the reference panel gets, or zero when
// it has nothing to say or the column is too narrow for what it says: then
// it is given up whole rather than drawn as truncated commands.
func (m *Model) copyPanelHeight() int {
	if m.width < narrowLimit {
		return 0
	}
	width := draw.InnerRect(m.sh.Rect(paneDetail)).W
	if width < copyPanelCols {
		return 0
	}
	_, h := splitHeights(m.sh.Body().H, len(m.copyLines(width)))
	return h
}

// relayout hands the shell this frame's tree and focus. Called after every
// Update so what View draws and what syncDetail sized agree.
func (m *Model) relayout() {
	focus := paneList
	if m.focus == ui.FocusDetail {
		focus = paneDetail
	}
	m.sh = m.sh.SetLayout(m.layout()).SetFocus(focus)
}

// helpKey is what the legend pins as help: "?" on a screen that browses,
// nothing where the screen takes keystrokes as text. A modal or panel over the
// screen never changes it: the legend is the screen's.
func (m *Model) helpKey() string {
	if m.currentTab().ScreenCaptures() {
		return ""
	}
	return "?"
}

// bodyHeight is how many rows the panels get.
func (m *Model) bodyHeight() int { return m.sh.Body().H }

// detailGeometry is the size of the detail viewport in either layout: the top
// of the right column, or the whole body below narrowLimit.
func (m *Model) detailGeometry() (width, height int) {
	rect := m.sh.Rect(paneDetail)
	if rect == (layout.Rect{}) {
		// Not placed: the list has the narrow screen. Size the detail as the
		// whole body so stepping into it lands on a sized viewport.
		rect = m.sh.Body()
	}
	r := draw.InnerRect(rect)
	return r.W, r.H
}
