# SDSL-V M39a workspace productization

Status: **COMPLETE — layout and ownership only** (2026-07-15)

## Problem and authority inventory

The prior Prometheus SDSL-V source root mixed all registry-owned sources in one
flat directory. It did not say whether a nearby source was production,
experimental, or historical, while generated headers and audit evidence lived
under different ownership systems. The complete significant-path inventory is:

| Path | Classification | Authority |
|---|---|---|
| `internal/sdslv/{lex,parse,validate,lower,emit,toolchain,vdmir}` | compiler/tooling | Go implementation |
| `internal/sdslv/testdata/language/` | permanent language test | SDSL-V fixture corpus |
| `internal/sdslv/test/out/` | temporary output | uncommitted test scratch |
| `examples/SDSL-V/*.sdslv` | example/tutorial | examples only |
| `examples/SDSL-V/*.sdslvtest` | permanent GPU test | bounded GPU test corpus |
| `examples/SDSL-V/M36a/BasicBenchmarks.sdslvbench` | benchmark corpus | stable benchmark/replay source |
| `examples/SDSL-V/M36a/artifacts/` | canonical benchmark artifact | manifest hash record |
| `internal/prometheus/shaders/sdslv/production/sgemm/` | production source authority | Prometheus registry source |
| `internal/prometheus/shaders/sdslv/experimental/` | experimental source | no source exists yet; policy boundary |
| `internal/prometheus/shaders/sdslv/historical/` | historical/audit pointer | no duplicate files |
| `internal/prometheus/native/reactor_vulkan_*_spirv.h` | generated production artifact | native manifest/generator-owned headers |
| `internal/prometheus/native/shaders/manifest.json` | production registry metadata | shader/implementation IDs |
| `internal/prometheus/native/generate_sdslv_shaders.ps1` | regeneration tooling | production header generator |
| `tools/{generate_m36b_canonical,sdslv_workspace_check}` | regeneration/verification tooling | canonical artifacts and path checks |
| `pkg/octxiliary/kaijuvulkan/` | sidecar/runtime integration | typed Kaiju protocol |
| `tools/octxiliary_kaiju_vulkan_spike/` | historical sidecar spike | non-production evidence |
| `tools/sdslv_benchmark_host/experiments/` | experimental host-tool source | not Prometheus authority |
| `internal/prometheus/DevelopmentReport/artifacts/SDSL_V_ORIGINAL_SPIRV_REWRITE/` | historical/audit source and artifacts | M34/M35 original-pair evidence |
| `internal/prometheus/DevelopmentReport/SDSL_V_M*.md` | development report | historical decision evidence |
| `out/`, `dist/`, `.oct/` | temporary output | do not commit |

The duplicated names in the M34/M35 artifact directory are intentional audit
evidence, not source authorities. No current standalone experimental Prometheus
SDSL-V kernel existed to move. The flat production source directory was the
actual ambiguity removed here.

## Migration

| Old path | New path | Reason |
|---|---|---|
| `internal/prometheus/shaders/sdslv/*.sdslv` | `internal/prometheus/shaders/sdslv/production/sgemm/*.sdslv` | make production ownership visible |
| manifest, static registry, generator, CLI/native test source references | updated paths | retain one source authority |

The M36/M37 benchmark corpus and canonical artifact paths were deliberately not
renamed: their paths contribute to stable benchmark/replay identities. Historical
M34/M35 artifacts also remain beside their reports to preserve audit provenance.

## SGEMM classification and identity

Scalar, tiled, memory-conservative, scalar-plus, tile16, Reg2x2, exacttail,
flowboard, derive, SRT, B2x2, A2x4, Packed4, and FP16 retain the same shader
and implementation IDs, entry points, dispatch contracts, storage/precision
contracts, and registry role. The code-owned native manifest is the complete
machine-readable inventory. SRT/B2x2/A2x4/Packed4/FP16 historical originals
remain audit-only under the original-five rewrite artifact directory.

Tiled and memory-conservative are legacy generated production inputs, not
historical/audit-only assets, because the production registry still uses them.
No selector policy, portfolio minimization, kernel optimization, or promotion
decision occurred in M39a.

## Generated artifact boundary

Every SDSL-V native header is regenerated only by the manifest-driven generator.
Its header records source path, entry point, exact command, DXC flags, and
Vulkan target. `sdslv_workspace_check -inventory` prints source SHA-256 and the
decoded SPIR-V module SHA-256, while the canonical M36a manifest records source
and SPIR-V hashes plus compiler metadata. The checker rejects a production
manifest source outside `production/`, duplicate production IDs/sources,
registry/manifest disagreement, missing generated provenance, missing canonical
paths, and canonical hash drift.

## Stable identity and binary impact

Moving sources changed only source-path and regeneration-command comments in
generated headers. Regeneration reproduced the decoded SPIR-V words; the
workspace inventory records module hashes for review. Shader IDs, entry points,
header symbols, registry metadata, selector policy, dispatch geometry, packing,
memory placement, async/ring lifecycle, Kaiju protocol, benchmark IDs, replay
IDs, and canonical M36a artifact hashes are unchanged.

## New reactor contribution path

Begin a fused-reduction candidate at
`internal/prometheus/shaders/sdslv/experimental/reduction/`. Promote only after
validation into `production/reduction/`, add native manifest/registry metadata,
generate the native header, extend existing language/GPU test homes and the
permanent benchmark corpus, and use the reserved native runtime family file.
Place the milestone report in this directory. No fused-reduction source,
runtime behavior, or selector path was implemented in M39a.

## Validation

Passed: source regeneration (including a second identical generated-header
hash pass), manifest parity, canonical artifact plus benchmark/replay identity
checks, workspace authority verification, `go test ./internal/source`,
`./internal/diagnostic`, `./internal/sdslv/...`, `./internal/octxiliary/...`,
`./pkg/octxiliary/...`, `./internal/cli`, `./cmd/oct`,
`./internal/... ./cmd/oct`, and `./tools/build_sidecars`; Linux shell syntax;
and `git diff --check`. The affected Windows native launcher built successfully
and the Marionette suite passed 351 tests with 5 explicit skips and 0 failures.
Kaiju source/protocol was not affected, so no sidecar rebuild was required.

Convergence outcome: **SUCCESS**

Milestone state: **COMPLETE**
