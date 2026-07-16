# SDSL-V workspace guide

This is the repository ownership guide. Language truth remains in
`docs/SDSL_V_LANGUAGE_SPEC.md` and `Language/reference/`.

## Authority map

| Path | Role | Production dependency |
|---|---|---|
| `internal/sdslv/` | compiler, toolchain, fixtures, benchmark execution | compiler/tooling only |
| `internal/sdslv/testdata/language/` | `.sdslvvalid` / `.sdslvinvalid` contracts | test tooling only |
| `examples/SDSL-V/` | examples, `.sdslvtest`, permanent benchmarks | no |
| `examples/SDSL-V/M36a/` | permanent benchmark corpus and canonical artifacts | benchmark/audit tooling only |
| `internal/prometheus/shaders/sdslv/production/` | sole Prometheus SDSL-V source authority | yes |
| `internal/prometheus/shaders/sdslv/experimental/` | future candidates; policy only today | no |
| `internal/prometheus/shaders/sdslv/historical/` | policy pointer to audit evidence | no |
| `internal/prometheus/native/` | registry, headers, runtime, tests, generator | yes |
| `internal/prometheus/DevelopmentReport/artifacts/SDSL_V_ORIGINAL_SPIRV_REWRITE/` | M34/M35 audit-only evidence | no |
| `pkg/octxiliary/kaijuvulkan/` | Kaiju protocol/cross-runtime integration | sidecar only |
| `tools/sdslv_benchmark_host/` | benchmark host and explicit host experiments | benchmark tooling only |
| `internal/prometheus/DevelopmentReport/` | development reports | no |
| `out/`, `dist/`, `.oct/`, `internal/sdslv/test/out/` | transient local/test output | never |

Production means registry-consumed and stable. Experimental sources may be
compiled, tested, and benchmarked but must not occur in the native manifest,
registry, or native build inputs. Historical evidence is audit-only. There are
no standalone experimental Prometheus SDSL-V kernels today; M34/M35 originals
remain beside their reports, avoiding a duplicate authority.

The tiled and memory-conservative registry entries retain `historical generated`
provenance but remain production build inputs because the registry consumes them.

## Production SGEMM portfolio

`internal/prometheus/native/shaders/manifest.json` owns stable IDs, source,
header symbol, entry point, workgroup, and footprint. Selector policy is
unchanged; registry membership is not universal selection.

| Kernel | ID | Entry point | Source authority | Contract |
|---|---:|---|---|---|
| Scalar | 1 | `main` | embedded `reactor_vulkan_sgemm.c` | baseline, 8x8 / 1x1 |
| Tiled | 2 | `main` | retained generated header | legacy production artifact, 8x8 / 1x1 |
| Memory-conservative | 3 | `main` | retained generated header | legacy production artifact, 8x8 / 1x1 |
| Scalar-plus | 4 | `SgemmScalarBaselinePlus8x8_CS` | `production/sgemm/sgemm_scalar_baseline_plus.sdslv` | FP32, 8x8 / 1x1 |
| Tile16 | 5 | `SgemmTile16x16SharedFp32_CS` | `production/sgemm/sgemm_tile16x16_shared_fp32.sdslv` | FP32 shared tile, 16x16 / 1x1 |
| Reg2x2 | 6 | `SgemmReg2x2Tile16x16Fp32Kernel_CS` | `production/sgemm/sgemm_reg2x2_tile16x16_fp32.sdslv` | FP32, 8x8 / 2x2 |
| Exacttail | 7 | `SgemmReg2x2Tile16x16ExactTailFp32Kernel_CS` | `production/sgemm/sgemm_reg2x2_tile16x16_exacttail_fp32.sdslv` | FP32, 8x8 / 2x2 |
| Flowboard | 8 | `SgemmReg2x2Tile16x16FlowBoardFp32Kernel_CS` | `production/sgemm/sgemm_reg2x2_tile16x16_flowboard_fp32.sdslv` | FP32, 8x8 / 2x2 |
| Derive | 9 | `SgemmReg2x2Tile16x16DeriveFp32Kernel_CS` | `production/sgemm/sgemm_reg2x2_tile16x16_derive_fp32.sdslv` | FP32, 8x8 / 2x2 |
| SRT | 10 | `SgemmSrt2AccumK_CS` | `production/sgemm/sgemm_srt_2accum_k.sdslv` | FP32, 8x8 / 1x1 |
| B2x2 | 11 | `SgemmB2x2_CS` | `production/sgemm/sgemm_b2x2_row_major_biased.sdslv` | FP32, 8x8 / 2x2 |
| A2x4 | 12 | `SgemmA2x4_CS` | `production/sgemm/sgemm_a2x4_row_biased_accum8.sdslv` | FP32, 8x8 / 2x4 |
| Packed4 | 13 | `SgemmPacked4_CS` | `production/sgemm/sgemm_packed4_fp32.sdslv` | packed `float4`, 8x8 / 1x1 |
| FP16 | 14 | `SgemmFp16StorageFp32Accum_CS` | `production/sgemm/sgemm_fp16_storage_fp32_accum.sdslv` | packed halves, FP32 accum, 8x8 / 1x1 |

ID 15 is the generated inline-HLSL proof asset; it is non-dispatchable and
non-selector-eligible.

## Artifact and validation workflow

Generated headers name their source, entry point, generator command, compiler
flags, and Vulkan target. The check validates authority, unique IDs, header
provenance, canonical hashes, and required documents. `-inventory` prints
source and decoded SPIR-V module SHA-256 values.

```powershell
powershell -ExecutionPolicy Bypass -File internal/prometheus/native/generate_sdslv_shaders.ps1
go run ./tools/sdslv_workspace_check -inventory
git diff --check
```

The generator writes scratch output to `out/sdslv/`. Canonical M36a artifacts
are separate and retain source/SPIR-V hashes, DXC identity/arguments, target,
entry point, resources, and benchmark IDs:

```powershell
go run ./tools/generate_m36b_canonical
go run ./tools/sdslv_workspace_check
```

`.sdslvvalid` accepts language behavior; `.sdslvinvalid` asserts diagnostics;
`.sdslvtest` is GPU correctness; `.sdslvbench` is performance only. Benchmark
and replay identity derives from source identity, so M36/M37 paths stay put.

## Contribution path

Add a candidate at `experimental/<family>/`; use the existing language corpus
and examples for contracts and only add a permanent benchmark when the identity
is intended to persist. Do not edit the production manifest or registry.

Promotion moves the one source authority to `production/<family>/`, then adds
stable ID, generated header metadata, and entry point to the manifest; updates
the registry; regenerates; extends correctness coverage; and runs all checks.
Retirement is allowed only after production unlinking and must preserve ID policy.

The next fused-reduction work begins at:

```text
internal/prometheus/shaders/sdslv/experimental/reduction/
```

Eventual production source is `production/reduction/`; metadata belongs in the
native manifest, generated headers in `internal/prometheus/native/`, runtime
in the reserved `reactor_vulkan_fused_reduction.c`, benchmarks/tests in the
existing permanent homes, and history in `DevelopmentReport/`. No reduction
implementation is introduced by this layout milestone.
