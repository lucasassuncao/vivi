# Keys

Navigation is arrow keys and `enter`, everywhere. Letters are only ever actions, so nothing moves by accident. The horizontal arrows read as depth: `→` goes further in and `←` comes back out, whether that means a folder or the pane beside it.

| Key | Does |
| --- | --- |
| `↑` / `↓` | move |
| `→` | go further in: open a folder, or step into the detail pane |
| `←` | come back out: close a folder, or leave the detail pane |
| `enter` | the same as `→`, wherever the cursor is |
| `home` / `end` | top / bottom |
| `tab` / `shift+tab`, `1`-`4` | switch tab: next, previous, or by number |
| `/` | fuzzy filter over what is already loaded |
| `r` | reveal, without copying: every field of the secret from the list, the selected field from the detail pane |
| `y` | copy, without showing it: the whole secret as JSON from the list, the selected field from the detail pane |
| `e` | edit the secret |
| `a` | create a secret here |
| `v` | version list |
| `d` | delete the secret (in the version list, `d` diffs instead) |
| `p` | on the Auth tab: go to the policy the selected role grants |
| `R` | reload what this tab is showing |
| `i` | token information  ·  `n` renew the token, in that panel |
| `:` | run any of these by name, listed as you type |
| `?` | help  ·  `q` quit |

`?` lists the keys for the tab you are on, plus the ones that work anywhere. The legend under the panes is contextual too, and drops the rare actions first when the terminal is narrow; `?` is the key it never drops.

## The editor

`e` opens the selected secret for editing and `a` starts a new one, asking first for its name in a small modal that shows the full path as you type; a slash in the name makes folders, and `enter` there only opens the editor, writing nothing. Both take over the detail pane with the same table the pane draws read-only. The table is for moving and removing; a value is typed in a form that opens over it, the same form for a new field and an existing one. The badge on the `data` rule says which of the two the keyboard is in, `NAVIGATING` or `EDITING A FIELD`, because in the form `a` and `x` are letters going into the value.

| Key | Does |
| --- | --- |
| `↑` / `↓` | move between fields |
| `enter` | open the selected field in the form |
| `a` | add a field: the form opens on its name |
| `x` | remove the selected field |
| `ctrl+s` | save, after a confirmation that lists every change |
| `esc` | cancel. With changes pending it asks first |

### The field form

```plain
╭────────────────────────────────────────────────────────────────────────────────────╮
│  Add field to kv/app/prod/db                                                       │
│                                                                                    │
│    key    tls_cert                                                                 │
│    type   ● string    number    bool    json                                       │
│    value    1 │ -----BEGIN CERTIFICATE-----                                        │
│             2 │ MIIDdzCCAl+gAwIBAgIEAgAAuTANBgkqhkiG9w0BAQUFADBaMQswCQYDVQQGEwJJRT │
│             3 │ -----END CERTIFICATE-----                                          │
│             4 │ ~                                                                  │
│                                                                                    │
│    writes "tls_cert": "-----BEGIN CERTIFICATE-----\nMIIDdzC…   3 lines · 145 B     │
│                                                                                    │
│  [enter] new line   [tab] move   [ctrl+s] add   [esc] cancel                       │
╰────────────────────────────────────────────────────────────────────────────────────╯
```

The form has a width of its own, wide enough for a PEM line beside the gutter, and stays that size on a wider terminal. Three rows, `tab` and `shift+tab` between them, and the hint at the bottom says what `enter` does on the row you are on:

| Row | Keys |
| --- | --- |
| `key` | type the name; `enter` moves on to the type |
| `type` | `←` / `→` choose `string`, `number`, `bool` or `json`; `enter` moves on to the value. Nothing else changes it |
| `value` | type or paste; `enter` is a line break, always |
| any | `ctrl+s` adds or applies the field, `esc` closes the form without |

`enter` never accepts the value, even a one-line one, and that is deliberate. A paste arrives whole, as one message the terminal brackets and vivi hands to the value area, so a certificate lands in a single keystroke's worth of work however long it is. But a terminal that cannot bracket a paste sends it as typing instead, each newline as an `enter`, and with `enter` accepting, the first line of a pasted `.env` would close the form and the rest would land on the table, where `a` opens a form and `x` removes a field. With `enter` a line break and the type row deaf to letters, whatever is pasted, wherever the focus is, stays inside the form for you to look at.

The value is a text area, not a line: four numbered rows at least, more as the value grows, scrolling once it outgrows the terminal. A certificate pasted in keeps every one of its lines, the `writes` row counts them, and the table shows `•••••••••  52 lines` for it afterwards. The row used to be a one-line input that quietly replaced every newline with a space, which is the reason the form exists. Opening an existing field shows its key and type but offers neither: the type is the server's.

### Types

Vault stores JSON, so a field is a string, a number, a bool or a nested object, and the application reading the secret often cares which. The editor never guesses from the value: a digit-only password stays a string, and a `port` you meant as a number is a number only because you said so.

* **A key from the server keeps the type the server gave it.** Opening `port: 5432` and typing `5433` writes the number `5433`, not the text; typing `abc` there is refused in the form, with `✗ expected a number, got "abc"`, before anything is saved. Untouched fields are written back exactly as they arrived, so opening the editor never flattens a number or an object into text.
* **A field added in this edit starts as a string** and the form's `type` row is where that changes. The `writes` row shows the JSON the save will write, live, which is the whole difference in one line: `"port": "30"` against `"port": 30`. The type is printed at the end of the table row for every added field, and for every field from the server that is not a plain string.
* **The confirmation repeats the choice**: an added key shows as `+ port (number)`, the last look before it is written. A value that does not fit its type never reaches that modal: `007` is a string until the type row reads `number`, at which point the form refuses it, because it is not one.

Numbers are kept as the digits you typed rather than parsed, so an integer past 2^53 round-trips without rounding. `json` accepts any valid JSON value, laid out over as many lines as you like; the form numbers them, so a syntax error can say which line it is on.

## Commands

Every action above also has a name. `:` opens a command line listing what can run right now, with the keystroke printed beside each entry, so finding an action by name is also how you learn the key for it. Some entries have no key beside them. Three carry an argument no keystroke could: `:goto kv/app/prod/db` walks straight to a path, expanding each level on the way, `:policy <name>` jumps to a policy by name, and `:version 3` jumps to a version by number. The rest are the `:copy-<line>` family, one per line the `Command Reference` panel is showing: there are more of them than there are letters, which is why they live only in the palette.

[COMMANDS.md](COMMANDS.md) is the full reference.

## The detail pane

The right column is two panels. The top one is the detail; the `Command Reference` panel under it prints the address of whatever the cursor is on and the `vault` commands that reach it: the plain read, the policy path a rule has to name to grant it, the `-field` form that prints one value with no JSON around it, the `-version` form while a historical version is open, the metadata read, and the listing of the folder it sits in. Nothing there is a secret value: paths and commands only, so it can stay on screen while values stay masked above it.

The panel takes a fixed share of the column (about a fifth, never less than its six rows and never more than half) rather than the height of whatever it happens to hold. Sized to its contents it moved every time the cursor did, and the detail panel above it resized with it.

It is named for what it holds rather than for a key, and it was not always. The title read `Copy` while `Y` and `C` were the keys that copied the path and the command to the clipboard and showed you neither, so the only way to check what you had was to paste it somewhere. Both keys are gone, and printing the lines instead makes them readable and selectable with the mouse. Nothing in the panel is copied for you: the one key that still copies, `y`, acts on the panel above, so a title promising an action no key there performs had outlived what it described. Each line does have a command that copies it (`:copy-path`, `:copy-read-command` and the rest, generated from whatever the panel is showing) for putting one on the clipboard in one go, which a mouse selection cannot do over ssh.

The detail panel is the one that scrolls: content longer than the terminal, a policy document or a secret with many keys, scrolls instead of overflowing. Move into the pane with `enter` and drive it there:

| Key | Does |
| --- | --- |
| `↑` / `↓` | move between the secret's fields, or scroll the document |
| `home` / `end` | first / last |
| `pgup` / `pgdn` | half a page |
| `esc` / `←` | back to the list |

The focused pane wears the accent border and an inverted title, and the footer shows how far into the document you are. On a secret the arrows walk the fields rather than scrolling by line, because the field under the cursor is what `r` and `y` act on. `y` follows that focus: from the list it takes the whole secret, since the cursor there is on a secret and not on a field.

In the version list (`v`):

| Key | Does |
| --- | --- |
| `↑` / `↓`, `home` / `end` | move |
| `space` | mark a version |
| `enter` | read the marked version |
| `d` | diff two marked versions |
| `↑` / `↓`, `r` | *inside the diff:* move, reveal every row at once |
| `b` | rollback: writes a **new** version, deletes nothing |
| `u` | undelete: reverse a soft delete, same version |
| `D` | destroy: erase a version's data, irreversible |
| `M` | delete the secret and its whole history, irreversible |

## `b`, `u`, `D`, `M`: four words for four different things

All four sound like some flavour of undo or delete, and none of them is a milder setting of another. On KV v2 they separate on what they touch and on what is left behind:

| Key | Where | Touches | Data comes back? | Still in the listing? |
| --- | --- | --- | --- | --- |
| `d` delete | tree / detail | one version, the one on screen, marked deleted | yes, with `u` | **yes** |
| `u` undelete | version list | one soft-deleted version | yes: it *is* the undo | yes |
| `b` rollback | version list | nothing: writes a new version | nothing was lost | yes |
| `D` destroy | version list | one version's data, erased | no | **yes** |
| `M` delete all | version list | every version **and** the metadata | no | **no** |

Note the `Where` column: soft delete is `d` pressed on the tree or in the detail pane. Inside the version list (`v`) the same key is **diff**, and the four operations above are the only ones that pane answers.

The pair worth reading twice is `D` and `M`. "Destroy" sounds like the end of the story and it is not: Vault answers `LIST` out of a secret's *metadata*, and destroy does not touch the metadata. So a destroyed secret stays in the tree: listed, selectable, and empty:

```plain
│     ▸ app/                       ││ not found
│ ●     app        ← still here    ││ this version was deleted or destroyed
│       types                      ││
│                                  ││ VERSIONS
│                                  ││   [ ] v1  2026-08-27 00:36  ✗ destroyed
```

Only `M` removes the metadata, and so only `M` makes the path disappear. The same holds at the CLI, which is the quickest way to convince yourself:

```console
$ vault kv destroy -mount=secret -versions=1 app
$ vault kv list -mount=secret /
app          # ← still listed, with nothing behind it

$ vault kv metadata delete -mount=secret app
$ vault kv list -mount=secret /
             # ← gone
```

`b` is the odd one out and the safe one: rollback reads an old version and writes its content back as a **new** version. It deletes nothing, so the history only ever grows, which is why it asks `y/n` while `D` and `M` make you type the path out.

On **KV v1** none of this applies. There are no versions to soft-delete, undelete, destroy or roll back, so `d` is the only one of the five that exists and it is immediate and final, which is why it gets the typed confirmation that `d` on a v2 mount does not need.
