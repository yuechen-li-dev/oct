# Prometheus Stage 2 — ABI and lifecycle vocabulary consolidation

Status: mechanical/static consolidation completed; live Gemma closure remains deferred.

## 1. Starting checkpoint and scope

Stage 2 began at commit `1044855f5d45e391b7e3f4e95c6018728087fee0` with a clean
worktree. The scope was limited to canonical ABI projection, ABI-preservation
checks, isolated internal M42–M48 semantic naming, and current documentation.

No Vulkan ownership extraction, runtime owner, session/slot/lease abstraction,
model/compiler feature, shader/package change, allocation-policy change,
dispatch change, synchronization change, weight-binding change, residency
change, or teardown change was authorized or performed.

## 2. Revised deferred-live decision

The external Gemma checkpoint is unavailable. Stage 0 therefore remains an
honest incomplete live closure. The required-live skip detector passes its
self-test, but the three payload-dependent witnesses were not rerun:

- fresh-session Q-first;
- fresh-session K-first;
- same-session M46-success followed by M49 required-weight rejection.

The exact prior characterization remains unchanged: the M49 admission boundary
returns `PROM_M46_DETAIL_STALE_WEIGHT_GENERATION` (`-7406`) before positional
dispatch. Stage 2 does not fix, move, suppress, bypass, or reinterpret it.
No live Gemma equivalence claim is made. Fresh allocation/residency/teardown
and checkpoint-dependent canonical Z-Image validation were also not performed.
When the payload is restored, the existing witnesses must run against this
reorganized checkpoint. A live failure is a regression investigation, not an
automatic attribution to the historical checkpoint.

## 3. Complete ABI inventory

### Canonical declaration and projection inventory

| Item | Canonical declaration/definition | Projection or consumers | Authority and result |
|---|---|---|---|
| 84 exported native functions | `internal/prometheus/native/reactor_api.h` | `reactor_api.c`, Windows DLL, Linux shared library, Go dynamic bridge | Header declarations; all 84 names remain present in `reactor_api.c` and the built Windows DLL |
| 69 public `Prometheus*` struct definitions | `reactor_api.h` | native implementation, Marionette, cgo bridge aliases | Header field order/types are canonical; Stage 0 witness and compiler validate selected layouts |
| Public aliases for model/session/request types | `reactor_api.h` | native implementation and tests | Existing aliases retained; no second layout declaration introduced |
| Public enum/flag/status/detail constants | `reactor_api.h` | native implementation, Go status projection, tests | Numeric declarations remain in the public header; ABI version is now `PROM_REACTOR_ABI_V1 = 1u` there |
| 169 public detail declarations / 150 distinct public values | `reactor_api.h` | native API and host status handling | Values are unchanged; collisions remain descriptive compatibility facts |
| Internal M42–M49 request/result/plan structs and enums | `reactor_vulkan.h` | `reactor_vulkan_transformer.c`, attention tests, runtime-internal headers | Internal native authority; seven isolated plan/request type spellings now have semantic canonical names with direct historical aliases |
| 133 internal M42–M49 detail declarations / 133 distinct values | `reactor_vulkan.h` | transformer implementation and native tests | Retained; no central remapping was introduced because it could alter diagnostic behavior |
| Native API definitions and veneers | `reactor_api.c` | linker/exported DLL | Definition coverage check confirms every header export has a source definition |
| Dynamic-loader function-pointer table | `native/prometheus_bridge_abi.h` | `bridge_dlopen_windows.go`, `bridge_dlopen_linux.go` | Shared authored projection; function pointers use public request/result types and platform calling convention |
| Go symbol-name and callback projection | `bridge.go` | platform loaders and Go tests | Host-language projection retained; authority checks compare expected ABI version and exported symbol names |
| Generated native headers | `native_manifest.json` and listed generated headers | native build and shader registry | Stage 1 generated inventory remains authoritative; zero generated files changed |
| Shader/model/package projections | shader manifest, lock, generated descriptors, temporary package build | package loader and native registry | Stage 0/package authority remains green; no bytes or identities changed |
| Test-only ABI mirrors | former cgo `oct_prom_*` structs; Stage 0 C++ assertions | Go bridge and Marionette | Former handwritten cgo layouts removed; C++ assertions now validate canonical aliases and signatures |
| Documentation declarations | Stage 0, Stage 1, full audit, handoff, and this report | reviewers and future stages | Disagreements remain classified as descriptive/disputed rather than promoted to authority |

### Exported symbol inventory

The canonical command extracts and checks this complete 84-name set from
`reactor_api.h`; the built Windows DLL has the same 84-name set.

```text
prometheus_reactor_abi_version
prometheus_reactor_runtime_create
prometheus_reactor_runtime_destroy
prometheus_reactor_runtime_probe
prometheus_reactor_runtime_vulkan_device_diagnostics
prometheus_reactor_runtime_ray_query_triangle_scene_create
prometheus_reactor_runtime_ray_query_triangle_scene_probe
prometheus_reactor_runtime_ray_query_triangle_scene_destroy
prometheus_reactor_runtime_ray_query_scene_create
prometheus_reactor_runtime_ray_query_scene_trace
prometheus_reactor_runtime_ray_query_scene_destroy
prometheus_reactor_runtime_ray_query_scene_create_empty
prometheus_ray_query_runtime_create
prometheus_reactor_runtime_ray_query_scene_add_triangles
prometheus_reactor_runtime_ray_query_scene_add_spheres
prometheus_reactor_runtime_ray_query_scene_commit
prometheus_reactor_runtime_ray_query_scene_submit_batch
prometheus_reactor_runtime_sgemm
prometheus_reactor_runtime_sgemm_benchmark_variant
prometheus_reactor_runtime_sgemm_resident_benchmark
prometheus_reactor_runtime_sgemm_batch
prometheus_reactor_runtime_sgemm_submit_async
prometheus_reactor_runtime_sgemm_query_async
prometheus_reactor_runtime_sgemm_async_diagnostics
prometheus_reactor_runtime_sgemm_consume_async
prometheus_reactor_runtime_sgemm_abandon_async
prometheus_reactor_runtime_sgemm_policy_diagnostics
prometheus_reactor_runtime_sgemm_policy_diagnostics_sized
prometheus_reactor_runtime_p15_test_seed_matured_reservation
prometheus_reactor_runtime_sgemm_batch_diagnostics
prometheus_reactor_runtime_fft
prometheus_reactor_runtime_fft_benchmark_variant
prometheus_reactor_runtime_fft_diagnostics
prometheus_reactor_runtime_fft_diagnostics_sized
prometheus_reactor_reduction_plan
prometheus_reactor_runtime_reduction
prometheus_reactor_runtime_reduction_diagnostics
prometheus_reactor_runtime_reduction_benchmark
prometheus_reactor_runtime_row_wise_softmax
prometheus_reactor_runtime_gemma4e2b_m1_input_rmsnorm
prometheus_reactor_runtime_gemma4e2b_m1_projection_activation_rmsnorm
prometheus_reactor_runtime_gemma4e2b_m1_head_rmsnorm
prometheus_reactor_runtime_gemma4e2b_m1_rope
prometheus_reactor_runtime_gemma4e2b_m1_head_rmsnorm_rope
prometheus_reactor_runtime_gemma4e2b_m1_attention_scores
prometheus_reactor_runtime_model_block_create
prometheus_reactor_runtime_model_block_upload_weights
prometheus_reactor_runtime_model_block_execute
prometheus_reactor_runtime_model_block_execute_m1b
prometheus_reactor_runtime_model_block_execute_m1c
prometheus_reactor_runtime_model_block_execute_m1d
prometheus_reactor_runtime_noise_refiner0_execute
prometheus_reactor_runtime_noise_refiner1_execute
prometheus_reactor_runtime_noise_refiner_rebind
prometheus_reactor_runtime_noise_refiner_execute_resident
prometheus_reactor_runtime_noise_refiner_execute_static_audit
prometheus_reactor_runtime_noise_refiner_audit_final
prometheus_reactor_runtime_context_refiner_create
prometheus_reactor_runtime_context_refiner_rebind
prometheus_reactor_runtime_context_refiner0_execute
prometheus_reactor_runtime_context_refiner_execute_resident
prometheus_reactor_runtime_context_refiner_execute_static_audit
prometheus_reactor_runtime_context_refiner_audit_final
prometheus_reactor_runtime_main_transformer_create
prometheus_reactor_runtime_main_transformer_rebind
prometheus_reactor_runtime_main_transformer_execute
prometheus_reactor_runtime_main_transformer_execute_static_audit
prometheus_reactor_runtime_main_transformer_audit_final
prometheus_reactor_runtime_model_block_get_evidence
prometheus_reactor_runtime_model_block_destroy
prometheus_reactor_runtime_compiled_model_session_create
prometheus_reactor_runtime_compiled_model_session_capture_completed
prometheus_reactor_runtime_compiled_model_session_compose_joint
prometheus_reactor_runtime_compiled_model_session_get_evidence
prometheus_reactor_runtime_compiled_model_session_set_main_attention_route
prometheus_reactor_runtime_compiled_model_session_destroy
prometheus_reactor_runtime_compiled_model_owner_create
prometheus_reactor_runtime_compiled_model_retarget
prometheus_reactor_runtime_compiled_model_prefetch
prometheus_reactor_runtime_compiled_model_activate_prefetch
prometheus_reactor_runtime_compiled_model_evaluation_reset
prometheus_runtime_create
prometheus_runtime_destroy
prometheus_runtime_probe
```

### Public struct inventory

The 69 definitions are in `reactor_api.h`, grouped by the following complete
declaration families: `PrometheusComplex32`; row-wise softmax and reduction
requests/plans/results/diagnostics/benchmarks; model-block, refiner,
transformer, compiled-session, and evidence requests/descriptors; FFT and
SGEMM batch/diagnostic types; `PrometheusCaps` and Vulkan diagnostics; all ray
query geometry/request/hit/batch/config types; `PrometheusReactorConfig` and
`PrometheusAsyncStatus`; the eight Gemma request/result types; and SGEMM async,
resident-benchmark, and policy diagnostics. The exact names are emitted by
`go run ./tools/prometheus_stage0 -check` under `abi.public_struct_names`;
the same output contains `abi.exported_symbol_names` for functions.

### Host and platform projections

`bridge.go` contains 15 dynamic symbol strings, 13 Go callback function types
(one is a direct head-RMSNorm alias),
and four Go result projection structs. The cgo bridge contains nine ABI type
aliases and 13 dynamic function-pointer declarations in the new shared header.
The Linux and Windows loader files retain only platform loader mechanics and
call wrappers. No Go struct layout is used as native ABI authority.

## 4. Canonical declaration map

| Authority | Canonical role | Derived/retained projections |
|---|---|---|
| `native/reactor_api.h` | Public native ABI: exports, public structs, public enums/flags/constants, ABI version | `reactor_api.c`, cgo aliases, Go loader checks, Marionette compile-time assertions |
| `native/reactor_vulkan.h` | Internal transformer declarations and M42–M49 detail families | transformer C implementation, native tests, direct historical aliases |
| `native/reactor_api.c` | Exported function definitions/veneer behavior | Windows DLL and Linux shared library linker output |
| `native/prometheus_bridge_abi.h` | One dynamic-loader projection of public types and calling-convention function pointers | both platform cgo files |
| `native/native_manifest.json` | Generated native file inventory | generated source fragments and build scripts |
| shader manifest/package build and model lock | Shader/package/model generated facts | generated headers, package objects, native registry, model descriptors |
| Stage 0 C++ witness | Layout/signature/value preservation evidence | no declaration authority is duplicated |

## 5. Duplicates removed or derived

- Removed both copies of nine handwritten cgo struct layouts (`oct_prom_caps`,
  config, async status, and Gemma request/result types).
- Derived those names as direct typedef aliases to `reactor_api.h` types.
- Derived loader function-pointer parameter types from public request/result
  types and kept Windows `__cdecl` in the shared projection.
- Moved the C ABI version value from a private `reactor_api.c` define to the
  public canonical constant `PROM_REACTOR_ABI_V1 = 1u`; the Go value remains a
  checked host projection for non-cgo tests.
- Renamed seven isolated internal type declarations to semantic canonical names
  and changed their native consumers mechanically. Each old M42–M48 type name
  remains as exactly one direct typedef alias.
- Extended the existing `prometheus_stage0` authority path; no competing ABI
  checker was created.

## 6. Declarations deliberately retained

- All 84 exported names and signatures.
- All public and internal numeric values, including `-7406`.
- M42–M49 function names and M49 request/result names, because compatibility
  visibility and unresolved coupling make a wholesale rename unsafe.
- The Go callback/result projection, because Go cannot import C declarations in
  pure-Go test builds without changing the loader boundary.
- Internal detail-code families and their historical collisions; central
  remapping would change diagnostic behavior and is outside a mechanical pass.
- Generated header, shader, model-lock, package, static-registry, package-only,
  and topology discrepancies from Stage 0/1.
- The repeated `MainTransformer1` successor projection and all descriptive
  `-7406` evidence.

## 7. Historical-to-semantic vocabulary map

| Historical spelling | Semantic vocabulary | Stage 2 action |
|---|---|---|
| M40b | resident matrix multiply / packed SGEMM | retained; outside this pass |
| M42 | single-head attention preparation/execution | `prom_single_head_attention_request`; direct `prom_m42_attention_request` alias |
| M43 | grouped multi-head attention aggregation | `prom_grouped_attention_plan`; direct `prom_m43_attention_plan` alias |
| M44 | attention output projection | `prom_attention_output_projection_plan`; direct historical alias |
| M45 | attention residual combine | `prom_attention_residual_plan`; direct historical alias |
| M46 | RMSNorm plan; weight preparation and binding validation | `prom_rmsnorm_plan`; M46 functions/details retained |
| M47 | gated feed-forward residual execution | `prom_gated_feed_forward_plan`; direct historical alias |
| M48 | transformer stack activation continuation | `prom_transformer_stack_plan`; direct historical alias |
| M49a | required-weight admission plus positional continuation | retained; unresolved boundary, no rename |
| M49b | execution-route policy/telemetry | retained; owner decision deferred |
| `generation` | qualify as content generation, binding generation, slot epoch, or snapshot generation | documentation and selected internal naming only; no integer collapse |
| `slot` | physical submission/storage location | retained; no lease/owner extraction |
| `pin` | retained-role validity/admissibility bit | retained; not renamed to lease |
| `lease` | temporary acquisition right | retained only where already explicit in SGEMM |
| `completion` / `readback` | completion observation / caller-visible readback | current docs and checks use the distinction |

The source rename is intentionally bounded. It does not imply cleaner runtime
ownership or new semantics. M46-success → M49 `-7406` remains the exact prior
unresolved characterization.

## 8. ABI assertions and checking authority

The canonical command remains:

```powershell
powershell -NoProfile -File .\tools\prometheus_authority.ps1
```

Its existing Stage 0 invocation now additionally verifies:

- 84 exported declarations and the complete symbol-name inventory;
- canonical function-signature SHA-256
  `89053790ac5a18d29a21141527e017efc2faa03932d3adc2307891fdb8da0262`;
- source-definition coverage for all exported names;
- 69 public struct declarations;
- 169 public and 133 internal detail declarations;
- canonical ABI version value and its checked Go projection;
- two platform includes of the shared bridge projection;
- nine public-type aliases and zero duplicate `oct_prom_*` struct declarations;
- package identity, manifest membership, kernel 68/69 entry points, lock
  topology characterization, and generated/native inventory.

The native Stage 0 witness adds compile-time checks for config/status sizes and
offsets, Gemma sizes/offsets, public function-pointer compatibility, ABI
version, and semantic-to-historical type identity. The Windows DLL was also
checked with `dumpbin /exports` against the 84-name header inventory.

## 9. Exact production files changed

| File | Mechanical change |
|---|---|
| `internal/prometheus/native/prometheus_bridge_abi.h` | new authored shared loader projection; aliases only |
| `internal/prometheus/bridge_dlopen_linux.go` | removed duplicated cgo declarations; includes shared projection |
| `internal/prometheus/bridge_dlopen_windows.go` | same; Windows loader mechanics unchanged |
| `internal/prometheus/native/reactor_api.h` | centralized ABI version constant only |
| `internal/prometheus/native/reactor_api.c` | consumes canonical ABI version constant only |
| `internal/prometheus/native/reactor_vulkan.h` | semantic canonical names for seven isolated plan/request types plus direct historical aliases |
| `internal/prometheus/native/reactor_vulkan_transformer.c` | mechanical use of semantic type names |

Tests/tools/docs changed separately are listed in the final handoff. No file was
deleted.

## 10. Proof that exported names and values are unchanged

- Header inventory: 84 exports before and after; canonical signature digest is
  unchanged from the starting checkpoint.
- `reactor_api.c` contains a definition for every header export.
- Windows `dumpbin /exports`: header 84, DLL 84, missing 0, extra 0.
- Numeric comparison of `reactor_api.h`: all 545 pre-existing `PROM_*`
  assignments retain their values; one canonical `PROM_REACTOR_ABI_V1 = 1u`
  declaration was added for the value previously private to `reactor_api.c`.
- Numeric comparison of `reactor_vulkan.h`: 505 assignments before and after;
  changed 0, missing 0, added 0.
- Public detail declarations remain 169 declarations mapping to 150 values;
  internal M42–M49 detail declarations remain 133 distinct values.
- `PROM_REACTOR_ABI_V1` remains 1; `PROM_M46_DETAIL_STALE_WEIGHT_GENERATION`
  remains `-7406`.

The ABI-version enum addition is a compile-time declaration only; it adds no
exported symbol and does not alter any struct, calling convention, or binary
layout.

## 11. Proof that layouts and calling conventions are unchanged

`reactor_api.h` public field declarations were not rewritten. The C++ witness
passes fixed sizes of 112, 160, 104, 96, 144, 144, 208, and 136 bytes for the
Gemma request/result types and checks the established offsets 144, 32, 72,
120, and 128. It additionally checks config/status offsets, shared aliases,
and function-pointer types against `decltype(&public_export)`.

The seven semantic internal names are typedef-identical to their historical
names, so field order, size, alignment, and offsets are compiler-enforced. The
shared bridge uses the same pointer parameter types as the public prototypes;
Windows retains `__cdecl`, and Linux retains the platform default C ABI.

## 12. Generated, shader, and package preservation

The post-change diff contains no generated header, SPIR-V, HLSL, shader source,
model lock/projection, package source, or native build-fragment path. The
generated-path intersection is empty. The repository authority and temporary
package build still pass with:

- package identity `prometheus.core@1`;
- 69 kernels and 69 variants;
- 68 artifacts and 18 implementations;
- kernel 68 `kernel-68-default` → `Gemma4E2BM1RopeHalfSplit_CS`;
- kernel 69 `kernel-69-default` → `Gemma4E2BM1AttentionScores_CS`;
- unchanged generated/static/package projection discrepancies.

No shader bytes, shader hashes, package membership, package identity, model
topology, or generated authoritative bytes were regenerated or normalized.

## 13. Available validation performed

### PASS

- Starting checkpoint and clean worktree confirmation.
- `prometheus_authority.ps1`.
- `go run ./tools/prometheus_stage0 -check`.
- `prometheus_stage0_required_live.ps1 -SelfTest`.
- `go test ./internal/prometheus/... -count=1`.
- Windows native build `internal/prometheus/native/build_windows.cmd`.
- `PrometheusStage0GemmaABIAndDetailSnapshot` Marionette filter.
- `dumpbin /exports` comparison against the canonical 84-name inventory.
- pre/post numeric assignment comparison for the public and internal headers.
- generated-header/native manifest and compiled-model lock checks through the
  canonical authority path.
- shader-package and Z-Image Go unit packages through the Prometheus Go lane.
- `git diff --check`.

### FAIL

None.

### SKIP

- Required-live Gemma witnesses because `G4E2B_CHECKPOINT_ROOT` is unavailable.
- Fresh allocation/residency/teardown witnesses.
- Checkpoint-dependent canonical Z-Image smoke.
- Live Linux Vulkan; Linux remains unclaimed.

### NOT RUN

- The live wrapper itself, deliberately; its skip-detection self-test was run.
- Full Vulkan Marionette execution suite; the native suite was compiled and the
  ABI witness was executed.
- Fresh kernel-68/69 live execution and fresh package-backed Z-Image execution.
- DXC/SPIR-V regeneration or live shader execution; static/package identities
  were checked without regenerating authoritative bytes.

## 14. Live validation explicitly not performed

The missing external checkpoint is an environmental prerequisite, not an
architectural failure. Stage 0 static characterization, required-live skip
detection, prior accepted static evidence, and current package authority remain
available. None of those facts is a claim that Gemma ran on this checkpoint.
The exact Q-first, K-first, same-session `-7406`, allocation, residency,
teardown, and canonical Z-Image live witnesses remain future work.

## 15. Remaining duplicated or disputed declarations

- Go symbol strings and Go callback/result projections remain a host projection;
  they are checked against the canonical header but cannot be directly included
  in pure-Go builds.
- Internal M42–M49 function names and detail names remain historical for
  compatibility and diagnostic continuity.
- Public/internal detail-code families remain separate; their numeric collisions
  and historical stage names are not repaired.
- The generated shader ID header, static registry, package-only IDs, package
  object count, and repeated `MainTransformer1` successor projection remain
  descriptive/disputed Stage 0/1 facts.
- Older generated-header provenance remains legacy/disputed.
- `-7406` remains an unresolved M46-to-M49 generation/hash handoff issue.

## 16. Rollback boundary

Reverting the single Stage 2 commit restores the two handwritten platform cgo
ABI copies, the private C ABI-version define, the historical canonical names of
the seven isolated internal types, the previous authority output shape, and the
pre-Stage-2 current documentation. It does not require reverting generated
headers, shader objects, package outputs, model projections, payloads, or
runtime ownership. No generated or payload file is in the rollback boundary.

## 17. Stage 3 prerequisites and exact proposed scope

Stage 3 must begin only after this report’s static checks remain green and the
owner decision is reviewed against the restored live witnesses when a payload
is available. Its exact proposed scope is the common Vulkan runtime/device
boundary only:

1. inventory and characterize the existing instance/device/queue/package/
   capability and child-teardown paths across the already-established runtime
   families;
2. choose one common runtime/device owner without changing creation,
   capability admission, shader loading, queue selection, synchronization, or
   teardown behavior;
3. preserve the current ABI veneers and compare before/after native traces;
4. run focused native and available live witnesses before considering later
   slot, session, weight-snapshot, or execution-plan ownership decisions.

Stage 3 must not begin common ownership extraction as an unbounded refactor and
must not repair generated topology or `-7406`. No Stage 3 implementation is part
of this commit.
