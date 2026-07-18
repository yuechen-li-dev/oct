param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("test", "artifact")]
    [string]$Lane,
    [string]$OctPath = ""
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
$fixture = "docs/build-week/recording/fixtures/JudgeDemo"
Set-Location $repoRoot

function Invoke-Oct([string[]]$OctArgs) {
    if ($OctPath -ne "") {
        & $OctPath @OctArgs
    } else {
        & go run ./cmd/oct @OctArgs
    }
    if ($LASTEXITCODE -ne 0) {
        throw "Oct exited with code $LASTEXITCODE"
    }
}

if ($Lane -eq "test") {
    Write-Host "> oct test $fixture --execution auto --json" -ForegroundColor Cyan
    Invoke-Oct @("test", $fixture, "--execution", "auto", "--json")
} else {
    Write-Host "> oct artifact $fixture --execution interpreted --json" -ForegroundColor Cyan
    Invoke-Oct @("artifact", $fixture, "--execution", "interpreted", "--json")
}
