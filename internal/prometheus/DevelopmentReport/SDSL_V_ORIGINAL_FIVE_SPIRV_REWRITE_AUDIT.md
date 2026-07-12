# SDSL-V original five SPIR-V rewrite audit

Status: **COMPLETE — success** (2026-07-11)

> M34a is now closed with a real Vulkan audit override, deterministic replay
> JSON, pairwise original/generated execution on hardware, and bounded timing
> evidence for all five original historical shaders and their SDSL-V
> replacements. The audit harness remains intentionally outside production
> policy and did not mutate the production registry, manifest, or binaries.

This audit still does not authorize an in-place production edit inside M34a.
It now closes the source, compiler, interface, hardware correctness, replay,
and bounded timing proof required to choose follow-up replacement candidates.

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
push layouts otherwise match exactly. B2x2 keeps the expected 2x2 dispatch
metadata. A2x4's generated shader and audit descriptor use truthful 2x4
footprint metadata; the production registry still carries the historical 2x2
value, which must be corrected during any production replacement.

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

## Semantic review and hardware proof

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
P13 DVT/PX16 reports. M34a now adds direct generated-vs-original execution
evidence through the audit override with identical logical inputs, production
packing, production descriptors, production push constants, production
dispatch, timestamps, synchronization, and readback. The bounded timing
results are diagnostic only; they are not a public benchmark claim.

## Missing features and quality gaps

1. **Language syntax/validator:** first-class vector component access or dot is
   missing for resource-loaded float4 values (Packed4 workaround).
2. **Language/intrinsics:** typed half unpack, bit shifts/masks-to-lanes, and
   scalar/vector conversion intrinsics are not first class (FP16 workaround).
3. **Compiler/interface:** no explicit emitted entry-point name such as `main`.
4. **Host contract:** A2x4's existing production dispatch metadata says 2x2 although the
   shader produces 2x4; replacement must not accidentally canonize this drift.

No compiler feature was added during this audit.

## Per-shader decisions

| Shader | Decision | Reason |
|---|---|---|
| SRT | **REPLACE** | passed all nine audit workloads, replay is deterministic, interface is clean, and bounded timing favored the generated shader |
| B2x2 | **REPLACE** | passed all nine audit workloads, preserved the exact host contract, and bounded timing strongly favored the generated shader |
| A2x4 | **REPLACE** | passed all nine audit workloads with truthful 2x4 audit metadata; production swap must correct the historical 2x2 metadata drift at the same time |
| Packed4 | **REPLACE AFTER MINOR COMPILER CLEANUP** | hardware result is green and timing is encouraging, but bounded inline-HLSL `dot(float4,float4)` remains a maintainability gap |
| FP16 | **REPLACE AFTER MINOR COMPILER CLEANUP** | hardware result is green and timing is encouraging, but bounded inline HLSL half-unpack/bit-lane logic still needs first-class language support |

## Overall recommendation and next milestone

Replace SRT, B2x2, and A2x4 in a follow-up production milestone. Correct
A2x4's production metadata to canonical 2x4 during that swap. Add narrow
first-class vector/dot and FP16/bit-conversion intrinsics, rerun this audit
harness, then replace Packed4 and FP16. M34a itself leaves production assets
unchanged and supplies the isolated audit evidence needed for that follow-up.

Production artifacts, registry entries, manifests, and runtime selection were
not changed during M34a. M34b subsequently promotes only SRT, B2x2, and A2x4;
this audit tree retains the historical headers as comparison fixtures.

## Validation recorded

M34a closeout: a focused MSVC-built audit binary runs all five immutable
original module words and their generated audit-tree candidates through the
real SGEMM host path. The bounded nine-shape matrix passed CPU/original/
candidate comparisons for every pair, including odd K, M tails, N tails,
combined tails, Packed4 padding cases, FP16 packed-lane preparation, and
deterministic replay. Validation accounting records that the current SGEMM
runtime did not request validation layers, `VK_LAYER_KHRONOS_validation` was
available on this machine, enabled-layer names were empty, and warning/error
counts were both zero in emitted audit JSON.

Bounded RTX 3070 audit timings at M=127,N=131,K=129 were:

- SRT: original 14368 / 14368 / 14464 ns, generated 8064 / 8256 / 8352 ns
- B2x2: original 28320 / 28480 / 28544 ns, generated 10752 / 10912 / 10944 ns
- A2x4: original 27008 / 27008 / 27040 ns, generated 31776 / 31776 / 31776 ns
- Packed4: original 9472 / 9664 / 9728 ns, generated 7168 / 7168 / 7200 ns
- FP16: original 11584 / 11616 / 12096 ns, generated 10624 / 10784 / 12480 ns

These are diagnostic audit samples, not a public benchmark contract. The
deterministic passing replay ID is
`SRT-2accum-K:63c5d6c24176ea5d:SRT-2accum-K:ab97b49624dd0965:3x5x7:99:main:SgemmSrt2AccumK_CS:1x1`.
The synthetic failing replay reproduces the same ID and reports its first
mismatch at row 0, column 0 with expected/original `0.57421875` and candidate
`1.07421875`.

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
syntax check, and `git diff --check`. Production SGEMM registry content,
manifest content, and production binaries remained unchanged throughout M34a.
