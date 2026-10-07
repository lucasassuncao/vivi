package tui

import (
	"os"
	"testing"
	"time"

	"github.com/lucasassuncao/bezel/draw"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// TestMain shrinks every clock the app runs on, once, before any test does: a
// timer does not answer inside the harness's wait, and was abandoned silently
// 1102 times a run. Here, not in newHarness, or parallel tests would race.
func TestMain(m *testing.M) {
	ui.BlinkSpeed = time.Millisecond
	draw.BlinkSpeed = time.Millisecond // bezel's own inputs: the palette's
	bannerLife = 10 * time.Millisecond
	clockInterval = time.Millisecond

	os.Exit(m.Run())
}
