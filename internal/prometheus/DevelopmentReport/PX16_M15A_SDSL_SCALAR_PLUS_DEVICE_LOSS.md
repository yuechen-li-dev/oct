# Px16 M15a SDSL Scalar-Plus Device Loss

## Outcome

Success.

The isolated `lowk_1024x1024x64 : SDSL_SCALAR_PLUS` `VK_ERROR_DEVICE_LOST` was caused by a shader/resource ABI mismatch, not by SGEMM indexing, dispatch geometry, buffer sizing, or push-constant layout.

The narrow fix was:

- change SDSL-V HLSL resource emission for `array<T>` resources from `Buffer<T>` / `RWBuffer<T>` to `StructuredBuffer<T>` / `RWStructuredBuffer<T>`;
- regenerate the checked-in Prometheus SDSL shader headers.

After that fix:

- the focused low-K resident-explicit repro passes;
- `lowk_1024x1024x64 : SDSL_SCALAR_PLUS` no longer device-loses;
- the earlier broad scalar-plus correctness failures in the validation lane also disappear.

## Root Cause

### What failed

The runtime descriptor ABI for Prometheus SGEMM is storage-buffer based:

- binding `0`: A
- binding `1`: B
- binding `2`: C
- descriptor type: `VK_DESCRIPTOR_TYPE_STORAGE_BUFFER`
- push constants: `m`, `n`, `k` as three `uint32_t` values at offsets `0`, `4`, `8`

That contract is visible in:

- `internal/prometheus/native/reactor_vulkan_sgemm.c`
  - descriptor set layout bindings `0/1/2`
  - push struct `prom_vk_push`
  - resident re-dispatch descriptor writes

### What the old scalar-plus shader actually emitted

Before the fix, SDSL HLSL emission used:

```hlsl
[[vk::binding(0, 0)]] Buffer<float> A;
[[vk::binding(1, 0)]] Buffer<float> B;
[[vk::binding(2, 0)]] RWBuffer<float> C;
```

DXC compiled that to SPIR-V texel-buffer/image-buffer resources, not storage-buffer blocks. The pre-fix disassembly showed:

- `OpCapability SampledBuffer`
- `OpCapability ImageBuffer`
- `OpTypeImage %float Buffer ...`
- `OpImageFetch`
- `OpImageWrite`

That does not match the runtime’s `VK_DESCRIPTOR_TYPE_STORAGE_BUFFER` binding model.

### Why that explains the symptoms

- It explains the old scalar-plus correctness failures: the shader was reading/writing through the wrong Vulkan resource class.
- It explains why the low-K row could escalate into `VK_ERROR_DEVICE_LOST` at submit on the Windows RTX 3070 path: the driver/runtime contract was invalid for that pipeline layout/resource interpretation.
- It explains why controls such as `BASELINE_SCALAR` did not fail on the same shape: the legacy baseline path already used the expected storage-buffer ABI.

## HLSL / SPIR-V Evidence

### Fixed emitted HLSL

After the fix, the emitted HLSL is:

```hlsl
[[vk::binding(0, 0)]] StructuredBuffer<float> A;
[[vk::binding(1, 0)]] StructuredBuffer<float> B;
[[vk::binding(2, 0)]] RWStructuredBuffer<float> C;
[[vk::push_constant]] ConstantBuffer<SgemmParams> params;
```

### Fixed SPIR-V shape

After regeneration, the scalar-plus SPIR-V disassembly shows storage-buffer layout instead:

- `OpTypeRuntimeArray %float`
- `OpTypeStruct %_runtimearr_float`
- `OpDecorate ... BufferBlock`
- `OpDecorate %_runtimearr_float ArrayStride 4`
- `OpAccessChain` loads/stores from `Uniform` storage-buffer blocks

and no longer uses:

- `OpCapability SampledBuffer`
- `OpCapability ImageBuffer`
- `OpTypeImage ... Buffer`
- `OpImageFetch`
- `OpImageWrite`

`spirv-val --target-env vulkan1.0 out/sdslv/sgemm_scalar_baseline_plus.spv` passes after the fix.

## Push Constant / Descriptor ABI Check

The push-constant ABI already matched and was not the bug.

Host contract in `reactor_vulkan_sgemm.c`:

- `sizeof(prom_vk_push) == 12`
- offsets:
  - `m = 0`
  - `n = 4`
  - `k = 8`

Shader contract in generated SPIR-V:

- push constant block offsets:
  - `m = 0`
  - `n = 4`
  - `k = 8`

Descriptor order also already matched:

- shader bindings: `A=0`, `B=1`, `C=2`
- runtime bindings: `A=0`, `B=1`, `C=2`

The issue was resource class, not binding order.

## Dispatch / Metadata Proof

For `SDSL_SCALAR_PLUS`, generated metadata is:

- `numthreads_x = 8`
- `numthreads_y = 8`
- `numthreads_z = 1`
- `outputs_per_invocation_m = 1`
- `outputs_per_invocation_n = 1`
- `tile_m = 1`
- `tile_n = 1`
- `unroll_k = 4`

So:

- `logical_m_per_group = 8 * 1 = 8`
- `logical_n_per_group = 8 * 1 = 8`

For the failing shape `M=1024`, `N=1024`, `K=64`:

- `groups_x = ceil(1024 / 8) = 128`
- `groups_y = ceil(1024 / 8) = 128`
- `groups_z = 1`

Maximum launched coordinates:

- max row candidate = `128 * 8 - 1 = 1023`
- max col candidate = `128 * 8 - 1 = 1023`

So dispatch coverage is exact for `1024x1024` and not an over-dispatch bug.

Focused repro artifacts also record that same geometry:

- `out/test-artifacts/prometheus_sgemm_px16_m15a_sdsl_scalar_plus_lowk_repro.json`
- `out/test-artifacts/prometheus_sgemm_px16_m15a_sdsl_scalar_plus_lowk_repro.md`

## Index / Bounds Proof

The scalar-plus source indexing is:

- `row = DispatchId.x`
- `col = DispatchId.y`
- guard `if row >= params.m return`
- guard `if col >= params.n return`
- `aRowBase = row * params.k`
- `for kk in 0..params.k step 4`
- `kIndex = kk + lane`
- guard `if kIndex < params.k`
- `A[aRowBase + kIndex]`
- `B[kIndex * params.n + col]`
- `C[row * params.n + col]`

For `M=1024`, `N=1024`, `K=64`:

- A:
  - `row < 1024`
  - `kIndex < 64`
  - max index = `(1023 * 64) + 63 = 65535 = M*K - 1`
- B:
  - `kIndex < 64`
  - `col < 1024`
  - max index = `(63 * 1024) + 1023 = 65535 = K*N - 1`
- C:
  - `row < 1024`
  - `col < 1024`
  - max index = `(1023 * 1024) + 1023 = 1048575 = M*N - 1`

Known buffer sizes:

- A elements = `1024 * 64 = 65536`
- B elements = `64 * 1024 = 65536`
- C elements = `1024 * 1024 = 1048576`

So the shader math itself is bounds-safe for the reported low-K shape.

## Focused Repro Commands

Commands run on July 9, 2026:

```bat
go test ./internal/sdslv/... ./cmd/oct
go test ./internal/prometheus/... ./cmd/oct
go test ./internal/... ./cmd/oct

go run ./cmd/oct sdslv check internal/prometheus/shaders/sdslv/sgemm_scalar_baseline_plus.sdslv
go run ./cmd/oct sdslv emit-hlsl internal/prometheus/shaders/sdslv/sgemm_scalar_baseline_plus.sdslv -o out/sdslv/sgemm_scalar_baseline_plus.hlsl
go run ./cmd/oct sdslv compile-spv internal/prometheus/shaders/sdslv/sgemm_scalar_baseline_plus.sdslv -o out/sdslv/sgemm_scalar_baseline_plus.spv

powershell -ExecutionPolicy Bypass -File internal/prometheus/native/generate_sdslv_shaders.ps1

cmd /c "call ""C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat"" -arch=x64 -host_arch=x64 >nul && internal\prometheus\native\build_windows.cmd"

out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16M15aSdslScalarPlusLowKRepro
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16ResidentExplicitFailureMatrix
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16Evt_CorrectnessValidationLane
out\prometheus\native\marionette_benchmarks.exe PrometheusSgemmPx16Evt
```

## Repro Result

The focused M15a low-K matrix now passes for:

- `128x128x64`
- `256x256x64`
- `512x512x64`
- `1024x1024x16`
- `1024x1024x32`
- `1024x1024x64`
- `1024x1024x65`

with controls:

- `BASELINE_SCALAR`
- `MEMORY_CONSERVATIVE`
- `SDSL_SCALAR_PLUS`

For the formerly failing row:

- shape: `lowk_1024x1024x64`
- variant: `SDSL_SCALAR_PLUS`
- staged result: passed
- resident result: passed
- failure stage: `none`
- `VkResult`: `none`
- dispatch groups: `(128, 128, 1)`
- descriptor bindings: `A=0, B=1, C=2`
- push constants: `12` bytes, offsets `0/4/8`

## Files Changed

- `internal/sdslv/emit/hlsl/hlsl.go`
- `internal/sdslv/emit/hlsl/hlsl_test.go`
- `cmd/oct/sdslv_command_test.go`
- `internal/prometheus/native/reactor_vulkan_sgemm_scalar_plus_spirv.h`
- `internal/prometheus/native/reactor_vulkan_sgemm_tile16x16_shared_fp32_spirv.h`
- `internal/prometheus/native/Marionette/reactor_px16_evt_benchmark_tests.cpp`

## Important Inconsistency Surfaced

The existing Px16 M13 documentation said scalar-plus preserved the legacy Prometheus SGEMM ABI. At the descriptor binding/order level that was true, but at the emitted Vulkan resource-class level it was not true before this fix.

That inconsistency is now explicit:

- intended ABI: storage buffers
- pre-fix emitted ABI: texel/image buffers
- fixed emitted ABI: storage buffers

## Next Step Recommendation

No broader SGEMM or selector change is recommended from this milestone.

The next safe follow-up is narrower:

- keep SDSL resource ABI under regression coverage so future shaders cannot silently fall back to texel-buffer emission again;
- if desired, add one explicit SPIR-V-shape regression that fails if array resources compile back to `OpTypeImage ... Buffer`.
