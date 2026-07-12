# Prometheus M34a audit shader registry

Status: **COMPLETE — success** (2026-07-11)

## Implemented audit boundary

`native/reactor_prometheus_audit.{h,cpp}` is a Marionette-only component.  It
is listed only in `native_manifest.json`'s `native_test_sources`, never in
`production_sources`; neither `reactor_shader_registry.c` nor
`reactor_vulkan_sgemm.c` includes it.  Production selection therefore cannot
enumerate, find, or dispatch an audit candidate.

`PrometheusAuditShaderDescriptor` uses named fields for identity, SPIR-V
ownership (embedded words or a file), explicit entry point, workgroup,
`outputs_per_invocation_m/n`, packing, precision, provenance, and comparison
group.  The small `PrometheusAuditShaderRegistry` accepts explicit embedded or
file registrations and only exposes deterministic name lookup/enumeration.

The file path validates nonempty, four-byte-aligned SPIR-V, magic, requested
compute entry point, `LocalSize`, and descriptor workgroup agreement before a
future Vulkan pipeline may be created.  It produces a FNV-1a module hash,
canonical dispatch geometry, deterministic replay identity, and compact JSON.

## Canonical footprint contract

| Shader | Rows/invocation | Columns/invocation |
|---|---:|---:|
| SRT-2accum-K | 1 | 1 |
| B2x2-row-major-biased | 2 | 2 |
| A2x4-row-biased-accum8 | 2 | 4 |
| Packed4FP32 | 1 | 1 |
| FP16-storage/FP32-accum | 1 | 1 |

The audit helper computes `groupsX = ceil(M / (localX * rows))` and
`groupsY = ceil(N / (localY * columns))`.  It refuses zero/overflowing
footprints and dispatches that do not cover M×N.  The A2x4 audit test proves
M=3,N=5 maps to one 8×8 workgroup using the truthful 2×4 contract.

## A2x4 historical metadata trace

The production 2×2 value is `k_meta_reg2x2` for implementation ID 5 in
`reactor_shader_registry.c`. `reactor_sgemm_dispatch_metadata.h` consumes it
directly for geometry, so it affects behavior: it over-dispatches the actual
2×4 shader in N, subject to the shader's guarded stores.  Buffer allocation
and CPU correctness output footprint are M×N and do not consume it.  Occupancy
and judgment code consume the historical `AGGRESSIVE_4X4_ACCUM8` label for
policy/telemetry, not output sizing.  P13/PX16 benchmark rows report metadata
for generated SDSL variants but do not correct the historical A2x4 descriptor.
No production metadata was changed.

## Execution seam

The existing real execution stack is concentrated in
`reactor_vulkan_sgemm.c`: it selects/creates a pipeline, packs scalar/float4/
u32-half inputs, writes set-0 bindings 0/1/2, pushes `{m,n,k}` (12 bytes),
records the command buffer, timestamps, synchronizes, and reads C back.  Its
smallest remaining seam is an explicit *test-only* pipeline and dispatch
metadata override at the `selected_pipeline`/`dispatch_geometry` boundary.
It is threaded through the existing function rather than copied into a shadow
host stack; the execution update below records the resulting hardware run.

## Tests and evidence

`reactor_prometheus_audit_tests.cpp` covers embedded registration, production
registry isolation by construction, bad magic, bad byte length, missing entry,
workgroup mismatch, file loading, canonical A2x4 dispatch, deterministic
replay, and JSON footprint output. A standalone Marionette build ran all four
tests successfully. The normal Windows native build launcher failed before
compilation because the current shell's pre-existing MSVC environment lacks
standard-library include paths (`stdint.h` cannot be found); the audit source
itself compiled with the installed UCRT64 C++23 compiler.

## Production-safety proof

`git diff` confirms `reactor_shader_registry.c` and
`native/shaders/manifest.json` are unchanged. Audit code is test-source-only;
deleting it and its test registration cannot alter production IDs, assets,
manifests, selection policy, or packaged reactor binaries.

## Validation, timing, and replay closeout

The closeout audit lane records honest validation accounting from the real test
environment. Vulkan validation was **not requested** by the current SGEMM
runtime path; `VK_LAYER_KHRONOS_validation` was **available** on this machine,
but no validation layer names were enabled. The recorded validation warning
count is 0 and the recorded validation error count is 0 for the emitted audit
JSON. This is explicit accounting, not a claim that a validation-enabled lane
ran.

`PrometheusAuditOriginalFivePairwiseHardware` now emits deterministic JSON with
schema version 1, validation accounting, per-workload replay IDs, first
mismatch details or `null`, dispatch groups, footprints, hashes, entry points,
and min/median/max kernel timings from repeated GPU timestamp samples. The
passing replay artifact is
`out/test-artifacts/PrometheusAuditReplayProofPassAndSyntheticFailure/prometheus_m34a_passing_replay_json.txt`;
the synthetic failing replay artifact is
`out/test-artifacts/PrometheusAuditReplayProofPassAndSyntheticFailure/prometheus_m34a_failing_replay_json.txt`.

The passing replay ID is
`SRT-2accum-K:63c5d6c24176ea5d:SRT-2accum-K:ab97b49624dd0965:3x5x7:99:main:SgemmSrt2AccumK_CS:1x1`.
The failing replay intentionally perturbs the candidate result after execution
through a test-only path and deterministically reproduces the same mismatch at
row 0, column 0 with expected/original `0.57421875` and candidate
`1.07421875`.

## 2026-07-11 execution update

The narrow override is now implemented in `reactor_vulkan_sgemm.c` as the
explicit `prom_reactor_runtime_sgemm_audit_impl` API. It creates a temporary
pipeline from an audit descriptor, then reuses the existing SGEMM execution
function, replacing only selected pipeline, compute packing mode, and dispatch
metadata. No production registry or selector reads audit data.

A focused MSVC rebuild compiled the changed SGEMM object and linked
`out/prometheus/native/prometheus_audit_tests.exe`. Its five-pair, nine-shape
matrix passed original/reference, candidate/reference, and original/candidate
comparisons. Timestamp sums in ns (original/generated) were SRT 55936/35264,
B2x2 71232/43840, A2x4 71040/65792, Packed4 43904/35456, and FP16 47008/39424.
They are diagnostic timing evidence only.

For A2x4 at M=3,N=17,K=7, canonical 2×4 used one Y group and historical 2×2
used two. Both matched the reference and each other, establishing guarded,
wasteful over-dispatch for that observed case. The audit descriptor remains
canonical 2×4; production metadata was not changed.

## Final audit recommendation

All five original/generated pairs passed the required nine-workload CPU/
original/candidate matrix through the real Vulkan SGEMM path. Representative
bounded RTX 3070 audit timings at M=127,N=131,K=129 were:

| Shader | Original min/median/max ns | Generated min/median/max ns |
|---|---:|---:|
| SRT | 14368 / 14368 / 14464 | 8064 / 8256 / 8352 |
| B2x2 | 28320 / 28480 / 28544 | 10752 / 10912 / 10944 |
| A2x4 | 27008 / 27008 / 27040 | 31776 / 31776 / 31776 |
| Packed4 | 9472 / 9664 / 9728 | 7168 / 7168 / 7200 |
| FP16 | 11584 / 11616 / 12096 | 10624 / 10784 / 12480 |

These are diagnostic timing samples only. They are sufficient to reject
catastrophic regressions and to support the following final decisions:

- **REPLACE**: SRT-2accum-K
- **REPLACE**: B2x2-row-major-biased
- **REPLACE** with canonical 2×4 metadata correction in the production swap:
  A2x4-row-biased-accum8
- **REPLACE AFTER MINOR COMPILER CLEANUP**: Packed4FP32
- **REPLACE AFTER MINOR COMPILER CLEANUP**: FP16-storage/FP32-accum

Overall recommendation: in the follow-up production milestone, replace SRT,
B2x2, and A2x4 together; correct A2x4 production metadata to truthful 2×4 at
the same time; add narrow first-class vector/dot and FP16/bit-conversion
intrinsics; then rerun this audit harness and replace Packed4 and FP16.

Follow-up production replacement scope, but not executed here:

- update the production shader registry/header references and
  `native/shaders/manifest.json` for the selected SDSL-V artifacts;
- keep generated SDSL-V SPIR-V under explicit generated-artifact ownership;
- preserve explicit candidate entry-point handling or switch the generated
  entry points to `main` as a reviewed compiler/output choice, not an audit
  requirement;
- correct A2x4 metadata to canonical 2×4 when the production registry entry is
  swapped;
- rerun `go test ./internal/source`, `go test ./internal/diagnostic`,
  `go test ./internal/sdslv/...`, `go test ./cmd/oct`,
  `go test ./internal/... ./cmd/oct`, `go run ./tools/prometheus_native_manifest -check`,
  `bash -n internal/prometheus/native/build_linux.sh`, `git diff --check`, and
  the focused native audit and SGEMM lanes;
- roll back by restoring the prior registry/header/manifest references if the
  production swap shows any correctness, validation, or timing regression.

Production registry entries, production manifest content, and production
selection behavior remain unchanged in M34a. No `.sdslvbench`, `[Benchmark]`,
graphics, or M34b work was added.

M34b consumes this registry only as audit/reference evidence: its historical
headers remain test-owned while production uses the promoted SDSL-V headers.

## M35b closeout

M35b completed the remaining Packed4 and FP16 production replacements. Their
historical originals are retained under the audit artifact directory, and this
audit harness loads those files rather than production headers.
