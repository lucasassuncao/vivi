# Demo tapes

Each tape records one tab, with [VHS](https://github.com/charmbracelet/vhs). `overview.tape` is the short tour for the top of the README; the other four go one level deeper, one per tab, and are what [EXAMPLES.md](../EXAMPLES.md) is made of.

| Tape | Shows |
| --- | --- |
| `overview.tape` | the four tabs in half a minute |
| `secrets.tape` | the lazy tree, masking, version history, the diff, and a KV v1 mount |
| `policies.tape` | reading a policy document, copying it, jumping to one by name |
| `auth.tape` | methods, the roles under them, and following a grant to its policy |
| `activity.tape` | the write log, and the edit that fills it |

## Recording

```bash
go install github.com/charmbracelet/vhs@latest   # plus ttyd and ffmpeg
make vault-up                                    # the seeded throwaway Vault
go build -o vivi.exe .
vhs docs/tapes/secrets.tape                      # from the repository root
```

From the repository root, always: `Output` is resolved against the working directory, not against the tape.

**The tapes are written for Windows.** Each one sets `Set Shell "powershell"` and exports the connection with `$env:`, and the build line above produces `vivi.exe`. Recording them elsewhere means changing those two lines per tape; nothing else in them is platform-specific.

The connection is set up hidden in every tape, against the dev Vault from `make vault-up`: dev mode, in memory, root token. Nothing here touches a real server.

`activity.tape` is the only one that writes, and it writes to a path no other tape shows.

## Navigation

The tapes move with `:goto` and `:policy` rather than counted arrow keys. A tape that walks the tree by arrow drifts the moment the fixture gains a mount, and every keystroke after the drift lands somewhere else, which is how the first version of these recorded a walk into an empty mount.
