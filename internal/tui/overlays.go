package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/bezel/layout"
	"github.com/lucasassuncao/bezel/legend"
	"github.com/lucasassuncao/bezel/overlay"
)

// The shell's own panels, as overlays on its stack: the help panel and the
// token panel. They draw; the legend stays the screen's under them, and the
// keys stay with handleKey, which reads the model and closes them.

type helpOverlay struct{ m *Model }

func (o helpOverlay) Update(tea.Msg) (overlay.Overlay, tea.Cmd) { return o, nil }
func (o helpOverlay) View(body layout.Rect) string              { return o.m.renderHelp(body.H) }
func (helpOverlay) Legend() []legend.Entry                      { return nil }

type tokenOverlay struct{ m *Model }

func (o tokenOverlay) Update(tea.Msg) (overlay.Overlay, tea.Cmd) { return o, nil }
func (o tokenOverlay) View(body layout.Rect) string              { return o.m.renderToken(body.H) }
func (tokenOverlay) Legend() []legend.Entry                      { return nil }
