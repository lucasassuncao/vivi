<!-- markdownlint-disable MD033 -->
<div align="center">

![VisualVault logo](images/image.png ':size=313')

</div>

<h1 align="center">VisualVault</h1>

<p align="center">
or <code>vv</code> (<code>vivi</code>) for short
</p>

<p align="center">
  <strong>An interactive terminal browser for HashiCorp Vault.</strong><br>
  The parts of the web UI you actually use, without the browser.
</p>

![The four tabs in half a minute](images/overview.gif)

## What it does

|  | |
|---|---|
| 🌲 **Lazy tree** | Opening it lists the mounts and nothing else. A path is read the first time you expand it, so a Vault with ten thousand secrets starts instantly. |
| 👁️ **Masked by default** | Values render as `•••••••••` until you press `r`, and the reveal is dropped the moment you navigate away. `y` copies without ever drawing it. |
| ✍️ **In-panel editing** | Edit fields where you read them, in a form that shows the JSON it will write. Saving uses check-and-set, so a concurrent write is rejected rather than clobbered. |
| 🕰️ **Full version history** | Read any version, diff two, roll back, soft-delete, undelete, destroy, or delete the metadata, with confirmation that scales to the damage. |
| 🔍 **Auditing** | Policies and auth methods, read-only, with a jump from a role straight to the policy it grants. |
| 🛡️ **Read-only mode** | `--read-only` refuses every write; `--read-only=prod` refuses them only where the badge says `PROD`. |
| 🔒 **Honest about denial** | A path your token cannot list is shown as locked, never hidden. |

## Install

```bash
go install github.com/lucasassuncao/vivi@latest
```

Or grab a binary from [Releases](https://github.com/lucasassuncao/vivi/releases). `vivi self-update` upgrades it in place, checked against the checksum published with the release.

## Connect

vivi authenticates exactly like the `vault` CLI, from the environment. It has no configuration file of its own.

```bash
export VAULT_ADDR=https://vault.corp:8200
export VAULT_TOKEN=...      # or: vault login
export VAULT_NAMESPACE=...  # optional, Vault Enterprise
vivi
```

The token needs `read` on `sys/mounts` on top of the paths you want to browse: that is what tells a KV v1 mount from a v2 one. See [Configuration](/CONFIGURATION.md).

## Where to go next

| Page | For |
| --- | --- |
| [Keys](/KEYS.md) | every keystroke, per pane |
| [Commands](/COMMANDS.md) | the `:` command line, and what is available where |
| [Safety](/SAFETY.md) | what vivi refuses to do, and the confirmations |
| [Configuration](/CONFIGURATION.md) | every environment variable it reads |
| [Development](/DEVELOPMENT.md) | the throwaway Vault, the suites, the devcontainer |
| [Architecture](/ARCHITECTURE.md) | ports and adapters, and where a change belongs |
| [Releasing](/RELEASING.md) | tags, ldflags, and the self-update coupling |

## Try it without a Vault

The repository ships a throwaway Vault, seeded with both KV versions, a deep tree, a secret whose history includes a deleted and a destroyed version, policies, auth methods with roles, and a subtree a limited token cannot read.

```bash
make vault-up       # docker compose up, then seed it
make vault-down     # stop it and discard the data
```
