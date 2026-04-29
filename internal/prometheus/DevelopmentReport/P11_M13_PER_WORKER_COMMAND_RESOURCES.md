# P11 M13 — Per-Worker Command Resource Ownership with Serialized Submit

## 1) M42 handoff summary

M42 recommended Candidate **B** as the next native step: per-worker command/resource ownership with a serialized single-queue submit bridge, explicitly deferring true multi-queue execution. M13 follows that recommendation.

## 2) Implemented resource ownership model

M13 adds explicit per-worker command resource ownership identities in the batch runtime path while keeping submission serialized.

Ownership is represented by worker-local runtime resource records and exported through batch diagnostics.

## 3) Per-worker resource bundle

Each worker now carries explicit identities and state for:

- worker id,
- queue mapping identity,
- queue family metadata,
- command pool identity,
- command buffer identity,
- fence/completion identity,
- slot identity,
- output staging identity,
- arena-bank identity,
- submit count,
- wait count,
- in-flight state,
- failure marker.

In this milestone, command pool/command buffer/fence are modeled as deterministic per-worker ownership identities in diagnostics-first form (simulated identity contract), with serialized submit still enforced.

## 4) Serialized submit bridge

M13 preserves serialized submit:

- workers prepare/execute under worker-owned resource identity,
- queue submit remains gated by serialized bridge mutex,
- maximum concurrent serialized submit remains constrained to <= 1,
- hardware parallelism claimed remains false.

Diagnostics now include serialized bridge enter count, serialized wait count, serialized execution count, and per-worker submit/wait/in-flight state.

## 5) Queue mapping diagnostics

M13 introduces explicit worker→queue mapping diagnostics:

- queue topology classification,
- queue mapping mode,
- per-worker queue assignment index.

Current behavior:

- single-queue topology reports single-queue serialized mapping,
- pseudo/shared topology reports per-worker mapped but serialized,
- parallel-eligible mapping remains diagnostic/deferred in this milestone.

## 6) Command resource ownership checks

M13 introduces ownership checks to prevent hidden cross-worker resource use:

- worker resource owner id is validated before use,
- wrong-owner access increments ownership violation diagnostics,
- wrong-owner access fails batch with explicit detail,
- output commit remains false on ownership violation failure.

## 7) Slot/arena ownership seam

M13 makes worker-local slot/output/arena ownership identities explicit in diagnostics.

No cross-worker slot borrowing or cross-worker arena borrowing behavior is introduced.

## 8) Failure/drain behavior

M11 failure/drain behavior is preserved:

- first failure wins,
- workers stop starting new work after failure observed,
- in-flight work drains,
- output commit remains false on failure,
- destroy-after-failure remains safe.

M13 adds ownership-violation failure as an explicit batch failure path with per-worker failure metadata surfaced.

## 9) Dominatus boundary

Unchanged from M10/M11:

- workers still do not run judgment,
- workers still do not mutate Dominatus directly,
- workers execute immutable plans and emit worker-local events,
- drain layer remains commit boundary.

## 10) Tests added

Added Marionette coverage for:

1. per-worker command resource identity + queue mapping diagnostics,
2. serialized bridge enter diagnostics,
3. wrong-owner command resource rejection with explicit detail and uncommitted output.

Existing P11 M6/M7/M8/M10/M11 tests remain and are executed as compatibility coverage.

## 11) What remains deferred

Deferred unchanged:

- true multi-queue parallel submit,
- per-worker independent queue execution,
- SPMC/MPMC queues,
- work stealing,
- lock-free queueing,
- parallel judgment,
- shared arena lock runtime path,
- public event stream,
- performance tuning.

## Language/reference consistency note

This milestone modifies native runtime/tests and diagnostics only; no `Language/reference` semantic contract changes were made.
