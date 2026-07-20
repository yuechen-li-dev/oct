param()

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..\..")

Push-Location $repoRoot
try {
    $outDir = "out/sdslv"
    # The registry manifest is the declarative source of SDSL-V asset wiring.
    # Stable shader and implementation IDs remain reviewed manifest facts.
    $manifest = Get-Content -Raw "internal/prometheus/native/shaders/manifest.json" | ConvertFrom-Json
    $jobs = @($manifest.shader_assets | Where-Object { $_.source_language -eq "sdslv" } | Sort-Object id | ForEach-Object {
        $stem = [IO.Path]::GetFileNameWithoutExtension($_.source)
        $header = "internal/prometheus/native/" + $_.header
        $nativeBase = $header -replace '\.h$', ''
        @{ Shader = $_.source; Stem = $stem; Header = $header; Symbol = $_.symbol;
           NativeTempHlsl = $nativeBase + ".hlsl"; NativeTempSpv = $nativeBase + ".spv" }
    })

    New-Item -ItemType Directory -Force -Path $outDir | Out-Null

    foreach ($job in $jobs) {
        go run ./cmd/oct sdslv check $job.Shader
        go run ./cmd/oct sdslv emit-hlsl $job.Shader -o (Join-Path $outDir ($job.Stem + ".hlsl"))
        go run ./cmd/oct sdslv compile-spv $job.Shader -o (Join-Path $outDir ($job.Stem + ".spv")) --validate --require-spirv-val
        go run ./cmd/oct sdslv generate-header $job.Shader -o $job.Header --symbol $job.Symbol --validate --require-spirv-val

        Remove-Item -LiteralPath $job.NativeTempHlsl -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $job.NativeTempSpv -ErrorAction SilentlyContinue
    }
}
finally {
    Pop-Location
}
