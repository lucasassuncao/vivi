package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// The command line itself: what it opens over, what it lists, and the arguments
// that are the reason it parses a line instead of matching a name. The table it
// reads from, and the scopes that decide what is on it, are in command_test.go.

// Esc returns to whatever the command line was opened from, which is not always
// browsing: the version list is the other place it opens.
func TestEscapeReturnsToTheModeItOpenedFrom(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("V")
	if h.mode() != "versions" {
		t.Fatalf("V did not open the version list: mode is %q", h.mode())
	}

	h.press(":")
	if h.mode() != "command" {
		t.Fatalf(`":" did not open from the version list: mode is %q`, h.mode())
	}
	h.press("esc")
	if h.mode() != "versions" {
		t.Errorf("esc left the version list behind: mode is %q", h.mode())
	}
}

// Inside the filter and the editor a colon is a character being typed. Opening
// the command line there would make the key unusable in a password or a path.
func TestColonIsLiteralTextWhereTextIsBeingTyped(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	h.press("/")
	h.press(":")
	if h.mode() != "filter" {
		t.Fatalf(`":" opened the command line from the filter: mode is %q`, h.mode())
	}
	if got := h.m.secretsTab.State().FilterValue; !strings.Contains(got, ":") {
		t.Errorf("the colon did not reach the filter: %q", got)
	}
}

func TestColonIsLiteralTextInTheEditor(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	h.press("e")
	if h.mode() != "edit" {
		t.Fatalf("e did not open the editor: mode is %q", h.mode())
	}
	h.press(":")
	if h.mode() != "edit" {
		t.Errorf(`":" opened the command line from the editor: mode is %q`, h.mode())
	}
}

// The keystroke column is the reason the panel exists.
func TestThePanelPrintsTheKeystrokeBesideTheName(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.typeCommand("rev")

	view := h.view()
	if !strings.Contains(view, ":reveal") {
		t.Errorf("the candidate is not listed: %q", view)
	}
	// The key is what the user is supposed to learn from the line.
	line := candidateLine(view, ":reveal")
	if !strings.Contains(line, "r") {
		t.Errorf("the reveal row does not name its key: %q", line)
	}
}

// Tab turns a choice into typed text, so that enter can then resolve it by the
// ordinary rule instead of remembering that a pick was made.
func TestTabCompletesTheOnlyCandidate(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.typeCommand("rev")
	h.press("tab")

	if h.mode() != "command" {
		t.Fatalf("tab closed the command line: mode is %q", h.mode())
	}
	if got := h.commandLine(); got != "reveal" {
		t.Errorf("tab completed to %q, want reveal", got)
	}
}

// An argument-taking command keeps the caret after a space: the line is not
// finished when the name is.
func TestTabLeavesRoomForAnArgument(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.typeCommand("got")
	h.press("tab")

	if got := h.commandLine(); got != "goto " {
		t.Errorf("tab completed to %q, want %q", got, "goto ")
	}
}

// An ambiguous line is the one failure worth staying open for: the answer is
// already on screen and one keystroke settles it.
func TestAnAmbiguousLineStaysOpenAndSaysSo(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.runCommand("cop")

	if h.mode() != "command" {
		t.Fatalf("an ambiguous line closed the command line: mode is %q", h.mode())
	}
	if !strings.Contains(h.view(), "cop is ambiguous") {
		t.Errorf("the ambiguity is not on screen: %q", h.view())
	}
}

// Arrow keys pick from the list, and the pick is what enter runs.
func TestArrowsPickFromTheList(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.typeCommand("copy-")

	h.press("down") // the first candidate, whichever name sorts first
	if !strings.Contains(h.view(), "› :copy-") {
		t.Fatalf("down did not highlight a candidate: %q", h.view())
	}

	h.press("enter")
	if h.mode() != "browse" {
		t.Errorf("running a picked candidate left the command line open: mode is %q", h.mode())
	}
}

// Until the user picks, nothing is highlighted: a default highlight would
// silently resolve the ambiguity that resolve() refuses to resolve.
func TestNothingIsPickedUntilTheUserPicks(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.typeCommand("copy-")

	if strings.Contains(h.view(), "› :") {
		t.Errorf("a candidate was highlighted before the user chose one: %q", h.view())
	}
}

// Typing after a pick invalidates it: the list underneath has changed, so the
// index no longer points at what was chosen.
func TestTypingClearsThePick(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.typeCommand("copy-")
	h.press("down")
	h.typeText("p")

	if strings.Contains(h.view(), "› :") {
		t.Errorf("the pick survived a keystroke that changed the list: %q", h.view())
	}
}

func TestACommandThatTakesNoArgumentRefusesOne(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.runCommand("reveal now")

	if h.m.secretsTab.State().Revealed != 0 {
		t.Error("the command ran despite the argument it does not take")
	}
	if !strings.Contains(h.m.banner(), "takes no argument") {
		t.Errorf("nothing said why: banner is %q", h.m.banner())
	}
}

func TestUnknownCommandReportsAndCloses(t *testing.T) {
	h := newHarness(t)
	h.runCommand("nope")

	if h.mode() != "browse" {
		t.Errorf("an unknown command left the line open: mode is %q", h.mode())
	}
	if !strings.Contains(h.m.banner(), "unknown command") {
		t.Errorf("the failure was not reported: banner is %q", h.m.banner())
	}
}

// An empty line closes without complaining: ":" then esc-by-enter is not a
// mistake worth a message.
func TestAnEmptyLineJustCloses(t *testing.T) {
	h := newHarness(t)
	h.runCommand("")

	if h.mode() != "browse" {
		t.Errorf("an empty line did not close: mode is %q", h.mode())
	}
	if h.m.banner() != "" {
		t.Errorf("an empty line complained: %q", h.m.banner())
	}
}

// The command line draws a rule the full width of the terminal, and it is the
// one width that reaches strings.Repeat without passing through a layout that
// has already floored it.
func TestTheCommandLineDrawsAtAnyWidth(t *testing.T) {
	for _, width := range []int{-5, 0, 1, 3, 40, 200} {
		h := newHarness(t)
		h.m.Update(tea.WindowSizeMsg{Width: width, Height: 20})
		h.press(":")
		h.typeText("co")
		h.view()
	}
}

// The arguments are what a keystroke cannot express, and the reason the line is
// parsed rather than matched.

func TestGotoWalksALazyPath(t *testing.T) {
	h := newHarness(t)
	h.runCommand("goto kv/app/prod/db")

	s := h.m.secretsTab.State()
	if s.CursorPath != "kv/app/prod/db" {
		t.Fatalf("goto did not land on the secret: cursor is on %q", s.CursorPath)
	}
	if !s.SecretLoaded {
		t.Error("landing on a secret should have read it")
	}
	if s.PendingGoto != "" {
		t.Errorf("the walk did not finish: %q is still pending", s.PendingGoto)
	}
}

// The walk has to open what it passes through, or it arrives on a row folded
// inside a closed parent and looks like nothing happened.
func TestGotoLeavesTheTrailOpen(t *testing.T) {
	h := newHarness(t)
	h.runCommand("goto kv/app/prod/db")

	var found bool
	for _, p := range h.visiblePaths() {
		if p == "kv/app/prod/db" {
			found = true
		}
	}
	if !found {
		t.Errorf("the target is not on screen: %v", h.visiblePaths())
	}
}

// A walk that cannot arrive has to say why and stop. Stopping matters as much:
// pendingGoto is what arriving listings match against, so a walk left pending
// would seize the next listing that happened to turn up.
func TestGotoReportsWhyItCouldNotArrive(t *testing.T) {
	tests := []struct {
		name string
		path string
		says string
	}{
		{"a path that is not there", "kv/app/prod/nope", ""},
		{"a mount that is not there", "nosuch/app", "nosuch"},
		// A denied subtree is information, not a failure, and the walk has to
		// stop on it rather than wait for a listing that will never come.
		{"a subtree the token may not list", "kv/restricted/root-key", "denied"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.runCommand("goto " + tt.path)

			if got := h.m.secretsTab.State().PendingGoto; got != "" {
				t.Errorf("the walk is still waiting on a listing: %q", got)
			}
			if !h.m.bannerIsError() {
				t.Errorf("the walk failed and said nothing: banner is %q", h.m.banner())
			}
			if tt.says != "" && !strings.Contains(h.m.banner(), tt.says) {
				t.Errorf("the banner does not name %q: %q", tt.says, h.m.banner())
			}
		})
	}
}

// the user's problem to solve first.
func TestGotoWorksFromAnotherTab(t *testing.T) {
	h := newHarness(t)
	h.runCommand("tab auth")
	h.runCommand("goto kv/app/prod/db")

	if h.m.tab != tabSecrets {
		t.Errorf("goto did not switch to the secrets tab: on %v", h.m.tab)
	}
	if got := h.m.secretsTab.State().CursorPath; got != "kv/app/prod/db" {
		t.Errorf("goto did not land: cursor is on %q", got)
	}
}

// The same for a policy name, which is the other address vivi understands.
func TestPolicyJumpsByNameFromAnywhere(t *testing.T) {
	h := newHarness(t)
	h.runCommand("policy app-admin")

	if h.m.tab != tabPolicies {
		t.Fatalf("policy did not switch tab: on %v", h.m.tab)
	}
	if got := h.m.policiesTab.Names()[h.m.policiesTab.Cursor()]; got != "app-admin" {
		t.Errorf("landed on %q, want app-admin", got)
	}
}

func TestTabCommandSwitchesTab(t *testing.T) {
	h := newHarness(t)
	h.runCommand("tab policies")

	if h.m.tab != tabPolicies {
		t.Errorf("the tab did not change: on %v", h.m.tab)
	}
}

func TestTabCommandNamesTheTabsItKnows(t *testing.T) {
	h := newHarness(t)
	h.runCommand("tab nope")

	if h.m.tab != tabSecrets {
		t.Errorf("an unknown tab name changed the tab anyway: on %v", h.m.tab)
	}
	if !strings.Contains(h.m.banner(), "secrets") {
		t.Errorf("the valid names were not offered: banner is %q", h.m.banner())
	}
}

func TestVersionCommandReadsThatVersion(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("V")
	h.runCommand("version 3")

	if got := h.m.secretsTab.State().SecretVersion; got != 3 {
		t.Errorf("read v%d, want v3", got)
	}
	if got := h.m.secretsTab.SecretValue("password"); got != "hunter2-v3" {
		t.Errorf("the pane is showing %v, not v3's data", got)
	}
}

// v1 of the fixture is destroyed and v2 soft-deleted: neither has data to show,
// and saying so beats an empty pane.
func TestVersionCommandRefusesAVersionWithNoData(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("V")
	before := h.m.secretsTab.State().SecretVersion

	h.runCommand("version 1")

	if now := h.m.secretsTab.State().SecretVersion; now != before {
		t.Errorf("a destroyed version was loaded anyway: now on v%d", now)
	}
	if !strings.Contains(h.m.banner(), "no data") {
		t.Errorf("nothing said why: banner is %q", h.m.banner())
	}
}

func TestFilterCommandFiltersTheTree(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.runCommand("filter cache")

	if got := h.m.secretsTab.State().Filter; got != "cache" {
		t.Errorf("the filter is %q, want cache", got)
	}
	for _, p := range h.visiblePaths() {
		if strings.HasSuffix(p, "/db") {
			t.Errorf("the filter did not narrow the tree: %v", h.visiblePaths())
			break
		}
	}
}

// Vault lets a directory and a secret share a name: a listing can answer
// ["app", "app/"]. Matching a walk on the name alone picks whichever the server
// sent first, which is how a jump to kv/app/prod/db lands inside secret app.
// Where the trailing slash lands the walk is the tree's rule, and is proved in
// the secrets package: the two it distinguishes are node kinds.

// candidateLine finds the rendered row a candidate is on, so an assertion can
// look at the whole row rather than the whole screen.
func candidateLine(view, name string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, name) {
			return line
		}
	}
	return ""
}

// A listing answering while the palette is open is the tab's, not the palette's.
func TestAnAnswerLandingUnderThePaletteReachesTheTab(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press(":")
	// The reload drops the tree and refetches while the palette is up.
	h.run(h.m.reloadCurrent())
	if h.mode() != "command" {
		t.Fatalf("the answer closed the palette: mode is %q", h.mode())
	}
	if len(h.visiblePaths()) == 0 {
		t.Error("the listing answered under the palette never reached the tree")
	}
}

// A generated copy name that is not showing now is refused, not unknown.
func TestAHiddenCopyCommandIsNotAvailableHere(t *testing.T) {
	h := newHarness(t)
	h.runCommand("copy-version-command")
	if !strings.Contains(h.m.banner(), "copy-version-command is not available here") {
		t.Errorf("banner is %q", h.m.banner())
	}
}

// A command that left its scope between enter and its run refuses to run.
func TestACommandOutOfScopeAtRunTimeRefuses(t *testing.T) {
	h := newHarness(t) // nothing selected: reveal has nothing to act on
	var reveal command
	for _, c := range commands {
		if c.name == "reveal" {
			reveal = c
		}
	}
	_, cmd := h.m.Update(commandMsg{c: reveal})
	h.run(cmd)
	if !strings.Contains(h.m.banner(), "reveal is not available here") {
		t.Errorf("an out-of-scope command ran or said nothing: banner is %q", h.m.banner())
	}
}
