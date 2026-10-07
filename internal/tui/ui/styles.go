package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/lucasassuncao/bezel/theme"
)

// Styles is every style the interface draws with, resolved once per session.
// Passed to a component inside a Context rather than rebuilt by it: a component
// deciding its own colours is a component a theme cannot reach.
type Styles struct {
	// Rt is the bezel theme these styles derive from.
	Rt theme.Resolved

	Header     lipgloss.Style
	HeaderDim  lipgloss.Style
	HeaderLbl  lipgloss.Style
	TabActive  lipgloss.Style
	TabIdle    lipgloss.Style
	Badge      lipgloss.Style
	BadgeWarn  lipgloss.Style
	BadgeDang  lipgloss.Style
	LegendKey  lipgloss.Style
	LegendText lipgloss.Style
	HCLComment lipgloss.Style
	HCLString  lipgloss.Style
	HCLKeyword lipgloss.Style
	HCLPunct   lipgloss.Style
	HCLNumber  lipgloss.Style
	Cursor     lipgloss.Style
	Dir        lipgloss.Style
	Secret     lipgloss.Style
	Mount      lipgloss.Style
	Denied     lipgloss.Style
	Key        lipgloss.Style
	Masked     lipgloss.Style
	Section    lipgloss.Style
	GroupLabel lipgloss.Style
	Help       lipgloss.Style
	ErrBanner  lipgloss.Style
	OKBanner   lipgloss.Style
	Modal      lipgloss.Style
	ModalTitle lipgloss.Style
	Danger     lipgloss.Style
	Warn       lipgloss.Style
	Dim        lipgloss.Style
	Editing    lipgloss.Style
	Changed    lipgloss.Style
}

// headerLabel colours the header's field labels. Fixed rather than taken from the
// theme: a palette that recoloured them would have "URL:" competing with the
// address it introduces.
var headerLabel = lipgloss.Color("#008b8b")

func NewStyles(t theme.Theme, dark bool) Styles {
	rt := theme.Resolve(t, dark)
	c := lipgloss.Color
	p := struct{ Accent, Selection, Border, Dim, Text, OK, Warn, Danger, OnAccent color.Color }{
		c(rt.Colors.Accent), c(rt.Colors.Selection), c(rt.Colors.Border), c(rt.Colors.Dim), c(rt.Colors.Text),
		c(rt.Colors.Success), c(rt.Colors.Warning), c(rt.Colors.Danger), c(rt.Colors.OnAccent),
	}

	return Styles{
		Rt: rt,

		Header:    lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		HeaderDim: lipgloss.NewStyle().Foreground(p.Dim),
		HeaderLbl: lipgloss.NewStyle().Bold(true).Foreground(headerLabel),
		// A filled block reads as "selected" at a glance, where an underline has
		// to be looked for.
		TabActive: lipgloss.NewStyle().Bold(true).Foreground(p.OnAccent).Background(p.Accent),
		TabIdle:   lipgloss.NewStyle().Foreground(p.Dim),

		Badge:     lipgloss.NewStyle().Bold(true).Foreground(p.OnAccent).Background(p.OK),
		BadgeWarn: lipgloss.NewStyle().Bold(true).Foreground(p.OnAccent).Background(p.Warn),
		BadgeDang: lipgloss.NewStyle().Bold(true).Foreground(p.OnAccent).Background(p.Danger),

		LegendKey:  lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		LegendText: lipgloss.NewStyle().Foreground(p.Dim),

		HCLComment: lipgloss.NewStyle().Foreground(p.Dim).Italic(true),
		HCLString:  lipgloss.NewStyle().Foreground(p.OK),
		HCLKeyword: lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		HCLPunct:   lipgloss.NewStyle().Foreground(p.Dim),
		HCLNumber:  lipgloss.NewStyle().Foreground(p.Warn),

		Cursor: lipgloss.NewStyle().Bold(true).Foreground(p.Selection),
		Dir:    lipgloss.NewStyle().Foreground(p.Text).Bold(true),
		Secret: lipgloss.NewStyle().Foreground(p.Text),
		Mount:  lipgloss.NewStyle().Foreground(p.Accent).Bold(true),
		Denied: lipgloss.NewStyle().Foreground(p.Warn),
		Key:    lipgloss.NewStyle().Foreground(p.Dim),
		Masked: lipgloss.NewStyle().Foreground(p.Dim),
		// Bold because the rule that used to separate these blocks is gone: the
		// heading has to do that work on its own now.
		Section: lipgloss.NewStyle().Foreground(p.Accent).Bold(true),
		// Framing, not content: dim so a section title never competes with the
		// mounts underneath it, which carry the accent colour themselves.
		GroupLabel: lipgloss.NewStyle().Foreground(p.Dim).Bold(true),
		Help:       lipgloss.NewStyle().Foreground(p.Dim),
		ErrBanner:  lipgloss.NewStyle().Foreground(p.Danger).Bold(true),
		OKBanner:   lipgloss.NewStyle().Foreground(p.OK),
		Modal:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Accent).Padding(0, 2),
		ModalTitle: lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		Danger:     lipgloss.NewStyle().Foreground(p.Danger).Bold(true),
		Warn:       lipgloss.NewStyle().Foreground(p.Warn),
		Dim:        lipgloss.NewStyle().Foreground(p.Dim),
		Editing:    lipgloss.NewStyle().Foreground(p.Accent).Bold(true),
		Changed:    lipgloss.NewStyle().Foreground(p.Warn).Bold(true),
	}
}
