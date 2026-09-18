// Package ui is the render kit the interface is built out of: the palette, the
// panel furniture, the text measurements, and the types a tab and its parent
// both have to name. It knows how to draw, and nothing about what is drawn.
package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Colors is the palette vivi renders with: an ANSI index ("63") or a hex value
// ("#c39bff"). Strings and not lipgloss types so a theme can come from anywhere,
// even a package built against another lipgloss; empty falls back to built-in.
type Colors struct {
	// Accent is focused borders, section rules and legend keys.
	Accent string
	// Selection is the cursor and the focused panel's title.
	Selection string
	// Border is the unfocused panel edges.
	Border string
	// Dim is secondary text: labels, legend text, masked values.
	Dim string
	// Success is a current version, a healthy result, a LOCAL server.
	Success string
	// Warning is a soft-deleted version, a DEV server, a token running low.
	Warning string
	// Danger is a destroyed version, an error, a PROD server.
	Danger string
}

// The built-in palette, one colour for a light terminal and one for a dark one,
// so vivi is readable on both without being told which it is. A named theme
// replaces them with fixed colours, which is the point of choosing one.
//
// Which half is used is decided per session rather than per colour: lipgloss v2
// dropped AdaptiveColor because reading the background means querying the
// terminal, and that is bubbletea's to do. See Model.Init.
var (
	defAccent = adaptive{"#5f00af", "#c39bff"}
	defDim    = adaptive{"#6c6c6c", "#8a8a8a"}
	defText   = adaptive{"#1c1c1c", "#e4e4e4"}
	defDanger = adaptive{"#af0000", "#ff6b6b"}
	defWarn   = adaptive{"#af5f00", "#ffb86c"}
	defOK     = adaptive{"#005f00", "#87d787"}
	defBorder = adaptive{"#d0d0d0", "#4e4e4e"}

	// defOnAccent is the text drawn on a filled badge or tab, where the normal
	// foreground would vanish into the fill. It stays adaptive under every
	// theme: a theme picks the fills, and this has to stay legible on them.
	defOnAccent = adaptive{"#ffffff", "#1c1c1c"}

	// The header's field labels. Fixed rather than taken from the theme, the way
	// the warning amber is: a palette that recoloured them would have "URL:"
	// competing with the address it introduces.
	defHeaderLabel = lipgloss.Color("#008b8b")
)

// adaptive is a built-in colour before the terminal has been asked which half
// of it applies.
type adaptive struct{ light, dark string }

// Palette is the resolved set of colours the styles are built from.
type Palette struct {
	Accent    color.Color
	Selection color.Color
	Border    color.Color
	Dim       color.Color
	Text      color.Color
	OK        color.Color
	Warn      color.Color
	Danger    color.Color
	OnAccent  color.Color
}

// resolve turns a theme into a palette, filling anything it does not specify
// from the built-in one. isDark is what the terminal answered about its
// background, and only the built-in colours have two halves to choose between.
func resolve(c Colors, isDark bool) Palette {
	lightDark := lipgloss.LightDark(isDark)
	builtin := func(a adaptive) color.Color {
		return lightDark(lipgloss.Color(a.light), lipgloss.Color(a.dark))
	}
	pick := func(value string, fallback adaptive) color.Color {
		if value == "" {
			return builtin(fallback)
		}
		return lipgloss.Color(value)
	}

	// A theme that names no selection colour uses its accent for it, and only
	// falls back to the built-in one when it names neither.
	selection := c.Selection
	if selection == "" {
		selection = c.Accent
	}

	return Palette{
		Accent:    pick(c.Accent, defAccent),
		Selection: pick(selection, defAccent),
		Border:    pick(c.Border, defBorder),
		Dim:       pick(c.Dim, defDim),
		Text:      builtin(defText),
		OK:        pick(c.Success, defOK),
		Warn:      pick(c.Warning, defWarn),
		Danger:    pick(c.Danger, defDanger),
		OnAccent:  builtin(defOnAccent),
	}
}
