$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
Set-Location $repoRoot

Write-Host "OPENAI BUILD WEEK - REPOSITORY EVIDENCE BOUNDARY" -ForegroundColor Cyan
Write-Host "Eligible start: 2026-07-13 09:00:00 Pacific"
Write-Host "Last pre-event commit:"
git show -s --date=iso-strict --format="  %H  %ad  %s" 8c029d6d8f8d5f698276edfda138fa96f5fb305e
Write-Host ""
Write-Host "23 ELIGIBLE COMMITS THROUGH AUDITED HEAD" -ForegroundColor Green
$range = "8c029d6d8f8d5f698276edfda138fa96f5fb305e..b07c8849efa00fe0455e827e9a162856f389878f"
git log --reverse "--date=format-local:%Y-%m-%d %H:%M" "--format=%h  %ad  %s" $range
