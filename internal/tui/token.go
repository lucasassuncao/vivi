package tui

import (
	"fmt"
	"github.com/lucasassuncao/bezel/draw"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// The token panel, which opens over any tab. The one modal with an action of
// its own, because it is the only place that knows whether the token can be
// renewed at all - see the "n" arm in handleKey.

// openToken shows the panel and refreshes what it reports, because a TTL read
// at startup is the one number in it guaranteed to be wrong by now.
func (m *Model) openToken() tea.Cmd {
	m.closeOverlays()
	m.sh = m.sh.Push(tokenOverlay{m})
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
		{"server", draw.Sanitize(m.server.Address)},
		{"namespace", orDash(draw.Sanitize(m.server.Namespace))},
		{"display name", orDash(draw.Sanitize(m.token.DisplayName))},
		{"accessor", orDash(draw.Sanitize(m.token.Accessor))},
		{"entity", orDash(draw.Sanitize(m.token.EntityID))},
		{"ttl", shortDuration(m.token.TTL)},
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
		b.WriteString(draw.Sanitize(p))
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

// orDash is s, or a dash when it is empty.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// shortDuration renders a lifetime the way a header has room for: the two
// largest units that say anything, truncated rather than rounded, so a
// countdown never shows more time than is left.
func shortDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	switch {
	case d >= 24*time.Hour:
		days := d / (24 * time.Hour)
		if h := (d % (24 * time.Hour)) / time.Hour; h > 0 {
			return fmt.Sprintf("%dd%dh", days, h)
		}
		return fmt.Sprintf("%dd", days)
	case d >= time.Hour:
		hours := d / time.Hour
		if mins := (d % time.Hour) / time.Minute; mins > 0 {
			return fmt.Sprintf("%dh%dm", hours, mins)
		}
		return fmt.Sprintf("%dh", hours)
	case d >= time.Minute:
		return fmt.Sprintf("%dm", d/time.Minute)
	default:
		return fmt.Sprintf("%ds", d/time.Second)
	}
}
