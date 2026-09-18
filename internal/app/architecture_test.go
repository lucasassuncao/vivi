package app_test

import (
	"go/build"
	"strings"
	"testing"
)

const module = "github.com/lucasassuncao/vivi"

// A layering rule in a README lasts until someone needs one function from the
// other side. This is the rule as a test: app is the inside, and the inside
// never reaches out.
func TestAppNeverImportsAnAdapter(t *testing.T) {
	forbidden := []string{
		// The driving adapter. If a rule needs a tree, a cursor or a footer, it
		// is not a rule - it is a rendering decision in the wrong package.
		module + "/internal/tui",

		// The terminal libraries, by name, because a rule that returns a styled
		// string has quietly become presentation even if it never says "tui".
		// Two spellings for the charm ones: v2 moved them to a vanity domain,
		// and a list naming only the old one would have stopped guarding.
		"charm.land",
		"github.com/charmbracelet",
		"github.com/muesli",
		"github.com/atotto/clipboard",
	}

	for _, imp := range importsOf(t, module+"/internal/app") {
		for _, bad := range forbidden {
			if strings.HasPrefix(imp, bad) {
				t.Errorf("app imports %s: the inside of the hexagon reached out to an adapter", imp)
			}
		}
	}
}

// The other direction must stay open: a tui that has stopped importing app
// has started re-deciding the rules itself.
func TestTheInterfaceAsksAppForItsRules(t *testing.T) {
	var found bool
	for _, imp := range importsOf(t, module+"/internal/tui") {
		if imp == module+"/internal/app" {
			found = true
		}
	}
	if !found {
		t.Error("tui no longer imports app, so its rules live somewhere else again")
	}
}

// tabs are the interface's components, one package each. They talk to their
// parent and to the render kit, and to each other not at all.
var tabs = []string{"activity", "auth", "policies", "secrets"}

// A tab importing a tab is the coupling the decomposition exists to remove: the
// Auth tab reaching a policy does it by message, through the parent, precisely
// so that neither has to know the other's package.
func TestNoTabImportsAnotherTab(t *testing.T) {
	for _, tab := range tabs {
		for _, imp := range importsOf(t, module+"/internal/tui/"+tab) {
			for _, other := range tabs {
				if other != tab && imp == module+"/internal/tui/"+other {
					t.Errorf("tab %s imports tab %s: they are talking behind the parent's back", tab, other)
				}
			}
		}
	}
}

// A tab never imports its parent either, which is what makes the parent free to
// import every tab. The one direction is the whole reason ui exists: anything
// both sides have to name lives there.
func TestNoTabImportsTheParent(t *testing.T) {
	for _, tab := range tabs {
		for _, imp := range importsOf(t, module+"/internal/tui/"+tab) {
			if imp == module+"/internal/tui" {
				t.Errorf("tab %s imports its parent: that is an import cycle waiting for the parent to use it", tab)
			}
		}
	}
}

// The render kit is shared and stays shared. A ui that imported a component
// would be the god object again, wearing the name of the thing that replaced it.
func TestTheRenderKitImportsNoComponent(t *testing.T) {
	for _, imp := range importsOf(t, module+"/internal/tui/ui") {
		if strings.HasPrefix(imp, module+"/internal/tui/") || imp == module+"/internal/tui" {
			t.Errorf("ui imports %s: the shared kit has started depending on what shares it", imp)
		}
	}
}

func importsOf(t *testing.T, path string) []string {
	t.Helper()

	pkg, err := build.Import(path, "", 0)
	if err != nil {
		t.Fatalf("import %s: %v", path, err)
	}
	// The package's own tests count too: a rule tested through a Bubbletea
	// harness has a Bubbletea dependency.
	return append(append([]string{}, pkg.Imports...), pkg.TestImports...)
}
