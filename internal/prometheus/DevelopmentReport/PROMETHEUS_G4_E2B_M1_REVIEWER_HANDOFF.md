# PROMETHEUS G4-E2B-M1 — raw-score stabilization handoff

## Current repository handoff — Concept/Vulkan EVT1

The immediate assignment is no longer Prometheus Stage 7 or additional
ray-query image work. Concept/Vulkan M0 is recorded in
`internal/prometheus/DevelopmentReport/PROMETHEUS_CONCEPT_VULKAN_M0_CHARACTERIZATION.md`,
with the normative language constitution at
`docs/Concept-Vulkan/CONCEPT_VULKAN_LANGUAGE_CONSTITUTION.md` and current
project briefing at `docs/OCT_SDSLV_PROMETHEUS_LIVING_STATUS.md`.

The accepted proof foundation is:

- M1 through M1D proved a real `.concept -> typed MIR -> generated C ->
  native build -> live executable` vertical beside the handwritten kernel-54
  witness;
- M1D remains honestly classified as `MEANINGFUL PROGRESSION`;
- the proposed M1E handwritten create-path failure seam is not the active
  assignment.

The most recent assignment is now recorded in
`internal/prometheus/DevelopmentReport/PROMETHEUS_CONCEPT_VULKAN_EVT1_M1B_C_BOUNDED_PURE_COMPTIME_EVALUATION_AND_FOUNDATIONAL_CONTROL_FLOW.md`:

```text
Concept/Vulkan EVT1 M1B-C — Bounded Pure Comptime Evaluation and Foundational Control Flow
```

M1A is accepted and closed. M1B-A is accepted and closed. M1B-B is accepted
and closed. EVT1 M1B-C is now honestly classified as `MEANINGFUL PROGRESSION`:
bounded pure compile-time evaluation, expression-valued `if`, ordinary runtime
and bounded compile-time `while`, `static_assert`, top-level/local `comptime`
declarations, and top-level `comptime` free functions all run through the EVT1
typed MIR, generated C, checked outputs, and native specimens. The isolated
remaining blocker is fixed-size arrays inside the compile-time value domain, so
M1B-C is not yet an honest `SUCCESS`. Production remains handwritten.

Concept/Vulkan is the bounded host-side mechanism profile implemented through
the working Go Oct/SDSL-V compiler lineage. Dominatus remains the Prometheus
control kernel; Concept/Vulkan consumes committed work and cannot authorize or
advance it. SDSL-V remains shader-side. Generated C/H remains the checked-in
Prometheus native boundary.

Stage 6 remains exact completed history: the 34-position Z-Image retarget
sequence is finite, but a static plan extraction was rejected because model
authorization/progression remains Vulkan-session owned and no Dominatus
authorization/completion seam exists. Stage 7 remains deferred until after the
initial Concept/Vulkan proof.

RQ-M1's accepted physical batch is also preserved: supported nonzero batches
record one `vkCmdDispatch(ray_count,1,1)` and one synchronous submission with
paired capacity and descriptor rebind-before-retire. Expanded image/cost
authority is paused, not completed or abandoned. EVT1 language work remains
separate from resumed batch or stage-7 work. The exact next language blocker
is now:

```text
Fixed-size arrays in the EVT1 M1B-C compile-time value domain
```

The historical kernel-54 milestone named `M1C` must not be confused with the
later EVT1 milestone named `M1B-C`. DragonGod remains the intended first
serious post-substrate framework direction after the required language
substrate; it is still deferred and not begun here.

## Historical repository handoff — Stage 3

The repository/generated-authority hygiene pass is recorded in
`internal/prometheus/DevelopmentReport/PROMETHEUS_STAGE1_REPOSITORY_AND_GENERATED_AUTHORITY.md`.
The mechanical ABI and vocabulary pass is recorded in
`internal/prometheus/DevelopmentReport/PROMETHEUS_STAGE2_ABI_AND_VOCABULARY_CONSOLIDATION.md`.
The common Vulkan runtime ownership extraction is recorded in
`internal/prometheus/DevelopmentReport/PROMETHEUS_STAGE3_VULKAN_RUNTIME_OWNERSHIP_EXTRACTION.md`.
The compact corpus index is
`internal/prometheus/DevelopmentReport/PROMETHEUS_DEVELOPMENT_EVIDENCE_INDEX.json`,
and the canonical repository-authority check is:

```powershell
powershell -NoProfile -File .\tools\prometheus_authority.ps1
```

Stage 0 remains the frozen lifecycle characterization and vocabulary authority, and Stage 2
remains the ABI/layout/detail preservation authority. Stage 3 establishes the concrete
`prom_vk_runtime` owner in `native/reactor_vulkan_runtime.[ch]`; SGEMM now borrows its device,
queues, command pools, capabilities, validation state, and package through that internal owner
while retaining SGEMM execution resources and policy locally. Construction failure cleanup and
reverse destruction order are explicit in the owner and SGEMM wrapper.
The Stage 3 M34b classification is **INHERITED DETERMINISTIC FAILURE**: the exact
validation-enabled production-variant command failed three times at both Stage 2
(`1f2fc2a5c95cbbefcb3dcc0b76751b752d5fed46`) and Stage 3
(`2bb06095a1755a30f5d5ab8f140838f59ee51be4`).  With `m=3`, `n=17`, and `k=7`,
variant 4 returns zero for each final-column cell where the CPU oracle expects
`1.6458333730697632`; variants 3 and 5 complete through the direct path.  The
independently rebuilt Stage 2/Stage 3 package manifests had identical SHA-256
values.  Do not change M34b, variants, shader bytes, descriptor/pipeline behavior,
or expected results to conceal this inherited behavioral authority.  The related
A2x4 footprint mismatch is inherited too.  Command pools remain in the concrete
Vulkan owner because they are queue-family-bound mechanical resources; SGEMM keeps
command buffers, reset/reuse, fences, submissions, and execution synchronization.
Dominatus decides and coordinates. Vulkan mechanisms execute and report facts.
The three required-live Gemma lanes remain outstanding because the external
checkpoint is unavailable. Stage 3 makes no live Gemma equivalence claim and does not repair
`-7406`, the repeated `MainTransformer1` topology, generated-header provenance, or registry/package
projection differences. The available Windows Vulkan preflight and lifecycle tests are separate
from the deferred payload-backed lanes.

## Stage 0 supersession

Stage 0 has frozen the current implementation vocabulary and executable characterization in
`internal/prometheus/DevelopmentReport/PROMETHEUS_STAGE0_CHARACTERIZATION_AND_VOCABULARY.md`.
This handoff's historical evidence remains valuable, but its immediate-next-milestone advice is
superseded: a future session must begin with the Stage 0 witnesses, not automatic Gemma feature
implementation.

The three independent Gemma lanes are:

- `TestGemma4E2BM1FreshSessionQFirstAuthority`
- `TestGemma4E2BM1FreshSessionKFirstAuthority`
- `TestGemma4E2BM1SameSession7406Characterization`

Run the required-live wrapper with a validated reactor and checkpoint:

```powershell
$env:OCT_PROMETHEUS_REACTOR = "<validated reactor DLL path>"
$env:G4E2B_CHECKPOINT_ROOT = "<validated external Gemma checkpoint root>"
powershell -NoProfile -File .\tools\prometheus_stage0_required_live.ps1
```

The wrapper fails on an environmental skip, missing PASS case, or zero meaningful work. The
ordinary test command may still skip when setup is absent, but a skip is never Stage 0 success.
The exact same-session boundary remains: first chain succeeds; second-chain M46 preparation
succeeds; immediately following M49 required-weight validation rejects with
`PROM_M46_DETAIL_STALE_WEIGHT_GENERATION` (`-7406`); positional dispatch has not begun. Do not
fix, bypass, weaken, move, or reinterpret `-7406` automatically.

The generated-authority check is:

```powershell
go run ./tools/prometheus_stage0 -check
```

It characterizes the `prometheus.core@1` package, source/package counts, kernel 68/69 identities,
partial generated/static projections, and the disputed repeated `MainTransformer1` topology
without repairing it. Accepted numerical authorities remain 1,800/1,800 bit-exact Q-first and
K-first scores, resident kernel-68 → kernel-69 execution, and no host Q/K detour. Accepted model
ceilings remain `643,587,076` bytes MinimumMemory and `1,005,407,748` bytes Prefetch. ABI/detail,
allocation, residency, teardown, and Windows policy witnesses are documented in the Stage 0 report.

Windows is the accepted live platform. Linux remains unclaimed. Do not start another reactor or
add more handwritten model execution. The first future implementation pass recommended by the
full audit is to establish the common Vulkan runtime/device ownership boundary, then make later
owner decisions for slots, sessions, weight snapshots, and generated authority using these frozen
witnesses.

## Proven

- The authority-verified checkpoint-backed Vulkan path runs package-backed
  `kernel-69-default` against the exact resident kernel-68 Q and K outputs.
- Fresh-session Q-first and K-first chains each match all `1,800 / 1,800`
  FP32 raw scores bit-exactly against the accepted sequential stage-local
  authority.
- Q, K, and score use bounded reusable resident roles: Q slot 0 (122,880
  bytes), K slot 1 (15,360 bytes), and distinct FP32 score slot 3 (7,200
  bytes). The ring depth is 3. Q/K have no host detour; only the final score
  readback occurs.
- Kernel-69 arithmetic, geometry, indexing, GQA mapping, BF16 decoding,
  sequential coordinate order, and post-sum `1/16` scaling are accepted. Do
  not reopen them without contradictory evidence.

## Open boundary

In one reusable native session, the first chain succeeds. On the second chain,
M46 weight preparation succeeds, but the immediately following M49 required-
weight validation rejects with `PROM_M46_DETAIL_STALE_WEIGHT_GENERATION`
(`-7406`) before any positional dispatch. The failure is the M46-to-M49
required-weight generation/hash handoff after prior score completion; it is
not initial M46 admission, Q/K ordering or binding, resident-ring depth, or
kernel-69 arithmetic.

`observed_weight_generation` and `requested_weight_generation` have been added
to the closed HeadRmsNormRope and score results. They are zero at this M49
early-return boundary because current propagation covers M46 rejection or full
M46/M49 completion, not M49's required-weight rejection. Possible causes
remain intentionally unproved: completion invalidation, missing reacquisition,
cached source metadata, stale slot role metadata, or asymmetric generation
advancement. The exact handoff to inspect is M46 `weight_result.generation` /
`weight_result.hash` into M49 `required_weight_generation` /
`required_weight_hash`.

## Relevant files and commands

- Native lifecycle and diagnostics: `native/reactor_api.c`,
  `native/reactor_api.h`, `native/reactor_vulkan_transformer.c`,
  `native/reactor_vulkan_runtime.[ch]`, and `native/reactor_vulkan_sgemm_internal.h`.
- Closed Go seam and live harness: `bridge.go`, `bridge_dlopen_windows.go`,
  `bridge_dlopen_linux.go`, `gemma4e2b_m1_rtx.go`, and
  `gemma4e2b_m1_rtx_test.go`.
- Resume command (with validated checkpoint, DLL, hardware, and validation
  environment variables configured):
  `go test -run TestGemma4E2BM1CanonicalQKVRTX -count=1 -v ./internal/prometheus`.

The next payload-enabled action is to rerun the existing Q-first, K-first,
same-session `-7406`, allocation/teardown, and canonical Z-Image witnesses
against the reorganized checkpoint. A live failure is a regression
investigation, not an automatic attribution to the historical checkpoint.
Do not automatically resume implementation of the `-7406` defect.

The exact Stage 4 candidate boundary is the SGEMM-to-reactor execution handoff: slots,
weight ownership, required generations/hashes, and any snapshot or preparation contract
that a second reactor would genuinely consume. It is not begun here; no slots, sessions,
leases, weight registries, binding snapshots, plans, or reactor registries were introduced.

## Current repository handoff — Stage 4

Stage 4 is recorded in
`internal/prometheus/DevelopmentReport/PROMETHEUS_STAGE4_RESOURCE_STATE_AND_EXECUTION_HANDOFF.md`.
It keeps Dominatus as the authority for SGEMM path/compute, layout/precision,
transfer, buffering, and lease decisions; the M35 mechanical branch now consumes
the committed visible Dominatus snapshot. SGEMM uses one private bounded handoff
containing only resolved mechanical command facts (dimensions, selected mode/
variant, slot, descriptor ranges, pipeline, dispatch geometry, and transfer wait
dependency); the pre-existing lease grant remains the final admission edge before
dispatch. `prom_vk_runtime` remains the common
mechanical Vulkan owner, while SGEMM retains its typed roles and execution state.

No M46/M49 weight generation/hash behavior, `-7406`, allocation/residency policy,
shader/package/generated authority, ABI, or model topology changed. The exact M34b
variant-4 final-column witness remains an inherited deterministic failure: expected
`1.6458333730697632`, observed `0`, with the related A2x4 footprint mismatch also
unchanged. Required-live Gemma, payload-backed teardown/Z-Image, and Linux remain
unclaimed.

The exact Stage 5 candidate boundary is a demonstrated shared mechanical
allocation/cleanup substrate only; it must not generalize SGEMM weight semantics
or add a reactor framework.

## Current repository handoff — Stage 5

Stage 5 is recorded in
`internal/prometheus/DevelopmentReport/PROMETHEUS_STAGE5_MECHANICAL_ALLOCATION_AND_CLEANUP_SUBSTRATE.md`.
It proved four current buffer constructors repeated only concrete Vulkan
mechanics, then consolidated buffer creation, requirement discovery,
allocation, zero-offset binding, and caller-requested mapping into two private
functions in `reactor_vulkan_common.c`. The existing wrappers retain all
memory-type/placement selection, queue-family sharing, device-address `pNext`,
mapping choice, and partial-failure cleanup decisions. No generic resource
model or allocation policy was added.

`prom_vk_runtime` remains the common Vulkan owner. Dominatus remains the
control kernel; its committed visible snapshot and the Stage 4 private SGEMM
execution handoff are unchanged. SGEMM retains typed A/B/C/upload roles,
arenas, descriptors, command resources, submission, synchronization, and
readback. Model weights/windows and ray acceleration-structure semantics stay
separate.

The focused real-Vulkan test passed for mapped and unmapped buffers, required
flags, alignment, offset-zero binding, and repeated cleanup. ABI signature,
84 exports, generated/shader/package authority, `prometheus.core@1`, and
kernel 68/69 identities are unchanged. M34b remains the exact inherited
deterministic failure (`m=3`, `n=17`, `k=7`, variant 4, three final-column
cells expected `1.6458333730697632`, observed `0`); the five A2x4 footprint
failures are unchanged. Required-live Gemma, payload-backed teardown/Z-Image,
and Linux remain unclaimed.

The exact Stage 6 candidate boundary is closed model execution planning: move
Z-Image stage order into a bounded plan and Gemma orchestration out of the API
veneer without changing generated identities, dispatch order, residency,
allocation ceilings, or numerical authority. Do not begin that work as part of
Stage 5.

## Current repository handoff — Stage 6

Stage 6 is recorded in
`internal/prometheus/DevelopmentReport/PROMETHEUS_STAGE6_CLOSED_MODEL_EXECUTION_PLAN.md`.
It establishes that the Z-Image model-session path contains a finite implicit
34-position weight-retarget sequence (NoiseRefiner0/1, ContextRefiner0/1,
MainTransformer0--29), duplicated in retarget and prefetch selectors.  The
current model session owns `retarget_position`; public façade calls separately
control capture, composition, and block execution.  No model-operation
authorization/completion edge currently passes through Dominatus.

Stage 6 therefore rejects a static-table-only extraction: it would not be a
closed model execution plan and would conceal mechanism-owned semantic
progress.  No production code, topology, `MainTransformer1` repeated successor
projection, weights/generations/hashes/bindings, allocation/teardown behavior,
Stage 4 SGEMM handoff, ABI, generated authority, shader/package identity,
M46/M49 behavior, `-7406`, or M34b result changed.  The next narrow boundary is
a private Dominatus model-operation authorization/observation seam; it is not
begun here.
