param(
    [Parameter(Mandatory = $true)][string] $LlamaSource,
    [Parameter(Mandatory = $true)][string] $BuildDirectory
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$commit = '86a9c79f866799eb0e7e89c03578ccfbcc5d808e'
$adapterPatch = Join-Path $PSScriptRoot 'patches\llama_cpp_86a9c79f_sdslv_rmsnorm.patch'
$perfPatch = Join-Path $PSScriptRoot 'patches\llama_cpp_86a9c79f_rmsnorm_perf_cases.patch'
$shader = Join-Path $PSScriptRoot 'shaders\rms_norm_f32_ggml_abi.sdslv'
$generated = Join-Path $PSScriptRoot 'generated\rms_norm_f32_ggml_abi_spirv.h'
$targetHeader = Join-Path $LlamaSource 'ggml\src\ggml-vulkan\ggml-vulkan-sdslv-experiment.hpp'

if ((git -C $LlamaSource rev-parse HEAD).Trim() -ne $commit) { throw "Expected llama.cpp commit $commit." }

$generateArgs = @('run', (Join-Path $root 'cmd\oct'), 'sdslv', 'generate-header', $shader, '-o', $generated, '--symbol', 'k_sdslv_benchmark_rms_norm_f32_ggml_abi_spirv', '--hlsl-out', (Join-Path $PSScriptRoot 'generated\rms_norm_f32_ggml_abi.hlsl'), '--spv-out', (Join-Path $PSScriptRoot 'generated\rms_norm_f32_ggml_abi.spv'), '--dxc', 'C:\VulkanSDK\1.4.350.0\Bin\dxc.exe', '--validate', '--require-spirv-val')
& go @generateArgs
Copy-Item -LiteralPath $generated -Destination $targetHeader -Force
git -C $LlamaSource apply --check $adapterPatch
git -C $LlamaSource apply --check $perfPatch
git -C $LlamaSource apply $adapterPatch
git -C $LlamaSource apply $perfPatch

$configureArgs = @('-S', $LlamaSource, '-B', $BuildDirectory, '-G', 'Visual Studio 18 2026', '-A', 'x64', '-DGGML_VULKAN=ON', '-DGGML_VULKAN_SDSLV_EXPERIMENT=ON', '-DGGML_NATIVE=OFF', '-DGGML_BUILD_TESTS=OFF', '-DGGML_BUILD_EXAMPLES=OFF', '-DLLAMA_BUILD_TESTS=ON', '-DLLAMA_BUILD_TOOLS=ON', '-DLLAMA_BUILD_EXAMPLES=OFF', '-DLLAMA_BUILD_SERVER=OFF')
& cmake @configureArgs
& cmake --build $BuildDirectory --config Release --target llama-bench test-backend-ops --parallel 4
