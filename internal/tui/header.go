package tui

import (
	"strings"
	"time"

	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// The two lines above the panes: which server this is, and which tab is open.
// The one part of the screen that is the same on every tab, because what it
// answers is the same everywhere - "where am I, and how long have I got".

// renderHeader draws three lines: what this program is, which server answered,
// and how long the session has. Each is truncated and never wrapped, so the
// chrome stays the height layout.go assumes.
//
// Within a line the order is still priority: a truncated line loses its right
// end, so what the next keystroke depends on goes first.
func (m *Model) renderHeader() string {
	return strings.Join([]string{
		ui.Truncate(m.headerIdentity(), m.width),
		ui.Truncate(m.headerServer(), m.width),
		ui.Truncate(m.headerSession(), m.width),
		ui.Truncate(m.headerVaultVersion(), m.width),
	}, "\n")
}

// headerIdentity is the program, its version, and the states that change what
// every key below does. A session that cannot write, or a server that cannot
// answer, outranks anything else on screen.
func (m *Model) headerIdentity() string {
	parts := []string{m.st.Header.Render("vivi") + " " + m.st.HeaderDim.Render(m.viviVersion())}
	if !m.canWrite() {
		parts = append(parts, m.readOnlyBadge())
	}
	// A sealed Vault answers nothing, and a standby serves reads locally and can
	// be behind the leader. Both explain what is about to look wrong.
	if m.server.Sealed {
		parts = append(parts, m.st.BadgeDang.Render(" SEALED "))
	}
	if m.server.Standby {
		parts = append(parts, m.st.BadgeWarn.Render(" STANDBY "))
	}
	return strings.Join(parts, " ")
}

// headerServer is which Vault answered. The environment rides the address
// rather than the first line now, so the badge is never read apart from the
// host it is judging.
func (m *Model) headerServer() string {
	line := m.st.HeaderLbl.Render("URL:") + " " + m.st.HeaderDim.Render(m.server.Address) +
		" " + m.environmentBadge()
	// The namespace changes what every path below means, so it qualifies the
	// address on the same line.
	if ns := m.server.Namespace; ns != "" {
		line += " " + m.st.HeaderDim.Render("ns:"+ns)
	}
	return line
}

// headerSession is what runs out, and why it sits above the server version: the
// token is the one of the two that ends the session.
func (m *Model) headerSession() string {
	return m.st.HeaderLbl.Render("Token Expiry Time:") + " " + m.tokenTTLLabel()
}

// headerVaultVersion is last because it is the line nobody reads twice.
func (m *Model) headerVaultVersion() string {
	v := m.server.Version
	if v == "" {
		v = "unknown"
	} else {
		v = "v" + v
	}
	return m.st.HeaderLbl.Render("Vault Version:") + " " + m.st.HeaderDim.Render(v)
}

// viviVersion is what the build stamped in. A binary built rather than released
// carries no tag, and saying so beats printing a version that is not one.
func (m *Model) viviVersion() string {
	if m.version == "" || m.version == "dev" {
		return "(dev build)"
	}
	return "v" + strings.TrimPrefix(m.version, "v")
}

// environmentBadge names the kind of server being operated on. It exists so a
// destroy on production never happens because the terminal looked like the
// sandbox one next to it.
func (m *Model) environmentBadge() string {
	env := app.ClassifyEnvironment(m.server.Address)
	label := "(" + env.Label() + ")"

	switch env {
	case app.EnvLocal:
		return m.st.Badge.Render(label)
	case app.EnvDev:
		return m.st.BadgeWarn.Render(label)
	default:
		return m.st.BadgeDang.Render(label)
	}
}

// tokenTTLLabel shows how much session is left, and turns loud as it runs out.
func (m *Model) tokenTTLLabel() string {
	if m.token == nil {
		return m.st.HeaderDim.Render("no token")
	}
	if m.token.TTL <= 0 && m.token.ExpiresAt.IsZero() {
		return m.st.HeaderDim.Render("no expiry")
	}

	left := m.remainingTTL()
	if left <= 0 {
		// Nothing this app can do will work from here, and saying so beats a
		// timer sitting quietly at zero.
		return m.st.BadgeDang.Render(" token expired ")
	}

	label := "token " + ui.ShortDuration(left)
	switch app.ClassifyTTL(left) {
	case app.TTLCritical:
		return m.st.BadgeDang.Render(" " + label + " ")
	case app.TTLSoon:
		return m.st.Warn.Render(label)
	default:
		return m.st.HeaderDim.Render(label)
	}
}

// remainingTTL counts down from the expiry the server reported, instead of
// repeating the TTL captured at startup: a number that never moves is worse
// than no number, because it is believed.
func (m *Model) remainingTTL() time.Duration {
	if m.token == nil {
		return 0
	}
	if m.token.ExpiresAt.IsZero() {
		return m.token.TTL
	}
	if left := time.Until(m.token.ExpiresAt); left > 0 {
		return left
	}
	return 0
}

func (m *Model) renderTabs() string {
	cells := make([]string, 0, len(tabNames))
	for i, name := range tabNames {
		if tab(i) == m.tab {
			cells = append(cells, m.st.TabActive.Render(" "+name+" "))
		} else {
			cells = append(cells, m.st.TabIdle.Render(" "+name+" "))
		}
	}
	return ui.Truncate(strings.Join(cells, " "), m.width)
}
