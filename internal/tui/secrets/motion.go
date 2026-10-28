package secrets

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/bezel/animation/text"
	"github.com/lucasassuncao/bezel/draw"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// The tab's motion, each piece saying what just happened where it happened: a
// value decoded as it is revealed, a typed path refused with a shake, a version
// struck through as it is deleted or destroyed. BEZEL_NO_MOTION skips them all.

const (
	scrambleFor = 300 * time.Millisecond
	strikeFor   = 400 * time.Millisecond
	shakeFor    = 400 * time.Millisecond
	shakeCells  = 2
)

// scrambleRevealed starts a scramble for every key revealed since before,
// which is what the reveal just put on screen.
func (m *Model) scrambleRevealed(before map[string]bool) tea.Cmd {
	if m.secret == nil {
		return nil
	}
	var cmds []tea.Cmd
	for k := range m.revealed {
		if before[k] {
			continue
		}
		if s := text.NewScramble(draw.Sanitize(valueToString(m.secret.Data[k])), scrambleFor); s.Active() {
			m.scrambles[k] = s
			cmds = append(cmds, s.Tick())
		}
	}
	return tea.Batch(cmds...)
}

// revealedValue is the value a revealed field shows: scrambling while it
// settles, then the value itself.
func (m *Model) revealedValue(k string) string {
	if s, ok := m.scrambles[k]; ok && s.Active() {
		return s.View()
	}
	return draw.Sanitize(valueToString(m.secret.Data[k]))
}

// strikeGone starts a strike for every version of the same secret that was
// live before this load and is deleted or destroyed now.
func (m *Model) strikeGone(was []vault.VersionInfo) tea.Cmd {
	live := map[int]bool{}
	for _, v := range was {
		live[v.Version] = !gone(v)
	}
	var cmds []tea.Cmd
	for _, v := range m.versions {
		if gone(v) && live[v.Version] {
			if s := text.NewStrike(versionLabel(v), strikeFor).WithInk(m.st.Dim.GetForeground()); s.Active() {
				m.strikes[v.Version] = s
				cmds = append(cmds, s.Tick())
			}
		}
	}
	return tea.Batch(cmds...)
}

// struck is a deleted or destroyed version's label, crossed out: drawn through
// while its strike runs, struck already for one that went before.
func (m *Model) struck(v vault.VersionInfo) string {
	if s, ok := m.strikes[v.Version]; ok && s.Active() {
		return s.View()
	}
	return text.NewStrike(versionLabel(v), 0).WithInk(m.st.Dim.GetForeground()).View()
}

func gone(v vault.VersionInfo) bool { return v.Destroyed || v.Deleted() }

// shakeRefusal sways the confirmation that refused what was typed in it.
func (m *Model) shakeRefusal() tea.Cmd {
	m.shake = text.NewShake(shakeCells, shakeFor)
	return m.shake.Tick()
}

// stopMotion drops whatever was moving for the secret that was on screen.
func (m *Model) stopMotion() {
	m.scrambles, m.strikes = map[string]text.Scramble{}, map[int]text.Strike{}
}

// animate hands a frame to whatever is moving; each takes only its own.
func (m *Model) animate(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for k, s := range m.scrambles {
		var cmd tea.Cmd
		if m.scrambles[k], cmd = s.Update(msg); !m.scrambles[k].Active() {
			delete(m.scrambles, k)
		}
		cmds = append(cmds, cmd)
	}
	for v, s := range m.strikes {
		var cmd tea.Cmd
		if m.strikes[v], cmd = s.Update(msg); !m.strikes[v].Active() {
			delete(m.strikes, v)
		}
		cmds = append(cmds, cmd)
	}
	var cmd tea.Cmd
	m.shake, cmd = m.shake.Update(msg)
	return tea.Batch(append(cmds, cmd)...)
}
