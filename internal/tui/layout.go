package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// How the terminal is divided, and nothing about what goes in the divisions.
// Every number is decided without rendering: the modal builders need the body
// height before there is a body, and View may not write to the model.

// The chrome is a known number of rows, which is what lets the body be sized
// without rendering first: every line is truncated and never wrapped.
// TestChromeIsTheHeightTheLayoutAssumes pins all four.
const (
	headerHeight = 4
	tabsHeight   = 1
	// One blank row above the tab strip and one below it. The strip reads as a
	// thing of its own rather than as the header's last line or the pane's first.
	headerGap = 1
	tabGap    = 1
)

// footerRows is the status line plus however many rows the legend takes. Still
// decided without rendering, since legendLines is pure - but no longer a
// function of the terminal size alone: a legend wraps in one tab and not another.
func (m *Model) footerRows() int {
	return 1 + len(m.legendLines(m.width))
}

// bodyHeight is how many rows the panels get. It is a pure function of the
// terminal size: View may not write to the model, and the modal builders need
// the same number without having rendered anything.
func (m *Model) bodyHeight() int {
	return max(1, m.height-headerHeight-headerGap-tabsHeight-tabGap-m.footerRows())
}

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
const narrowLimit = 72

// Right column sizing. The column holds the detail panel and, under it, the
// copy panel.
const (
	// minPanelOuter is the smallest a panel can be and still be one: a title
	// edge, a row of content, a bottom edge. titledPanel floors at the same
	// number, so less is not smaller but taller than asked, pushing the layout.
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

// rightColumn is how the right column is divided, which the renderer and
// syncDetail both have to agree on: one sizes the viewport and the other draws
// it, and a disagreement is a pane that scrolls by the wrong amount.
func (m *Model) rightColumn(bodyHeight int) (width, detailHeight, copyHeight int) {
	_, width = splitWidths(m.width)

	// Narrow enough and nothing the panel says survives the trip to the screen,
	// so it is given up whole rather than drawn as truncated commands. Width
	// before rows: zero rows and rows that do not fit are one answer here.
	rows := 0
	if ui.PanelContentWidth(width) >= copyPanelCols {
		rows = len(m.copyLines(ui.PanelContentWidth(width)))
	}

	detailHeight, copyHeight = splitHeights(bodyHeight, rows)
	return width, detailHeight, copyHeight
}

// splitWidths divides the terminal between the two panes.
func splitWidths(total int) (left, right int) {
	left = min(max(total/leftWidthPart, minLeftWidth), maxLeftWidth)
	return left, total - left
}

// detailGeometry is the size of the detail viewport in either layout: the top
// of the right column, or the whole body below narrowLimit. renderPanes draws
// it and syncDetail fills it, so both have to take the number from here.
func (m *Model) detailGeometry(bodyHeight int) (width, height int) {
	if m.width < narrowLimit {
		return ui.PanelContentWidth(m.width), ui.PanelContentHeight(bodyHeight)
	}

	rightWidth, detailHeight, _ := m.rightColumn(bodyHeight)
	return ui.PanelContentWidth(rightWidth), ui.PanelContentHeight(detailHeight)
}

func (m *Model) renderPanes(height int) string {
	// One pane below narrowLimit, and the focus says which: stepping into a
	// secret replaces the list and esc puts it back. Drawing the list
	// unconditionally left a narrow split unable to show a secret at all.
	if m.width < narrowLimit {
		if m.focus == ui.FocusDetail {
			return m.titledPanel(m.rightTitle(), m.width, height, true, m.detail.View())
		}
		left := m.renderList(ui.PanelContentWidth(m.width), ui.PanelContentHeight(height))
		return m.titledPanel(m.leftTitle(), m.width, height, true, left)
	}

	leftWidth, _ := splitWidths(m.width)
	rightWidth, detailHeight, copyHeight := m.rightColumn(height)

	left := m.renderList(ui.PanelContentWidth(leftWidth), ui.PanelContentHeight(height))

	// Height on a lipgloss style is a floor, not a ceiling: taller content
	// pushes the border down the screen. The left is clipped by the panel, the
	// right goes through a viewport so the overflow stays reachable.
	leftPane := m.titledPanel(m.leftTitle(), leftWidth, height, m.focus == ui.FocusList, left)
	rightPane := m.titledPanel(m.rightTitle(), rightWidth, detailHeight, m.focus == ui.FocusDetail, m.detail.View())

	// The copy panel never takes focus: it holds no cursor and nothing to
	// scroll, so an idle border is the honest way to draw it.
	if copyHeight > 0 {
		copyPane := m.titledPanel(copyPanelTitle, rightWidth, copyHeight, false,
			strings.Join(m.copyLines(ui.PanelContentWidth(rightWidth)), "\n"))
		rightPane = lipgloss.JoinVertical(lipgloss.Left, rightPane, copyPane)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

// leftTitle names what the list is showing.
func (m *Model) leftTitle() string {
	left, _ := m.currentTab().Titles()
	return left
}

// rightTitle names what the detail pane is showing, which doubles as the
// breadcrumb for a tree scrolled deep enough to lose its context.
func (m *Model) rightTitle() string {
	_, right := m.currentTab().Titles()
	return right
}

// clipHeight drops whatever does not fit, so a pane can never grow its border
// past the bottom of the screen.
func clipHeight(content string, height int) string {
	// A negative height is a slice bound, not a small screen: lines[:-1] does
	// not clip, it panics. Every caller floors its height today, which is
	// exactly the kind of guarantee that holds until one of them stops.
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= height {
		return content
	}
	return strings.Join(lines[:height], "\n")
}

// renderList is the left pane of the active tab, filled while rendering.
func (m *Model) renderList(width, height int) string {
	return m.currentTab().RenderList(width, height, m.uiContext())
}

// renderDetail is the right pane of the active tab, filled from syncDetail.
func (m *Model) renderDetail(width int) string {
	return m.currentTab().RenderDetail(width, m.uiContext())
}

// float centres content over the panes, which are drawn and then written over
// so what the modal asks about stays on screen. Clipped to the body first, or a
// tall box loses its bottom rows outside the background and ends mid-border.
func (m *Model) float(content string, height int) string {
	return ui.CompositeCenter(clipHeight(content, height), ui.PadHeight(m.renderPanes(height), height))
}
