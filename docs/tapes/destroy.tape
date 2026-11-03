# Destroying a version: the gate you type through, and the empty secret it leaves.
#
# It destroys kv/app/prod/queue, which no other tape shows. Recording it again
# needs `make vault-down` and `make vault-up` first, to bring the version back.

Output docs/images/destroy.gif

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

Type ":goto kv/app/prod/queue"
Sleep 900ms
Enter
Sleep 2.5s

# -- D in the version list asks for the secret's name, not y/n -----------------

Type "v"
Sleep 2s
Type "D"
Sleep 3.5s

# Half the name keeps it locked; the whole name unlocks it.
Set TypingSpeed 180ms
Type "que"
Sleep 1.5s
Enter
Sleep 1.5s
Type "ue"
Sleep 1.5s
Set TypingSpeed 45ms
Enter
Sleep 3s

# -- Destroyed, and still in the listing --------------------------------------

Escape
Sleep 3.5s

Type "q"
Sleep 1.5s
