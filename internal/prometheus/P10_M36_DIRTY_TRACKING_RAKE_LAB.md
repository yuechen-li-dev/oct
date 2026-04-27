# P10 M36 — Dirty Tracking Rake Lab for Buffer Invalidation, Judgment Facts, and Slot Readiness

## 1) Current dirty-tracking state (required first step)

### 1.1 What dirty keys/domains/slots currently track

Dominatus already has staged/visible state, domain taxonomy, slot-scoped keys, event/trace rings, and last-commit dirty queries.

- Domains include SGEMM, SLOT, QUEUE, MEMORY, DIAGNOSTICS, FFT, and ASYNC.
- Key catalog includes SGEMM fact/judgment keys, slot lifecycle and invalidation counters, queue policy keys, memory keys, and async lifecycle keys.
- Slot-scoped key metadata already marks per-slot keys (`slot_scoped=1`) so slot-local dirtiness can be isolated.
- Runtime event kinds already include readiness/lifecycle transitions (`SLOT_READY`, `SLOT_COMPLETE`, `SLOT_FAILED`, `SLOT_CONSUMED`, `SLOT_CLEANUP`, `SLOT_INVALIDATED`).

### 1.2 What P10 M13 selector caching already implemented

M13 implemented dirty-key dependency caching for two selectors only:

1. M35 buffering selector.
2. Transfer-policy selector.

Reuse contract is already explicit: cache valid + visible projection + dependent dirty mask for last commit equals zero (+ transfer path-input guard for transfer selector). Generation-only change does not force recompute.

### 1.3 Why the three remaining applications are distinct

1. **Buffer invalidation** controls correctness and lifetime of allocation/reuse artifacts (A/B/C or packed/precision variants).
2. **Judgment fact caching** controls recompute lifetime for selector decisions over visible facts.
3. **Slot readiness dirty tracking** controls scheduler attention lifetime (which slots must be revisited at decision boundaries).

These are different ownership surfaces (memory artifacts vs judgment outputs vs scheduling state), so a single generic "dirty-tracking" extension is unsafe.

### 1.4 Why model before implementation

Without a dependency contract first, implementation risks:

- false invalidation churn (performance regression),
- missed invalidation (correctness failure),
- stale selector reuse (wrong path/policy decision),
- readiness polling blow-up or missed-ready hazards under future N-slot concurrency.

M36 is therefore a pure contract/rake milestone that narrows native work to deterministic, testable rules.

## 2) Track A — Buffer invalidation findings

### 2.1 Correct dependency keys

- **A buffer key** = `(m, k, compute_or_padded_k, layoutA, precisionA, required_bytesA)`
- **B buffer key** = `(k, n, compute_or_padded_k, layoutB, precisionB, required_bytesB)`
- **C buffer key** = `(m, n, layoutC, precisionC, required_bytesC)`

### 2.2 Invalidation mapping

- `m` change invalidates A and C, not B.
- `n` change invalidates B and C, not A.
- `k` change invalidates A and B; C only if output representation/capacity also changes.
- layout change invalidates all artifacts whose layout encoding changes.
- precision/storage change invalidates artifacts whose interpretation or byte width changes.
- capacity growth invalidates any artifact that no longer satisfies required bytes even if logical dims are unchanged.

### 2.3 Dimension-specific invalidation benefit

Yes—dimension-specific invalidation is materially useful.

Rake cases show one avoidable false invalidation whenever only `m` or only `n` changes under coarse `(m,n,k)`-style keys. Per-artifact masks remove this churn while preserving zero missed invalidations in the modeled cases.

### 2.4 Correctness hazards to guard

1. Reusing by logical shape hash alone (missing required-bytes key) can cause undersized buffer reuse.
2. Packed4/FP16/scalar transitions require representation namespace in the cache key; scalar fallback cannot reuse packed layout storage blindly.
3. `k`-derived padded/compute-K changes can break A/B reuse even when logical K appears unchanged at API level.

## 3) Track B — Judgment fact / selector cache findings

### 3.1 Selector cache statuses

- **Safe now (already implemented):** M35 buffering selector, transfer-policy selector.
- **Safe next:** layout/precision selector (with explicit dependency mask).
- **Defer:** full path/compute selector caching.

### 3.2 Required dirty dependency masks

- **M35 mask:** memory budget, required slots, per-mode headroom, variance, predictability, starvation risk, WIP waste, fallback availability.
- **Transfer mask:** queue families, dedicated availability, transfer support/disable gates, workload threshold, sync ownership, fallback/upload flags.
- **Layout/precision mask (next):** shape-family, layout keys, precision keys, packed4/fp16 eligibility facts, tolerance/fallback facts.
- **Path/compute (defer):** composed mask spanning path policy + layout/precision + transfer + buffering slices.

### 3.3 Why not "cache everything"

Path/compute selector composes other slices and is higher hazard if a sub-slice dirty event is omitted or mis-coalesced across multiple commits. M36 recommends deferring until composed dependency proof + multi-commit coalescing guards are explicit.

## 4) Track C — Slot readiness dirty tracking findings

### 4.1 Readiness representation

Use **per-boundary dirty-slot mask** + optional **spill-list** when slot count exceeds bit width.

At each decision boundary compute:

- dirty slots,
- ready slot set,
- failed slot set,
- attention set (ready ∪ failed ∪ invalidated).

### 4.2 Transitions that must be tracked

- `preparing -> ready`
- `in-flight/submitted -> complete/consumed`
- `* -> failed`
- `cleanup -> empty`
- explicit invalidation transitions

### 4.3 Scheduler work reduction

Yes, materially.

In modeled N-slot cases (4/8/16 style), dirty-set lookup avoids O(N) full scans when only a few slots changed. The rake table shows significantly higher avoided polling in sparse-dirty scenarios.

### 4.4 Hazards

1. Missed dirty bit -> slot starvation or missed failure cleanup.
2. Duplicate dirty emission -> double scheduling or repeated cleanup.
3. Overflow without spill handling -> dropped readiness updates.
4. Multi-commit coalescing must preserve terminal fail/invalid states and latest generation.

## 5) Cross-track interactions

1. Buffer invalidation must emit slot-impact dirty signals for readiness/scheduler logic when a prepared slot is invalidated by shape/layout/precision/capacity change.
2. Layout/precision selector cache (next) must align with buffer invalidation keys to avoid stale selector choosing a representation whose buffers were invalidated.
3. Path/compute caching should remain deferred until buffering + layout/precision cache contracts are stable.

## 6) Implementation recommendations

1. Implement **buffer artifact dependency invalidation** next (A/B/C scoped keys + byte-capacity class).
2. Extend selector caching to **layout/precision** after buffer invalidation is in place.
3. Finalize slot readiness dirty-mask ABI/protocol before N-slot/work-stealing native implementation.
4. Keep full path/compute cache deferred.

## 7) Next milestone ordering

1. **Next P10 implementation milestone:** native buffer invalidation contract from M36 Track A.
2. **Then:** layout/precision selector cache extension with strict dirty mask.
3. **Before N-slot/work-stealing:** implement slot readiness dirty-mask tracking and boundary-coalescing semantics.
4. **Deferred:** full path/compute cache, scheduler concurrency/work-stealing, allocator redesign.

## 8) Required final answers

1. Correct A/B/C dependency keys are the per-artifact contracts listed in §2.1.
2. Shape/layout/precision invalidation mapping is listed in §2.2.
3. Yes, dimension-specific invalidation reduces unnecessary rebuilds (see §2.3).
4. Native guardrails are listed in §2.4.
5. Safe to cache now: M35 and transfer.
6. Cache next: layout/precision selector.
7. Keep deferred: full path/compute selector.
8. Required dirty masks are listed in §3.2.
9. Slot readiness should use dirty-slot mask + overflow spill list (§4.1).
10. Yes, it materially reduces future N-slot scheduler work (§4.3).
11. Required transitions are listed in §4.2.
12. Implement readiness dirty-mask/boundary coalescing before N-slot/work-stealing (§7).
13. Next milestone: buffer invalidation implementation (§7).
14. Deferred items: full path/compute caching, scheduler concurrency, allocator redesign (§7).

## 9) Inconsistency/documentation-gap note

Current Dominatus key catalog includes broad SGEMM/slot/memory keys and invalidation counters, but it does not yet define first-class per-artifact buffer dependency keys (`A/B/C` dependency surfaces) as explicit contracts. M36 formalizes this gap as implementation guidance.
