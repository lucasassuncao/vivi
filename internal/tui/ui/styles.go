package ui

import "charm.land/lipgloss/v2"

// Styles is every style the interface draws with, resolved once per session.
// Passed to a component inside a Context rather than rebuilt by it: a component
// deciding its own colours is a component a theme cannot reach.
type Styles struct {
	Pal Palette

	Header     lipgloss.Style
	HeaderDim  lipgloss.Style
	HeaderLbl  lipgloss.Style
	TabActive  lipgloss.Style
	TabIdle    lipgloss.Style
	PaneTitle  lipgloss.Style
	TitleIdle  lipgloss.Style
	BorderOn   lipgloss.Style
	BorderOff  lipgloss.Style
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

func NewStyles(c Colors, isDark bool) Styles {
	p := resolve(c, isDark)

	return Styles{
		Pal: p,

		Header:    lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		HeaderDim: lipgloss.NewStyle().Foreground(p.Dim),
		HeaderLbl: lipgloss.NewStyle().Bold(true).Foreground(defHeaderLabel),
		// A filled block reads as "selected" at a glance, where an underline has
		// to be looked for.
		TabActive: lipgloss.NewStyle().Bold(true).Foreground(p.OnAccent).Background(p.Accent),
		TabIdle:   lipgloss.NewStyle().Foreground(p.Dim),

		// Panel titles carry the focus signal as well as the border does, which
		// keeps the interface readable without colour.
		PaneTitle: lipgloss.NewStyle().Bold(true).Foreground(p.OnAccent).Background(p.Selection),
		TitleIdle: lipgloss.NewStyle().Foreground(p.Dim),
		BorderOn:  lipgloss.NewStyle().Foreground(p.Accent),
		BorderOff: lipgloss.NewStyle().Foreground(p.Border),

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
