# Prometheus FR-M0: managed row-wise FP32 softmax boundary

## Status

**SUCCESS.** The production package-backed route executed on the admitted Windows NVIDIA GeForce RTX 3070 with Vulkan validation enabled. Its output was compared elementwise against the independent double-precision CPU authority, including the Z-Image width 1,056. No CPU fallback or historical attention result is used as correctness authority.

The earlier Windows "Vulkan unavailable" result was not a GPU, loader, ICD, subgroup, or package-corruption failure. `vulkaninfo --summary` reports the RTX 3070 (vendor `0x10de`, device `0x2488`), Vulkan `1.4.329`, NVIDIA driver `596.36`, and the Khronos validation layer. The affected test created the runtime with a null configuration; Windows adjacency discovery consequently looked for `shaders` beside `out/prometheus/native/marionette_tests.exe`, while the staged package is at `out/prometheus/native/SerialCanonical/shaders`. The RTX authority test now passes that explicit, externally staged package root. The package-backed route then creates and executes successfully.

Linux/WSL is a separate, honest negative admission result: the loader sees the same hardware through Mesa Dozen as `Microsoft Direct3D12 (NVIDIA GeForce RTX 3070)` but exposes Vulkan `1.2.335`, below Prometheus's Vulkan-1.4 contract. The FR-M0 test therefore skips after runtime capability admission. This does not affect the Windows RTX authority and was not bypassed.

## Reused accepted architecture

The implementation reuses M39b's persistent reduction ring, deterministic capacity growth, mapped input/output buffers, descriptor rebinding, command buffer, one synchronous queue submission, fence completion, timestamp query, diagnostics, package resolver, and cleanup path. It adds no allocator, handle registry, descriptor system, package resolver, or synchronization policy.

RQ-M1 remains closed as accepted in `PROMETHEUS_RQ_M1_MANAGED_BATCH_PROGRESS.md`; FR-M0 does not alter its camera oracle or timing-instrumentation deferrals.

## Public semantic contract

`PrometheusRowWiseSoftmaxRequest` and `prometheus_reactor_runtime_row_wise_softmax` accept caller-owned contiguous row-major FP32 input and output. Both declared element counts must exactly be `row_count * elements_per_row`; no strides are supported. The maximum admitted shape is 1,024 rows by 1,056 elements, with at most 16,777,216 elements (67,108,864 bytes per contiguous FP32 input or output array).

Calls complete synchronously. The runtime copies caller input before execution and copies all output before success returns. Exact same-base in-place calls are supported because input is copied into the runtime-owned mapped buffer before final output copyback; all other overlap is rejected. On failure, `output_written` remains zero and caller output is untouched. Zero rows or zero width with zero declared counts are synchronous no-ops with no dispatch or submission. The ABI uses the repository's `struct_size` minimum-size rule.

The contract rejects NaN, positive infinity, and negative infinity before any transfer or dispatch with `PROM_SOFTMAX_DETAIL_NONFINITE_INPUT` and reports the first nonfinite element. This deliberately preserves the accepted finite-only M39b numerical policy rather than silently inheriting shader-dependent nonfinite behavior. Finite rows use stable FP32 softmax: subtract row maximum, exponentiate, sum, and normalize. This covers one-element and constant rows, and large positive/negative finite magnitudes.

## Admitted topology and subgroup contract

Each nonempty FR-M0 request forces the fused M39b plan: one 256-invocation workgroup owns each row and all rows are `GroupID.x` in one physical dispatch. Each invocation scans columns separated by 256. `WaveActiveMax` reduces within each subgroup, subgroup leaders store partials to workgroup memory, subgroup zero merges those partials, and the same sequence performs the sum. The shader uses `GroupMemoryBarrierWithGroupSync` only for the workgroup-local merge; it does not claim cross-workgroup synchronization.

Admission requires compute-stage subgroup support plus subgroup arithmetic and basic operations, and requires that the physical subgroup size be nonzero, no larger than 256, and divide the 256-thread workgroup. Unsupported subgroup features and topology are rejected precisely. Width 1,056 is admitted; widths above it are rejected. FR-M0 does not route them through M39b's hierarchical composed route.

The former 1,024 bound was an initial public/planner safety choice, not a shader indexing, descriptor-range, mapped-buffer, workgroup-memory, or device limit. Each of the 256 lanes strides by 256 and therefore scans up to five elements at width 1,056. Workgroup memory contains subgroup partials, not one entry per input element. The generic M39b default planner still stages rows above 1,024; only FR-M0's explicitly forced fused route uses the separately audited 1,056 envelope. This preserves one-workgroup ownership and introduces no cross-workgroup synchronization claim.

The canonical SDSL-V source is `shaders/sdslv/production/reduction/softmax_fused.sdslv`. Production lowering uses DXC `vulkan1.3`, SPIR-V 1.6, and Vulkan-1.4 validation. Inspection of the generated module confirms `OpGroupNonUniformFMax`, `OpGroupNonUniformFAdd`, and `OpGroupNonUniformBroadcastFirst`.

## Package identity

Logical kernel identity remains `kernel-20-default` / `reduction-softmax-fused`. The rebuilt digest-addressed production object is `a173644e0ab363d7c399d2c21ab92bfac705de4728d67ef68863fd09389bc2e4`. `oct sdslv package build` and `oct sdslv package check` succeeded with 66 kernels and 65 deduplicated objects. No generated C payload authority or arbitrary caller-supplied SPIR-V route was added.

## RTX authority and validation

- `oct sdslv check`, DXC production compilation, and `spirv-val` passed.
- SPIR-V disassembly confirms the three subgroup operations above.
- Package build/check passed. Package metadata and resolver rejection paths remain covered by the package validation and native shader-registry lane.
- Linux native reactor and Marionette binaries built; smoke passed. Its FR-M0 GPU execution correctly skipped because WSL exposes only Vulkan 1.2.
- Windows native reactor and Marionette binaries built with MSVC.
- The public-header-only C foreign caller compiled with `-std=c11`.
- FR-M0 semantic boundary test passed: finite-policy rejection, output freshness, zero no-op, partial-alias rejection, and invalid-handle rejection.
- On the admitted Windows RTX 3070, with `PROMETHEUS_VK_VALIDATION=1`, the corpus passed for widths `1, 31, 32, 33, 64, 129, 256, 257, 1023, 1024, 1025, 1055, 1056`. It covers one element, constants, mixed signs, finite positive and negative extremes, non-powers-of-two, heterogeneous five-row batches, repeated execution, growth/reuse, alternating small/large calls, exact in-place operation, nonfinite rejection, partial-alias rejection, and recovery after rejection.
- Every nonempty call records exactly one physical dispatch and one synchronous submission, with all rows batched. The GPU test produced `out/test-artifacts/prometheus_fr_m0_rtx_authority.json`.
- RTX numerical maxima: absolute `1.49011611938e-08`; relative `4.83548900776e-07` (denominator floored at `1e-6` near zero); row-sum `1.09954271466e-07`; shift-invariance `0`; minimum output `1.14735188017e-09`; ordering violations `0`; Vulkan validation errors `0`.
- Capacity grew and descriptors rebound for the larger rows. After both persistent slots had reached the 1,056-row capacity, alternating one-element and 1,056-element calls added no buffer allocations and increased the reuse counter (52 at the authority snapshot). Successful calls always reported fresh output; pre-transfer nonfinite rejection left caller output unchanged, and a subsequent valid call succeeded.

Timing expansion is intentionally deferred: this closure is numerical and structural authority, not a mature performance milestone.

## Z-Image relation and limits

The accepted Z-Image path uses attention widths 32, 1,024, and 1,056. FR-M0 now admits all three through its independent one-workgroup-owned route. The existing production attention softmax remains model-reactor coupled and is not extracted; FR-M0 shares only the accepted reduction runtime and package plumbing.

Deferred inventory: standalone sum/max/dot, Welford mean/variance, layer normalization, prefix scan, and hierarchical multi-workgroup reductions.

## Exact next milestone

FR-M0 is closed. The exact next milestone, if approved separately, is a device-resident Z-Image attention-score handoff that retains explicit ownership and synchronization boundaries while reusing the admitted 1,056-row softmax kernel. It must not be folded into this host-array ABI or generalized into a fusion system.

