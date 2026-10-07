package ui

import (
	"github.com/lucasassuncao/bezel/layout"
	"github.com/lucasassuncao/bezel/overlay"
)

// The box a floating panel is drawn in is bezel's; these bind it to vivi's
// modal style and to the terminal width the model keeps.

// ModalInner is the width a modal's content gets when it takes the terminal.
func ModalInner(width int) int { return overlay.Inner(layout.Rect{W: width}) }

// ModalBox frames content and clips it to the terminal and the body height.
func (s Styles) ModalBox(lines []string, bodyHeight, termWidth int) string {
	return overlay.Box(lines, layout.Rect{W: termWidth, H: bodyHeight}, s.Modal)
}

// ModalBoxWidth is ModalBox at a width of the caller's choosing.
func (s Styles) ModalBoxWidth(lines []string, bodyHeight, inner, termWidth int) string {
	return overlay.BoxWidth(lines, layout.Rect{W: termWidth, H: bodyHeight}, inner, s.Modal)
}
