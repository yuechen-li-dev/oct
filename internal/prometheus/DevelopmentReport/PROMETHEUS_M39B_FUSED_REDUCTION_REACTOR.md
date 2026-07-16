# Prometheus M39b fused reduction reactor

## Outcome

M39b is **production-ready for bounded row-wise FP32 sum, max, and stable
softmax**. The implementation has no CPU fallback. CPU routines are correctness
oracles only. The production selector is deliberately simple: softmax widths up
to and including 1024 use the fused one-workgroup shader; larger widths use the
explicit five-dispatch plan.

The implementation is confined to reduction work. No convolution, FFT,
graphics, Direct3D, PTX, distributed scheduling, model import, arbitrary-axis
tensor executor, or generic fusion language was introduced.

## Stub audit

The pre-M39b `internal/prometheus/native/reactor_vulkan_fused_reduction.c` was a
15-line P12 M4 topology placeholder. It claimed only the translation-unit name
for a possible future family and mentioned softmax, LayerNorm, reduction, and
scan as examples. Its explicit non-claims were public ABI, capability reporting,
probe behavior, and runtime behavior.

Implementation-oriented findings:

- The file and its inclusion in the native source manifest were real. Every
  proposed type, function, resource, and behavior was a placeholder.
- It represented no input/output buffers, row shape, reduction axis,
  temporaries, dispatch, stage, registry asset, pipeline, timestamp, validation,
  request identity, or physical slot.
- It therefore neither assumed one dispatch nor supported staged execution. It
  predated and did not participate in the M29 persistent submission ring,
  M30/M30a multi-token quarantine/reap semantics, or M31 shared-ring batching.
- `internal/prometheus/native/README.md` and
  `internal/prometheus/DevelopmentReport/P12_M1_VULKAN_REACTOR_FILE_TOPOLOGY_PLAN.md`
  described the file as inert. The earlier
  `internal/prometheus/DevelopmentReport/PROMETHEUS_R0_FORENSIC_ARCHITECTURE_AUDIT.md`
  correctly classified it as an uncertain placeholder.
- The fused-reduction family name still matches current Prometheus vocabulary.
  The filename and family separation were retained. The inert body and its
  speculative ownership of LayerNorm/scan were removed rather than preserved.

The replacement was audited against
`PX16_M29_SUBMISSION_RING.md`, `PX16_M30_MULTITOKEN_ASYNC.md`,
`PX16_M30A_ASYNC_QUARANTINE_REAP.md`, `PX16_M31_BATCH_REFILL_RING.md`,
`SDSL_V_M39A_WORKSPACE_PRODUCTIZATION.md`, the native SGEMM implementation, and
the current shader manifest/registry. SGEMM was used only as a lifecycle and
integration reference.

## Production contract

`PrometheusReductionRequest` in `internal/prometheus/native/reactor_api.h`
expresses:

- contiguous row-major FP32 `input` and `output` host views;
- `row_count` and `elements_per_row`;
- exact input and output element counts for size validation;
- operation: sum, max, or softmax;
- no finalization for sum/max and stable-softmax finalization for softmax;
- optional test/benchmark strategy forcing for fused versus composed softmax.

Sum and max write one FP32 scalar per row. Softmax writes one FP32 element per
input element. Rows and widths must be nonzero. The production envelope is:

- rows: 1 through 1024;
- elements per row: 1 through 1,048,576;
- total elements: at most 16,777,216;
- local size: 256;
- elements per stage-1 partial: 1024;
- at most eight plan records; current plans use at most five.

The API is honestly row-wise only. It does not represent an arbitrary axis,
strides, a graph, or dynamic tensor layout. Invalid size, operation,
finalization, strategy, nonfinite input, plan metadata, and temporary capacity
have distinct `PROM_REDUCTION_DETAIL_*` results.

`PrometheusReductionPlan` is the machine-readable trace. It records operation,
shape, strategy, local size, partial count, stage count, temporary alignment and
bytes, per-stage shader/implementation IDs and dispatch geometry, and a stable
FNV-derived replay ID.

## Algorithms and dispatch planning

For sum/max at width <= 1024, one workgroup owns one row. Each invocation loads
a strided subset, then 256 shared values reduce with a fixed eight-round tree.
The tree is deterministic for a fixed configuration. At larger widths, stage 1
dispatches one group per 1024-element row chunk into a device-local partial
buffer; stage 2 dispatches one group per row over those partials. The width
limit bounds the partial count to 1024, so two stages are sufficient and no row
is truncated.

For softmax at width <= 1024, one group performs max reduction, recomputes
`exp(x - max)` for the sum reduction, and recomputes it once more for normalized
output. This avoids an exponent buffer. At larger widths the five GPU stages
are:

1. row-max partials;
2. final row max;
3. shifted-exp sum partials;
4. final row sum;
5. broadcast normalization.

All stages are recorded into one slot-owned command buffer, use explicit
compute-write to compute-read/write barriers, and are submitted once behind one
fence. No CPU readback occurs between stages.

The user-suggested SDSL-V cleanup is part of the production sources: the large
repeated `thread.GroupIndex` reduction ladders are one bounded loop over an
exhaustive payload enum, `ReductionLane.Idle | Active { Partner: f32 }`, selected
and consumed with `match`. The generated HLSL/SPIR-V and hardware results agree
with the prior algorithm while keeping the lane decision explicit.

## Temporary storage and lifecycle

Temporary storage is explicit:

- sum/max staged: `rows * ceil(width / 1024) * 4` bytes;
- composed softmax: the same partial allocation when partial count is greater
  than one, plus `rows * 4` bytes for row maxima and `rows * 4` bytes for row
  sums;
- fused softmax: zero temporary bytes;
- logical alignment: four bytes; Vulkan allocation obeys the device memory
  requirements returned for each buffer.

Each reduction ring slot owns reusable host-visible coherent input/output
buffers and device-local partial/max/sum buffers. Buffers grow only when the
slot's capacity is insufficient and are then reused; Vulkan memory is not
allocated/freed per dispatch. Requests beyond the documented envelope are
rejected, never spilled to system RAM.

The reduction family reuses the current Vulkan instance, physical/logical
device, compute queue, and command pool, but owns a bounded persistent ring
(default depth two, configurable one through four). Each slot owns one command
buffer, fence, eight descriptor sets, timestamp query pair, generation, and
temporary buffers. The family creates its common descriptor layout, pipeline
layout, descriptor pool, query pool, and five pipelines once.

One logical request retains one request ID even when it has five dispatches.
Logical validation/execution failure increments logical failure evidence.
Physical completion uncertainty quarantines the slot; reap waits for a known
fence result before recycling it. Slot generation and request identity make
replay/freshness visible. Injected late-stage failures prove that logical
failure does not imply physical leakage, and runtime destruction releases
family state before the shared Vulkan owners.

This synchronous reduction ABI does not introduce a second token system. The
existing SGEMM stale/fresh multi-token and shared-ring semantics are unchanged;
the full existing native suite remains their authority.

## Numerical behavior and correctness oracle

GPU input and accumulation are FP32. Sum is not associative, so comparison uses
documented absolute/relative tolerances rather than requiring a double-precision
ordering match. Max starts from negative infinity after host validation.

M39b deliberately rejects every NaN, positive infinity, and negative infinity
before GPU submission and reports the first nonfinite index. Thus max
comparisons cannot accidentally turn NaN into a false pass, and softmax's valid
domain is finite FP32 input. Empty rows are invalid.

Softmax computes `max(x)`, then `exp(x - max)`, then normalizes by the shifted
sum. Tests cover large positive and negative offsets, equal values, a dominant
value, odd widths, and multi-stage rows. The oracle additionally requires every
output to be finite and nonnegative and every row sum to be within `3e-4` of
one. Mismatch reports include row, column, expected, actual, absolute error, and
relative error; scalar reductions report the row.

The only targeted inline HLSL in reduction source is the one-expression `exp`
primitive because SDSL-V has no `Exp` intrinsic. All control flow, resource
access, staging, reduction, and normalization remain readable SDSL-V.

During implementation, stream resource-bundle lowering was found to discard
explicit field `[binding(n)]` attributes and then alphabetically reassign
bindings. That contradicted documented SDSL-V binding semantics and initially
produced valid but incorrectly wired SPIR-V. `internal/sdslv/lower/lower.go`
now preserves those attributes, and
`TestModulePreservesExplicitBindingsFromStreamResourceBundle` permanently
covers the fix.

## Shader inventory and ownership

All five sources are production-owned under
`internal/prometheus/shaders/sdslv/production/reduction/`:

| Shader ID | Implementation ID | Source | Role | SPIR-V SHA-256 |
|---:|---:|---|---|---|
| 16 | 1001 | `row_sum_stage.sdslv` | row sum / partial / final | `99709a55cc1a061384468dc3d7e8cedaec0e9543c82227cb5d672fc0d1c769d0` |
| 17 | 1002 | `row_max_stage.sdslv` | row max / partial / final | `8f65e2143b30dc174a6f063e70ca6e1fe46cb28284bdbee4aa3da3a3f4bff4c2` |
| 18 | 1003 | `softmax_exp_sum_stage.sdslv` | shifted-exp sum partial | `0586fa39a0f51a60654e42f27848b0ec752ce100cf6917027f97d571faadf02b` |
| 19 | 1004 | `softmax_normalize.sdslv` | strided input, packed broadcast/finalization | `f02c36221658b6f88a129d33c0248d582d55f67672deb9f7bd5dd73c512aaee7` |
| 20 | 1005 | `softmax_fused.sdslv` | strided input, packed one-workgroup stable softmax | `4c57993dfb3e66caddc251109f8570edef49f38ff9bc0ab411c0a6bfa91891dc` |

The native manifest owns source, generated header, symbol, entry point, local
size, four descriptor bindings, 32-byte push constants, stage role, and width
envelope. The registry exposes reduction production assets separately from the
stable SGEMM production table, and fixed reduction implementations are never
SGEMM-selector eligible. Experimental accessors are empty. The workspace check
enforces ID uniqueness, deterministic ordering, source/header provenance,
implementation references, authority agreement, selector isolation, and
production/experimental root confinement.

Every generated module passed `spirv-val` through the Vulkan SDK 1.4.350.0
toolchain. Repeated generation produced identical headers.

## GPU evidence

Hardware: NVIDIA GeForce RTX 3070, vendor 4318, device 9352, NVIDIA driver
596.36 (packed version 2500395008), Vulkan device API 1.4.329 (packed version
4211017), compute queue family 0. The evidence base revision was
`ef1c24cb1c0f087dfd16aceb780e64bd2d2b2347`. DXC was
`C:\\VulkanSDK\\1.4.350.0\\Bin\\dxc.exe`, identity
`1.10(5347-fe261573) / 1.9.0.5347`; the Windows build used MSVC 14.51.36231
from Visual Studio 2026 18.7.3.

Validation-enabled correctness passed all seven permanent native reduction
facts and all six permanent `.sdslvtest` cases. The plan corpus covers widths
1, 2, 3, 31, 32, 33, 63, 64, 65, 127, 128, 129, 255, 256, 257, 511, 512,
513, 1024, and 4096, and row counts 1, 2, 16, 128, and 1024 without a hardware
Cartesian explosion. Validation errors and device loss were zero.

The final Windows native sweep passed 364 facts (332 passed, 32 intentional
capability/path skips, zero failures). The focused reactor sweep passed 94 with
two expected FP16-selection skips. M29 submission ring, M30 multi-token async,
M30a quarantine/reap, and M31 shared-ring refill facts passed independently.
The full requested Go matrix, native manifest check, workspace check,
`bash -n`, and `git diff --check` passed. The Linux builder produced the ELF
reactor and Marionette binaries and passed its harness smoke check. No Linux
Vulkan hardware execution is claimed.

Selected RTX 3070 medians from the bounded benchmark corpus:

| Workload | Stages | Temporary bytes | GPU median | End-to-end median |
|---|---:|---:|---:|---:|
| sum 1x32 | 1 | 0 | 6.912 us | 97.6 us |
| sum 128x4096 | 2 | 2048 | 90.336 us | 683.9 us |
| max 1x32 | 1 | 0 | 6.976 us | 94.8 us |
| max 128x4096 | 2 | 2048 | 89.760 us | 682.6 us |
| softmax 1x64 | 1 | 0 | 8.448 us | 98.2 us |
| softmax 1024x512 | 1 | 0 | 117.984 us | 9.453 ms |
| softmax 16x4096 | 5 | 384 | 40.800 us | 1.346 ms |

At 128 rows, fused versus composed softmax GPU medians were 10.912/16.128 us
at width 64, 12.160/17.568 us at 128, 14.336/21.824 us at 256,
20.736/32.576 us at 512, and 40.224/53.792 us at 1024. Fused won every tested
width, which supports the bounded 1024 threshold without a device tuning table.

The run created five pipelines, used ring depth two, recorded 40 buffer grows,
1220 buffer reuses, and 6144 bytes of retained temporary capacity, and ended
with zero validation errors. Every benchmark
record includes min/median/max GPU and end-to-end time, correctness, validation,
stage count, temporary bytes, replay ID, and device-loss status in
`out/test-artifacts/PrometheusReduction_M39bRtx3070Corpus/`.

## Production decision and next workload

Classification: **Production-ready for bounded row-wise reductions and
softmax**. Correctness, validation, lifecycle safety, replay identity, bounded
performance evidence, source/artifact ownership, persistent reuse, and absence
of CPU fallback all meet the promotion bar.

The exact next recommended workload is a real attention-score row softmax with
128 rows by 1024 elements, consuming an SGEMM-produced Vulkan buffer directly.
That will test cross-reactor device-buffer handoff before adding LayerNorm or
RMSNorm. Those normalization operations fit the map/reduce/finalize plan shape,
but remain explicitly outside M39b.

## M40b device-input extension

M40b completed that exact next workload without changing the public reduction
API or the fused/staged threshold. The bounded composition owner binds an
internal device-buffer view as reduction input. `InputRowStride` now controls
fused and normalize reads, while `ElementsPerRow` remains the logical softmax
width and packed output stride. Ordinary M39b calls retain identical behavior
because their input stride equals their logical width.

The SGEMM output stays owned by the shared persistent slot until softmax and
the optional final readback complete. The producer-to-consumer barrier is
compute shader write to compute shader read on the exact C buffer. Completion
uncertainty quarantines the whole slot, and persistent-B replacement waits for
physical reap. Full ownership, command traces, RTX 3070 evidence, and the
experimental classification are recorded in
`PROMETHEUS_M40B_DEVICE_RESIDENT_COOPERATIVE_INFERENCE_PATH.md`.
