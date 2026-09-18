package secrets

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// confirmation is a pending destructive action. The friction scales with
// reversibility: a save asks y/n, while destroy and delete-metadata - the only
// two Vault cannot undo - require the last path segment to be typed.
type confirmation struct {
	title string
	// lines is the body: what will change, and what it costs.
	lines []string
	// danger renders the modal in the alarming palette.
	danger bool

	// requireText, when set, must be typed exactly before the action can run.
	requireText string
	input       textinput.Model

	// inputLabel names the input and moves it above the lines, drawn as a form
	// row; preview, when set, is a second row computed from what is typed so
	// far, labelled previewLabel. A bare "> " prompt said nothing about either.
	inputLabel   string
	previewLabel string
	preview      func(typed string) string

	// confirmLabel overrides the default "[y] confirm" hint.
	confirmLabel string

	// capture routes every keystroke to the input even when the modal is not a
	// dangerous one, for modals that ask for free text such as a new path.
	capture bool

	// onConfirm is dispatched when the user accepts, onCancel when they escape a
	// modal returning elsewhere. Messages and not callbacks: a callback updates
	// outside the loop, and clearing the modal first left its input nil.
	onConfirm tea.Msg
	onCancel  tea.Msg
}

// newConfirm builds a y/n confirmation.
func newConfirm(title string, lines []string, onConfirm tea.Msg) *confirmation {
	return &confirmation{title: title, lines: lines, onConfirm: onConfirm}
}

// newDangerConfirm builds a confirmation that must be typed out. require is the
// text the user has to reproduce, normally the last segment of the path.
func newDangerConfirm(title string, lines []string, require string, onConfirm tea.Msg) *confirmation {
	in := ui.NewInput()
	in.Prompt = "> "
	in.Placeholder = require
	in.Focus()

	return &confirmation{
		title:       title,
		lines:       lines,
		danger:      true,
		requireText: require,
		input:       in,
		onConfirm:   onConfirm,
	}
}

// satisfied reports whether the confirmation may run.
func (c *confirmation) satisfied() bool {
	if c.requireText == "" {
		return true
	}
	return c.input.Value() == c.requireText
}

// needsInput reports whether the modal is capturing keystrokes.
func (c *confirmation) needsInput() bool { return c.requireText != "" }

// accepted is the message to dispatch, handing over the text the modal captured
// to whichever message asked for it.
func (c *confirmation) accepted() tea.Msg {
	if taker, ok := c.onConfirm.(inputTaker); ok {
		return taker.withInput(c.input.Value())
	}
	return c.onConfirm
}

// inputTaker is a confirmation message that needs the text the user typed. It
// keeps the reading of the input at the one moment the input still exists,
// without the message having to reach for the model to find it.
type inputTaker interface {
	withInput(string) tea.Msg
}

// The confirmed messages. Each names a transition the user has agreed to, so a
// destroy is as traceable through the loop as a keystroke or a server answer.

type confirmedCreateMsg struct {
	parent *node
	name   string
}

func (m confirmedCreateMsg) withInput(s string) tea.Msg {
	m.name = s
	return m
}

// confirmedDiscardMsg drops an edit that wrote nothing. creating says the edit
// was a new secret, whose selection the tree never had and has to lose.
type confirmedDiscardMsg struct{ creating bool }

type confirmedSaveMsg struct {
	node *node
	data map[string]any
	cas  int
	// editor is where a rejected write returns to, with the edit intact. It
	// travels with the operation because the modal replaced the editing mode:
	// nothing else is still holding it.
	editor *editor
}

// confirmedReloadMsg answers a check-and-set rejection by dropping the edit and
// reading the server's version.
type confirmedReloadMsg struct{ node *node }

// restoreModeMsg puts a mode back after a modal that declined to leave it. The
// modal replaced whatever was on screen, so cancelling has to carry the way
// back with it: a check-and-set rejection returns to the editor, edit intact.
type restoreModeMsg struct{ mode mode }

type confirmedDeleteMsg struct {
	node     *node
	versions []int
}

type confirmedDestroyMsg struct {
	node     *node
	versions []int
}

type confirmedDeleteMetadataMsg struct{ node *node }

type confirmedRollbackMsg struct {
	node            *node
	toVersion, base int
}

// dispatch turns a message into the command that delivers it, which is how a
// confirmation re-enters the update loop instead of mutating from a callback.
func dispatch(msg tea.Msg) tea.Cmd {
	if msg == nil {
		return nil
	}
	return func() tea.Msg { return msg }
}

// keyConfirm routes keys while a modal is open. The modal comes from the mode,
// so it is never missing, and leaving is one assignment: it cannot be cleared
// while its own state lingers behind.
func (m *Model) keyConfirm(c *confirmation, msg tea.KeyPressMsg) tea.Cmd {
	// Accepting reads the input and dispatches the message the modal carries;
	// what that message does happens in the update loop, one message later.
	switch msg.String() {
	case "esc":
		m.mode = browsing{}
		return dispatch(c.onCancel)
	case "enter":
		if c.satisfied() {
			accepted := c.accepted()
			m.mode = browsing{}
			return dispatch(accepted)
		}
		return nil
	case "y", "Y":
		// A typed confirmation must not be satisfiable by a single key.
		if !c.needsInput() && !c.capture {
			accepted := c.accepted()
			m.mode = browsing{}
			return dispatch(accepted)
		}
	case "n", "N":
		// The same way out as esc. The modal offers them as one hint, so a
		// modal that carries a way back has to honour both.
		if !c.needsInput() && !c.capture {
			m.mode = browsing{}
			return dispatch(c.onCancel)
		}
	}

	if c.needsInput() || c.capture {
		var cmd tea.Cmd
		c.input, cmd = c.input.Update(msg)
		return cmd
	}
	return nil
}
