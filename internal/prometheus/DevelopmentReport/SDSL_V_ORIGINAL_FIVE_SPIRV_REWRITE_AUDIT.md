# SDSL-V original five SPIR-V rewrite audit

## M35a follow-up status (2026-07-12)

The audit-only Packed4 and FP16 candidate sources now use first-class SDSL-V
vector/intrinsic syntax instead of bounded inline HLSL. M35a completed the
language/compiler work, reran the pairwise hardware audit, and reran the same
lane with Vulkan validation enabled. Historical assets and production ownership
remain unchanged.

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

The rewrites use ordinary loops and guarded reads/writes. Packed4 now uses
first-class vector loads, component reads, and `Dot`. FP16 now uses
`Unpack<F16x2>`, component reads, and closed conversion intrinsics. Tensor
notation would obscure rather than improve these explicit storage/layout
contracts.

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
  FP32 accumulation.
- No shader requires barriers or explicit memory visibility beyond ordinary
  storage-buffer dispatch ordering.

The originals have historical RTX 3070 correctness/performance evidence in
P13 DVT/PX16 reports. M34a now adds direct generated-vs-original execution
evidence through the audit override with identical logical inputs, production
packing, production descriptors, production push constants, production
dispatch, timestamps, synchronization, and readback. The bounded timing
results are diagnostic only; they are not a public benchmark claim.

## Missing features and quality gaps

1. **Compiler/interface:** no explicit emitted entry-point name such as `main`.
2. **Host contract:** A2x4's existing production dispatch metadata says 2x2 although the
   shader produces 2x4; replacement must not accidentally canonize this drift.

M35a added the previously missing vector, dot, packed-format, bitcast, and
convert support; Packed4 and FP16 are no longer blocked on inline HLSL.

## Per-shader decisions

| Shader | Decision | Reason |
|---|---|---|
| SRT | **REPLACE** | passed all nine audit workloads, replay is deterministic, interface is clean, and bounded timing favored the generated shader |
| B2x2 | **REPLACE** | passed all nine audit workloads, preserved the exact host contract, and bounded timing strongly favored the generated shader |
| A2x4 | **REPLACE** | passed all nine audit workloads with truthful 2x4 audit metadata; production swap must correct the historical 2x2 metadata drift at the same time |
| Packed4 | **READY FOR M35b PRODUCTION REPLACEMENT** | M35a removed the inline-HLSL dependency, preserved exact SPIR-V size parity with the prior audit candidate, and reran the full hardware matrix cleanly |
| FP16 | **READY FOR M35b PRODUCTION REPLACEMENT** | M35a removed the inline-HLSL dependency, preserved exact SPIR-V size parity with the prior audit candidate, and reran the full hardware matrix cleanly |

## Overall recommendation and next milestone

M34b already replaces SRT, B2x2, and A2x4. M35a now closes the remaining
language/compiler gap for Packed4 and FP16 without mutating production assets.
M35b can use this evidence to perform the bounded production replacement work.

Production artifacts, registry entries, manifests, and runtime selection were
not changed during M34a. M34b subsequently promotes only SRT, B2x2, and A2x4;
this audit tree retains the historical headers as comparison fixtures.

## Validation recorded

M35a closeout reran the focused MSVC-built audit binary through the real SGEMM
host path. The bounded nine-shape matrix again passed CPU/original/candidate
comparisons for every pair, including the new inline-HLSL-free Packed4 and
FP16 candidates. A validation-enabled rerun recorded requested=`true`,
available=`true`, enabled=`true`, debug-utils-active=`true`,
`VK_LAYER_KHRONOS_validation`, warning count `0`, error count `0`, and device
loss `false`.

Bounded RTX 3070 audit timings at M=127,N=131,K=129 were:

- SRT: original 14368 / 14368 / 14464 ns, generated 8064 / 8256 / 8352 ns
- B2x2: original 28320 / 28480 / 28544 ns, generated 10752 / 10912 / 10944 ns
- A2x4: original 27008 / 27008 / 27040 ns, generated 31776 / 31776 / 31776 ns
- Packed4: original 9344 / 9376 / 9504 ns, generated 7072 / 7072 / 7072 ns
- FP16: original 11456 / 11488 / 11680 ns, generated 10432 / 10592 / 10752 ns

These are diagnostic audit samples, not a public benchmark contract.
Representative deterministic replay IDs now also include:

- Packed4:
  `Packed4FP32:6e05b9cfcc098acd:Packed4FP32:9e3e5735b0449db:3x5x7:99:main:SgemmPacked4_CS:1x1`
- FP16:
  `FP16-storage/FP32-accum:850db288b9c711c6:FP16-storage/FP32-accum:221182d05316b72b:3x5x7:99:main:SgemmFp16StorageFp32Accum_CS:1x1`

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
