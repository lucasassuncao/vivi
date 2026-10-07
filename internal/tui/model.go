// Package tui is the driving adapter: vivi's interactive browser over a Vault
// server. It reads through vault.Client and never holds an SDK client, so the
// whole model runs against an in-memory one. Every request is a tea.Cmd.
package tui

import (
	"context"
	"github.com/lucasassuncao/bezel/draw"
	"github.com/lucasassuncao/bezel/theme"
	"io"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/bezel/shell"
	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/tui/activity"
	"github.com/lucasassuncao/vivi/internal/tui/auth"
	"github.com/lucasassuncao/vivi/internal/tui/policies"
	"github.com/lucasassuncao/vivi/internal/tui/secrets"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

type tab int

const (
	tabSecrets tab = iota
	tabPolicies
	tabAuth
	tabActivity
)

var tabNames = []string{"Secrets", "Policies", "Auth", "Activity"}

// What is in front of the user besides a tab is the shell's overlay stack:
// the help panel, the token panel, the command line. browsing reports that
// none of them is up - a tab's own modals are the tab's, and the two never
// meet: ":" opened over the version list leaves the version list where it was.
func (m *Model) browsing() bool { return !m.sh.HasOverlay() }

// modeName is what the crash note says about where the user was.
func (m *Model) modeName() string {
	switch {
	case m.showingCommands():
		return "command"
	case m.showingHelp():
		return "help"
	case m.showingToken():
		return "token"
	}
	return "browse"
}

func (m *Model) showingHelp() bool {
	_, ok := m.sh.TopOverlay().(helpOverlay)
	return ok
}

func (m *Model) showingToken() bool {
	_, ok := m.sh.TopOverlay().(tokenOverlay)
	return ok
}

// Options configures the browser.
type Options struct {
	Client vault.Client
	Token  *vault.TokenInfo
	// Ctx bounds every request the browser makes. It is the program's own
	// context, so quitting cancels whatever is still in flight instead of
	// leaving it to run out its own timeout. The zero value is Background.
	Ctx context.Context
	// Theme is what to render with. The zero value is bezel's adaptive default.
	Theme theme.Theme

	// Access is how much of the Vault this session may change. The zero value
	// is app.ReadWrite, which is the ordinary session.
	Access app.Access
	// ReadOnly is the policy Access was resolved from, kept only so a refusal
	// can say why rather than only that.
	ReadOnly app.ReadOnlyPolicy

	// Version is vivi's own version, for the header. Empty in a binary that was
	// built rather than released, which is what the header then says.
	Version string

	// Debug is where keystrokes are recorded, when VIVI_DEBUG named a file.
	// Nil is the ordinary session, which records nothing.
	Debug io.Writer
}

// Model is the bubbletea model for vivi.
type Model struct {
	ctx    context.Context
	client vault.Client
	st     ui.Styles
	// colors is the theme the styles were built from, kept because they are
	// built twice: once dark, and again when the terminal says otherwise.
	theme theme.Theme

	width, height int

	// sh is the chrome: header, tab strip, panels, status row, legend and the
	// overlays floating over the panes. vivi keeps the keys.
	sh  shell.Shell
	tab tab

	// version is vivi's own, shown in the header. Not the Vault's.
	version string
	// debug is nil unless VIVI_DEBUG asked for a log. See debug.go.
	debug io.Writer

	// The four tabs. The shell holds each component and never what is inside it.
	secretsTab  *secrets.Model
	policiesTab *policies.Model
	authTab     *auth.Model
	activityTab *activity.Model

	token  *vault.TokenInfo
	server vault.ServerInfo

	// access and readOnly are fixed for the life of the session: the policy met
	// the address once, before the alternate screen opened.
	access   app.Access
	readOnly app.ReadOnlyPolicy

	// detail is the right pane, a viewport over its content rather than the
	// content: a lipgloss height is a floor, not a ceiling, and a policy longer
	// than the terminal would push the pane's own border off screen.
	detail viewport.Model
	focus  ui.Focus

	// bannerCmds are the expiry timers of banners set during this Update, so
	// a handler that has no command to return still gets its banner cleared.
	bannerCmds []tea.Cmd

	pending int
}

// New builds the model. Preflight has already run by the time this is called,
// so the client is known to work and the token is known to be valid.
func New(opts Options) *Model {
	// Sized on the first WindowSizeMsg; the zero size only matters before the
	// terminal has told us anything.
	detail := viewport.New()

	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	m := &Model{
		detail: detail,
		ctx:    ctx,
		client: opts.Client,
		theme:  opts.Theme,
		// Dark until the terminal answers otherwise, which is also what a
		// terminal that will not answer is treated as. See Init.
		st:          ui.NewStyles(opts.Theme, true),
		activityTab: activity.New(),
		token:       opts.Token,
		server:      opts.Client.Server(),
		access:      opts.Access,
		readOnly:    opts.ReadOnly,
		version:     opts.Version,
		debug:       opts.Debug,
	}

	// The tabs that reach a server are built here and not in the literal: each
	// takes the Model as its ui.Requester, so the wrapping of a call - the
	// timeout, the session context, the in-flight count - stays in one place.
	m.secretsTab = secrets.New(opts.Client, m)
	m.policiesTab = policies.New(opts.Client, m)
	m.authTab = auth.New(opts.Client, m)

	m.sh = m.newShell()
	return m
}

// newShell builds the chrome. The tabs answer for their own legends through
// tabAdapter; the header and the write filter read the model, which is a
// pointer, so the callbacks stay current.
func (m *Model) newShell() shell.Shell {
	tabs := make([]shell.Tab, len(tabNames))
	for i := range tabNames {
		tabs[i] = tabAdapter{m: m, t: tab(i)}
	}
	return shell.New(shell.Config{
		Layout: m.layout(),
		Tabs:   tabs,
		Theme:  m.st.Shell(),
		Title:  "vivi",
		Header: func(shell.Context, int) []string { return m.headerLines() },
		Can:    m.can,
		// vivi answers every key itself: its tabs' actions are display only.
		StatusTTL:   bannerLife,
		LegendLines: legendMaxLines,
	})
}

// Init asks the terminal what colour its background is, along with the first
// listing. The answer picks between the built-in palette's light and dark
// halves; in v1 lipgloss asked on its own, and in v2 it is the program that
// holds the terminal, so it is the program that asks.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.secretsTab.Load(), tickClock(), tea.RequestBackgroundColor)
}

// Update folds a message into the model and then re-projects the detail pane,
// which is the one component holding state of its own.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	defer m.guardPanic()
	cmd := m.update(msg)
	m.relayout()
	m.syncDetail()
	// Every banner expires, including the ones set by a handler with no command
	// to return: a refusal that stays up until the next message is a session
	// that looks like it is still refusing.
	if len(m.bannerCmds) > 0 {
		cmd = tea.Batch(append([]tea.Cmd{cmd}, m.bannerCmds...)...)
		m.bannerCmds = nil
	}
	return m, cmd
}

func (m *Model) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	// What a tab asks the parent for. A tab owns its own state and nothing else,
	// so everything outside it arrives here as a request rather than a write.
	case ui.BannerMsg:
		switch {
		case msg.Cause != nil:
			return m.fail(msg.Text, msg.Cause)
		case msg.Err:
			return m.failWith(msg.Text)
		default:
			m.notify(msg.Text)
		}
		return nil

	case ui.FocusMsg:
		m.focus = msg.Focus
		return nil

	case ui.ScrollTopMsg:
		m.detail.GotoTop()
		return nil

	case ui.ScrollToLineMsg:
		m.scrollToLine(msg.Line)
		return nil

	case ui.CopyMsg:
		return m.sh.Copy(msg.Text, msg.What, msg.Op)

	case ui.RecordedMsg:
		m.activityTab.Record(msg.Op, msg.Path, msg.Detail, msg.Err)
		return nil

	case ui.JumpToPolicyMsg:
		return m.jumpToPolicy(msg.Name)

	case ui.ShowSecretsMsg:
		m.setTab(tabSecrets)
		return nil

	case ui.OpenCmdlineMsg:
		return m.openCmdline()

	case commandMsg:
		// Judged again: an answer may have moved it out of scope since enter.
		if !msg.c.scope(m) {
			return m.failWith(msg.c.name + " is not available here")
		}
		return msg.c.run(m, msg.arg)

	case tea.BackgroundColorMsg:
		// The terminal answering the query Init sent. A light background flips
		// the built-in palette; a named theme is fixed colours and unmoved.
		m.st = ui.NewStyles(m.theme, msg.IsDark())
		m.sh = m.sh.SetTheme(m.st.Shell())
		return nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.sh, _, _ = m.sh.Update(msg)
		m.relayout()
		// The one widget sized to the terminal rather than measured at render:
		// a text area has to know its width to wrap and its height to scroll,
		// and it is the Secrets tab that holds one.
		m.secretsTab.Resize(m.contextIn(m.bodyHeight()))
		return nil

	case clockMsg:
		// Nothing to fold in: the token countdown reads the clock when it
		// renders, and this is what brings the render around.
		return tickClock()

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	// The shell's own messages: a banner expiring, the palette finishing, a
	// command picked in it, a paste into it. Everything else is the tabs'.
	if sh, handled, cmd := m.sh.Update(msg); handled {
		m.sh = sh
		return cmd
	}
	return m.handleData(msg)
}

// syncDetail projects the model onto the detail viewport, whose only state of
// its own is the scroll offset. Doing it here rather than while rendering is
// what lets View be pure, and frees followFieldCursor from needing a frame first.
func (m *Model) syncDetail() {
	// Both layouts keep a viewport: narrow, the detail is the only pane once it
	// has focus and needs scrolling more, not less. The shell placed it, so the
	// renderer and this agree by construction.
	width, height := m.detailGeometry()

	right := m.renderDetail(width)

	m.detail.SetWidth(width)
	m.detail.SetContent(right)
	// Overflowing, the last row is kept for "↓ N more lines", as in the lists.
	// Set once: a full height first would clamp the scroll a row short.
	if height > 1 && m.detail.TotalLineCount() > height {
		height--
	}
	m.detail.SetHeight(height)
}

// reloadCurrent drops what the tab in front of the user is showing and fetches
// it again. It used to reload only the tree node, so "R" did nothing on
// Policies and Auth - the two tabs read once and never again.
func (m *Model) reloadCurrent() tea.Cmd {
	return m.currentTab().Reload()
}

// uiContext is the whole of what a tab is allowed to read of the session. By
// value, so a tab can look and cannot write: everything it wants changed goes
// back up as a message.
func (m *Model) uiContext() ui.Context {
	var policies []string
	if m.token != nil {
		policies = m.token.Policies
	}
	// The terminal's height and not the body's. Measuring the body means asking
	// the footer how tall it is, the footer asks the tab for its legend, and the
	// tab is handed a context: that is a cycle, and it overflowed the stack.
	// The paths that draw into the body pass the real height through contextIn.
	return ui.Context{
		Styles:        m.st,
		Focus:         m.focus,
		Width:         m.width,
		Height:        m.height,
		TokenPolicies: policies,
		Access:        m.access,
		ReadOnly:      m.readOnly,
	}
}

// contextIn is uiContext for a caller that has already measured the body it is
// drawing into, which is every caller that draws rather than asks.
func (m *Model) contextIn(bodyHeight int) ui.Context {
	ctx := m.uiContext()
	ctx.Height = bodyHeight
	return ctx
}

func (m *Model) nextTab(delta int) {
	m.setTab(tab((int(m.tab) + delta + len(tabNames)) % len(tabNames)))
	m.detail.GotoTop()
}

// setTab moves to a tab and drops the mode the old one owned. The version list
// and the filter are Secrets-tab state, and the jumps run from inside them: one
// that left the list armed put its destructive keys on a screen showing a
// policy, still aimed at the secret that went away with the tab.
func (m *Model) setTab(t tab) {
	if t != m.tab {
		m.currentTab().Blur()
	}
	m.tab = t
	m.sh = m.sh.SetTab(int(t))
	m.closeOverlays()
}

// closeOverlays drops the shell's own panels: the command line, help, token.
func (m *Model) closeOverlays() {
	for m.sh.HasOverlay() {
		m.sh = m.sh.Pop()
	}
}

// ensureTabLoaded fetches a tab's data the first time it is shown.
func (m *Model) ensureTabLoaded() tea.Cmd {
	return m.currentTab().Load()
}

// notify shows a transient message in the footer. Sanitized where it is set
// rather than where it is drawn: most banners name a path or an error the
// server worded, and the footer is one line that outlives every pane.
func (m *Model) notify(msg string) { m.setBanner(draw.Sanitize(msg), shell.OK) }

// setBanner puts a message on the status row for bannerLife, keeping the
// expiry timer for Update to return.
func (m *Model) setBanner(text string, level shell.Level) {
	var cmd tea.Cmd
	m.sh, cmd = m.sh.SetStatus(text, level, bannerLife)
	m.bannerCmds = append(m.bannerCmds, cmd)
}

// banner is the status row's message, empty when none is up.
func (m *Model) banner() string { return m.sh.Status() }

// fail shows an error in the footer and returns the command that clears it.
// Errors never tear down the interface: a denied path or a failed write is
// information, not a reason to lose the session.
func (m *Model) fail(what string, err error) tea.Cmd {
	return m.failWith(what + ": " + humanize(err))
}

// failWith shows a message the caller has already worded, for the failures where
// the error's own wording would be misleading.
func (m *Model) failWith(msg string) tea.Cmd {
	m.setBanner(draw.Sanitize(msg), shell.Error)
	return nil
}

// humanize is ui.Humanize, kept as a name here because thirty call sites read
// better without the qualifier and because a tab now needs the same wording.
func humanize(err error) string { return ui.Humanize(err) }

// bannerIsError reports whether the status row shows a failure.
func (m *Model) bannerIsError() bool { return m.sh.StatusLevel() == shell.Error }
