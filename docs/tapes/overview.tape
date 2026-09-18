# vivi in half a minute: the four tabs, and what each one is for.
#
# Prerequisites and how to record: see docs/tapes/README.md.

Output docs/images/overview.gif

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

# -- The mounts, grouped by what they can do ----------------------------------

Type "vivi"
Sleep 400ms
Enter
Sleep 3.5s

# -- A secret, masked until asked ---------------------------------------------

Type ":goto kv/app/prod/db"
Sleep 900ms
Enter
Sleep 3s
Type "r"
Sleep 2.5s

# -- The other three tabs -----------------------------------------------------

Type "2"
Sleep 2.5s
Type "3"
Sleep 2.5s
Type "4"
Sleep 2.5s

Type "q"
Sleep 1.5s
