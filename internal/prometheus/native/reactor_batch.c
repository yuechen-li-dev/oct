#include "reactor_vulkan_sgemm_internal.h"
#include "reactor_batch.h"

/*
 * PROMETHEUS BATCH EXECUTION ATLAS
 *
 * PURPOSE
 *   This is the authoritative public SGEMM batch contract. It executes a
 *   validated caller-order batch without changing public ABI or diagnostics.
 *
 * AUTHORITY
 *   public batch API -> immutable batch plan -> centralized admission/refill
 *   -> M30 task lifecycle -> M29 shared physical ring
 *   -> physical completion evidence -> staged outputs -> atomic ordered commit
 *
 * THIS MODULE OWNS
 *   Immutable per-entry planning; logical-width and partition metadata;
 *   per-entry runtime state; deterministic failure reduction; stop admission
 *   and skipped-tail semantics; refill coordination; staging, publication,
 *   ordered commit, and batch diagnostics preparation.
 *
 * THIS MODULE DOES NOT OWN
 *   Vulkan device or queue creation; physical submission slots; public async
 *   token lifecycle; quarantine/reap implementation; shader or pipeline
 *   creation; implementation selection policy; P14/P15 algorithms; sync
 *   execution; or FFT.
 *
 * CRITICAL INVARIANTS
 *   1. A logical lane is not a CPU thread, Vulkan queue, or physical ring slot.
 *   2. Immutable entry identity survives task and slot reuse.
 *   3. No failure publishes partial caller output; stop admission never
 *      abandons physical ownership.
 *   4. Physical ownership resolves through completion, drain, reap, or
 *      quarantine. Diagnostics report observed execution or explicit planning
 *      facts, never decorative execution.
 *   5. CPU reference SGEMM is not native production batch authority. Supported
 *      public batch options execute the real engine or fail explicitly.
 *
 * INDEX
 *   ATLAS 1 planning; 2 logical partitioning; 3 runtime/failure state;
 *   4 admission/refill; 5 drain/ownership resolution; 6 staging/publication;
 *   7 ordered commit; 8 diagnostics; 9 test-only seams.
 *
 * HISTORY
 *   This replaces the removed P11 worker-local/CPU-authority architecture.
 *   See ../DevelopmentReport/PROMETHEUS_P11_ARCHITECTURE_RETROSPECTIVE.md.
 *   Future concurrency is design-only: see
 *   ../DevelopmentReport/PROMETHEUS_CONCURRENCY_FUTURE_DIRECTIONS.md.
 */

// ATLAS 1 — Immutable planning

enum {
  PROM_SGEMM_BATCH_PLAN_GENERATION = 1u,
  PROM_SGEMM_BATCH_MAX_LOGICAL_WIDTH = 8u,
};

static uint32_t prom_sgemm_batch_requested_logical_width(uint32_t flags) {
  const uint32_t requested = flags & 0xffu;
  return requested == 0u ? 1u : requested;
}

// ATLAS 2 — Logical partitioning
static uint32_t prom_sgemm_batch_logical_lane(uint32_t entry_id,
                                              uint32_t entry_count,
                                              uint32_t logical_width,
                                              uint32_t flags) {
  if ((flags & PROM_BATCH_FLAG_PARTITION_CONTIGUOUS) != 0u) {
    if (entry_count == 0u || logical_width == 0u) {
      return 0u;
    }
    return (uint32_t)(((uint64_t)entry_id * (uint64_t)logical_width) /
                      (uint64_t)entry_count);
  }
  return logical_width == 0u ? 0u : entry_id % logical_width;
}

static uint32_t prom_sgemm_batch_test_failure_entry(uint32_t flags,
                                                     uint32_t entry_count) {
  const uint32_t encoded =
      (flags & PROM_BATCH_FLAG_TEST_FAIL_ENTRY_MASK) >> PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT;
  if (encoded == 0u || entry_count == 0u || encoded > entry_count) {
    return UINT32_MAX;
  }
  return encoded - 1u;
}

int prom_sgemm_batch_plan_build(const PrometheusSgemmBatchEntry* entries,
                                uint32_t entry_count,
                                uint32_t flags,
                                prom_sgemm_batch_plan* out_plan,
                                uint32_t* out_failed_entry_id) {
  uint32_t entry_id;
  uint32_t requested_logical_width;
  uint32_t planned_logical_width;
  uint32_t partition_policy;

  if (out_failed_entry_id != NULL) {
    *out_failed_entry_id = UINT32_MAX;
  }
  if (out_plan == NULL) {
    return PROM_ERROR;
  }
  memset(out_plan, 0, sizeof(*out_plan));
  if (entries == NULL || entry_count == 0u) {
    return PROM_ERROR;
  }

  requested_logical_width = prom_sgemm_batch_requested_logical_width(flags);
  planned_logical_width = requested_logical_width;
  if (planned_logical_width > PROM_SGEMM_BATCH_MAX_LOGICAL_WIDTH) {
    planned_logical_width = PROM_SGEMM_BATCH_MAX_LOGICAL_WIDTH;
  }
  if (planned_logical_width > entry_count) {
    planned_logical_width = entry_count;
  }
  if (planned_logical_width == 0u) {
    return PROM_ERROR;
  }
  partition_policy = (flags & PROM_BATCH_FLAG_PARTITION_CONTIGUOUS) != 0u
                         ? PROM_BATCH_PARTITION_CONTIGUOUS
                         : PROM_BATCH_PARTITION_ROUND_ROBIN;
  out_plan->entries = (prom_sgemm_batch_entry_plan*)calloc((size_t)entry_count,
                                                             sizeof(*out_plan->entries));
  if (out_plan->entries == NULL) {
    return PROM_ERROR;
  }

  out_plan->entry_count = entry_count;
  out_plan->requested_logical_width = requested_logical_width;
  out_plan->planned_logical_width = planned_logical_width;
  out_plan->partition_policy = partition_policy;
  out_plan->plan_generation = PROM_SGEMM_BATCH_PLAN_GENERATION;
  for (entry_id = 0u; entry_id < entry_count; ++entry_id) {
    const PrometheusSgemmBatchEntry* entry = &entries[entry_id];
    prom_sgemm_batch_entry_plan* plan_entry = &out_plan->entries[entry_id];
    uint32_t a_count;
    uint32_t b_count;
    uint32_t c_count;
    if (entry->a == NULL || entry->b == NULL || entry->c == NULL ||
        entry->m == 0u || entry->n == 0u || entry->k == 0u ||
        !prom_vk_checked_mul_u32(entry->m, entry->k, &a_count) ||
        !prom_vk_checked_mul_u32(entry->k, entry->n, &b_count) ||
        !prom_vk_checked_mul_u32(entry->m, entry->n, &c_count) ||
        (size_t)a_count > SIZE_MAX / sizeof(float) ||
        (size_t)b_count > SIZE_MAX / sizeof(float) ||
        (size_t)c_count > SIZE_MAX / sizeof(float)) {
      if (out_failed_entry_id != NULL) {
        *out_failed_entry_id = entry_id;
      }
      prom_sgemm_batch_plan_destroy(out_plan);
      return PROM_ERROR;
    }
    plan_entry->entry_id = entry_id;
    plan_entry->logical_lane = prom_sgemm_batch_logical_lane(entry_id,
                                                              entry_count,
                                                              planned_logical_width,
                                                              flags);
    plan_entry->plan_generation = out_plan->plan_generation;
    plan_entry->m = entry->m;
    plan_entry->n = entry->n;
    plan_entry->k = entry->k;
    plan_entry->a_element_count = (size_t)a_count;
    plan_entry->b_element_count = (size_t)b_count;
    plan_entry->c_element_count = (size_t)c_count;
    plan_entry->a_byte_count = (size_t)a_count * sizeof(float);
    plan_entry->b_byte_count = (size_t)b_count * sizeof(float);
    plan_entry->c_byte_count = (size_t)c_count * sizeof(float);
    plan_entry->a = entry->a;
    plan_entry->b = entry->b;
    plan_entry->c = entry->c;
    /* M31's current real path is the existing direct baseline dispatch. */
    plan_entry->selected_path = PROM_VK_PATH_DIRECT;
    plan_entry->compute_mode = PROM_VK_COMPUTE_BASELINE;
    plan_entry->requested_variant = 0u;
    plan_entry->executed_variant = 0u;
    plan_entry->planning_flags = flags & (PROM_BATCH_FLAG_PARTITION_CONTIGUOUS | 0xffu);
  }
  return PROM_OK;
}

void prom_sgemm_batch_plan_destroy(prom_sgemm_batch_plan* plan) {
  if (plan == NULL) {
    return;
  }
  free(plan->entries);
  memset(plan, 0, sizeof(*plan));
}

// ATLAS 3 — Per-entry runtime state and deterministic failure reduction

/* This rank is permanent for the batch reducer and is defined only by real
   execution phases. */
enum {
  PROM_BATCH_FAILURE_PHASE_STAGING_ALLOCATION = 1u,
  PROM_BATCH_FAILURE_PHASE_TASK_ALLOCATION = 2u,
  PROM_BATCH_FAILURE_PHASE_COMMAND_RECORD = 3u,
  PROM_BATCH_FAILURE_PHASE_SUBMIT = 4u,
  PROM_BATCH_FAILURE_PHASE_COMPLETION_OBSERVATION = 5u,
  PROM_BATCH_FAILURE_PHASE_RESULT_COPY_TO_STAGING = 6u,
  PROM_BATCH_FAILURE_PHASE_COMMIT = 7u,
};

typedef struct prom_batch_failure_reducer {
  uint32_t selected;
  uint32_t entry_id;
  uint32_t phase;
  int detail;
  uint64_t observation_sequence;
} prom_batch_failure_reducer;

static int prom_batch_failure_precedes(const prom_batch_failure_reducer* current,
                                       uint32_t entry_id,
                                       uint32_t phase,
                                       uint64_t observation_sequence) {
  if (current->selected == 0u) return 1;
  if (entry_id != current->entry_id) return entry_id < current->entry_id;
  if (phase != current->phase) return phase < current->phase;
  return observation_sequence < current->observation_sequence;
}

static void prom_batch_failure_offer(prom_batch_failure_reducer* reducer,
                                     prom_sgemm_batch_entry_runtime* runtime,
                                     uint32_t phase,
                                     int detail,
                                     uint64_t observation_sequence) {
  runtime->failure_phase = phase;
  runtime->failure_detail = detail;
  runtime->observation_sequence = observation_sequence;
  runtime->state = PROM_BATCH_ENTRY_FAILED;
  if (prom_batch_failure_precedes(reducer, runtime->entry_id, phase, observation_sequence)) {
    reducer->selected = 1u;
    reducer->entry_id = runtime->entry_id;
    reducer->phase = phase;
    reducer->detail = detail;
    reducer->observation_sequence = observation_sequence;
  }
}

/* The batch engine deliberately uses the M30 task-owned buffer and M29 physical-slot
   lifecycle. The immutable plan below owns only logical facts; a logical lane
   is never welded to a physical slot, queue, or command resource. */
int prom_sgemm_batch_execute(prometheus_runtime* rt,
                                        const PrometheusSgemmBatchEntry* entries,
                                        uint32_t entry_count,
                                        uint32_t flags,
                                        uint32_t* out_stage,
                                        int* out_detail_code) {
  prom_sgemm_batch_plan plan;
  prom_sgemm_async_task** jobs = NULL;
  float** staged = NULL;
  prom_sgemm_batch_entry_runtime* runtime_entries = NULL;
  prom_batch_failure_reducer primary_failure;
  uint32_t next = 0u, completed = 0u, completion_count = 0u, failed_entry = UINT32_MAX;
  uint32_t failed_worker = UINT32_MAX, failure_stage = PROM_STAGE_NONE;
  int failure_detail = 0;
  uint32_t effective_depth, i;
  const uint32_t injected_failure_entry = prom_sgemm_batch_test_failure_entry(flags, entry_count);
  uint64_t submits0, polls0, waits0, full0, harvest0, quarantine0, reap0, feedback0, skipped0;
  int failed = 0;
  uint64_t observation_sequence = 0u;

  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_NONE, 0);
  memset(&plan, 0, sizeof(plan));
  memset(&primary_failure, 0, sizeof(primary_failure));
  if (rt->async_runtime_unsafe_to_reuse != 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_ASYNC_FAILED);
    return PROM_ERROR;
  }
  if (entries == NULL || entry_count == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_ERROR);
    return PROM_ERROR;
  }
  effective_depth = rt->submission_ring_diag.configured_depth;
  if (effective_depth > PROM_SGEMM_ASYNC_MAX_TASKS) effective_depth = PROM_SGEMM_ASYNC_MAX_TASKS;
  if (effective_depth == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_ERROR);
    return PROM_ERROR;
  }
  if (prom_sgemm_batch_plan_build(entries, entry_count, flags, &plan, &failed_entry) != PROM_OK) {
    failed_worker = failed_entry < entry_count ? prom_sgemm_batch_logical_lane(failed_entry,
                                                                                 entry_count,
                                                                                 prom_sgemm_batch_requested_logical_width(flags),
                                                                                 flags)
                                                : UINT32_MAX;
    memset(&rt->batch_diag, 0, sizeof(rt->batch_diag));
    rt->batch_diag.last_batch_entry_count = entry_count;
    rt->batch_diag.requested_workers = prom_sgemm_batch_requested_logical_width(flags);
    rt->batch_diag.effective_workers = rt->batch_diag.requested_workers > PROM_SGEMM_BATCH_MAX_LOGICAL_WIDTH
                                         ? PROM_SGEMM_BATCH_MAX_LOGICAL_WIDTH
                                         : rt->batch_diag.requested_workers;
    rt->batch_diag.partition_policy = (flags & PROM_BATCH_FLAG_PARTITION_CONTIGUOUS) != 0u
                                        ? PROM_BATCH_PARTITION_CONTIGUOUS
                                        : PROM_BATCH_PARTITION_ROUND_ROBIN;
    rt->batch_diag.plan_generation = PROM_SGEMM_BATCH_PLAN_GENERATION;
    rt->batch_diag.batch_state = PROM_BATCH_STATE_FAILED;
    rt->batch_diag.failed_entry_id = failed_entry;
    rt->batch_diag.failed_worker_id = failed_worker;
    rt->batch_diag.failure_stage = PROM_STAGE_INIT;
    rt->batch_diag.failure_detail = PROM_DETAIL_BATCH_PLAN_INVALID;
    rt->batch_diag.failure_count = 1u;
    rt->batch_diag.first_failure_stable = 1u;
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_DETAIL_BATCH_PLAN_INVALID);
    return PROM_ERROR;
  }
  jobs = (prom_sgemm_async_task**)calloc(entry_count, sizeof(*jobs));
  staged = (float**)calloc(entry_count, sizeof(*staged));
  runtime_entries = (prom_sgemm_batch_entry_runtime*)calloc(entry_count, sizeof(*runtime_entries));
  if (jobs == NULL || staged == NULL || runtime_entries == NULL) goto oom;
  memset(&rt->batch_diag, 0, sizeof(rt->batch_diag));
  rt->batch_diag.last_batch_entry_count = entry_count;
  /* These v1 fields are plan-derived logical facts, not physical workers. */
  rt->batch_diag.requested_workers = plan.requested_logical_width;
  rt->batch_diag.effective_workers = plan.planned_logical_width;
  rt->batch_diag.partition_policy = plan.partition_policy;
  rt->batch_diag.plan_generation = plan.plan_generation;
  rt->batch_diag.batch_state = PROM_BATCH_STATE_RUNNING;
  /* v1 compatibility fields: one shared physical queue, no batch worker
     threads or worker-local resources. Per-lane arrays below are logical
     planning aggregates only; P11-only resource and slot fields stay zero. */
  rt->batch_diag.execution_mode = PROM_BATCH_EXECUTION_SINGLE_WORKER;
  rt->batch_diag.worker_resource_mode = PROM_BATCH_WORKER_RESOURCE_SHARED;
  rt->batch_diag.queue_topology_classification = PROM_BATCH_QUEUE_TOPOLOGY_SINGLE_QUEUE;
  rt->batch_diag.queue_mapping_mode = PROM_BATCH_QUEUE_MAPPING_SINGLE_QUEUE_SERIALIZED;
  rt->batch_diag.physical_ring_depth_configured = rt->submission_ring_diag.configured_depth;
  rt->batch_diag.physical_ring_depth_effective = effective_depth;
  submits0 = rt->submission_ring_diag.total_submits; polls0 = rt->submission_ring_diag.total_polls;
  waits0 = rt->submission_ring_diag.total_forced_waits; full0 = rt->submission_ring_diag.ring_full_count;
  harvest0 = rt->submission_ring_diag.total_query_harvests; quarantine0 = rt->async_quarantine_event_count;
  reap0 = rt->async_reap_success_count; feedback0 = rt->async_feedback_committed_count; skipped0 = rt->async_feedback_skipped_count;

  // ATLAS 6 — Staging and atomic publication
  /* The plan owns all validated logical facts. Staging remains physical batch
     state, but every allocation succeeds before first Vulkan admission. */
  for (i = 0u; i < entry_count; ++i) {
    const prom_sgemm_batch_entry_plan* plan_entry = &plan.entries[i];
    runtime_entries[i].entry_id = plan_entry->entry_id;
    runtime_entries[i].plan_generation = plan_entry->plan_generation;
    runtime_entries[i].logical_lane = plan_entry->logical_lane;
    runtime_entries[i].state = PROM_BATCH_ENTRY_PLANNED;
    runtime_entries[i].physical_slot_id = UINT32_MAX;
  }
  for (i = 0u; i < entry_count; ++i) {
    const prom_sgemm_batch_entry_plan* plan_entry = &plan.entries[i];
    staged[i] = (float*)calloc(plan_entry->c_element_count, sizeof(float));
    if (staged[i] == NULL) {
      failed = 1;
      failed_entry = plan_entry->entry_id;
      failed_worker = plan_entry->logical_lane;
      failure_stage = PROM_STAGE_TRANSFER_OUT;
      failure_detail = PROM_ERROR;
      prom_batch_failure_offer(&primary_failure, &runtime_entries[i],
                               PROM_BATCH_FAILURE_PHASE_STAGING_ALLOCATION,
                               failure_detail, ++observation_sequence);
      break;
    }
    if (plan_entry->logical_lane < 8u) {
      rt->batch_diag.worker_assigned_count[plan_entry->logical_lane] += 1u;
    }
  }
  // ATLAS 4 — Admission and centralized refill
  while (!failed && completed < entry_count) {
    uint32_t made_progress = 0u;
    /* Stable entry-ID admission is the central schedule. */
    while (next < entry_count && !failed) {
      const prom_sgemm_batch_entry_plan* plan_entry = &plan.entries[next];
      prom_sgemm_submission_slot* slot = NULL;
      prom_sgemm_async_task* task;
      VkResult result;
      if (rt->submission_ring_diag.outstanding >= effective_depth) { rt->submission_ring_diag.ring_full_count += 1u; break; }
      for (i = 0u; i < effective_depth; ++i) if (rt->submission_ring[i].state == PROM_SGEMM_SUBMISSION_SLOT_EMPTY) { slot = &rt->submission_ring[i]; break; }
      if (slot == NULL) { rt->submission_ring_diag.ring_full_count += 1u; break; }
      task = prom_async_task_allocate(rt);
      if (task == NULL) { failed = 1; failed_entry = plan_entry->entry_id; failed_worker = plan_entry->logical_lane; failure_stage = PROM_STAGE_SUBMIT; failure_detail = PROM_DETAIL_ASYNC_QUEUE_FULL; prom_batch_failure_offer(&primary_failure, &runtime_entries[next], PROM_BATCH_FAILURE_PHASE_TASK_ALLOCATION, failure_detail, ++observation_sequence); break; }
      runtime_entries[next].state = PROM_BATCH_ENTRY_ADMITTED;
      slot->state = PROM_SGEMM_SUBMISSION_SLOT_PREPARING; slot->generation += 1u;
      slot->submission_sequence = rt->submission_ring_diag.next_sequence++; slot->physical_completion_confirmed = 0u;
      task->m = plan_entry->m; task->n = plan_entry->n; task->k = plan_entry->k; task->compute_k = plan_entry->k;
      task->physical_slot_id = slot->slot_id; task->physical_slot_generation = slot->generation;
      task->batch_entry_id = plan_entry->entry_id; task->batch_plan_generation = plan_entry->plan_generation;
      task->submission_sequence = rt->async_next_submission_sequence++; task->selected_path = plan_entry->selected_path;
      task->compute_mode = plan_entry->compute_mode; task->requested_variant = plan_entry->requested_variant; task->executed_variant = plan_entry->executed_variant;
      result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags, (VkDeviceSize)plan_entry->a_byte_count, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT, VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1, &task->a);
      if (result == VK_SUCCESS) result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags, (VkDeviceSize)plan_entry->b_byte_count, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT, VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1, &task->b);
      if (result == VK_SUCCESS) result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags, (VkDeviceSize)plan_entry->c_byte_count, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT, VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1, &task->c);
      if (result != VK_SUCCESS) { slot->state = PROM_SGEMM_SUBMISSION_SLOT_EMPTY; prom_async_task_release(rt, task); failed = 1; failed_entry = plan_entry->entry_id; failed_worker = plan_entry->logical_lane; failure_stage = PROM_STAGE_TRANSFER_IN; failure_detail = (int)result; prom_batch_failure_offer(&primary_failure, &runtime_entries[next], PROM_BATCH_FAILURE_PHASE_COMMAND_RECORD, failure_detail, ++observation_sequence); break; }
      memcpy(task->a.mapped, plan_entry->a, plan_entry->a_byte_count); memcpy(task->b.mapped, plan_entry->b, plan_entry->b_byte_count); memset(task->c.mapped, 0, plan_entry->c_byte_count);
      if (prom_async_record_slot(rt, slot, task) != PROM_OK || prom_sgemm_ring_submit_slot(rt, slot) != PROM_OK) { slot->state = PROM_SGEMM_SUBMISSION_SLOT_EMPTY; prom_async_task_release(rt, task); failed = 1; failed_entry = plan_entry->entry_id; failed_worker = plan_entry->logical_lane; failure_stage = PROM_STAGE_SUBMIT; failure_detail = PROM_DETAIL_BATCH_QUEUE_SUBMIT_FAILED; prom_batch_failure_offer(&primary_failure, &runtime_entries[next], PROM_BATCH_FAILURE_PHASE_SUBMIT, failure_detail, ++observation_sequence); break; }
      task->lifecycle_state = PROM_ASYNC_STATE_SUBMITTED; jobs[next] = task;
      runtime_entries[next].state = PROM_BATCH_ENTRY_SUBMITTED;
      runtime_entries[next].submission_sequence = task->submission_sequence;
      runtime_entries[next].physical_slot_id = slot->slot_id;
      runtime_entries[next].physical_slot_generation = slot->generation;
      if (next < 64u) { rt->batch_diag.m31_submission_sequence[next] = task->submission_sequence; rt->batch_diag.m31_physical_slot_id[next] = slot->slot_id; }
      if (plan_entry->logical_lane < 8u) { rt->batch_diag.worker_submit_count[plan_entry->logical_lane] += 1u; rt->batch_diag.worker_event_count[plan_entry->logical_lane] += 2u; }
      rt->batch_diag.refill_count += (next >= effective_depth) ? 1u : 0u;
      ++next; made_progress = 1u;
      if ((flags & PROM_BATCH_FLAG_FAIL_AFTER_FIRST_SUBMIT) != 0u && next == 1u) { failed = 1; failed_entry = 0u; failed_worker = plan.entries[0].logical_lane; failure_stage = PROM_STAGE_SUBMIT; failure_detail = PROM_DETAIL_BATCH_EXECUTION_FAILED; prom_batch_failure_offer(&primary_failure, &runtime_entries[0], PROM_BATCH_FAILURE_PHASE_SUBMIT, failure_detail, ++observation_sequence); }
      if (!failed && next - 1u == injected_failure_entry) { failed = 1; failed_entry = injected_failure_entry; failed_worker = plan.entries[injected_failure_entry].logical_lane; failure_stage = PROM_STAGE_SUBMIT; failure_detail = PROM_DETAIL_BATCH_EXECUTION_FAILED; prom_batch_failure_offer(&primary_failure, &runtime_entries[injected_failure_entry], PROM_BATCH_FAILURE_PHASE_SUBMIT, failure_detail, ++observation_sequence); }
      // ATLAS 9 — Test-only seams (flag decoding only; no test-only route)
      if (!failed && (flags & PROM_BATCH_FLAG_TEST_DUAL_FAIL_FIRST_TWO) != 0u && next == 2u) {
        /* Test-only simultaneous candidate seam.  Both candidates exist before
           the stop flag is set, so the reducer—not discovery order—selects
           entry zero as the stable primary cause. */
        prom_batch_failure_offer(&primary_failure, &runtime_entries[1], PROM_BATCH_FAILURE_PHASE_SUBMIT,
                                 PROM_DETAIL_BATCH_EXECUTION_FAILED, ++observation_sequence);
        prom_batch_failure_offer(&primary_failure, &runtime_entries[0], PROM_BATCH_FAILURE_PHASE_SUBMIT,
                                 PROM_DETAIL_BATCH_EXECUTION_FAILED, ++observation_sequence);
        failed = 1; failed_entry = primary_failure.entry_id; failed_worker = runtime_entries[failed_entry].logical_lane;
        failure_stage = PROM_STAGE_SUBMIT; failure_detail = primary_failure.detail;
      }
    }
    for (i = 0u; i < next; ++i) {
      prom_sgemm_async_task* task = jobs[i];
      if (task == NULL || task->lifecycle_state != PROM_ASYNC_STATE_SUBMITTED) continue;
      if (task->batch_entry_id != plan.entries[i].entry_id ||
          task->batch_plan_generation != plan.entries[i].plan_generation) {
        failed = 1; failed_entry = plan.entries[i].entry_id; failed_worker = plan.entries[i].logical_lane;
        failure_stage = PROM_STAGE_SUBMIT; failure_detail = PROM_DETAIL_BATCH_EXECUTION_FAILED;
        prom_batch_failure_offer(&primary_failure, &runtime_entries[i], PROM_BATCH_FAILURE_PHASE_COMPLETION_OBSERVATION, failure_detail, ++observation_sequence); continue;
      }
      if (prom_async_poll_task(rt, task) != PROM_OK || task->lifecycle_state == PROM_ASYNC_STATE_FAILED) { if (!failed) { failed = 1; failed_entry = plan.entries[i].entry_id; failed_worker = plan.entries[i].logical_lane; failure_stage = task->final_stage; failure_detail = task->final_detail; prom_batch_failure_offer(&primary_failure, &runtime_entries[i], PROM_BATCH_FAILURE_PHASE_COMPLETION_OBSERVATION, failure_detail, ++observation_sequence); } continue; }
      if (task->lifecycle_state == PROM_ASYNC_STATE_READY) {
        memcpy(staged[i], task->c.mapped, plan.entries[i].c_byte_count);
        rt->submission_ring[task->physical_slot_id].state = PROM_SGEMM_SUBMISSION_SLOT_EMPTY;
        task->lifecycle_state = PROM_ASYNC_STATE_CONSUMED;
        runtime_entries[i].state = PROM_BATCH_ENTRY_COMPLETED;
        if (i < 64u) { rt->batch_diag.m31_completion_status[i] = 1u; rt->batch_diag.m31_gpu_duration_ns[i] = task->gpu_duration_ns; rt->batch_diag.m31_completion_order[completion_count] = i; }
        ++completion_count; ++completed; made_progress = 1u;
        if (plan.entries[i].logical_lane < 8u) { rt->batch_diag.worker_completed_count[plan.entries[i].logical_lane] += 1u; rt->batch_diag.worker_event_count[plan.entries[i].logical_lane] += 1u; }
      }
    }
    prom_async_process_completion_feedback(rt);
    /* A consumed M30 record becomes eligible for table reuse after ordered
       feedback.  Clear the batch's pointer at the same transition so a later
       refill cannot be mistaken for the earlier entry on a recycled record. */
    for (i = 0u; i < next; ++i) {
      if (jobs[i] != NULL && jobs[i]->feedback_pending == 0u && jobs[i]->feedback_committed != 0u) {
        if (jobs[i]->lifecycle_state == PROM_ASYNC_STATE_FAILED) runtime_entries[i].feedback_skipped = 1u;
        else runtime_entries[i].feedback_committed = 1u;
      }
      if (jobs[i] != NULL && jobs[i]->lifecycle_state == PROM_ASYNC_STATE_CONSUMED && jobs[i]->feedback_pending == 0u) {
        prom_async_task_release(rt, jobs[i]);
        jobs[i] = NULL;
      }
    }
    if (!failed && completed < entry_count && !made_progress && rt->submission_ring_diag.outstanding != 0u) {
      if (prom_sgemm_ring_wait_oldest(rt) != PROM_OK) { failed = 1; failure_stage = PROM_STAGE_SUBMIT; failure_detail = PROM_DETAIL_BATCH_FENCE_WAIT_FAILED; }
    }
  }
  // ATLAS 5 — Drain and physical ownership resolution
  /* The reducer freezes at this admission-stop boundary.  Anything discovered
     while draining is lifecycle evidence, never a replacement batch cause. */
  if (primary_failure.selected != 0u) {
    failed_entry = primary_failure.entry_id;
    failed_worker = runtime_entries[failed_entry].logical_lane;
    failure_detail = primary_failure.detail;
  }
  if (failed) {
    for (i = next; i < entry_count; ++i) {
      if (runtime_entries[i].state == PROM_BATCH_ENTRY_PLANNED) {
        runtime_entries[i].state = PROM_BATCH_ENTRY_SKIPPED;
      }
    }
  }
  /* Batch failure stops admission, then resolves every owned submission before
     releasing its independent buffers.  Quarantined ownership uses M30a's
     explicit reaper rather than recycling a possibly-live slot. */
  for (i = 0u; i < next; ++i) {
    prom_sgemm_async_task* task = jobs[i];
    if (task == NULL) continue;
    if (task->lifecycle_state == PROM_ASYNC_STATE_SUBMITTED) {
      VkFence fence = rt->submission_ring[task->physical_slot_id].fence;
      rt->submission_ring_diag.total_forced_waits += 1u;
      (void)vkWaitForFences(rt->device, 1u, &fence, VK_TRUE, UINT64_MAX);
      (void)prom_async_poll_task(rt, task);
    }
    prom_async_process_completion_feedback(rt);
    if (task->slot_quarantined != 0u) (void)prom_async_reap_quarantined_slots(rt, 1u);
    if (task->physical_slot_id < PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH &&
        task->slot_quarantined == 0u && rt->async_runtime_unsafe_to_reuse == 0u) {
      rt->submission_ring[task->physical_slot_id].state = PROM_SGEMM_SUBMISSION_SLOT_EMPTY;
    }
    if (runtime_entries[i].state != PROM_BATCH_ENTRY_FAILED) runtime_entries[i].state = PROM_BATCH_ENTRY_DRAINED;
    prom_async_task_release(rt, task);
  }
  // ATLAS 8 — Diagnostics preparation
  rt->batch_diag.physical_ring_depth_configured = rt->submission_ring_diag.configured_depth;
  rt->batch_diag.physical_ring_depth_effective = effective_depth;
  rt->batch_diag.current_in_flight = rt->submission_ring_diag.outstanding;
  rt->batch_diag.max_in_flight = rt->submission_ring_diag.max_outstanding;
  rt->batch_diag.total_submits = rt->submission_ring_diag.total_submits - submits0; rt->batch_diag.total_polls = rt->submission_ring_diag.total_polls - polls0;
  rt->batch_diag.total_forced_waits = rt->submission_ring_diag.total_forced_waits - waits0; rt->batch_diag.ring_full_count = rt->submission_ring_diag.ring_full_count - full0;
  rt->batch_diag.query_harvest_count = rt->submission_ring_diag.total_query_harvests - harvest0; rt->batch_diag.quarantine_count = rt->async_quarantine_event_count - quarantine0;
  rt->batch_diag.reap_count = rt->async_reap_success_count - reap0; rt->batch_diag.feedback_committed_count = rt->async_feedback_committed_count - feedback0; rt->batch_diag.feedback_skipped_count = rt->async_feedback_skipped_count - skipped0;
  rt->batch_diag.m31_completion_count = completion_count;
  // ATLAS 7 — Ordered commit
  if (!failed && completed == entry_count) {
    for (i = 0u; i < entry_count; ++i) { memcpy(plan.entries[i].c, staged[i], plan.entries[i].c_byte_count); runtime_entries[i].state = PROM_BATCH_ENTRY_COMMITTED; if (i < 64u) rt->batch_diag.m31_commit_order[i] = i; }
    rt->batch_diag.m31_commit_count = entry_count; rt->batch_diag.output_committed = 1u; rt->batch_diag.batch_state = PROM_BATCH_STATE_SUCCEEDED;
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, 0);
  } else {
    rt->batch_diag.batch_state = PROM_BATCH_STATE_FAILED; rt->batch_diag.failed_entry_id = failed_entry; rt->batch_diag.failed_worker_id = failed_worker;
    rt->batch_diag.failure_stage = failure_stage; rt->batch_diag.failure_detail = failure_detail; rt->batch_diag.failure_count = 1u; rt->batch_diag.first_failure_stable = 1u;
    prom_vk_set_status(out_stage, out_detail_code, failure_stage == PROM_STAGE_NONE ? PROM_STAGE_SUBMIT : failure_stage, failure_detail == 0 ? PROM_ERROR : failure_detail);
  }
  for (i = 0u; i < entry_count; ++i) free(staged[i]); free(staged); free(jobs); free(runtime_entries);
  prom_sgemm_batch_plan_destroy(&plan);
  return failed || completed != entry_count ? PROM_ERROR : PROM_OK;
oom:
  for (i = 0u; i < entry_count; ++i) if (staged != NULL) free(staged[i]);
  free(staged); free(jobs); free(runtime_entries);
  prom_sgemm_batch_plan_destroy(&plan);
  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_ERROR);
  return PROM_ERROR;
}
