param(
    [string]$ShaderPackageRoot = ""
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$nativeRoot = Join-Path $repoRoot 'internal\prometheus\native'
$outRoot = Join-Path $repoRoot 'out\prometheus\native\concept_vulkan_m1d'
$vulkanSdk = $env:VULKAN_SDK
if ([string]::IsNullOrWhiteSpace($vulkanSdk) -or -not (Test-Path (Join-Path $vulkanSdk 'Include\vulkan\vulkan.h'))) {
    throw 'VULKAN_SDK must name an installed Vulkan SDK with real headers.'
}

if (-not (Get-Command cl.exe -ErrorAction SilentlyContinue) -or [string]::IsNullOrWhiteSpace($env:INCLUDE)) {
    $vsDevCmd = 'C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat'
    if (-not (Test-Path $vsDevCmd)) { throw 'MSVC developer tools were not found.' }
    & cmd.exe /c "call `"$vsDevCmd`" -arch=x64 -host_arch=x64 >nul && set" | ForEach-Object {
        if ($_ -match '^(.*?)=(.*)$') { Set-Item -Path "env:$($matches[1])" -Value $matches[2] }
    }
}

if ([string]::IsNullOrWhiteSpace($ShaderPackageRoot)) {
    $ShaderPackageRoot = Join-Path $repoRoot 'out\prometheus\native\SerialCanonical\shaders'
}
if (-not (Test-Path (Join-Path $ShaderPackageRoot 'manifest.json'))) {
    throw "shader package manifest was not found: $ShaderPackageRoot"
}

$manifest = Get-Content -Raw (Join-Path $nativeRoot 'native_manifest.json') | ConvertFrom-Json
Remove-Item -Recurse -Force $outRoot -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $outRoot | Out-Null

$include = @("/I$($vulkanSdk)\Include", "/I$nativeRoot")
$common = @('/nologo', '/TC', '/std:c11', '/O2', '/W4', '/DPROM_CONCEPT_VULKAN_CONFORMANCE') + $include
$objects = @()
foreach ($relative in $manifest.production_sources) {
    $source = Join-Path $nativeRoot $relative
    $object = Join-Path $outRoot (([IO.Path]::GetFileNameWithoutExtension($relative)) + '.obj')
    & cl.exe @common "/Fo$object" '/c' $source
    if ($LASTEXITCODE -ne 0) { throw "conformance compile failed: $relative" }
    $objects += $object
}
foreach ($source in @(
    (Join-Path $nativeRoot 'reactor_concept_vulkan_kernel54.generated.c'),
    (Join-Path $nativeRoot 'Marionette\concept_vulkan_m1d_conformance.c')
)) {
    $object = Join-Path $outRoot (([IO.Path]::GetFileNameWithoutExtension($source)) + '.obj')
    & cl.exe @common "/Fo$object" '/c' $source
    if ($LASTEXITCODE -ne 0) { throw "conformance compile failed: $source" }
    $objects += $object
}

$exe = Join-Path $outRoot 'concept_vulkan_m1d_conformance.exe'
& link.exe /nologo $objects "/OUT:$exe" "/LIBPATH:$($vulkanSdk)\Lib" vulkan-1.lib
if ($LASTEXITCODE -ne 0) { throw 'conformance link failed' }

$rayObject = Join-Path $outRoot 'reactor_vulkan_ray_query.obj'
$symbolText = (& dumpbin.exe /symbols $rayObject) -join "`n"
if ($symbolText -notmatch 'prom_concept_vulkan_kernel54_handwritten_adapter' -or
    $symbolText -notmatch 'prom_concept_vulkan_kernel54_generated_adapter') {
    throw 'conformance adapter symbols are absent from the conformance object'
}
$publicHeaders = Get-ChildItem (Join-Path $nativeRoot 'include') -Filter '*.h' -File
if (($publicHeaders | Select-String -Pattern 'concept_vulkan_kernel54|PROM_CONCEPT_VULKAN_CONFORMANCE' -Quiet)) {
    throw 'conformance symbol leaked into a public header'
}

& $exe $ShaderPackageRoot
$runResult = $LASTEXITCODE
if ($runResult -eq 77) { Write-Host 'SKIP: admitted ray-query runtime unavailable'; exit 0 }
if ($runResult -ne 0) { throw "conformance execution failed: $runResult" }
Write-Host "PASS: $exe"
