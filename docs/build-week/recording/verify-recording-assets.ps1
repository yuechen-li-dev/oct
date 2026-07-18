$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
Set-Location $repoRoot

$required = @(
    "README.md",
    "internal/prometheus/shaders/sdslv/production/reduction/softmax_fused.sdslv",
    "internal/prometheus/DevelopmentReport/PROMETHEUS_M39B_FUSED_REDUCTION_REACTOR.md",
    "Examples/SDSL-V/conformance/graphics/CanonicalGraphicsProgram.sdslvvalid",
    "Examples/SDSL-V/conformance/artifacts/ForwardTextured.vertex.hlsl",
    "Examples/SDSL-V/conformance/artifacts/ForwardTextured.bundle.json",
    "internal/prometheus/DevelopmentReport/artifacts/M47/gated_ffn_complete_transformer_block_rtx3070.json",
    "internal/prometheus/DevelopmentReport/artifacts/M48/multi_block_golden_path_evt_closeout.json",
    "docs/development/OCT_MCP_AGENT_DOGFOODING.md"
)

foreach ($path in $required) {
    if (-not (Test-Path -LiteralPath $path)) {
        throw "Missing recording asset: $path"
    }
    Write-Host "FOUND  $path" -ForegroundColor Green
}

& (Join-Path $PSScriptRoot "run-judge-demo.ps1") -Lane test
& (Join-Path $PSScriptRoot "run-judge-demo.ps1") -Lane artifact

$artifactPath = Join-Path $repoRoot "out/build-week/judge-demo/summary.json"
$artifact = Get-Item -LiteralPath $artifactPath
$hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $artifactPath).Hash.ToLowerInvariant()
if ($artifact.Length -ne 62) {
    throw "Unexpected artifact size: $($artifact.Length)"
}
if ($hash -ne "4efc9d55ed0eac0e8401f92f1d6b320e7e4c4b7e09778f1c5b7f9917c484209c") {
    throw "Unexpected artifact hash: $hash"
}
Write-Host "RECORDING PREFLIGHT PASSED - artifact is 62 bytes with expected SHA-256" -ForegroundColor Cyan
