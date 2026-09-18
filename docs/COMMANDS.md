# Commands

vivi has two ways to run the same action: a keystroke, and a colon command.

The keystrokes are the fast path and nothing about them changes. The colon commands exist because a keystroke has to be known before it can be used, and this is an app for someone who does not open Vault every day. Pressing `:` opens a command line that lists what can be run *right now*, with the keystroke printed beside each entry, so the slow path teaches the fast one.

Colon commands also carry the two things a keystroke cannot: arguments, and the rare operations that never earned a key of their own.

```
─────────────────────────────────────────────────────────────────────
  :copy                    y   copy the secret, or one field
  :copy-path                   kv/app/prod/db
  :copy-policy-path            kv/data/app/prod/db
  :copy-read-command           vault kv get -mount=kv app/prod/db
:cop█
[enter] run  •  [tab] complete  •  [↑/↓] choose  •  [esc] cancel
```

## Using the command line

| key | effect |
|---|---|
| `:` | open it, listing everything available right here |
| any text | narrow the list by prefix |
| `↑` / `↓` | move through the candidates |
| `tab` | complete to the highlighted candidate |
| `enter` | run the highlighted candidate, or what was typed |
| `esc` | close, changing nothing |

### In what order

Alphabetically, always. The list is read by name and narrowed by prefix, so the name is what the eye follows, and a family ends up together without anyone placing it: `:copy-path` and the rest of the generated copies sit under `:copy`. It also keeps the list still. The generated ones change with the cursor, and an order by importance would have them appearing in the middle of the list, moving the entry under the highlight.

### What gets offered

One rule decides it:

> Offer a command when it can do something, **or** when it can explain what is missing. Hide it only when it is meaningless here.

Both halves matter. Offering what cannot work teaches a keystroke that will not work either, the way `:delete` on a folder used to. But hiding everything that cannot run right now is its own failure: a `:diff` that vanishes until two versions are marked never gets to say *"mark exactly two versions with space"*, and nobody learns that marking exists.

So `:delete` is absent on a folder, and `:diff` is present with nothing marked.

### Where `:` does not open

Inside the editor and inside the filter, `:` is a literal character: it is part of the value or the pattern being typed. The command line opens from the browser and from the version list only.

## Name resolution

Some names are prefixes of others (`:copy` of `:copy-path`, `:delete` of `:delete-metadata`). The rule is fixed:

1. an exact match wins, always;
2. otherwise a unique prefix runs;
3. otherwise nothing runs and the candidates stay on screen.

So `:copy` is never ambiguous, `:cop` is, and `:del` resolves to nothing while `:delete` is exact.

## Reference

### Anywhere

| command | key | notes |
|---|---|---|
| `:help` | `?` | the contextual key panel |
| `:token` | `i` | token information, where it can also be renewed |
| `:reload` | `R` | refetch what the current tab is showing |
| `:tab <name>` | `tab` / `shift+tab`, `1`–`4` | `secrets`, `policies`, `auth`, `activity` |
| `:goto <path>` | none | jump to a secret path, switching tab on the way |
| `:policy <name>` | none | jump to a policy by name, switching tab on the way |
| `:quit` | `q` | leave vivi |

### Secrets

| command | key | available when |
|---|---|---|
| `:reveal` | `r` | a field is selected |
| `:copy` | `y` | a secret is loaded (the whole secret from the list, the selected field from the pane) |
| `:copy-<line>`, `:copy-<line>-command` | none | one per line the Command Reference is showing, see below |
| `:versions` | `v` | the mount keeps history and the secret has some |
| `:edit` | `e` | a secret is loaded |
| `:create` | `a` | anywhere in a KV mount (a secret means: beside it) |
| `:delete` | `d` | the cursor is on a secret |
| `:filter <pattern>` | `/` | always, on this tab |

#### The Command Reference lines

The panel in the bottom right prints the addresses and commands that reach what the cursor is on, one labelled line each. Every line it shows has a command that copies it, named for the line and for what the line holds: an address copies as `:copy-path` or `:copy-policy-path`, a CLI command says so, `:copy-read-command`, `:copy-field-command`, `:copy-version-command`, `:copy-metadata-command`, `:copy-list-command`. The suffix is the whole difference: `:copy-path` puts `kv/app/prod/db` on the clipboard, `:copy-list-command` puts `vault kv list -mount=kv app/prod` there, not what that command would print. The palette lists the line itself as the title, so what you will get is on screen before you press enter. The policy line is `policy-path` and not `policy` so that its command never collides with `:copy-policy` on the Policies tab, which copies a document rather than an address.

They are generated from the panel rather than listed beside it, so the two cannot drift. That also means the set changes with the cursor: a folder has a path and a listing and nothing else, so it offers `:copy-path` and `:copy-list-command` and no others. A secret being created, which does not exist on the server yet, offers the path and the folder it will land in.

The clipboard is not the same act as selecting the text with a mouse, and it is the one that works over ssh.

`:goto` has no keystroke behind it, and is the reason the command line parses arguments at all. It walks a full path (`kv/app/prod/db`) expanding each level as the listing arrives, which is several seconds of arrow keys reduced to one line. A path that turns out not to exist, or that stops at a folder the token may not list, says so and leaves the cursor where it was.

The trailing slash is part of the path. Vault lets a directory and a secret share a name (a listing can answer `["app", "app/"]`, and both are real) so `:goto secret/app` goes to the secret and `:goto secret/app/` to the directory. When only one of the two exists, either form finds it.

### Policies

| command | key | available when |
|---|---|---|
| `:copy-policy` | `y` | a policy document is open |

Typing `:copy` here works: it names no command on this tab, but it is a unique prefix of one that does.

### Auth

| command | key | available when |
|---|---|---|
| `:role-policy` | `p` | the selected role grants a policy |

### Version list

Reached with `:versions` or `v`.

| command | key | available when |
|---|---|---|
| `:read` | `enter` | the selected version is readable |
| `:version <n>` | none | version `n` exists and is readable |
| `:diff` | `d` | always (says so when two are not marked) |
| `:rollback` | `b` | always (says so when there is nothing to restore) |
| `:undelete` | `u` | the selected version is soft-deleted |
| `:destroy` | `D` | the selected version is not already destroyed |
| `:delete-metadata` | `M` | a secret is selected |

The destructive ones still go through the confirmations they always did. `:destroy` and `:delete-metadata` are irreversible in Vault, and typing the name out is not treated as consent.

## Adding a command

Commands live in one table, `commands` in `internal/tui/command.go`. An entry carries its name, the keystroke that already runs it, a one-line title, the predicate that decides whether it is available, and the function that runs it.

The candidate list, the completion and the `?` panel's command section all read that table, so a new command appears in all three by being added once. Where in the table it goes does not matter: the palette sorts by name, and the table is grouped by tab only so it reads well. What a new entry must not do is duplicate logic: `run` should call the same method the keystroke calls, so the two paths cannot drift into doing different things.

## Read-only sessions

`--read-only`, `--read-only=prod`, or `VIVI_READ_ONLY` refuses every operation that changes the Vault. The commands below vanish from the palette, because the rule at the top of this page still holds: an entry that does nothing when chosen teaches a keystroke that will also do nothing. What explains the absence is the `READ-ONLY` badge in the header, which is on screen the whole time.

| command | key | refused when read-only |
|---|---|---|
| `:edit` | `e` | yes |
| `:create` | `a` | yes |
| `:delete` | `d` | yes |
| `:rollback` | `b` | yes |
| `:undelete` | `u` | yes |
| `:destroy` | `D` | yes |
| `:delete-metadata` | `M` | yes |

Everything else works unchanged, including `:versions`, `:diff`, `:copy` and the token panel's `n`: renewing your own token is not an operation on the Vault, and a session that cannot renew is one that ends in the middle of the reading it was opened for.

Pressing a refused key says so and names the reason. The `?` panel keeps listing them under a banner rather than hiding them: it is the complete reference, and a reference that quietly omits half the app is worse than one that says which half is closed.

A session becomes read-only from `--read-only` or `VIVI_READ_ONLY`; the values each accepts are in [CONFIGURATION.md](CONFIGURATION.md).
