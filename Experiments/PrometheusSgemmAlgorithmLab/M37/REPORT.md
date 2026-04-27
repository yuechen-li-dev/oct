# P11 M1 / Prometheus SGEMM Algorithm Lab M37 — Memory Ownership Strategy Lab

## 1) Why memory ownership matters now

Prometheus has already moved beyond a single static SGEMM path. The reactor now carries fixed-double buffering, transfer-queue support, Packed4FP32, FP16StorageFP32Accum, Dominatus ownership bookkeeping, per-artifact invalidation, and dirty-key selector caching. The next scaling step (N-slot/work-stealing) multiplies live artifacts and transition frequency, so memory ownership can no longer remain an implicit side effect of "create buffer when needed".

### Why this lab exists (required framing)

1. **Why memory ownership matters now**: buffer lifecycle decisions now directly affect correctness (invalidation), scheduling (slot readiness), and budget behavior under mode churn.
2. **Why status quo is insufficient for N-slot/work-stealing**: per-buffer allocation scales allocation/reallocation events roughly with slot count and churn; N-slot turns this into ownership noise and failure risk.
3. **Why Prometheus allocation is not generic malloc/VMA**: artifacts are role-typed (A/B/C/staging/readback), dependency-keyed, and driven by deterministic shape/layout/precision transitions, not arbitrary object lifetimes.
4. **Connection to P10 dirty-key/per-artifact invalidation**: P10 made invalidation precise by artifact dependency keys; memory ownership must preserve those same keys or it reintroduces false invalidations and stale reuse hazards.
5. **What M37 must answer**: choose the first implementation direction for memory ownership before N-slot/work-stealing, with enough structural evidence to drive P11 M2 / M38.

## 2) Candidate strategies

Compared candidates:

- **A — Status quo per-buffer allocation** (baseline).
- **B — Generic block suballocator / VMA-like model** (conceptual, not implemented).
- **C — Typed arena pools per buffer role**.
- **D — Shape-class slabs**.
- **E — Sparse virtual buffers / demand-paged binding** (conceptual research candidate).

## 3) Workload scenarios

M37 model covers seven required scenarios:

1. steady same-shape,
2. single-dimension churn (m-only/n-only/k-only),
3. layout/precision churn,
4. fixed-double two-slot,
5. future N-slot (4/8),
6. memory-constrained pressure,
7. shape-cluster reuse.

## 4) Metrics

Structural metrics collected for each candidate:

- peak committed memory,
- average committed memory,
- internal slack,
- allocation count,
- reallocation/growth count,
- invalidation count,
- reuse count,
- false invalidation avoided,
- missed invalidation risk,
- fragmentation risk,
- Vulkan feature requirement risk,
- synchronization risk,
- implementation complexity score,
- N-slot readiness score,
- Dominatus/dirty-key fit score.

These are model-level infrastructure metrics (not hardware timings).

## 5) Findings by candidate

### A) Status quo per-buffer allocation

- Strength: lowest conceptual complexity.
- Weakness: highest allocation/reallocation volume and weakest N-slot scaling.
- Verdict: keep only as baseline/reference.

### B) Generic block suballocator (VMA-like)

- Strength: better memory packing than status quo.
- Weakness: introduces generic fragmentation/freelist policy concerns that do not align tightly with role-typed Prometheus artifacts.
- Why not first: added allocator-generality burden before proving role-specific ownership contracts.
- Verdict: defer unless typed arenas fail requirements.

### C) Typed arena pools per role

- Strength: best balance of reuse, bounded complexity, low missed-invalidation risk, and strong Dominatus dirty-key fit.
- Strength: directly mirrors Prometheus artifact model (A/B/C/staging/readback + layout/precision namespaces + capacity classes).
- Verdict: **first implementation target**.

### D) Shape-class slabs

- Strength: excellent reuse in clustered shape families.
- Weakness: slack waste increases when shape distribution drifts.
- Verdict: useful second-layer optimization, not first ownership substrate.

### E) Sparse virtual buffers

- Strength: low committed bytes and low allocation events in principle.
- Weakness: high feature/synchronization risk and high implementation complexity for near-term product needs.
- Verdict: research branch only.

## 6) Implementation recommendation

### Final answers

1. **Which strategy first?** Typed arena pools per buffer role.
2. **Under what assumptions?** Dominatus remains source-of-truth for dirty keys, P10 per-artifact invalidation contracts remain authoritative, and near-term path is fixed-double advancing to moderate N-slot.
3. **What should be deferred?** Generic VMA-like allocator as first product step; keep as contingency if typed arenas miss targets.
4. **Is shape-class slab worth follow-up?** Yes, but only after typed arenas are landed and workload corpus confirms stable class clusters.
5. **Is sparse binding worth follow-up?** Yes only as research; no for near-term product implementation.
6. **What should P11 M2 / M38 do next?** Implement typed arenas (A/B/C/staging/readback optional), capacity-class reuse keys, growth/failure policies, and budget ledgering with churn-focused rakes.
7. **What should absolutely not be implemented yet?** Sparse binding production path, full generic allocator framework, and N-slot/work-stealing runtime behavior changes.

## 7) Risks / rakes for next milestone (P11 M2 / M38)

Required rakes before declaring ownership-ready:

- typed-arena growth/shrink hysteresis,
- byte-accurate capacity reuse correctness,
- per-artifact invalidation compatibility with P10 dirty keys,
- layout/precision churn isolation,
- fixed-double to N-slot ownership transitions,
- failure/recovery under budget pressure,
- deterministic memory budget accounting.

## 8) Deferred ideas

- Shape-class slab layer on top of typed arenas, contingent on real workload clusters.
- Sparse virtual binding branch for longer-horizon research.
- Generic suballocator path only as fallback if typed arenas prove insufficient.

## Inconsistency/documentation gap check

No direct conflict with current `Language/reference` syntax/style guidance was encountered for this milestone's Oct artifacts. If future native implementation details diverge from the modeled ownership contracts here, that should be surfaced explicitly in P11 M2 / M38.
