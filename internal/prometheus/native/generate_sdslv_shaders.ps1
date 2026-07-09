param()

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..\..")

Push-Location $repoRoot
try {
    $outDir = "out/sdslv"
    $jobs = @(
        @{
            Shader = "internal/prometheus/shaders/sdslv/sgemm_scalar_baseline_plus.sdslv"
            Stem = "sgemm_scalar_baseline_plus"
            Header = "internal/prometheus/native/reactor_vulkan_sgemm_scalar_plus_spirv.h"
            Symbol = "k_prom_sgemm_scalar_plus_spirv"
            NativeTempHlsl = "internal/prometheus/native/reactor_vulkan_sgemm_scalar_plus_spirv.hlsl"
            NativeTempSpv = "internal/prometheus/native/reactor_vulkan_sgemm_scalar_plus_spirv.spv"
        },
        @{
            Shader = "internal/prometheus/shaders/sdslv/sgemm_tile16x16_shared_fp32.sdslv"
            Stem = "sgemm_tile16x16_shared_fp32"
            Header = "internal/prometheus/native/reactor_vulkan_sgemm_tile16x16_shared_fp32_spirv.h"
            Symbol = "k_prom_sgemm_tile16x16_shared_fp32_spirv"
            NativeTempHlsl = "internal/prometheus/native/reactor_vulkan_sgemm_tile16x16_shared_fp32_spirv.hlsl"
            NativeTempSpv = "internal/prometheus/native/reactor_vulkan_sgemm_tile16x16_shared_fp32_spirv.spv"
        },
        @{
            Shader = "internal/prometheus/shaders/sdslv/sgemm_reg2x2_tile16x16_fp32.sdslv"
            Stem = "sgemm_reg2x2_tile16x16_fp32"
            Header = "internal/prometheus/native/reactor_vulkan_sgemm_reg2x2_tile16x16_fp32_spirv.h"
            Symbol = "k_prom_sgemm_reg2x2_tile16x16_fp32_spirv"
            NativeTempHlsl = "internal/prometheus/native/reactor_vulkan_sgemm_reg2x2_tile16x16_fp32_spirv.hlsl"
            NativeTempSpv = "internal/prometheus/native/reactor_vulkan_sgemm_reg2x2_tile16x16_fp32_spirv.spv"
        }
    )

    New-Item -ItemType Directory -Force -Path $outDir | Out-Null

    foreach ($job in $jobs) {
        go run ./cmd/oct sdslv check $job.Shader
        go run ./cmd/oct sdslv emit-hlsl $job.Shader -o (Join-Path $outDir ($job.Stem + ".hlsl"))
        go run ./cmd/oct sdslv compile-spv $job.Shader -o (Join-Path $outDir ($job.Stem + ".spv"))
        go run ./cmd/oct sdslv generate-header $job.Shader -o $job.Header --symbol $job.Symbol

        Remove-Item -LiteralPath $job.NativeTempHlsl -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $job.NativeTempSpv -ErrorAction SilentlyContinue
    }
}
finally {
    Pop-Location
}
