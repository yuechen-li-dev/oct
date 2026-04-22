param(
    [string]$RepoRoot = "",
    [string]$OctPath = "",
    [string]$ReactorPath = "",
    [string]$CCPath = "",
    [string]$CXXPath = "",
    [int]$WarmupRuns = 1,
    [int]$MeasuredRuns = 3,
    [string]$OutDir = ""
)

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $false

function Resolve-RepoRoot([string]$hint) {
    if (-not [string]::IsNullOrWhiteSpace($hint)) {
        return (Resolve-Path $hint).Path
    }
    if ($PSScriptRoot) {
        return (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
    }
    return (Get-Location).Path
}

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

function Invoke-OctCommand([string[]]$arguments, [string]$stdoutPath, [string]$stderrPath) {
    if (Test-Path $stdoutPath) { Remove-Item -Force $stdoutPath }
    if (Test-Path $stderrPath) { Remove-Item -Force $stderrPath }
    $process = Start-Process -FilePath $OctPath -ArgumentList $arguments -NoNewWindow -Wait -PassThru -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
    $stdoutText = if (Test-Path $stdoutPath) { [System.IO.File]::ReadAllText($stdoutPath) } else { "" }
    $stderrText = if (Test-Path $stderrPath) { [System.IO.File]::ReadAllText($stderrPath) } else { "" }
    return [pscustomobject]@{
        ExitCode   = $process.ExitCode
        StdoutText = $stdoutText
        StderrText = $stderrText
        Combined   = $stdoutText + $stderrText
    }
}

function Parse-BenchmarkRun([string]$path) {
    $text = [System.IO.File]::ReadAllText($path)
    $pattern = 'BenchmarkCaseResult \{\s*Name: "(?<Name>[^"]+)"\s*DurationNs: \((?<DurationNs>\d+)\)\s*BackendRequested: "(?<BackendRequested>[^"]+)"\s*BackendUsed: "(?<BackendUsed>[^"]+)"\s*Status: "(?<Status>[^"]+)"\s*Environment: "(?<Environment>[^"]+)"\s*ReportedWallNs: \((?<ReportedWallNs>\d+)\)\s*\}'
    $matches = [System.Text.RegularExpressions.Regex]::Matches($text, $pattern)
    $cases = New-Object System.Collections.Generic.List[object]
    foreach ($match in $matches) {
        $cases.Add([pscustomobject]@{
            Name             = $match.Groups["Name"].Value
            DurationNs       = [int64]$match.Groups["DurationNs"].Value
            BackendRequested = $match.Groups["BackendRequested"].Value
            BackendUsed      = $match.Groups["BackendUsed"].Value
            Status           = $match.Groups["Status"].Value
            Environment      = $match.Groups["Environment"].Value
            ReportedWallNs   = [int64]$match.Groups["ReportedWallNs"].Value
        })
    }
    if ($cases.Count -eq 0) {
        throw "No BenchmarkCaseResult entries found in $path"
    }
    return $cases.ToArray()
}

function Parse-M6DecisionSelections([string]$path) {
    $text = [System.IO.File]::ReadAllText($path)
    $pattern = 'M6DecisionProbeSelection \{\s*Shape: "(?<Shape>[^"]+)"\s*M: \((?<M>\d+)\)\s*N: \((?<N>\d+)\)\s*K: \((?<K>\d+)\)\s*Strategy: "(?<Strategy>[^"]+)"\s*Family: "(?<Family>[^"]+)"\s*BlockSize: \((?<BlockSize>\d+)\)\s*KBlock: \((?<KBlock>\d+)\)\s*StabilizedStrategy: "(?<StabilizedStrategy>[^"]+)"\s*StabilizedFamily: "(?<StabilizedFamily>[^"]+)"\s*StabilizedBlockSize: \((?<StabilizedBlockSize>\d+)\)\s*StabilizedKBlock: \((?<StabilizedKBlock>\d+)\)\s*\}'
    $matches = [System.Text.RegularExpressions.Regex]::Matches($text, $pattern)
    $items = @{}
    foreach ($match in $matches) {
        $shape = $match.Groups["Shape"].Value
        $items[$shape] = [pscustomobject]@{
            Shape               = $shape
            M                   = [int]$match.Groups["M"].Value
            N                   = [int]$match.Groups["N"].Value
            K                   = [int]$match.Groups["K"].Value
            Strategy            = $match.Groups["Strategy"].Value
            Family              = $match.Groups["Family"].Value
            BlockSize           = [int]$match.Groups["BlockSize"].Value
            KBlock              = [int]$match.Groups["KBlock"].Value
            StabilizedStrategy  = $match.Groups["StabilizedStrategy"].Value
            StabilizedFamily    = $match.Groups["StabilizedFamily"].Value
            StabilizedBlockSize = [int]$match.Groups["StabilizedBlockSize"].Value
            StabilizedKBlock    = [int]$match.Groups["StabilizedKBlock"].Value
        }
    }
    if ($items.Count -eq 0) {
        throw "No M6DecisionProbeSelection entries found in $path"
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
    if ($value -ge 1000000000) { return ("{0:N3}s" -f ($value / 1e9)) }
    if ($value -ge 1000000) { return ("{0:N3}ms" -f ($value / 1e6)) }
    if ($value -ge 1000) { return ("{0:N3}us" -f ($value / 1e3)) }
    return "$value ns"
}

function Parse-M6BenchName([string]$name) {
    $pattern = '^PrometheusSgemmAlgorithmLab\.M6a_(?<Shape>[^_]+)_(?<Family>SingleCall|Blocked|KDecomposition)(?:_(?<ParamType>B|K)(?<Param>\d+))?$'
    $match = [System.Text.RegularExpressions.Regex]::Match($name, $pattern)
    if (-not $match.Success) {
        throw "Unexpected M6 benchmark case name: $name"
    }
    $family = $match.Groups["Family"].Value
    $paramType = $match.Groups["ParamType"].Value
    $param = 0
    if ($match.Groups["Param"].Success) {
        $param = [int]$match.Groups["Param"].Value
    }
    $blockSize = 0
    $kBlock = 0
    if ($paramType -eq "B") {
        $blockSize = $param
    } elseif ($paramType -eq "K") {
        $kBlock = $param
    }
    return [pscustomobject]@{
        ShapeKey   = $match.Groups["Shape"].Value
        Family     = $family
        BlockSize  = $blockSize
        KBlock     = $kBlock
        ConfigKey  = if ($family -eq "SingleCall") { "SingleCall" } elseif ($family -eq "Blocked") { "Blocked:B$blockSize" } else { "KDecomposition:K$kBlock" }
    }
}

function Config-Label([object]$config) {
    if ($config.Family -eq "SingleCall") {
        return "SingleCall"
    }
    if ($config.Family -eq "Blocked") {
        return "Blocked(blockSize=$($config.BlockSize))"
    }
    return "KDecomposition(kBlock=$($config.KBlock))"
}

function Family-Winner-Stability([object[]]$winners) {
    $items = @($winners)
    if ($items.Count -eq 0) {
        return "no measured runs"
    }
    $unique = @($items | Sort-Object -Unique)
    if ($unique.Count -eq 1) {
        return "stable winner: $($unique[0])"
    }
    return "winner varies across runs: " + (($items) -join ", ")
}

if ($WarmupRuns -lt 0) {
    throw "WarmupRuns must be >= 0"
}
if ($MeasuredRuns -lt 3) {
    throw "MeasuredRuns must be >= 3"
}

$repo = Resolve-RepoRoot $RepoRoot
$expPath = Join-Path $repo "Experiments\PrometheusSgemmAlgorithmLab\M4"

if ([string]::IsNullOrWhiteSpace($OutDir)) {
    $OutDir = Join-Path $repo "out\prometheus\sgemm_lab_m6a"
} elseif (-not [System.IO.Path]::IsPathRooted($OutDir)) {
    $OutDir = Join-Path $repo $OutDir
}
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

$ReactorPath = Resolve-OptionalTool $repo $ReactorPath @(
    (Join-Path $repo "internal\prometheus\reactor\prometheus_reactor.dll"),
    (Join-Path $repo "out\prometheus\native\prometheus_reactor.dll")
) "Prometheus reactor DLL"

$CCPath = Resolve-OptionalTool $repo $CCPath @(
    "C:\msys64\ucrt64\bin\gcc.exe",
    "C:\Users\yuech\mingw64\bin\gcc.exe",
    "C:\Program Files\GNU Octave\Octave-9.4.0\mingw64\bin\gcc.exe"
) "C compiler"

$CXXPath = Resolve-OptionalTool $repo $CXXPath @(
    "C:\msys64\ucrt64\bin\g++.exe",
    "C:\Users\yuech\mingw64\bin\g++.exe",
    "C:\Program Files\GNU Octave\Octave-9.4.0\mingw64\bin\g++.exe"
) "C++ compiler"

$env:CGO_ENABLED = "1"
$env:CC = $CCPath
$env:CXX = $CXXPath
$env:OCT_PROMETHEUS_REACTOR = $ReactorPath
$toolchainBin = Split-Path -Parent $CCPath
if (-not [string]::IsNullOrWhiteSpace($toolchainBin)) {
    $env:PATH = $toolchainBin + ";" + $env:PATH
}

if ([string]::IsNullOrWhiteSpace($OctPath)) {
    $OctPath = Join-Path $OutDir "oct_m6a.exe"
    Write-Host "Building fresh oct CLI for M6a..."
    & go build -o $OctPath .\cmd\oct
    if ($LASTEXITCODE -ne 0) {
        throw "go build ./cmd/oct failed with exit code $LASTEXITCODE"
    }
} else {
    $OctPath = Resolve-RepoPath $repo $OctPath
}

$shapeConfigs = @(
    [pscustomobject]@{ ShapeKey = "S8x8x8"; ShapeLabel = "8x8x8"; M = 8; N = 8; K = 8; Region = "small-square" },
    [pscustomobject]@{ ShapeKey = "S12x12x12"; ShapeLabel = "12x12x12"; M = 12; N = 12; K = 12; Region = "small-square" },
    [pscustomobject]@{ ShapeKey = "S16x16x16"; ShapeLabel = "16x16x16"; M = 16; N = 16; K = 16; Region = "small-square" },
    [pscustomobject]@{ ShapeKey = "S20x20x20"; ShapeLabel = "20x20x20"; M = 20; N = 20; K = 20; Region = "small-square" },
    [pscustomobject]@{ ShapeKey = "S24x24x24"; ShapeLabel = "24x24x24"; M = 24; N = 24; K = 24; Region = "balanced-medium" },
    [pscustomobject]@{ ShapeKey = "S32x32x32"; ShapeLabel = "32x32x32"; M = 32; N = 32; K = 32; Region = "balanced-medium" },
    [pscustomobject]@{ ShapeKey = "S40x40x40"; ShapeLabel = "40x40x40"; M = 40; N = 40; K = 40; Region = "balanced-medium" },
    [pscustomobject]@{ ShapeKey = "S48x48x48"; ShapeLabel = "48x48x48"; M = 48; N = 48; K = 48; Region = "balanced-medium" },
    [pscustomobject]@{ ShapeKey = "R16x64x8"; ShapeLabel = "16x64 * 64x8"; M = 16; N = 8; K = 64; Region = "rectangular-k-heavy" },
    [pscustomobject]@{ ShapeKey = "R8x128x16"; ShapeLabel = "8x128 * 128x16"; M = 8; N = 16; K = 128; Region = "rectangular-k-heavy" },
    [pscustomobject]@{ ShapeKey = "K8x8x256"; ShapeLabel = "8x8x256"; M = 8; N = 8; K = 256; Region = "rectangular-k-heavy" },
    [pscustomobject]@{ ShapeKey = "K12x12x192"; ShapeLabel = "12x12x192"; M = 12; N = 12; K = 192; Region = "rectangular-k-heavy" },
    [pscustomobject]@{ ShapeKey = "K16x16x384"; ShapeLabel = "16x16x384"; M = 16; N = 16; K = 384; Region = "rectangular-k-heavy" },
    [pscustomobject]@{ ShapeKey = "K16x16x512"; ShapeLabel = "16x16x512"; M = 16; N = 16; K = 512; Region = "rectangular-k-heavy" }
)

$configSpecs = @(
    [pscustomobject]@{ Family = "SingleCall"; BlockSize = 0; KBlock = 0; Suffix = "SingleCall"; ConfigKey = "SingleCall" },
    [pscustomobject]@{ Family = "Blocked"; BlockSize = 2; KBlock = 0; Suffix = "Blocked_B2"; ConfigKey = "Blocked:B2" },
    [pscustomobject]@{ Family = "Blocked"; BlockSize = 4; KBlock = 0; Suffix = "Blocked_B4"; ConfigKey = "Blocked:B4" },
    [pscustomobject]@{ Family = "Blocked"; BlockSize = 8; KBlock = 0; Suffix = "Blocked_B8"; ConfigKey = "Blocked:B8" },
    [pscustomobject]@{ Family = "Blocked"; BlockSize = 16; KBlock = 0; Suffix = "Blocked_B16"; ConfigKey = "Blocked:B16" },
    [pscustomobject]@{ Family = "KDecomposition"; BlockSize = 0; KBlock = 4; Suffix = "KDecomposition_K4"; ConfigKey = "KDecomposition:K4" },
    [pscustomobject]@{ Family = "KDecomposition"; BlockSize = 0; KBlock = 8; Suffix = "KDecomposition_K8"; ConfigKey = "KDecomposition:K8" },
    [pscustomobject]@{ Family = "KDecomposition"; BlockSize = 0; KBlock = 16; Suffix = "KDecomposition_K16"; ConfigKey = "KDecomposition:K16" },
    [pscustomobject]@{ Family = "KDecomposition"; BlockSize = 0; KBlock = 32; Suffix = "KDecomposition_K32"; ConfigKey = "KDecomposition:K32" }
)

$expectedConfigCount = 9

function Invoke-ArtifactExport() {
    $stdoutPath = Join-Path $OutDir "artifact.stdout.raw.txt"
    $stderrPath = Join-Path $OutDir "artifact.stderr.txt"
    $result = Invoke-OctCommand @("artifact", $expPath) $stdoutPath $stderrPath
    $exitCode = $result.ExitCode
    $stdoutText = $result.Combined
    [System.IO.File]::WriteAllText((Join-Path $OutDir "artifact.stdout.txt"), $stdoutText)
    if ($exitCode -ne 0) {
        throw "oct artifact failed with exit code $exitCode.`n$stdoutText"
    }
}

function Invoke-BenchShapeRun([string]$shapeKey, [string]$phase, [int]$index) {
    $filter = "PrometheusSgemmAlgorithmLab.M6a_{0}_" -f $shapeKey
    $stem = "{0}_{1}_{2:D2}" -f $shapeKey, $phase, $index
    $artifact = Join-Path $OutDir ($stem + ".octagon")
    $stdoutPath = Join-Path $OutDir ($stem + ".stdout.txt")
    $stderrPath = Join-Path $OutDir ($stem + ".stderr.txt")
    $result = Invoke-OctCommand @("bench", $expPath, "--filter", $filter, "--octagon-out", $artifact) $stdoutPath $stderrPath
    $exitCode = $result.ExitCode
    $stdoutText = $result.Combined
    [System.IO.File]::WriteAllText($stdoutPath, $stdoutText)
    if ($exitCode -ne 0) {
        Write-Host ("Batch run failed for {0} {1} #{2}; falling back to per-config runs." -f $shapeKey, $phase, $index)
        $rows = New-Object System.Collections.Generic.List[object]
        foreach ($config in $configSpecs) {
            $rows.Add((Invoke-BenchConfigRun $shapeKey $config $phase $index))
        }
        return $rows.ToArray()
    }

    $cases = Parse-BenchmarkRun $artifact
    if (@($cases).Count -ne $expectedConfigCount) {
        throw "Expected $expectedConfigCount benchmark cases for $filter, found $(@($cases).Count)"
    }

    $parsed = New-Object System.Collections.Generic.List[object]
    foreach ($case in $cases) {
        if ($case.BackendUsed -ne "prometheus") {
            throw "Benchmark $($case.Name) did not use Prometheus backend. Used=$($case.BackendUsed) Status=$($case.Status)"
        }
        if ($case.Status -ne "ok") {
            throw "Benchmark $($case.Name) reported non-ok status: $($case.Status)"
        }
        if ($case.Environment -ne "windows_native_vulkan") {
            throw "Benchmark $($case.Name) did not report windows_native_vulkan. Environment=$($case.Environment)"
        }
        $config = Parse-M6BenchName $case.Name
        if ($config.ShapeKey -ne $shapeKey) {
            throw "Benchmark case shape mismatch. Expected $shapeKey, got $($config.ShapeKey)"
        }
        $parsed.Add([pscustomobject]@{
            ShapeKey          = $shapeKey
            Phase             = $phase
            RunIndex          = $index
            Name              = $case.Name
            Family            = $config.Family
            BlockSize         = $config.BlockSize
            KBlock            = $config.KBlock
            ConfigKey         = $config.ConfigKey
            DurationNs        = $case.DurationNs
            BackendRequested  = $case.BackendRequested
            BackendUsed       = $case.BackendUsed
            Status            = $case.Status
            Environment       = $case.Environment
            ReportedWallNs    = $case.ReportedWallNs
            Artifact          = $artifact
            StdoutPath        = $stdoutPath
        })
    }

    $configKeys = @($parsed | ForEach-Object { $_.ConfigKey } | Sort-Object -Unique)
    if ($configKeys.Count -ne $expectedConfigCount) {
        throw "Expected $expectedConfigCount unique M6 configs for $shapeKey, found $($configKeys.Count)"
    }
    return $parsed.ToArray()
}

function Invoke-BenchConfigRun([string]$shapeKey, [object]$config, [string]$phase, [int]$index) {
    $benchmarkName = "PrometheusSgemmAlgorithmLab.M6a_{0}_{1}" -f $shapeKey, $config.Suffix
    $stem = "{0}_{1}_{2}_{3:D2}" -f $shapeKey, ($config.Suffix -replace '[^A-Za-z0-9_]', '_'), $phase, $index
    $artifact = Join-Path $OutDir ($stem + ".octagon")
    $stdoutPath = Join-Path $OutDir ($stem + ".stdout.txt")
    $stderrPath = Join-Path $OutDir ($stem + ".stderr.txt")
    $result = Invoke-OctCommand @("bench", $expPath, "--filter", $benchmarkName, "--octagon-out", $artifact) $stdoutPath $stderrPath
    $exitCode = $result.ExitCode
    $stdoutText = $result.Combined
    [System.IO.File]::WriteAllText($stdoutPath, $stdoutText)

    if ($exitCode -ne 0) {
        return [pscustomobject]@{
            ShapeKey          = $shapeKey
            Phase             = $phase
            RunIndex          = $index
            Name              = $benchmarkName
            Family            = $config.Family
            BlockSize         = $config.BlockSize
            KBlock            = $config.KBlock
            ConfigKey         = $config.ConfigKey
            DurationNs        = [int64]-1
            BackendRequested  = "prometheus"
            BackendUsed       = "prometheus"
            Status            = "execution_failure"
            Environment       = "windows_native_vulkan"
            ReportedWallNs    = [int64]-1
            Artifact          = $artifact
            StdoutPath        = $stdoutPath
            Failure           = $true
            FailureSummary    = ($stdoutText.Trim())
        }
    }

    $cases = Parse-BenchmarkRun $artifact
    if (@($cases).Count -ne 1) {
        throw "Expected exactly one benchmark case for $benchmarkName, found $(@($cases).Count)"
    }
    $case = @($cases)[0]
    if ($case.BackendUsed -ne "prometheus") {
        throw "Benchmark $benchmarkName did not use Prometheus backend. Used=$($case.BackendUsed) Status=$($case.Status)"
    }
    if ($case.Status -ne "ok") {
        throw "Benchmark $benchmarkName reported non-ok status: $($case.Status)"
    }
    if ($case.Environment -ne "windows_native_vulkan") {
        throw "Benchmark $benchmarkName did not report windows_native_vulkan. Environment=$($case.Environment)"
    }

    return [pscustomobject]@{
        ShapeKey          = $shapeKey
        Phase             = $phase
        RunIndex          = $index
        Name              = $case.Name
        Family            = $config.Family
        BlockSize         = $config.BlockSize
        KBlock            = $config.KBlock
        ConfigKey         = $config.ConfigKey
        DurationNs        = $case.DurationNs
        BackendRequested  = $case.BackendRequested
        BackendUsed       = $case.BackendUsed
        Status            = $case.Status
        Environment       = $case.Environment
        ReportedWallNs    = $case.ReportedWallNs
        Artifact          = $artifact
        StdoutPath        = $stdoutPath
        Failure           = $false
        FailureSummary    = ""
    }
}

Write-Host "== M6a artifact pass (decision boundary probe) =="
Invoke-ArtifactExport

$artifactSource = Join-Path $repo "Experiments\PrometheusSgemmAlgorithmLab\M4\m6_decision_probe.octagon"
if (-not (Test-Path $artifactSource)) {
    throw "Expected artifact missing: $artifactSource"
}
$artifactCopy = Join-Path $OutDir "m6_decision_probe.octagon"
Copy-Item -Force $artifactSource $artifactCopy
$decisionSelections = Parse-M6DecisionSelections $artifactCopy

$warmups = New-Object System.Collections.Generic.List[object]
for ($i = 1; $i -le $WarmupRuns; $i++) {
    foreach ($shape in $shapeConfigs) {
        Write-Host ("Warmup {0}/{1}: {2}" -f $i, $WarmupRuns, $shape.ShapeKey)
        foreach ($row in (Invoke-BenchShapeRun $shape.ShapeKey "warmup" $i)) {
            $warmups.Add($row)
        }
    }
}

$measured = New-Object System.Collections.Generic.List[object]
for ($i = 1; $i -le $MeasuredRuns; $i++) {
    foreach ($shape in $shapeConfigs) {
        Write-Host ("Measured {0}/{1}: {2}" -f $i, $MeasuredRuns, $shape.ShapeKey)
        foreach ($row in (Invoke-BenchShapeRun $shape.ShapeKey "measured" $i)) {
            $measured.Add($row)
        }
    }
}

$shapeSummaries = New-Object System.Collections.Generic.List[object]
foreach ($shape in $shapeConfigs) {
    if (-not $decisionSelections.ContainsKey($shape.ShapeKey)) {
        throw "Decision selections missing required shape $($shape.ShapeKey)"
    }
    $decision = $decisionSelections[$shape.ShapeKey]
    $rowsForShape = @($measured | Where-Object { $_.ShapeKey -eq $shape.ShapeKey })
    if ($rowsForShape.Count -ne ($MeasuredRuns * $expectedConfigCount)) {
        throw "Expected $($MeasuredRuns * $expectedConfigCount) measured rows for $($shape.ShapeKey), found $($rowsForShape.Count)"
    }

    $configSummaries = New-Object System.Collections.Generic.List[object]
    $winnersByRun = New-Object System.Collections.Generic.List[string]
    for ($runIndex = 1; $runIndex -le $MeasuredRuns; $runIndex++) {
        $runRows = @($rowsForShape | Where-Object { $_.RunIndex -eq $runIndex })
        if ($runRows.Count -ne $expectedConfigCount) {
            throw "Expected $expectedConfigCount rows for $($shape.ShapeKey) run $runIndex, found $($runRows.Count)"
        }
        $successfulRunRows = @($runRows | Where-Object { -not $_.Failure } | Sort-Object ReportedWallNs, DurationNs)
        if ($successfulRunRows.Count -eq 0) {
            $winnersByRun.Add("no successful config")
        } else {
            $winnersByRun.Add((Config-Label $successfulRunRows[0]))
        }
    }

    $configKeys = @($rowsForShape | ForEach-Object { $_.ConfigKey } | Sort-Object -Unique)
    foreach ($configKey in $configKeys) {
        $rows = @($rowsForShape | Where-Object { $_.ConfigKey -eq $configKey })
        if ($rows.Count -ne $MeasuredRuns) {
            throw "Expected $MeasuredRuns rows for config $configKey on $($shape.ShapeKey), found $($rows.Count)"
        }
        $first = $rows[0]
        $successfulRows = @($rows | Where-Object { -not $_.Failure })
        $failedRows = @($rows | Where-Object { $_.Failure })
        $reportedWalls = @($successfulRows | ForEach-Object { [int64]$_.ReportedWallNs })
        $durations = @($successfulRows | ForEach-Object { [int64]$_.DurationNs })
        $medianReportedWall = if ($reportedWalls.Count -gt 0) { Get-Median $reportedWalls } else { [int64]::MaxValue }
        $averageReportedWall = if ($reportedWalls.Count -gt 0) { Get-Average $reportedWalls } else { [int64]::MaxValue }
        $medianDuration = if ($durations.Count -gt 0) { Get-Median $durations } else { [int64]::MaxValue }
        $minReportedWall = if ($reportedWalls.Count -gt 0) { [int64](($reportedWalls | Measure-Object -Minimum).Minimum) } else { [int64]::MaxValue }
        $maxReportedWall = if ($reportedWalls.Count -gt 0) { [int64](($reportedWalls | Measure-Object -Maximum).Maximum) } else { [int64]::MaxValue }
        $configSummaries.Add([pscustomobject]@{
            ConfigKey             = $configKey
            Family                = $first.Family
            BlockSize             = $first.BlockSize
            KBlock                = $first.KBlock
            Label                 = Config-Label $first
            BackendRequested      = $first.BackendRequested
            BackendUsed           = $first.BackendUsed
            Status                = $first.Status
            Environment           = $first.Environment
            ReportedWallNs        = $reportedWalls
            DurationNs            = $durations
            FailureCount          = $failedRows.Count
            SuccessCount          = $successfulRows.Count
            FailureSummaries      = @($failedRows | ForEach-Object { $_.FailureSummary })
            MedianReportedWallNs  = $medianReportedWall
            AverageReportedWallNs = $averageReportedWall
            MedianDurationNs      = $medianDuration
            MinReportedWallNs     = $minReportedWall
            MaxReportedWallNs     = $maxReportedWall
        })
    }

    $orderedConfigs = @($configSummaries | Sort-Object @{Expression = { if ($_.SuccessCount -gt 0) { 0 } else { 1 } }}, MedianReportedWallNs, MedianDurationNs)
    $fastestConfig = $orderedConfigs[0]
    $familyFastest = @($orderedConfigs | Where-Object { $_.SuccessCount -gt 0 } | Group-Object Family | ForEach-Object {
        $_.Group | Sort-Object MedianReportedWallNs, MedianDurationNs | Select-Object -First 1
    } | Sort-Object MedianReportedWallNs, MedianDurationNs)
    $fastestFamily = $familyFastest[0]

    $chosenLabel = if ($decision.Family -eq "SingleCall") {
        "SingleCall"
    } elseif ($decision.Family -eq "Blocked") {
        "Blocked(blockSize=$($decision.BlockSize))"
    } else {
        "KDecomposition(kBlock=$($decision.KBlock))"
    }
    $stabilizedLabel = if ($decision.StabilizedFamily -eq "SingleCall") {
        "SingleCall"
    } elseif ($decision.StabilizedFamily -eq "Blocked") {
        "Blocked(blockSize=$($decision.StabilizedBlockSize))"
    } else {
        "KDecomposition(kBlock=$($decision.StabilizedKBlock))"
    }

    $shapeSummaries.Add([pscustomobject]@{
        ShapeKey                = $shape.ShapeKey
        ShapeLabel              = $shape.ShapeLabel
        Region                  = $shape.Region
        M                       = $shape.M
        N                       = $shape.N
        K                       = $shape.K
        ChosenStrategy          = $decision.Strategy
        ChosenFamily            = $decision.Family
        ChosenBlockSize         = $decision.BlockSize
        ChosenKBlock            = $decision.KBlock
        ChosenLabel             = $chosenLabel
        StabilizedStrategy      = $decision.StabilizedStrategy
        StabilizedFamily        = $decision.StabilizedFamily
        StabilizedBlockSize     = $decision.StabilizedBlockSize
        StabilizedKBlock        = $decision.StabilizedKBlock
        StabilizedLabel         = $stabilizedLabel
        FastestFamily           = $fastestFamily.Family
        FastestFamilyLabel      = $fastestFamily.Label
        FastestConfigLabel      = $fastestConfig.Label
        RankingStability        = Family-Winner-Stability $winnersByRun
        ConfigResults           = $orderedConfigs
        PolicyChangedSelection  = ($chosenLabel -ne $stabilizedLabel)
    })
}

$summaryObject = [pscustomobject]@{
    GeneratedAt         = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
    RepoRoot            = $repo
    OctPath             = $OctPath
    ExperimentPath      = $expPath
    ReactorPath         = $ReactorPath
    CCPath              = $CCPath
    CXXPath             = $CXXPath
    WarmupRuns          = $WarmupRuns
    MeasuredRuns        = $MeasuredRuns
    DecisionSelections  = $decisionSelections.Values
    Shapes              = $shapeSummaries
    MeasuredRunsRaw     = $measured
}
[System.IO.File]::WriteAllText((Join-Path $OutDir "summary.json"), ($summaryObject | ConvertTo-Json -Depth 8))

$blockedWins = @($shapeSummaries | Where-Object { $_.FastestFamily -eq "Blocked" })
$singleWins = @($shapeSummaries | Where-Object { $_.FastestFamily -eq "SingleCall" })
$kWins = @($shapeSummaries | Where-Object { $_.FastestFamily -eq "KDecomposition" })
$policyChanges = @($shapeSummaries | Where-Object { $_.PolicyChangedSelection })
$allBlockedConfigs = @($shapeSummaries | ForEach-Object { $_.ConfigResults } | Where-Object { $_.Family -eq "Blocked" })
$allKConfigs = @($shapeSummaries | ForEach-Object { $_.ConfigResults } | Where-Object { $_.Family -eq "KDecomposition" })
$meaningfulBlocked = @($allBlockedConfigs | Sort-Object MedianReportedWallNs | Select-Object -First 10 | ForEach-Object { $_.BlockSize } | Sort-Object -Unique)
$meaningfulK = @($allKConfigs | Sort-Object MedianReportedWallNs | Select-Object -First 10 | ForEach-Object { $_.KBlock } | Sort-Object -Unique)

$lines = New-Object System.Collections.Generic.List[string]
$lines.Add('# M6a Windows-Native Prometheus Summary')
$lines.Add("")
$lines.Add('Environment:')
$lines.Add('- Windows')
$lines.Add('- `OCT_PROMETHEUS_REACTOR` pointed at the real Prometheus reactor DLL')
$lines.Add('- `CGO_ENABLED=1`')
$lines.Add(('- `CC={0}`' -f $CCPath))
$lines.Add(('- `CXX={0}`' -f $CXXPath))
$lines.Add(('- Fresh CLI built at `{0}`' -f $OctPath))
$lines.Add("")
$lines.Add('Probe regions:')
$lines.Add('- small-square')
$lines.Add('- balanced-medium')
$lines.Add('- rectangular-k-heavy')
$lines.Add("")

foreach ($shape in $shapeSummaries) {
    $lines.Add(("## {0} ({1})" -f $shape.ShapeLabel, $shape.Region))
    $lines.Add("")
    $lines.Add(('- chooser: `{0}`' -f $shape.ChosenLabel))
    $lines.Add(('- stabilized chooser (`hysteresis=12`, `min_commit=2`): `{0}`' -f $shape.StabilizedLabel))
    $lines.Add(('- fastest family: `{0}`' -f $shape.FastestFamily))
    $lines.Add(('- fastest coarse parameter choice: `{0}`' -f $shape.FastestConfigLabel))
    $lines.Add(('- ranking stability: {0}' -f $shape.RankingStability))
    if ($shape.ChosenFamily -eq $shape.FastestFamily) {
        $lines.Add('- chooser directionality: family choice is directionally correct on this shape')
    } else {
        $lines.Add(('- chooser directionality: family choice is directionally wrong on this shape (`{0}` vs `{1}`)' -f $shape.ChosenFamily, $shape.FastestFamily))
    }
    if ($shape.ChosenLabel -eq $shape.FastestConfigLabel) {
        $lines.Add('- coarse-parameter directionality: chosen coarse parameter matches the fastest observed config')
    } else {
        $lines.Add(('- coarse-parameter directionality: chosen config differs from fastest observed config (`{0}` vs `{1}`)' -f $shape.ChosenLabel, $shape.FastestConfigLabel))
    }
    $winnerSpread = $shape.ConfigResults[0].MaxReportedWallNs - $shape.ConfigResults[0].MinReportedWallNs
    if ($winnerSpread -gt ($shape.ConfigResults[0].MedianReportedWallNs / 2)) {
        $lines.Add('- variance note: winner spread is large enough that sub-family ordering should be treated cautiously')
    } else {
        $lines.Add('- variance note: winner spread stays reasonably tight across the three measured runs')
    }
    $lines.Add("")
    $lines.Add('| Config | Median wall | Runs (ns) | Median outer |')
    $lines.Add('| --- | --- | --- | --- |')
    foreach ($config in $shape.ConfigResults) {
        $runSeries = ($config.ReportedWallNs | ForEach-Object { $_.ToString() }) -join ", "
        $lines.Add(("| {0} | {1} | `{2}` | {3} |" -f $config.Label, (Format-Ns $config.MedianReportedWallNs), $runSeries, (Format-Ns $config.MedianDurationNs)))
    }
    $lines.Add("")
}

$lines.Add('## Overall Conclusions')
$lines.Add("")
if ($blockedWins.Count -gt 0) {
    $lines.Add(('- `Blocked` survives this Windows-native probe. It wins on: {0}' -f (($blockedWins | ForEach-Object { $_.ShapeLabel }) -join ", ")))
} else {
    $lines.Add('- `Blocked` does not win any measured probe shape and should be pruned unless later evidence reopens it.')
}
$lines.Add(('- `SingleCall` wins on: {0}' -f (($singleWins | ForEach-Object { $_.ShapeLabel }) -join ", ")))
$lines.Add(('- `KDecomposition` wins on: {0}' -f (($kWins | ForEach-Object { $_.ShapeLabel }) -join ", ")))
if ($meaningfulBlocked.Count -gt 0) {
    $lines.Add(('- meaningful blocked `blockSize` regimes in this run: {0}' -f (($meaningfulBlocked | Sort-Object) -join ", ")))
}
if ($meaningfulK.Count -gt 0) {
    $lines.Add(('- meaningful K-decomposition `kBlock` regimes in this run: {0}' -f (($meaningfulK | Sort-Object) -join ", ")))
}
if ($policyChanges.Count -eq 0) {
    $lines.Add('- optional `hysteresis` / `min_commit` do not change any measured one-shot M6 decision in this probe surface, so they are not justified yet for this chooser.')
} else {
    $lines.Add(('- optional `hysteresis` / `min_commit` changed {0} shape selections and need follow-up before controller formalization.' -f $policyChanges.Count))
}
$lines.Add('- M6 remains hosted inside the M4 experiment directory in this checkout; the runner and summary treat that as an explicit repository seam rather than silently inventing a new experiment root.')
[System.IO.File]::WriteAllText((Join-Path $OutDir "summary.md"), ($lines -join "`r`n") + "`r`n")

Write-Host "M6a artifacts written to:"
Write-Host ("  {0}" -f $artifactCopy)
Write-Host ("  {0}" -f (Join-Path $OutDir "summary.json"))
Write-Host ("  {0}" -f (Join-Path $OutDir "summary.md"))
