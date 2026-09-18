# Prints the `## ` help text of every makefile passed as an argument. A script
# and not an inline -Command: a recipe carrying PowerShell's `$_` is expanded
# by sh.exe first, which make picks over cmd.exe whenever one is on PATH.
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Files)

Select-String -Path $Files -Pattern '^([a-zA-Z_-]+):.*## (.+)' |
    Sort-Object { $_.Matches[0].Groups[1].Value } |
    ForEach-Object {
        Write-Host -NoNewline -ForegroundColor Cyan ('{0,-20}' -f $_.Matches[0].Groups[1].Value)
        Write-Host (' ' + $_.Matches[0].Groups[2].Value)
    }
