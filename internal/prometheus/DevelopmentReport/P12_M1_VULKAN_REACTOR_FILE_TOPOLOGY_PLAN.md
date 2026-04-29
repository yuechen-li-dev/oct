# P12 M1 — Vulkan Reactor File Topology Migration Plan

## 1) Why P12 exists

`internal/prometheus/native/reactor_vulkan.c` now contains most of the SGEMM Vulkan reactor runtime in one compilation unit, including both low-level Vulkan plumbing and SGEMM-family policy/runtime concerns.

This has reached the point where topology clarity is the bottleneck, not missing features.

P12 is therefore a **topology cleanup** milestone:

- no feature additions,
- no behavior changes,
- no public ABI redesign,
- no SGEMM policy/runtime rewrite.

Primary objective: move from one large monolith to a **family-oriented layout**:

- `reactor_vulkan_common.c`
- `reactor_vulkan_sgemm.c`
- `reactor_vulkan_fft.c` (stub later)
- `reactor_vulkan_fused_reduction.c` (stub later)
- `reactor_vulkan.h`

---

## 2) Current reactor_vulkan.c section audit

Audit basis:

- `internal/prometheus/native/reactor_vulkan.c`
- `internal/prometheus/native/reactor_vulkan.h`
- `internal/prometheus/native/reactor_api.h`
- `internal/prometheus/native/reactor_api.c`
- `internal/prometheus/native/build_stub.sh`
- `internal/prometheus/native/build_windows.cmd`
- `internal/prometheus/native/Marionette/*`
- non-Vulkan companion modules:
  - `reactor_judgment_engine.*`
  - `reactor_policy_memory.*`
  - `reactor_slot_hfsm.*`
  - `reactor_dominatus_blackboard.*`
  - `reactor_dominatus_sgemm_adapter.*`
  - `reactor_dominatus_slot_adapter.*`

### 2.1 High-level map (reactor_vulkan.c)

> Line ranges are approximate section envelopes and can be refined during M2/M3 file cuts.

1. **Includes/macros/platform glue** (~1–76)
   - platform includes (`windows.h` / `pthread.h`), Vulkan include, shader blob includes, runtime constants.

2. **Runtime structs/state and SGEMM-centric types** (~37–557)
   - buffer wrappers, arena metadata, policy/controller state, slot diagnostics, selector caches, batch structs, runtime struct with Vulkan handles and SGEMM/batch/async state.

3. **Handle registry + cross-platform mutex/thread shims** (~563–656)
   - global registry and OS-specific synchronization/thread wrappers.

4. **Embedded shader binaries + generic status helpers** (~659–770)
   - SPIR-V arrays and shared status setter.

5. **Batch test flags, planning, worker orchestration, and event drain** (~771–1504)
   - batch test helpers, worker partitioning, worker resources, event ring drain, worker execution loop, thread entrypoints.

6. **Selector cache + Dominatus staging/commit mirroring glue** (~1505–1803)
   - selector-cache invalidation, transfer policy selection, async telemetry mirroring, transfer telemetry commit/staging.

7. **Size/layout helpers + slot lifecycle + layout/precision transforms** (~1804–2478)
   - safe-size math, slot HFSM coupling helpers, FP16 conversion helpers, Packed4 helpers, reference SGEMM helpers.

8. **SGEMM controller defaults/state transitions** (~2479–2660)
   - policy memory defaults, shape signature, lookahead/outstanding/chunk tuning state management.

9. **Registry membership checks + Vulkan device/memory helpers** (~2661–2864)
   - registry add/remove, software Vulkan detection, memory type lookup, buffer create/destroy.

10. **Typed arena ownership and artifact invalidation logic** (~2865–3775)
    - arena capacity checks, budget enforcement, compatibility, shrink logic, artifact invalidation counters/reasons.

11. **Runtime init/cleanup** (~3776–4262)
    - last-shape note, runtime cleanup, Vulkan init (instance/device/queues/pipelines/pools/fences/semaphores).

12. **Public ABI impl + SGEMM execution paths** (~4263–EOF)
    - create/destroy/probe,
    - single-call SGEMM path,
    - async submit/query/consume/abandon,
    - batch dispatch and multi-queue submit logic,
    - diagnostics export.

### 2.2 Public ABI ownership today

`reactor_api.c` is currently a thin forwarder to `prom_reactor_runtime_*_impl` symbols declared in `reactor_vulkan.h`. This means SGEMM ABI symbol names can remain stable while source files are split, as long as impl symbol names are preserved.

### 2.3 Build and test integration dependencies today

- `build_stub.sh` and `build_windows.cmd` both compile `reactor_vulkan.c` explicitly.
- Marionette tests are linked with the same source list and rely on the current SGEMM behavior/diagnostics exposed through reactor ABI.

### 2.4 Topology inconsistency explicitly surfaced

The intended long-term topology calls out a unified `reactor_vulkan.h`. A file with that name already exists, but currently acts as a **narrow ABI-forward declaration header** (impl entrypoints), not yet as a shared common+family internal header.

This is a naming/scope mismatch that should be resolved intentionally in P12 M2/M3 without broadening the header into a god-header.

---

## 3) Target file topology (authoritative for P12)

### 3.1 `reactor_vulkan_common.c`

Role: shared Vulkan plumbing only.

Scope (candidate set):

- platform/Vulkan include glue needed by multiple families,
- queue family discovery and non-SGEMM-specific device capability probing,
- generic memory-type lookup,
- generic Vulkan buffer/pool/fence/semaphore utility helpers,
- generic error/status helpers.

Must **not** absorb SGEMM policy/runtime logic.

### 3.2 `reactor_vulkan_sgemm.c`

Role: full SGEMM reactor family implementation.

Scope:

- SGEMM runtime state,
- single-call SGEMM,
- batch dispatch + worker runtime,
- typed arenas + artifact invalidation,
- Dominatus integration for SGEMM facts/decisions/telemetry,
- Packed4/FP16 logic,
- transfer queue integration for SGEMM,
- async lifecycle,
- SGEMM diagnostics export,
- SGEMM ABI impl entrypoints.

Allowed to remain large; sectioned for locality.

### 3.3 `reactor_vulkan_fft.c`

Role: future FFT family reactor (stub in later milestone).

Planned eventual scope:

- FFT kernels/pipelines,
- FFT resource ownership,
- FFT-specific Dominatus facts/diagnostics,
- FFT ABI (if introduced later).

M1/M2/M3 behavior: no fake capability, no behavioral claims.

### 3.4 `reactor_vulkan_fused_reduction.c`

Role: future fused reduction family reactor (stub in later milestone).

Planned eventual scope:

- softmax/layernorm/reduction/scan-style kernels,
- fused-reduction resource ownership,
- family diagnostics/facts,
- ABI only if added later.

M1/M2/M3 behavior: no fake capability, no behavioral claims.

### 3.5 `reactor_vulkan.h`

Role: shared internal Vulkan reactor declarations/types used across `common` + family files.

Constraints:

- keep SGEMM-private structs private to `reactor_vulkan_sgemm.c` unless truly shared,
- avoid becoming a catch-all include dumping ground,
- retain stable impl symbol declarations expected by `reactor_api.c`.

---

## 4) What belongs in common (and only common)

Move candidates from current `reactor_vulkan.c` to `reactor_vulkan_common.c` in M2 (low-risk first):

1. `set_status(...)` style status helpers.
2. generic safe-size helpers usable by future families.
3. Vulkan-neutral utility checks (e.g., software-driver string helper if not SGEMM-specific).
4. queue family / memory type discovery helpers.
5. generic buffer lifecycle wrappers not coupled to SGEMM arena semantics.
6. generic command/fence helper wrappers not coupled to SGEMM batch runtime behavior.

Guardrails:

- No controller policy logic.
- No Packed4/FP16 policy or transforms.
- No typed arena artifact ownership logic.
- No SGEMM transfer selector caches.
- No SGEMM diagnostics structure population.

---

## 5) What stays in SGEMM (explicit ownership)

Remain in `reactor_vulkan_sgemm.c`:

- SGEMM runtime struct fields and SGEMM-specific state machines,
- all batch planner/worker/event runtime,
- async SGEMM lifecycle,
- transfer queue selection and SGEMM transfer telemetry,
- Dominatus SGEMM staging/commit integration,
- typed arenas and artifact invalidation policies,
- Packed4/FP16 layout/precision selection and helpers,
- public SGEMM ABI impl functions (`prom_reactor_runtime_*_impl`).

Rationale: preserves “one complete SGEMM reactor file” locality for implementation and audit workflows.

---

## 6) FFT/fused-reduction stub scope

Not part of M1 code changes.

For future stubs (M4):

- Add source files that compile but expose no new runtime capability.
- Keep them self-contained and intentionally inert.
- Avoid wiring any probe claims that imply FFT or fused-reduction support.
- Add explicit comments: “placeholder for future family implementation.”

---

## 7) Required internal SGEMM section banner plan

When `reactor_vulkan.c` is renamed to `reactor_vulkan_sgemm.c` (M3), enforce clear in-file banners. Proposed canonical order:

1. `SGEMM Includes / Platform Glue`
2. `SGEMM Runtime State`
3. `Vulkan Common Integration`
4. `SGEMM Dominatus Integration`
5. `SGEMM Policy / Judgment Fact Building`
6. `SGEMM Typed Arena / Buffer Artifact Ownership`
7. `SGEMM Layout Precision: Packed4 / FP16`
8. `SGEMM Transfer Queue Integration`
9. `SGEMM Single-Call Execution`
10. `SGEMM Async Lifecycle`
11. `SGEMM Batch Dispatch / Worker Runtime`
12. `SGEMM Multi-Queue Submit`
13. `SGEMM Diagnostics Export`
14. `Public SGEMM ABI Entrypoints`

Notes:

- ordering may shift slightly to preserve compile-time dependency flow,
- section titles should match actual content labels used in file,
- avoid micro-file splitting beyond common-vs-family boundary.

---

## 8) Staged migration plan (actionable)

### P12 M1 (this milestone)

- Complete topology audit.
- Publish this migration plan.
- No behavior changes.

### P12 M2

- Introduce `reactor_vulkan_common.c` and evolve `reactor_vulkan.h` for shared declarations.
- Move only low-risk, clearly generic Vulkan helpers.
- Keep SGEMM policy/runtime fully in existing file.
- Build + Marionette verification.

### P12 M3

- Rename `reactor_vulkan.c` -> `reactor_vulkan_sgemm.c`.
- Update Linux and Windows build scripts and any source manifests.
- Add SGEMM section banners for locality and auditability.
- Preserve impl symbol names and ABI behavior.

### P12 M4

- Add inert stubs:
  - `reactor_vulkan_fft.c`
  - `reactor_vulkan_fused_reduction.c`
- Do not expose new capability flags.

### P12 M5

- Clean remaining references:
  - build script drift,
  - Marionette compile lists,
  - comments/docs mentioning old monolithic filename.
- Verify no stray references to removed file names.

### P12 M6

- Full verification pass:
  - Linux build script,
  - Windows build script parity review,
  - Marionette full run,
  - any bridge loading checks that depend on produced shared library.
- Documentation update finalization.

---

## 9) Risks and mitigations

1. **Static symbol breakage during file split**
   - Risk: moved helpers remain `static` but required cross-file.
   - Mitigation: move with explicit prototype declarations in `reactor_vulkan.h`; compile with warnings-as-errors in CI where available.

2. **Build script drift (Linux/Windows)**
   - Risk: one script updated, the other not.
   - Mitigation: update `build_stub.sh` and `build_windows.cmd` in same PR; add checklist item requiring both compile lists to match family/common files.

3. **Marionette source list omissions**
   - Risk: test binary links missing new reactor files.
   - Mitigation: update both compile invocations (library + marionette) and run Marionette smoke/full suites.

4. **Accidental behavior change**
   - Risk: helper movement alters subtle runtime state flow.
   - Mitigation: M2 limited to low-risk generic helpers; no policy edits; compare diagnostics-sensitive Marionette tests before/after.

5. **`common` file absorbs SGEMM policy**
   - Risk: conceptual boundary erosion.
   - Mitigation: use explicit “common allowlist” from Section 4; reject any move that references SGEMM controller/arena/transfer selector state.

6. **Public ABI symbol rename/regression**
   - Risk: `reactor_api.c` forwarders fail to link if impl symbols renamed.
   - Mitigation: preserve `prom_reactor_runtime_*_impl` names unchanged; only move implementation bodies.

7. **Include-order / Vulkan macro breakage**
   - Risk: `_WIN32`/`WIN32_LEAN_AND_MEAN`/`NOMINMAX` and Vulkan include order changes cause platform compile regressions.
   - Mitigation: keep platform include block centralized and consistent, then refactor incrementally with build checks on both toolchains.

8. **LLM audit locality degradation by over-splitting SGEMM**
   - Risk: SGEMM logic fragmented across too many files, harming maintainability.
   - Mitigation: keep SGEMM in one family file with section banners; split only truly cross-family Vulkan plumbing.

9. **Header bloat (`reactor_vulkan.h` becomes god-header)**
   - Risk: leaked internals and coupling.
   - Mitigation: expose minimal cross-file contracts; keep family-private structs/functions in family `.c` files.

10. **Native/non-Vulkan module boundary confusion**
    - Risk: topology migration drifts into Dominatus/Judgment/Policy module rewrites.
    - Mitigation: keep those modules in place; only adjust include/declaration boundaries required by file split.

---

## 10) Non-goals (explicit)

This plan intentionally excludes:

- FFT implementation,
- fused-reduction implementation,
- SGEMM algorithm or policy redesign,
- batch concurrency redesign,
- typed arena semantic changes,
- public API shape changes,
- renaming exported/runtime ABI symbols,
- moving Dominatus modules beyond minimal include/declaration adjustments,
- claiming any new reactor capability.

---

## Acceptance checklist for P12 M1

- [x] Current Vulkan reactor sections audited.
- [x] Target file topology defined.
- [x] Common vs SGEMM ownership clarified.
- [x] FFT/fused-reduction future scope defined as stubs only.
- [x] SGEMM section banner plan defined.
- [x] Staged migration sequence documented.
- [x] Risks + mitigations documented.
- [x] No behavior changes introduced.
