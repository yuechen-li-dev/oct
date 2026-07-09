param()

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..\..")

Push-Location $repoRoot
try {
    $shader = "internal/prometheus/shaders/sdslv/sgemm_scalar_baseline_plus.sdslv"
    $outDir = "out/sdslv"
    $header = "internal/prometheus/native/reactor_vulkan_sgemm_scalar_plus_spirv.h"
    $symbol = "k_prom_sgemm_scalar_plus_spirv"
    $nativeTempHlsl = "internal/prometheus/native/reactor_vulkan_sgemm_scalar_plus_spirv.hlsl"
    $nativeTempSpv = "internal/prometheus/native/reactor_vulkan_sgemm_scalar_plus_spirv.spv"

    New-Item -ItemType Directory -Force -Path $outDir | Out-Null

    go run ./cmd/oct sdslv check $shader
    go run ./cmd/oct sdslv emit-hlsl $shader -o (Join-Path $outDir "sgemm_scalar_baseline_plus.hlsl")
    go run ./cmd/oct sdslv compile-spv $shader -o (Join-Path $outDir "sgemm_scalar_baseline_plus.spv")
    go run ./cmd/oct sdslv generate-header $shader -o $header --symbol $symbol

    Remove-Item -LiteralPath $nativeTempHlsl -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $nativeTempSpv -ErrorAction SilentlyContinue
}
finally {
    Pop-Location
}
