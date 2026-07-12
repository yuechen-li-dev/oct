# SDSL-V M35a — Packed vector intrinsics

Status: **COMPLETE — success** (2026-07-12)

## Scope and outcome

M35a closes the last two language/compiler gaps that kept the audited Packed4
and FP16 SGEMM rewrites behind inline HLSL. SDSL-V now supports:

- first-class vector component reads for `float2`–`float4` and `uint2`–`uint4`
- `Dot(float2|float3|float4, same) -> f32`
- closed compiler-defined generic intrinsics
- `Pack<F16x2>(float2) -> u32`
- `Unpack<F16x2>(u32) -> float2`
- closed same-width scalar `Bitcast<T>`
- closed scalar numeric `Convert<T>`

These are compiler-owned intrinsic families, not user-defined generics.
Concepts, configs, and template shaders remain the only user specialization
mechanism. Production Packed4/FP16 shaders, registry entries, and manifests
remain unchanged in M35a.

## Motivating audit evidence

The audit-only Packed4 rewrite previously required one inline-HLSL fragment:
`dot(float4, float4)`.

The audit-only FP16 rewrite previously required:

- packed-half expansion via `f16tof32`
- lane extraction from a packed `u32`
- lane selection through inline ternaries

Both audit rewrites now contain no `HLSL<...>` blocks and still compile to
valid SPIR-V. Their generated SPIR-V byte sizes remain exactly equal to the
prior inline-HLSL candidates:

- Packed4 candidate: `2016` bytes
- FP16 candidate: `2468` bytes

## Closed intrinsic and component matrix

Validator-owned supported forms:

- component reads:
  - `.x` on width `2..4`
  - `.y` on width `2..4`
  - `.z` on width `3..4`
  - `.w` on width `4`
  - base types: `float2|float3|float4|uint2|uint3|uint4`
- `Dot(float2, float2) -> f32`
- `Dot(float3, float3) -> f32`
- `Dot(float4, float4) -> f32`
- `Unpack<F16x2>(u32) -> float2`
- `Pack<F16x2>(float2) -> u32`
- `Bitcast<u32>(f32) -> u32`
- `Bitcast<f32>(u32) -> f32`
- `Bitcast<i32>(u32) -> i32`
- `Bitcast<u32>(i32) -> u32`
- `Convert<f32>(u32|i32) -> f32`
- `Convert<u32>(f32|i32) -> u32`
- `Convert<i32>(f32|u32) -> i32`

Rejected forms now fail in validation before lowering:

- component access on scalars
- out-of-range vector lanes such as `float2.z` or `float3.w`
- `Dot` with wrong arity, scalar inputs, mixed widths, or mixed kinds
- unknown intrinsic families or unknown packed formats
- generic syntax on ordinary user functions
- unsupported `Bitcast` width/pair combinations
- unsupported `Convert` source/target pairs

`F16x2` is a compiler-known format descriptor only. It is not a runtime type,
cannot be named as an ordinary value type, and cannot be extended by user code.

## Compiler representation and lowering

The parser now records compiler-generic call syntax directly on `ast.CallExpr`,
including the type/format argument and `<` / `>` spans. Validation resolves the
closed intrinsic matrix into compiler-owned metadata rather than leaving the
backend to infer meaning from names. Lowering preserves the semantics as
backend-neutral VD-MIR nodes:

- `VectorExtractExpr`
- `IntrinsicCallExpr` for `Dot`, `Pack`, `Unpack`, `Bitcast`, and `Convert`

HLSL emission is native and deterministic:

- vector lane read → `value.x` / `value.y` / `value.z` / `value.w`
- `Dot` → `dot(a, b)`
- `Unpack<F16x2>` → `f16tof32(uint2(low, high))`
- `Pack<F16x2>` → `f32tof16(...)` plus low/high lane combine
- `Bitcast` → `asuint` / `asfloat` / `asint`
- `Convert` → explicit scalar casts

Exactly-once evaluation is preserved through validation, lowering, and HLSL
materialization; nested `Pack<F16x2>(...)` now forces operand prelude
materialization instead of duplicating evaluation.

## Parser, semantic, and corpus coverage

Focused permanent coverage added in this milestone:

- parser tests for vector lane reads, compiler-generic intrinsic syntax, nested
  intrinsic calls, malformed generic syntax, and exact type-argument spans
- validator tests for the full approved component/intrinsic matrix
- diagnostic span tests for scalar component rejection, bad lanes, bad generic
  syntax, and unsupported intrinsic combinations
- lowering and HLSL tests for native lowering and exactly-once materialization
- dedicated valid corpus:
  - `internal/sdslv/testdata/language/m35a-valid/VectorComponentsAndDot.sdslvvalid`
  - `internal/sdslv/testdata/language/m35a-valid/PackedIntrinsicsAndConversions.sdslvvalid`
- dedicated invalid corpus under
  `internal/sdslv/testdata/language/m35a-invalid/`

## GPU execution suite

`examples/SDSL-V/M35a/PackedVectorIntrinsics.sdslvtest` passes on the native
host path and covers:

- float2/3/4 component access
- `Dot` for float2/3/4
- `Unpack<F16x2>` lane order
- `Pack<F16x2>` round-trip for exactly representable values
- signed zero
- infinity
- stable NaN behavior
- `Bitcast` round-trip
- `Convert` numeric semantics
- multiple invocations and stable replay

Stable case IDs:

- multiple-invocation replay proof:
  `sdslv-cee03b47422cb60a9e363bfb`
- NaN behavior proof:
  `sdslv-f5308ad68af2b2c32785d8e0`

The stable replay case reran cleanly by explicit `--case` selection.

## Packed4 and FP16 hardware audit rerun

The existing M34a pairwise audit harness reran successfully on real Vulkan
hardware for the nine-workload matrix. Historical original, new intrinsic
candidate, and CPU oracle all agreed for every row. The prior inline-HLSL
candidate was not retained as a distinct runtime artifact in this tree; M35a
therefore proves parity by direct original-vs-new hardware replay plus exact
generated-size parity with the prior inline-HLSL candidate.

Representative `M=127, N=131, K=129` audit timings:

| Pair | Historical original min / median / max ns | New intrinsic candidate min / median / max ns |
|---|---:|---:|
| Packed4FP32 | `9344 / 9376 / 9504` | `7072 / 7072 / 7072` |
| FP16-storage/FP32-accum | `11456 / 11488 / 11680` | `10432 / 10592 / 10752` |

Representative replay IDs (`M=3, N=5, K=7`, seed `99`):

- Packed4:
  `Packed4FP32:6e05b9cfcc098acd:Packed4FP32:9e3e5735b0449db:3x5x7:99:main:SgemmPacked4_CS:1x1`
- FP16:
  `FP16-storage/FP32-accum:850db288b9c711c6:FP16-storage/FP32-accum:221182d05316b72b:3x5x7:99:main:SgemmFp16StorageFp32Accum_CS:1x1`

Audit-source status:

- Packed4 audit rewrite is inline-HLSL-free
- FP16 audit rewrite is inline-HLSL-free

## SPIR-V accounting

Current original vs new intrinsic candidate structural counts:

| Shader | bytes O/G | instructions O/G |
|---|---:|---:|
| Packed4FP32 | `2788 / 2016` | `181 / 124` |
| FP16-storage/FP32-accum | `4268 / 2468` | `280 / 144` |

Generated SPIR-V validates with `spirv-val`, contains no inline-HLSL source
placeholder, and preserves the expected backend shape without unexpected
capabilities or control-flow expansion.

## Validation-enabled lane

With `PROMETHEUS_VK_VALIDATION=1`, the rerun recorded:

- requested: `true`
- available: `true`
- enabled: `true`
- debug utils active: `true`
- enabled layer: `VK_LAYER_KHRONOS_validation`
- warning count: `0`
- error count: `0`
- device loss: `false`

This evidence now comes from runtime validation accounting captured directly
from the SGEMM runtime during the pairwise audit run.

## Validation commands

The following requested validation commands passed in this checkout:

- `go test ./internal/source`
- `go test ./internal/diagnostic`
- `go test ./internal/sdslv/...`
- `go test ./cmd/oct`
- `go test ./internal/... ./cmd/oct`
- `go run ./tools/prometheus_native_manifest -check`
- `bash -n internal/prometheus/native/build_linux.sh`
- `git diff --check`

Additional M35a proof commands passed:

- `go run ./cmd/oct sdslv check` on both M35a valid fixtures
- `go run ./cmd/oct sdslv test examples/SDSL-V/M35a/PackedVectorIntrinsics.sdslvtest`
- `go run ./cmd/oct sdslv test ... --case sdslv-cee03b47422cb60a9e363bfb`
- `out\\prometheus\\native\\marionette_tests.exe PrometheusAuditOriginalFivePairwiseHardware`
- the same audit lane with `PROMETHEUS_VK_VALIDATION=1`

## Production boundary and M35b handoff

M35a intentionally stops at language/compiler proof plus audit-only shader
evidence. Production Packed4/FP16 assets remain unchanged. M35b can now focus
on bounded production replacement work using already-proven first-class SDSL-V
vector and packed-format intrinsics rather than inline HLSL.

## M35b completion

M35b promoted both candidates into the production SDSL-V tree with identical
bytes: Packed4 is 2016 bytes and FP16 is 2468 bytes. See
`PROMETHEUS_M35B_FINAL_HISTORICAL_SHADER_REPLACEMENT.md` for production,
validation, archive, and rollback evidence.
