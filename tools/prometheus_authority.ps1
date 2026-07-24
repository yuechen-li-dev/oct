param(
    [switch]$SkipPackageRebuild
)

$ErrorActionPreference = "Stop"
$repo = (git rev-parse --show-toplevel).Trim()
Set-Location $repo

$results = [System.Collections.Generic.List[object]]::new()
$stage0Report = $null
$durableOutEvidence = @(
    "out/prometheus/native/p6c/summary.json",
    "out/prometheus/native/p6c/summary.md",
    "out/prometheus/sgemm_lab_m4d/summary.json",
    "out/prometheus/sgemm_lab_m4d/summary.md",
    "out/prometheus_fft_algorithm_lab/m1/m1_fft_cases.octagon",
    "out/prometheus_fft_algorithm_lab/m1/m1_fft_plan_traces.octagon",
    "out/prometheus_fft_algorithm_lab/m1/m1_fft_report.md",
    "out/prometheus_fft_algorithm_lab/m1/m1_fft_results.octagon",
    "out/test-artifacts/P13_M5_DVT2_Rtx3070ValidationArtifact/p13_dvt2_rtx3070_validation.txt"
)

function Invoke-AuthorityCommand([string]$Name, [string]$Program, [string[]]$Arguments) {
    $stderrPath = Join-Path ([IO.Path]::GetTempPath()) ("prometheus-authority-" + [guid]::NewGuid().ToString("N") + ".err")
    try {
        $output = & $Program @Arguments 2> $stderrPath
        $exitCode = $LASTEXITCODE
        $item = [ordered]@{
            name = $Name
            status = if ($exitCode -eq 0) { "PASS" } else { "FAIL" }
            exit_code = $exitCode
        }
        if ($exitCode -ne 0) {
            $item.error = ((Get-Content $stderrPath -ErrorAction SilentlyContinue) | Select-Object -Last 8) -join " | "
            if (-not $item.error) { $item.error = ($output | Select-Object -Last 8) -join " | " }
        }
        $results.Add([pscustomobject]$item)
        return [pscustomobject]@{ Output = $output; ExitCode = $exitCode }
    }
    finally {
        if (Test-Path -LiteralPath $stderrPath) { Remove-Item -LiteralPath $stderrPath -Force }
    }
}

function Test-TrackedRepositoryState {
    $tracked = @(git ls-files)
    $debris = @($tracked | Where-Object {
        ($_.StartsWith("internal/prometheus/.octmake/")) -or
        ($_.StartsWith("out/") -and $_ -notin $durableOutEvidence) -or
        ($_ -match "(^|/)__pycache__/") -or
        ([IO.Path]::GetExtension($_).ToLowerInvariant() -eq ".pyc")
    })
    $payload = @($tracked | Where-Object {
        $_ -match "(?i)(^|/)(checkpoints?|weights?|payloads?)/|\.(safetensors|gguf|ckpt|pth|onnx)$"
    })
    $item = [ordered]@{
        name = "tracked-repository-state"
        status = if ($debris.Count -eq 0 -and $payload.Count -eq 0) { "PASS" } else { "FAIL" }
        tracked_debris = $debris
        tracked_payload_or_model_data = $payload
    }
    $results.Add([pscustomobject]$item)
    return $item.status -eq "PASS"
}

function Test-IndexedPaths {
    $indexPath = "internal/prometheus/DevelopmentReport/PROMETHEUS_DEVELOPMENT_EVIDENCE_INDEX.json"
    if (-not (Test-Path -LiteralPath $indexPath)) {
        $results.Add([pscustomobject][ordered]@{ name = "evidence-index-paths"; status = "FAIL"; error = "missing $indexPath" })
        return $false
    }
    $index = Get-Content -Raw $indexPath | ConvertFrom-Json
    $missing = [System.Collections.Generic.List[string]]::new()
    foreach ($section in @("current_authorities", "current_reviewer_handoffs", "current_validation_characterization", "historical_entries", "generated_or_measured_artifacts", "unknown_entries")) {
        foreach ($entry in @($index.$section)) {
            if ($entry.path -and -not (Test-Path -LiteralPath $entry.path)) { $missing.Add($entry.path) }
        }
    }
    $item = [ordered]@{
        name = "evidence-index-paths"
        status = if ($missing.Count -eq 0) { "PASS" } else { "FAIL" }
        indexed_file_count = $index.corpus.tracked_file_count_at_stage0_checkpoint
        missing_paths = @($missing)
    }
    $results.Add([pscustomobject]$item)
    return $item.status -eq "PASS"
}

try {
    $native = Invoke-AuthorityCommand "native-manifest-and-generated-inventory" "go" @("run", "./tools/prometheus_native_manifest", "-check")
    $lock = Invoke-AuthorityCommand "compiled-model-lock-projections" "go" @("run", "./tools/compiled_model_lock", "-check")
    $repositoryStateOk = Test-TrackedRepositoryState
    $indexOk = Test-IndexedPaths

    $packageDir = Join-Path ([IO.Path]::GetTempPath()) ("prometheus-authority-" + [guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $packageDir -Force | Out-Null
    try {
        if ($SkipPackageRebuild -and (Test-Path -LiteralPath "out/prometheus/native/SerialCanonical/shaders/manifest.json")) {
            $stage0 = Invoke-AuthorityCommand "stage0-package-authority" "go" @("run", "./tools/prometheus_stage0", "-check")
        }
        else {
            $packageOut = Join-Path $packageDir "shaders"
            $idsOut = Join-Path $packageDir "reactor_shader_ids.generated.h"
            $build = Invoke-AuthorityCommand "temporary-clean-clone-package-build" "go" @("run", "./cmd/oct", "sdslv", "package", "build", "--manifest", "internal/prometheus/native/shaders/manifest.json", "--repo", ".", "--out", $packageOut, "--ids", $idsOut)
            if ($build.ExitCode -eq 0) {
                $stage0 = Invoke-AuthorityCommand "stage0-package-authority" "go" @("run", "./tools/prometheus_stage0", "-check", "-package-dir", $packageOut)
            }
        }
        if ($stage0.Output) {
            try { $stage0Report = ($stage0.Output -join "`n") | ConvertFrom-Json } catch { }
        }
    }
    finally {
        $resolvedTemp = [IO.Path]::GetFullPath($packageDir)
        $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
        if ($resolvedTemp.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolvedTemp)) {
            Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
        }
    }
}
catch {
    $results.Add([pscustomobject][ordered]@{ name = "authority-coordinator"; status = "FAIL"; error = $_.Exception.Message })
}

$failed = @($results | Where-Object { $_.status -eq "FAIL" }).Count
[ordered]@{
    schema = "prometheus.repository-authority.v1"
    status = if ($failed -eq 0) { "PASS" } else { "FAIL" }
    checks = @($results)
    stage0 = $stage0Report
} | ConvertTo-Json -Depth 12

if ($failed -ne 0) { exit 1 }
