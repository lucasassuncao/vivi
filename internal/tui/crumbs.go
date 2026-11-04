package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/bezel"
	"github.com/lucasassuncao/bezel/component/breadcrumb"
	"github.com/lucasassuncao/bezel/draw"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// titleLead is the cells before a pane's title on its top edge: the corner,
// one edge cell and the space the label opens with.
const titleLead = 3

// crumbParts is the secret path the detail's title names, one part per level,
// and false on any other tab or while the title names no path.
func (m *Model) crumbParts() ([]string, bool) {
	if m.tab != tabSecrets {
		return nil, false
	}
	path, ok := m.secretsTab.TitlePath()
	if !ok {
		return nil, false
	}
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	for i, p := range parts {
		parts[i] = draw.Sanitize(p)
	}
	return parts, true
}

// crumbs is the detail's title as a path, worn in the title's own style so a
// focused pane keeps its filled label, and cut from the middle to fit.
func (m *Model) crumbs(parts []string) breadcrumb.Model {
	th := m.st.Shell()
	title := th.Panel.Title
	if m.focus == ui.FocusDetail {
		title = th.PanelFocused.Title
	}
	width := max(1, m.sh.Rect(paneDetail).W-2*titleLead)
	return breadcrumb.New(parts).Blur().SetWidth(width).WithStyles(func(breadcrumb.Styles) breadcrumb.Styles {
		return breadcrumb.Styles{Part: title, Last: title.Bold(true), Cursor: title, Separator: title, More: title}
	})
}

// clickTitle goes to the folder of the detail's title under x, a cell of the
// top edge, as :goto would; the last part is what is shown already. false
// when x is on no part.
func (m *Model) clickTitle(x int) (tea.Cmd, bool) {
	parts, ok := m.crumbParts()
	if !ok {
		return nil, false
	}
	at := bezel.ClickMsg{MouseClickMsg: tea.MouseClickMsg{X: x - m.sh.Rect(paneDetail).X - titleLead, Button: tea.MouseLeft}}
	bc, r := m.crumbs(parts).Update(at)
	if !r.Handled {
		return nil, false
	}
	i := bc.Cursor()
	if i == len(parts)-1 {
		return nil, true
	}
	return m.secretsTab.Goto(strings.Join(parts[:i+1], "/") + "/"), true
}
