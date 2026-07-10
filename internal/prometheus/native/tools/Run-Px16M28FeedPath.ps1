param(
    [switch]$Build
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..\..')).Path
$out = Join-Path $root 'out\profiling\px16_m28'
New-Item -ItemType Directory -Force -Path $out | Out-Null
if ($Build) {
    & cmd.exe /c ('call "C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat" -arch=x64 -host_arch=x64 >nul && internal\prometheus\native\build_windows.cmd')
    if ($LASTEXITCODE -ne 0) { throw 'native build failed' }
}
$env:VK_INSTANCE_LAYERS = ''
$env:VK_LAYER_PATH = ''
$envReport = @("timestamp=$(Get-Date -Format o)", "VK_INSTANCE_LAYERS=$env:VK_INSTANCE_LAYERS", "VK_LAYER_PATH=$env:VK_LAYER_PATH")
$envReport | Set-Content -Encoding utf8 (Join-Path $out 'environment.txt')
& (Join-Path $root 'out\prometheus\native\marionette_tests.exe') PrometheusSgemmPx16M28FeedPath 2>&1 | Tee-Object -FilePath (Join-Path $out 'feed_path_test.log')
if ($LASTEXITCODE -ne 0) { throw 'M28 feed-path test failed' }
Write-Host "Artifacts: $root\out\test-artifacts\prometheus_sgemm_px16_m28_feed_path.{json,md}"
