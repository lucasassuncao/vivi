<!-- markdownlint-disable MD033 -->
<p align="center">
  <img src="docs/pages/image.png" alt="VisualVault logo" width="313" height="313">
</p>

<h1 align="center">VisualVault</h1>

<p align="center">
or <code>vv</code> (<code>vivi</code>) for short
</p>

<p align="center">
  <strong>An interactive terminal browser for HashiCorp Vault.</strong><br>
  The parts of the web UI you actually use, without the browser.
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white">
  <img alt="Platforms" src="https://img.shields.io/badge/platforms-linux%20%7C%20macOS%20%7C%20windows-lightgrey">
  <img alt="Vault KV" src="https://img.shields.io/badge/Vault%20KV-v1%20%26%20v2-000000?logo=vault&logoColor=white">
</p>

<p align="center">
  <a href="https://lucasassuncao.github.io/vivi/">Website</a>
  &nbsp;·&nbsp;
  <a href="https://lucasassuncao.github.io/vivi/docs/">Documentation</a>
</p>
<!-- markdownlint-enable MD033 -->

Reaching for a secret shouldn't mean opening a browser, logging in again, and clicking through four levels of tree. `vault kv get` is fine when you already know the path. VisualVault is for the rest: finding what exists, comparing versions, fixing one field, and understanding who can read what.

The binary is called `vivi`, leaving `vv` free as a shell alias if you want a two-letter command of your own.

## Quick Demo

![The four tabs in half a minute](docs/images/overview.gif)

---

## Highlights

| | |
| --- | --- |
| 🌲 **Lazy tree** | Opening it lists the mounts and nothing else. A path is read the first time you expand it, so a Vault with ten thousand secrets starts instantly. |
| 👁️ **Masked by default** | Values render as `•••••••••` until you press `r` on the one you want, and the reveal is dropped the moment you navigate away. `y` copies without ever drawing it: the whole secret as JSON from the list, one field from the detail pane. |
| ✍️ **In-panel editing** | Edit fields where you read them. A value is typed in a form that shows the JSON it will write: a field keeps the type the server gave it, a new one is `string`, `number`, `bool` or `json` because you chose so, never because of what the value looks like. The value is a text area, so a pasted certificate keeps its lines. Saving writes a new version with check-and-set, so a concurrent write is rejected instead of silently clobbered. |
| 🕰️ **Full version history** | Read any version, diff two of them, roll back, soft-delete, undelete, destroy, or delete the metadata, with confirmation that scales to the damage. |
| 🔍 **Auditing** | Policies and auth methods, read-only, with a jump from a role straight to the policy it grants. Every secret says what the server will let you do there, the path a policy has to name to grant it, and which of your policies already does. |
| 🛡️ **Read-only mode** | `--read-only` refuses every write; `--read-only=prod` refuses them only where the badge says `PROD`. The header says so, the keys say why, and nothing reaches the server. |
| 🔒 **Honest about denial** | A path your token cannot list is shown as locked, never hidden. A browser that quietly omits what you can't see is worthless for an audit. |
| 🚨 **Knows where it is** | The header badges the environment, names the server version, and warns when this node is a standby or sealed. It counts the token down, loudly under fifteen minutes. Production never looks like the sandbox terminal next to it. |
| 🧩 **KV v1 and v2** | v1 mounts degrade the interface instead of breaking it: no history, no rollback, and a delete that warns it cannot be undone. |

## Install

```bash
go install github.com/lucasassuncao/vivi@latest
```

Or grab a binary from [Releases](https://github.com/lucasassuncao/vivi/releases), or build from source:

```bash
make build          # goreleaser, current platform
make install        # go install
```

`vivi self-update` upgrades the binary in place from GitHub releases. The download is checked against the checksum published with the release, and a release that publishes none is **not** installed: overwriting the binary you are running is the wrong place to accept an unverified file. `--allow-unverified` is there for the case where you know why the checksum is missing.

## Getting started

VisualVault authenticates exactly like the `vault` CLI, from the environment. It has no configuration file of its own.

```bash
export VAULT_ADDR=https://vault.corp:8200
export VAULT_TOKEN=...      # or: vault login
export VAULT_NAMESPACE=...  # optional, Vault Enterprise
vivi
```

### What the token has to be allowed to do

vivi draws a tree of mounts, so it reads the mount table once at startup and needs `read` on `sys/mounts`. That is what tells a KV v1 mount from a v2 one, and every path below depends on the answer. A token denied there starts, but onto an empty tree, with `load mounts: permission denied` in the footer. It is the one capability beyond the paths you actually want to read, and the `vault` CLI does not need it because it probes one mount at a time.

```hcl
path "sys/mounts" {
  capabilities = ["read"]
}
```

Everything else is the ordinary per-path policy you already have. The Policies and Auth tabs read `sys/policies/acl` and `sys/auth`, and a token denied there still browses secrets: those tabs say what was refused instead of failing the session.

### Read-only sessions

`--read-only` refuses every write; `--read-only=prod` refuses them only when the server is not a recognisably local or sandboxed one. `VIVI_READ_ONLY` sets the same thing from a shell profile, which is where the decision is made calmly. See [docs/SAFETY.md](docs/SAFETY.md#read-only-sessions).

### Try it without a Vault

The repository ships a throwaway Vault, seeded with both KV versions, a deep tree, a secret whose history includes a deleted and a destroyed version, policies, auth methods with roles, and a subtree a limited token cannot read.

```bash
make vault-up       # docker compose up, then seed it
make vault-down     # stop it and discard the data
```

It prints two tokens: `root`, and a demo token denied on `kv/restricted/` so you can watch the tree render a locked subtree. See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for every mount, path and policy it creates, and for running it from the devcontainer.

## Commands

| Command | Does |
| --- | --- |
| `vivi` | opens the browser |
| `vivi --read-only` | refuses every write, wherever this points |
| `vivi --read-only=prod` | refuses them only where the badge says `PROD` |
| `vivi --theme <name>` | renders with a named colour theme |
| `vivi --list-themes` | lists the available themes |
| `vivi --version` | prints the version |
| `vivi self-update` | installs the latest release from GitHub, checksum-verified |
| `vivi self-update --list` | shows the available releases |
| `vivi self-update --version <tag>` | installs one release by tag instead of the latest |
| `vivi self-update --prerelease` | includes rc/beta/alpha releases |
| `vivi self-update --allow-unverified` | installs a release that publishes no checksum |

### Themes

The palette is adaptive by default and follows the terminal background. `--theme`, or `VIVI_THEME`, replaces it with one of yedit's 56. See [docs/CONFIGURATION.md](docs/CONFIGURATION.md#themes).

## Keys

Navigation is arrow keys and `enter`, everywhere. Letters are only ever actions, so nothing moves by accident. The horizontal arrows read as depth: `→` goes further in and `←` comes back out.

| Key | Does |
| --- | --- |
| `↑` / `↓` | move |
| `→` / `enter` | go further in: open a folder, or step into the detail pane |
| `←` / `esc` | come back out |
| `tab` / `shift+tab`, `1`-`4` | switch tab |
| `:` | the command line, which lists what can run right here |
| `/` | fuzzy filter over what is already loaded |
| `r` | reveal  ·  `y` copy  ·  `v` version list |
| `e` | edit  ·  `a` create  ·  `d` delete |
| `?` | every key for where you are |

See [docs/KEYS.md](docs/KEYS.md) for the rest: the detail pane, the editor and how it types a new field, the version list, and why `b`, `u`, `D` and `M` are four different words.

## The four tabs

| Tab | Left | Right |
| --- | --- | --- |
| **Secrets** | mounts and paths | data, versions, the version's facts and the secret's |
| **Policies** | policy names | the HCL |
| **Auth** | methods → roles | role config and the policies it grants |
| **Activity** | writes made this session | the operation, its result, its error |

`i` opens the token panel over any tab: accessor, TTL, policies, entity. When authentication comes from an environment variable, that panel is the only way to know which badge you are wearing.

One recording per tab, each a level deeper than the tour above, is in [docs/EXAMPLES.md](docs/EXAMPLES.md).

## Safety

Confirmation scales with the damage. A save or a soft-delete asks `y/n`; the two operations Vault cannot undo make you type the path out. A session can also refuse every write, from `--read-only` or `VIVI_READ_ONLY`, and the header says so the whole time.

Values are masked until asked for, and everything that arrives from the server is neutralised before it is drawn: a secret is content someone else wrote, and written raw to a terminal that content is not text but a program.

See [docs/SAFETY.md](docs/SAFETY.md).

## How it is built

vivi is ports and adapters: `internal/app` holds the decisions that are true about operating a Vault whoever is asking, and it imports no terminal library, a test enforcing that by name, so a policy cannot drift back into the code that draws modals. The TUI is one adapter over that; `internal/vault/fake` is a second one behind the port, which is why the whole suite runs with no server.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the layout and what each file decides, [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) to build and test it, and [docs/RELEASING.md](docs/RELEASING.md) for cutting a release.

## Non-goals

Dynamic secrets, leases, and token revocation. Writing to policies, auth methods, roles or entities. Interactive login. A configuration file. Each is either better served by the `vault` CLI or too dangerous to hand to a keystroke.

`VIVI_THEME` and `VIVI_READ_ONLY` are not a configuration file: the rule is that vivi keeps no state of its own on disk, not that it has no preferences. Both are read the same way the connection is, from the environment. [docs/CONFIGURATION.md](docs/CONFIGURATION.md) is the whole surface.

## License

Released under the [MIT License](LICENSE).
