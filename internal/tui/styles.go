package tui

import "github.com/lucasassuncao/vivi/internal/tui/ui"

// Colors is re-exported because a theme is chosen outside this package, by cmd,
// and a caller picking a palette should not have to know which package draws
// with it. The palette and every style built from it live in ui.
type Colors = ui.Colors
