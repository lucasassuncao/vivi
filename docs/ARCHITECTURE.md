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

Everything about being a terminal program: the tree, the panes, the modes, the key table and the rendering. It is not one package but a parent and five below it:

```
internal/tui            the parent: the session, the shell, the keys, the commands
├── secrets             the Secrets tab
├── policies            the Policies tab
├── auth                the Auth tab
├── activity            the Activity tab
└── ui                  what the parent and a tab both have to speak
```

The chrome is not vivi's. The header row, the tab strip, the panes' frames, the legend, the toasts and the overlay stack are drawn by bezel's `shell`, which the parent builds in `newShell` and configures: the layout, the tabs, the theme, the header lines, the write filter. vivi answers every key itself, so every action it hands the shell is display only: there to be printed in the legend and the palette, never to be run by the shell.

Neutralising ANSI escapes and bidirectional overrides is still a *terminal* concern, and so still the adapter's (a web front end would need HTML escaping instead, not this). It is done by bezel's `draw.Sanitize` on everything that arrives from the server, at the point it is drawn.

### A tab is a package

A tab used to be a file in one package, which bought navigability and nothing else: everything still saw everything, so a tab could reach into the parent's fields, and the boundary held only as long as nobody needed to cross it. As a package, a tab cannot see the parent at all. The Go import graph makes it so: the parent imports every tab, so a tab importing the parent would be a cycle the compiler refuses.

What a tab is to the parent is the `Tab` interface in `tab.go`: what it loads, what it draws in each pane, what keys it takes, what its legend, help and Command Reference say, and what it floats over the panes. Every method there was a `switch m.tab` arm before it was a method. `tabAdapter` in `legend.go` turns a `Tab` into what bezel's shell asks for.

Since a tab cannot write the parent, everything it wants from outside itself goes one of two ways:

- **A message**, for what the parent owns: a toast (`BannerMsg`), the clipboard (`CopyMsg`), the focus, the detail pane's scroll. The tab returns a command that emits one, and the parent folds it.
- **The `Requester`**, for the server. It is injected rather than asked for by message because the answer has to come back to the tab that asked; the parent only wraps the call in the session's context, the timeout and the count of calls in flight.

### internal/tui/ui: the shared vocabulary

What the parent and a tab both have to name lives in `ui`, because it can live in neither: the parent imports every tab, and a tab cannot import the parent.

| file | is |
|---|---|
| `context.go` | the session's view of itself that every call hands a tab: focus, the body's size, styles, the token's policies, whether it may write |
| `messages.go` `request.go` | the two ways a tab asks for what it does not own |
| `legend.go` | the shapes a legend is built from: `ListLegend`, `ScrollLegend`, `Actions` for the second row, `CapWrite` for what a read-only session drops |
| `styles.go` `shell.go` | vivi's styles, and the bezel theme dressed in them so the shell's chrome wears the tabs' colours |
| `panel.go` `modal.go` `text.go` `values.go` | the furniture every pane is built from: the narrow limit, the modal box, the mask, how a value and a failure read on screen |

### The parent

| file | is |
|---|---|
| `model.go` | the session's state, `Update`, the shell it builds, and what bounds a request |
| `tab.go` | the `Tab` interface, and the last switch over the tabs, which is the dispatch table itself |
| `update.go` | what the loop does with an answer: the token is the parent's, everything else is offered to the tabs |
| `keys.go` | where a keystroke goes: the mode decides, then the tab |
| `layout.go` | how the terminal is divided, decided without rendering anything, and the Command Reference sliding away on `c` |
| `view.go` | the smallest terminal vivi draws into, and the frame |
| `header.go` | the lines above the panes: which server this is, and how dangerous it is |
| `legend.go` | the tabs as the shell sees them: their names, their status, their keys |
| `command.go` `actions.go` | the table of `:` commands, and the same table as bezel actions for the palette |
| `copy.go` | the Command Reference panel at the bottom of the right column |
| `help.go` `token.go` `overlays.go` | the panels that open over any tab: `?` and the token |
| `readonly.go` | how a refusal is worded, and the header's half of the read-only gate |
| `cmds.go` `text.go` | the parent's own traffic and timers: the token's countdown, the clock, the toast's life |
| `crash.go` `debug.go` | the report a panic leaves, and the keystroke log `VIVI_DEBUG` asks for |

`layout.go` and `legend.go` stay in the parent on purpose. They read every tab's context and divide the screen for whichever is in front: they are cross-cutting by design, and pushing them into the tabs would put one rule in four places.

### The tabs

**secrets** is the large one, and the only one that writes, which is why the read-only gate that every write passes through (`mutate`, in its `readonly.go`) lives there and not in the parent. It is one file per component, each holding what it draws, what keys it answers and what it asks the server for:

| file | is |
|---|---|
| `model.go` `tab.go` | the tab's state and its modes, and what it answers the parent |
| `update.go` `cmds.go` | the server's answers, and every request as a `tea.Cmd` |
| `tree.go` | the lazy forest and the rows it draws |
| `secrets.go` | the detail pane: data, versions, metadata, permissions |
| `edit.go` | the editor: its table, its pane, and the save |
| `field.go` | the form one field is typed in, the one place a value is typed |
| `versions.go` | the version list, and the operations that only exist there |
| `diff.go` | the comparison of two versions |
| `confirm.go` `modal.go` | the pending destructive action, its friction graded by `app/safety.go`, and its box |
| `copy.go` | what `y` copies, and the Command Reference's rows |
| `legend.go` `palette.go` | the keys the footer offers, and what the palette needs to know and run |
| `goto.go` | walking to a path for `:goto` |
| `inspect.go` `crash.go` | what the tab says about itself to the tests that drive the whole app, and to a crash report |

**policies**, **auth** and **activity** are a file each (`policies` has `hcl.go` beside it, the small lexer that colours a policy document). Activity loads nothing: it is the log of what this session wrote, built from the writes as they happen.

### Tests

Every file with behaviour worth pinning has a `_test.go` beside it. The ones without are the small drawing helpers and the shared vocabulary in `ui`, which the component tests exercise through the frames they render. Three kinds of test file are deliberately not paired with a component, and the parent and `secrets` each have their own of the first and the last:

- **`harness_test.go` contains no tests at all.** It is the driver: it runs the model synchronously, feeding every command's message back so an assertion can read the state after the server has answered. It drops timers (the toast's frames and expiry, the clock, the caret's blink): feeding one back would re-arm it and the run would never settle.
- **`invariants_test.go` pins the architecture**, the way `internal/app/architecture_test.go` does for the layering: View never writes to the model, and the update loop is total over its own messages. These fail when a design rule is broken, not when a feature is.
- **`panics_test.go` is a regression suite.** Each test is a crash that happened once (a mode with no payload, a stale command cursor, a negative height) kept so it cannot happen twice. It is named for the symptom because the cases have nothing else in common.

The smaller tabs are tested in two places: in their own package for what they draw, and in the parent (`policies_test.go`, `auth_test.go`, `activity_test.go`) for the tab inside the whole app: reached by its key, jumping to another tab, showing what a write in Secrets left in the log.

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
