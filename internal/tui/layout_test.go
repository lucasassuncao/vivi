package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The left column is a third of the terminal, but only between a floor that
// keeps paths readable and a ceiling that stops an ultrawide screen from
// handing half the width to short names.
func TestLeftPaneWidthIsClamped(t *testing.T) {
	tests := []struct {
		total, wantLeft int
	}{
		{80, minLeftWidth},
		{120, 40},
		{400, maxLeftWidth},
	}

	for _, tc := range tests {
		left, right := splitWidths(tc.total)
		if left != tc.wantLeft {
			t.Errorf("width %d: left pane %d, want %d", tc.total, left, tc.wantLeft)
		}
		if left+right != tc.total {
			t.Errorf("width %d: panes sum to %d", tc.total, left+right)
		}
	}
}

// The two panels together are the column, and neither may be taller than what it
// was given: a column that overflows pushes its own bottom border off the screen.
func TestTheRightColumnAlwaysFitsItsHeight(t *testing.T) {
	for _, height := range []int{6, 8, 10, 12, 16, 24, 40, 80} {
		h := newHarness(t)
		h.m.Update(tea.WindowSizeMsg{Width: 120, Height: height})
		h.open("kv/app/prod/db")

		body := h.m.bodyHeight()
		_, detail, copyPanel := h.m.rightColumn(body)

		if copyPanel > 0 && detail+copyPanel != body {
			t.Errorf("height %d: the panels sum to %d, want the body height %d",
				height, detail+copyPanel, body)
		}
		if copyPanel > 0 && copyPanel > detail {
			t.Errorf("height %d: the copy panel (%d) outgrew the detail it annotates (%d)",
				height, copyPanel, detail)
		}
	}
}

// The copy panel is a fixed share of the column, not the height of what it
// holds. Sized to its contents it changed as the cursor moved - two rows, five,
// six - and the detail above resized every time, so reading meant watching.
func TestTheCopyPanelKeepsItsShareAsTheCursorMoves(t *testing.T) {
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 44})
	h.open("kv/app/prod/db")

	body := h.m.bodyHeight()
	_, onSecret, copyOnSecret := h.m.rightColumn(body)

	// A folder has fewer rows to show; the panels must not move for it.
	h.press("up")
	_, onFolder, copyOnFolder := h.m.rightColumn(body)
	if copyOnFolder != copyOnSecret || onFolder != onSecret {
		t.Errorf("the split moved with the selection: secret %d/%d, folder %d/%d",
			onSecret, copyOnSecret, onFolder, copyOnFolder)
	}

	// A historical version adds one; likewise.
	h.press("down")
	h.press("V")
	h.press("down")
	h.press("enter")
	_, onHistorical, copyOnHistorical := h.m.rightColumn(body)
	if copyOnHistorical != copyOnSecret || onHistorical != onSecret {
		t.Errorf("the split moved for a historical version: %d/%d, want %d/%d",
			onHistorical, copyOnHistorical, onSecret, copyOnSecret)
	}
}

// The detail viewport is the top panel now, not the whole column. Sized to the
// column it would believe it can show rows the copy panel is standing on, and
// the scroll percentage in the footer would report on a pane that big.
func TestTheDetailViewportIsSizedToItsOwnPanel(t *testing.T) {
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	h.open("kv/app/prod/db")

	_, detail, copyPanel := h.m.rightColumn(h.m.bodyHeight())
	if copyPanel == 0 {
		t.Fatal("precondition: this screen should have a copy panel")
	}
	if got, want := h.m.detail.Height(), ui.PanelContentHeight(detail); got != want {
		t.Errorf("the viewport is %d rows, but its panel holds %d", got, want)
	}
}

// A share of the column is the rule, but a share of a short one is a border
// with a path and nothing else - the commands get cut. The floor stops that;
// the ceiling stops the floor making the panel the larger half.
func TestTheCopyPanelIsBoundedAtBothEnds(t *testing.T) {
	for _, body := range []int{6, 8, 11, 15, 19, 29, 39, 55, 75, 200} {
		detail, copyPanel := splitHeights(body, copyPanelRows)

		switch {
		case detail+copyPanel != body:
			t.Errorf("body %d: the panels sum to %d", body, detail+copyPanel)
		case copyPanel > detail:
			t.Errorf("body %d: the copy panel (%d) outgrew the detail (%d)", body, copyPanel, detail)
		case copyPanel < minPanelOuter:
			t.Errorf("body %d: the copy panel is %d rows, below what a panel costs", body, copyPanel)
		}

		// Where the column can afford both panels every row, it must: the floor
		// doing its job. Below that the ceiling wins and a row is lost, which
		// beats a reference panel taking a third from the secret it annotates.
		if body >= copyPanelCeilingPart*(copyPanelRows+2) && copyPanel-2 < copyPanelRows {
			t.Errorf("body %d: the copy panel holds %d of its %d rows while the detail has %d",
				body, copyPanel-2, copyPanelRows, detail)
		}
	}
}

// Nothing to address means no panel, at any height: a border spending rows on
// an empty rectangle is worse than the single panel it replaced.
func TestNoRowsMeansNoPanelAtAnyHeight(t *testing.T) {
	for _, body := range []int{1, 6, 20, 60} {
		if detail, copyPanel := splitHeights(body, 0); copyPanel != 0 || detail != body {
			t.Errorf("body %d: split %d/%d, want the whole column to the detail", body, detail, copyPanel)
		}
	}
}

// The two panels sit side by side, so a disagreement about height leaves one
// border hanging below the other. It happened on Activity: the empty state's
// hint was wider than the pane, wrapped, and the extra row pushed it down.
func TestBothPanelsAlwaysAgreeOnHeight(t *testing.T) {
	sizes := []tea.WindowSizeMsg{
		{Width: 200, Height: 64},
		{Width: 120, Height: 30},
		{Width: 90, Height: 20},
		{Width: 60, Height: 12},
	}

	for _, size := range sizes {
		for index, name := range tabNames {
			h := newHarness(t)
			h.m.Update(size)
			h.open("kv/app/prod/db")
			h.press("enter")
			h.m.tab = tab(index)

			bodyHeight := size.Height - 4 // header, tabs, and the two footer rows
			body := h.m.renderPanes(bodyHeight)

			lines := strings.Split(body, "\n")
			if len(lines) != bodyHeight {
				t.Errorf("%dx%d %s: body is %d rows, want %d",
					size.Width, size.Height, name, len(lines), bodyHeight)
			}
			for i, line := range lines {
				if width := lineWidth(line); width != size.Width {
					t.Errorf("%dx%d %s: row %d is %d columns, want %d",
						size.Width, size.Height, name, i, width, size.Width)
					break
				}
			}
		}
	}
}

// The layout sizes the body without measuring a render, sound only while the
// chrome is as tall as it says. The footer answers for itself, since the legend
// may take a second line, so its answer must match what it draws at any width.
func TestChromeIsTheHeightTheLayoutAssumes(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	for _, width := range []int{40, 60, 80, 100, 160, 220} {
		h.m.Update(tea.WindowSizeMsg{Width: width, Height: 30})

		if got := lipgloss.Height(h.m.renderFooter()); got != h.m.footerRows() {
			t.Errorf("width %d: footer draws %d lines, the layout was told %d",
				width, got, h.m.footerRows())
		}
		if got := lipgloss.Height(h.view()); got > 30 {
			t.Errorf("width %d: the screen is %d lines in a 30-line terminal", width, got)
		}
	}

	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if got := lipgloss.Height(h.m.renderHeader()); got != headerHeight {
		t.Errorf("header is %d lines, layout assumes %d", got, headerHeight)
	}
	if got := lipgloss.Height(h.m.renderTabs()); got != tabsHeight {
		t.Errorf("tab strip is %d lines, layout assumes %d", got, tabsHeight)
	}
	if got := lipgloss.Height(h.view()); got > 30 {
		t.Errorf("the screen is %d lines in a 30-line terminal", got)
	}
}

// A box that floats has to float over something. Painting it on an empty canvas
// blanks the body, which reads as a black rectangle on most terminals and hides
// the very path the modal is asking about.
func TestFloatingBoxesKeepThePanesBehindThem(t *testing.T) {
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 110, Height: 40})
	h.open("kv/app/prod/db")

	h.press("?")
	view := h.view()
	// "Anywhere" is the meta section, which every contextual help ends with, so
	// this asserts the modal is up without pinning which tab's keys it lists.
	if !strings.Contains(view, "Anywhere") {
		t.Fatal("the help should be on screen")
	}
	if !strings.Contains(view, "▾ kv/") {
		t.Fatalf("the tree should still be visible around the help:\n%s", view)
	}

	h.press("esc")
	h.press("enter")
	h.press("V")
	h.press("D")
	view = h.view()
	if !strings.Contains(view, "Destroy") {
		t.Fatal("the destroy modal should be on screen")
	}
	if !strings.Contains(view, "▾ kv/") {
		t.Fatalf("the tree should still be visible around the modal:\n%s", view)
	}
}

// A column too narrow for the commands gives the panel up whole: six rows of
// ellipsis are worth less than six more rows of the secret, and the lines that
// truncate first are the two nobody types from memory.
func TestTheCopyPanelGivesUpOnANarrowColumn(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	for _, tc := range []struct {
		width int
		want  bool
	}{
		{80, false}, // the column holds 56 columns: one short of the widest line
		{92, true},  // the first width where every command reads whole
		{120, true}, // and anything above it
	} {
		h.m.Update(tea.WindowSizeMsg{Width: tc.width, Height: 40})

		_, _, copyHeight := h.m.rightColumn(h.m.bodyHeight())
		if got := copyHeight > 0; got != tc.want {
			t.Errorf("width %d: copy panel present = %v, want %v (height %d)",
				tc.width, got, tc.want, copyHeight)
		}

		// Whatever the panel does, the column is spent: the detail takes back
		// exactly what the reference gave up.
		detailHeight := h.m.bodyHeight() - copyHeight
		if _, d, _ := h.m.rightColumn(h.m.bodyHeight()); d != detailHeight {
			t.Errorf("width %d: detail %d, want %d - the column does not add up",
				tc.width, d, detailHeight)
		}

		if copyHeight == 0 && strings.Contains(h.view(), copyPanelTitle) {
			t.Errorf("width %d: the panel is gone from the layout but still on screen", tc.width)
		}
	}
}

// The one thing the app exists to do has to work at every width it draws. Below
// narrowLimit the single pane used to be the list unconditionally, so a narrow
// split browsed forever and saw no value. Drill-down: esc gives the list back.
// A value taller than the terminal scrolls inside the form; the modal itself
// never grows past the body, which is what would push its hint and its border
// off the screen exactly when the value is the kind worth checking.
func TestATallValueScrollsInsideTheFormAt80x24(t *testing.T) {
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	h.open("kv/app/prod/db")
	h.press("enter", "e")

	h.press("a")
	h.typeText("tls_cert")
	h.press("enter", "enter")
	h.paste(strings.Repeat("MIIDdzCCAl+gAwIBAgIEAgAAuTANBgkqhkiG9w0BAQUFADBaM\n", 39) + "-----END-----")

	view := h.view()
	t.Logf("\n%s", view)
	if strings.Contains(view, "more lines, resize") {
		t.Errorf("the form overflowed the body instead of scrolling its value")
	}
	if !strings.Contains(view, "[ctrl+s] add") {
		t.Errorf("the hint fell off the bottom of the form")
	}
	if !strings.Contains(view, "40 lines") {
		t.Errorf("the count does not say how tall the value really is")
	}
	if got := strings.Count(view, "\n") + 1; got > 24 {
		t.Errorf("the screen is %d rows tall, the terminal is 24", got)
	}
}

func TestASecretIsReachableAtEveryWidth(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	keys := h.m.secretsTab.FieldKeys()
	if len(keys) == 0 {
		t.Fatal("precondition: the secret has no fields to look for")
	}
	field := keys[0]

	for _, width := range []int{minTerminalWidth, 40, 60, narrowLimit - 1, narrowLimit, 80, 120} {
		h.m.Update(tea.WindowSizeMsg{Width: width, Height: 24})

		h.press("esc") // back to the list, wherever the last width left the focus
		if h.m.focus != ui.FocusList {
			t.Fatalf("width %d: esc did not return to the list", width)
		}
		if view := h.view(); !strings.Contains(view, "app/prod") && !strings.Contains(view, "db") {
			t.Errorf("width %d: the list is focused but not on screen", width)
		}

		h.press("enter")
		if h.m.focus != ui.FocusDetail {
			t.Errorf("width %d: enter on a secret did not open it", width)
		}
		if view := h.view(); !strings.Contains(view, field) {
			t.Errorf("width %d: the secret is open but field %q is nowhere on screen:\n%s",
				width, field, view)
		}

		h.press("esc")
		if h.m.focus != ui.FocusList {
			t.Errorf("width %d: esc did not come back out of the secret", width)
		}
	}
}
