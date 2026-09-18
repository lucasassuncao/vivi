package tui

import (
	"io"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// monochrome is what a terminal without colour makes of a frame. v2 renders the
// escapes whatever it is drawing into and downsamples them where it writes, so
// this is that writer, told to keep nothing.
func monochrome(s string) string {
	var b strings.Builder
	w := colorprofile.NewWriter(&b, nil)
	w.Profile = colorprofile.Ascii
	_, _ = io.WriteString(w, s)
	return b.String()
}

func TestPanelsAreTitled(t *testing.T) {
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	h.open("kv/app/prod/db")
	h.press("enter")

	view := h.view()
	if !strings.Contains(view, "Secrets") {
		t.Fatal("the list panel should be titled")
	}
	// The detail title doubles as the breadcrumb for the selected path.
	if !strings.Contains(view, "kv/app/prod/db") {
		t.Fatal("the detail panel should be titled with the selected path")
	}
}

// Section headings used to be rules: the name plus box-drawing dashes to the
// pane's full width, which at 250 columns spent two hundred dashes on one word.
// The heading is typographic now, so a wider terminal must not lengthen it.
func TestSectionHeadingsDoNotGrowWithTheTerminal(t *testing.T) {
	// The pane borders still scale with the terminal, so the check is scoped to
	// the heading lines themselves: those are the ones that used to be rules.
	for _, w := range []int{100, 250} {
		h := newHarness(t)
		h.m.Update(tea.WindowSizeMsg{Width: w, Height: 40})
		h.open("kv/app/prod/db")

		headings := 0
		for _, line := range strings.Split(h.view(), "\n") {
			if !strings.Contains(line, "DATA") && !strings.Contains(line, "VERSIONS") &&
				!strings.Contains(line, "METADATA") {
				continue
			}
			headings++
			if run := longestRun(line, '─'); run > 0 {
				t.Errorf("width %d: a section heading still draws a rule (%d dashes): %q",
					w, run, line)
			}
		}
		if headings == 0 {
			t.Fatalf("width %d: no section headings on screen to check", w)
		}
	}
}

// longestRun is the longest unbroken run of r in s.
func longestRun(s string, r rune) int {
	best, run := 0, 0
	for _, c := range s {
		if c == r {
			run++
			if run > best {
				best = run
			}
			continue
		}
		run = 0
	}
	return best
}

// Focus is drawn twice and only one survives a terminal without colour: the
// Ascii profile drops foreground and background, so the two panels came out
// byte for byte identical. The border characters carry it, and this pins that.
func TestFocusSurvivesAMonochromeTerminal(t *testing.T) {
	h := newHarness(t)
	focused := monochrome(h.m.titledPanel("Secrets", 30, 5, true, "content"))
	idle := monochrome(h.m.titledPanel("Secrets", 30, 5, false, "content"))

	if focused == idle {
		t.Errorf("the focused panel is indistinguishable without colour:\n%s", focused)
	}
	// Both panels are the size they were asked for, whichever border set they
	// drew: a heavier box that measured differently would break the side by side
	// join the moment focus moved.
	for _, panel := range []struct {
		name    string
		content string
	}{{"focused", focused}, {"idle", idle}} {
		if got := lipgloss.Height(panel.content); got != 5 {
			t.Errorf("the %s panel is %d rows, want 5", panel.name, got)
		}
		if got := lipgloss.Width(panel.content); got != 30 {
			t.Errorf("the %s panel is %d columns, want 30", panel.name, got)
		}
	}
}
