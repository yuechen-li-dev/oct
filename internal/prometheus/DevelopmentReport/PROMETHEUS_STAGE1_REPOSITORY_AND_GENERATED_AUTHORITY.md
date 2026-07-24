# Prometheus Stage 1 — repository and generated authority hygiene

Status at authoring: repository/generated hygiene pass; no runtime ownership or
behavioral change.

## 1. Scope and starting checkpoint

Stage 1 starts at `e519a9a89e33ef7efc01ad45f111ab0ea44b02be`,
`prometheus: freeze lifecycle characterization and vocabulary`, on `main`, with
`origin/main` at the same commit. The primary authorities are:

- `internal/prometheus/DevelopmentReport/PROMETHEUS_FULL_ARCHITECTURE_AUDIT.md`;
- `internal/prometheus/DevelopmentReport/PROMETHEUS_STAGE0_CHARACTERIZATION_AND_VOCABULARY.md`;
- `internal/prometheus/DevelopmentReport/PROMETHEUS_G4_E2B_M1_REVIEWER_HANDOFF.md`.

This pass does not extract runtime ownership, rename M42–M49 symbols, change
Vulkan lifecycle behavior, change shader/package bytes, repair topology, or fix
the M46→M49 `-7406` boundary.

## 2. Baseline repository state

The required baseline commands were run before editing:

| Check | Result |
|---|---|
| `git rev-parse HEAD` | PASS — `e519a9a89e33ef7efc01ad45f111ab0ea44b02be` |
| `git status --short` | PASS — clean |
| `git branch -vv` / remote | PASS — `main` tracks `origin/main` at `e519a9a8` |
| Stage 0 commit file set | PASS — ten-file characterization commit intact |
| `git ls-files` | PASS — 4,378 tracked files at baseline |
| `go run ./tools/prometheus_stage0 -check` | PASS — source/package counts, package identity, kernel 68/69, topology, ABI/detail snapshot |
| `git diff --check` | PASS |

The baseline contained 22 tracked files under
`internal/prometheus/.octmake`, 252 tracked files below `out/`, and one tracked
Python `.pyc`. The baseline also contained tracked numerical images, oracle
descriptors, and payload documentation; those are not debris and remain in
place. No new local checkpoint, model weight, cache, binary, image, or oracle
bundle was present.

## 3. Material classification model

| Class | Repository examples | Stage 1 treatment |
|---|---|---|
| Authored production source | `internal/prometheus/native/*.c`, `*.h`; `shaders/**/*.sdslv` | preserve; not regenerated |
| Authored test source | `internal/prometheus/native/Marionette/*.cpp`; Go harnesses | preserve; behavior unchanged |
| Generated source required for builds | native SPIR-V headers; `native_sources_*.{sh,cmd}` | declare generator and consumers; preserve bytes |
| Generated metadata/descriptors | `models/zimage-turbo/resolved_*`; shader ID projection | declare generator; check provenance |
| Packaged/content-addressed shader artifacts | ignored `out/prometheus/native/SerialCanonical/shaders/manifest.json` and `objects/sha256/*` | regenerate in a temporary directory; never commit build output |
| Regenerable build output | binaries, object files, logs, benchmark stdout under `out/` | remove tracked debris; ignore recurrence |
| Cache/tool state | `.octmake`, `__pycache__`, `.pyc` | remove tracked state; ignore recurrence |
| Local payload/checkpoint/model data | external Gemma checkpoint root and reactor payloads | never add; not present in Stage 1 |
| Numerical oracle/accepted fixture | `DevelopmentReport/artifacts/**`, accepted PNGs, durable summaries | preserve; classify as evidence, not build input |
| Current durable architectural evidence | the three Stage 0/audit/handoff authorities and this report | index explicitly |
| Frozen historical development evidence | the remaining milestone reports and artifacts | do not rewrite or mass-move |
| Unclassified/disputed | explicit entries in the evidence index; unresolved generated projections | label honestly; do not bless semantics |

The machine-readable evidence index is
`internal/prometheus/DevelopmentReport/PROMETHEUS_DEVELOPMENT_EVIDENCE_INDEX.json`.
It records the baseline corpus as 596 files: 218 Markdown documents and 378
artifact files. The index intentionally uses explicit current entries and
deterministic historical/artifact path classes rather than inferring authority
from milestone filenames.

## 4. Exact tracked debris removed

The removal set was resolved from `git ls-files` and validated before deletion:

| Exact tracked class | Count | Reason | Regenerator/producer | Recurrence rule |
|---|---:|---|---|---|
| `internal/prometheus/.octmake/**` | 22 | make state, trace, and failure records; includes a 32,863-line state file and a 2,205-line trace | `oct make` / `Make.oct` using `Make.Config.StateDir` | `.octmake/`, `**/.octmake/` |
| `out/**` excluding the nine evidence paths below | 243 | binaries, libraries, benchmark runs, logs, test output, and package-adjacent build material; all regenerable | native build scripts, benchmark/test lanes, `oct sdslv package build` | existing `out/` rule |
| `tools/__pycache__/zimage_reference_capture.cpython-312.pyc` | 1 | Python interpreter bytecode | Python import/compile | `*.pyc`, `**/__pycache__/` |
| **Total** | **266** | proven generated/cache state | | |

The exact top-level `out/` deletion set was every tracked path in `out/**`
except these explicitly referenced durable evidence files:

- `out/prometheus/native/p6c/summary.json`;
- `out/prometheus/native/p6c/summary.md`;
- `out/prometheus/sgemm_lab_m4d/summary.json`;
- `out/prometheus/sgemm_lab_m4d/summary.md`;
- `out/prometheus_fft_algorithm_lab/m1/m1_fft_cases.octagon`;
- `out/prometheus_fft_algorithm_lab/m1/m1_fft_plan_traces.octagon`;
- `out/prometheus_fft_algorithm_lab/m1/m1_fft_report.md`;
- `out/prometheus_fft_algorithm_lab/m1/m1_fft_results.octagon`;
- `out/test-artifacts/P13_M5_DVT2_Rtx3070ValidationArtifact/p13_dvt2_rtx3070_validation.txt`.

The preserved paths are referenced respectively by the P6C, M4D, FFT M1, and
P13 validation reports. No durable numerical evidence or accepted fixture was
deleted. The deletion is recoverable from the parent commit until this commit
is intentionally reverted.

## 5. Ignore rules added or narrowed

`.gitignore` now explicitly covers nested Oct make state (`.octmake/` and
`**/.octmake/`) and Python bytecode (`*.pyc` and `**/__pycache__/`). The existing
`out/` rule remains the local build/test-output boundary. The authority checker
allowlists the nine preserved evidence paths so that future debris cannot be
mistaken for durable evidence.

## 6. Complete generated/generated-like inventory

The existing authority file remains
`internal/prometheus/native/native_manifest.json`; no second generated-header
manifest was introduced. Its `generated_headers` list is now deterministic and
complete for the native generated-like set: 59 paths, consisting of
`reactor_shader_ids.generated.h` plus 58 `reactor_*_spirv.h` headers.

The same manifest now has `generated_artifact_sets`. Each set records repository
paths, generator/provenance, declared input authority, clean-clone/build need,
reproducibility check, source-control/package/content-addressing status, known
consumers, deletion/regeneration result, and normative/descriptive status.

| Set | Count | Generator | Authority status |
|---|---:|---|---|
| `native-generated-headers-declared` | 32 | `go run ./cmd/oct sdslv generate-header ... --validate --require-spirv-val` | normative checked-in build inputs; provenance is partial for older headers |
| `native-generated-headers-legacy-disputed` | 27 | historical SDSL-V header/package generation; original command not recorded | legacy/disputed; bytes preserved, not semantically blessed |
| `model-lock-projections` | 3 | `go run ./tools/compiled_model_lock -write` | normative lock projections |
| `native-build-fragments` | 2 | `go run ./tools/prometheus_native_manifest -write` | normative build projections |
| `source-controlled-sdslv-shader-projections` | 48 | SDSL-V generation/package pipeline named by the shader manifest | checked-in generated projections; source/manifest authority applies |
| `external-hlsl-compiled-projections` | 4 | external HLSL compiler pipeline; historical command not recorded | legacy/disputed; exact bytes preserved |

Other generated-like material is recorded in this report and the index rather
than promoted into native build authority:

- `internal/prometheus/native/shaders/manifest.json` is authored source/package
  input with 69 source assets, 11 experimental assets, and 18 compute
  implementations.
- Source-controlled shader outputs under `internal/prometheus/shaders/**`
  include generated HLSL, SPIR-V, SPIR-V assembly, and generated headers. The
  11 experimental manifest entries explicitly carry `output`, `generated_hlsl`,
  `generated_header`, and `inspection` paths; older production projections have
  incomplete per-asset provenance and remain descriptive/legacy where the
  manifest does not name them.
- The exact tracked shader projection count is 134 files: 66 authored `.sdslv`
  sources, 18 `.spv` objects, 18 `.hlsl` files (16 generated SDSL-V HLSL plus
  two authored external-HLSL sources), 16 generated shader headers, two
  `.spvasm` disassemblies, 11 JSON inspection/descriptor files, and three
  README/notes files. The 18 source-controlled `.spv` objects are generated
  shader objects, not local Vulkan build output; they remain byte-authoritative
  only where their source/manifest or historical evidence names them.
- `internal/prometheus/models/zimage-turbo/resolved_audit_schedule.h`,
  `resolved_audit_arena.json`, and `resolved_descriptor.h` are generated model
  projections from `lock-tagon.octagon`, checked by the compiled-model-lock
  tool.
- `internal/prometheus/native/reactor_api.h` and related ABI declarations are
  authored ABI source, not generated projections. The executable ABI/detail
  snapshot in `native/Marionette/stage0_characterization_tests.cpp` is the
  authority for sizes, offsets, enums, and `PROM_M46_DETAIL_STALE_WEIGHT_GENERATION`;
  Stage 1 does not alter that declaration surface.
- Ignored package output is generated under
  `out/prometheus/native/SerialCanonical/shaders/`. Package artifacts are
  content-addressed by `objects/sha256/<digest>`; the current Stage 0 witness
  is 69 kernels, 69 variants, 68 package artifacts, 69 provenance records,
  and 74 observed local objects, including six extra local objects. The six
  extras are not blessed as package artifacts.
- Development-report `artifacts/**` are generated or measured durable
  evidence, not clean-clone build inputs. They are indexed and retained.

## 7. The 32-versus-59 reconciliation

The audit reported 32 entries in
`native_manifest.json.generated_headers` versus 59 tracked generated-like
native headers. Direct `git ls-files` evidence resolves the discrepancy:

1. The 32 pre-existing entries are genuine checked-in SPIR-V build inputs.
2. The 27 additional paths are also generated-like headers: the extra SDSL-V
   SGEMM/reduction projections, shader-ID package projection, Z-Image
   experimental/control/topology projections, and package-only/native-test
   projections. They are not authored C headers merely named like outputs.
3. Their original generation commands are not recorded uniformly. They are
   therefore declared as `legacy_disputed`, not promoted to semantic authority.
4. All 59 are now listed in `generated_headers`, and all 59 are covered by one
  `generated_artifact_sets` declaration. Missing paths, duplicate paths, and
   undeclared native `spirv.h`/`generated.h` files fail
   `go run ./tools/prometheus_native_manifest -check`.

The reconciliation changes declaration coverage only. It does not regenerate
or rewrite any header, package artifact, shader object, static registry entry,
kernel identity, or runtime consumer.

## 8. Generator and consumer map

| Authority/input | Generator | Checked-in output | Consumers |
|---|---|---|---|
| SDSL-V shader source and recorded package manifest | `oct sdslv generate-header`; package build | native SPIR-V headers; shader package output | native C and Marionette tests; package builder |
| `native_manifest.json` | `go run ./tools/prometheus_native_manifest -write` | `native_sources_windows.cmd`, `native_sources_linux.sh` | Windows/Linux native builds |
| `lock-tagon.octagon` + `audit_stages.oct` | `go run ./tools/compiled_model_lock -write` | resolved descriptor/schedule/arena | Z-Image model-block native code and bridge |
| shader package manifest | `go run ./cmd/oct sdslv package build ...` | ignored package manifest, generated IDs, SHA-256 objects | Stage 0/package loader and native shader package |
| test/benchmark commands | native/Go test lanes | `out/**` outputs | temporary validation only; not source authority |

## 9. Manifest/lock/generated/registry/package authority map

The accepted authority flow remains:

`native/shaders/manifest.json` → package build and generated projections →
native shader package/registry → runtime ABI.

`lock-tagon.octagon` is authoritative for the accepted model-lock identity and
the compiled-model-lock projections. `native_manifest.json` is authoritative
for native source membership and generated-header declaration coverage.
Package identity remains `prometheus.core@1`; package membership, kernel 68
(`kernel-68-default`), and kernel 69 (`kernel-69-default`) remain protected by
Stage 0. The static registry is not declared complete merely because its
projection is indexed.

Intentionally unresolved after Stage 1:

- generated shader ID header/package/static-registry count differences;
- package-only identities 52–69 absent from the static registry projection;
- the six extra local shader objects;
- the 29 repeated `MainTransformer1` successors in
  `lock-tagon.octagon`;
- older generated headers whose exact original command is not recorded;
- any semantic dispute between manifest, lock, generated projection, static
  registry, and package beyond the currently accepted Stage 0 checks.

## 10. Normative checks versus descriptive snapshots

Normative checks remain package identity and source/package membership, kernel
68/69 variant IDs, object sufficiency, lock identity/projection validity,
generated declaration completeness, ABI export/detail snapshots, and the
absence of tracked Prometheus build/cache/payload debris. Descriptive snapshots
remain the generated header/package/static-registry differences, repeated
topology, package-only IDs, extra local objects, and the M49 early-return
generation fields. Stage 1 does not convert a descriptive snapshot into a
semantic contract.

## 11. Canonical authority-check command

The single developer entry point is:

```powershell
powershell -NoProfile -File .\tools\prometheus_authority.ps1
```

It orchestrates the existing focused authorities rather than reimplementing
their package or lock parsing:

1. `go run ./tools/prometheus_native_manifest -check`;
2. `go run ./tools/compiled_model_lock -check`;
3. tracked debris/payload-state scan with the explicit durable-evidence
   allowlist;
4. report-index path validation;
5. a temporary clean-clone shader package build followed by
   `go run ./tools/prometheus_stage0 -check -package-dir <temporary package>`.

It emits deterministic JSON with schema
`prometheus.repository-authority.v1` and exits nonzero on normative failure.
The temporary package is removed after the check; the working tree is not used
as package-output authority for that clean-clone check.

## 12. Development-report classification/index result

`PROMETHEUS_DEVELOPMENT_EVIDENCE_INDEX.json` provides:

- current architectural authorities: the full audit, Stage 0 report, G4 handoff,
  and this Stage 1 report;
- current reviewer handoff: G4 E2B M1, with Stage 0 identified as its
  implementation-advice supersession;
- current validation/characterization: the full audit and executable Stage 0
  characterization;
- historical milestone and superseded handoff examples with path, era, status,
  and superseding authority;
- generated/measured artifact scope, including the 378-file report-artifact
  corpus and nine retained out-of-tree evidence files;
- explicit unknown entries for future-direction/roadmap documents whose current
  applicability is not inferred from filenames.

No report corpus was mass-moved. The remaining 215 non-authority Markdown
documents remain historical by the index's path rule pending a future,
independently reviewable archive map.

## 13. Repository policy

- Authored source belongs in the existing native, shader, model, Go, and test
  locations; it is not generated by Stage 1.
- Checked-in generated build inputs must be listed in
  `native_manifest.json.generated_headers` or a generated artifact set with a
  generator and input authority. If provenance is missing, the entry is
  legacy/disputed rather than silently blessed.
- Generated native build fragments and lock projections may remain checked in
  because clean-clone builds consume them; their focused `-check` commands must
  pass.
- Generated shader package manifests, generated IDs used only by package
  staging, compiled objects, binaries, logs, benchmark output, and caches must
  not be committed. Package objects are identified by SHA-256 filename and
  package manifest provenance, not by directory name alone.
- Local payloads, checkpoints, model weights, machine-specific absolute paths,
  and personal environment configuration must never be committed.
- Numerical fixtures, accepted oracles, and durable measured evidence remain
  source-controlled only when intentionally reviewed and indexed; their age or
  output-shaped path is not enough reason to delete them.
- Current evidence is limited to the indexed authority documents and explicit
  Stage 0 witnesses. Historical evidence remains available but is not current
  semantic authority.
- A clean clone must contain authored source, checked-in generated build inputs,
  lock projections, manifests, and durable evidence. It must generate temporary
  package/build output before native execution; it must not require a checked-in
  `out/` or `.octmake/` directory.
- Contributors run the complete repository-authority check with
  `tools/prometheus_authority.ps1`.

## 14. Files deliberately left in place

All production C/C++/Go/Oct source, all existing generated headers and shader
objects, all model-lock projections, all Stage 0 reports/tests, all numerical
oracles and accepted fixtures, all report artifacts, and the nine referenced
durable out-of-tree evidence files were deliberately left in place. The
tracked Godot cache outside the Prometheus scope was not touched.

## 15. Disputed facts deliberately not repaired

Stage 1 does not repair `MainTransformer1` successor projection, M46→M49
`-7406`, static-registry/package projections, package-only identities, generated
header byte drift, shader package membership, kernel count, ABI/detail values,
allocation ceilings, Vulkan lifecycle, or any Gemma/Z-Image execution path.

## 16. Validation performed

| Lane | Result | Notes |
|---|---|---|
| native manifest/generated inventory | PASS | 59 headers, six artifact sets, no missing/duplicate paths |
| compiled model lock | PASS | `71ef202b4e34b562bd0d8526d1e0c674640cbba02fb7c484d8dadf981c8b226e`; no projection rewrite |
| canonical authority check | PASS | temporary package output only; JSON schema `prometheus.repository-authority.v1` |
| Stage 0 check | PASS | local witness retained 74 objects/6 extras; temporary package witness 68/0; baseline descriptive counts remain visible |
| required-live wrapper self-test | PASS | synthetic PASS/SKIP/zero-work detection |
| focused Prometheus Go tests | PASS with expected SKIP | Prometheus, Gemma, shaderpackage, Z-Image, native-manifest, and compiled-lock packages; payload-backed tests skipped |
| Windows native build | PASS | `internal/prometheus/native/build_windows.cmd` completed; compiler warnings only |
| native ABI/detail snapshot | PASS | `PrometheusStage0GemmaABIAndDetailSnapshot` passed |
| shader-package validation | PASS | shaderpackage Go tests plus temporary package authority check |
| Z-Image unit regression | PASS with expected SKIP | unit contracts passed; local cache/payload tests skipped because `OCT_EVT2_CACHE` is unset |
| clean-clone regeneration | PASS | temporary package build and Stage 0 check; no working-tree package output used |
| fresh Gemma/canonical Z-Image smoke/allocation/teardown | NOT RUN | required payload/reactor are not configured |
| Linux live Vulkan | NOT RUN | repository does not claim live Linux validation |
| `git diff --check` | PASS | |

Authoritative generated bytes are unchanged: `git diff --name-only` reports no
modified generated header, shader, lock projection, package source, or native
build-fragment byte. The temporary package build produced the accepted package
identity and counts without writing to the worktree. Stage 1 changes only
declarations, checks, ignores, reports, and proven tracked debris.

## 17. Stage 0 live status

The three required-live lanes remain `SKIP / NOT RUN` unless a validated
`OCT_PROMETHEUS_REACTOR` and external `G4E2B_CHECKPOINT_ROOT` are configured:

- `TestGemma4E2BM1FreshSessionQFirstAuthority`;
- `TestGemma4E2BM1FreshSessionKFirstAuthority`;
- `TestGemma4E2BM1SameSession7406Characterization`.

The required-live wrapper treats environmental skip and zero work as failure;
Stage 1 does not weaken that behavior.

## 18. Remaining risks and unknowns

The 27 legacy/disputed generated headers still lack uniformly recorded original
commands and independent delete/regenerate byte proofs. The package/static
registry projection remains intentionally partial. The report corpus has a
large historical tail and only a compact explicit index; bulk archival is not
part of Stage 1. Local payload-backed Gemma closure, Windows hardware
validation, and live Linux Vulkan remain environment-dependent.

## 19. Exact rollback boundary

Revert this single hygiene commit to restore the 266 tracked debris files, the
old 32-entry generated-header declaration, the prior ignore rules, and the
Stage 1 authority/index files. No production runtime source, public ABI, shader
source/object, package identity, or durable evidence is part of the rollback
boundary.

## 20. Revised Stage 2 boundary

The original Stage 1 sequencing note deferred Stage 2 until the three live
witnesses were closed. The revised project decision permits the mechanical
Stage 2 ABI/vocabulary pass while the external Gemma checkpoint remains
unavailable. That pass must preserve the Stage 0 live boundary and must not
repair `-7406`, generated topology, registry/package discrepancies, or
generated-header provenance ambiguity. Stage 3 common Vulkan runtime/device
ownership extraction remains later still and is not authorized by Stage 2.
