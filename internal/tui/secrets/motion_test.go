package secrets

import (
	"strings"
	"testing"
	"time"

	"github.com/lucasassuncao/bezel/animation/text"
	"github.com/lucasassuncao/bezel/bezeltest"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// settle delivers frames to everything scrambling, first at start and then
// a second later, which lands every one of them.
func (h *harness) settle() {
	start := time.Now()
	for _, at := range []time.Time{start, start.Add(time.Second)} {
		for _, s := range h.m.scrambles {
			h.m.Data(s.FrameAt(at))
		}
	}
}

// A revealed value is decoded on screen before it reads as itself.
func TestARevealedValueScramblesThenSettles(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	h.m.Key(bezeltest.Key("r"), h.ctx)
	keys := h.m.sortedKeys()
	if len(h.m.scrambles) != len(keys) {
		t.Fatalf("revealing the secret scrambled %d of %d fields", len(h.m.scrambles), len(keys))
	}
	h.settle()
	if len(h.m.scrambles) != 0 {
		t.Errorf("%d fields are still scrambling after a second", len(h.m.scrambles))
	}
	if want := h.secretData("app/prod/db")["host"].(string); !strings.Contains(h.view(), want) {
		t.Errorf("the settled value %q is not on screen:\n%s", want, h.view())
	}
}

// With motion off the value is there at once, as it always was.
func TestAStillRevealShowsTheValueAtOnce(t *testing.T) {
	t.Setenv("BEZEL_NO_MOTION", "1")
	h := newHarness(t)
	h.open("kv/app/prod/db")

	h.press("r")
	if len(h.m.scrambles) != 0 {
		t.Error("a still reveal scrambled")
	}
	if want := h.secretData("app/prod/db")["host"].(string); !strings.Contains(h.view(), want) {
		t.Errorf("the value %q is not on screen:\n%s", want, h.view())
	}
}

// A typed confirmation refuses a wrong path with a shake, and stays open.
func TestAWrongPathShakesTheConfirmation(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")
	h.press("M")
	if h.modal() == nil || !h.modal().needsInput() {
		t.Fatal("precondition: M asks for the path to be typed")
	}
	h.typeText("kv/app/prod/nope")

	h.m.Key(bezeltest.Key("enter"), h.ctx)
	if !h.m.shake.Active() {
		t.Error("a wrong path did not shake the confirmation")
	}
	if h.modal() == nil {
		t.Error("a wrong path closed the confirmation")
	}
}

// A version that goes from live to deleted or destroyed is struck through;
// one that was already gone, or another secret's, is not struck again.
func TestAVersionThatGoesIsStruckThrough(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	was := append([]vault.VersionInfo(nil), h.m.versions...)
	current := len(h.m.versions) - 1
	for i, v := range h.m.versions {
		if v.Version == h.m.meta.CurrentVersion {
			current = i
		}
	}
	h.m.versions[current].DeletionTime = time.Now()

	h.m.strikeGone(was)
	if len(h.m.strikes) != 1 || !h.m.strikes[h.m.meta.CurrentVersion].Active() {
		t.Errorf("want only the deleted version struck, got %d strikes", len(h.m.strikes))
	}
}

// Gone versions stay crossed out, with the state beside them still saying which.
func TestGoneVersionsStayStruck(t *testing.T) {
	h := newHarness(t)
	h.open("kv/app/prod/db")

	pane := h.m.renderVersions(100)
	for _, v := range h.m.versions {
		struck := text.NewStrike(versionLabel(v), 0).WithInk(h.m.st.Dim.GetForeground()).View()
		if gone(v) != strings.Contains(pane, struck) {
			t.Errorf("v%d (gone %v) is struck %v", v.Version, gone(v), !gone(v))
		}
	}
	if plain := stripANSI(pane); !strings.Contains(plain, "✗ destroyed") || !strings.Contains(plain, "⌫ deleted") {
		t.Errorf("the states beside the struck rows are gone:\n%s", plain)
	}
}
