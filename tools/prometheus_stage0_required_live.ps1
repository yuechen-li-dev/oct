param(
    [switch]$SelfTest
)

$ErrorActionPreference = "Stop"

function Assert-LiveLaneOutput {
    param(
        [string]$Name,
        [string[]]$Output
    )

    $text = $Output -join "`n"
    if ($text -match "--- SKIP: $Name") {
        throw "$Name skipped; required-live validation cannot treat a skip as green"
    }
    if ($text -notmatch "--- PASS: $Name") {
        throw "$Name did not report a completed PASS case"
    }
}

if ($SelfTest) {
    Assert-LiveLaneOutput -Name "SyntheticPass" -Output @("--- PASS: SyntheticPass (0.00s)")
    try {
        Assert-LiveLaneOutput -Name "SyntheticSkip" -Output @("--- SKIP: SyntheticSkip (0.00s)")
        throw "skip self-test did not reject"
    } catch {
        if ($_.Exception.Message -notmatch "skipped") { throw }
    }
    try {
        Assert-LiveLaneOutput -Name "SyntheticZeroWork" -Output @("PASS")
        throw "zero-work self-test did not reject"
    } catch {
        if ($_.Exception.Message -notmatch "completed PASS") { throw }
    }
    Write-Output "required-live skip-detection self-test: PASS"
    exit 0
}

$required = @("OCT_PROMETHEUS_REACTOR", "G4E2B_CHECKPOINT_ROOT")
foreach ($name in $required) {
    if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($name))) {
        throw "required-live prerequisite $name is unset"
    }
}
$env:OCT_RUN_PROMETHEUS_INTEGRATION = "1"
$env:PROMETHEUS_REQUIRE_VULKAN_HARDWARE = "1"
$env:PROMETHEUS_VK_VALIDATION = "1"

& go run ./tools/prometheus_stage0 -check
if ($LASTEXITCODE -ne 0) { throw "Stage 0 generated-authority check failed" }

$lanes = @(
    "TestGemma4E2BM1FreshSessionQFirstAuthority",
    "TestGemma4E2BM1FreshSessionKFirstAuthority",
    "TestGemma4E2BM1SameSession7406Characterization"
)
foreach ($lane in $lanes) {
    $output = & go test -run "^$lane$" -count=1 -v ./internal/prometheus 2>&1
    $output | Write-Output
    if ($LASTEXITCODE -ne 0) { throw "$lane failed" }
    Assert-LiveLaneOutput -Name $lane -Output $output
}

Write-Output "required-live Stage 0 Gemma lanes: PASS"
