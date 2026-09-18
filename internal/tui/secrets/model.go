package secrets

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
	"github.com/lucasassuncao/vivi/internal/vault"
)

// mode is what this tab is doing, with whatever that takes riding inside it:
// "editing with no editor" is a state that cannot be written down. As an enum
// beside nullable fields it could, and a failed rollback did exactly that.
//
// These six are the tab's own. The shell keeps a machine of its own for the
// command line, the help panel and the token panel, and the two never meet: ":"
// opened over the version list leaves the version list exactly where it was.
type mode = ui.Mode

type browsing struct{}

// filtering types a fuzzy pattern over what is already loaded.
type filtering struct{}

type editing struct{ editor *editor }

type confirming struct{ confirmation *confirmation }

// choosingVersion is the version list over the current secret.
type choosingVersion struct{}

type showingDiff struct{ diff *diffResult }

func (browsing) Name() string        { return "browse" }
func (filtering) Name() string       { return "filter" }
func (editing) Name() string         { return "edit" }
func (confirming) Name() string      { return "confirm" }
func (choosingVersion) Name() string { return "versions" }
func (showingDiff) Name() string     { return "diff" }

// modeIs reports which mode the tab is in, for the readers that do not care
// what it carries. A type assertion and not a Name() comparison: the misspelled
// string compiled and answered false for the life of the process.
func modeIs[M mode](m *Model) bool {
	_, ok := m.currentMode().(M)
	return ok
}

// Model is the Secrets tab. Twenty fields, and every one of them was a field of
// the shell's model before this package existed.
type Model struct {
	req    ui.Requester
	client vault.Client

	mode mode

	tree        tree
	filterInput textinput.Model

	// The secret the detail pane is drawing, and its history. This is what is on
	// screen, not what the cursor is on: moving between secrets leaves it until
	// every answer has arrived, so the pane changes once. See secretLoad.
	sel           *node
	secret        *vault.Secret
	secretVersion int
	versions      []vault.VersionInfo
	meta          *vault.Metadata
	detailErr     error

	// loading is the node being read, while it is being read; nil means the pane
	// is showing everything it waits for. Nothing is cached between visits: a
	// remembered value goes stale the moment somebody else writes.
	loading *secretLoad
	// capabilities is what the server says this token may do at the selected
	// secret, and capsErr is why it could not say. Both are keyed to capsNode:
	// an answer that arrives after the cursor has moved describes another path.
	capabilities []string
	capsErr      error
	capsNode     *node
	fieldCursor  int
	revealed     map[string]bool

	versionCursor int
	marked        map[int]bool

	// The diff modal keeps its own cursor and reveal set. Sharing the pane's
	// would tie them both ways: loading a secret wipes revealed and would blank
	// an open diff, and a reveal made inside would outlive the modal.
	diffCursor   int
	diffRevealed map[string]bool

	// pendingGoto is a ":goto" still walking down to its path. Every level is a
	// listing that has to arrive before the next one can be asked for, so the
	// walk is state that incoming messages advance rather than a loop.
	pendingGoto string

	// policyDocs are the documents of this token's policies, which the reverse
	// lookup reads. Fetched once and never refetched: a policy changing under a
	// session is rarer than opening one secret after another.
	policyDocs      map[string]string
	policyDocsAsked bool

	// mountsAsked keeps the first listing to one request. The shell asks on
	// every visit to this tab, because that is what it asks of every tab.
	mountsAsked bool

	// What the session looks like from here, refreshed at every entry point by
	// adopt. Held rather than threaded because a hundred render helpers read
	// m.st, and rewriting them all to take a context would have been the change
	// most likely to introduce the regression this whole move exists to avoid.
	st            ui.Styles
	width, height int
	focus         ui.Focus
	access        app.Access
	readOnly      app.ReadOnlyPolicy
	tokenPolicies []string

	// outbox is what the tab wants the shell to do, collected during a call and
	// handed up when it returns. It exists so notify and fail read here exactly
	// as they read when this was all one package.
	outbox []tea.Msg
}

func New(client vault.Client, req ui.Requester) *Model {
	filter := ui.NewInput()
	filter.Prompt = "/"
	filter.Placeholder = "filter what is already loaded"

	return &Model{
		req:          req,
		client:       client,
		mode:         browsing{},
		filterInput:  filter,
		revealed:     map[string]bool{},
		diffRevealed: map[string]bool{},
		marked:       map[int]bool{},
		policyDocs:   map[string]string{},
	}
}

// adopt copies the session's view of itself at the start of every entry point,
// so the helpers below read the current frame and never the last one.
func (m *Model) adopt(ctx ui.Context) {
	m.st, m.focus = ctx.Styles, ctx.Focus
	m.width, m.height = ctx.Width, ctx.Height
	m.access, m.readOnly = ctx.Access, ctx.ReadOnly
	m.tokenPolicies = ctx.TokenPolicies
}

// drain hands the shell everything the call collected, alongside whatever
// command the call itself produced.
func (m *Model) drain(cmd tea.Cmd) tea.Cmd {
	if len(m.outbox) == 0 {
		return cmd
	}
	out := m.outbox
	m.outbox = nil
	return tea.Batch(cmd, ui.Emit(out...))
}

// bodyHeight is the pane the tab is drawing into, which the shell measured.
func (m *Model) bodyHeight() int { return m.height }

// notify, fail and failWith are the shell's footer, reached from here. They
// keep their old names and old shapes so the call sites did not have to move
// when the code around them did.
// All three queue rather than return, and the two that return a command return
// nil. A caller that drops the return still says what it refused: when this was
// one package these wrote the banner field, and a dropped command lost only the
// timer that clears it. Queuing keeps that, and keeps every call site as it was.
func (m *Model) notify(msg string) {
	m.outbox = append(m.outbox, ui.BannerMsg{Text: msg})
}

func (m *Model) fail(what string, err error) tea.Cmd {
	m.outbox = append(m.outbox, ui.BannerMsg{Text: what, Cause: err})
	return nil
}

func (m *Model) failWith(msg string) tea.Cmd {
	m.outbox = append(m.outbox, ui.BannerMsg{Text: msg, Err: true})
	return nil
}

// scrollTop sends the detail pane back to its first line. The viewport is the
// shell's, so this is a request.
func (m *Model) scrollTop() {
	m.outbox = append(m.outbox, ui.ScrollTopMsg{})
}

// record puts a finished write in the Activity tab, through the shell: a tab
// does not name another tab.
func (m *Model) record(op, path, detail string, err error) {
	m.outbox = append(m.outbox, ui.RecordedMsg{Op: op, Path: path, Detail: detail, Err: err})
}

// humanize keeps the name a hundred error paths already use.
func humanize(err error) string { return ui.Humanize(err) }

// currentMode is m.mode with the one value it must never be: nil, which the
// type system hands to any bad assignment and which then fails a keystroke
// later, in a blameless method. It reads and does not repair.
func (m *Model) currentMode() mode {
	if m.mode == nil {
		return browsing{}
	}
	return m.mode
}

// currentEditor is the open editor, or nil. The renderers need it without
// knowing whether the mode is the editing one.
func (m *Model) currentEditor() *editor {
	if e, ok := m.mode.(editing); ok {
		return e.editor
	}
	return nil
}

// ensureState repairs what a bad transition could have left nil, before any
// handler dereferences it.
func (m *Model) ensureState() {
	if m.mode == nil {
		m.mode = browsing{}
	}
	if m.revealed == nil {
		m.revealed = map[string]bool{}
	}
	if m.diffRevealed == nil {
		m.diffRevealed = map[string]bool{}
	}
	if m.marked == nil {
		m.marked = map[int]bool{}
	}
	if m.policyDocs == nil {
		m.policyDocs = map[string]string{}
	}
}
