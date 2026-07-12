# SDSL-V original five SPIR-V rewrite audit

Status: **IN PROGRESS — meaningful progression** (2026-07-11)

> M34a now has a test-owned audit descriptor/registry, arbitrary SPIR-V file
> preflight, embedded-module support, explicit entry-point validation,
> canonical footprint/dispatch calculation, deterministic replay identity, and
> JSON summary support. It remains intentionally outside production sources.
> The real Vulkan pipeline substitution and five hardware comparisons are still
> outstanding; this does not change any replacement decision.

This audit does not authorize a production replacement. It closes the source,
compiler, interface, validation, and static-structure portion for all five
opaque historical assets. Generated-shader hardware A/B execution remains
unproven because the production harness can select only registry-embedded
modules; it has no bounded SPIR-V override seam. Temporarily replacing headers
and registry entry names would make the evidence depend on a production edit,
contrary to this pass's audit-first requirement.

## Scope and authoritative originals

The exact five are registry shader IDs 10 through 14. This is not a filename
guess: `internal/prometheus/native/shaders/manifest.json` and
`reactor_shader_registry.c` identify them as the remaining source-language
`spirv`, `historical generated` assets corresponding to the early SRT,
B2x2, A2x4, Packed4, and FP16 paths. The embedded scalar baseline is not a
header artifact; tiled and memory-conservative have separate provenance; the
later shader IDs 4–9 are already SDSL-V generated.

| Shader | Authoritative original | Added by | Production status |
|---|---|---|---|
| SRT-2accum-K | `native/reactor_vulkan_srt_2accum_k_spirv.h` | `495ef752` | dispatchable occupancy variant |
| B2x2-row-major-biased | `native/reactor_vulkan_b2x2_row_major_biased_spirv.h` | `495ef752` | dispatchable occupancy variant |
| A2x4-row-biased-accum8 | `native/reactor_vulkan_a2x4_row_biased_accum8_spirv.h` | `495ef752` | dispatchable occupancy variant |
| Packed4FP32 | `native/reactor_vulkan_packed4_spirv.h` | `d4218969` | policy-selected packed-layout path |
| FP16-storage/FP32-accum | `native/reactor_vulkan_fp16_spirv.h` | `7bdab80c` | policy-gated packed-half path |

No readable original source is checked in for these five. Their authoritative
source representation is the immutable `uint32_t` word array. Extraction is
reproducible with `go run ./tools/spirv_header_extract <header> <out.spv>`.

## Interface inventory

All originals use `main`, Vulkan `GLCompute`, local size `8x8x1`, set 0 bindings
0/1/2, and a 12-byte push constant `{m@0,n@4,k@8}`. There are no specialization
constants, workgroup variables, barriers, atomics, subgroup operations, or
images. Global invocation X maps to row (or output-row block) and Y to column
(or output-column block).

| Shader | binding 0 / 1 | binding 2 | output per invocation | host representation |
|---|---|---|---|---|
| SRT | readonly scalar f32 / f32 | readwrite f32 | 1x1 | canonical row-major |
| B2x2 | readonly scalar f32 / f32 | readwrite f32 | 2x2 | canonical row-major |
| A2x4 | readonly scalar f32 / f32 | readwrite f32 | 2x4 | canonical row-major |
| Packed4 | readonly float4 / float4 | readwrite f32 | 1x1 | A row-packed, B column-packed, padded K |
| FP16 | readonly u32 / u32 | readwrite f32 | 1x1 | two binary16 lanes per u32, canonical linear order |

The SDSL-V entry names differ because current compilation names entries from
the shader (`Sgemm..._CS`). A production swap would need matching manifest and
registry entry strings, or a compiler option to emit `main`. Descriptor and
push layouts otherwise match exactly. B2x2/A2x4 generated dispatch metadata
must retain the existing 2x2 metadata convention; A2x4's historical metadata
already understates its 2x4 footprint, a pre-existing host-contract defect
documented by PX16 M12 and not silently corrected here.

Runtime consumers are the shader registry, Vulkan pipeline creation in
`reactor_vulkan_sgemm.c`, explicit occupancy benchmark dispatch for SRT/B2x2/
A2x4, and layout/precision selection for Packed4/FP16. Existing correctness and
benchmark coverage lives primarily in `reactor_stub_tests.cpp`,
`reactor_p13_m4_occupancy_benchmark_tests.cpp`, and
`reactor_px16_evt_benchmark_tests.cpp`. P13 DVT reports contain RTX 3070
evidence for the originals only.

## Rewrites and toolchain

Sources are under
`DevelopmentReport/artifacts/SDSL_V_ORIGINAL_SPIRV_REWRITE/sdslv/`; generated
VD-MIR, HLSL, SPIR-V, and disassembly are under `generated/`; extracted original
SPIR-V and disassembly are under `original/`.

The rewrites use ordinary loops and guarded reads/writes. Packed4 uses one
bounded inline-HLSL `dot(float4,float4)` because SDSL-V validation does not
currently expose float4 component access/dot. FP16 uses bounded inline HLSL for
half expansion and lane choice because conversion/bit-extraction intrinsics are
not first class. Tensor notation would obscure rather than improve these
explicit storage/layout contracts.

Compilation used DXC from Vulkan SDK 1.4.341.1 with `-spirv -T cs_6_0`, target
`vulkan1.0`, `-O3`, and no debug option. Both sides validate with
`spirv-val --target-env vulkan1.0`. Original producer versions and optimization
flags are not recoverable from the headers; therefore byte/instruction deltas
are informative, not proof of compiler superiority.

## Structural comparison

Counts are direct normalized disassembly counts; names and source markers are
ignored. “Instructions” counts `Op*` lines.

| Shader | bytes O/G | instructions O/G | blocks O/G | loads O/G | stores O/G | access chains O/G | branches O/G | phi O/G |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| SRT | 3716 / 2268 | 234 / 138 | 12 / 13 | 44 / 8 | 11 / 1 | 20 / 8 | 10 / 11 | 1 / 4 |
| B2x2 | 5960 / 3776 | 388 / 232 | 34 / 29 | 77 / 17 | 28 / 4 | 31 / 20 | 33 / 28 | 4 / 13 |
| A2x4 | 8356 / 5980 | 541 / 365 | 44 / 49 | 123 / 27 | 46 / 8 | 47 / 34 | 43 / 48 | 0 / 23 |
| Packed4 | 2788 / 2016 | 181 / 124 | 10 / 10 | 24 / 6 | 9 / 1 | 11 / 6 | 8 / 8 | 1 / 2 |
| FP16 | 4268 / 2468 | 280 / 144 | 16 / 10 | 37 / 6 | 22 / 1 | 13 / 6 | 11 / 8 | 1 / 2 |

DXC keeps loop accumulators and indices in SSA, explaining fewer explicit
loads/stores and more phi nodes. No generated module contains a barrier,
atomic, subgroup instruction, workgroup variable, dead helper function, or
unexpected capability. Guarded B2x2/A2x4 reads become structured selections;
tail stores remain conditional. The A2x4 rewrite has more blocks/branches than
the original despite fewer total instructions, so smaller size is not treated
as unqualified improvement. Addressing is visibly direct and no resource load
is duplicated merely to feed multiple accumulators.

## Semantic review and unresolved proof

- SRT preserves two independent accumulation chains and odd-K guarding, but
  combining `even + odd` changes floating-point reduction order relative to a
  scalar K-order oracle. It must be compared against the original with the
  repository tolerance, not exact bits.
- B2x2 and A2x4 preserve per-input guards and per-output tail stores. They also
  preserve the useful 2x2 and 2x4 footprints rather than the misleading public
  “4x4” label.
- Packed4 preserves padded float4 storage and dot accumulation. Its host packing
  contract, not source array syntax, determines compatibility.
- FP16 preserves packed-u32 resources, binary16 expansion, scalar K order, and
  FP32 accumulation. The inline HLSL is valid SPIR-V but is a portability and
  maintainability compromise.
- No shader requires barriers or explicit memory visibility beyond ordinary
  storage-buffer dispatch ordering.

The originals have historical RTX 3070 correctness/performance evidence in
P13 DVT/PX16 reports. The generated modules have **no execution evidence yet**.
It would be false to transfer that evidence based on matching interfaces or
successful validation. No generated-vs-original performance claim is made.

## Missing features and quality gaps

1. **Language syntax/validator:** first-class vector component access or dot is
   missing for resource-loaded float4 values (Packed4 workaround).
2. **Language/intrinsics:** typed half unpack, bit shifts/masks-to-lanes, and
   scalar/vector conversion intrinsics are not first class (FP16 workaround).
3. **Compiler/interface:** no explicit emitted entry-point name such as `main`.
4. **Test infrastructure:** no bounded real-interface SPIR-V override/pairwise
   audit harness with identical buffers, dispatch, timestamps, and validation.
5. **Host contract:** A2x4's existing dispatch metadata says 2x2 although the
   shader produces 2x4; replacement must not accidentally canonize this drift.

No compiler feature was added during this audit.

## Per-shader decisions

| Shader | Decision | Reason |
|---|---|---|
| SRT | **DO NOT REPLACE YET** | clean expression and better static shape, but no generated hardware/replay/performance proof |
| B2x2 | **DO NOT REPLACE YET** | clean guarded rewrite and comparable structure, but no generated hardware proof |
| A2x4 | **DO NOT REPLACE YET** | interface is expressible, but branch count grows and host metadata drift plus missing hardware proof remain |
| Packed4 | **DO NOT REPLACE YET** | interface matches, but idiomatic source still needs inline HLSL and no generated hardware proof exists |
| FP16 | **DO NOT REPLACE YET** | exact packed interface is expressible only with conversion escape hatches and lacks generated hardware proof |

## Overall recommendation and next milestone

Retain all five originals for now. Add one small audit-only Vulkan comparison
seam that accepts a SPIR-V path plus entry name while reusing the real SGEMM
buffer packing and timestamp path. Run original/generated pairs on identical
boundary, odd-K, tail, repeated, and representative benchmark shapes on the
RTX 3070 with validation layers enabled. If those results are green, SRT and
B2x2 are the strongest immediate replacement candidates; A2x4 requires an
explicit metadata decision; Packed4/FP16 benefit from narrow intrinsic cleanup
before replacement.

Production artifacts, registry entries, manifests, and runtime selection were
not changed. Rollback is therefore deletion of this audit tree only.

## Validation recorded

- five `sdslv check` passes
- five VD-MIR and HLSL emissions
- five DXC Vulkan 1.0 `-O3` compilations
- ten `spirv-val` passes (five original, five generated)
- ten successful `spirv-dis` outputs
- direct descriptor/push/workgroup interface inspection

The following requested repository lanes also passed in this checkout:
`go test ./internal/source`, `go test ./internal/diagnostic`,
`go test ./internal/sdslv/...`, `go test ./cmd/oct`,
`go test ./internal/... ./cmd/oct`, native-manifest check, Linux build-script
syntax check, and `git diff --check`. Production SGEMM hardware checks and the
M29–M33c execution suites were not rerun because the compiler was unchanged;
more importantly, those lanes cannot close the missing generated pairwise
execution proof.
