# The Auth tab: which methods are enabled, the roles under them, and the jump
# from a role to the policy it grants.

Output docs/images/auth.gif

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

# -- The enabled methods ------------------------------------------------------

Type "3"
Sleep 3s

# -- Into the roles under one of them -----------------------------------------

Right
Sleep 2.5s
Down
Sleep 1.5s

# -- The role's detail: its policies and its token settings -------------------

Right
Sleep 3s

# -- p follows the grant to the policy document itself, switching tab ----------

Type "p"
Sleep 3.5s

Type "q"
Sleep 1.5s
