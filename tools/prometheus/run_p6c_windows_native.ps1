param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\\..")).Path,
    [string]$OctPath = "",
    [string]$ExperimentPath = "",
    [string]$ReactorPath = "",
    [string]$CCPath = "",
    [string]$CXXPath = "",
    [int]$WarmupRuns = 1,
    [int]$MeasuredRuns = 5,
    [string]$OutputDir = ""
)

$ErrorActionPreference = "Stop"

function Resolve-RepoPath([string]$base, [string]$candidate) {
    if ([string]::IsNullOrWhiteSpace($candidate)) {
        return ""
    }
    if ([System.IO.Path]::IsPathRooted($candidate)) {
        return (Resolve-Path $candidate).Path
    }
    return (Resolve-Path (Join-Path $base $candidate)).Path
}

function Choose-FirstExisting([string[]]$candidates, [string]$label) {
    foreach ($candidate in $candidates) {
        if (-not [string]::IsNullOrWhiteSpace($candidate) -and (Test-Path $candidate)) {
            return (Resolve-Path $candidate).Path
        }
    }
    throw "Unable to locate $label. Checked: $($candidates -join ', ')"
}

function Resolve-OptionalTool([string]$repoRoot, [string]$provided, [string[]]$candidates, [string]$label) {
    if (-not [string]::IsNullOrWhiteSpace($provided)) {
        return Resolve-RepoPath $repoRoot $provided
    }
    return Choose-FirstExisting $candidates $label
}

function Parse-BenchmarkRun([string]$path) {
    $text = [System.IO.File]::ReadAllText($path)
    $pattern = 'BenchmarkCaseResult \{\s*Name: "(?<Name>[^"]+)"\s*DurationNs: \((?<DurationNs>\d+)\)\s*BackendRequested: "(?<BackendRequested>[^"]+)"\s*BackendUsed: "(?<BackendUsed>[^"]+)"\s*Status: "(?<Status>[^"]+)"\s*Environment: "(?<Environment>[^"]+)"\s*ReportedWallNs: \((?<ReportedWallNs>\d+)\)\s*\}'
    $matches = [System.Text.RegularExpressions.Regex]::Matches($text, $pattern)
    $cases = @()
    foreach ($match in $matches) {
        $cases += [pscustomobject]@{
            Name             = $match.Groups["Name"].Value
            DurationNs       = [int64]$match.Groups["DurationNs"].Value
            BackendRequested = $match.Groups["BackendRequested"].Value
            BackendUsed      = $match.Groups["BackendUsed"].Value
            Status           = $match.Groups["Status"].Value
            Environment      = $match.Groups["Environment"].Value
            ReportedWallNs   = [int64]$match.Groups["ReportedWallNs"].Value
        }
    }
    if ($cases.Count -eq 0) {
        throw "No BenchmarkCaseResult entries found in $path"
    }
    return $cases
}

function Get-Median([int64[]]$values) {
    $sorted = $values | Sort-Object
    if ($sorted.Count -eq 0) {
        return [int64]0
    }
    $mid = [int]($sorted.Count / 2)
    if (($sorted.Count % 2) -eq 1) {
        return [int64]$sorted[$mid]
    }
    return [int64](($sorted[$mid - 1] + $sorted[$mid]) / 2)
}

function Get-Average([int64[]]$values) {
    if ($values.Count -eq 0) {
        return [int64]0
    }
    [int64]$sum = 0
    foreach ($value in $values) {
        $sum += $value
    }
    return [int64]($sum / $values.Count)
}

function Format-Ns([int64]$value) {
    if ($value -ge 1000000000) {
        return ("{0:N3}s" -f ($value / 1e9))
    }
    if ($value -ge 1000000) {
        return ("{0:N3}ms" -f ($value / 1e6))
    }
    if ($value -ge 1000) {
        return ("{0:N3}us" -f ($value / 1e3))
    }
    return "$value ns"
}

if ($WarmupRuns -lt 0) {
    throw "WarmupRuns must be >= 0"
}
if ($MeasuredRuns -lt 1) {
    throw "MeasuredRuns must be >= 1"
}

$RepoRoot = (Resolve-Path $RepoRoot).Path

if ([string]::IsNullOrWhiteSpace($OctPath)) {
    $OctPath = Choose-FirstExisting @(
        (Join-Path $RepoRoot "oct.exe"),
        (Join-Path $RepoRoot "oct")
    ) "oct executable"
} else {
    $OctPath = Resolve-RepoPath $RepoRoot $OctPath
}

if ([string]::IsNullOrWhiteSpace($ExperimentPath)) {
    $ExperimentPath = Join-Path $RepoRoot "Experiments\\PrometheusBenchmarkHarness\\M0"
} else {
    $ExperimentPath = Resolve-RepoPath $RepoRoot $ExperimentPath
}

if ([string]::IsNullOrWhiteSpace($ReactorPath)) {
    $ReactorPath = Choose-FirstExisting @(
        (Join-Path $RepoRoot "internal\\prometheus\\reactor\\prometheus_reactor.dll"),
        (Join-Path $RepoRoot "out\\prometheus\\native\\prometheus_reactor.dll")
    ) "Prometheus reactor DLL"
} else {
    $ReactorPath = Resolve-RepoPath $RepoRoot $ReactorPath
}

if ([string]::IsNullOrWhiteSpace($OutputDir)) {
    $OutputDir = Join-Path $RepoRoot "out\\prometheus\\native\\p6c"
} else {
    if ([System.IO.Path]::IsPathRooted($OutputDir)) {
        $OutputDir = $OutputDir
    } else {
        $OutputDir = Join-Path $RepoRoot $OutputDir
    }
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

$CCPath = Resolve-OptionalTool $RepoRoot $CCPath @(
    "C:\\Users\\yuech\\mingw64\\bin\\gcc.exe",
    "C:\\Program Files\\GNU Octave\\Octave-9.4.0\\mingw64\\bin\\gcc.exe"
) "C compiler"

$CXXPath = Resolve-OptionalTool $RepoRoot $CXXPath @(
    "C:\\Users\\yuech\\mingw64\\bin\\g++.exe",
    "C:\\Program Files\\GNU Octave\\Octave-9.4.0\\mingw64\\bin\\g++.exe"
) "C++ compiler"

$env:CGO_ENABLED = "1"
$env:CC = $CCPath
$env:CXX = $CXXPath
$env:OCT_PROMETHEUS_REACTOR = $ReactorPath

function Invoke-BenchRun([string]$label, [int]$index) {
    $artifact = Join-Path $OutputDir ("{0}_{1:D2}.octagon" -f $label, $index)
    $command = @($ExperimentPath, "--octagon-out", $artifact)
    $stdout = & $OctPath bench @command 2>&1
    $exitCode = $LASTEXITCODE
    $stdoutText = ($stdout | Out-String)
    [System.IO.File]::WriteAllText((Join-Path $OutputDir ("{0}_{1:D2}.stdout.txt" -f $label, $index)), $stdoutText)
    if ($exitCode -ne 0) {
        throw "oct bench failed during $label run $index with exit code $exitCode.`n$stdoutText"
    }
    return [pscustomobject]@{
        Artifact = $artifact
        Stdout   = $stdoutText
        Cases    = Parse-BenchmarkRun $artifact
    }
}

for ($i = 1; $i -le $WarmupRuns; $i++) {
    Write-Host ("Warmup {0}/{1}" -f $i, $WarmupRuns)
    [void](Invoke-BenchRun "warmup" $i)
}

$measuredRunResults = @()
for ($i = 1; $i -le $MeasuredRuns; $i++) {
    Write-Host ("Measured {0}/{1}" -f $i, $MeasuredRuns)
    $measuredRunResults += Invoke-BenchRun "measured" $i
}

$grouped = @{}
foreach ($run in $measuredRunResults) {
    foreach ($case in $run.Cases) {
        if (-not $grouped.ContainsKey($case.Name)) {
            $grouped[$case.Name] = New-Object System.Collections.Generic.List[object]
        }
        $grouped[$case.Name].Add($case)
    }
}

$summaryCases = New-Object System.Collections.Generic.List[object]
foreach ($name in ($grouped.Keys | Sort-Object)) {
    $cases = $grouped[$name]
    $first = $cases[0]
    foreach ($case in $cases) {
        if ($case.BackendRequested -ne $first.BackendRequested -or
            $case.BackendUsed -ne $first.BackendUsed -or
            $case.Status -ne $first.Status -or
            $case.Environment -ne $first.Environment) {
            throw "Inconsistent metadata across measured runs for $name"
        }
    }

    $durations = @($cases | ForEach-Object { [int64]$_.DurationNs })
    $reportedWalls = @($cases | Where-Object { $_.ReportedWallNs -gt 0 } | ForEach-Object { [int64]$_.ReportedWallNs })

    $summaryCases.Add([pscustomobject]@{
        Name               = $name
        BackendRequested   = $first.BackendRequested
        BackendUsed        = $first.BackendUsed
        Status             = $first.Status
        Environment        = $first.Environment
        MedianDurationNs   = Get-Median $durations
        AverageDurationNs  = Get-Average $durations
        MinDurationNs      = [int64](($durations | Measure-Object -Minimum).Minimum)
        MaxDurationNs      = [int64](($durations | Measure-Object -Maximum).Maximum)
        MedianReportedWallNs = if ($reportedWalls.Count -gt 0) { Get-Median $reportedWalls } else { [int64]0 }
        AverageReportedWallNs = if ($reportedWalls.Count -gt 0) { Get-Average $reportedWalls } else { [int64]0 }
    })
}

$summaryObject = [pscustomobject]@{
    GeneratedAt   = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
    RepoRoot      = $RepoRoot
    OctPath       = $OctPath
    ExperimentPath = $ExperimentPath
    ReactorPath   = $ReactorPath
    WarmupRuns    = $WarmupRuns
    MeasuredRuns  = $MeasuredRuns
    Cases         = $summaryCases
}

$summaryJson = $summaryObject | ConvertTo-Json -Depth 6
[System.IO.File]::WriteAllText((Join-Path $OutputDir "summary.json"), $summaryJson)

$lines = New-Object System.Collections.Generic.List[string]
$lines.Add("# P6c Windows-native summary")
$lines.Add("")
$lines.Add(("Generated at: {0}" -f $summaryObject.GeneratedAt))
$lines.Add(("Warmup runs: {0}" -f $WarmupRuns))
$lines.Add(("Measured runs: {0}" -f $MeasuredRuns))
$lines.Add("")
$lines.Add("| Case | Req | Used | Status | Env | Median outer | Avg outer | Median inner wall |")
$lines.Add("| --- | --- | --- | --- | --- | --- | --- | --- |")
foreach ($case in $summaryCases) {
    $medianInnerWall = "n/a"
    if ($case.MedianReportedWallNs -gt 0) {
        $medianInnerWall = Format-Ns $case.MedianReportedWallNs
    }
    $lines.Add((
        "| {0} | {1} | {2} | {3} | {4} | {5} | {6} | {7} |" -f
        $case.Name,
        $case.BackendRequested,
        $case.BackendUsed,
        $case.Status,
        $case.Environment,
        (Format-Ns $case.MedianDurationNs),
        (Format-Ns $case.AverageDurationNs),
        $medianInnerWall
    ))
}
[System.IO.File]::WriteAllText((Join-Path $OutputDir "summary.md"), ($lines -join "`r`n") + "`r`n")

Write-Host "P6c summary written to:"
Write-Host ("  {0}" -f (Join-Path $OutputDir "summary.json"))
Write-Host ("  {0}" -f (Join-Path $OutputDir "summary.md"))
