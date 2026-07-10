# Prometheus R2b — immutable SGEMM batch planning

Status: implemented. R2b creates an in-file logical planning seam for the existing M31 Vulkan batch path. It does not redirect P11, change public ABI, change M29/M30/M30a ownership, alter selectors, or introduce a generic job abstraction.

## Result

M31 now builds one prom_sgemm_batch_plan before any physical admission. The plan is the source of truth for caller-order entry identity, dimensions, checked element/byte counts, requested/planned logical width, partition policy, logical lane, direct-baseline dispatch facts, and plan generation. M31 then consumes that immutable record to create M30 task records and M29 submissions.

The retained P11 prom_batch_plan and worker executor are unchanged. Routing is unchanged: zero flags and the existing narrow fail-entry seam reach M31; ordinary nonzero P11 controls still reach P11.

## Exact plan types

The internal reactor_vulkan.h declarations are:

    typedef struct prom_sgemm_batch_entry_plan {
      uint32_t entry_id;
      uint32_t logical_lane;
      uint32_t plan_generation;
      uint32_t m, n, k;
      size_t a_element_count, b_element_count, c_element_count;
      size_t a_byte_count, b_byte_count, c_byte_count;
      const float* a;
      const float* b;
      float* c;
      uint32_t selected_path, compute_mode;
      uint32_t requested_variant, executed_variant;
      uint32_t planning_flags;
    } prom_sgemm_batch_entry_plan;

    typedef struct prom_sgemm_batch_plan {
      uint32_t entry_count;
      uint32_t requested_logical_width;
      uint32_t planned_logical_width;
      uint32_t partition_policy;
      uint32_t plan_generation;
      prom_sgemm_batch_entry_plan* entries;
    } prom_sgemm_batch_plan;

Construction and destruction use prom_sgemm_batch_plan_build and prom_sgemm_batch_plan_destroy. The records are SGEMM-specific; no operation union, vtable, registry, queue descriptor, FFT field, or generic job type was added.

## Ownership and lifetime contract

1. M31 builds the plan before creating task records or acquiring a physical slot.
2. The builder validates all entries and checked byte counts. M31 allocates every caller-output staging buffer after planning and before admission.
3. The plan owns its entry-record allocation only. A/B/C pointers are immutable caller identities, not owned buffers.
4. The plan stays alive through all admission, completion, drain, reap, and ordered commit work. It is destroyed only after all M31 task references are released.
5. No physical ring slot owns or mutates a plan entry. No plan owns a command buffer, fence, queue, slot, task pointer, staging buffer, quarantine state, or runtime-global policy state.
6. M31 copies batch_entry_id and batch_plan_generation into an admitted M30 task and checks both before completion attribution. Task-table reuse cannot become batch-entry identity.

## Deterministic planning

Entry ID is the caller-order index. The plan generation is the immutable plan schema generation 1 for this R2b record shape; all entries in one batch share it.

Requested logical width is the current low-byte v1 hint, with zero decoded as one by the existing helper. Planned logical width is min(requested logical width, 8). It is not derived from a Vulkan queue or ring depth. This preserves current M31 diagnostics' bounded logical lane arrays without changing routing.

For planned width W and entry count N:

- Round-robin: logical lane equals entry ID modulo W.
- Contiguous: logical lane equals floor(entry ID times W divided by N).

N less than W leaves some lanes empty; entries remain in caller order. A requested width above eight clamps only planned logical width. A single entry maps to lane zero. The public API rejects empty batches before planning.

## Validation and M31 integration

The builder rejects null entry arrays, zero entries, null A/B/C pointers, zero dimensions, multiplication overflow, byte-count overflow, and invalid logical width. It reports the caller-order failing entry. It does not submit Vulkan work.

M31 uses plan counts for task-buffer allocation and copies, plan dimensions for dispatch inputs, plan lane for existing logical diagnostic aggregates, and plan identities for completion and commit. Physical state remains separate: M30 task IDs and lifecycle, M29 slot/generation, submission sequence, fence/query evidence, completion duration, staging allocations, quarantine/reap, and commit state.

PrometheusSgemmBatchDiagnostics layout is unchanged. Entry count, requested workers, effective workers, partition policy, plan generation, and worker-array aggregates are now explicitly plan-derived logical facts. Physical-ring and completion fields retain their existing M29/M30/M31 sources. P11-only physical-worker fields were not reinterpreted.

## No-replan and failure preparation

The plan is built exactly once by M31 and never mutates. Task admission copies the immutable entry ID and plan generation; completion checks that pair against the plan entry before staging the result. A mismatch is a batch execution failure, not a remap to a task-table index.

This prepares R2c without adding its failure reducer. Entry ID is the stable logical identity; logical lane is metadata; phase/detail remain failure facts; and slot/task indices remain physical evidence rather than user-visible failure identity. Device-loss/runtime-safety handling remains M30a behavior.

## Tests added and evidence

Added permanent focused tests in reactor_m29_fixed_double_tests.cpp:

- PrometheusSgemmBatchPlanDeterministicRoundRobin
- PrometheusSgemmBatchPlanDeterministicContiguous
- PrometheusSgemmBatchPlanValidatesBeforeAdmission
- PrometheusSgemmBatchPlanEntryIdentitySurvivesTaskReuse
- PrometheusSgemmBatchPlanIsBuiltOnce

The validation test directly checks plan failure and, on the real M31 hardware route, checks an invalid later entry produces zero physical submits, stable entry ID, no output commit, and untouched caller sentinels. The task-reuse test uses depth one to force record recycling and verifies per-entry submission/commit evidence. The built-once test verifies one plan generation across all completion and commit records.

Windows validation:

- manifest check: pass;
- full native Windows build: pass;
- all five plan tests: pass on NVIDIA GeForce RTX 3070;
- PrometheusSgemmBatchRefillRing: pass on RTX 3070;
- M29 and M30 focused lanes: pass;
- resident and EVT correctness lanes: pass;
- unchanged slow P11 suite: 62 pass, 1 expected skip;
- go test ./internal/prometheus/... ./cmd/oct: pass;
- go test ./internal/... ./cmd/oct: pass;
- Linux manifest parity and bash -n build_linux.sh: pass; Linux runtime build was not available in this Windows environment.

## M30a quarantine/reap triage

The initially failing focused lane was reproduced three times. The first failed
assertion was replacement completion after queued task A. The dependent
assertions then showed replacement consume failing, unchanged replacement
output, unreaped quarantine, no replacement feedback, and task A still
queryable instead of stale. The observed stale-query detail was
PROM_DETAIL_ASYNC_NOT_READY rather than PROM_DETAIL_ASYNC_NO_TASK. This is the
public-API state for an outstanding submitted record, not a plan-attribution
failure.

The physical interpretation is explicit in the M30a code path: task A's
injected observation failure makes its slot QUARANTINED at its current slot
generation; the replacement uses the other physical slot and remains
SUBMITTED until the single compute queue reaches it. The failed reap counter
and reap-success counter remained unchanged while the old wait helper
performed 2,000 immediate nonblocking queries. In that state the only fence
result permitted by prom_async_poll_task for replacement is VK_NOT_READY; a
success would make it READY, while a Vulkan error would make it FAILED. The
quarantined A slot likewise had not reached the signaled-fence branch of the
reaper. No query-result failure path or task-record identity mismatch was
observed.

To isolate R2b, the task-attribution fields and M31 identity check were
temporarily removed while leaving the immutable plan otherwise present. The
native build succeeded and all three M30a reruns failed identically. Those
fields are not read or written by the public M30a submit/query/abandon/reap
path. The fault was therefore exposed, not caused, by R2b.

The narrow safe correction replaces the fixed-count busy spin in the
test-only wait helper with deadline-based nonblocking polling for five seconds
and std::this_thread::yield between polls. It neither waits on Vulkan directly
nor changes M30a ownership, fences, query handling, slot reuse, production
timeouts, or the assertion. It gives the already-queued real GPU work an
opportunity to reach the normal fence/query completion path. M30a then passed
three consecutive focused runs and once more in the full required matrix.

## R2c remains deliberately unstarted

R2b does not redirect flags, port P11 worker/event machinery, change first-failure selection, add an event framework, revise diagnostics, or remove the CPU oracle/empty-submit executor. R2c may now port selected durable semantics into this explicit plan-to-ring boundary, starting with deterministic first-failure and real event/feedback proofs.

## Acceptance judgment

**R2b ACCEPTED.** The immutable plan, preflight, attribution, logical/physical
separation, P11 preservation, and RTX-3070 M31 proof all pass. The M30a issue
was a pre-existing test-harness correctness bug, not an R2b regression; its
narrow nonblocking-poll correction is validated by the complete required
matrix. R2c has not begun.
