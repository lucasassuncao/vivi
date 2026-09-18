# Safety

Editing secrets is easy to get wrong in ways you cannot undo, so confirmation scales with the damage. A save or a soft-delete asks `y/n`. The two operations Vault cannot undo make you type the path out:

```plain
╭───────────────────────────────────────────────────────────────────╮
│  Destroy versions                                                 │
│                                                                   │
│  kv/app/prod/db                                                   │
│                                                                   │
│  Versions: v4 (current)                                           │
│                                                                   │
│  This erases the data permanently.                                │
│  Vault has no undo for destroy: undelete will not bring it back.  │
│                                                                   │
│  The secret stays in the listing with an empty version.           │
│  Delete all (M) is what removes the path itself.                  │
│                                                                   │
│  Type db to confirm:                                              │
│                                                                   │
│  >                                                                │
│                                                                   │
│  type the exact text to unlock   [esc] cancel                     │
╰───────────────────────────────────────────────────────────────────╯
```

* **Check-and-set on every KV v2 write.** The save carries the version you opened. If someone wrote in the meantime, the write is rejected, your edit is kept, and you choose between reloading and continuing. It never overwrites for you. A KV v1 mount has no check-and-set to carry, so a save there cannot detect a concurrent write; the modal says out loud that it overwrites the only copy, which is the whole of what v1 can promise.
* **A KV v1 delete gets the same strong gate**, because v1 has no undo either.
* **A field's type is never guessed.** A number that arrives from the server goes back as a number and an object as an object, untouched fields are written back exactly as they arrived, and a new field is a string until the form's type row says otherwise. The corruption this prevents is the invisible kind: a digit-only password turned into a number, or a port turned into text, reads fine in vivi and breaks the application that consumes it. A value that does not fit its type is refused in the form, beside the value, before there is anything to confirm. See [KEYS.md](KEYS.md#types).
* **A value keeps its lines.** The field form is a text area, so a certificate or a key pasted into it arrives on the server with every line break it had. The one-line input it replaced turned each of them into a space, silently, and a PEM without its breaks is a certificate nothing can load.
* **Rollback destroys nothing.** It writes the old content as a new version, and the modal says so.
* **No secret ever leaves the panel.** Values are masked by default, reveals are scoped to one field and dropped on navigation, and nothing of a secret's content is written to the activity log. The diff follows the same rule rather than a stricter one: it opens masked, `r` reveals every row at once because a diff is read as a whole, and closing the modal puts them all back behind the dots. The one exception is opt-in and yours: `VIVI_DEBUG` writes a keystroke log, and a keystroke log is what you typed. See [CONFIGURATION.md](CONFIGURATION.md#vivi_debug).
* **Nothing from the server can drive your terminal.** A value, a key, a policy or an error message is content someone else wrote, and written raw to a terminal that content is not text but a program: an escape sequence can set your clipboard, wipe the screen, or draw a convincing fake confirmation prompt, and a bidirectional override can make a policy read `["read"]` while granting something else. Control characters are shown as escapes instead of being run, and shown rather than dropped, because `a\x00b` and `ab` are different secrets. What you copy and what you save are still the real bytes.

## Read-only sessions

The confirmations scale with the damage, and they cover the moment you are about to do something. They do not cover the session you opened only to look at production, which is exactly the session you were not being careful in.

```bash
vivi --read-only          # refuse every write, wherever this points
vivi --read-only=prod     # refuse them only where the badge says PROD
export VIVI_READ_ONLY=prod   # the same, decided once in a shell profile
```

`prod` is the one worth persisting. It pairs with the environment badge: the badge says what kind of server this is, and this makes vivi act on its own answer instead of only reporting it. The default is `off`, so nothing changes for a session that does not ask.

A read-only session badges the header, drops the write keys from the legend, hides them from `:`, and says why when you press one anyway. Renewing your own token still works, since a session that cannot renew is one that ends in the middle of the reading it was opened for.

If the address is missing, the token is expired, or the server is unreachable, it says why on the command line and exits. It never opens onto an empty tree that could equally mean "empty Vault" or "broken token".

```console
$ vivi
vivi: VAULT_ADDR: not set
  export VAULT_ADDR=https://your-vault:8200
```
