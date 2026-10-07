package cmd

import (
	"fmt"
	"os"

	"github.com/lucasassuncao/bezel/theme"

	"github.com/lucasassuncao/vivi/internal/app"
)

// ThemeEnvVar names the theme when no flag is given. vivi has no configuration
// file of its own, so an environment variable is how a preference persists,
// the same way the connection itself is configured.
const ThemeEnvVar = "VIVI_THEME"

// resolveTheme turns a theme name into a theme. An empty name is bezel's
// adaptive default, the only palette that reads correctly on both light and dark
// terminals: a named theme is fixed colours, and therefore a deliberate choice.
func resolveTheme(name string) (theme.Theme, error) {
	if name == "" {
		name = os.Getenv(ThemeEnvVar)
	}
	selected, err := theme.Lookup(name)
	if err != nil {
		return theme.Theme{}, fmt.Errorf("%w; run \"vivi --list-themes\" to see the %d available", err, len(theme.All()))
	}
	return selected, nil
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
