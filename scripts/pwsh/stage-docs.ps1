# Assembles the docs site into -Out: -Docs/pages as the root, plus the markdown,
# the recordings and the icons beside it, where docsify and the landing page read
# them. GitHub Pages publishes this same directory.
param(
    [Parameter(Position = 0)][string]$Docs,
    [Parameter(Position = 1)][string]$Out
)
$ErrorActionPreference = 'Stop'

if (-not $Docs -or -not $Out) {
    [Console]::Error.WriteLine('usage: stage-docs.ps1 <docs-dir> <out-dir>')
    exit 2
}

if (Test-Path -LiteralPath $Out) {
    Remove-Item -LiteralPath $Out -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $Out | Out-Null
Copy-Item -Path "$Docs/pages/*" -Destination $Out -Recurse
Copy-Item -Path "$Docs/*.md" -Destination $Out
Copy-Item -LiteralPath "$Docs/images", "$Docs/icons" -Destination $Out -Recurse
Write-Host "Docs site staged in $Out/"
