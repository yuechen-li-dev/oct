# Prometheus Stage 0 — characterization and vocabulary freeze

Status: **MEANINGFUL PROGRESSION — live checkpoint-backed Gemma validation is pending local payload setup**

## 1. Scope and checkpoint

**[established authority]** This report places Prometheus under glass at
`4bc7bf40e0c2989c998135b322b25f071e65af37` (`prometheus: add full architecture audit`). The full
architecture audit and the G4-E2B-M1 reviewer handoff remain historical authorities; this document
records the executable Stage 0 characterization added on top of that checkpoint.

**[established authority]** The high-level architecture and execution core remain credible. The
dominant issue is accumulated implementation debt and missing internal ownership boundaries, not
a wholesale replacement requirement. Common Vulkan runtime/device ownership, compute-slot and
lease ownership, model-session/activation-residency ownership, weight-binding transactions and
immutable snapshots, generated shader/model/ABI authority, and a closed compiled execution plan
are future destinations. Stage 0 does not create those owners or abstractions.

**[directly observed current behavior]** Production runtime implementation files are unchanged.
The allowed changes are validation harness code, a native ABI test, build/test registration, the
structured authority checker, the required-live wrapper, this report, and the reviewer handoff.

## 2. Baseline validation table

`PASS` means required work ran and assertions completed. `SKIP` means a prerequisite was absent
and the reason was reported. `NOT RUN` means this pass did not invoke the lane; it is not a pass.

| Surface | State | Evidence |
|---|---|---|
| Checkpoint and worktree | **PASS** | `git rev-parse HEAD` matched the required SHA; worktree was clean before edits. |
| Tracked local debris | **PASS** | No tracked paths under `out/`, `dist/`, or Prometheus local output roots. |
| Windows Vulkan hardware | **PASS** | `vulkaninfo --summary`: Vulkan 1.4, NVIDIA GeForce RTX 3070 API 1.4.329, validation layer present. |
| Focused Go baseline | **PASS** | Focused Prometheus, Gemma, shader-package, native-manifest, and lock tests passed; live tests reported explicit skips. |
| Stage 0 authority inventory | **PASS** | `go run ./tools/prometheus_stage0 -check` emitted schema `prometheus.stage0.authority.v1`, status `PASS`. |
| Required-live skip self-test | **PASS** | The wrapper rejected synthetic skip and zero-work output. |
| Native ABI/detail snapshot | **PASS** | Marionette filter: 1 passed, 0 skipped, 0 failed. |
| Windows native build | **PASS** | `internal/prometheus/native/build_windows.cmd` completed successfully; manifest check completed. |
| Fresh Q-first authority | **SKIP / NOT RUN** | `G4E2B_CHECKPOINT_ROOT` was unset; required-live validation was not greened. |
| Fresh K-first authority | **SKIP / NOT RUN** | Same missing checkpoint prerequisite. |
| Same-session `-7406` characterization | **SKIP / NOT RUN** | Same missing checkpoint prerequisite. |
| Kernel 68/69 SDSL-V, DXC, SPIR-V | **NOT RUN in this pass** | Prior accepted evidence remains cited; no new pass is claimed. |
| Shader package Go validation | **PASS** | `go test ./internal/prometheus/shaderpackage -count=1`. |
| Z-Image unit regression | **PASS** | `go test ./internal/prometheus/zimage -count=1`. |
| Canonical Z-Image smoke | **NOT RUN** | No local canonical payload invocation; no image hash was revalidated. |
| Linux | **UNCLAIMED** | Linux was not run to completion and is not a required passing lane. |

The local Vulkan device is available. The missing `G4E2B_CHECKPOINT_ROOT` and reactor path are
ordinary local setup prerequisites described by `docs/EVT2_LOCAL_PAYLOADS.md`, not an architectural
blocker. They do prevent honest completion of the three live authorities in this checkpoint.

## 3. Canonical semantic vocabulary

The following vocabulary is frozen for Stage 0. Exact fields are intentionally not collapsed when
they currently carry equal integers. Ownership is classified as **explicit**, **implicit**,
**duplicated**, or **unresolved**.

| Canonical term | Current fields / types | Value identifies | Lifecycle/invariant | Ownership / detail |
|---|---|---|---|---|
| Model identity | Lock `ManifestIdentity`, `ModelSemanticIdentity`, `ProductionExecutionIdentity`, `AuditProfileIdentity`; API `model_contract_identity`, `execution_plan_identity`, `canonical_authority_identity` | Immutable model/contract facts | Created by lock/generation; read at model/session creation and evidence capture; stable thereafter | **Duplicated**; no `-7406` relation |
| Checkpoint identity | `tools/gemma4e2b_m1_reference.py::CHECKPOINT_SHA256`; `G4E2B_CHECKPOINT_ROOT` | Immutable source checkpoint | Validated before live preparation; wrong checkpoint must not be used silently | **Explicit** in reference tooling, **implicit** in Go setup |
| Package identity | Shader manifest `package.id`, `package.version`, `runtime_abi`; C `prom_shader_package` | Immutable packaged shader authority | Created by packaging; loader validates before module use | **Explicit** |
| Immutable weight identity/hash | `prom_model_block_weight_resource.content_identity`, `.layout_identity`; `prom_transformer_parameter_resource.hash`; `m46_weight_hash` | Immutable weight bytes/layout or content hash | Created during load/prepare; compared at binding/execute boundaries | **Duplicated** across model, transformer, and M46 |
| Required weight identity/hash | `PrometheusGemma4E2BM1HeadRmsNormRopeRequest.required_weight_generation/hash`; `prom_m49a_m46_request` fields | Caller-captured validation snapshot | Captured after M46 preparation; M49 compares before positional dispatch | **Duplicated scalar snapshot**; `-7406` protects mismatch |
| Active weight generation | `prom_reduction_runtime_state::m46_weight_generation`; model `binding_generation`, `descriptor_generation`, `prefetch_generation`, `prefetch_descriptor_generation` | Mutable active binding/storage version | Written by upload/rebind/activation; read by validation and execution | **Duplicated** concepts sharing `uint32_t` |
| Source-captured weight generation | Request `weight_generation`, `required_weight_generation`, `source_output_generation`; result `observed_weight_generation`, `requested_weight_generation` | Value captured at a request/result boundary | Captured before validation; must agree with current authority | **Implicit/duplicated**; early `-7406` return leaves outer fields zero |
| Weight preparation | `prom_reactor_runtime_m46_prepare_weight`; Go wrapper `prometheus_reactor_runtime_gemma4e2b_m1_head_rmsnorm_rope` | Preparation transition, not an owner | Hash/check/upload/wait, write M46 active generation/hash, return result | **Procedural writer**; semantic owner unresolved |
| Weight binding | `state->m46_weight`, `m46_weight_generation/hash/model_width`; model active/pending/prefetch resources | Mutable binding of immutable data to device storage | Prepare/rebind replaces active binding; M49 validates captured snapshot | **Implicit** in shared runtime state |
| Validation snapshot | `prom_m49a_m46_request.required_weight_generation/hash` plus output/source fields | Immutable expected-at-call values | Created at handoff; invalid when active M46 pair differs | No first-class type; **duplicated** |
| Runtime/device owner | `prometheus_runtime`, `prom_vk_runtime_services`, attached Vulkan/device state | Mutable device/runtime resources | Created at runtime construction; destroyed by native destroy | **Explicit lifetime**, broad ownership unresolved |
| Execution session | `prom_compiled_model_session_state`, `session_id`, `next_session_id`; public session APIs | Session identity and stream state | Create/capture/compose/reset/teardown; evidence validates generation | **Explicit** for compiled model; Gemma raw-score ID is not exported |
| Operation identity/epoch | `logical_request_id`, `next_logical_request_id`, `submission_sequence`, M49 execution index; Stage 0 trace labels | Request occurrence/order, not content | Allocated at planning/submission; compared for ordering/replay | **Mixed** with replay identities; trace epochs are harness labels |
| Submission slot | `prom_reduction_slot.slot_id`, `.state`, SGEMM physical-slot fields | Reusable physical submission/storage location | Acquire before dispatch; completion/reap returns it | **Explicit** slot state; reduction payload overloaded |
| Slot epoch | `prom_reduction_slot.generation`; `physical_slot_generation` | Reuse epoch for a slot | Increment/reuse; consumers validate slot plus epoch | **Explicit scalar** |
| Activation residency | `resident_input_generation`, `output_generation`; resident streams and Gemma Q/K fields | Logical content resident in device storage | Producer completion makes content available; consumer validates before reuse | **Explicit** in compiled model; Gemma roles implicit |
| Activation content generation | `output_generation`, `resident_input_generation`, stream `.generation`, `.producer_output_generation` | Logical activation content version | Producer writes; reset/rebind invalidates; downstream reads | **Explicit fields**, but type-shared with bindings |
| Retained Q/K role | `gemma4e2b_m1_rope_q_slot_id/_slot_generation/_valid`, K equivalents | Role-specific retained resident output | Role production retains; kernel 69 consumes; score completion clears | **Implicit six-scalar role owner** |
| Retained-role generation | Q/K `_slot_generation` fields | Slot epoch for retained role | Written at retention; kernel 69 validates; release clears validity | **Explicit value, procedural release** |
| Buffer identity | `prom_device_buffer_view.buffer`, `.owning_device`, `.owning_lifetime_id`, `.owning_slot_id`, `.owning_slot_generation` | Physical storage and owner epoch | Created/attached at allocation; read by planners/descriptors; teardown invalidates | **Explicit mechanism**, broad lifetime vocabulary |
| Range | `prom_device_buffer_view.offset`, `.byte_length`; request/result byte ranges | Subrange of physical buffer | View creation; overlap/descriptor checks protect non-aliasing | **Explicit** |
| Pin | Q/K `_valid`; slot validity/state | Retention/admissibility bit, not a lease object | Set after role production; cleared after score/reset | **Implicit boolean** |
| Lease/acquisition | `prom_reduction_acquire_slot`, SGEMM physical-slot acquisition/quarantine | Temporary right to submit/use a slot | Acquire before dispatch; completion/reap releases/quarantines | **Explicit** in SGEMM; no typed Gemma lease |
| Completion | Fence waits, slot transitions, `prom_reduction_reap`, dispatch/readback counters | Work completion and safe reuse | Completion precedes release/reuse and teardown | **Procedural but observable** |
| Invalidation | Q/K valid clearing, reset/rebind validity, `output_valid`/stream validity | Content or binding no longer admissible | Completion, reset, rebind, or failure | **Duplicated booleans/call order** |
| Release | Score success clears Q/K valid; slot reap; runtime/session destroy | End of role, slot, or owner lifetime | Must follow completion; teardown leaves no owner resources | **Procedural**, broad runtime owner explicit |
| Score destination generation | Result `score_slot_generation`, `score_hash`; score slot/range | Physical slot epoch plus content hash, not content generation | Dispatch writes; readback completes; slot releasable | Slot epoch explicit; content generation unresolved |

## 4. Exact field/type ownership notes

**[directly observed current behavior]** Native Gemma state is in
`internal/prometheus/native/reactor_vulkan_runtime_internal.h` (`prom_reduction_runtime_state`,
`prom_reduction_slot`, and `prom_model_block_state`). Public request/result shapes are in
`internal/prometheus/native/reactor_api.h`; lifecycle functions are in `reactor_api.c` and the
transformer implementation.

The current M46-to-M49 path is:

1. `prometheus_reactor_runtime_gemma4e2b_m1_attention_scores` builds two positional requests.
2. `prometheus_reactor_runtime_gemma4e2b_m1_head_rmsnorm_rope` calls
   `prom_reactor_runtime_m46_prepare_weight`.
3. M46 writes `state->m46_weight_generation/hash/model_width` and returns those values.
4. The API copies the return values to M49 `required_weight_generation/hash`.
5. `prom_reactor_runtime_m49a_execute_m46` compares those fields to the mutable singleton before
   slot acquisition and positional dispatch.

**[unresolved]** The public ABI exposes no session/owner identity, no direct second-call M46 result
at the M49 early return, and no typed validation snapshot. Stage 0 therefore proves the boundary
and source-level copy path, but cannot claim which later writer changes the compared value without
production instrumentation. Instrumentation is deliberately not added.

## 5. M42–M49 milestone-to-semantic mapping

Milestone names remain in source. Proposed names below are documentation-only and are not aliases.

| Milestone/symbol | Current responsibility, inputs, outputs | Semantic name | Rename risk |
|---|---|---|---|
| M42 `prom_reactor_runtime_m42_*` | Single-head attention preparation/execution and weight binding; resident inputs to head outputs | single-head attention preparation/execution | Ownership-sensitive |
| M43 `prom_reactor_runtime_m43_*` | Bounded grouped multi-head aggregation over explicit device views | grouped multi-head attention aggregation | Ownership-sensitive |
| M44 `prom_reactor_runtime_m44_*` | Attention output projection and WO binding | attention output projection | Mechanical only after consumers are inventoried |
| M45 `prom_reactor_runtime_m45_*` | Attention residual combination | attention residual combine | Unsafe while request nesting remains coupled |
| M46 `prom_reactor_runtime_m46_prepare_weight` / M46 functions | RMSNorm-adjacent preparation, validation, upload/binding, execution | RMSNorm weight preparation and binding validation | Ownership-sensitive; number hides two responsibilities |
| M47 `prom_reactor_runtime_m47_*` | Gated FFN/residual path and binding | gated feed-forward residual execution | Ownership-sensitive |
| M48 `prom_reactor_runtime_m48_*` | Stack continuation and resident activation handoff | transformer stack activation continuation | Unsafe before execution-plan boundary |
| M49a `prom_reactor_runtime_m49a_execute_m46` | Required-weight validation, M46 execution, positional continuation/composition | compiled operation admission plus positional continuation | Unsafe before consolidation |
| M49b/controller terms | Numerical route/controller and execution-selection telemetry | execution-route policy/telemetry | Potentially obsolete only after owner decision |
| `kernel-68-default` / `Gemma4E2BM1RopeHalfSplit_CS` | Resident Q/K positional preparation output | resident positional Q/K producer | Mechanical mapping; package identity stays authoritative |
| `kernel-69-default` / `Gemma4E2BM1AttentionScores_CS` | Resident Q/K score production, GQA mapping, scaling, 1,800 FP32 outputs | resident-Q/K score producer | Mechanical mapping; arithmetic contract accepted |

## 6. Fresh-session Q-first authority

**[established authority]** `TestGemma4E2BM1FreshSessionQFirstAuthority` is independently
invocable with preparation order `1`. It creates fresh score runtime/session state, requires
exactly 1,800 values, retains bit-exact comparison, and asserts two positional dispatches, one
score dispatch, one final readback, no host Q/K detour, and a written score destination. It is not
a same-session reuse test.

**[prior accepted live evidence]** Fresh Q-first completed `1,800 / 1,800` FP32 comparisons
bit-exactly against the stage-local sequential authority. This shell did not rerun it because
`G4E2B_CHECKPOINT_ROOT` was unset; the required-live wrapper rejects that condition.

## 7. Fresh-session K-first authority

**[established authority]** `TestGemma4E2BM1FreshSessionKFirstAuthority` is independently
invocable with preparation order `0`. A separate test invocation creates fresh runtime state and
applies the same 1,800-value and lifecycle assertions as Q-first.

**[prior accepted live evidence]** Fresh K-first completed `1,800 / 1,800` FP32 comparisons
bit-exactly. This shell did not rerun it for the same missing external checkpoint prerequisite.

## 8. Same-session `-7406` characterization

**[established authority]** `TestGemma4E2BM1SameSession7406Characterization` is explicitly a
known-defect characterization. It passes only when this exact sequence is observed:

```text
first chain succeeds
  -> second chain M46 preparation boundary is observed
  -> immediately following M49 required-weight validation rejects with -7406
  -> positional dispatch has not begun
```

It fails on first-chain failure, second M46 failure, M49 success, another detail code, another
boundary, any positional/score dispatch, absent lifecycle evidence, skip, or zero work. It is not
in a suite whose meaning is all supported execution succeeds.

**[prior accepted live evidence]** The current boundary is
`PROM_M46_DETAIL_STALE_WEIGHT_GENERATION` (`-7406`) in
`prom_reactor_runtime_m49a_execute_m46`, before positional dispatch. M46 preparation of the
second chain succeeds. `observed_weight_generation` and `requested_weight_generation` are zero at
this early return because the outer API propagates them on M46 rejection or completed M46/M49,
not on this M49 validation rejection.

**[unresolved]** The trace does not assert a root cause. It establishes disagreement at the
M46-to-M49 required-weight generation/hash handoff after score completion; it does not attribute
the disagreement to score arithmetic, Q/K order, kernel 69, or external activation residency.

## 9. M46→M49 lifecycle trace and invariant relationships

The Go trace type `gemma4e2bSameSessionTrace` is a compact structured assertion record. The test
logs JSON, not a nondeterministic full-log golden file. It records operation labels, first-chain
result counters, second boundary result, M46-boundary observation, dispatch/output booleans, and
propagated generation fields.

| Boundary | Required relationship | Evidence / limitation |
|---|---|---|
| First preparation | Preparation order is explicit; M46 succeeds before positional work | Harness records order; source shows M46 write/return path |
| First positional dispatch | Q/K are resident and dispatch occurs without host Q/K detour | Accepted evidence: Q slot 0, K slot 1; 122,880/15,360-byte ranges; detour 0 |
| First score completion | Distinct score slot, 7,200 bytes, 1,800 FP32 values written/read back | Accepted evidence: score slot 3 and one final readback |
| First release/retention | Completion precedes retained Q/K release | Source path and audit; no fresh teardown run here |
| Second M46 preparation | Preparation succeeds before M49 boundary | Prior live evidence plus source-level copy path; ABI lacks direct early-return M46 result |
| M46→M49 handoff | Required generation/hash equals active M46 pair | M49 comparison is the invariant producing `-7406` |
| M49 validation | Rejection precedes slot acquisition/positional dispatch | Direct source control flow and accepted live boundary |
| Post-rejection state | No positional dispatch, score dispatch, or score destination write | Required by same-session assertions |

**[directly observed current behavior]** On the successful copy path, M46 returned generation/hash
and the copied required generation/hash are equal. The failing early return exposes no direct
second-call observed/requested result. Distinguishing values include preparation order,
source-captured generation inputs, operation occurrence, slot epochs, and success-versus-rejection
result. The public result cannot identify the last writer of a differing singleton value.

**[audit inference]** The current procedural writers are M46 preparation for active singleton
state and the API wrapper for the required snapshot. Neither is proven to be the durable semantic
owner. The protected invariant is equality of required generation/hash and active M46
generation/hash at M49 admission.

## 10. Explicitly unresolved ownership questions

- **[unresolved]** Which component owns the M46 active binding across reusable-session score
  completion and the next preparation?
- **[unresolved]** Whether M46 generation/hash is invalidated, advanced, or overwritten after the
  first score completion in the failing live session.
- **[unresolved]** Whether the M49 required pair is an immutable snapshot or duplicated request
  scalars copied through the API.
- **[unresolved]** Whether Q/K role validity is a lease, pin, or procedural boolean; current
  fields do not encode the distinction as a type.
- **[unresolved]** Whether score content has a logical generation distinct from
  `score_slot_generation`, which is a physical slot epoch.
- **[unresolved]** Whether the generated `MainTransformer1` successor projection is inert,
  partially authoritative, or wrong.
- **[unresolved]** Whether the static shader registry is intentionally partial for package-only
  kernels 52–69 or is a generated-authority gap.

## 11. Generated/model/shader/package authority map

**[established authority]** `go run ./tools/prometheus_stage0 -check` emits schema
`prometheus.stage0.authority.v1` with the following current inventory:

```json
{
  "source": {"assets": 69, "implementations": 18},
  "package": {"identity": "prometheus.core@1", "kernels": 69, "variants": 69,
    "artifacts": 68, "implementations": 18, "provenance": 69, "object_files": 74,
    "extra_object_files": 6},
  "projection": {"generated_kernel_ids": 66,
    "package_not_generated_ids": [67, 68, 69], "static_registry_ids": 51,
    "package_only_ids": [52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69],
    "unreferenced_package_only_ids": null},
  "gemma": {"kernel_68_variant": "kernel-68-default",
    "kernel_69_variant": "kernel-69-default"},
  "topology": {"main_transformer_blocks": 30,
    "repeated_main_transformer1_successors": 29,
    "status": "descriptive_disputed_current_projection"},
  "abi": {"exported_symbols": 84, "stale_weight_detail_code": -7406}
}
```

The checker normatively protects source/package counts and membership, package identity, Gemma
variant IDs, object-file sufficiency, and native-source references for package-only IDs. It
descriptively snapshots generated-header and static-registry projections. Six extra local object
files are not blessed as package artifacts.

**[descriptive characterization]** `lock-tagon.octagon` has 30 main transformer blocks and 29
repeated `Successor: "MainTransformer1"` entries. The projection is characterized exactly, not
accepted as semantic topology. The audit indicates runtime execution consumes bridge/generated
parameter tables without proving this topology field authoritative. Future work must decide
whether this projection is inert, partial, or authoritative before changing it.

The existing authority flow remains:

```text
manifest -> lock/generated descriptors -> static native registry/loaders
         -> shader package -> native/public ABI
```

Stage 0 checks these existing surfaces; it does not create a second manifest or repair drift.

## 12. ABI, layout, enum, and detail-code characterization

**[established authority]** Registered native test
`PrometheusStage0GemmaABIAndDetailSnapshot` statically and dynamically protects the current
public Gemma ABI layout:

| Type / field | Current size or offset |
|---|---:|
| `PrometheusGemma4E2BM1InputRMSNormRequest` | 112 bytes |
| `PrometheusGemma4E2BM1InputRMSNormResult` | 160 bytes |
| `PrometheusGemma4E2BM1RopeRequest` | 104 bytes |
| `PrometheusGemma4E2BM1RopeResult` | 96 bytes |
| `PrometheusGemma4E2BM1HeadRmsNormRopeRequest` | 144 bytes |
| `PrometheusGemma4E2BM1HeadRmsNormRopeResult` | 144 bytes |
| `PrometheusGemma4E2BM1AttentionScoresRequest` | 208 bytes |
| `PrometheusGemma4E2BM1AttentionScoresResult` | 136 bytes |
| Attention request `preparation_order` | offset 144 |
| Attention result `query_slot_id` | offset 32 |
| Attention result `score_byte_range` | offset 72 |
| Attention result `observed_weight_generation` | offset 120 |
| Attention result `requested_weight_generation` | offset 128 |

It also protects `PROM_M46_DETAIL_STALE_WEIGHT_GENERATION == -7406`, `PROM_OK == 0`,
`PROM_ERROR == 1`, and current stage/status values. The public export inventory is 84
`PROM_REACTOR_API` declarations in `reactor_api.h`; no export was added or removed.

**[established authority]** Platform policy remains Vulkan 1.4 semantics, DXC highest supported
target spelling `vulkan1.3`, generated SPIR-V 1.6, and validation under
`spirv-val --target-env vulkan1.4`. The spelling does not downgrade runtime policy.

## 13. Allocation, residency, and teardown characterization

**[established authority]** Accepted model-owned ceilings remain:

- `MinimumMemory`: `643,587,076` bytes.
- `Prefetch`: `1,005,407,748` bytes.

**[prior accepted live evidence]** The model uses one active and one prefetched bounded weight
window where Prefetch permits it; persistent device activations remain resident while weights
stream through bounded windows. Thirty structurally uniform transformer layers reuse one compiled
assembly with distinct parameter sets. Accepted Prefetch evidence reports 88,473,600-byte mapped
windows and preserves the stated ceiling.

**[prior accepted Gemma evidence]** Raw-score residency uses ring depth 3, Q slot 0 with 122,880
bytes, K slot 1 with 15,360 bytes, and distinct score slot 3 with 7,200 bytes. Q/K are consumed
resident on device; only final scores are read back. Completion precedes reuse.

**[directly observed current behavior]** Stage 0 adds no general allocator/accounting framework.
Fresh Vulkan validation, teardown resource enumeration, and a new allocation-growth run are
**NOT RUN** here because the checkpoint-backed live payload was not configured. No new
zero-validation-error or teardown pass is claimed.

## 14. Skip-detection behavior

`tools/prometheus_stage0_required_live.ps1` is the authoritative required-live wrapper. It:

- requires `OCT_PROMETHEUS_REACTOR` and `G4E2B_CHECKPOINT_ROOT`;
- enables integration, required Vulkan hardware, and Vulkan validation flags;
- runs the generated-authority checker;
- invokes each named Gemma test independently;
- rejects `--- SKIP: ...`, missing `--- PASS: ...`, nonzero test exit, and zero-work output.

The self-test covers synthetic PASS, SKIP, and zero-work output. Ordinary developer suites may
retain environmental skips, but this wrapper cannot turn them green. Missing payload is setup
failure, not successful numerical or lifecycle validation. Linux is not made required.

## 15. Tests and tools added or changed

- `internal/prometheus/gemma4e2b_m1_rtx_test.go`: three independent authorities and exact
  same-session defect assertions; JSON lifecycle trace logging.
- `internal/prometheus/gemma4e2b_m1_rtx.go`: validation-lane plumbing only; no native runtime
  semantics, weight generation, binding, activation, dispatch, or teardown changes.
- `internal/prometheus/native/Marionette/stage0_characterization_tests.cpp`: ABI/layout/detail
  snapshot test.
- `internal/prometheus/native/native_manifest.json`, `native_sources_windows.cmd`, and
  `native_sources_linux.sh`: generated test registration only.
- `tools/prometheus_stage0/main.go`: structured generated/package/ABI/topology inventory.
- `tools/prometheus_stage0_required_live.ps1`: required-live execution and skip detector.
- This report and `PROMETHEUS_G4_E2B_M1_REVIEWER_HANDOFF.md`.

No production C/C++ runtime source, shader source/bytes, package identity, public ABI definition,
allocation policy, synchronization order, or teardown implementation was changed.

## 16. Stable contracts protected for future refactors

1. Fresh Q-first and K-first remain independent authorities, each requiring 1,800/1,800 bit-exact
   FP32 scores and explicit preparation order.
2. The same-session lane is a known-defect characterization, not a success-path test, and must
   reproduce the exact M46-preparation → M49 `-7406` pre-positional boundary.
3. Kernel 69 consumes resident kernel-68 Q/K outputs with no host Q/K detour.
4. The score destination is separate from Q/K storage and read back only after completion.
5. Package identity is `prometheus.core@1`; current package/source counts and Gemma variant IDs
   are stable witnesses.
6. M46 `-7406` remains the stale required-weight generation detail; it must not be bypassed,
   weakened, moved, or converted into a skip.
7. ABI sizes, offsets, status/detail values, Vulkan policy, and DXC/validator relationship remain
   unchanged.
8. `MinimumMemory` and `Prefetch` ceilings remain unchanged.
9. Required-live lanes fail on skip and zero work.

## 17. Descriptive snapshots that are not accepted semantic contracts

- Repeated `MainTransformer1` successors in generated lock topology.
- Six parallel Gemma Q/K role scalars and procedural valid-bit release.
- Package-only shader IDs 52–69 absent from the static registry projection while native source
  references remain present.
- Generated shader ID header containing 66 IDs while the package contains 69 kernels.
- Six extra local shader object files beyond the 68 packaged artifacts.
- Zero observed/requested generation fields at the M49 early return.
- Equality of similar generation values across different lifecycle concepts.

These are evidence snapshots for later owner decisions, not permissions to repair or bless them.

## 18. Findings requiring future owner decisions

The first future implementation pass should follow the full audit’s sequencing: establish the
common Vulkan runtime/device ownership boundary and its evidence before changing the M46/M49
weight transaction. Then decide explicit compute-slot/lease and model-session/activation-residency
owners, followed by immutable weight snapshots and generated-authority closure. Do not begin
another reactor and do not add more handwritten model execution as a Stage 0 follow-up.

The future pass must use these witnesses to determine whether the M46 active binding and M49
required snapshot are one transaction or two, without weakening the current rejection until the
replacement behavior is independently characterized.

## 19. Exact validation commands

Baseline and focused checks:

```powershell
git rev-parse HEAD
git status --short
vulkaninfo --summary
go test ./internal/prometheus ./internal/prometheus/gemma4e2b ./internal/prometheus/shaderpackage ./tools/prometheus_native_manifest ./tools/compiled_model_lock -count=1 -v
go run ./tools/prometheus_stage0 -check
powershell -NoProfile -File .\tools\prometheus_stage0_required_live.ps1 -SelfTest
& .\internal\prometheus\native\build_windows.cmd
& .\out\prometheus\native\marionette_tests.exe PrometheusStage0GemmaABIAndDetailSnapshot
```

Required-live setup and independent authorities:

```powershell
$env:OCT_PROMETHEUS_REACTOR = "<validated reactor DLL path>"
$env:G4E2B_CHECKPOINT_ROOT = "<validated external Gemma checkpoint root>"
powershell -NoProfile -File .\tools\prometheus_stage0_required_live.ps1
go test -run '^TestGemma4E2BM1FreshSessionQFirstAuthority$' -count=1 -v ./internal/prometheus
go test -run '^TestGemma4E2BM1FreshSessionKFirstAuthority$' -count=1 -v ./internal/prometheus
go test -run '^TestGemma4E2BM1SameSession7406Characterization$' -count=1 -v ./internal/prometheus
```

The remaining repository-authority commands are the existing kernel 68/69 SDSL-V checks, DXC
compile, `spirv-val --target-env vulkan1.4`, shader-package validation, Z-Image regression, and
canonical Z-Image smoke from the current payload-backed authority. Results must be recorded as
PASS, FAIL, SKIP, or NOT RUN; absent payload is not a pass.

## 20. Explicit next-pass boundary

Stage 0 ends at characterization. It does not fix `-7406`, alter generations/hashes/bindings/
snapshots, change activation roles or slot/lease behavior, repair generated topology, change
shader/package/ABI/Vulkan policy, add a reactor, or add another handwritten model layer. The next
pass may make an owner decision only after the live witnesses are green and the full-audit
sequencing is accepted.
