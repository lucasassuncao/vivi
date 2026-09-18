package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The token panel, which opens over any tab. The one modal with an action of
// its own, because it is the only place that knows whether the token can be
// renewed at all - see the "n" arm in handleKey.

// openToken shows the panel and refreshes what it reports, because a TTL read
// at startup is the one number in it guaranteed to be wrong by now.
func (m *Model) openToken() tea.Cmd {
	m.mode = showingToken{}
	return m.reloadToken()
}

// renderToken answers "with which badge am I operating", which is the only way
// to know when authentication comes from the environment.
func (m *Model) renderToken(bodyHeight int) string {
	var b strings.Builder
	b.WriteString(m.st.ModalTitle.Render("Token"))
	b.WriteString("\n\n")

	if m.token == nil {
		b.WriteString(m.st.ErrBanner.Render("could not look up the token"))
		return m.st.ModalBox(strings.Split(b.String(), "\n"), bodyHeight, m.width)
	}

	// The name, the ids and the policy names below are all chosen by whoever
	// configured the auth method, so none of them is drawn as it arrived.
	rows := [][2]string{
		{"server", ui.Sanitize(m.server.Address)},
		{"namespace", ui.OrDash(ui.Sanitize(m.server.Namespace))},
		{"display name", ui.OrDash(ui.Sanitize(m.token.DisplayName))},
		{"accessor", ui.OrDash(ui.Sanitize(m.token.Accessor))},
		{"entity", ui.OrDash(ui.Sanitize(m.token.EntityID))},
		{"ttl", ui.ShortDuration(m.token.TTL)},
		{"renewable", fmt.Sprint(m.token.Renewable)},
	}
	for _, r := range rows {
		fmt.Fprintf(&b, "  %-14s %s\n", r[0], r[1])
	}

	b.WriteString("\n  policies\n")
	if len(m.token.Policies) == 0 {
		b.WriteString(m.st.Dim.Render("    (none)\n"))
	}
	for _, p := range m.token.Policies {
		b.WriteString("    ")
		b.WriteString(ui.Sanitize(p))
		b.WriteString("\n")
	}

	// The offer lives here because this panel is the only place that knows
	// whether the token can be renewed at all. Advertising the key anywhere
	// else would mean advertising it blind.
	if m.token.Renewable {
		b.WriteString("\n")
		b.WriteString(m.st.Dim.Render("  n  renew this token"))
	}

	b.WriteString("\n")
	b.WriteString(m.st.Dim.Render("esc closes"))
	return m.st.ModalBox(strings.Split(b.String(), "\n"), bodyHeight, m.width)
}

func (m *Model) reloadToken() tea.Cmd {
	return m.loadToken()
}
