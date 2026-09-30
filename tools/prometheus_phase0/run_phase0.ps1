# Prometheus Phase 0 GPU verification. From the repository root:
#   powershell -ExecutionPolicy Bypass -File tools\prometheus_phase0\run_phase0.ps1
# Builds and runs Marionette for the pre-Phase-0 baseline (a temporary git
# worktree) and for the current checkout, then compares results per test.
# Everything is logged to tools\prometheus_phase0\out\phase0.log.
param([string]$Baseline = "a8bbdd60")
$ErrorActionPreference = "Continue"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$out = Join-Path $PSScriptRoot "out"
New-Item -ItemType Directory -Force -Path $out | Out-Null
$log = Join-Path $out "phase0.log"
"Prometheus Phase 0 verification $(Get-Date -Format o)" | Out-File -Encoding utf8 $log
function Log([string]$t) { $t | Out-File -Encoding utf8 -Append $log; Write-Host $t }

$goRoot = "C:\Program Files\Go\bin"
if (-not (Get-Command go -ErrorAction SilentlyContinue) -and (Test-Path $goRoot)) { $env:PATH = "$goRoot;$env:PATH" }
Set-Location $root
Log "root=$root head=$(git rev-parse --short HEAD) branch=$(git branch --show-current) baseline=$Baseline"

function Build-And-Run([string]$tree, [string]$label) {
    Log ""
    Log "==== build $label"
    $launcher = Join-Path $tree "internal\prometheus\native\build_windows_launcher.cmd"
    & cmd /c "`"$launcher`"" 2>&1 | Out-Null
    $buildLog = Join-Path $tree "out\test-artifacts\prometheus_native_windows_build.log"
    if (Test-Path $buildLog) { Get-Content $buildLog -Tail 15 | ForEach-Object { Log "  $_" } }
    $exe = Join-Path $tree "out\prometheus\native\marionette_tests.exe"
    if (-not (Test-Path $exe)) { Log "---- build $label : FAIL (no marionette_tests.exe)"; return $null }
    Log "---- build $label : PASS"
    Log "==== run $label"
    $runFile = Join-Path $out "$label.run.txt"
    Push-Location $tree
    & $exe *> $runFile
    $code = $LASTEXITCODE
    Pop-Location
    Log "---- run $label : exit $code"
    Get-Content $runFile | Select-String -Pattern '^Summary:' | ForEach-Object { Log "  $($_.Line)" }
    $results = @{}
    $last = $null
    foreach ($line in Get-Content $runFile) {
        if ($line -match '^\[RUN\] (.+)$') { $last = $Matches[1]; $results[$last] = "NO-RESULT" }
        elseif ($line -match '^\[(PASS|FAIL|SKIP)\] (.+)$') { $results[$Matches[2]] = $Matches[1] }
    }
    return $results
}

# Baseline worktree
$baseTree = Join-Path $env:TEMP "oct-phase0-baseline"
if (Test-Path $baseTree) { git worktree remove --force $baseTree 2>$null | Out-Null; Remove-Item -Recurse -Force $baseTree -ErrorAction SilentlyContinue }
git worktree add --detach $baseTree $Baseline 2>&1 | Out-Null
$base = Build-And-Run $baseTree "baseline"
$cur = Build-And-Run $root "phase0"

Log ""
Log "==== comparison"
if ($base -eq $null -or $cur -eq $null) {
    Log "comparison skipped: a build failed"
} else {
    $regressions = @(); $fixed = @(); $removed = @(); $added = @()
    foreach ($k in $base.Keys) {
        if (-not $cur.ContainsKey($k)) { $removed += "$k ($($base[$k]))"; continue }
        if ($base[$k] -eq "PASS" -and $cur[$k] -ne "PASS") { $regressions += "$k : PASS -> $($cur[$k])" }
        if ($base[$k] -ne "PASS" -and $cur[$k] -eq "PASS") { $fixed += "$k : $($base[$k]) -> PASS" }
    }
    foreach ($k in $cur.Keys) { if (-not $base.ContainsKey($k)) { $added += "$k : $($cur[$k])" } }
    $bs = $base.Values | Group-Object | ForEach-Object { "$($_.Name)=$($_.Count)" }
    $cs = $cur.Values | Group-Object | ForEach-Object { "$($_.Name)=$($_.Count)" }
    Log "baseline: $($bs -join ' ')"
    Log "phase0:   $($cs -join ' ')"
    Log "REGRESSIONS ($($regressions.Count)):"; $regressions | Sort-Object | ForEach-Object { Log "  $_" }
    Log "NEWLY PASSING ($($fixed.Count)):"; $fixed | Sort-Object | ForEach-Object { Log "  $_" }
    Log "NEW OR RENAMED TESTS ($($added.Count)):"; $added | Sort-Object | ForEach-Object { Log "  $_" }
    Log "REMOVED TESTS: $($removed.Count)"
}

Log ""
Log "==== go test ./internal/prometheus/"
go test ./internal/prometheus/ -count=1 2>&1 | Select-Object -Last 25 | ForEach-Object { Log "  $_" }

git worktree remove --force $baseTree 2>$null | Out-Null
Log ""
Log "==== done"
