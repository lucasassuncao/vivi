# The Policies tab: reading the documents that decide what a token may do.

Output docs/images/policies.gif

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

# -- The policy list ----------------------------------------------------------

Type "2"
Sleep 2.5s
Down
Sleep 800ms
Down
Sleep 1.5s

# -- Reading one: the HCL, highlighted, in the pane that scrolls ---------------

Right
Sleep 3s
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 1.5s

# -- The whole document to the clipboard --------------------------------------

Type "y"
Sleep 2.5s

# -- :policy jumps by name, with no walking ------------------------------------

Escape
Sleep 700ms
Type ":policy vivi-demo"
Sleep 900ms
Enter
Sleep 3s

Type "q"
Sleep 1.5s
