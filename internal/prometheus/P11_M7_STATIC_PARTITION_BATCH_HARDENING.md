# P11 M7 — Static Partition Batch Skeleton Hardening Pass

## 1) M6 handoff summary

M6 established a first native static-partition batch path with:

- public/native batch API + diagnostics surface,
- immutable per-entry plans (`plan_generation = 40`),
- central single-threaded planning and policy decisions,
- effective worker calculation with requested/hardware/memory caps,
- deterministic round-robin default partitioning with optional contiguous partitioning,
- worker event-ring accounting with explicit critical overflow failure,
- atomic failure progression and drain-oriented terminal failure,
- ordered caller-visible output commit only on full success,
- explicit batch diagnostics,
- isolated batch path preserving existing single-SGEMM entry-point behavior.

## 2) M6 skeleton audit and classification

### 2.1 Dispatch plan construction
- **Status:** implemented and tested.
- **Notes:** M6 builds immutable plans centrally, stamps generation `40`, and performs shape/pointer validation before execution.

### 2.2 Worker assignment
- **Status:** implemented but under-tested (M6 baseline).
- **M7 hardening:** contiguous assignment determinism now covered with targeted failure injection and worker-id assertions.

### 2.3 Worker cap calculation
- **Status:** implemented but under-tested (M6 baseline).
- **M7 hardening:** explicit hardware-cap, memory-cap, zero-worker memory failure, and single-queue conservative reason coverage added.

### 2.4 Event-ring accounting
- **Status:** implemented but under-tested (M6 baseline).
- **M7 hardening:** explicit worker-event drain helper added; ring drain remains truthful and diagnostics-visible for success and failure.

### 2.5 Failure/drain state transitions
- **Status:** implemented but under-tested (M6 baseline).
- **M7 hardening:** late-failure path strengthened; cleanup now normalizes unexpected error exits into explicit fail/drain/fail terminal state with coherent diagnostics.

### 2.6 Output staging / commit logic
- **Status:** implemented and partially tested (M6 baseline).
- **M7 hardening:** single-entry equivalence and late-failure atomic no-partial-commit behavior covered.

### 2.7 Diagnostics publication
- **Status:** implemented but under-tested (M6 baseline).
- **M7 hardening:** wider truthfulness assertions added for requested/effective workers, cap reasons, state/failure fields, overflow/drain counters, commit flag, and plan generation.

### 2.8 Dominatus/event-staging touchpoints
- **Status:** skeleton-only and documented.
- **M7 hardening:** explicit drain classification seam added to distinguish diagnostics-only event usage vs deferred Dominatus staging candidates.

### 2.9 Single-SGEMM compatibility
- **Status:** implemented and tested.
- **M7 hardening:** direct single-entry batch-vs-single-path numerical equivalence test added.

## 3) Dominatus / event-staging status (gap narrowed, not closed)

M7 intentionally keeps M6 ownership boundaries:

- workers still do not mutate Dominatus directly,
- workers still do not run judgment,
- event rings remain the worker lifecycle evidence channel,
- drain remains the ownership boundary for future staged publication.

M7 clarifies this with a dedicated internal event-drain seam:

- `batch_event_destination_for_kind(...)` classifies each worker event as either:
  - **diagnostics-only now**, or
  - **Dominatus-deferred candidate** for later batch lifecycle staging.
- `batch_drain_worker_events(...)` drains worker rings into a summary without introducing worker-side blackboard writes.

This narrows ambiguity while avoiding premature Dominatus key/API expansion.

### Deferred Dominatus contract (explicit)

Still deferred beyond M7:

1. durable Dominatus batch lifecycle key schema,
2. commit-time promotion of staged batch event facts into visible blackboard state,
3. external/public stream for batch lifecycle event publication.

## 4) Tests added/strengthened

Added/strengthened Marionette coverage for:

1. single-entry batch equivalence with single SGEMM path,
2. contiguous static partition determinism and ordered/no-commit semantics on failure,
3. hardware-cap worker selection reason correctness,
4. memory-cap worker selection reason correctness,
5. zero-worker memory failure with uncommitted output,
6. late failure after prior progress with no partial caller-visible commit,
7. overflow behavior preserving explicit failure and no commit (existing M6 test retained),
8. workers do not run judgment and do not mutate Dominatus policy counters,
9. diagnostics truthfulness on success/failure paths.

## 5) Diagnostics hardening

M7 tightened diagnostics coherence by:

- preserving truthful cap-reason semantics (`NONE` when uncapped, conservative single-queue vs hardware vs memory cap reasons when capped),
- ensuring non-success cleanup paths still report explicit failed terminal state and coherent failure stage/detail,
- computing drain count via drained worker events rather than inferred sums.

## 6) Behavior intentionally unchanged

M7 intentionally does **not** change:

- batch API surface shape,
- immutable plan generation (`40`),
- centralized policy planning model,
- static partition runtime model,
- single-SGEMM execution semantics.

## 7) Deferred scope

Still deferred:

- real multi-queue execution,
- worker threading expansion beyond skeleton sequencing,
- SPMC / MPMC queue machinery,
- work stealing,
- parallel judgment,
- N-slot stealing scheduler,
- performance tuning,
- external/public batch event stream API.

## 8) Validation results

Validation performed via Marionette targeted batch suite plus existing SGEMM and full native suite runs after patching.

## Language/reference consistency note

No intentional Oct language semantic additions were made in this hardening pass. Native C/C++ changes target runtime implementation behavior and diagnostics only.
