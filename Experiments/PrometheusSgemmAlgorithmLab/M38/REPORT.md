# P11 M2 / Prometheus SGEMM Algorithm Lab M38 — Typed Arena Pool Rake Lab

## 1) M37 handoff summary

M37 selected typed arena pools per buffer role as the first implementation target because they matched Prometheus artifact roles (A/B/C/staging/readback), fit P10 dirty-key invalidation semantics, and achieved strong reuse without introducing generic allocator fragility.

Required restatement from M37:

1. **Why typed arena pools won M37**: they gave the best balance of low missed-invalidation risk, high deterministic reuse, and moderate implementation complexity for fixed-double moving toward N-slot.
2. **Why generic VMA-like allocation is deferred**: general-purpose block management introduced fragmentation/freelist policy burden before role-specific ownership contracts were proven.
3. **Why shape-class slabs are deferred**: they can help on clustered workloads but need stable shape clusters first; otherwise they risk slack waste and policy complexity.
4. **Why sparse binding is research-only**: it carries high synchronization/feature risk and is disproportionate to near-term ownership correctness needs.
5. **What M38 must prove before implementation**: typed arenas need explicit lifecycle contracts for growth, shrink, reuse, invalidation compatibility, ownership transitions, and budget failure/recovery.

## 2) Typed arena model

M38 modeled typed arenas per role:

- A arena
- B arena
- C arena
- staging/upload arena
- readback arena (included for completeness)

Each arena tracked:

- `role`
- `required_bytes`
- `capacity_bytes`
- `committed_bytes`/`live_bytes`
- `generation`
- `artifact_dependency_key`
- `layout_namespace`
- `precision_storage_namespace`
- `memory_location_class`
- `owner_slot_id`
- `valid`
- `failure_state`

### Reuse rule (enforced)

Reuse was legal only when all conditions held:

- artifact key compatible
- capacity >= required bytes
- memory type/location compatible
- ownership permits reuse (not in-flight by another owner)
- layout namespace compatible
- precision/storage namespace compatible

Otherwise the model forced explicit grow, rebuild, or failure.

## 3) Candidate arena policies

Compared exactly the required candidate set:

- **A: Grow-only arena**
- **B: Grow with hysteresis shrink**
- **C: Exact-fit rebuild**
- **D: Budget-aware grow-only**
- **E: Slot-local typed arenas (optional preview)**

Policy B used conservative shrink hysteresis:

- shrink only when `capacity > 2x required` for 6 consecutive low-usage epochs,
- cooldown of 4 epochs between shrinks,
- no shrink while arena is in-flight,
- minimum floor of 64 MiB per role.

## 4) Workload scenarios

M38 executed the required compact scenario set:

1. steady same-shape
2. M-only churn
3. N-only churn
4. K-only churn
5. layout churn (Scalar ↔ Packed4)
6. precision churn (FP32 ↔ FP16 storage)
7. capacity growth then shrink
8. fixed-double two-slot handoff
9. future N-slot preview (4 and 8)
10. budget pressure/failure

## 5) Metrics

Collected structural metrics required by the milestone:

- capacity bytes per role
- required bytes per role
- committed/live bytes
- slack bytes + slack ratio
- grow/shrink/rebuild/reuse/invalidation counts
- budget rejection/failure/recovery counts
- generation count behavior
- peak + average committed memory
- false invalidation avoided
- missed invalidation hazard
- layout/precision mismatch rejection count
- N-slot projected arena count
- implementation complexity score

## 6) Rake findings

### 6.1 Unsafe same-shape/different-representation reuse

No missed invalidation hazards were observed when compatibility keys included both `layout_namespace` and `precision_storage_namespace`. All representation mismatches were rejected explicitly.

### 6.2 Undersized capacity reuse

All policies except exact-fit rebuild rejected undersized reuse and performed grow/rebuild/fail explicitly. No silent undersized reuse was observed.

### 6.3 Over-eager shrink thrash

Exact-fit rebuild (Policy C) thrashed heavily under oscillating workloads. Conservative hysteresis (Policy B) avoided shrink/grow ping-pong.

### 6.4 Memory hoarding

Grow-only (Policy A) retained excess memory after peaks (higher average slack). Policy B reduced retained slack while maintaining high reuse.

### 6.5 P10 per-artifact invalidation compatibility

Artifact dependency rake matched P10 M14 dependency intent: A/B/C invalidated only when their dependency keys changed. No missed invalidation hazards were found.

### 6.6 Slot ownership violation

Fixed-double handoff and 4/8-slot previews showed zero ownership overwrite violations when reuse required owner token + in-flight completion.

### 6.7 Budget failure ambiguity

Budget-aware policy produced explicit typed rejection events; no silent partial allocations were observed.

### 6.8 N-slot multiplication

Arena count scaled linearly with slot count (2→4→8 => 10→20→40 role arenas). Budget pressure increased materially at 8 slots; contract remains tractable with per-slot budget ledgering.

## 7) Winning policy recommendation

**Recommended first native implementation policy**:

- **Typed arenas with grow + conservative hysteresis shrink (Policy B)**,
- with **budget-aware explicit rejection path (Policy D behavior) as mandatory guardrail**.

Rationale:

- avoids Policy C thrash,
- controls Policy A memory hoarding,
- keeps contract implementation complexity bounded,
- preserves deterministic compatibility with P10 dirty-key invalidation.

## 8) Implementation contract

### 8.1 Exact reuse conditions

Reuse allowed only if:

`artifact_key_compatible && layout_namespace_equal && precision_storage_namespace_equal && capacity_bytes >= required_bytes && memory_location_compatible && ownership_allows`

### 8.2 Exact grow/rebuild/failure conditions

- **Grow** when `required_bytes > capacity_bytes` and projected budget remains within limit.
- **Rebuild** when memory location/type changes, required namespace is incompatible, or policy requires physical replacement.
- **Fail explicitly** when projected commit exceeds budget, allocation fails, or ownership conflict prevents safe reuse/grow.

### 8.3 Namespace representation

Compatibility key must include:

- role
- artifact dependency key
- layout namespace (`scalar`, `packed4`, ...)
- precision/storage namespace (`fp32-storage`, `fp16-storage-fp32-accum`, ...)
- memory location class (`device-local`, `host-visible-staging`, `readback`)

### 8.4 Arena generation semantics

Generation increments on:

- grow,
- rebuild,
- explicit invalidation,
- recovery allocation after budget rejection.

Consumers must verify generation equality before reuse; generation mismatch triggers rebind/revalidate.

### 8.5 Budget rejection reporting

Budget rejection should return a typed event containing:

- role,
- owner slot,
- required bytes,
- current capacity,
- projected committed bytes,
- budget limit,
- generation,
- selected recovery action (`fail`, `retry-after-shrink`, `serial-fallback`, `defer`).

## 9) Risks to test in native implementation

1. Vulkan allocation granularity vs modeled byte accounting.
2. Fence/timeline completion edge cases before ownership handoff.
3. Cross-queue staging/readback ownership transitions.
4. Shrink policy safety under prolonged asynchronous in-flight workloads.
5. Deterministic budget ledger updates under concurrent slot pressure.

## 10) Deferred scope

Deferred (unchanged from M37 intent):

- generic VMA-like allocator implementation,
- shape-class slab layer,
- sparse binding production path,
- N-slot/work-stealing runtime behavior.

## 11) Executable simulation backstop (.oct / .octest)

To back the M38 claims with executable artifacts, this milestone now includes:

- `prometheus_sgemm_algorithm_lab_m38.oct` (typed-arena simulation data model and contract records),
- `prometheus_sgemm_algorithm_lab_m38.octest` (contract assertions and rake checks across scenarios).

Executed command:

- `go run ./cmd/oct test Experiments/PrometheusSgemmAlgorithmLab/M38`

Observed result:

- 6 passed, 0 failed.

Impact on findings:

- No recommendation changes were required after execution.
- Policy B (grow + conservative hysteresis shrink) remains the first implementation recommendation.
- Exact-fit rebuild remains rejected due to churn/thrash behavior.
- Namespace-gated compatibility + explicit budget failure handling remain required.

## Final answers required by milestone

1. **Which typed arena policy should Prometheus implement first?** Policy B (grow + conservative hysteresis shrink) with explicit budget-aware rejection behavior.
2. **Should arenas be grow-only, exact-fit, or hysteresis-shrinking?** Hysteresis-shrinking (conservative) is preferred; grow-only remains acceptable fallback; exact-fit is rejected due to churn thrash.
3. **What are the exact reuse conditions?** Artifact key + layout namespace + precision/storage namespace + memory location compatibility + capacity sufficiency + ownership availability.
4. **What are exact grow/rebuild/failure conditions?** Grow on required>capacity within budget; rebuild on incompatible namespace/location changes; explicit failure on budget/allocation/ownership conflict.
5. **How should layout/precision namespaces be represented?** As first-class fields in the compatibility key and invalidation key, never implicit shape-only derivation.
6. **How should arena generations work?** Monotonic per-arena generation increments on grow/rebuild/invalidate/recovery; consumers require generation match.
7. **How should budget rejection be reported?** Typed explicit rejection event with role/owner/bytes/budget/generation and mandated recovery mode.
8. **How does policy interact with fixed-double?** Slot ownership token + in-flight completion gate prevents overwrite during two-slot handoff.
9. **What changes for future N-slot?** Contract is unchanged; per-slot arena multiplicity and budget pressure increase, requiring stricter ledger guardrails.
10. **Is another rake lab needed before native implementation?** Not mandatory for contract definition; a small pre-native validation pass is still useful for Vulkan granularity and queue handoff edge cases.
11. **What should P11 M3 / M39 implement?** Native Vulkan typed arenas for A/B/C + staging (readback optional), implementing this exact compatibility, generation, and budget rejection contract.

## Inconsistency/documentation-gap check

No direct contradiction with `Language/reference` syntax/style guidance was encountered because this milestone is artifact/report modeling work, not new Oct language semantics. If native Vulkan implementation in M39 diverges from this contract, that divergence must be surfaced explicitly.
