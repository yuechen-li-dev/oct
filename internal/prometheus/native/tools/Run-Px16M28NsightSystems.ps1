param([switch]$Build)
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..\..')).Path
$nsys = Get-Command nsys -ErrorAction SilentlyContinue
if ($null -eq $nsys) {
    $candidate = 'C:\Program Files\NVIDIA Corporation\Nsight Systems 2025.6.3\target-windows-x64\nsys.exe'
    if (Test-Path $candidate) { $nsys = Get-Item $candidate }
}
$nsysPath = if ($null -ne $nsys.PSPath) { $nsys.FullName } else { $nsys.Source }
if ([string]::IsNullOrEmpty($nsysPath)) { throw 'nsys was not found on PATH or in the standard Nsight Systems installation directory.' }
$out = Join-Path $root 'out\profiling\px16_m28'
New-Item -ItemType Directory -Force -Path $out | Out-Null
if ($Build) { & (Join-Path $PSScriptRoot 'Run-Px16M28FeedPath.ps1') -Build; if ($LASTEXITCODE -ne 0) { throw 'build failed' } }
& $nsysPath --version | Tee-Object -FilePath (Join-Path $out 'nsys-version.txt')
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$base = Join-Path $out ("nsys_m28_resident_$stamp")
& $nsysPath profile --trace=vulkan,wddm --sample=none --force-overwrite=false --output=$base -- `
    (Join-Path $root 'out\prometheus\native\marionette_tests.exe') PrometheusSgemmPx16M28FeedPath
if ($LASTEXITCODE -ne 0) { throw 'Nsight Systems capture failed' }
Write-Host "Capture written under $out. Inspect Vulkan queue submissions, fence waits, and GPU idle gaps in Nsight Systems."
