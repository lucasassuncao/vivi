# The Activity tab: every write this session made, and what it took to make one.
#
# This is the only tape that changes the Vault. It edits kv/app/stage/cache,
# which no other tape shows, so the fixture the others record against is the
# same before and after. `make vault-up` resets it either way.

Output docs/images/activity.gif

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

Type "vivi"
Sleep 400ms
Enter
Sleep 3s

# -- Nothing has been written yet ---------------------------------------------

Type "4"
Sleep 2.5s

# -- Make one write -----------------------------------------------------------

Type ":goto kv/app/stage/cache"
Sleep 900ms
Enter
Sleep 2.5s

Type "e"
Sleep 2s
Enter
Sleep 1.2s
Type "?maxmemory=256mb"
Sleep 1.2s
Enter
Sleep 1.5s
# Saving asks first, and names the version the write will produce.
Ctrl+S
Sleep 3.5s
Type "y"
Sleep 3s

# -- The log now has it, with the version it produced -------------------------

Type "4"
Sleep 3.5s

Type "q"
Sleep 1.5s
