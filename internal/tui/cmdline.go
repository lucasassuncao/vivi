package tui

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// cmdline is the colon command line and the list of what it could still become.
// It carries the mode it was opened from - the version list is the other place
// ":" opens - and that is what every scope predicate is judged against.
type cmdline struct {
	input textinput.Model
	from  mode

	// cursor is the highlighted candidate, or -1 for none. It starts at -1 so
	// enter obeys what was typed: a default highlight would resolve the
	// ambiguity resolve() refuses to, running ":cop" as copy by table order.
	cursor int

	// err is shown under the candidates. It belongs to the panel rather than to
	// the footer banner because the footer is where the command line itself is
	// being typed: a banner would cover the line the message is about.
	err string
}

// cmdlineRows is how many candidates are shown at once. The list is a hint, not
// a document: past this the user should be typing, not scrolling.
const cmdlineRows = 8

func (commanding) Name() string { return "command" }

// browseMode is the mode commands are judged and run against: the cmdline's
// own mode is transient and belongs to the cmdline, not to the app.
func (m *Model) browseMode() mode {
	if c, ok := m.mode.(commanding); ok && c.line != nil && c.line.from != nil {
		return c.line.from
	}
	return m.currentMode()
}

// browseModeIs is modeIs against the mode a command is judged in, which while
// the palette is open is the mode it was opened from rather than the palette
// itself.
func browseModeIs[M mode](m *Model) bool {
	_, ok := m.browseMode().(M)
	return ok
}

// openCmdline starts a colon command from the current mode.
func (m *Model) openCmdline() tea.Cmd {
	in := ui.NewInput()
	in.Prompt = ":"
	in.Focus()

	m.mode = commanding{line: &cmdline{input: in, from: m.mode, cursor: -1}}
	return textinput.Blink
}

// leaveCmdline puts back the mode the command line was opened from. A method
// and not four assignments because mode is an interface: reaching an exit with
// no mode behind it would set nil, and crash a keystroke later.
func (m *Model) leaveCmdline(p *cmdline) {
	if p == nil || p.from == nil {
		m.mode = browsing{}
		return
	}
	m.mode = p.from
}

// keyCommand drives the cmdline.
func (m *Model) keyCommand(p *cmdline, msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.leaveCmdline(p)
		return nil

	case "enter":
		return m.runTyped(p)

	case "up", "down":
		cands := m.candidates(p.input.Value())
		if len(cands) == 0 {
			return nil
		}
		delta := 1
		if msg.String() == "up" {
			delta = -1
		}
		// From "nothing picked", down lands on the first and up on the last,
		// so both arrows reach the list without a wasted press.
		if p.cursor < 0 {
			p.cursor = 0
			if delta < 0 {
				p.cursor = len(cands) - 1
			}
			return nil
		}
		p.cursor = (p.cursor + delta + len(cands)) % len(cands)
		return nil

	case "tab":
		return m.complete(p)
	}

	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	// Typing invalidates the pick: the list under it has changed, so the index
	// no longer points at what the user chose. It also answers whatever the last
	// message complained about.
	p.cursor, p.err = -1, ""
	return cmd
}

// complete turns a choice made with the arrows into typed text, so the line is
// an exact name and enter runs it by the ordinary rule. With nothing picked it
// completes the only candidate; an argument-taking command keeps the caret.
func (m *Model) complete(p *cmdline) tea.Cmd {
	cands := m.candidates(p.input.Value())

	var c command
	switch {
	case p.cursor >= 0 && p.cursor < len(cands):
		c = cands[p.cursor]
	case len(cands) == 1:
		c = cands[0]
	default:
		return nil
	}

	line := c.name
	if c.arg != "" {
		line += " "
		// Keep an argument that was already typed: completing ":got kv/app"
		// must not throw the path away.
		if _, arg := splitCommand(p.input.Value()); arg != "" {
			line += arg
		}
	}
	p.input.SetValue(line)
	p.input.CursorEnd()
	p.cursor = -1
	return nil
}

// runTyped resolves the line and runs it, leaving the cmdline either way. The
// order matters: resolved while the cmdline is still the mode, since that is
// what scopes are judged against, and restored before run.
func (m *Model) runTyped(p *cmdline) tea.Cmd {
	c, arg, err := m.chosen(p)
	if err != nil {
		return m.reportResolution(p, err)
	}

	if c.arg == "" && arg != "" {
		m.leaveCmdline(p)
		return m.failWith(":" + c.name + " takes no argument")
	}

	m.leaveCmdline(p)
	return c.run(m, arg)
}

// reportResolution says what a line that named nothing does to the panel. Only
// one failure keeps the command line open, the one whose answer is already on
// screen: an ambiguous prefix needs a keystroke, not a fresh start.
func (m *Model) reportResolution(p *cmdline, err error) tea.Cmd {
	var ambiguous ambiguousPrefixError
	if errors.As(err, &ambiguous) {
		p.err = err.Error()
		return nil
	}

	m.leaveCmdline(p)

	var empty emptyLineError
	if errors.As(err, &empty) {
		return nil // ":" and straight back out: not a mistake, so not a message
	}
	return m.failWith(err.Error())
}

// chosen is what enter will run: the candidate picked with the arrows, or
// whatever the typed line resolves to. A pick is a decision the resolution
// rules do not have to second-guess, which is why it is checked first.
func (m *Model) chosen(p *cmdline) (command, string, error) {
	cands := m.candidates(p.input.Value())
	if p.cursor >= 0 && p.cursor < len(cands) {
		_, arg := splitCommand(p.input.Value())
		return cands[p.cursor], arg, nil
	}
	return m.resolve(p.input.Value())
}

// runGoto hands the path to the Secrets tab, which is whose vocabulary a path
// is. Reading it off the line was this file's whole part in it.
func (m *Model) runGoto(path string) tea.Cmd {
	return m.secretsTab.Goto(path)
}

// Rendering.

// renderCmdline draws the candidate list over the bottom of the panes, directly
// above the command line. Not centred like the modals: the list belongs to the
// line being typed, and the middle of the screen would separate the two.
func (m *Model) renderCmdline(p *cmdline, height int) string {
	box := m.cmdlineBox(p)
	if box == "" {
		return m.renderPanes(height)
	}

	_, boxHeight := ui.BlockSize(box)
	panes := ui.PadHeight(m.renderPanes(height), height)
	return ui.Composite(box, panes, 0, max(0, height-boxHeight))
}

func (m *Model) cmdlineBox(p *cmdline) string {
	var lines []string

	cands := m.candidates(p.input.Value())
	if len(cands) == 0 {
		lines = []string{m.st.Dim.Render("  no command matches")}
	} else {
		// The window follows the cursor, so a pick below the fold scrolls into
		// view. The cursor can outlive the list it indexes - incoming answers
		// change it - and a stale one past the end makes cands[start:end] panic.
		start := min(max(p.cursor-cmdlineRows+1, 0), max(0, len(cands)-1))
		end := min(start+cmdlineRows, len(cands))

		// One width for the whole name column: a ragged left edge on the titles
		// makes a list this short read as unrelated lines rather than a table.
		nameWidth := 0
		for _, c := range cands[start:end] {
			nameWidth = max(nameWidth, lipgloss.Width(c.label()))
		}

		for i := start; i < end; i++ {
			lines = append(lines, m.cmdlineRow(cands[i], nameWidth, i == p.cursor))
		}
		if end < len(cands) {
			lines = append(lines, m.st.Dim.Render(fmt.Sprintf("  … %d more", len(cands)-end)))
		}
	}

	if p.err != "" {
		lines = append(lines, m.st.Danger.Render("  "+p.err))
	}

	// A rule marks the panel off from the panes; a border would make it a modal.
	// max is not decoration: strings.Repeat panics on a negative count, and this
	// width comes from the terminal without a layout flooring it first.
	lines = append([]string{m.st.Dim.Render(strings.Repeat("─", max(0, m.width)))}, lines...)

	// Every line is padded to the full width so the panel replaces what is
	// under it. Left short, the pane borders show through the gaps at the end
	// of each row and the panel reads as a rendering fault.
	for i, line := range lines {
		lines[i] = pad(ui.Truncate(line, m.width), m.width)
	}
	return strings.Join(lines, "\n")
}

// pad grows a rendered line to a width, measuring what is on screen rather than
// what is in the string: the styles have written escape sequences into it that
// occupy no columns.
func pad(line string, width int) string {
	if gap := width - lipgloss.Width(line); gap > 0 {
		return line + strings.Repeat(" ", gap)
	}
	return line
}

// cmdlineRow is one candidate: what to type, what to press instead, and what it
// does. The keystroke column is the point of the panel - the slow path teaching
// the fast one - so it keeps its width even on commands with no key.
func (m *Model) cmdlineRow(c command, nameWidth int, picked bool) string {
	name := pad(c.label(), nameWidth)

	key := "  "
	if c.key != "" {
		key = c.key
	}
	key = pad(key, 7)

	if picked {
		return ui.Truncate(m.st.Cursor.Render("› "+name+"  "+key+"  "+c.title), m.width)
	}
	return ui.Truncate("  "+m.st.Key.Render(name)+"  "+
		m.st.LegendKey.Render(key)+"  "+m.st.Help.Render(c.title), m.width)
}
