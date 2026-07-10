param([switch]$RequireHardware)
$ErrorActionPreference = 'Stop'
$root = Resolve-Path (Join-Path $PSScriptRoot '..\..\..')
$artifact = Join-Path $root 'out\test-artifacts\prometheus_vulkan_preflight_windows.json'
New-Item -ItemType Directory -Force (Split-Path $artifact) | Out-Null
$loader = Join-Path $env:WINDIR 'System32\vulkan-1.dll'
$vulkanInfo = (Get-Command vulkaninfo.exe -ErrorAction Stop).Source
$savedErrorActionPreference = $ErrorActionPreference
$ErrorActionPreference = 'Continue'
$summary = & $vulkanInfo --summary 2>&1
$ErrorActionPreference = $savedErrorActionPreference
$exitCode = $LASTEXITCODE
$nvidiaJson = Get-ChildItem "$env:WINDIR\System32\DriverStore\FileRepository" -Filter nv-vk64.json -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty FullName
$record = [ordered]@{ architecture=$env:PROCESSOR_ARCHITECTURE; cwd=(Get-Location).Path; vulkan_sdk=$env:VULKAN_SDK; vk_icd_filenames=$env:VK_ICD_FILENAMES; vk_layer_path=$env:VK_LAYER_PATH; loader_path=$loader; loader_exists=(Test-Path $loader); vulkaninfo_path=$vulkanInfo; vulkaninfo_exit=$exitCode; nvidia_icd_json=$nvidiaJson; nvidia_rtx_3070_visible=($summary -match 'NVIDIA GeForce RTX 3070'); summary=($summary | Out-String) }
$record | ConvertTo-Json -Depth 4 | Set-Content -Encoding utf8 $artifact
$record.summary | Set-Content -Encoding utf8 (Join-Path $root 'out\test-artifacts\prometheus_vulkan_preflight_windows.md')
if ($RequireHardware -and (!$record.loader_exists -or $exitCode -ne 0 -or !$record.nvidia_rtx_3070_visible)) { throw 'Vulkan hardware preflight failed; inspect prometheus_vulkan_preflight_windows.json.' }
