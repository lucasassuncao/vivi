package ui

import (
	"charm.land/lipgloss/v2"
	"github.com/lucasassuncao/bezel/draw"
	"github.com/lucasassuncao/bezel/legend"
	"github.com/lucasassuncao/bezel/theme"
)

// Shell is the bezel theme with vivi's chrome on top, so the shell's tab
// strip, status row, legend and modals wear the same colours the tabs do.
// The panels keep bezel's own style.
func (s Styles) Shell() theme.Resolved {
	r := s.Rt
	r.Chrome = draw.ChromeStyle{
		Title:     s.Header,
		Info:      s.HeaderDim,
		Tab:       s.TabIdle,
		TabActive: s.TabActive,
		Status:    lipgloss.NewStyle().Foreground(lipgloss.Color(s.Rt.Colors.Dim)),
		Dim:       s.Dim,
	}
	r.Legend = legend.Style{Key: s.LegendKey, Text: s.LegendText}
	r.StatusOK = s.OKBanner
	r.StatusErr = s.ErrBanner
	r.Modal = s.Modal
	// The item styles the shell's widgets draw lists with.
	r.Cursor = s.Cursor
	r.Dim = s.Dim
	r.Section = s.Section
	r.Danger = s.Danger
	return r
}
