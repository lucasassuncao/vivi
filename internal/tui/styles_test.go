package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/vault/fake"
)

// A theme has to reach the screen, and the roles it does not define have to
// keep their built-in colours rather than collapsing into the ones it does.
// No colour profile to force: v2 renders the escapes whatever it draws into.
func TestThemeColoursReachTheRender(t *testing.T) {
	srv := fake.New()
	token, err := srv.TokenInfo(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	plain := New(Options{Client: srv, Token: token})
	themed := New(Options{Client: srv, Token: token, Colors: Colors{
		Accent: "#ff00ff", Danger: "#00ff00",
	}})
	for _, m := range []*Model{plain, themed} {
		m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	}

	if plain.render() == themed.render() {
		t.Fatal("a theme should change what is drawn")
	}
	if !strings.Contains(themed.render(), "255;0;255") {
		t.Fatal("the theme's accent should appear in the rendered escapes")
	}

	// Warning is the role no theme supplies, so it must not have been dragged
	// to the danger colour: deleted and destroyed have to stay distinguishable.
	if themed.st.Pal.Warn == themed.st.Pal.Danger {
		t.Fatal("an unset warning colour must not fall back to danger")
	}
}
