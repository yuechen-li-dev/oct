# P11 M3 / Follow-up — Typed Arena Pools (Native Vulkan)

## 1. M38 handoff summary

M38 moved from static table assertions to executable Oct simulation and reconfirmed the allocator direction:

- typed arena pools by role,
- strict compatibility keys (artifact + namespace + memory class + capacity + ownership),
- conservative hysteresis shrink (not exact-fit rebuild),
- explicit budget rejection paths,
- generation increments on structural capacity/invalidation transitions.

## 2. Follow-up audit (M38 contract coverage)

### 2.1 Audit result before this follow-up

| Contract item | Pre-follow-up status |
|---|---|
| compatible reuse | implemented + partially tested |
| grow on capacity shortfall | implemented + partially tested |
| explicit budget rejection | implemented + partially tested |
| layout namespace mismatch | implemented + partially tested |
| precision namespace mismatch | implemented + partially tested |
| hysteresis shrink | partially implemented (bookkeeping present; direct proof/tests incomplete) |
| no shrink while in-flight | partially implemented (guard path existed, test gap) |
| role-specific generation increments | implemented + partially tested |
| M/N/K per-artifact invalidation under typed arenas | behavior preserved via M14, typed-arena-specific tests missing |
| fixed-double ownership interaction under typed arenas | legacy M29 coverage existed, typed-arena-specific proof gap |
| per-role reuse/grow/shrink/rebuild diagnostics | missing in public diagnostics (aggregate only) |

### 2.2 Follow-up changes

This follow-up adds:

- per-role diagnostics counters (reuse/grow/shrink/rebuild) in `PrometheusSgemmPolicyDiagnostics`,
- explicit grow-only fallback semantics (shrink remains disabled in current runtime path and is asserted by tests),
- explicit no-shrink-while-in-flight rejection accounting,
- focused typed-arena Marionette tests for reuse/grow/budget/layout/precision/shrink/cooldown/in-flight/MNK and diagnostics truthfulness.

## 3. Implemented arena roles

This pass uses fixed typed arenas for:

- `A`
- `B`
- `C`
- `upload/staging`

Readback remains coupled to staged `C` ownership (same lifecycle boundary in current Vulkan runtime path).

## 4. Arena state and compatibility key

Each arena tracks fixed state including:

- role,
- required bytes,
- capacity bytes,
- committed/live bytes,
- generation,
- artifact dependency key components,
- layout namespace,
- precision/storage namespace,
- memory location class,
- owner slot id,
- valid flag,
- in-flight flag,
- last failure reason,
- low-usage epoch count,
- shrink cooldown,
- reuse/grow/shrink/rebuild and rejection counters.

Compatibility gate (reuse legal only when all pass):

- artifact-key compatible,
- layout namespace equal,
- precision namespace equal,
- memory class compatible,
- capacity >= required,
- ownership allows.

## 5. Growth/shrink policy

### 5.1 Growth / rebuild

- reuse on compatible + sufficient capacity + allowed ownership,
- grow/rebuild when incompatibility or short capacity occurs and budget projection allows,
- explicit budget rejection on projected over-budget growth.

### 5.2 Hardening update: shrink gating and paired staged invariants

The hardening pass applies two allocator-edge fixes:

1. **No same-pass grow/rebuild + shrink on the same role**
   - shrink checks now run only for roles that were reused in the current ensure pass,
   - roles that rebuilt/grew are excluded from shrink evaluation until later passes.

2. **Paired staged-buffer shrink now carries an explicit symmetric-size contract**
   - paired shrink helper now receives first/second required byte sizes,
   - helper explicitly guards the current invariant that paired staged buffers must have equal required bytes,
   - mismatch is treated as a guarded failure path (no partial shrink mutation).

This keeps shrink bookkeeping coherent with structural transitions and makes the staged-pair assumption explicit instead of implicit.

## 6. Budget ledger behavior

Arena ledger tracks:

- budget limit bytes,
- per-arena capacity contribution,
- total committed bytes,
- projected committed bytes,
- explicit rejection reason/detail.

Hardening update:

- budget projection is now computed over an **active role mask**:
  - direct path: `A + B + C`
  - staged path: `A + B + C + upload`
- direct-path checks no longer charge inactive upload arena retained capacity.

Budget rejection still emits explicit typed detail (`PROM_DETAIL_ARENA_BUDGET_REJECTED`) and avoids partial allocation.

## 7. Generation semantics

Arena generation increments on:

- grow/rebuild capacity changes,
- shrink capacity changes (when/if shrink is re-enabled in a later pass).

Generation is exported per role in diagnostics and asserted by follow-up tests.

## 8. P10 M14 invalidation compatibility

P10 M14 dependency surfaces are preserved:

- A: `(m, k, compute_or_padded_k, layout, precision, required_bytes)`
- B: `(k, n, compute_or_padded_k, layout, precision, required_bytes)`
- C: `(m, n, layout, precision, required_bytes)`

Follow-up typed-arena tests now explicitly verify M-only / N-only / K-only generation behavior aligned with M14 reuse/invalidation intent.

## 9. Dominatus / diagnostics integration

Diagnostics now expose enough to prove typed-arena behavior:

- per-role capacity/required/generation,
- per-role reuse/grow/shrink/rebuild,
- aggregate grow/shrink/rebuild,
- ownership/namespace/budget rejection aggregates,
- total/projected committed bytes,
- budget limit,
- last arena failure reason.

Integration remains intentionally narrow; no broad Dominatus domain migration beyond relevant memory diagnostics.

## 10. Tests added/strengthened in follow-up

Added or strengthened native Marionette tests for:

1. compatible reuse does not change generation,
2. grow path generation/counter behavior,
3. explicit budget rejection and state preservation,
4. layout namespace mismatch behavior,
5. precision namespace mismatch behavior,
6. grow-only fallback semantics (shrink remains disabled),
7. no shrink while in-flight,
8. M/N/K typed-arena dependency behavior,
9. fixed-double ownership safety compatibility,
10. diagnostics truthfulness.

Hardening-specific additions:

11. rebuild/grow pass does not also report shrink on the same role and generation remains single-step for the structural transition,
12. staged paired-buffer symmetry invariants remain explicit in diagnostics coverage.

## 11. Validation status

- targeted `PrometheusReactor_P11_M3_TypedArenas*` tests pass,
- existing `PrometheusReactor_M14_BufferArtifacts*` behavior remains intact,
- full Marionette native suite passes in this environment (with existing backend-dependent skips where expected).

Hardening follow-up also re-runs:

- `PrometheusReactor_P11_M3_TypedArenas`,
- `PrometheusReactor_M14_BufferArtifacts`,
- `M29_FixedDouble`,
- `Packed4`,
- `FP16`,
- full `marionette_tests`.

## 12. Deferred scope

Still explicitly deferred:

- generic VMA-style allocator,
- shape-class slab layer,
- sparse virtual buffers,
- N-slot/work-stealing allocator expansion,
- memory compaction,
- cross-runtime arena sharing,
- FFT arena migration,
- public allocator API redesign.

## 13. Inconsistency / gap note

M38 requested a typed budget rejection *event payload* with role/owner/projection/recovery action. This follow-up exposes typed detail + diagnostics counters/bytes and per-role arena counters, but does not yet introduce a separate public event object schema.
