param(
    [string]$RepoRoot = "",
    [string]$OctPath = "",
    [string]$OutDir = ""
)

$ErrorActionPreference = 'Stop'

function Resolve-RepoRoot([string]$hint) {
    if ($hint -and (Test-Path $hint)) {
        return (Resolve-Path $hint).Path
    }
    if ($PSScriptRoot) {
        return (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
    }
    return (Get-Location).Path
}

$repo = Resolve-RepoRoot $RepoRoot
if (-not $OctPath) {
    $OctPath = Join-Path $repo "bin/oct.exe"
}
if (-not (Test-Path $OctPath)) {
    throw "oct executable not found at $OctPath. Build oct first (go build ./cmd/oct)."
}

if (-not $OutDir) {
    $OutDir = Join-Path $repo "out/prometheus/sgemm_lab_m6"
}
if (-not (Test-Path $OutDir)) {
    New-Item -ItemType Directory -Path $OutDir | Out-Null
}

$expPath = "Experiments/PrometheusSgemmAlgorithmLab/M4"
Write-Host "== M6 artifact pass (decision object boundary probe) =="
& $OctPath artifact $expPath

$artifactSrc = Join-Path $repo "Experiments/PrometheusSgemmAlgorithmLab/M4/m6_decision_probe.octagon"
$artifactDst = Join-Path $OutDir "m6_decision_probe.octagon"
if (-not (Test-Path $artifactSrc)) {
    throw "expected artifact missing: $artifactSrc"
}
Copy-Item $artifactSrc $artifactDst -Force

Write-Host "== M6 benchmark pass (Windows-native truth source) =="
$benchOut = Join-Path $OutDir "m6_windows_bench.octagon"
& $OctPath bench $expPath --octagon-out $benchOut

Write-Host "M6 windows-native probe artifacts written:"
Write-Host "  $artifactDst"
Write-Host "  $benchOut"
