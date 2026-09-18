package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// View is the dispatcher, so what is tested here is the property every mode has
// to have: it draws, it fits, and it changes nothing. What each mode draws is
// tested with the component that draws it.

// A terminal can be any size, and a panic inside View leaves the user in a
// broken alternate screen with no way to read the error.
func TestViewSurvivesEverySizeAndMode(t *testing.T) {
	sizes := []tea.WindowSizeMsg{
		{Width: 40, Height: 10},
		{Width: 80, Height: 24},
		{Width: 200, Height: 60},
		{Width: 20, Height: 5},
	}

	for _, size := range sizes {
		h := newHarness(t)
		h.m.Update(size)
		h.open("kv/app/prod/db")
		h.press("enter")

		checks := []struct {
			name string
			keys []string
		}{
			{"browse", nil},
			{"versions", []string{"V"}},
			{"confirm", []string{"V", "D"}},
			{"editor", []string{"e"}},
			{"help", []string{"?"}},
			{"token", []string{"i"}},
			{"filter", []string{"/"}},
			{"activity tab", []string{"4"}},
			{"policies tab", []string{"2"}},
			{"auth tab", []string{"3"}},
		}

		for _, c := range checks {
			h := newHarness(t)
			h.m.Update(size)
			h.open("kv/app/prod/db")
			h.press("enter")
			h.press(c.keys...)

			view := h.view()
			if view == "" {
				t.Errorf("%dx%d %s: empty view", size.Width, size.Height, c.name)
			}
			lines := strings.Split(view, "\n")
			if len(lines) > size.Height {
				t.Errorf("%dx%d %s: view is %d lines, %d too tall",
					size.Width, size.Height, c.name, len(lines), len(lines)-size.Height)
			}
			for _, line := range lines {
				if width := lineWidth(line); width > size.Width {
					t.Errorf("%dx%d %s: line overflows by %d columns: %q",
						size.Width, size.Height, c.name, width-size.Width, line)
					break
				}
			}
			h.press("esc")
		}
	}
}

// The layout arithmetic assumes positive dimensions. Zero is the state before
// the first WindowSizeMsg and negative should not happen - but strings.Repeat
// and every slice bound would turn that into a crash in raw mode.
func TestRenderingSurvivesDegenerateTerminalSizes(t *testing.T) {
	sizes := []struct{ w, h int }{
		{0, 0}, {-1, -1}, {1, 1}, {2, 3}, {10, 4},
		{narrowLimit - 1, 5}, {narrowLimit, 6}, {400, 3},
	}

	for _, size := range sizes {
		h := newHarness(t)
		h.open("kv/app/prod/db")
		h.m.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})

		// Every mode gets drawn at every size: the command line, the modals and
		// the help panel each do their own arithmetic.
		for _, open := range []func(){
			func() {},
			func() { h.press(":") },
			func() { h.press("?") },
			func() { h.press("i") },
			func() { h.press("V") },
			func() { h.press("e") },
			func() { h.press("/") },
		} {
			open()
			h.view() // panics on failure, which is the assertion
			h.press("esc")
			h.press("esc")
		}
	}
}

// A Vault policy is routinely longer than the terminal - the built-in "default"
// is around ninety lines. Before the detail pane was clipped, one of those
// pushed its own border off the bottom and wrecked the layout.
func TestLongDetailContentIsClippedAndScrollable(t *testing.T) {
	h := newHarness(t)

	long := strings.Repeat("path \"sys/whatever\" {\n  capabilities = [\"read\"]\n}\n\n", 30)
	h.m.policiesTab.Restore([]string{"default"}, 0, long)
	h.m.tab = tabPolicies

	// The size message comes after the fields are set, because it is the message
	// that re-projects the detail viewport: the pane is filled by the update
	// loop now, so a test that reaches into the model has to let one run.
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	view := h.view()
	if lines := strings.Split(view, "\n"); len(lines) > 24 {
		t.Fatalf("a long policy overflowed the screen: %d lines", len(lines))
	}
	if !strings.Contains(view, "detail 0%") {
		t.Fatal("the footer must say where in the document the pane is")
	}

	// The rest has to be reachable, or the pane is just hiding data. Scrolling
	// happens in the pane, which the right arrow enters.
	h.press("right")
	if h.m.focus != ui.FocusDetail {
		t.Fatal("right should move focus into the detail pane")
	}

	before := h.m.detail.YOffset()
	h.press("pgdown")
	if h.m.detail.YOffset() <= before {
		t.Fatal("pgdown should scroll the detail pane")
	}

	h.press("pgup")
	if h.m.detail.YOffset() != before {
		t.Fatalf("pgup should scroll back, got offset %d", h.m.detail.YOffset())
	}

	// Scrolling cannot run off the end.
	for range 100 {
		h.press("pgdown")
	}
	if h.m.detail.YOffset() > h.m.detail.TotalLineCount()-h.m.detail.Height() {
		t.Fatalf("offset %d ran past the end (%d lines, %d tall)",
			h.m.detail.YOffset(), h.m.detail.TotalLineCount(), h.m.detail.Height())
	}
	if lines := strings.Split(h.view(), "\n"); len(lines) > 24 {
		t.Fatalf("overflowed after scrolling: %d lines", len(lines))
	}
}

func TestDetailScrollResetsWhenTheSelectionChanges(t *testing.T) {
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	h.m.policiesTab.Restore([]string{"a", "b"}, 0, strings.Repeat("line\n", 100))
	h.m.tab = tabPolicies
	h.view()

	h.press("right", "pgdown")
	if h.m.detail.YOffset() == 0 {
		t.Fatal("precondition: the pane should have scrolled")
	}

	h.press("esc", "down")
	if h.m.detail.YOffset() != 0 {
		t.Fatal("moving to another policy must start it from the top")
	}
}

// View is a pure function of the model. It used to record three things while
// rendering, which locked the update loop into running after a render: the
// field cursor scrolled by an offset that existed only once View had been called.
func TestViewDoesNotMutateTheModel(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	type state struct {
		cursor, fieldCursor, versionCursor int
		detailW, detailH, yOffset          int
		mode                               string
		pending                            int
	}
	get := func() state {
		s := h.m.secretsTab.State()
		return state{
			s.TreeCursor, s.FieldCursor, s.VersionCursor,
			h.m.detail.Width(), h.m.detail.Height(), h.m.detail.YOffset(),
			h.mode(), h.m.pending,
		}
	}

	// Every mode, since each takes a different path through View.
	for _, keys := range [][]string{{}, {"?"}, {"i"}, {"V"}, {"/"}, {"e"}, {"d"}} {
		h.press(keys...)
		before := get()
		first := h.view()
		after := get()
		if before != after {
			t.Errorf("View mutated the model after %v:\n before %+v\n after  %+v", keys, before, after)
		}
		// And it is idempotent: rendering twice gives the same screen.
		if second := h.view(); first != second {
			t.Errorf("View is not idempotent after %v", keys)
		}
		h.press("esc", "esc")
	}
}

// A window below the minimum used to be drawn anyway: the panel floors at three
// rows, the difference came off the bottom, and the legend went with it - so the
// line saying "?" reaches the key list was the first casualty.
func TestATerminalBelowTheMinimumSaysSo(t *testing.T) {
	sizes := []struct{ w, h int }{
		{minTerminalWidth - 1, minTerminalHeight},
		{minTerminalWidth, minTerminalHeight - 1},
		{20, 5},
	}

	for _, size := range sizes {
		h := newHarness(t)
		h.m.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})

		view := stripANSI(h.view())
		if !strings.Contains(view, "too small") {
			t.Errorf("%dx%d: the screen does not say why it is empty:\n%s", size.w, size.h, view)
		}
		if !strings.Contains(view, "30×10") {
			t.Errorf("%dx%d: the notice does not name the size vivi needs:\n%s", size.w, size.h, view)
		}
		for i, line := range strings.Split(h.view(), "\n") {
			if width := lineWidth(line); width > size.w {
				t.Errorf("%dx%d: row %d of the notice is %d columns", size.w, size.h, i, width)
				break
			}
		}
	}

	// One column and one row above the floor is the app, not the notice.
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: minTerminalWidth, Height: minTerminalHeight})
	if strings.Contains(stripANSI(h.view()), "too small") {
		t.Errorf("the declared minimum should draw the app:\n%s", h.view())
	}
}
