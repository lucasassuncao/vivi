package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// The shell's own traffic: the token it counts down, and the two timers. Every
// other request belongs to a tab, which builds it and reads its answer; what
// stays here is how a request is wrapped, because that is the same for all of
// them.

func (tokenMsg) AnswersRequest() {}
func (renewMsg) AnswersRequest() {}

type tokenMsg struct {
	info *vault.TokenInfo
	err  error
}

// renewMsg is the answer to a renewal. It carries a fresh TokenInfo rather than
// a duration, because the server may cap the extension and the header has to
// count down what will actually be honoured.
type renewMsg struct {
	info *vault.TokenInfo
	err  error
}

// bannerExpiredMsg clears a transient footer message.
type bannerExpiredMsg struct{ id int }

// clockMsg brings the render around so the header's token countdown moves on
// its own. Nothing in the model changes: the countdown is computed at render
// time, and without a wake-up it would sit at whatever the last key left.
type clockMsg struct{}

func tickClock() tea.Cmd {
	return tea.Tick(clockInterval, func(time.Time) tea.Msg { return clockMsg{} })
}

// callTimeout bounds every request so a hung server cannot leave a node
// spinning forever.
const callTimeout = 30 * time.Second

// Request is the ui.Requester the tabs are handed. The pending counter rises
// here, because as the caller's bookkeeping two forgot; and the context is
// captured now, so the closure holds no reference to the Model.
func (m *Model) Request(call func(context.Context) tea.Msg) tea.Cmd {
	m.pending++
	parent := m.ctx

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, callTimeout)
		defer cancel()
		return call(ctx)
	}
}

// request is Request with the client handed in, for the shell's own two calls.
func (m *Model) request(call func(context.Context, vault.Client) ui.RequestAnswer) tea.Cmd {
	client := m.client
	return m.Request(func(ctx context.Context) tea.Msg { return call(ctx, client) })
}

func (m *Model) loadToken() tea.Cmd {
	return m.request(func(ctx context.Context, c vault.Client) ui.RequestAnswer {
		info, err := c.TokenInfo(ctx)
		return tokenMsg{info: info, err: err}
	})
}

// renewToken extends the session without leaving the app.
func (m *Model) renewToken() tea.Cmd {
	return m.request(func(ctx context.Context, c vault.Client) ui.RequestAnswer {
		info, err := c.RenewToken(ctx)
		return renewMsg{info: info, err: err}
	})
}

func expireBanner(id int) tea.Cmd {
	return tea.Tick(bannerLife, func(time.Time) tea.Msg {
		return bannerExpiredMsg{id: id}
	})
}
