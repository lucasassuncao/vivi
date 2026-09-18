# The Secrets tab: the lazy tree, masking, version history and the diff.
#
# Navigation is `:goto` rather than counted arrow keys. A tape that walks the
# tree by arrow drifts the moment the fixture gains a mount, and every keystroke
# after the drift lands somewhere else.

Output docs/images/secrets.gif

Require ttyd
Require ffmpeg

Set Shell         "powershell"
Set FontSize      14
Set Width         1400
Set Height        860
Set Theme         "Catppuccin Mocha"
Set Framerate     30
Set PlaybackSpeed 1.0
Set TypingSpeed   45ms
Set Padding       20

# -- Connection, off camera ---------------------------------------------------

Hide
Type "$env:VAULT_ADDR = 'http://127.0.0.1:8200'"
Enter
Type "$env:VAULT_TOKEN = 'root'"
Enter
Type "$env:PATH = $PWD.Path + ';' + $env:PATH"
Enter
Type "clear"
Enter
Show

# -- Mounts are grouped by what they can do, not alphabetically ---------------

Type "vivi"
Sleep 400ms
Enter
Sleep 3.5s

# -- :goto walks a full path, expanding each level as the listing arrives ------

Type ":goto kv/app/prod/db"
Sleep 900ms
Enter
Sleep 3s

# -- Masked by default; r reveals the selected field only ---------------------

Type "r"
Sleep 3s

# -- Four versions, one soft-deleted and one destroyed ------------------------

Type "v"
Sleep 3s

# Mark two and diff them. The two live ones, v4 and v3: v2 is soft-deleted and
# has no data to compare, which is a refusal worth seeing but not here.
Space
Sleep 800ms
Down
Sleep 500ms
Space
Sleep 900ms
Type "d"
Sleep 3.5s
Escape
Sleep 700ms
Escape
Sleep 1s

# -- A KV v1 mount: no history, and a delete with no undo ---------------------

Type ":goto legacy/old/app"
Sleep 900ms
Enter
Sleep 3s

Type "q"
Sleep 1.5s
