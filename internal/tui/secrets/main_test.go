package secrets

import (
	"os"
	"testing"
	"time"

	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// TestMain shrinks the caret's clock before any test runs: the harness waits
// out every command, and each keystroke into an input arms a 530ms blink.
func TestMain(m *testing.M) {
	ui.BlinkSpeed = time.Millisecond
	os.Exit(m.Run())
}
