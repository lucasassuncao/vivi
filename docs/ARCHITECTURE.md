# Architecture

vivi is ports and adapters. The dependency arrows only ever point inward.

```
                    ┌─────────────────┐
                    │  internal/cmd   │   composition root: cobra, flags,
                    │                 │   preflight, wiring
                    └────────┬────────┘
                             │
              ┌──────────────┴──────────────┐
              ▼                             ▼
    ┌──────────────────┐          ┌──────────────────┐
    │  internal/tui    │          │ internal/updater │
    │ driving adapter  │          │  self-update     │
    │ bubbletea        │          └──────────────────┘
    └────────┬─────────┘
             │
             ▼
    ┌──────────────────┐
    │  internal/app    │   the rules. no terminal, no cursor, no styles.
    └────────┬─────────┘
             │
             ▼
    ┌──────────────────┐          ┌──────────────────────┐
    │ internal/vault   │◄─────────│ internal/vault/fake  │
    │ port + SDK       │          │ driven adapter, tests│
    │ adapter          │          └──────────────────────┘
    └──────────────────┘
```

Two adapters implement the same port, and the whole test suite runs against the in-memory one. That is the property ports and adapters exists to buy, and it was true before the layout had names for it.

Only the arrows that constrain anything are drawn. `internal/cmd` also reaches `app` and `vault` directly (it resolves the read-only policy against the address and runs preflight before the browser opens) but a composition root touching every layer is what makes it the composition root, so those two arrows say nothing a reader has to check.

## internal/app: the inside

The rules that are true about operating a Vault, whoever is asking and however the answer is drawn:

| file | decides |
|---|---|
| `environment.go` | which server is dangerous, and how loudly to badge it |
| `access.go` | whether this session may write at all, and what to say when it may not |
| `versions.go` | which versions an operation applies to, and how to name them |
| `write.go` | the check-and-set value a write carries; whether a version may be edited |
| `paths.go` | the path a policy has to name to grant a secret |
| `policy.go` | which of the token's policies actually reach a path |
| `safety.go` | how much friction each operation earns before it runs |

`safety.go` is the reason the package exists. *"Destroy needs the path typed out; delete on a v2 mount does not; delete on a v1 mount does"* is a policy about Vault, not about Bubbletea, and it used to be spelled out five separate times in the code that builds modals, which is how a policy quietly stops being one. It is now one graded function with a table test, and the modals ask it.

Nothing here imports a terminal library. `TestAppNeverImportsAnAdapter` enforces that by name, including the package's own test imports: a rule that can only be tested through a Bubbletea harness is a rule with a Bubbletea dependency.

## internal/tui: the driving adapter

Everything about being a terminal program: the tree, the panes, the modes, the key table, the rendering, and `safe.go`.

`safe.go` stays here on purpose. Neutralising ANSI escapes and bidirectional overrides is a *terminal* concern (a web front end would need HTML escaping instead, not this) so it is an adapter's duty to its own medium, not a rule about Vault.

### One file per component

The package is the unit of encapsulation in Go, so splitting it into files buys navigability rather than isolation: everything here still sees everything. What it buys is that a component is one file: what it draws, what keys it answers, and what it asks the server for, in the order you read them.

`view.go` and `model.go` used to be two thousand and fourteen hundred lines and held every tab between them, so a change to the Auth pane meant three hops. The cut below is what they became.

| file | is |
|---|---|
| `view.go` | the dispatcher: which component is on screen right now, and nothing else |
| `model.go` | the state, `Update`, and what bounds a request |
| `update.go` | what the loop does with an answer, including the stale ones |
| `keys.go` | where a keystroke goes: the mode decides, then the tab |
| `layout.go` | how the terminal is divided, decided without rendering anything |
| `header.go` | the two lines above the panes |
| `legend.go` | the two lines below them |
| `secrets.go` `policies.go` `auth.go` `activity.go` | one tab each: its panes and its keys |
| `tree.go` | the lazy forest and the rows it draws |
| `edit.go` | the field editor: its model, its pane, and the save |
| `field.go` | the form one field is typed in: key, type, a text area for the value, and what the save will write |
| `versions.go` | the version list, and the four operations only it carries |
| `diff.go` | the comparison |
| `copy.go` | the Command Reference panel, and what reaches the clipboard |
| `command.go` `cmdline.go` | the table of commands, and the line that runs them |
| `help.go` `token.go` `modal.go` `confirm.go` | the things that float over a tab |
| `panel.go` `overlay.go` `text.go` `styles.go` `safe.go` `hcl.go` | the shared furniture |
| `readonly.go` | the one gate every write passes, and how a refusal is worded |
| `crash.go` `debug.go` | the report a panic leaves, and the keystroke log `VIVI_DEBUG` asks for |
| `cmds.go` | every request, as a `tea.Cmd` |

Two of those are deliberately *not* distributed into the components. `layout.go` divides the column for whichever tab is in front, and `legend.go` reads the context of every one of them: they are cross-cutting by design, and splitting them across four files would put one rule in four places.

There is no `helpers.go`, and there was an `actions.go`: six hundred lines holding the editor's keys, the diff's keys, the version list's four operations and the modal's key handler, which is the drawer this layout exists to prevent. Each of those went to the component it belongs to. A file named for what it contains can be found; a file named for what it is not cannot.

Every file with behaviour worth pinning has a `_test.go` beside it, and the tests moved with the code they cover. The ones without are the small drawing helpers (`confirm.go`, `crash.go`, `debug.go`, `field.go`, `modal.go`, `overlay.go`, `text.go`), which the component tests exercise through the frames they render. Three test files are deliberately not paired with a component, and they are three different things rather than one:

- **`harness_test.go` contains no tests at all.** It is the driver: it runs the model synchronously, feeding every command's message back so an assertion can read the state after the server has answered. Every other test file depends on it.
- **`invariants_test.go` pins the architecture**, the way `internal/app/architecture_test.go` does for the layering: View never writes to the model, and the update loop is total over its own messages. These fail when a design rule is broken, not when a feature is.
- **`panics_test.go` is a regression suite.** Each test is a crash that happened once (a mode with no payload, a stale command cursor, a negative height) kept so it cannot happen twice. It is named for the symptom because the cases have nothing else in common.

## internal/vault: the port, and one adapter

`Client` is the port: eighteen methods, plain domain types (`Secret`, `Mount`, `VersionInfo`, `Metadata`), and sentinel errors. `apiClient` is the driven adapter over the HashiCorp SDK. `fake` is the second one.

### The one thing still mixed

The port and its SDK adapter share a package, so anything depending on the port links the HashiCorp SDK, including `internal/app`, which has no business knowing that SDK exists.

Splitting them is mechanical: `Client`, the types and the errors stay in `internal/vault`; `apiClient` and everything importing `hashicorp/vault/api` moves to `internal/vault/api`. Nothing else changes, because the interface is already the only thing anyone depends on.

It is deliberately not done yet. On its own it is cosmetic, since the arrows already point the right way, and it is worth doing when it buys something concrete, such as a second driven adapter that is not the SDK.

## The rule, as a test

`internal/app/architecture_test.go` checks both directions:

- **app never imports an adapter**: not `tui`, not charmbracelet, not muesli, not the clipboard.
- **tui still imports app**: a front end that has stopped asking for the rules has started re-deciding them, which is the failure this whole layout exists to prevent, arriving quietly.

A layering rule in a README is a layering rule until the first afternoon someone needs one function from the other side.

The adapter the fake stands in for is exercised against a real server by the integration suite; [DEVELOPMENT.md](DEVELOPMENT.md) is what it runs against.
