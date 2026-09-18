package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The legend is the whole answer to "this app is for someone who does not use
// Vault every day". At any width it must at least offer the way to the rest.
func TestLegendKeepsHelpAtEveryWidth(t *testing.T) {
	h := newHarness(t)

	for _, width := range []int{40, 60, 80, 120, 200} {
		line := h.m.renderLegend(width)
		if !strings.Contains(line, "[?]") {
			t.Errorf("width %d: the help key was truncated away: %q", width, line)
		}
	}
}

// Movement outranks the destructive actions, which are the ones an occasional
// user is least likely to remember and the ones the ? panel exists to carry.
func TestLegendDropsRareActionsBeforeMovement(t *testing.T) {
	h := newHarness(t)
	// On a secret, so the destructive keys are offered at all: the legend shows
	// what the cursor is on, and a folder has nothing to delete.
	h.open("kv/app/prod/db")

	line := h.m.renderLegend(60)
	if !strings.Contains(line, "move") {
		t.Errorf("movement was dropped before the rare actions: %q", line)
	}
	if strings.Contains(line, "delete") {
		t.Errorf("a destructive action survived at 60 columns, so the order is wrong: %q", line)
	}
}

// The separator used to be costed in bytes ("  •  " is seven bytes and five
// columns), so the line gave up about two columns early per pair. Given room
// for everything, nothing may be dropped.
func TestLegendFitsEverythingWhenThereIsRoom(t *testing.T) {
	h := newHarness(t)
	// The widest legend is the one on a secret, which is the only place every
	// action applies. That is the one that has to fit.
	h.open("kv/app/prod/db")

	line := stripANSI(h.m.renderLegend(300))
	if strings.Contains(line, "…") {
		t.Errorf("nothing should be truncated at 300 columns: %q", line)
	}
	// The first and last pairs of the list: if both are present nothing in
	// between was dropped. The middle keys are not named here because what they
	// are depends on the node under the cursor.
	for _, want := range []string{"[?]", "[d] delete"} {
		if !strings.Contains(line, want) {
			t.Errorf("at 300 columns %s should fit: %q", want, line)
		}
	}
}

// The legend and the palette are two lists of what can be done here, and only
// the palette was ever checked against a scope. A key advertised in the bar
// that the palette does not offer is a key that does nothing when pressed.
func TestTheLegendNeverOffersAKeyThePaletteWouldNot(t *testing.T) {
	// Movement and the panels are not commands: they are how you get anywhere,
	// and the palette has nothing to say about them.
	navigation := map[string]bool{
		"?": true, "tab": true, ":": true, "↑/↓": true,
		"→/enter": true, "esc/←": true, "pgup/pgdn": true,
	}

	for _, where := range []struct {
		name string
		walk func(*harness)
	}{
		{"on a mount", func(*harness) {}},
		{"on a folder", func(h *harness) { h.open("kv/app/prod") }},
		{"on a secret", func(h *harness) { h.open("kv/app/prod/db") }},
		{"in the detail pane", func(h *harness) { h.open("kv/app/prod/db"); h.press("enter") }},
	} {
		t.Run(where.name, func(t *testing.T) {
			h := newHarness(t)
			where.walk(h)

			offered := map[string]bool{}
			for _, c := range h.m.available() {
				offered[c.key] = true
			}
			for _, e := range h.m.legend() {
				if navigation[e.Key] || offered[e.Key] {
					continue
				}
				t.Errorf("the bar offers [%s] %s, which the palette does not", e.Key, e.Action)
			}
		})
	}
}

// The ellipsis is part of the line, so it has to come out of the same budget.
// It did not, and the byte-counted separator was the only thing hiding it: the
// footer overflowed its own pane by up to two columns.
func TestLegendNeverOverflowsIncludingTheEllipsis(t *testing.T) {
	h := newHarness(t)

	for _, width := range []int{20, 30, 40, 50, 60, 80, 100, 140} {
		if got := lipgloss.Width(h.m.renderLegend(width)); got > width {
			t.Errorf("width %d: rendered %d columns: %q", width, got, h.m.renderLegend(width))
		}
	}
}

// The legend used to carry "[→] open folder" and "[enter] detail" side by side,
// as fixed text, which was wrong wherever the cursor was: the two keys always do
// one thing, and what that thing is depends on the node.
func TestLegendDescribesWhatTheCursorIsOn(t *testing.T) {
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})

	h.focus("kv/")
	folder := h.m.renderLegend(200)
	if !strings.Contains(folder, "→/enter") {
		t.Errorf("on a folder the two keys are the same and should be shown together: %q", folder)
	}
	if strings.Contains(folder, "detail") {
		t.Errorf("on a folder enter expands, it does not open a detail: %q", folder)
	}

	h.open("kv/app/prod/db")
	secret := h.m.renderLegend(200)
	if strings.Contains(secret, "open folder") {
		t.Errorf("a secret has no folder to open: %q", secret)
	}
	if !strings.Contains(secret, "focus detail") {
		t.Errorf("on a secret both keys step into the detail pane: %q", secret)
	}
	if !strings.Contains(secret, "[→/enter]") {
		t.Errorf("right steps in on a secret too and should be shown: %q", secret)
	}
}

// Below narrowLimit there is one pane and enter drills into it, replacing the
// list rather than focusing a column beside it. The legend must name the verb
// that happens: "focus detail" describes a pane not on screen at this width.
func TestLegendNamesTheNarrowVerbForOpeningASecret(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.m.Update(tea.WindowSizeMsg{Width: narrowLimit - 1, Height: 20})

	var offered string
	for _, hi := range h.m.legend() {
		if strings.Contains(hi.Action, "focus detail") {
			t.Errorf("a terminal too narrow for two panes still offers to focus one: %+v", hi)
		}
		if strings.Contains(hi.Action, "open secret") {
			offered = hi.Key
		}
	}
	if offered == "" {
		t.Error("the legend never says how to open the secret under the cursor")
	}
}

// The legend answers "what can I press here", and here is a tab and a focus
// together. The focused-pane branch sat ahead of the tabs, so a policy document
// got the secrets legend: five keys, none of which does anything to a policy.
func TestTheLegendOnlyNamesKeysThatDoSomethingHere(t *testing.T) {
	// The keys that belong to a secret and to nothing else.
	secretsOnly := []string{"reveal", "edit", "versions", "delete", "fields", "copy secret"}

	for _, tc := range []struct {
		tab   string
		key   string
		wants []string
	}{
		{"policies", "2", []string{"scroll", "copy HCL"}},
		{"auth", "3", []string{"scroll", "go to policy"}},
		{"activity", "4", []string{"scroll"}},
	} {
		h := newHarness(t)
		h.press(tc.key)
		// Right walks inward, and how far inward the pane is depends on the
		// tab: Auth stops at the role list on the way.
		for range 3 {
			if h.m.focus == ui.FocusDetail {
				break
			}
			h.press("right")
		}
		if h.m.focus != ui.FocusDetail {
			t.Fatalf("%s: precondition, the detail pane should have focus", tc.tab)
		}

		var named []string
		for _, entry := range h.m.legend() {
			named = append(named, entry.Action)
		}
		line := strings.Join(named, " · ")

		for _, unwanted := range secretsOnly {
			if strings.Contains(line, unwanted) {
				t.Errorf("%s detail: the legend offers %q, which belongs to a secret: %s",
					tc.tab, unwanted, line)
			}
		}
		for _, want := range tc.wants {
			if !strings.Contains(line, want) {
				t.Errorf("%s detail: the legend does not offer %q: %s", tc.tab, want, line)
			}
		}
		// Every focused pane has to say how to leave it: the arrows all belong
		// to the pane while it holds focus, so none of them doubles as a way out.
		if !strings.Contains(line, "back to list") {
			t.Errorf("%s detail: the legend does not say how to leave the pane: %s", tc.tab, line)
		}
	}
}

// The help panel and the legend describe the same screen and must not disagree
// about what that screen is. The panel had the tab-and-focus pairing right while
// the legend did not, which is how the mismatch went unnoticed.
func TestTheHelpPanelAgreesWithTheLegendAboutTheDetailPane(t *testing.T) {
	for _, key := range []string{"1", "2", "3", "4"} {
		h := newHarness(t)
		h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 44})
		h.open("kv/app/prod/db")
		h.press(key)
		h.press("right")
		if h.m.focus != ui.FocusDetail {
			continue // this tab has nowhere to focus, which is its own answer
		}

		legendScrolls := false
		for _, entry := range h.m.legend() {
			if entry.Action == "scroll" || entry.Action == "fields" {
				legendScrolls = true
			}
		}
		if !legendScrolls {
			t.Errorf("tab %s: the legend says nothing about what the arrows do in the pane", key)
		}

		h.press("?")
		panel := h.view()
		if !strings.Contains(panel, "Anywhere") {
			t.Fatalf("tab %s: the help panel is not on screen", key)
		}
		if !strings.Contains(panel, "back to the list") {
			t.Errorf("tab %s: the help panel does not say how to leave the pane:\n%s", key, panel)
		}
	}
}

// The legend is the route to everything else, so the way in has to be on it.
func TestTheLegendOffersTheCommandLine(t *testing.T) {
	h := newHarness(t)

	if line := h.m.renderLegend(120); !strings.Contains(line, "[:]") {
		t.Errorf("the legend does not mention the command line: %q", line)
	}
}

// The status line says nothing about the tree until there is something to say.
// A count of expanded nodes reports what was opened rather than what exists, so
// it changes as you browse. The filter stays: the user put the pane into it.
func TestTheSecretsFooterOnlyReportsTheFilter(t *testing.T) {
	h := newHarness(t)

	if strings.Contains(h.view(), "nodes") {
		t.Errorf("the footer still counts nodes:\n%s", h.view())
	}

	h.press("/")
	h.typeText("kv")
	h.press("enter")

	if !strings.Contains(h.view(), "filter: kv") {
		t.Errorf("the footer does not report the filter the pane is under:\n%s", h.view())
	}
}

// The point of the second line: the secrets legend names ten actions and wants
// 143 columns, so on one line "drop the rare keys first" was the permanent
// state rather than a degradation path. Two lines hold it from here up.
//
// The floor climbed from eighty as words were added: "[tab] change tab" took it
// to eighty-four, and naming what "r" and "y" act on - "reveal secret", "copy
// secret" - to eighty-eight. Line two has no slack, and each addition also
// summons the "+n in [?]" mark, so one entry in pushes two out.
func TestTheWholeLegendSurvivesAnEightyEightColumnTerminal(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.m.Update(tea.WindowSizeMsg{Width: 88, Height: 30})

	lines := h.m.legendLines(88)
	if len(lines) != 2 {
		t.Errorf("the legend takes %d lines at eighty-eight columns, want 2", len(lines))
	}

	rendered := strings.Join(lines, "\n")
	for _, e := range h.m.legend() {
		if !strings.Contains(rendered, e.Action) {
			t.Errorf("%q is missing from the legend at eighty-eight columns:\n%s", e.Action, rendered)
		}
	}
	if strings.Contains(rendered, "in [?]") {
		t.Errorf("the legend claims keys were dropped but everything fits:\n%s", rendered)
	}
}

// Below about seventy columns not even two lines hold it, and the mark has to
// say how many keys it stands for: an ellipsis says something is missing, a
// count says one or six - the difference between pressing on and pressing "?".
func TestTheLegendCountsWhatItHadToDrop(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	total := len(h.m.legend())
	lines := h.m.legendLines(50)
	rendered := strings.Join(lines, "\n")

	if len(lines) > legendMaxLines {
		t.Fatalf("the legend spent %d lines, the footer budgets %d", len(lines), legendMaxLines)
	}

	shown := 0
	for _, e := range h.m.legend() {
		if strings.Contains(rendered, e.Action) {
			shown++
		}
	}
	if shown == total {
		t.Skip("fifty columns turned out to be enough; nothing was dropped")
	}
	if want := fmt.Sprintf("+%d in [?]", total-shown); !strings.Contains(rendered, want) {
		t.Errorf("the legend dropped %d keys and does not say so - wanted %q in:\n%s",
			total-shown, want, rendered)
	}
}

// The wording of the "r" toggle is the Secrets tab's own and is proved there:
// it conjugates on which fields are up, which is that tab's state.
