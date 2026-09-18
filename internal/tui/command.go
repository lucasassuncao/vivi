package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasassuncao/vivi/internal/tui/secrets"
	"github.com/lucasassuncao/vivi/internal/tui/ui"
)

// command is one action, named so it can be typed and keyed so it can be
// pressed: run calls the method the keystroke calls, so ":copy" and "y" cannot
// drift. The legend, help and key switch were three lists nothing checked.
type command struct {
	name string
	// arg names the argument in the palette, and is empty for the commands
	// that take none. It is a placeholder, not a value: "goto" shows as
	// ":goto <path>".
	arg string
	// key is the keystroke that already runs this, printed beside the name so
	// the palette teaches the fast path instead of replacing it. Empty for the
	// commands that exist only here, which are the ones taking an argument.
	key   string
	title string
	// scope decides whether the command can run at all right now. The palette
	// lists only what passes: an entry that does nothing when chosen is worse
	// than an absent one, because it teaches a keystroke that also does nothing.
	scope func(*Model) bool
	run   func(*Model, string) tea.Cmd
}

// label is how the command is written in the palette.
func (c command) label() string {
	if c.arg == "" {
		return ":" + c.name
	}
	return ":" + c.name + " <" + c.arg + ">"
}

// act adapts a keystroke handler that returns nothing, which most of them do:
// the ones that only open a modal or flip a mode have no request to issue.
func act(f func(*Model)) func(*Model, string) tea.Cmd {
	return func(m *Model, _ string) tea.Cmd { f(m); return nil }
}

// issue adapts a keystroke handler that already returns a command.
func issue(f func(*Model) tea.Cmd) func(*Model, string) tea.Cmd {
	return func(m *Model, _ string) tea.Cmd { return f(m) }
}

// onSecrets adapts an action on the Secrets tab. It takes no argument; the
// context goes in because every way into a tab carries one.
func onSecrets(f func(*secrets.Model, ui.Context) tea.Cmd) func(*Model, string) tea.Cmd {
	return func(m *Model, _ string) tea.Cmd { return f(m.secretsTab, m.uiContext()) }
}

// onSecretsArg is onSecrets for the two commands that take one.
func onSecretsArg(f func(*secrets.Model, string, ui.Context) tea.Cmd) func(*Model, string) tea.Cmd {
	return func(m *Model, arg string) tea.Cmd { return f(m.secretsTab, arg, m.uiContext()) }
}

// Scope. Offer a command when it can act or explain what is missing; hide it
// only when meaningless here, so a scope matches its action's guard. ":delete"
// on a folder taught a dead keystroke; a vanished ":diff" never says "mark two".

// always is the scope of a command with no preconditions at all.
func always(*Model) bool { return true }

// browsingTab reports that a tab is in front of the user, rather than a modal
// or the version list.
func browsingTab(m *Model, t tab) bool {
	return m.tab == t && browseModeIs[browsing](m)
}

func onPoliciesTab(m *Model) bool { return browsingTab(m, tabPolicies) }
func onAuthTab(m *Model) bool     { return browsingTab(m, tabAuth) }

// selectedSecret is for actions that need a secret and not a folder. The copies
// need no scope of their own: they exist only where the panel drew the line
// they copy, and a folder's panel draws a path and a listing and nothing else.
func selectedSecret(m *Model) bool {
	return onSecretsTab(m) && m.secretsTab.SelectedSecret()
}

// onSecretsTab is the scope shared by everything in the secrets tab: that tab,
// with its own panes in front of the user rather than one of its modals.
func onSecretsTab(m *Model) bool {
	return m.tab == tabSecrets && m.secretsTab.IsBrowsing()
}

// hasField reports that there is a selected field for reveal and copy to act
// on, which is the guard those two apply anyway.
func hasField(m *Model) bool {
	return onSecretsTab(m) && m.secretsTab.HasField()
}

func onVersionList(m *Model) bool {
	return m.tab == tabSecrets && m.secretsTab.OnVersionList()
}

// commands is the whole table, and the single source of truth for what vivi can
// be asked to do. It is grouped by tab for reading, not for display: the palette
// sorts by name, so where an entry is added here changes nothing.
var commands = []command{
	// Secrets.
	{name: "reveal", key: "r", title: "reveal the secret, or one field from the detail pane",
		scope: hasField, run: onSecrets((*secrets.Model).RevealCurrentField)},
	{name: "copy", key: "y", title: "copy the secret, or one field from the detail pane",
		scope: hasField, run: onSecrets((*secrets.Model).CopyCurrentValue)},
	// The per-line copies are not listed here: they are generated from whatever
	// the Command Reference is showing. See copyCommands.
	{name: "edit", key: "e", title: "edit the secret",
		scope: func(m *Model) bool { return onSecretsTab(m) && m.secretsTab.HasSecret() && writable(m) },
		run:   onSecrets((*secrets.Model).BeginEdit)},
	{name: "create", key: "a", title: "create a secret here",
		scope: func(m *Model) bool { return onSecretsTab(m) && m.secretsTab.CanCreateHere() && writable(m) },
		run:   onSecrets((*secrets.Model).BeginCreate)},
	{name: "versions", key: "v", title: "the version list",
		scope: func(m *Model) bool { return onSecretsTab(m) && m.secretsTab.HasVersions() },
		run:   onSecrets((*secrets.Model).OpenVersions)},
	{name: "filter", arg: "pattern", key: "/", title: "filter what is already loaded",
		scope: onSecretsTab, run: onSecretsArg((*secrets.Model).Filter)},
	// Two entries and not one, because "d" takes what the cursor is on and the
	// cursor is on a field in the pane. One command wearing both meanings would
	// have a title that is wrong in one of the two places.
	{name: "delete", key: "d", title: "delete the secret",
		scope: func(m *Model) bool {
			return selectedSecret(m) && m.focus != ui.FocusDetail && writable(m)
		},
		run: onSecrets((*secrets.Model).ConfirmDeleteCurrent)},
	{name: "delete-field", key: "d", title: "remove the selected field from the secret",
		scope: func(m *Model) bool {
			return hasField(m) && m.focus == ui.FocusDetail && writable(m)
		},
		run: onSecrets((*secrets.Model).ConfirmDeleteField)},

	// Policies. The secrets tab's line is labelled "policy-path", so its
	// generated ":copy-policy-path" and this ":copy-policy" never collide.
	{name: "copy-policy", key: "y", title: "copy the policy document",
		scope: func(m *Model) bool { return onPoliciesTab(m) && m.policiesTab.HasDoc() },
		run:   issue(func(m *Model) tea.Cmd { return m.policiesTab.CopyDoc() })},

	// Version list.
	{name: "read", key: "enter", title: "read the selected version",
		scope: func(m *Model) bool { return onVersionList(m) && m.secretsTab.ReadableVersion() },
		run:   onSecrets((*secrets.Model).ReadSelectedVersion)},
	{name: "version", arg: "n", title: "read a version by number",
		scope: onVersionList, run: onSecretsArg((*secrets.Model).ReadVersion)},
	// diff and rollback stay offered when they cannot run yet: both answer with
	// the instruction the user is missing, and marking is not discoverable from
	// a command that has disappeared.
	{name: "diff", key: "d", title: "compare two marked versions",
		scope: onVersionList, run: onSecrets((*secrets.Model).StartDiff)},
	{name: "rollback", key: "b", title: "write an old version back as a new one",
		scope: func(m *Model) bool { return onVersionList(m) && writable(m) },
		run:   onSecrets((*secrets.Model).ConfirmRollback)},
	{name: "undelete", key: "u", title: "restore a soft-deleted version",
		scope: func(m *Model) bool {
			v := m.secretsTab.CurrentVersion()
			return onVersionList(m) && v != nil && v.Deleted() && writable(m)
		},
		run: onSecrets((*secrets.Model).ConfirmUndelete)},
	{name: "destroy", key: "D", title: "erase versions permanently",
		scope: func(m *Model) bool {
			v := m.secretsTab.CurrentVersion()
			return onVersionList(m) && v != nil && !v.Destroyed && writable(m)
		},
		run: onSecrets((*secrets.Model).ConfirmDestroy)},
	{name: "delete-metadata", key: "M", title: "delete the secret and its whole history",
		scope: func(m *Model) bool { return onVersionList(m) && m.secretsTab.HasSelection() && writable(m) },
		run:   onSecrets((*secrets.Model).ConfirmDeleteMetadata)},

	// Auth.
	{name: "role-policy", key: "p", title: "go to the policy this role grants",
		scope: func(m *Model) bool { return onAuthTab(m) && m.authTab.HasRolePolicy() },
		run:   issue(func(m *Model) tea.Cmd { return m.authTab.JumpToSelectedRolePolicy() })},

	// Anywhere. goto and policy are the two jumps and both switch tab on the
	// way: a path or a policy name is an address, and one that only works from
	// the right tab is half an address.
	{name: "goto", arg: "path", title: "jump to a secret path, expanding as it goes",
		scope: always, run: (*Model).runGoto},
	{name: "policy", arg: "name", title: "jump to a policy by name",
		scope: always, run: (*Model).runPolicy},
	{name: "tab", arg: "name", key: "tab", title: "switch tab: secrets, policies, auth, activity",
		scope: always, run: (*Model).runTab},
	{name: "reload", key: "R", title: "refetch what this tab is showing",
		scope: always, run: issue((*Model).reloadCurrent)},
	{name: "token", key: "i", title: "token information",
		scope: always, run: issue((*Model).openToken)},
	{name: "help", key: "?", title: "the contextual key panel",
		scope: always, run: act((*Model).openHelp)},
	{name: "quit", key: "q", title: "leave vivi",
		scope: always, run: func(*Model, string) tea.Cmd { return tea.Quit }},
}

// copyRowNames is every command a Command Reference line can generate. Only
// lookup reads it, so that an unavailable ":copy-metadata-command" says it is
// not available here instead of reading as a typo. A name missing from it costs
// a worse error message and nothing else.
var copyRowNames = []string{
	"copy-path", "copy-policy-path", // the addresses
	"copy-read-command", "copy-field-command", "copy-version-command",
	"copy-metadata-command", "copy-list-command",
	"copy-name", // policies
}

// copyCommands mirrors the Command Reference: one command per line it is
// showing, named for that line and carrying the line itself as its title, so
// the palette answers "what will I get" without a second lookup.
//
// Generated rather than tabled. The panel already decides what is addressable
// here - a folder has a path and a listing and no read - and a second list
// would be a copy of that judgement, kept in step by hand.
func (m *Model) copyCommands() []command {
	rows := m.copyRows()
	out := make([]command, 0, len(rows))
	for _, row := range rows {
		label, text := row.Label, row.Text
		out = append(out, command{
			name:  row.CommandName(),
			title: text,
			scope: always,
			// The banner echoes the line, not the label: none of these carry a
			// value - the panel already has them on screen - and seeing what
			// landed on the clipboard is the whole confirmation.
			run: func(m *Model, _ string) tea.Cmd {
				return m.copyToClipboard("copy "+label, "copied: "+text, text)
			},
		})
	}
	return out
}

// available is what can run right now, in alphabetical order. The palette is
// looked up by name and narrowed by prefix, so the name is what the eye follows
// and a family shares a place: the generated copies land beside ":copy" without
// being put there, and the list does not move under a cursor when the set
// changes with it.
func (m *Model) available() []command {
	out := make([]command, 0, len(commands))
	for _, c := range commands {
		if c.scope(m) {
			out = append(out, c)
		}
	}
	out = append(out, m.copyCommands()...)

	slices.SortFunc(out, func(a, b command) int { return strings.Compare(a.name, b.name) })
	return out
}

// candidates is what an unfinished query could still become, narrowed to what
// can run. A query with a space in it has already named its command, so only
// the word before the space is matched: ":goto kv/" is still a match for goto.
func (m *Model) candidates(query string) []command {
	name, _ := splitCommand(query)

	out := make([]command, 0, len(commands))
	for _, c := range m.available() {
		if strings.HasPrefix(c.name, name) {
			out = append(out, c)
		}
	}
	return out
}

// splitCommand cuts a typed line into the command name and its argument. The
// argument keeps its inner spaces; only the first gap separates the two.
func splitCommand(line string) (name, arg string) {
	line = strings.TrimPrefix(strings.TrimSpace(line), ":")
	name, arg, found := strings.Cut(line, " ")
	if !found {
		return name, ""
	}
	return name, strings.TrimSpace(arg)
}

// resolve turns a typed line into the command to run. Some names are prefixes
// of others, so the order is fixed: exact match, then unique prefix, then
// nothing. Without the first ":copy" is unrunnable; without the last ":d" fires.
func (m *Model) resolve(line string) (command, string, error) {
	name, arg := splitCommand(line)
	if name == "" {
		return command{}, "", emptyLineError{}
	}

	var prefix []command
	for _, c := range m.available() {
		if c.name == name {
			return c, arg, nil
		}
		if strings.HasPrefix(c.name, name) {
			prefix = append(prefix, c)
		}
	}

	switch len(prefix) {
	case 1:
		return prefix[0], arg, nil
	case 0:
		// A command that exists but cannot run here is a different mistake from
		// one that does not exist: "reveal is not available here" is actionable,
		// "unknown command: reveal" sends them hunting a typo that is not there.
		if c, ok := lookup(name); ok {
			return command{}, "", unavailableCommandError{name: c.name}
		}
		return command{}, "", unknownCommandError{name: name}
	default:
		return command{}, "", ambiguousPrefixError{name: name, matches: prefix}
	}
}

// lookup finds a command by exact name regardless of whether it can run. The
// generated copies are matched against their names alone, since here the
// question is only whether the name exists at all.
func lookup(name string) (command, bool) {
	for _, c := range commands {
		if c.name == name {
			return c, true
		}
	}
	for _, generated := range copyRowNames {
		if name == generated {
			return command{name: name}, true
		}
	}
	return command{}, false
}

// The failures resolve reports. They are types rather than sentinel values
// because each carries what the message needs to name, and the command line
// treats one of them differently from the rest.
type emptyLineError struct{}

func (emptyLineError) Error() string { return "no command" }

type unknownCommandError struct{ name string }

func (e unknownCommandError) Error() string { return "unknown command: " + e.name }

type unavailableCommandError struct{ name string }

func (e unavailableCommandError) Error() string { return e.name + " is not available here" }

type ambiguousPrefixError struct {
	name    string
	matches []command
}

func (e ambiguousPrefixError) Error() string {
	names := make([]string, 0, len(e.matches))
	for _, c := range e.matches {
		names = append(names, c.name)
	}
	slices.Sort(names)
	return e.name + " is ambiguous: " + strings.Join(names, ", ")
}

// The argument-taking commands. These are the ones with no keystroke behind
// them, and the reason the palette parses a line instead of matching a name.

func (m *Model) runPolicy(name string) tea.Cmd {
	if name == "" {
		m.notify("policy needs a name; the Policies tab lists them")
		return nil
	}
	return m.jumpToPolicy(name)
}

func (m *Model) runTab(name string) tea.Cmd {
	for i, t := range tabNames {
		if strings.EqualFold(t, name) {
			return m.switchTab(tab(i))
		}
	}
	m.notify("no tab called " + name + ": " + strings.ToLower(strings.Join(tabNames, ", ")))
	return nil
}

// The handlers the table names are methods and not closures written here, so
// the keystroke and the command call the same code. Each lives with the
// component it opens: openVersions in versions.go, openHelp in help.go.
