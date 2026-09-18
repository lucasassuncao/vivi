package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/tui"
	"github.com/lucasassuncao/yedit/theme"
)

// ThemeEnvVar names the theme when no flag is given. vivi has no configuration
// file of its own, so an environment variable is how a preference persists,
// the same way the connection itself is configured.
const ThemeEnvVar = "VIVI_THEME"

// resolveTheme turns a theme name into a palette. An empty name is the built-in
// adaptive one, the only palette that reads correctly on both light and dark
// terminals: a named theme is fixed colours, and therefore a deliberate choice.
func resolveTheme(name string) (tui.Colors, error) {
	if name == "" {
		name = os.Getenv(ThemeEnvVar)
	}
	if name == "" {
		return tui.Colors{}, nil
	}

	selected, ok := theme.All()[strings.ToLower(name)]
	if !ok {
		return tui.Colors{}, fmt.Errorf("unknown theme %q; run \"vivi --list-themes\" to see the %d available",
			name, len(theme.All()))
	}

	return paletteOf(selected), nil
}

// paletteOf maps a yedit theme onto vivi's roles. yedit has no warning colour
// and vivi needs one - soft-deleted and destroyed must not look alike - so that
// role is left empty here for the built-in palette to fill.
func paletteOf(t theme.Theme) tui.Colors {
	c := theme.ResolveColors(t)
	return tui.Colors{
		Accent:    c.ActiveBorderColor,
		Selection: c.SelectionColor,
		Border:    c.InactiveBorderColor,
		Dim:       c.AvailableItemColor,
		Success:   c.ExistingItemColor,
		Danger:    c.ErrorColor,
	}
}

// themeNames lists what --theme accepts, grouped the way the registry groups
// them so the list reads as categories rather than as fifty-six words.
func themeNames() string {
	var b strings.Builder
	for _, category := range theme.Categories() {
		names := append([]string(nil), category.Themes...)
		slices.Sort(names)
		fmt.Fprintf(&b, "  %-14s %s\n", category.Name, strings.Join(names, ", "))
	}
	return b.String()
}

// ReadOnlyEnvVar names the read-only policy when no flag is given, since vivi
// has no config file. The preference most worth persisting: VIVI_READ_ONLY=prod
// in a profile is a decision made calmly, not with the cursor already on prod.
const ReadOnlyEnvVar = "VIVI_READ_ONLY"

// DebugEnvVar names a file to record keystrokes into. A TUI has no stdout to
// print to, so this is the only way to see what the terminal sent.
const DebugEnvVar = "VIVI_DEBUG"

// resolveReadOnly turns the flag, or the environment behind it, into a policy.
func resolveReadOnly(flag string) (app.ReadOnlyPolicy, error) {
	if flag == "" {
		flag = os.Getenv(ReadOnlyEnvVar)
	}
	return app.ParseReadOnly(flag)
}
