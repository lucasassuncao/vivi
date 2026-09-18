package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/secrets"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The Elm Architecture rests on two promises the Go compiler cannot make: view
// is pure, and update is total. Elm enforces both in the language; in Go they
// are conventions, and these are the two as tests.

// A view that writes to the model cannot be called twice for the same answer,
// and the runtime renders when it likes: a frame is a question about the state,
// never a step in it. The shallow copy catches an assignment to a field.
func TestViewDoesNotWriteToTheModel(t *testing.T) {
	// Every state the renderer branches on, since each takes a different path
	// through the panels and the legend.
	for _, open := range []struct {
		name string
		keys []string
	}{
		{"browsing", nil},
		{"detail focused", []string{"enter"}},
		{"version list", []string{"V"}},
		{"editor", []string{"e"}},
		{"command line", []string{":"}},
		{"help", []string{"?"}},
		{"token", []string{"i"}},
		{"filter", []string{"/"}},
		{"confirmation", []string{"d"}},
	} {
		h := newHarness(t)
		h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		h.open("kv/app/prod/db")
		h.press(open.keys...)

		before := snapshot(h.m)
		h.m.render()
		h.m.render() // twice: a renderer that settles on the second call is still writing

		if after := snapshot(h.m); after != before {
			t.Errorf("%s: rendering changed the model\nbefore %+v\nafter  %+v",
				open.name, before, after)
		}
	}
}

// modelState is every field of the model a renderer could plausibly reach, in a
// form that can be compared. The Model itself cannot be: it carries lipgloss
// styles, which hold maps.
type modelState struct {
	mode                       mode
	tab                        tab
	focus                      ui.Focus
	width, height              int
	secrets                    secrets.State
	policyCursor, policyCount  int
	policyHCL                  string
	methodCursor, roleCursor   int
	onRoles                    bool
	activityCursor, activities int
	pending                    int
	banner                     string
	bannerErr                  bool
	bannerID                   int
	pendingPolicy              string
	detailY, detailW, detailH  int
}

func snapshot(m *Model) modelState {
	return modelState{
		mode: m.mode, tab: m.tab, focus: m.focus,
		width: m.width, height: m.height,
		// Every tab reports itself. The Secrets one has enough state to need a
		// struct of its own, and it is comparable for the same reason this is.
		secrets:      m.secretsTab.State(),
		policyCursor: m.policiesTab.Cursor(), policyCount: len(m.policiesTab.Names()),
		policyHCL:    m.policiesTab.Doc(),
		methodCursor: m.authTab.MethodCursor(), roleCursor: m.authTab.RoleCursor(),
		onRoles:        m.authTab.OnRoles(),
		activityCursor: m.activityTab.Cursor(), activities: len(m.activityTab.Entries()),
		pending: m.pending,
		banner:  m.banner, bannerErr: m.bannerErr, bannerID: m.bannerID,
		pendingPolicy: m.policiesTab.Pending(),
		detailY:       m.detail.YOffset(), detailW: m.detail.Width(), detailH: m.detail.Height(),
	}
}

// The nil mode is the specific case this used to fail on: reading it repaired
// it, so the first frame after a bad transition wrote to the model.
func TestRenderingAMissingModeDoesNotRepairIt(t *testing.T) {
	h := newHarness(t)
	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	h.open("kv/app/prod/db")

	h.m.mode = nil
	h.m.View()

	if h.m.mode != nil {
		t.Error("the view repaired the mode, so it wrote to the model")
	}
	// Update is where the repair belongs, and it has to happen before the
	// handler that would otherwise dereference it.
	h.m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if h.m.mode == nil {
		t.Error("update did not restore the mode")
	}
}

// Elm's update is total because Msg is a closed union; here it closes only by
// convention, so a new message can fall through the whole dispatch chain. The
// failure is quiet: the answer returns and nothing happens.
func TestEveryMessageHasABranch(t *testing.T) {
	fset := token.NewFileSet()
	files := parseNonTestFiles(t, fset)

	declared := map[string]token.Position{}
	handled := map[string]bool{}

	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.TypeSpec:
				// The naming convention is the union: a type ending in Msg is a
				// message, and every message is a type ending in Msg.
				if strings.HasSuffix(node.Name.Name, "Msg") {
					declared[node.Name.Name] = fset.Position(node.Pos())
				}
			case *ast.TypeSwitchStmt:
				for _, name := range switchedTypes(node) {
					handled[name] = true
				}
			case *ast.TypeAssertExpr:
				// The single-type form, which is how requestAnswer and the
				// commanding mode are tested.
				if ident, ok := node.Type.(*ast.Ident); ok {
					handled[ident.Name] = true
				}
			}
			return true
		})
	}

	if len(declared) == 0 {
		t.Fatal("no message types found, so this test proves nothing")
	}
	for name, pos := range declared {
		if !handled[name] {
			t.Errorf("%s is declared at %s but no branch in the update loop matches it: "+
				"a message with no case falls through to the nested inputs and is lost", name, pos)
		}
	}
}

// The other half of the same promise: a branch that matches a message type no
// longer declared is a case that can never run, which reads as coverage the
// loop does not have.
func TestEveryBranchMatchesAMessageThatExists(t *testing.T) {
	fset := token.NewFileSet()
	files := parseNonTestFiles(t, fset)

	declared := map[string]bool{}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok && strings.HasSuffix(ts.Name.Name, "Msg") {
				declared[ts.Name.Name] = true
			}
			return true
		})
	}

	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			sw, ok := n.(*ast.TypeSwitchStmt)
			if !ok {
				return true
			}
			for _, name := range switchedTypes(sw) {
				// Only the package own Msg names: tea.KeyMsg and the
				// component messages are selector expressions, not idents.
				if !strings.HasSuffix(name, "Msg") || declared[name] {
					continue
				}
				t.Errorf("%s: a branch matches %s, which is not a declared message",
					fset.Position(sw.Pos()), name)
			}
			return true
		})
	}
}

func parseNonTestFiles(t *testing.T, fset *token.FileSet) []*ast.File {
	t.Helper()

	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}

	var files []*ast.File
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name) // #nosec G304 -- the package's own sources
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, f)
	}
	return files
}

// switchedTypes are the bare type names a type switch matches on. Selector
// expressions - tea.KeyMsg, tea.WindowSizeMsg - belong to a nested component
// and are not part of this package's union.
func switchedTypes(sw *ast.TypeSwitchStmt) []string {
	var out []string
	for _, clause := range sw.Body.List {
		cc, ok := clause.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, expr := range cc.List {
			if ident, ok := expr.(*ast.Ident); ok {
				out = append(out, ident.Name)
			}
		}
	}
	return out
}
