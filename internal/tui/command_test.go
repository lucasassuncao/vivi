package tui

import (
	"strings"
	"testing"
)

// The table is the single source of truth for what vivi can be asked to do, so
// a duplicated or unnamed entry is a bug in the one place nothing else checks.
func TestCommandTableIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range commands {
		switch {
		case c.name == "":
			t.Errorf("a command has no name")
		case seen[c.name]:
			t.Errorf("%q is in the table twice", c.name)
		case c.title == "":
			t.Errorf("%q has no title, so the palette would list a blank row", c.name)
		case c.scope == nil:
			t.Errorf("%q has no scope, so availability cannot be decided", c.name)
		case c.run == nil:
			t.Errorf("%q has nothing to run", c.name)
		}
		seen[c.name] = true
	}
}

// The palette is read by name, and the generated copies come from a panel that
// changes with the cursor: sorted, where an entry is written and where it is
// drawn stop being the same question.
func TestThePaletteIsAlphabetical(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	avail := h.m.available()
	if len(avail) < 2 {
		t.Fatalf("a loaded secret should offer several commands, got %d", len(avail))
	}
	for i := 1; i < len(avail); i++ {
		if avail[i-1].name > avail[i].name {
			t.Errorf("%q is listed before %q", avail[i-1].name, avail[i].name)
		}
	}
}

// A command with no key is reachable from the palette, and only from it: the
// "?" panel lists keys alone.
func TestEveryCommandIsReachable(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	for _, c := range commands {
		if c.key != "" || !c.scope(h.m) {
			continue
		}
		// Typed by name: the palette shows only a window of its candidates.
		h.typeCommand(c.name)
		if !strings.Contains(h.view(), c.label()) {
			t.Errorf("%q has no key and the palette does not list it: it is unreachable", c.name)
		}
		h.press("esc")
	}

	h.press("?")
	if strings.Contains(h.view(), ":goto <path>") {
		t.Error("the help panel lists a keyless command; it belongs to the palette alone")
	}
}

// The resolution order is the whole reason ":copy" is usable at all: it is a
// prefix of two other commands, and without the exact-match rule it could never
// be run.
func TestResolutionPrefersExactOverPrefix(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.runCommand("copy")

	if strings.Contains(h.m.banner(), "ambiguous") {
		t.Fatalf(":copy was ambiguous: %q", h.m.banner())
	}
	if got := h.copied(); got == "" || got == "kv/app/prod/db" {
		t.Errorf("exact name lost to a longer one: copied %q", got)
	}
}

func TestResolutionAcceptsAUniquePrefix(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.runCommand("rev")

	if len(h.m.secretsTab.RevealedKeys()) == 0 {
		t.Errorf("rev should run reveal: banner is %q", h.m.banner())
	}
}

// An ambiguous prefix has to run nothing. Picking by table order would make
// ":d" a coin toss between delete and diff, one of which is destructive.
func TestResolutionRefusesAnAmbiguousPrefix(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.runCommand("cop")

	if h.mode() != "command" {
		t.Fatalf("an ambiguous prefix ran something: mode is %q", h.mode())
	}
	if !strings.Contains(h.view(), "copy-path") {
		t.Errorf("the candidates should be named: %q", h.view())
	}
}

// Naming a command that exists but cannot run here is a different mistake from
// naming one that does not exist, and only one of the two is a typo.
func TestUnavailableAndUnknownAreDifferentErrors(t *testing.T) {
	h := newHarness(t)

	// Nothing is selected yet, so reveal has nothing to act on.
	h.runCommand("reveal")
	if !strings.Contains(h.m.banner(), "reveal is not available here") {
		t.Errorf("a real command out of scope should be unavailable: %q", h.m.banner())
	}

	h.runCommand("revealx")
	if !strings.Contains(h.m.banner(), "unknown command: revealx") {
		t.Errorf("a name that is in no table should be unknown: %q", h.m.banner())
	}
}

// The command and the keystroke must be the same action, not two that agree.
func TestCommandAndKeystrokeDoTheSameThing(t *testing.T) {
	byKey := newHarness(t)
	byKey.open("kv/app/prod/db")
	byKey.press("r")

	byName := newHarness(t)
	byName.open("kv/app/prod/db")
	byName.runCommand("reveal")

	pressed := byKey.m.secretsTab.RevealedKeys()
	named := byName.m.secretsTab.RevealedKeys()

	if len(pressed) == 0 {
		t.Fatal("pressing r revealed nothing, so the comparison is meaningless")
	}
	if len(named) != len(pressed) {
		t.Errorf(":reveal revealed %d fields, r revealed %d", len(named), len(pressed))
	}
	shown := map[string]bool{}
	for _, k := range named {
		shown[k] = true
	}
	for _, k := range pressed {
		if !shown[k] {
			t.Errorf(":reveal did not reveal %q, which r did", k)
		}
	}
}

// Scope is judged against the mode the command line was opened from. Judging it
// against the live mode would make every scope false the moment it is asked,
// because the live mode is the command line itself.
func TestScopeIsJudgedAgainstTheModeUnderneath(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("V")
	h.press(":")

	var found bool
	for _, c := range h.m.available() {
		if c.name == "destroy" {
			found = true
		}
		if c.name == "reveal" {
			t.Errorf("reveal is a browsing command and should not be offered over the version list")
		}
	}
	if !found {
		t.Error("destroy should be available from the version list")
	}
}

// Scope review. The rule is in command.go: offer what can run or explain, hide
// what is meaningless. Each of these was a real mismatch between a scope and the
// guard inside the action it names.

// ":delete" asked whether anything was selected; confirmDeleteCurrent asks
// whether a secret is. On a folder the command was offered and did nothing,
// which taught that "d" does nothing too.
func TestDeleteIsNotOfferedOnAFolder(t *testing.T) {
	h := newHarness(t)
	h.focus("kv/")

	for _, c := range h.m.available() {
		if c.name == "delete" {
			t.Error("delete is offered on a folder, where it cannot run")
		}
	}

	h.open("kv/app/prod/db")
	if !offers(h, "delete") {
		t.Error("delete should be offered on a secret")
	}
}

// Creating a sibling is the common case, and the cursor is on the first secret
// exactly when you want the second.
func TestCreateIsOfferedBesideASecret(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	if !offers(h, "create") {
		t.Fatal("create should be offered with the cursor on a secret")
	}
	h.runCommand("create")
	if h.mode() != "confirm" {
		t.Fatalf("create did not open the name prompt: mode is %q", h.mode())
	}
	if got := h.modalTitle(); !strings.Contains(got, "kv/app/prod") {
		t.Errorf("the new secret goes in the parent folder, not the secret: %q", got)
	}
}

// The instruction is the point: a command that vanishes until two versions are
// marked cannot tell anyone that marking exists.
func TestDiffStaysOfferedSoItCanSayWhatIsMissing(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("V")

	if !offers(h, "diff") {
		t.Fatal("diff should be offered on the version list even with nothing marked")
	}
	h.runCommand("diff")
	if !strings.Contains(h.m.banner(), "two versions") {
		t.Errorf("diff did not say what was missing: banner is %q", h.m.banner())
	}
}

// Every scope must be reachable: one that is false everywhere is a command
// nobody can run, and the table is the only place that would say so.
func TestEveryCommandIsAvailableSomewhere(t *testing.T) {
	reachable := map[string]bool{}

	note := func(h *harness) {
		for _, c := range h.m.available() {
			reachable[c.name] = true
		}
	}

	secrets := newHarness(t)
	secrets.open("kv/app/prod/db")
	note(secrets)

	// The detail pane is its own context: the cursor is on a field there, and
	// the keys that act on one are only offered where one is selected.
	pane := newHarness(t)
	pane.open("kv/app/prod/db")
	pane.press("enter")
	note(pane)

	versions := newHarness(t)
	versions.open("kv/app/prod/db")
	versions.press("V")
	note(versions)
	versions.press("down", "down") // v2, which is soft-deleted
	note(versions)

	policies := newHarness(t)
	policies.runCommand("tab policies")
	note(policies)

	auth := newHarness(t)
	auth.runCommand("tab auth")
	auth.press("right")
	note(auth)

	for _, c := range commands {
		if !reachable[c.name] {
			t.Errorf("%q is in the table but available in no context", c.name)
		}
	}
}

// The version list is Secrets-tab state, and the jumps run from inside it. A
// jump that left the mode armed put D, M, b and u on a screen showing a policy,
// still pointed at the secret that scrolled away with the tab.
func TestJumpingToAnotherTabLeavesTheVersionList(t *testing.T) {
	jumps := map[string]string{
		"tab":    "tab policies",
		"policy": "policy app-prod-db",
	}

	for name, line := range jumps {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.open("kv/app/prod/db")
			h.press("enter")
			h.press("V")
			if h.mode() != "versions" {
				t.Fatalf("V should open the version list, mode=%s", h.mode())
			}

			h.runCommand(line)

			if h.mode() != "browse" {
				t.Fatalf("the version list outlived the jump, mode=%s", h.mode())
			}
		})
	}
}

// The palette lists exactly what the scopes allow, judged on the real panel
// rather than on a copy of its filter.
func TestThePaletteAgreesWithTheScopes(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	for _, c := range commands {
		h.typeCommand(c.name)
		// A candidate row starts with two spaces; the typed line does not.
		listed := strings.Contains(h.view(), "  "+c.label()+" ")
		if want := c.scope(h.m); listed != want {
			t.Errorf("%q: listed %v, scope says %v", c.name, listed, want)
		}
		h.press("esc")
	}
}
