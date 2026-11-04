package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/bezel/shell"
)

// actions is the command table as bezel actions for its palette. The key is
// only a hint: vivi still routes its own keys.
func (m *Model) actions() []shell.Action {
	out := make([]shell.Action, 0, len(commands))
	for _, c := range commands {
		out = append(out, m.action(c))
	}
	return out
}

// action turns one table row into a palette command judged by its scope.
func (m *Model) action(c command) shell.Action {
	opts := []shell.Option{shell.When(func(shell.Context) bool { return c.scope(m) })}
	if c.arg != "" {
		opts = append(opts, shell.Arg(c.arg))
	}
	if c.key != "" {
		opts = append(opts, shell.KeyHint(c.key))
	}
	run := func(ac shell.ActionContext) tea.Cmd {
		return func() tea.Msg { return commandMsg{c: c, arg: ac.Arg} }
	}
	return shell.Command(c.name, c.title, run, opts...)
}

// commandMsg runs a palette command in vivi's own Update, after the shell is
// stored: run changes m.sh, and inside Shell.Update that change would be lost.
type commandMsg struct {
	c   command
	arg string
}

// showingCommands reports bezel's palette on top of the stack.
func (m *Model) showingCommands() bool {
	_, ok := m.sh.CommandLine()
	return ok
}

// openCmdline opens the palette over what is in front: browsing, or the
// version list. Scopes are judged now, against that.
func (m *Model) openCmdline() tea.Cmd {
	m.closeOverlays()
	m.sh = m.sh.SetActions(m.actions()...).OpenCommands()
	return nil
}

// runGoto hands the path to the Secrets tab, which is whose vocabulary a path
// is. Reading it off the line was the palette's whole part in it.
func (m *Model) runGoto(path string) tea.Cmd {
	return m.secretsTab.Goto(path)
}
