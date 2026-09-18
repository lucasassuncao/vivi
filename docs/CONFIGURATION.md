# Configuration

vivi has no configuration file, and that is a [non-goal](https://github.com/lucasassuncao/vivi#non-goals), not an omission. Everything it reads comes from the environment, which means a session is described by the shell that started it and nothing survives on disk.

This page is the whole surface: six variables vivi itself reads, three more that only matter when you are working on vivi, and the Vault SDK's transport set, which passes straight through.

## The connection

| Variable | Read by | Meaning |
|---|---|---|
| `VAULT_ADDR` | `vault.ConfigFromEnv` | The server. Required; vivi refuses to start without it. |
| `VAULT_TOKEN` | `vault.ConfigFromEnv` | The token. Optional: see the fallback below. |
| `VAULT_NAMESPACE` | `vault.ConfigFromEnv` | Vault Enterprise namespace. Sent on every request when set, ignored by open-source Vault. |

When `VAULT_TOKEN` is empty, vivi falls back to `~/.vault-token`, the same file `vault login` writes. So `vault login` and then `vivi` works with no token in the environment at all, which is the better habit: a token in a shell variable ends up in the shell's history file and in the environment of every process you start from it.

There is no flag for any of the three. They are the `vault` CLI's own variables, and vivi reading the same ones is what lets you switch tools mid-session without re-describing the connection.

### TLS and proxies

vivi builds its client from `api.DefaultConfig()`, so every transport variable the Vault SDK honours works here without vivi knowing about it: `VAULT_CACERT`, `VAULT_CAPATH`, `VAULT_CLIENT_CERT`, `VAULT_CLIENT_KEY`, `VAULT_TLS_SERVER_NAME`, `VAULT_SKIP_VERIFY`, `VAULT_CLIENT_TIMEOUT`, `VAULT_MAX_RETRIES`, `VAULT_HTTP_PROXY`, and the rest of the set the `vault` CLI documents. They are not vivi's, they are not listed above, and they behave exactly as they do for the CLI.

`VAULT_SKIP_VERIFY=true` turns off certificate verification for the whole session. It is the right answer for a self-signed sandbox you own and the wrong one everywhere else, and vivi shows no warning when it is set, because the SDK never tells it.

## Preferences

| Variable | Values | Meaning |
|---|---|---|
| `VIVI_THEME` | any name from `vivi --list-themes` | Replaces the adaptive palette with a fixed one. |
| `VIVI_READ_ONLY` | `on`/`true`/`yes`/`1`/`always`, `prod`/`production`, `off`/`false`/`no`/`0`/`never` | Refuses every write, or only on a server that is not a recognised sandbox. |

Both have a flag that wins over them: `--theme` and `--read-only`. An unknown value for `VIVI_READ_ONLY` is an error at startup rather than a silent `off`, because a typo in a shell profile must not quietly hand back write access.

`VIVI_READ_ONLY=prod` in a shell profile is the one worth setting. It is a decision made calmly, in an editor, rather than with the cursor already on a production path.

## `VIVI_DEBUG`

| Variable | Values | Meaning |
|---|---|---|
| `VIVI_DEBUG` | a file path | Records every keystroke to that file, created `0600` and appended to. |

A TUI has no stdout to print to: it is drawing on it. So when a key does nothing, or a keyboard layout sends something unexpected, this is the only way to see what actually arrived.

**It records what you type, and what you type includes secret values.** A new field's value, an edited one, and on Windows a pasted certificate or password (a paste arrives as keystrokes there) all land in the file in full, beside the typed confirmations. It is off unless you name a path, the file is created `0600`, and nothing else in vivi writes secret content anywhere. Turn it on to answer a keyboard question, and delete the file afterwards.

```bash
VIVI_DEBUG=/tmp/vivi-keys.log vivi
```

## Working on vivi

These are for contributors; vivi never reads them.

| Variable | Read by | Meaning |
|---|---|---|
| `VIVI_INTEGRATION` | `internal/vault` tests | Unset, the integration suite skips. `make test-integration` sets it. |
| `VAULT_FORWARD_PORT`, `VAULT_FORWARD_TARGET` | `.devcontainer/vault-forward.sh` | Override the port and host the devcontainer forwards to. Probed automatically otherwise. |

The integration suite also refuses any `VAULT_ADDR` that is not loopback, whatever `VIVI_INTEGRATION` says: it creates and deletes real paths. See [DEVELOPMENT.md](DEVELOPMENT.md).

## A starting point

[`.env.example`](https://github.com/lucasassuncao/vivi/blob/main/.env.example) at the repository root points at the throwaway Vault, and lists every other variable on this page commented out, so the set is visible without leaving the file. Copy it to `.env` if your shell or editor loads one; vivi itself reads only the environment, never the file.

## Themes


By default the palette is adaptive: it follows the terminal background, so it reads on light and dark alike. Naming a theme replaces it with a fixed set of colours, drawn from [yedit](https://github.com/lucasassuncao/yedit)'s registry of 56.

```bash
vivi --list-themes
vivi --theme grape
export VIVI_THEME=mint    # vivi has no config file; the environment is the preference
```

One role is never taken from the theme: the warning colour. A soft-deleted version can be undeleted and a destroyed one cannot, so those two must not look alike, and yedit's palette has no colour that means "reversible damage". That role keeps the built-in amber under every theme.

There is deliberately no `vivi get` or `vivi ls`: the `vault` CLI already covers scripting. There is no `vivi doctor` either: every run checks the address, the token and the server that answers before drawing anything, and what your token may do at a given path is a section of the detail panel rather than a command of its own.
