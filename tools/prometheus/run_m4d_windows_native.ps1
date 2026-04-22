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
    if (@($cases).Count -eq 0) {
        throw "No BenchmarkCaseResult entries found in $path"
    }
    return $cases
}

function Parse-ChooseStrategySelections([string]$path) {
    $text = [System.IO.File]::ReadAllText($path)
    $pattern = 'M4dCaseSelection \{\s*Shape: "(?<Shape>[^"]+)"\s*M: \((?<M>\d+)\)\s*N: \((?<N>\d+)\)\s*K: \((?<K>\d+)\)\s*ChosenStrategy: "(?<ChosenStrategy>[^"]+)"\s*ChosenFamily: "(?<ChosenFamily>[^"]+)"\s*\}'
    $matches = [System.Text.RegularExpressions.Regex]::Matches($text, $pattern)
    $items = @{}
    foreach ($match in $matches) {
        $shape = $match.Groups["Shape"].Value
        $items[$shape] = [pscustomobject]@{
            Shape          = $shape
            M              = [int]$match.Groups["M"].Value
            N              = [int]$match.Groups["N"].Value
            K              = [int]$match.Groups["K"].Value
            ChosenStrategy = $match.Groups["ChosenStrategy"].Value
            ChosenFamily   = $match.Groups["ChosenFamily"].Value
        }
    }
    if ($items.Count -eq 0) {
        throw "No M4CaseSelection entries found in $path"
    }
    return $items
}

function Get-Median([int64[]]$values) {
    $sorted = [int64[]]@($values)
    if ($sorted.Count -eq 0) {
        return [int64]0
    }
    [System.Array]::Sort($sorted)
    $mid = [int][math]::Floor($sorted.Count / 2)
    if (($sorted.Count % 2) -eq 1) {
        return [int64]$sorted[$mid]
    }
    return [int64](($sorted[$mid - 1] + $sorted[$mid]) / 2)
}

function Get-Average([int64[]]$values) {
    $items = @($values)
    if ($items.Count -eq 0) {
        return [int64]0
    }
    [int64]$sum = 0
    foreach ($value in $items) {
        $sum += $value
    }
    return [int64]($sum / $items.Count)
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

function Family-ForStrategy([string]$strategy) {
    switch ($strategy) {
        "Baseline" { return "SingleCall" }
        "IKJ" { return "SingleCall" }
        "KIJ" { return "SingleCall" }
        "Blocked" { return "Blocked" }
        "BlockedIKJ" { return "Blocked" }
        "KBlocked" { return "KDecomposition" }
        "KBlockedIKJ" { return "KDecomposition" }
        "StagedIKJ" { return "KDecomposition" }
        default { throw "Unknown strategy label: $strategy" }
    }
}

function Representative-ForFamily([string]$family) {
    switch ($family) {
        "SingleCall" { return "Baseline" }
        "Blocked" { return "Blocked" }
        "KDecomposition" { return "KBlocked" }
        default { throw "Unknown family: $family" }
    }
}

function Family-Description([string]$family) {
    switch ($family) {
        "SingleCall" { return "SingleCallRep benchmark for single-call delegation (`Baseline`/`IKJ`/`KIJ`)" }
        "Blocked" { return "BlockedRep benchmark for output-tiled delegation (`Blocked`/`BlockedIKJ`)" }
        "KDecomposition" { return "KDecompositionRep benchmark for K-decomposition delegation (`KBlocked`/`KBlockedIKJ`/`StagedIKJ`)" }
        default { throw "Unknown family: $family" }
    }
}

function Family-ForRepresentative([string]$representative) {
    switch ($representative) {
        "SingleCallRep" { return "SingleCall" }
        "BlockedRep" { return "Blocked" }
        "KDecompositionRep" { return "KDecomposition" }
        default { throw "Unknown representative: $representative" }
    }
}

function Ranking-Stability([object[]]$rankings) {
    $items = @($rankings)
    if ($items.Count -eq 0) {
        return "no measured runs"
    }
    $first = (@($items[0]) -join " > ")
    $allSame = $true
    foreach ($ranking in $items) {
        if ((@($ranking) -join " > ") -ne $first) {
            $allSame = $false
            break
        }
    }
    if ($allSame) {
        return "stable across all runs: $first"
    }

    $tops = @($items | ForEach-Object { @($_)[0] })
    $uniqueTops = @($tops | Sort-Object -Unique)
    if ($uniqueTops.Count -eq 1) {
        return "winner stable ($($uniqueTops[0])) but full ordering varies"
    }
    return "winner varies across runs: " + ((@($items | ForEach-Object { @($_) -join " > " })) -join "; ")
}

if ($WarmupRuns -lt 0) {
    throw "WarmupRuns must be >= 0"
}
if ($MeasuredRuns -lt 3) {
    throw "MeasuredRuns must be >= 3"
}

$RepoRoot = (Resolve-Path $RepoRoot).Path

if ([string]::IsNullOrWhiteSpace($ExperimentPath)) {
    $ExperimentPath = Join-Path $RepoRoot "Experiments\\PrometheusSgemmAlgorithmLab\\M4"
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
    $OutputDir = Join-Path $RepoRoot "out\\prometheus\\sgemm_lab_m4d"
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

if ([string]::IsNullOrWhiteSpace($OctPath)) {
    $OctPath = Join-Path $OutputDir "oct_m4d.exe"
    Write-Host "Building fresh oct CLI for M4d..."
    & go build -o $OctPath .\cmd\oct
    if ($LASTEXITCODE -ne 0) {
        throw "go build ./cmd/oct failed with exit code $LASTEXITCODE"
    }
} else {
    $OctPath = Resolve-RepoPath $RepoRoot $OctPath
}

$shapeConfigs = @(
    [pscustomobject]@{ ShapeKey = "S16x16"; ShapeLabel = "16x16x16"; M = 16; N = 16; K = 16 },
    [pscustomobject]@{ ShapeKey = "S32x32"; ShapeLabel = "32x32x32"; M = 32; N = 32; K = 32 },
    [pscustomobject]@{ ShapeKey = "R16x64x8"; ShapeLabel = "16x64 * 64x8"; M = 16; N = 8; K = 64 },
    [pscustomobject]@{ ShapeKey = "R8x128x16"; ShapeLabel = "8x128 * 128x16"; M = 8; N = 16; K = 128 },
    [pscustomobject]@{ ShapeKey = "K8x8x256"; ShapeLabel = "8x8 * 256"; M = 8; N = 8; K = 256 },
    [pscustomobject]@{ ShapeKey = "K16x16x512"; ShapeLabel = "16x16 * 512"; M = 16; N = 16; K = 512 }
)

$familyConfigs = @(
    [pscustomobject]@{ Family = "SingleCall"; Representative = "SingleCallRep" },
    [pscustomobject]@{ Family = "Blocked"; Representative = "BlockedRep" },
    [pscustomobject]@{ Family = "KDecomposition"; Representative = "KDecompositionRep" }
)

function Invoke-ArtifactExport() {
    $stdout = & $OctPath artifact $ExperimentPath 2>&1
    $exitCode = $LASTEXITCODE
    $stdoutText = ($stdout | Out-String)
    [System.IO.File]::WriteAllText((Join-Path $OutputDir "artifact.stdout.txt"), $stdoutText)
    if ($exitCode -ne 0) {
        throw "oct artifact failed with exit code $exitCode.`n$stdoutText"
    }
}

function Invoke-BenchRun([string]$shapeKey, [string]$representative, [string]$phase, [int]$index) {
    $filter = "PrometheusSgemmAlgorithmLab.M4d_{0}_{1}" -f $shapeKey, $representative
    $stem = "{0}_{1}_{2}_{3:D2}" -f $shapeKey, $representative, $phase, $index
    $artifact = Join-Path $OutputDir ($stem + ".octagon")
    $stdoutPath = Join-Path $OutputDir ($stem + ".stdout.txt")
    $stdout = & $OctPath bench $ExperimentPath --filter $filter --octagon-out $artifact 2>&1
    $exitCode = $LASTEXITCODE
    $stdoutText = ($stdout | Out-String)
    [System.IO.File]::WriteAllText($stdoutPath, $stdoutText)
    if ($exitCode -ne 0) {
        throw "oct bench failed for $filter during $phase run $index with exit code $exitCode.`n$stdoutText"
    }

    $cases = Parse-BenchmarkRun $artifact
    if (@($cases).Count -ne 1) {
        throw "Expected exactly one benchmark case for $filter, found $(@($cases).Count)"
    }

    $case = @($cases)[0]
    if ($case.BackendUsed -ne "prometheus") {
        throw "Benchmark $filter did not use Prometheus backend. Used=$($case.BackendUsed) Status=$($case.Status)"
    }
    if ($case.Status -ne "ok") {
        throw "Benchmark $filter reported non-ok status: $($case.Status)"
    }

    return [pscustomobject]@{
        Filter = $filter
        ShapeKey = $shapeKey
        Representative = $representative
        Phase = $phase
        RunIndex = $index
        Name = $case.Name
        DurationNs = $case.DurationNs
        BackendRequested = $case.BackendRequested
        BackendUsed = $case.BackendUsed
        Status = $case.Status
        Environment = $case.Environment
        ReportedWallNs = $case.ReportedWallNs
        Artifact = $artifact
        StdoutPath = $stdoutPath
    }
}

Invoke-ArtifactExport

$chooserSourcePath = Join-Path $ExperimentPath "m4d_choose_strategy.octagon"
if (-not (Test-Path $chooserSourcePath)) {
    throw "Expected chooser artifact at $chooserSourcePath after oct artifact"
}
$chooserCopyPath = Join-Path $OutputDir "m4d_choose_strategy.octagon"
Copy-Item -Force $chooserSourcePath $chooserCopyPath
$chooserSelections = Parse-ChooseStrategySelections $chooserCopyPath

$warmups = New-Object System.Collections.Generic.List[object]
for ($i = 1; $i -le $WarmupRuns; $i++) {
    foreach ($shape in $shapeConfigs) {
        foreach ($family in $familyConfigs) {
            Write-Host ("Warmup {0}/{1}: {2} {3}" -f $i, $WarmupRuns, $shape.ShapeKey, $family.Representative)
            $warmups.Add((Invoke-BenchRun $shape.ShapeKey $family.Representative "warmup" $i))
        }
    }
}

$measured = New-Object System.Collections.Generic.List[object]
for ($i = 1; $i -le $MeasuredRuns; $i++) {
    foreach ($shape in $shapeConfigs) {
        foreach ($family in $familyConfigs) {
            Write-Host ("Measured {0}/{1}: {2} {3}" -f $i, $MeasuredRuns, $shape.ShapeKey, $family.Representative)
            $measured.Add((Invoke-BenchRun $shape.ShapeKey $family.Representative "measured" $i))
        }
    }
}

$shapeSummaries = New-Object System.Collections.Generic.List[object]
foreach ($shape in $shapeConfigs) {
    if (-not $chooserSelections.ContainsKey($shape.ShapeKey)) {
        throw "Chooser selections missing required shape $($shape.ShapeKey)"
    }
    $chooser = $chooserSelections[$shape.ShapeKey]
    $familyResults = New-Object System.Collections.Generic.List[object]
    $rankings = New-Object System.Collections.Generic.List[object]

    for ($runIndex = 1; $runIndex -le $MeasuredRuns; $runIndex++) {
        $runRows = @($measured | Where-Object { $_.ShapeKey -eq $shape.ShapeKey -and $_.RunIndex -eq $runIndex } | Sort-Object ReportedWallNs, DurationNs)
        if ($runRows.Count -ne $familyConfigs.Count) {
            throw "Expected $($familyConfigs.Count) measured rows for $($shape.ShapeKey) run $runIndex, found $($runRows.Count)"
        }
        $rankings.Add(@($runRows | ForEach-Object { Family-ForRepresentative $_.Representative }))
    }

    foreach ($family in $familyConfigs) {
        $rows = @($measured | Where-Object { $_.ShapeKey -eq $shape.ShapeKey -and $_.Representative -eq $family.Representative })
        if ($rows.Count -ne $MeasuredRuns) {
            throw "Expected $MeasuredRuns measured rows for $($shape.ShapeKey) $($family.Representative), found $($rows.Count)"
        }
        $first = $rows[0]
        foreach ($row in $rows) {
            if ($row.BackendRequested -ne $first.BackendRequested -or
                $row.BackendUsed -ne $first.BackendUsed -or
                $row.Status -ne $first.Status -or
                $row.Environment -ne $first.Environment) {
                throw "Inconsistent metadata for $($shape.ShapeKey) $($family.Representative)"
            }
        }
        $durations = @($rows | ForEach-Object { [int64]$_.DurationNs })
        $reportedWalls = @($rows | ForEach-Object { [int64]$_.ReportedWallNs })
        $familyResults.Add([pscustomobject]@{
            Family = $family.Family
            Representative = $family.Representative
            Description = Family-Description $family.Family
            BackendRequested = $first.BackendRequested
            BackendUsed = $first.BackendUsed
            Status = $first.Status
            Environment = $first.Environment
            ReportedWallNs = $reportedWalls
            DurationNs = $durations
            MedianReportedWallNs = Get-Median $reportedWalls
            AverageReportedWallNs = Get-Average $reportedWalls
            MedianDurationNs = Get-Median $durations
            AverageDurationNs = Get-Average $durations
            MinReportedWallNs = [int64](($reportedWalls | Measure-Object -Minimum).Minimum)
            MaxReportedWallNs = [int64](($reportedWalls | Measure-Object -Maximum).Maximum)
        })
    }

    $orderedFamilies = @($familyResults | Sort-Object MedianReportedWallNs, MedianDurationNs)
    $fastest = $orderedFamilies[0]
    $chooserFamily = Family-ForStrategy $chooser.ChosenStrategy

    $shapeSummaries.Add([pscustomobject]@{
        ShapeKey = $shape.ShapeKey
        ShapeLabel = $shape.ShapeLabel
        M = $shape.M
        N = $shape.N
        K = $shape.K
        ChosenStrategy = $chooser.ChosenStrategy
        ChosenFamily = $chooser.ChosenFamily
        ChosenRepresentative = Representative-ForFamily $chooserFamily
        FastestFamily = $fastest.Family
        FastestRepresentative = $fastest.Representative
        RankingStability = Ranking-Stability $rankings
        FamilyResults = $orderedFamilies
        RankingsByRun = $rankings
    })
}

$summaryObject = [pscustomobject]@{
    GeneratedAt = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
    RepoRoot = $RepoRoot
    OctPath = $OctPath
    ExperimentPath = $ExperimentPath
    ReactorPath = $ReactorPath
    WarmupRuns = $WarmupRuns
    MeasuredRuns = $MeasuredRuns
    RetainedFamilies = @(
        "SingleCall represented by SingleCallRep; chooser aliases Baseline/IKJ/KIJ collapse here after M4c",
        "Blocked represented by BlockedRep; chooser aliases Blocked/BlockedIKJ collapse here after M4c",
        "KDecomposition represented by KDecompositionRep; chooser aliases KBlocked/KBlockedIKJ/StagedIKJ collapse here after M4c"
    )
    Shapes = $shapeSummaries
    MeasuredRunsRaw = $measured
}

$summaryJson = $summaryObject | ConvertTo-Json -Depth 8
[System.IO.File]::WriteAllText((Join-Path $OutputDir "summary.json"), $summaryJson)

$reactorDisplay = $ReactorPath
if ($reactorDisplay.StartsWith($RepoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    $reactorDisplay = $reactorDisplay.Substring($RepoRoot.Length).TrimStart('\')
}

$lines = New-Object System.Collections.Generic.List[string]
$lines.Add("# M4d Windows-Native Prometheus Summary")
$lines.Add("")
$lines.Add("Environment:")
$lines.Add("- Windows")
$lines.Add("- Native NVIDIA Vulkan environment required")
$lines.Add(('- `OCT_PROMETHEUS_REACTOR={0}`' -f $reactorDisplay))
$lines.Add('- `CGO_ENABLED=1`')
$lines.Add(('- `CC={0}`' -f $CCPath))
$lines.Add(('- `CXX={0}`' -f $CXXPath))
$lines.Add(('- Fresh CLI built from current source: `{0}`' -f $OctPath))
$lines.Add("")
$lines.Add("Retained bridged strategy families tested:")
$lines.Add('- `SingleCallRep` benchmark for single-call delegation (`Baseline`/`IKJ`/`KIJ`)')
$lines.Add('- `BlockedRep` benchmark for output-block decomposition (`Blocked`/`BlockedIKJ`)')
$lines.Add('- `KDecompositionRep` benchmark for K-decomposition (`KBlocked`/`KBlockedIKJ`/`StagedIKJ`)')
$lines.Add("")
$lines.Add("Note:")
$lines.Add('- `StagedIKJ` is not benchmarked as a separate family because M4c made it an execution alias of `KBlocked`; staged viability in this summary is therefore judged through the shared K-decomposition family rather than a fake distinct row.')
$lines.Add("")

foreach ($shape in $shapeSummaries) {
    $lines.Add(("## Shape {0}" -f $shape.ShapeLabel))
    $lines.Add("")
    $lines.Add(('ChosenStrategy: `{0}`' -f $shape.ChosenStrategy))
    $lines.Add(('ChosenFamily: `{0}` (represented by `{1}` in the retained M4d surface)' -f $shape.ChosenFamily, $shape.ChosenRepresentative))
    $lines.Add(('FastestObserved: `{0}` via `{1}` representative at median reported wall `{2}`' -f $shape.FastestFamily, $shape.FastestRepresentative, (Format-Ns $shape.FamilyResults[0].MedianReportedWallNs)))
    $lines.Add(("RankingStability: {0}" -f $shape.RankingStability))
    $lines.Add("")
    $lines.Add("| Family | Representative | BackendUsed | Status | Environment | ReportedWallNs | Median wall | Median outer |")
    $lines.Add("| --- | --- | --- | --- | --- | --- | --- | --- |")
    foreach ($family in $shape.FamilyResults) {
        $wallSeries = ($family.ReportedWallNs | ForEach-Object { $_.ToString() }) -join ", "
        $lines.Add((
            '| {0} | {1} | {2} | {3} | {4} | `{5}` | {6} | {7} |' -f
            $family.Family,
            $family.Representative,
            $family.BackendUsed,
            $family.Status,
            $family.Environment,
            $wallSeries,
            (Format-Ns $family.MedianReportedWallNs),
            (Format-Ns $family.MedianDurationNs)
        ))
    }
    $lines.Add("")

    if ($shape.ChosenFamily -eq $shape.FastestFamily) {
        $lines.Add("- Current chooser is directionally right on this shape once M4c alias collapse is respected.")
    } else {
        $lines.Add(("- Current chooser is directionally wrong on this shape: it selects `{0}` but the fastest retained family is `{1}`." -f $shape.ChosenFamily, $shape.FastestFamily))
    }

    $fastestRange = $shape.FamilyResults[0].MaxReportedWallNs - $shape.FamilyResults[0].MinReportedWallNs
    if ($fastestRange -gt $shape.FamilyResults[0].MedianReportedWallNs / 2) {
        $lines.Add("- Variance note: the fastest family shows visible wall-time spread, so the ordering should be treated as suggestive rather than absolute.")
    } else {
        $lines.Add("- Variance note: the fastest family stays reasonably tight across the three measured runs.")
    }
    $lines.Add("")
}

$chooserMismatchShapes = @($shapeSummaries | Where-Object { $_.ChosenFamily -ne $_.FastestFamily })
$stagedWins = @($shapeSummaries | Where-Object { $_.FastestFamily -eq "KDecomposition" })
$lines.Add("## Overall")
$lines.Add("")
$lines.Add(("- Chooser mismatches: {0}/{1} required shapes" -f $chooserMismatchShapes.Count, $shapeSummaries.Count))
if ($stagedWins.Count -gt 0) {
    $lines.Add(("- K-decomposition wins on: {0}" -f (($stagedWins | ForEach-Object { $_.ShapeLabel }) -join ", ")))
} else {
    $lines.Add("- K-decomposition does not win any required shape in this run.")
}
$lines.Add("- `StagedIKJ` remains non-distinct after M4c; M5 should not score it separately from the shared K-decomposition family unless the backend path becomes structurally different again.")

[System.IO.File]::WriteAllText((Join-Path $OutputDir "summary.md"), ($lines -join "`r`n") + "`r`n")

Write-Host "M4d summary written to:"
Write-Host ("  {0}" -f (Join-Path $OutputDir "summary.json"))
Write-Host ("  {0}" -f (Join-Path $OutputDir "summary.md"))
