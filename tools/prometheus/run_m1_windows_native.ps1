param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\\..")).Path,
    [string]$OctPath = "",
    [string]$ExperimentPath = "",
    [string]$ReactorPath = "",
    [string]$CCPath = "",
    [string]$CXXPath = "",
    [int]$WarmupRuns = 1,
    [int]$MeasuredRuns = 3,
    [string]$OutputDir = ""
)

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $false

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

function Parse-BenchmarkCase([string]$path) {
    $text = [System.IO.File]::ReadAllText($path)
    $pattern = 'BenchmarkCaseResult \{\s*Name: "(?<Name>[^"]+)"\s*DurationNs: \((?<DurationNs>\d+)\)\s*BackendRequested: "(?<BackendRequested>[^"]+)"\s*BackendUsed: "(?<BackendUsed>[^"]+)"\s*Status: "(?<Status>[^"]+)"\s*Correctness: (?<Correctness>true|false)\s*Environment: "(?<Environment>[^"]+)"\s*DetailCode: \((?<DetailCode>-?\d+)\)\s*DetailName: "(?<DetailName>[^"]+)"\s*ReportedWallNs: \((?<ReportedWallNs>\d+)\)\s*\}'
    $matches = [System.Text.RegularExpressions.Regex]::Matches($text, $pattern, [System.Text.RegularExpressions.RegexOptions]::Singleline)
    if ($matches.Count -ne 1) {
        throw "Expected exactly one BenchmarkCaseResult in $path, found $($matches.Count)"
    }
    $match = $matches[0]
    return [pscustomobject]@{
        Name             = $match.Groups["Name"].Value
        DurationNs       = [int64]$match.Groups["DurationNs"].Value
        BackendRequested = $match.Groups["BackendRequested"].Value
        BackendUsed      = $match.Groups["BackendUsed"].Value
        Status           = $match.Groups["Status"].Value
        Correctness      = ($match.Groups["Correctness"].Value -eq "true")
        Environment      = $match.Groups["Environment"].Value
        DetailCode       = [int]$match.Groups["DetailCode"].Value
        DetailName       = $match.Groups["DetailName"].Value
        ReportedWallNs   = [int64]$match.Groups["ReportedWallNs"].Value
    }
}

function Parse-KeyValueLine([string]$text) {
    $result = @{}
    foreach ($field in ($text -split '\s+')) {
        if ($field -notmatch '=') {
            continue
        }
        $parts = $field -split '=', 2
        $result[$parts[0]] = $parts[1]
    }
    return $result
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
    $ExperimentPath = Join-Path $RepoRoot "Experiments\\PrometheusBenchmarkHarness\\M1"
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
    $OutputDir = Join-Path $RepoRoot "out\\prometheus\\benchmark_harness_m1"
} elseif (-not [System.IO.Path]::IsPathRooted($OutputDir)) {
    $OutputDir = Join-Path $RepoRoot $OutputDir
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
$toolchainBin = Split-Path -Parent $CCPath
if (-not [string]::IsNullOrWhiteSpace($toolchainBin)) {
    $env:PATH = $toolchainBin + ";" + $env:PATH
}

$cases = @(
    [pscustomobject]@{ Id = "tiny_direct_baseline"; Filter = "Main.M1_TinyDirectBaseline_M002_N002_K002"; Flags = 0; ExpectedDetail = "direct"; Description = "Tiny direct baseline correctness"; Group = "A" },
    [pscustomobject]@{ Id = "forced_direct_tiled"; Filter = "Main.M1_ForcedDirectTiled_M032_N032_K032"; Flags = 18432; ExpectedDetail = "direct_tiled"; Description = "Forced direct+tiled reachability"; Group = "A" },
    [pscustomobject]@{ Id = "forced_staged_baseline"; Filter = "Main.M1_ForcedStagedBaseline_M016_N016_K016"; Flags = 1024; ExpectedDetail = "staged_upload_readback"; Description = "Forced staged baseline reachability"; Group = "A" },
    [pscustomobject]@{ Id = "forced_staged_tiled"; Filter = "Main.M1_ForcedStagedTiled_M032_N032_K032"; Flags = 17408; ExpectedDetail = "staged_upload_readback_tiled"; Description = "Forced staged+tiled reachability"; Group = "A" },
    [pscustomobject]@{ Id = "forced_staged_tiled_tails"; Filter = "Main.M1_ForcedStagedTiledTails_M035_N029_K019"; Flags = 17408; ExpectedDetail = "staged_upload_readback_tiled"; Description = "Non-multiple staged+tiled tails"; Group = "B" },
    [pscustomobject]@{ Id = "forced_staged_tiled_rectangular"; Filter = "Main.M1_ForcedStagedTiledRectangular_M128_N016_K064"; Flags = 17408; ExpectedDetail = "staged_upload_readback_tiled"; Description = "Rectangular staged+tiled correctness"; Group = "B" },
    [pscustomobject]@{ Id = "natural_auto_staged_tiled"; Filter = "Main.M1_NaturalAutoStagedTiled_M128_N128_K032"; Flags = 0; ExpectedDetail = "staged_upload_readback_tiled"; Description = "Natural auto-selected staged+tiled"; Group = "B" }
)

function Invoke-BenchCaseRun([object]$caseConfig, [string]$label, [int]$index) {
    $artifact = Join-Path $OutputDir ("{0}_{1}_{2:D2}.octagon" -f $caseConfig.Id, $label, $index)
    $stdoutPath = Join-Path $OutputDir ("{0}_{1}_{2:D2}.stdout.txt" -f $caseConfig.Id, $label, $index)
    $env:OCT_PROMETHEUS_REACTOR_TEST_FLAGS = [string]$caseConfig.Flags
    $stdout = & $OctPath bench $ExperimentPath --filter $caseConfig.Filter --octagon-out $artifact 2>&1
    $exitCode = $LASTEXITCODE
    $stdoutText = ($stdout | Out-String)
    [System.IO.File]::WriteAllText($stdoutPath, $stdoutText)
    if ($exitCode -ne 0) {
        throw "oct bench failed for $($caseConfig.Id) $label run $index with exit code $exitCode.`n$stdoutText"
    }
    return Parse-BenchmarkCase $artifact
}

function Invoke-AsyncRun([string]$label, [int]$index) {
    $artifact = Join-Path $OutputDir ("async_{0}_{1:D2}.octagon" -f $label, $index)
    $stdoutPath = Join-Path $OutputDir ("async_{0}_{1:D2}.stdout.txt" -f $label, $index)
    Remove-Item Env:OCT_PROMETHEUS_REACTOR_TEST_FLAGS -ErrorAction SilentlyContinue
    $stdout = & $OctPath prometheus-m1-async --octagon-out $artifact 2>&1
    $exitCode = $LASTEXITCODE
    $stdoutText = ($stdout | Out-String)
    [System.IO.File]::WriteAllText($stdoutPath, $stdoutText)
    if ($exitCode -ne 0) {
        throw "oct prometheus-m1-async failed during $label run $index with exit code $exitCode.`n$stdoutText"
    }
    return Parse-KeyValueLine $stdoutText
}

foreach ($case in $cases) {
    for ($i = 1; $i -le $WarmupRuns; $i++) {
        Write-Host ("Warmup {0} {1}/{2}" -f $case.Id, $i, $WarmupRuns)
        [void](Invoke-BenchCaseRun $case "warmup" $i)
    }
}

$measuredCaseResults = @{}
foreach ($case in $cases) {
    $measuredCaseResults[$case.Id] = New-Object System.Collections.Generic.List[object]
    for ($i = 1; $i -le $MeasuredRuns; $i++) {
        Write-Host ("Measured {0} {1}/{2}" -f $case.Id, $i, $MeasuredRuns)
        $measuredCaseResults[$case.Id].Add((Invoke-BenchCaseRun $case "measured" $i))
    }
}

for ($i = 1; $i -le $WarmupRuns; $i++) {
    Write-Host ("Warmup async {0}/{1}" -f $i, $WarmupRuns)
    [void](Invoke-AsyncRun "warmup" $i)
}

$measuredAsyncRuns = New-Object System.Collections.Generic.List[object]
for ($i = 1; $i -le $MeasuredRuns; $i++) {
    Write-Host ("Measured async {0}/{1}" -f $i, $MeasuredRuns)
    $measuredAsyncRuns.Add((Invoke-AsyncRun "measured" $i))
}

$summaryCases = New-Object System.Collections.Generic.List[object]
foreach ($case in $cases) {
    $runs = $measuredCaseResults[$case.Id]
    $first = $runs[0]
    foreach ($run in $runs) {
        if ($run.BackendRequested -ne $first.BackendRequested -or
            $run.BackendUsed -ne $first.BackendUsed -or
            $run.Status -ne $first.Status -or
            $run.Environment -ne $first.Environment -or
            $run.DetailCode -ne $first.DetailCode -or
            $run.DetailName -ne $first.DetailName) {
            throw "Inconsistent metadata across measured runs for $($case.Id)"
        }
        if (-not $run.Correctness) {
            throw "Correctness failure observed in measured run for $($case.Id)"
        }
    }
    if ($case.ExpectedDetail -ne "" -and $first.DetailName -ne $case.ExpectedDetail) {
        throw "Case $($case.Id) expected detail $($case.ExpectedDetail) but observed $($first.DetailName)"
    }

    $durations = @($runs | ForEach-Object { [int64]$_.DurationNs })
    $reportedWalls = @($runs | Where-Object { $_.ReportedWallNs -gt 0 } | ForEach-Object { [int64]$_.ReportedWallNs })
    $summaryCases.Add([pscustomobject]@{
        Id                   = $case.Id
        Group                = $case.Group
        Description          = $case.Description
        Filter               = $case.Filter
        Flags                = $case.Flags
        BackendRequested     = $first.BackendRequested
        BackendUsed          = $first.BackendUsed
        Status               = $first.Status
        Correctness          = $true
        Environment          = $first.Environment
        DetailCode           = $first.DetailCode
        DetailName           = $first.DetailName
        MedianDurationNs     = Get-Median $durations
        AverageDurationNs    = Get-Average $durations
        MedianReportedWallNs = if ($reportedWalls.Count -gt 0) { Get-Median $reportedWalls } else { [int64]0 }
        AverageReportedWallNs = if ($reportedWalls.Count -gt 0) { Get-Average $reportedWalls } else { [int64]0 }
    })
}

$asyncOutcomes = @($measuredAsyncRuns | ForEach-Object { $_["outcome"] } | Sort-Object -Unique)
$asyncEnvironments = @($measuredAsyncRuns | ForEach-Object { $_["vulkan_env"] } | Sort-Object -Unique)
$asyncSubmitDetails = @($measuredAsyncRuns | ForEach-Object { $_["submit_detail_name"] } | Sort-Object -Unique)
$asyncConsumeDetails = @($measuredAsyncRuns | ForEach-Object { $_["consume_detail_name"] } | Sort-Object -Unique)
$asyncWallValues = @($measuredAsyncRuns | Where-Object { $_["wall"] -and $_["wall"].EndsWith("ns") } | ForEach-Object { [int64]($_["wall"] -replace 'ns$', '') })
$asyncSummary = [pscustomobject]@{
    OutcomeConsistency   = ($asyncOutcomes.Count -eq 1)
    Outcome              = $asyncOutcomes -join ","
    Environment          = $asyncEnvironments -join ","
    SubmitDetailName     = $asyncSubmitDetails -join ","
    ConsumeDetailName    = $asyncConsumeDetails -join ","
    MedianWallNs         = if ($asyncWallValues.Count -gt 0) { Get-Median $asyncWallValues } else { [int64]0 }
    AverageWallNs        = if ($asyncWallValues.Count -gt 0) { Get-Average $asyncWallValues } else { [int64]0 }
    QueryLifecycle       = ((@($measuredAsyncRuns | ForEach-Object { $_["query_lifecycle"] } | Sort-Object -Unique)) -join ",")
    QueryAttempts        = ((@($measuredAsyncRuns | ForEach-Object { $_["query_attempts"] } | Sort-Object -Unique)) -join ",")
}

$stagedTiledLive = @($summaryCases | Where-Object { $_.DetailName -eq "staged_upload_readback_tiled" -or $_.DetailName -eq "staged_upload_tiled" })
$naturalAuto = @($summaryCases | Where-Object { $_.Id -eq "natural_auto_staged_tiled" })

$summaryObject = [pscustomobject]@{
    GeneratedAt     = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
    RepoRoot        = $RepoRoot
    OctPath         = $OctPath
    ExperimentPath  = $ExperimentPath
    ReactorPath     = $ReactorPath
    WarmupRuns      = $WarmupRuns
    MeasuredRuns    = $MeasuredRuns
    ComponentsValidated = @(
        "P8c staged memory path",
        "P8d tiled compute path",
        "P8d.1 staged+tiled reachability",
        "P8f judgment seam observability",
        "P8e/P8e.1 async lifecycle on real hardware when allowed"
    )
    SyncCases       = $summaryCases
    AsyncSummary    = $asyncSummary
    Conclusions     = [pscustomobject]@{
        StagedTiledLiveOnHardware = ($stagedTiledLive.Count -gt 0)
        NaturalAutoReachedStagedTiled = ($naturalAuto.Count -eq 1 -and $naturalAuto[0].DetailName -eq "staged_upload_readback_tiled")
        AsyncOutcome = $asyncSummary.Outcome
    }
}

$summaryJson = $summaryObject | ConvertTo-Json -Depth 8
[System.IO.File]::WriteAllText((Join-Path $OutputDir "summary.json"), $summaryJson)

$lines = New-Object System.Collections.Generic.List[string]
$lines.Add("# Prometheus benchmark harness M1 summary")
$lines.Add("")
$lines.Add(("Generated at: {0}" -f $summaryObject.GeneratedAt))
$lines.Add(("Warmup runs: {0}" -f $WarmupRuns))
$lines.Add(("Measured runs: {0}" -f $MeasuredRuns))
$lines.Add("")
$lines.Add("Validated reactor stack components:")
$lines.Add("- P8c staged memory path")
$lines.Add("- P8d tiled compute path")
$lines.Add("- P8d.1 staged+tiled reachability")
$lines.Add("- P8f judgment seam observability")
$lines.Add("- P8e/P8e.1 async lifecycle on real hardware where allowed")
$lines.Add("")
$lines.Add("## Sync cases")
$lines.Add("")
$lines.Add("| Group | Case | Detail | Status | Correctness | Env | Median outer | Median inner wall |")
$lines.Add("| --- | --- | --- | --- | --- | --- | --- | --- |")
foreach ($case in $summaryCases) {
    $inner = "n/a"
    if ($case.MedianReportedWallNs -gt 0) {
        $inner = Format-Ns $case.MedianReportedWallNs
    }
    $lines.Add((
        "| {0} | {1} | {2} ({3}) | {4} | {5} | {6} | {7} | {8} |" -f
        $case.Group,
        $case.Description,
        $case.DetailName,
        $case.DetailCode,
        $case.Status,
        $case.Correctness,
        $case.Environment,
        (Format-Ns $case.MedianDurationNs),
        $inner
    ))
}
$lines.Add("")
$lines.Add("## Async")
$lines.Add("")
$lines.Add(("Outcome: {0}" -f $asyncSummary.Outcome))
$lines.Add(("Environment: {0}" -f $asyncSummary.Environment))
$lines.Add(("Query lifecycle: {0}" -f $asyncSummary.QueryLifecycle))
$lines.Add(("Submit detail: {0}" -f $asyncSummary.SubmitDetailName))
$lines.Add(("Consume detail: {0}" -f $asyncSummary.ConsumeDetailName))
if ($asyncSummary.MedianWallNs -gt 0) {
    $lines.Add(("Median async wall: {0}" -f (Format-Ns $asyncSummary.MedianWallNs)))
}
$lines.Add("")
$lines.Add("## Conclusions")
$lines.Add("")
$lines.Add(("Staged+tiled live on real hardware: {0}" -f $summaryObject.Conclusions.StagedTiledLiveOnHardware))
$lines.Add(("Natural auto-selection reached staged+tiled: {0}" -f $summaryObject.Conclusions.NaturalAutoReachedStagedTiled))
$lines.Add(("Async path outcome on real hardware: {0}" -f $summaryObject.Conclusions.AsyncOutcome))
$lines.Add("")
$lines.Add("Scope intentionally left for later:")
$lines.Add("- threshold retuning")
$lines.Add("- new kernels")
$lines.Add("- judgment-engine redesign")
$lines.Add("- broad async redesign")
[System.IO.File]::WriteAllText((Join-Path $OutputDir "summary.md"), ($lines -join "`r`n") + "`r`n")

Write-Host "M1 summary written to:"
Write-Host ("  {0}" -f (Join-Path $OutputDir "summary.json"))
Write-Host ("  {0}" -f (Join-Path $OutputDir "summary.md"))
