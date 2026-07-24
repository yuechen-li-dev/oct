#if !defined(_WIN32) && !defined(_POSIX_C_SOURCE)
#define _POSIX_C_SOURCE 200809L
#endif

#include "reactor_vulkan_sgemm_internal.h"
#include "reactor_batch.h"

typedef struct prom_vk_push {
  uint32_t m;
  uint32_t n;
  uint32_t k;
} prom_vk_push;

enum {
  PROM_VK_PUSH_FIELD_OFFSET_M = 0,
  PROM_VK_PUSH_FIELD_OFFSET_N = 4,
  PROM_VK_PUSH_FIELD_OFFSET_K = 8,
  PROM_VK_SHADER_PUSH_BYTES = 12,
};

enum {
  PROM_SGEMM_LOOKAHEAD_DEFAULT = 2u,
  PROM_SGEMM_LOOKAHEAD_MIN = 0u,
  PROM_SGEMM_LOOKAHEAD_MAX = 2u,
  PROM_SGEMM_OUTSTANDING_DEFAULT = 2u,
  PROM_SGEMM_OUTSTANDING_MIN = 1u,
  PROM_SGEMM_OUTSTANDING_MAX = 2u,
  PROM_SGEMM_CHUNK_DEFAULT = 16u,
  PROM_SGEMM_CHUNK_MIN = 8u,
  PROM_SGEMM_CHUNK_MAX = 32u,
  PROM_SGEMM_WASTE_BUDGET_UNITS = 64u,
  PROM_SGEMM_RETREAT_PERMILLE = 250u,
  PROM_SGEMM_RECOVER_PERMILLE = 120u,
  PROM_SGEMM_RECOVERY_WINDOW = 3u,
  PROM_SGEMM_HYSTERESIS_MARGIN = 40u,
  PROM_SGEMM_PACKED4_MODE_BUDGET_AGGRESSIVE = 380u,
  PROM_SGEMM_PACKED4_MODE_BUDGET_SAFE = 220u,
  PROM_SGEMM_PACKED4_MODE_BUDGET_RECOVERY = 140u,
  PROM_ARENA_SHRINK_LOW_USAGE_EPOCHS = 6u,
  PROM_ARENA_SHRINK_COOLDOWN_EPOCHS = 4u,
};

/* This diagnostic is deliberately process-local instead of part of the public
 * numerical ABI.  It is used only by targeted reactor coverage tests: a
 * distinctive finite value makes any element the dispatched kernel did not
 * overwrite visible at readback. */
static uint32_t prom_sgemm_diagnostic_sentinel_enabled(void) {
  const char* value = getenv("PROMETHEUS_SGEMM_DIAGNOSTIC_SENTINEL");
  return value != NULL && strcmp(value, "1") == 0 ? 1u : 0u;
}

static void prom_sgemm_initialize_direct_output(void* mapped, size_t byte_count) {
  if (mapped == NULL) return;
  if (prom_sgemm_diagnostic_sentinel_enabled() == 0u) {
    memset(mapped, 0, byte_count);
    return;
  }
  {
    float* values = (float*)mapped;
    const size_t count = byte_count / sizeof(*values);
    for (size_t index = 0u; index < count; ++index) values[index] = -1234567.0f;
  }
}

#define PROM_ARENA_DEFAULT_BUDGET_BYTES (512ull * 1024ull * 1024ull)
#define PROM_ARENA_DEFAULT_SHRINK_FLOOR_BYTES (64ull * 1024ull * 1024ull)

/*
 * Push-constant layout contract (M11 hygiene port):
 * - host and shader use the same field list and order: m, n, k
 * - no mixed-width fields
 * - no reliance on implicit host padding
 * - append-only evolution only: add new fields at the end and update both
 *   this host contract and the shader module together
 */
#if defined(__cplusplus)
static_assert(offsetof(prom_vk_push, m) == PROM_VK_PUSH_FIELD_OFFSET_M, "push.m offset drift");
static_assert(offsetof(prom_vk_push, n) == PROM_VK_PUSH_FIELD_OFFSET_N, "push.n offset drift");
static_assert(offsetof(prom_vk_push, k) == PROM_VK_PUSH_FIELD_OFFSET_K, "push.k offset drift");
static_assert(sizeof(prom_vk_push) == PROM_VK_SHADER_PUSH_BYTES, "push struct size drift");
#else
_Static_assert(offsetof(prom_vk_push, m) == PROM_VK_PUSH_FIELD_OFFSET_M, "push.m offset drift");
_Static_assert(offsetof(prom_vk_push, n) == PROM_VK_PUSH_FIELD_OFFSET_N, "push.n offset drift");
_Static_assert(offsetof(prom_vk_push, k) == PROM_VK_PUSH_FIELD_OFFSET_K, "push.k offset drift");
_Static_assert(sizeof(prom_vk_push) == PROM_VK_SHADER_PUSH_BYTES, "push struct size drift");
#endif

static void* g_active_handles[PROMETHEUS_MAX_TRACKED_HANDLES];

#if defined(_WIN32)
static SRWLOCK g_registry_lock = SRWLOCK_INIT;

static void registry_lock(void) {
  AcquireSRWLockExclusive(&g_registry_lock);
}

static void registry_unlock(void) {
  ReleaseSRWLockExclusive(&g_registry_lock);
}
#else
static pthread_mutex_t g_registry_mutex = PTHREAD_MUTEX_INITIALIZER;

static void registry_lock(void) {
  pthread_mutex_lock(&g_registry_mutex);
}

static void registry_unlock(void) {
  pthread_mutex_unlock(&g_registry_mutex);
}
#endif

/* SPIR-V for:
 * #version 450
 * layout(local_size_x=8, local_size_y=8) in;
 * layout(set=0,binding=0) readonly buffer ABuffer{float a[];};
 * layout(set=0,binding=1) readonly buffer BBuffer{float b[];};
 * layout(set=0,binding=2) writeonly buffer CBuffer{float c[];};
 * layout(push_constant) uniform Push{uint m; uint n; uint k;} pc;
 * SPIR-V confirms offsets m=0, n=4, k=8.
 * ... naive row-major SGEMM C=A*B
 */

static uint32_t prom_runtime_request_resource_lease(prometheus_runtime* rt,
                                                    const prom_resource_lease_facts* facts,
                                                    prom_resource_lease_decision* out_decision) {
  prom_dom_sgemm_resource_lease_projection projection;
  if (rt == NULL || facts == NULL || out_decision == NULL) {
    return 0u;
  }
  if (prom_dom_sgemm_stage_resource_lease_facts(&rt->blackboard, facts) == 0u) {
    return 0u;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  if (prom_dom_sgemm_build_resource_lease_facts_from_visible(&rt->blackboard, facts, &projection) == 0u) {
    return 0u;
  }
  prom_judgment_engine_decide_resource_lease(&projection.facts, out_decision);
  if (prom_dom_sgemm_stage_resource_lease_decision(&rt->blackboard, out_decision) == 0u) {
    return 0u;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  return 1u;
}

static void prom_fill_lease_pressure_classes(prometheus_runtime* rt,
                                             uint32_t selected_recipe_variant,
                                             uint32_t shape_class,
                                             uint32_t device_band,
                                             uint64_t work_units,
                                             prom_resource_lease_facts* facts) {
  if (facts == NULL) {
    return;
  }
  facts->register_pressure_class = 1u;
  facts->shared_memory_pressure_class = 1u;
  facts->memory_bandwidth_pressure_class = 1u;
  facts->compute_pressure_class = 1u;
  facts->pipeline_latency_pressure_class = 1u;

  if (selected_recipe_variant == PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) {
    facts->register_pressure_class = 4u;
    facts->compute_pressure_class = 4u;
  } else if (selected_recipe_variant == PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE) {
    facts->memory_bandwidth_pressure_class = 3u;
    facts->shared_memory_pressure_class = 3u;
  }
  if (device_band == PROM_OCCUPANCY_DEVICE_BAND_REGISTER_CONSTRAINED) {
    facts->register_pressure_class = facts->register_pressure_class < 3u ? 3u : facts->register_pressure_class;
  }
  if (shape_class == PROM_OCCUPANCY_SHAPE_CLASS_LARGE_SQUARE || work_units >= (uint64_t)PROM_JUDGMENT_TILED_WORK_THRESHOLD) {
    facts->compute_pressure_class = facts->compute_pressure_class < 3u ? 3u : facts->compute_pressure_class;
    facts->memory_bandwidth_pressure_class = facts->memory_bandwidth_pressure_class < 3u ? 3u : facts->memory_bandwidth_pressure_class;
    facts->pipeline_latency_pressure_class = 3u;
  }
  if (rt != NULL && rt->slot_diag.m35_budget_rejection_count != 0u) {
    facts->memory_bandwidth_pressure_class = 4u;
    facts->shared_memory_pressure_class = facts->shared_memory_pressure_class < 3u ? 3u : facts->shared_memory_pressure_class;
  }
}

// ============================================================================
// SGEMM Dominatus Integration
// ============================================================================

static uint32_t selector_cache_enabled(const prometheus_runtime* rt) {
  if (rt == NULL) {
    return 0u;
  }
  return ((rt->vulkan.test_flags & PROM_TESTCFG_DISABLE_SELECTOR_CACHE) == 0u) ? 1u : 0u;
}

static void invalidate_selector_caches(prometheus_runtime* rt) {
  if (rt == NULL) {
    return;
  }
  if (rt->m35_selector_cache.valid != 0u) {
    rt->m35_selector_cache.invalidation_count += 1u;
  }
  rt->m35_selector_cache.valid = 0u;
  rt->m35_selector_cache.last_decision_reused = 0u;

  if (rt->transfer_selector_cache.valid != 0u) {
    rt->transfer_selector_cache.invalidation_count += 1u;
  }
  rt->transfer_selector_cache.valid = 0u;
  rt->transfer_selector_cache.last_decision_reused = 0u;

  if (rt->layout_precision_selector_cache.valid != 0u) {
    rt->layout_precision_selector_cache.invalidation_count += 1u;
  }
  rt->layout_precision_selector_cache.valid = 0u;
  rt->layout_precision_selector_cache.last_decision_reused = 0u;
}

// ============================================================================
// SGEMM Transfer Queue Integration
// ============================================================================

static void select_transfer_queue_policy(const prom_judgment_decision* judgment_decision,
                                         const prom_dom_transfer_queue_facts* facts,
                                         prom_dom_transfer_queue_decision* out_decision) {
  if (out_decision == NULL) {
    return;
  }
  memset(out_decision, 0, sizeof(*out_decision));
  if (judgment_decision == NULL || facts == NULL) {
    return;
  }
  out_decision->transfer_policy_selected = 0u;
  out_decision->selected_transfer_policy = 0u;
  out_decision->transfer_queue_used = 0u;
  out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_REQUIRED;
  if (judgment_decision->selected_path != PROM_VK_PATH_STAGED_UPLOAD) {
    return;
  }
  if (facts->transfer_queue_disabled_by_config != 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_DISABLED_BY_CONFIG;
    return;
  }
  if (facts->dedicated_transfer_available == 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_NO_DEDICATED_QUEUE;
    return;
  }
  if (facts->queue_families_differ == 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_PSEUDO_SHARED_QUEUE;
    return;
  }
  if (facts->transfer_queue_supported == 0u || facts->transfer_sync_ownership_supported == 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_SYNC_OWNERSHIP_UNSUPPORTED;
    return;
  }
  if (facts->transfer_workload_large_enough == 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_SMALL_SHAPE_LOW_BENEFIT;
    return;
  }
  if (facts->transfer_fallback_available == 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_REQUIRED;
    return;
  }
  out_decision->transfer_policy_selected = 1u;
  out_decision->selected_transfer_policy = 1u;
  out_decision->transfer_queue_used = 1u;
  out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_NONE;
}

static void mirror_async_from_visible(prometheus_runtime* rt) {
  prom_dom_async_snapshot snapshot;
  if (rt == NULL) {
    return;
  }
  if (prom_dom_sgemm_read_visible_async_snapshot(&rt->blackboard, &snapshot) == 0u) {
    return;
  }
  rt->async_task_id = snapshot.task_id;
  rt->async_state = snapshot.lifecycle_state;
  rt->async_stage = snapshot.stage;
  rt->async_failure_detail = snapshot.failure_detail;
}

static void stage_commit_async_snapshot(prometheus_runtime* rt, prom_dom_event_kind event_kind, int reason_code) {
  prom_dom_async_snapshot snapshot;
  uint64_t slot_generation = 0u;
  if (rt == NULL) {
    return;
  }
  memset(&snapshot, 0, sizeof(snapshot));
  snapshot.task_id = rt->async_task_id;
  snapshot.lifecycle_state = rt->async_state;
  snapshot.stage = rt->async_stage;
  snapshot.detail_code = rt->async_state == PROM_ASYNC_STATE_FAILED ? rt->async_failure_detail : rt->async_final_detail;
  snapshot.ready = rt->async_state == PROM_ASYNC_STATE_READY ? 1u : 0u;
  snapshot.failed = rt->async_state == PROM_ASYNC_STATE_FAILED ? 1u : 0u;
  snapshot.consumed = rt->async_state == PROM_ASYNC_STATE_CONSUMED ? 1u : 0u;
  snapshot.outstanding_tasks = rt->async_state == PROM_ASYNC_STATE_SUBMITTED ? 1u : 0u;
  snapshot.failure_stage = rt->async_state == PROM_ASYNC_STATE_FAILED ? rt->async_stage : PROM_STAGE_NONE;
  snapshot.failure_detail = rt->async_failure_detail;
  snapshot.submit_detail = rt->async_final_detail;
  snapshot.query_detail = snapshot.detail_code;
  snapshot.slot_id = rt->slot_diag.async_slot_id;
  if (snapshot.slot_id >= 0 && (uint32_t)snapshot.slot_id < 2u) {
    slot_generation = prom_slot_hfsm_metadata(&rt->slots[snapshot.slot_id])->generation;
  }
  snapshot.slot_generation = slot_generation;
  snapshot.owns_slot = rt->slot_diag.async_slot_id >= 0 ? 1u : 0u;
  snapshot.transfer_complete = rt->slot_diag.async_transfer_complete;
  snapshot.compute_complete = rt->in_flight_submit == 0u ? 1u : 0u;
  snapshot.readback_complete = (snapshot.compute_complete != 0u && snapshot.ready != 0u) ? 1u : 0u;
  if (prom_dom_sgemm_stage_async_snapshot(&rt->blackboard, &snapshot, event_kind, reason_code) != 0u) {
    prom_dom_sgemm_commit(&rt->blackboard);
    mirror_async_from_visible(rt);
  }
}

static void set_async_state(prometheus_runtime* rt, uint32_t state, uint32_t stage, int detail) {
  prom_dom_event_kind event_kind = PROM_DOM_EVENT_NONE;
  if (rt == NULL) {
    return;
  }
  rt->async_state = state;
  rt->async_stage = stage;
  if (state == PROM_ASYNC_STATE_FAILED) {
    rt->async_failure_detail = detail;
  } else {
    rt->async_failure_detail = 0;
  }
  if (state == PROM_ASYNC_STATE_SUBMITTED) {
    event_kind = PROM_DOM_EVENT_ASYNC_SUBMITTED;
  } else if (state == PROM_ASYNC_STATE_READY) {
    event_kind = PROM_DOM_EVENT_ASYNC_READY;
  } else if (state == PROM_ASYNC_STATE_FAILED) {
    event_kind = PROM_DOM_EVENT_ASYNC_FAILED;
  } else if (state == PROM_ASYNC_STATE_CONSUMED) {
    event_kind = PROM_DOM_EVENT_ASYNC_CONSUMED;
  }
  stage_commit_async_snapshot(rt, event_kind, detail);
}

static void prom_slot_mark_failure(prometheus_runtime* rt, uint32_t slot_id, int reason);
static int prom_slot_mark_complete(prometheus_runtime* rt, uint32_t slot_id);
static void stage_transfer_complete_telemetry(prometheus_runtime* rt, uint32_t complete, uint32_t slot_id, int reason_code);
static void stage_transfer_failure_telemetry(prometheus_runtime* rt, uint32_t slot_id, int reason_code);
static uint32_t stage_slot_runtime_diag_snapshot(prometheus_runtime* rt, int reason_code);
static void commit_slot_runtime_diag_snapshot(prometheus_runtime* rt, int reason_code);

static uint32_t stage_slot_runtime_diag_snapshot(prometheus_runtime* rt, int reason_code) {
  prom_dom_slot_runtime_diag_snapshot diag_snapshot;
  if (rt == NULL) {
    return 0u;
  }
  memset(&diag_snapshot, 0, sizeof(diag_snapshot));
  diag_snapshot.current_slot_id = rt->slot_diag.current_slot_id;
  diag_snapshot.next_slot_id = rt->slot_diag.next_slot_id;
  diag_snapshot.slot_state[0] = (uint32_t)prom_slot_hfsm_current_state(&rt->slots[0]);
  diag_snapshot.slot_state[1] = (uint32_t)prom_slot_hfsm_current_state(&rt->slots[1]);
  diag_snapshot.slot_generation[0] = prom_slot_hfsm_metadata(&rt->slots[0])->generation;
  diag_snapshot.slot_generation[1] = prom_slot_hfsm_metadata(&rt->slots[1])->generation;
  diag_snapshot.slot_valid[0] = prom_slot_hfsm_metadata(&rt->slots[0])->valid;
  diag_snapshot.slot_valid[1] = prom_slot_hfsm_metadata(&rt->slots[1])->valid;
  diag_snapshot.swap_count = rt->slot_diag.swap_count;
  diag_snapshot.max_wip_depth = rt->slot_diag.max_wip_depth;
  diag_snapshot.overwrite_rejection_count = rt->slot_diag.overwrite_rejection_count;
  diag_snapshot.stale_buffer_rejection_count = rt->slot_diag.stale_buffer_rejection_count;
  diag_snapshot.shape_invalidation_count = rt->slot_diag.shape_invalidation_count;
  diag_snapshot.layout_invalidation_count = rt->slot_diag.layout_invalidation_count;
  diag_snapshot.capacity_invalidation_count = rt->slot_diag.capacity_invalidation_count;
  diag_snapshot.inflight_rejection_count = rt->slot_diag.inflight_rejection_count;
  diag_snapshot.cleanup_success_count = rt->slot_diag.cleanup_success_count;
  diag_snapshot.failure_slot_id = rt->slot_diag.failure_slot_id;
  diag_snapshot.failure_reason = rt->slot_diag.failure_reason;
  return prom_dom_slot_stage_runtime_diag(&rt->blackboard, &diag_snapshot, reason_code);
}

static void commit_slot_runtime_diag_snapshot(prometheus_runtime* rt, int reason_code) {
  if (stage_slot_runtime_diag_snapshot(rt, reason_code) != 0u) {
    prom_dom_slot_commit(&rt->blackboard);
  }
}

static int update_async_progress(prometheus_runtime* rt) {
  VkResult vk_result;

  if (rt == NULL) {
    return PROM_ERROR;
  }
  if (rt->async_state != PROM_ASYNC_STATE_SUBMITTED) {
    return PROM_OK;
  }
  if ((rt->vulkan.test_flags & PROM_TESTCFG_FAIL_ASYNC_POLL) != 0u) {
    rt->in_flight_submit = 0u;
    set_async_state(rt, PROM_ASYNC_STATE_FAILED, PROM_STAGE_SUBMIT, PROM_DETAIL_INJECTED_ASYNC_POLL_FAILURE);
    if (rt->slot_diag.async_slot_id >= 0) {
      prom_slot_mark_failure(rt, (uint32_t)rt->slot_diag.async_slot_id, PROM_DETAIL_INJECTED_ASYNC_POLL_FAILURE);
    }
    return PROM_ERROR;
  }
  if (rt->slot_diag.transfer_queue_used != 0u && rt->slot_diag.async_transfer_complete == 0u) {
    vk_result = vkGetFenceStatus(rt->vulkan.device, rt->transfer_submit_fence);
    if (vk_result == VK_SUCCESS) {
      stage_transfer_complete_telemetry(rt, 1u, rt->slot_diag.async_slot_id < 0 ? 0u : (uint32_t)rt->slot_diag.async_slot_id, 0);
    } else if (vk_result == VK_NOT_READY) {
      return PROM_OK;
    } else {
      rt->in_flight_submit = 0u;
      if (rt->slot_diag.async_slot_id >= 0) {
        stage_transfer_failure_telemetry(rt, (uint32_t)rt->slot_diag.async_slot_id, (int)vk_result);
      }
      if (rt->slot_diag.async_slot_id >= 0) {
        prom_slot_mark_failure(rt, (uint32_t)rt->slot_diag.async_slot_id, (int)vk_result);
      }
      set_async_state(rt, PROM_ASYNC_STATE_FAILED, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
  }
  vk_result = vkGetFenceStatus(rt->vulkan.device, rt->submit_fence);
  if (vk_result == VK_SUCCESS) {
    rt->in_flight_submit = 0u;
    if (rt->slot_diag.async_slot_id >= 0 && !prom_slot_mark_complete(rt, (uint32_t)rt->slot_diag.async_slot_id)) {
      prom_slot_mark_failure(rt, (uint32_t)rt->slot_diag.async_slot_id, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
      set_async_state(rt, PROM_ASYNC_STATE_FAILED, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
      return PROM_ERROR;
    }
    set_async_state(rt, PROM_ASYNC_STATE_READY, PROM_STAGE_SUBMIT, rt->async_final_detail);
    return PROM_OK;
  }
  if (vk_result == VK_NOT_READY) {
    return PROM_OK;
  }
  rt->in_flight_submit = 0u;
  if (rt->slot_diag.async_slot_id >= 0) {
    prom_slot_mark_failure(rt, (uint32_t)rt->slot_diag.async_slot_id, (int)vk_result);
  }
  set_async_state(rt, PROM_ASYNC_STATE_FAILED, PROM_STAGE_SUBMIT, (int)vk_result);
  return PROM_ERROR;
}

static uint32_t sync_transfer_diag_from_visible(prometheus_runtime* rt) {
  prom_dom_transfer_queue_snapshot snapshot;
  if (rt == NULL) {
    return 0u;
  }
  if (prom_dom_sgemm_read_visible_transfer_queue_diagnostics(&rt->blackboard, &snapshot) == 0u) {
    return 0u;
  }
  rt->slot_diag.transfer_policy_selected = snapshot.transfer_policy_selected;
  rt->slot_diag.transfer_queue_used = snapshot.transfer_queue_used;
  rt->slot_diag.transfer_fallback_reason = snapshot.transfer_fallback_reason;
  rt->slot_diag.dedicated_transfer_available = snapshot.dedicated_transfer_available;
  rt->slot_diag.transfer_queue_family_index = snapshot.transfer_queue_family_index;
  rt->slot_diag.compute_queue_family_index = snapshot.compute_queue_family_index;
  rt->slot_diag.queue_families_differ = snapshot.queue_families_differ;
  rt->slot_diag.queue_family_handoff_count = snapshot.queue_family_handoff_count;
  rt->slot_diag.transfer_compute_wait_count = snapshot.transfer_compute_wait_count;
  rt->slot_diag.transfer_failure_slot_id = snapshot.transfer_failure_slot_id;
  rt->slot_diag.transfer_failure_reason = snapshot.transfer_failure_reason;
  rt->slot_diag.transfer_failure_count = snapshot.transfer_failure_count;
  rt->slot_diag.async_transfer_complete = snapshot.async_transfer_complete;
  rt->slot_diag.async_transfer_completion_generation = snapshot.async_transfer_completion_generation;
  return 1u;
}

static void commit_transfer_runtime_telemetry(prometheus_runtime* rt) {
  if (rt == NULL) {
    return;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  sync_transfer_diag_from_visible(rt);
}

static void stage_transfer_complete_telemetry(prometheus_runtime* rt, uint32_t complete, uint32_t slot_id, int reason_code) {
  if (rt == NULL) {
    return;
  }
  if (complete != 0u) {
    rt->slot_diag.async_transfer_completion_generation += 1u;
  }
  if (prom_dom_sgemm_stage_transfer_complete(&rt->blackboard,
                                             complete,
                                             rt->slot_diag.async_transfer_completion_generation,
                                             slot_id,
                                             reason_code) == 0u) {
    return;
  }
  commit_transfer_runtime_telemetry(rt);
}

static void stage_transfer_handoff_telemetry(prometheus_runtime* rt, uint32_t slot_id, int reason_code, uint64_t handoff_delta) {
  if (rt == NULL) {
    return;
  }
  rt->slot_diag.queue_family_handoff_count += handoff_delta;
  if (prom_dom_sgemm_stage_transfer_handoff(&rt->blackboard, rt->slot_diag.queue_family_handoff_count, slot_id, reason_code) == 0u) {
    return;
  }
  commit_transfer_runtime_telemetry(rt);
}

static void stage_transfer_wait_telemetry(prometheus_runtime* rt, uint32_t slot_id, int reason_code) {
  if (rt == NULL) {
    return;
  }
  rt->slot_diag.transfer_compute_wait_count += 1u;
  if (prom_dom_sgemm_stage_transfer_wait(&rt->blackboard, rt->slot_diag.transfer_compute_wait_count, slot_id, reason_code) == 0u) {
    return;
  }
  commit_transfer_runtime_telemetry(rt);
}

static void stage_transfer_failure_telemetry(prometheus_runtime* rt, uint32_t slot_id, int reason_code) {
  if (rt == NULL) {
    return;
  }
  rt->slot_diag.transfer_failure_slot_id = (int)slot_id;
  rt->slot_diag.transfer_failure_reason = reason_code;
  rt->slot_diag.transfer_failure_count += 1u;
  if (prom_dom_sgemm_stage_transfer_failure(&rt->blackboard,
                                            rt->slot_diag.transfer_failure_slot_id,
                                            rt->slot_diag.transfer_failure_reason,
                                            rt->slot_diag.transfer_failure_count) == 0u) {
    return;
  }
  commit_transfer_runtime_telemetry(rt);
}

// ============================================================================
// SGEMM Typed Arena / Buffer Artifact Ownership
// ============================================================================

static int checked_float_buffer_size(uint32_t rows, uint32_t cols, VkDeviceSize* out_vk_size, size_t* out_copy_size) {
  uint32_t elements;
  uint64_t bytes;

  if (out_vk_size == NULL || out_copy_size == NULL) {
    return 0;
  }
  if (!prom_vk_checked_mul_u32(rows, cols, &elements)) {
    return 0;
  }
  bytes = (uint64_t)elements * (uint64_t)sizeof(float);
  if (bytes > (uint64_t)SIZE_MAX) {
    return 0;
  }

  *out_copy_size = (size_t)bytes;
  *out_vk_size = (VkDeviceSize)bytes;
  return 1;
}

static int checked_packed_fp16_buffer_size(uint32_t rows, uint32_t cols, VkDeviceSize* out_vk_size, size_t* out_copy_size) {
  uint32_t elements;
  uint64_t words;
  uint64_t bytes;
  if (out_vk_size == NULL || out_copy_size == NULL) {
    return 0;
  }
  if (!prom_vk_checked_mul_u32(rows, cols, &elements)) {
    return 0;
  }
  words = ((uint64_t)elements + 1u) / 2u;
  bytes = words * (uint64_t)sizeof(uint32_t);
  if (bytes > (uint64_t)SIZE_MAX) {
    return 0;
  }
  *out_copy_size = (size_t)bytes;
  *out_vk_size = (VkDeviceSize)bytes;
  return 1;
}

static uint32_t prom_slot_other_id(uint32_t slot_id) {
  return slot_id == 0u ? 1u : 0u;
}

static uint32_t prom_slot_compute_layout_code(prom_vk_path_mode path, prom_vk_compute_mode compute_mode) {
  return ((uint32_t)path << 16u) | (uint32_t)compute_mode;
}

static uint32_t prom_slot_wip_depth(const prometheus_runtime* rt) {
  uint32_t depth = 0u;
  uint32_t i;
  for (i = 0u; i < 2u; ++i) {
    const prom_slot_state state = prom_slot_hfsm_current_state(&rt->slots[i]);
    if (state == PROM_SLOT_PREPARING || state == PROM_SLOT_READY || state == PROM_SLOT_CURRENT || state == PROM_SLOT_IN_FLIGHT) {
      depth += 1u;
    }
  }
  return depth;
}

static void prom_slot_stage_commit_event(prometheus_runtime* rt,
                                         uint32_t slot_id,
                                         prom_dom_event_kind event_kind,
                                         prom_slot_state state,
                                         int reason_code,
                                         uint32_t has_current_slot,
                                         uint32_t current_slot_id,
                                         uint32_t has_next_slot,
                                         uint32_t next_slot_id) {
  const prom_slot_metadata* metadata;
  if (rt == NULL || slot_id >= 2u) {
    return;
  }

  metadata = prom_slot_hfsm_metadata(&rt->slots[slot_id]);
  if (metadata == NULL) {
    return;
  }

  if (prom_dom_slot_stage_lifecycle(&rt->blackboard,
                                    event_kind,
                                    slot_id,
                                    state,
                                    metadata,
                                    has_current_slot,
                                    current_slot_id,
                                    has_next_slot,
                                    next_slot_id,
                                    reason_code) == 0u) {
    return;
  }

  if (stage_slot_runtime_diag_snapshot(rt, reason_code) == 0u) {
    return;
  }

  prom_dom_slot_commit(&rt->blackboard);
}

static void prom_slot_track_wip(prometheus_runtime* rt) {
  const uint64_t depth = (uint64_t)prom_slot_wip_depth(rt);
  if (depth > rt->slot_diag.max_wip_depth) {
    rt->slot_diag.max_wip_depth = depth;
  }
}

static int prom_slot_cleanup_to_empty(prometheus_runtime* rt, prom_slot_hfsm* slot) {
  prom_slot_state state;
  if (slot == NULL) {
    return 0;
  }
  state = prom_slot_hfsm_current_state(slot);
  if (state == PROM_SLOT_EMPTY) {
    return 1;
  }
  if (state == PROM_SLOT_IN_FLIGHT) {
    return 0;
  }
  if (prom_slot_hfsm_cleanup(slot) == 0u) {
    return 0;
  }
  if (rt != NULL) {
    rt->slot_diag.cleanup_success_count += 1u;
  }
  prom_slot_stage_commit_event(rt,
                               prom_slot_hfsm_metadata(slot)->slot_id,
                               PROM_DOM_EVENT_SLOT_CLEANUP,
                               PROM_SLOT_EMPTY,
                               0,
                               0u,
                               0u,
                               0u,
                               0u);
  return 1;
}

static int prom_slot_prepare_for_call(prometheus_runtime* rt,
                                      uint32_t slot_id,
                                      uint32_t m,
                                      uint32_t n,
                                      uint32_t k,
                                      uint32_t layout_code,
                                      uint32_t precision_code,
                                      uint64_t required_capacity_bytes) {
  prom_slot_hfsm* slot;
  prom_slot_metadata metadata;
  const prom_slot_metadata* existing;
  const prom_slot_state state = prom_slot_hfsm_current_state(&rt->slots[slot_id]);
  int invalidation_reason = 0;

  if (state == PROM_SLOT_IN_FLIGHT || state == PROM_SLOT_CURRENT) {
    rt->slot_diag.inflight_rejection_count += 1u;
    commit_slot_runtime_diag_snapshot(rt, PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED);
    return PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED;
  }

  slot = &rt->slots[slot_id];
  existing = prom_slot_hfsm_metadata(slot);
  if (existing->valid != 0u) {
    if (existing->shape.m != m || existing->shape.n != n || existing->shape.k != k) {
      rt->slot_diag.shape_invalidation_count += 1u;
      invalidation_reason = PROM_DETAIL_SLOT_STALE_REJECTED;
    }
    if (existing->layout.layout != layout_code) {
      rt->slot_diag.layout_invalidation_count += 1u;
      invalidation_reason = PROM_DETAIL_SLOT_INVALID_LAYOUT;
    }
    if (existing->required_capacity_bytes < required_capacity_bytes) {
      rt->slot_diag.capacity_invalidation_count += 1u;
      invalidation_reason = PROM_DETAIL_SLOT_STALE_REJECTED;
    }
    if (invalidation_reason != 0) {
      prom_slot_hfsm_mark_invalidated(slot, invalidation_reason);
      prom_slot_stage_commit_event(rt,
                                   slot_id,
                                   PROM_DOM_EVENT_SLOT_INVALIDATED,
                                   prom_slot_hfsm_current_state(slot),
                                   invalidation_reason,
                                   0u,
                                   0u,
                                   0u,
                                   0u);
      rt->slot_diag.stale_buffer_rejection_count += 1u;
    }
  }

  if (!prom_slot_cleanup_to_empty(rt, slot)) {
    if (prom_slot_hfsm_current_state(slot) == PROM_SLOT_IN_FLIGHT) {
      rt->slot_diag.inflight_rejection_count += 1u;
      commit_slot_runtime_diag_snapshot(rt, PROM_DETAIL_SLOT_INFLIGHT_REJECTED);
      return PROM_DETAIL_SLOT_INFLIGHT_REJECTED;
    }
    rt->slot_diag.overwrite_rejection_count += 1u;
    return PROM_DETAIL_SLOT_OVERWRITE_REJECTED;
  }
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_PREPARING) == 0u) {
    rt->slot_diag.overwrite_rejection_count += 1u;
    return PROM_DETAIL_SLOT_OVERWRITE_REJECTED;
  }
  prom_slot_stage_commit_event(rt,
                               slot_id,
                               PROM_DOM_EVENT_SLOT_PREPARED,
                               PROM_SLOT_PREPARING,
                               0,
                               0u,
                               0u,
                               0u,
                               0u);

  metadata = *prom_slot_hfsm_metadata(slot);
  metadata.slot_id = slot_id;
  metadata.generation += 1u;
  metadata.valid = 1u;
  metadata.shape.m = m;
  metadata.shape.n = n;
  metadata.shape.k = k;
  metadata.layout.layout = layout_code;
  metadata.layout.precision = precision_code;
  metadata.required_capacity_bytes = required_capacity_bytes;
  metadata.failure_reason = 0;
  prom_slot_hfsm_set_metadata(slot, &metadata);

  if (prom_slot_hfsm_transition(slot, PROM_SLOT_READY) == 0u) {
    rt->slot_diag.overwrite_rejection_count += 1u;
    return PROM_DETAIL_SLOT_OVERWRITE_REJECTED;
  }
  rt->slot_diag.next_slot_id = slot_id;
  prom_slot_track_wip(rt);
  prom_slot_stage_commit_event(rt,
                               slot_id,
                               PROM_DOM_EVENT_SLOT_READY,
                               PROM_SLOT_READY,
                               0,
                               0u,
                               0u,
                               1u,
                               slot_id);
  return 0;
}

static int prom_slot_swap_ready_to_current(prometheus_runtime* rt, uint32_t slot_id) {
  prom_slot_hfsm* slot = &rt->slots[slot_id];
  if (prom_slot_hfsm_current_state(slot) != PROM_SLOT_READY) {
    rt->slot_diag.stale_buffer_rejection_count += 1u;
    return 0;
  }
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_CURRENT) == 0u) {
    return 0;
  }
  rt->slot_diag.current_slot_id = slot_id;
  rt->slot_diag.next_slot_id = prom_slot_other_id(slot_id);
  rt->slot_diag.swap_count += 1u;
  prom_slot_track_wip(rt);
  prom_slot_stage_commit_event(rt,
                               slot_id,
                               PROM_DOM_EVENT_SLOT_PROMOTED_CURRENT,
                               PROM_SLOT_CURRENT,
                               0,
                               1u,
                               slot_id,
                               1u,
                               rt->slot_diag.next_slot_id);
  return 1;
}

static void prom_slot_mark_failure(prometheus_runtime* rt, uint32_t slot_id, int reason) {
  prom_slot_hfsm* slot = &rt->slots[slot_id];
  (void)prom_slot_hfsm_fail(slot, reason);
  rt->slot_diag.failure_slot_id = (int)slot_id;
  rt->slot_diag.failure_reason = reason;
  prom_slot_stage_commit_event(rt,
                               slot_id,
                               PROM_DOM_EVENT_SLOT_FAILED,
                               PROM_SLOT_FAILED,
                               reason,
                               0u,
                               0u,
                               0u,
                               0u);
}

static int prom_slot_mark_submitted(prometheus_runtime* rt, uint32_t slot_id) {
  prom_slot_hfsm* slot = &rt->slots[slot_id];
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_IN_FLIGHT) == 0u) {
    return 0;
  }
  prom_slot_track_wip(rt);
  prom_slot_stage_commit_event(rt,
                               slot_id,
                               PROM_DOM_EVENT_SLOT_SUBMITTED,
                               PROM_SLOT_IN_FLIGHT,
                               0,
                               0u,
                               0u,
                               0u,
                               0u);
  rt->slot_diag.async_slot_id = (int)slot_id;
  return 1;
}

static int prom_slot_mark_complete(prometheus_runtime* rt, uint32_t slot_id) {
  prom_slot_hfsm* slot = &rt->slots[slot_id];
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_CONSUMED) == 0u) {
    return 0;
  }
  prom_slot_stage_commit_event(rt,
                               slot_id,
                               PROM_DOM_EVENT_SLOT_COMPLETE,
                               PROM_SLOT_CONSUMED,
                               0,
                               0u,
                               0u,
                               0u,
                               0u);
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_EMPTY) == 0u) {
    return 0;
  }
  prom_slot_stage_commit_event(rt,
                               slot_id,
                               PROM_DOM_EVENT_SLOT_CONSUMED,
                               PROM_SLOT_EMPTY,
                               0,
                               0u,
                               0u,
                               0u,
                               0u);
  if (rt->slot_diag.async_slot_id == (int)slot_id) {
    rt->slot_diag.async_slot_id = -1;
  }
  prom_slot_track_wip(rt);
  return 1;
}

static int prom_buffering_reason_to_detail(prom_buffering_reason_code reason) {
  if (reason == PROM_BUFFERING_REASON_PULL_LAG_LATE_STAGE_STARVATION) {
    return PROM_DETAIL_BUFFERING_PULL_LAG_LATE_STAGE_STARVATION;
  }
  if (reason == PROM_BUFFERING_REASON_PULL_LAG_MEMORY_EDGE_REJECTED) {
    return PROM_DETAIL_BUFFERING_PULL_LAG_MEMORY_EDGE_REJECTED;
  }
  if (reason == PROM_BUFFERING_REASON_PULL_LAG_VARIANCE_MISS) {
    return PROM_DETAIL_BUFFERING_PULL_LAG_VARIANCE_MISS;
  }
  if (reason == PROM_BUFFERING_REASON_PULL_LAG_COMPUTE_UNSTABLE) {
    return PROM_DETAIL_BUFFERING_PULL_LAG_COMPUTE_UNSTABLE;
  }
  if (reason == PROM_BUFFERING_REASON_PULL_LAG_WIP_WASTE_EXCEEDED) {
    return PROM_DETAIL_BUFFERING_PULL_LAG_WIP_WASTE_EXCEEDED;
  }
  if (reason == PROM_BUFFERING_REASON_NO_BUFFERING_MODE_FEASIBLE) {
    return PROM_DETAIL_BUFFERING_NO_MODE_FEASIBLE;
  }
  return 0;
}

// ============================================================================
// SGEMM Layout Precision: Packed4 / FP16
// ============================================================================

uint16_t prom_sgemm_float32_to_fp16_bits(float value) {
  union { float f; uint32_t u; } in;
  uint32_t sign;
  uint32_t exponent;
  uint32_t mantissa;
  in.f = value;
  sign = (in.u >> 16u) & 0x8000u;
  exponent = (in.u >> 23u) & 0xffu;
  mantissa = in.u & 0x7fffffu;
  if (exponent == 0xffu) {
    return (uint16_t)(sign | (mantissa == 0u ? 0x7c00u : 0x7e00u));
  }
  if (exponent > 142u) {
    return (uint16_t)(sign | 0x7c00u);
  }
  if (exponent < 113u) {
    uint32_t shifted;
    uint32_t remainder;
    if (exponent < 103u) {
      return (uint16_t)sign;
    }
    mantissa |= 0x800000u;
    shifted = 126u - exponent;
    remainder = mantissa & ((1u << shifted) - 1u);
    mantissa >>= shifted;
    if (remainder > (1u << (shifted - 1u)) ||
        (remainder == (1u << (shifted - 1u)) && (mantissa & 1u) != 0u)) {
      mantissa += 1u;
    }
    return (uint16_t)(sign | mantissa);
  }
  exponent = exponent - 112u;
  {
    const uint32_t remainder = mantissa & 0x1fffu;
    mantissa >>= 13u;
    if (remainder > 0x1000u ||
        (remainder == 0x1000u && (mantissa & 1u) != 0u)) {
      mantissa += 1u;
    }
  }
  if (mantissa == 0x400u) {
    mantissa = 0u;
    exponent += 1u;
  }
  if (exponent >= 31u) {
    return (uint16_t)(sign | 0x7c00u);
  }
  return (uint16_t)(sign | (exponent << 10u) | mantissa);
}

float prom_sgemm_fp16_bits_to_float32(uint16_t value) {
  uint32_t sign = ((uint32_t)value & 0x8000u) << 16u;
  uint32_t exponent = ((uint32_t)value >> 10u) & 0x1fu;
  uint32_t mantissa = (uint32_t)value & 0x3ffu;
  union { uint32_t u; float f; } out;
  if (exponent == 0u) {
    if (mantissa == 0u) {
      out.u = sign;
      return out.f;
    }
    exponent = 127u - 15u + 1u;
    while ((mantissa & 0x400u) == 0u) {
      mantissa <<= 1u;
      exponent -= 1u;
    }
    mantissa &= 0x3ffu;
    out.u = sign | (exponent << 23u) | (mantissa << 13u);
    return out.f;
  }
  if (exponent == 31u) {
    out.u = sign | 0x7f800000u | (mantissa << 13u);
    return out.f;
  }
  exponent = exponent + (127u - 15u);
  out.u = sign | (exponent << 23u) | (mantissa << 13u);
  return out.f;
}

static void prom_pack_fp16_pairs(const float* src, uint32_t element_count, uint32_t* dst_words) {
  uint32_t i;
  for (i = 0u; i < element_count; i += 2u) {
    uint16_t lo = prom_sgemm_float32_to_fp16_bits(src[i]);
    uint16_t hi = (i + 1u < element_count) ? prom_sgemm_float32_to_fp16_bits(src[i + 1u]) : (uint16_t)0u;
    dst_words[i / 2u] = (uint32_t)lo | ((uint32_t)hi << 16u);
  }
}

static uint32_t prom_round_up4_u32(uint32_t value) {
  return (value + 3u) & ~3u;
}

static uint32_t prom_packed4_tail_count(uint32_t m, uint32_t n, uint32_t k) {
  uint32_t tails = 0u;
  if ((m & 3u) != 0u) {
    tails += 1u;
  }
  if ((n & 3u) != 0u) {
    tails += 1u;
  }
  if ((k & 3u) != 0u) {
    tails += 1u;
  }
  return tails;
}

static uint32_t prom_packed4_padding_waste_permille(uint32_t m, uint32_t n, uint32_t k) {
  const uint32_t pad_k = (4u - (k & 3u)) & 3u;
  const uint64_t padded_lanes = (uint64_t)pad_k * (uint64_t)(m + n);
  const uint64_t denom = (uint64_t)m * (uint64_t)n;
  if (denom == 0u) {
    return 0u;
  }
  return (uint32_t)((padded_lanes * 1000u) / denom);
}

static uint32_t prom_packed4_mode_budget_permille(prom_policy_mode mode) {
  if (mode == PROM_POLICY_MODE_SAFE) {
    return PROM_SGEMM_PACKED4_MODE_BUDGET_SAFE;
  }
  if (mode == PROM_POLICY_MODE_RECOVERY) {
    return PROM_SGEMM_PACKED4_MODE_BUDGET_RECOVERY;
  }
  return PROM_SGEMM_PACKED4_MODE_BUDGET_AGGRESSIVE;
}

static void prom_packed4_record_fallback(prom_sgemm_controller_state* state, prom_packed4_reject_reason reason) {
  if (state == NULL) {
    return;
  }
  if (reason == PROM_PACKED4_REJECT_PADDING_WASTE) {
    state->packed4_fallback_reason_padding_waste += 1u;
  } else if (reason == PROM_PACKED4_REJECT_SMALL_SHAPE) {
    state->packed4_fallback_reason_small_shape += 1u;
  } else if (reason == PROM_PACKED4_REJECT_CAPABILITY_MISSING) {
    state->packed4_fallback_reason_capability_missing += 1u;
  } else if (reason == PROM_PACKED4_REJECT_FALLBACK_REQUIRED) {
    state->packed4_fallback_reason_fallback_required += 1u;
  } else if (reason == PROM_PACKED4_REJECT_MODE_BUDGET_DENIED) {
    state->packed4_fallback_reason_mode_budget_denied += 1u;
    state->packed4_mode_budget_denials += 1u;
  }
}

static int prom_fp16_reject_reason_to_detail(prom_fp16_reject_reason reason) {
  if (reason == PROM_FP16_REJECT_STRICT_FP32) return PROM_DETAIL_FP16_STRICT_FP32;
  if (reason == PROM_FP16_REJECT_TOLERANCE_UNKNOWN) return PROM_DETAIL_FP16_TOLERANCE_UNKNOWN;
  if (reason == PROM_FP16_REJECT_TOLERANCE_EXCEEDED) return PROM_DETAIL_FP16_TOLERANCE_EXCEEDED;
  if (reason == PROM_FP16_REJECT_SPECIAL_VALUE) return PROM_DETAIL_FP16_SPECIAL_VALUE;
  if (reason == PROM_FP16_REJECT_CAPABILITY_MISSING) return PROM_DETAIL_FP16_CAPABILITY_MISSING;
  if (reason == PROM_FP16_REJECT_FALLBACK_REQUIRED) return PROM_DETAIL_FP16_FALLBACK_REQUIRED;
  if (reason == PROM_FP16_REJECT_NOT_TOP_UTILITY) return PROM_DETAIL_FP16_NOT_TOP_UTILITY;
  return 0;
}

static uint32_t prom_fp16_scan_special_values(const float* values, uint64_t count) {
  uint64_t index;
  if (values == NULL) {
    return 1u;
  }
  for (index = 0u; index < count; ++index) {
    if (!isfinite(values[index])) {
      return 1u;
    }
  }
  return 0u;
}

static void prom_fp16_prepare_production_tolerance_facts(const float* a,
                                                         const float* b,
                                                         uint32_t m,
                                                         uint32_t n,
                                                         uint32_t k,
                                                         prom_sgemm_controller_state* state,
                                                         uint32_t* has_special_values,
                                                         int* utility_score) {
  uint64_t index;
  float max_a = 0.0f;
  float max_b = 0.0f;
  float max_a_error = 0.0f;
  float max_b_error = 0.0f;
  double conservative_error;
  const float output_absolute_tolerance = 1.0e-4f;
  if (state == NULL || has_special_values == NULL || utility_score == NULL) {
    return;
  }

  *has_special_values =
      prom_fp16_scan_special_values(a, (uint64_t)m * (uint64_t)k) != 0u ||
      prom_fp16_scan_special_values(b, (uint64_t)k * (uint64_t)n) != 0u
          ? 1u
          : 0u;
  for (index = 0u; index < (uint64_t)m * (uint64_t)k; ++index) {
    const float packed = prom_sgemm_fp16_bits_to_float32(prom_sgemm_float32_to_fp16_bits(a[index]));
    const float magnitude = fabsf(a[index]);
    const float error = fabsf(a[index] - packed);
    if (magnitude > max_a) max_a = magnitude;
    if (error > max_a_error) max_a_error = error;
  }
  for (index = 0u; index < (uint64_t)k * (uint64_t)n; ++index) {
    const float packed = prom_sgemm_fp16_bits_to_float32(prom_sgemm_float32_to_fp16_bits(b[index]));
    const float magnitude = fabsf(b[index]);
    const float error = fabsf(b[index] - packed);
    if (magnitude > max_b) max_b = magnitude;
    if (error > max_b_error) max_b_error = error;
  }
  /* This upper bound compares the original FP32 products to products formed
     from the exact FP16 payload that will be dispatched.  It is deliberately
     conservative: selecting FP16 with a false "zero error" claim is worse
     than selecting the established FP32/packed fallback. */
  conservative_error = (double)k * ((double)max_a_error * (double)max_b +
                                    (double)max_a * (double)max_b_error +
                                    (double)max_a_error * (double)max_b_error);
  state->fp16_tolerance_known = 1u;
  state->fp16_tolerance_pass = conservative_error <= (double)output_absolute_tolerance ? 1u : 0u;
  state->fp16_max_absolute_error = (float)conservative_error;
  state->fp16_max_relative_error = max_a > 0.0f && max_b > 0.0f
                                      ? (float)(conservative_error / ((double)k * (double)max_a * (double)max_b))
                                      : 0.0f;
  state->fp16_aggregate_error = (float)conservative_error;
  state->fp16_worst_case_element_index = 0u;
  state->fp16_k_error_growth = (float)k;
  state->fp16_cancellation_risk = 0.0f;
  *utility_score = 900;
}

static void prom_compute_scalar_row_major(const float* a, const float* b, float* c, uint32_t m, uint32_t n, uint32_t k) {
  uint32_t row;
  for (row = 0u; row < m; ++row) {
    uint32_t col;
    for (col = 0u; col < n; ++col) {
      float sum = 0.0f;
      uint32_t kk;
      for (kk = 0u; kk < k; ++kk) {
        sum += a[row * k + kk] * b[kk * n + col];
      }
      c[row * n + col] = sum;
    }
  }
}

static void prom_pack_a_packed4_rowmajor(const float* src, float* dst, uint32_t m, uint32_t k, uint32_t k4) {
  uint32_t row;
  memset(dst, 0, (size_t)m * (size_t)k4 * sizeof(float));
  for (row = 0u; row < m; ++row) {
    memcpy(dst + (size_t)row * (size_t)k4, src + (size_t)row * (size_t)k, (size_t)k * sizeof(float));
  }
}

static void prom_pack_b_packed4_colmajor(const float* src, float* dst, uint32_t n, uint32_t k, uint32_t k4) {
  uint32_t col;
  memset(dst, 0, (size_t)n * (size_t)k4 * sizeof(float));
  for (col = 0u; col < n; ++col) {
    uint32_t kk;
    float* dst_col = dst + (size_t)col * (size_t)k4;
    for (kk = 0u; kk < k; ++kk) {
      dst_col[kk] = src[(size_t)kk * (size_t)n + (size_t)col];
    }
  }
}

static void prom_apply_debug_row_major_oracle(prometheus_runtime* rt,
                                              const float* a,
                                              const float* b,
                                              float* c,
                                              uint32_t m,
                                              uint32_t n,
                                              uint32_t k) {
  size_t compare_index;
  size_t compare_len = (size_t)m * (size_t)n;
  float* row_major_oracle;
  if (rt == NULL || (rt->vulkan.test_flags & PROM_TESTCFG_PACKED4_DEBUG_ORACLE_CHECK) == 0u) {
    return;
  }
  row_major_oracle = (float*)malloc(compare_len * sizeof(float));
  if (row_major_oracle == NULL) {
    return;
  }
  prom_compute_scalar_row_major(a, b, row_major_oracle, m, n, k);
  for (compare_index = 0u; compare_index < compare_len; ++compare_index) {
    if (c[compare_index] != row_major_oracle[compare_index]) {
      rt->sgemm_controller.packed4_row_major_check_failures += 1u;
      c[compare_index] = row_major_oracle[compare_index];
    }
  }
  free(row_major_oracle);
}

// ============================================================================
// SGEMM Policy / Judgment Fact Building
// ============================================================================

static prom_sgemm_controller_defaults prom_sgemm_default_config(void) {
  prom_sgemm_controller_defaults defaults;
  defaults.lookahead_default = PROM_SGEMM_LOOKAHEAD_DEFAULT;
  defaults.lookahead_min = PROM_SGEMM_LOOKAHEAD_MIN;
  defaults.lookahead_max = PROM_SGEMM_LOOKAHEAD_MAX;
  defaults.outstanding_default = PROM_SGEMM_OUTSTANDING_DEFAULT;
  defaults.outstanding_min = PROM_SGEMM_OUTSTANDING_MIN;
  defaults.outstanding_max = PROM_SGEMM_OUTSTANDING_MAX;
  defaults.chunk_default = PROM_SGEMM_CHUNK_DEFAULT;
  defaults.chunk_min = PROM_SGEMM_CHUNK_MIN;
  defaults.chunk_max = PROM_SGEMM_CHUNK_MAX;
  defaults.waste_budget_units = PROM_SGEMM_WASTE_BUDGET_UNITS;
  defaults.retreat_permille = PROM_SGEMM_RETREAT_PERMILLE;
  defaults.recover_permille = PROM_SGEMM_RECOVER_PERMILLE;
  defaults.recovery_window = PROM_SGEMM_RECOVERY_WINDOW;
  return defaults;
}

static uint32_t prom_subtract_saturating_u32(uint32_t left, uint32_t right) {
  return left > right ? left - right : 0u;
}

static uint32_t prom_sgemm_shape_signature(uint32_t m, uint32_t n, uint32_t k) {
  return (m * 31u) ^ (n * 131u) ^ (k * 521u);
}

static uint32_t prom_sgemm_clamp_u32(uint32_t value, uint32_t min_value, uint32_t max_value) {
  if (value < min_value) {
    return min_value;
  }
  if (value > max_value) {
    return max_value;
  }
  return value;
}

static uint32_t prom_sgemm_waste_proxy_units(uint64_t work_units, uint32_t shape_changed, uint32_t software_vulkan) {
  uint32_t base_units = (uint32_t)(work_units / 65536u);
  if (base_units > 24u) {
    base_units = 24u;
  }
  if (shape_changed != 0u) {
    base_units += 10u;
  }
  if (software_vulkan != 0u) {
    base_units += 4u;
  }
  return base_units;
}

static void prom_sgemm_controller_init(prom_sgemm_controller_state* state) {
  prom_sgemm_controller_defaults defaults;
  if (state == NULL) {
    return;
  }
  memset(state, 0, sizeof(*state));
  defaults = prom_sgemm_default_config();
  prom_policy_memory_init(&state->policy_memory, PROM_POLICY_MODE_AGGRESSIVE);
  state->policy_thresholds.retreat_enter_permille = defaults.retreat_permille;
  state->policy_thresholds.retreat_exit_permille =
      defaults.recover_permille > PROM_SGEMM_HYSTERESIS_MARGIN ? defaults.recover_permille : defaults.recover_permille / 2u;
  state->policy_thresholds.recovery_enter_permille = defaults.retreat_permille + PROM_SGEMM_HYSTERESIS_MARGIN;
  state->policy_thresholds.recovery_exit_permille = defaults.recover_permille;
  state->policy_thresholds.min_commit_decisions = 2u;
  state->policy_thresholds.retreat_cooldown_decisions = defaults.recovery_window;
  state->policy_thresholds.recovery_hold_decisions = defaults.recovery_window;
  state->lookahead = defaults.lookahead_default;
  state->outstanding_depth = defaults.outstanding_default;
  state->chunk_size = defaults.chunk_default;
  state->last_mode = PROM_POLICY_MODE_AGGRESSIVE;
}

static prom_policy_mode prom_sgemm_controller_step(prom_sgemm_controller_state* state,
                                                   uint32_t m,
                                                   uint32_t n,
                                                   uint32_t k,
                                                   uint64_t work_units,
                                                   uint32_t software_vulkan) {
  prom_sgemm_controller_defaults defaults;
  uint32_t signature;
  uint32_t shape_changed;
  uint32_t waste_units;
  uint32_t waste_budget;
  prom_policy_mode mode;
  if (state == NULL) {
    return PROM_POLICY_MODE_AGGRESSIVE;
  }

  defaults = prom_sgemm_default_config();
  signature = prom_sgemm_shape_signature(m, n, k);
  shape_changed = state->last_shape_signature == 0u || state->last_shape_signature != signature ? 1u : 0u;

  waste_units = prom_sgemm_waste_proxy_units(work_units, shape_changed, software_vulkan);
  waste_budget = defaults.waste_budget_units;
  state->wasted_work_units_last = waste_units;
  state->wasted_work_units_total += (uint64_t)waste_units;
  if (state->pending_waste_units > waste_budget) {
    state->pending_waste_units = waste_budget;
  }
  if (shape_changed != 0u) {
    state->pending_waste_units += 8u;
    if (state->pending_waste_units > waste_budget) {
      state->pending_waste_units = waste_budget;
    }
    if (state->decision_count != 0u) {
      state->burst_dampening_count += 1u;
    }
  }
  state->pending_waste_units += waste_units / 2u;
  if (state->pending_waste_units > waste_budget) {
    state->pending_waste_units = waste_budget;
  }
  if ((state->pending_waste_units * 1000u) / waste_budget >= defaults.retreat_permille) {
    state->lag_early_warning_count += 1u;
  }

  state->policy_facts.waste_ratio_permille = (waste_units * 1000u) / waste_budget;
  state->policy_facts.pending_waste_ratio_permille = (state->pending_waste_units * 1000u) / waste_budget;
  state->policy_facts.hard_retreat_override = state->pending_waste_units >= waste_budget ? 1u : 0u;
  state->policy_facts.hard_recovery_override = 0u;

  mode = prom_judgment_engine_update_policy_mode(&state->policy_memory, &state->policy_facts, &state->policy_thresholds);
  state->decision_count += 1u;
  if ((uint32_t)mode != state->last_mode) {
    state->transition_count += 1u;
    if (state->transition_count > 1u) {
      state->instability_count += 1u;
    }
  }
  if (mode == PROM_POLICY_MODE_AGGRESSIVE) {
    state->aggressive_mode_decisions += 1u;
    state->lookahead = defaults.lookahead_default;
    state->outstanding_depth = defaults.outstanding_default;
    state->chunk_size = defaults.chunk_default;
  } else if (mode == PROM_POLICY_MODE_SAFE) {
    state->safe_mode_decisions += 1u;
    state->lookahead = 1u;
    state->outstanding_depth = 1u;
    state->chunk_size = shape_changed != 0u ? defaults.chunk_min : 12u;
  } else {
    state->recovery_mode_decisions += 1u;
    state->lookahead = 1u;
    state->outstanding_depth = 1u;
    state->chunk_size = 12u;
    if (state->policy_memory.recovery_cooldown_remaining <= 1u) {
      state->lookahead = defaults.lookahead_default;
      state->outstanding_depth = defaults.outstanding_default;
    }
  }
  if (shape_changed != 0u && state->chunk_size > defaults.chunk_min) {
    state->chunk_size -= 2u;
    if (state->chunk_size < defaults.chunk_min) {
      state->chunk_size = defaults.chunk_min;
    }
  }
  state->lookahead = prom_sgemm_clamp_u32(state->lookahead, defaults.lookahead_min, defaults.lookahead_max);
  state->outstanding_depth =
      prom_sgemm_clamp_u32(state->outstanding_depth, defaults.outstanding_min, defaults.outstanding_max);
  state->chunk_size = prom_sgemm_clamp_u32(state->chunk_size, defaults.chunk_min, defaults.chunk_max);
  if (state->lookahead < defaults.lookahead_min || state->lookahead > defaults.lookahead_max ||
      state->outstanding_depth < defaults.outstanding_min || state->outstanding_depth > defaults.outstanding_max ||
      state->chunk_size < defaults.chunk_min || state->chunk_size > defaults.chunk_max) {
    state->bound_violation_count += 1u;
  }
  if (mode == PROM_POLICY_MODE_SAFE && state->last_mode != PROM_POLICY_MODE_SAFE) {
    state->retreat_count += 1u;
  }
  if (mode == PROM_POLICY_MODE_RECOVERY && state->last_mode != PROM_POLICY_MODE_RECOVERY) {
    state->recovery_count += 1u;
  }
  if (state->pending_waste_units >= waste_budget) {
    state->budget_depletion_count += 1u;
  }
  state->pending_waste_units = prom_subtract_saturating_u32(state->pending_waste_units, waste_units);
  state->last_shape_signature = signature;
  state->last_shape_m = m;
  state->last_shape_n = n;
  state->last_shape_k = k;
  state->last_mode = (uint32_t)mode;
  return mode;
}

static int registry_contains(void* handle) {
  size_t i;
  int found = 0;
  registry_lock();
  for (i = 0; i < PROMETHEUS_MAX_TRACKED_HANDLES; ++i) {
    if (g_active_handles[i] == handle) {
      found = 1;
      break;
    }
  }
  registry_unlock();
  return found;
}

int prom_reactor_runtime_validate_handle(void* handle) {
  if (handle == NULL || !registry_contains(handle)) return 0;
  if (((prometheus_runtime*)handle)->magic != PROMETHEUS_RUNTIME_MAGIC) return 0;
  return 1;
}
int prom_reactor_runtime_get_vk_services(void* handle, prom_vk_runtime_services* out_services) {
  prometheus_runtime* rt;
  if (out_services == NULL) return PROM_ERROR;
  memset(out_services, 0, sizeof(*out_services));
  if (!prom_reactor_runtime_validate_handle(handle)) return PROM_INVALID_HANDLE;

  rt = (prometheus_runtime*)handle;
  out_services->instance = rt->vulkan.instance;
  out_services->physical_device = rt->vulkan.physical_device;
  out_services->device = rt->vulkan.device;
  out_services->compute_queue = rt->vulkan.compute_queue;
  out_services->compute_queue_family_index = rt->vulkan.queue_family_index;
  out_services->compute_command_pool = rt->vulkan.command_pool;
  out_services->transfer_queue = rt->vulkan.transfer_queue;
  out_services->transfer_queue_family_index = rt->vulkan.transfer_queue_family_index;
  out_services->transfer_command_pool = rt->vulkan.transfer_command_pool;
  out_services->transfer_queue_available = rt->vulkan.transfer_queue_enabled;
  out_services->backend_available = rt->vulkan.available;
  out_services->backend_reason_code = rt->vulkan.reason_code;
  out_services->test_flags = rt->vulkan.test_flags;
  out_services->reduction_test_flags = rt->reduction_test_flags;
  out_services->reduction_ring_depth = rt->reduction_ring_depth;
  out_services->timestamp_query_supported = rt->timestamp_query_supported;
  out_services->timestamp_valid_bits = rt->vulkan.timestamp_valid_bits;
  out_services->timestamp_period_ns = rt->vulkan.timestamp_period_ns;
  out_services->validation_enabled = rt->vulkan.validation_enabled;
  out_services->validation_warning_count = rt->vulkan.validation_warning_count;
  out_services->validation_error_count = rt->vulkan.validation_error_count;
  out_services->cooperative_matrix_state = rt->vulkan.cooperative_matrix_state;
  out_services->cooperative_matrix_extension_spec_version = rt->vulkan.cooperative_matrix_extension_spec_version;
  out_services->cooperative_matrix_feature_enabled = rt->vulkan.cooperative_matrix_feature_enabled;
  out_services->cooperative_matrix_shader_float16_enabled = rt->vulkan.cooperative_matrix_shader_float16_enabled;
  out_services->cooperative_matrix_vulkan_memory_model_enabled = rt->vulkan.cooperative_matrix_vulkan_memory_model_enabled;
  out_services->cooperative_matrix_tuple_count = rt->vulkan.cooperative_matrix_tuple_count;
  out_services->cooperative_matrix_selected_m = rt->vulkan.cooperative_matrix_selected_m;
  out_services->cooperative_matrix_selected_n = rt->vulkan.cooperative_matrix_selected_n;
  out_services->cooperative_matrix_selected_k = rt->vulkan.cooperative_matrix_selected_k;
  out_services->subgroup_size = rt->vulkan.subgroup_size;
  out_services->subgroup_supported_stages = rt->vulkan.subgroup_supported_stages;
  out_services->subgroup_supported_operations = rt->vulkan.subgroup_supported_operations;
  out_services->subgroup_compute_supported = rt->vulkan.subgroup_compute_supported;
  out_services->subgroup_arithmetic_supported = rt->vulkan.subgroup_arithmetic_supported;
  out_services->subgroup_basic_supported = rt->vulkan.subgroup_basic_supported;
  out_services->subgroup_shuffle_supported = rt->vulkan.subgroup_shuffle_supported;
  out_services->subgroup_fixed_size_32_admitted = rt->vulkan.subgroup_fixed_size_32_admitted;
  out_services->subgroup_owned_attention_admitted = rt->vulkan.subgroup_owned_attention_admitted;
  out_services->subgroup_owned_attention_topology_proven = rt->vulkan.subgroup_owned_attention_topology_proven;
  out_services->ray_query_state = rt->vulkan.ray_query_state;
  out_services->ray_query_acceleration_structure_extension_supported = rt->vulkan.ray_query_acceleration_structure_extension_supported;
  out_services->ray_query_extension_supported = rt->vulkan.ray_query_extension_supported;
  out_services->ray_query_deferred_host_operations_extension_supported = rt->vulkan.ray_query_deferred_host_operations_extension_supported;
  out_services->ray_query_buffer_device_address_supported = rt->vulkan.ray_query_buffer_device_address_supported;
  out_services->ray_query_acceleration_structure_supported = rt->vulkan.ray_query_acceleration_structure_supported;
  out_services->ray_query_supported = rt->vulkan.ray_query_supported;
  out_services->create_acceleration_structure = rt->vulkan.create_acceleration_structure;
  out_services->destroy_acceleration_structure = rt->vulkan.destroy_acceleration_structure;
  out_services->get_acceleration_structure_build_sizes = rt->vulkan.get_acceleration_structure_build_sizes;
  out_services->cmd_build_acceleration_structures = rt->vulkan.cmd_build_acceleration_structures;
  out_services->get_acceleration_structure_device_address = rt->vulkan.get_acceleration_structure_device_address;

  if (rt->vulkan.available == 0u) return PROM_ERROR;
  if (rt->vulkan.device == VK_NULL_HANDLE || rt->vulkan.compute_queue == VK_NULL_HANDLE || rt->vulkan.command_pool == VK_NULL_HANDLE) {
    return PROM_ERROR;
  }
  return PROM_OK;
}

int prom_reactor_runtime_get_shader_package(void* handle, prom_shader_package** out_package) {
  prometheus_runtime* runtime;
  if (out_package == NULL) return PROM_ERROR;
  *out_package = NULL;
  if (!prom_reactor_runtime_validate_handle(handle)) return PROM_INVALID_HANDLE;
  runtime = (prometheus_runtime*)handle;
  if (runtime->vulkan.shader_package == NULL) return PROM_ERROR;
  *out_package = runtime->vulkan.shader_package;
  return PROM_OK;
}

const char* prom_vk_subgroup_owned_attention_admission_reason(
    const prom_vk_runtime_services* services) {
  if (services == NULL || services->subgroup_compute_supported == 0u) return "missing compute-stage subgroup support";
  if (services->subgroup_size != 32u) return "subgroup size is not 32";
  if (services->subgroup_arithmetic_supported == 0u) return "missing subgroup arithmetic support";
  if (services->subgroup_basic_supported == 0u) return "missing subgroup basic support";
  if (services->subgroup_shuffle_supported == 0u) return "missing subgroup shuffle support";
  return NULL;
}

const char* prom_main_attention_route_select(uint32_t requested_route,
                                             const prom_vk_runtime_services* services,
                                             uint32_t token_count, uint32_t head_count,
                                             uint32_t head_width, uint32_t fused_width,
                                             uint32_t output_stride, uint32_t dispatch_groups,
                                             prom_main_attention_route_decision* out_decision) {
  const char* reason = NULL;
  uint32_t fallback = PROM_MAIN_ATTENTION_FALLBACK_NONE;
  if (out_decision == NULL) return "missing route decision output";
  memset(out_decision, 0, sizeof(*out_decision));
  out_decision->requested_route = requested_route;
  if (requested_route != PROM_MAIN_ATTENTION_ROUTE_AUTO &&
      requested_route != PROM_MAIN_ATTENTION_ROUTE_SERIAL_CANONICAL &&
      requested_route != PROM_MAIN_ATTENTION_ROUTE_SUBGROUP_OWNED32) return "unknown main attention route";
  if (requested_route == PROM_MAIN_ATTENTION_ROUTE_SERIAL_CANONICAL) {
    out_decision->selected_route = PROM_MAIN_ATTENTION_ROUTE_SERIAL_CANONICAL;
    out_decision->shader_id = 41u;
    return NULL;
  }
  if (services == NULL) { reason = "missing Vulkan 1.4 runtime services"; fallback = PROM_MAIN_ATTENTION_FALLBACK_RUNTIME_CONTRACT; }
  else if (services->subgroup_compute_supported == 0u) { reason = "missing compute-stage subgroup support"; fallback = PROM_MAIN_ATTENTION_FALLBACK_SUBGROUP_COMPUTE; }
  else if (services->subgroup_size != 32u) { reason = "subgroup size is not 32"; fallback = PROM_MAIN_ATTENTION_FALLBACK_SUBGROUP_SIZE; }
  else if (services->subgroup_arithmetic_supported == 0u) { reason = "missing subgroup arithmetic support"; fallback = PROM_MAIN_ATTENTION_FALLBACK_SUBGROUP_ARITHMETIC; }
  else if (services->subgroup_basic_supported == 0u) { reason = "missing subgroup basic support"; fallback = PROM_MAIN_ATTENTION_FALLBACK_SUBGROUP_BASIC; }
  else if (services->subgroup_shuffle_supported == 0u) { reason = "missing subgroup shuffle support"; fallback = PROM_MAIN_ATTENTION_FALLBACK_SUBGROUP_SHUFFLE; }
  else if (token_count != 1056u || head_count != 30u || head_width != 128u || fused_width != 11520u ||
           output_stride != 3840u || dispatch_groups != 3960u) { reason = "SubgroupOwned32 shape/layout contract mismatch"; fallback = PROM_MAIN_ATTENTION_FALLBACK_SHAPE; }
  if (reason == NULL) {
    out_decision->selected_route = PROM_MAIN_ATTENTION_ROUTE_SUBGROUP_OWNED32;
    out_decision->shader_id = 49u;
    return NULL;
  }
  out_decision->fallback_reason = fallback;
  if (requested_route == PROM_MAIN_ATTENTION_ROUTE_SUBGROUP_OWNED32) return reason;
  out_decision->selected_route = PROM_MAIN_ATTENTION_ROUTE_SERIAL_CANONICAL;
  out_decision->shader_id = 41u;
  return NULL;
}

const char* prom_main_attention_route_asset_rejection_reason(
    const prom_main_attention_route_decision* decision, const prom_shader_asset* asset) {
  if (decision == NULL || decision->selected_route == 0u || decision->shader_id == 0u)
    return "main attention route has no selected shader";
  if (asset == NULL || asset->shader_id != decision->shader_id)
    return "selected main attention shader is absent from the runtime registry";
  if (asset->authority != PROM_SHADER_AUTHORITY_PRODUCTION)
    return "selected main attention shader lacks production authority";
  if (asset->stage != PROM_SHADER_STAGE_COMPUTE)
    return "selected main attention shader is not a compute asset";
  if (asset->source_language != PROM_SHADER_SOURCE_SDSLV)
    return "selected main attention shader is not SDSL-V owned";
  if (asset->package_variant_id == NULL || asset->entry_point == NULL)
    return "selected main attention shader package identity is incomplete";
  return NULL;
}


static int registry_add(void* handle) {
  size_t i;
  int added = 0;
  registry_lock();
  for (i = 0; i < PROMETHEUS_MAX_TRACKED_HANDLES; ++i) {
    if (g_active_handles[i] == NULL) {
      g_active_handles[i] = handle;
      added = 1;
      break;
    }
  }
  registry_unlock();
  return added;
}

static void registry_remove(void* handle) {
  size_t i;
  registry_lock();
  for (i = 0; i < PROMETHEUS_MAX_TRACKED_HANDLES; ++i) {
    if (g_active_handles[i] == handle) {
      g_active_handles[i] = NULL;
      break;
    }
  }
  registry_unlock();
}

// ============================================================================
// Vulkan Common Integration
// ============================================================================

static void destroy_all_execution_buffers(prometheus_runtime* rt) {
  uint32_t i = 0u;
  if (rt == NULL) {
    return;
  }
  prom_vk_destroy_buffer(rt->vulkan.device, &rt->direct_c);
  prom_vk_destroy_buffer(rt->vulkan.device, &rt->direct_b);
  prom_vk_destroy_buffer(rt->vulkan.device, &rt->direct_a);
  prom_vk_destroy_buffer(rt->vulkan.device, &rt->staged_readback_c);
  prom_vk_destroy_buffer(rt->vulkan.device, &rt->staged_upload_b);
  prom_vk_destroy_buffer(rt->vulkan.device, &rt->staged_upload_a);
  prom_vk_destroy_buffer(rt->vulkan.device, &rt->staged_device_c);
  prom_vk_destroy_buffer(rt->vulkan.device, &rt->staged_device_b);
  prom_vk_destroy_buffer(rt->vulkan.device, &rt->staged_device_a);
  rt->has_direct_buffers = 0u;
  rt->has_staged_buffers = 0u;
  memset(&rt->direct_a_key, 0, sizeof(rt->direct_a_key));
  memset(&rt->direct_b_key, 0, sizeof(rt->direct_b_key));
  memset(&rt->direct_c_key, 0, sizeof(rt->direct_c_key));
  memset(&rt->staged_a_key, 0, sizeof(rt->staged_a_key));
  memset(&rt->staged_b_key, 0, sizeof(rt->staged_b_key));
  memset(&rt->staged_c_key, 0, sizeof(rt->staged_c_key));
  for (i = 0u; i < PROM_ARENA_ROLE_COUNT; ++i) {
    rt->arenas[i].capacity_bytes = 0u;
    rt->arenas[i].required_bytes = 0u;
    rt->arenas[i].committed_live_bytes = 0u;
    rt->arenas[i].valid = 0u;
    rt->arenas[i].in_flight = 0u;
    rt->arenas[i].artifact_key_valid = 0u;
  }
  rt->slot_diag.p11_m3_total_committed_bytes = 0u;
}

static int ensure_buffer_capacity(const prom_vk_buffer* buffer, VkDeviceSize required_size) {
  if (buffer == NULL) {
    return 0;
  }
  return buffer->buffer != VK_NULL_HANDLE && buffer->memory != VK_NULL_HANDLE && buffer->size >= required_size;
}

static uint64_t arena_total_committed_bytes(const prometheus_runtime* rt) {
  uint64_t total = 0u;
  uint32_t i = 0u;
  if (rt == NULL) {
    return 0u;
  }
  for (i = 0u; i < PROM_ARENA_ROLE_COUNT; ++i) {
    total += rt->arenas[i].capacity_bytes;
  }
  return total;
}

static uint64_t arena_total_committed_bytes_masked(const prometheus_runtime* rt, uint32_t role_mask) {
  uint64_t total = 0u;
  uint32_t i = 0u;
  if (rt == NULL) {
    return 0u;
  }
  for (i = 0u; i < PROM_ARENA_ROLE_COUNT; ++i) {
    if ((role_mask & (1u << i)) == 0u) {
      continue;
    }
    total += rt->arenas[i].capacity_bytes;
  }
  return total;
}

static void arena_track_required(prom_typed_arena* arena, const prom_buffer_artifact_key* required) {
  if (arena == NULL || required == NULL) {
    return;
  }
  arena->required_bytes = required->required_bytes;
}

static void arena_commit_key(prom_typed_arena* arena, const prom_buffer_artifact_key* required) {
  if (arena == NULL || required == NULL) {
    return;
  }
  arena->artifact_key_valid = required->valid;
  arena->artifact_key_m = required->m;
  arena->artifact_key_n = required->n;
  arena->artifact_key_k = required->k;
  arena->artifact_key_compute_or_padded_k = required->compute_or_padded_k;
  arena->artifact_key_required_bytes = required->required_bytes;
  arena->layout_namespace = required->layout;
  arena->precision_namespace = required->precision;
  arena->valid = 1u;
}

static int arena_compatible(const prom_typed_arena* arena,
                            const prom_buffer_artifact_key* required,
                            prom_arena_memory_class memory_class,
                            int owner_slot_id,
                            uint32_t allow_inflight_owner_reuse) {
  if (arena == NULL || required == NULL || arena->valid == 0u || required->valid == 0u) {
    return 0;
  }
  if (arena->layout_namespace != required->layout || arena->precision_namespace != required->precision) {
    return 0;
  }
  if (arena->memory_class != memory_class) {
    return 0;
  }
  if (arena->artifact_key_valid == 0u ||
      ((arena->role == PROM_ARENA_ROLE_A) &&
       (arena->artifact_key_m != required->m ||
        arena->artifact_key_k != required->k ||
        arena->artifact_key_compute_or_padded_k != required->compute_or_padded_k)) ||
      ((arena->role == PROM_ARENA_ROLE_B) &&
       (arena->artifact_key_n != required->n ||
        arena->artifact_key_k != required->k ||
        arena->artifact_key_compute_or_padded_k != required->compute_or_padded_k)) ||
      ((arena->role == PROM_ARENA_ROLE_C) &&
       (arena->artifact_key_m != required->m ||
        arena->artifact_key_n != required->n))) {
    return 0;
  }
  if (arena->capacity_bytes < required->required_bytes) {
    return 0;
  }
  if (arena->owner_slot_id >= 0 && arena->owner_slot_id != owner_slot_id && arena->in_flight != 0u &&
      allow_inflight_owner_reuse == 0u) {
    return 0;
  }
  return 1;
}

static int arena_budget_allows(const prometheus_runtime* rt,
                               prom_arena_role role,
                               uint64_t required_bytes,
                               uint32_t active_role_mask,
                               uint64_t* projected_out) {
  uint64_t total_committed = 0u;
  uint64_t projected = 0u;
  uint64_t old_capacity = 0u;
  if (rt == NULL || role >= PROM_ARENA_ROLE_COUNT || (active_role_mask & (1u << role)) == 0u) {
    return 0;
  }
  total_committed = arena_total_committed_bytes_masked(rt, active_role_mask);
  old_capacity = rt->arenas[role].capacity_bytes;
  projected = total_committed - old_capacity + required_bytes;
  if (projected_out != NULL) {
    *projected_out = projected;
  }
  return projected <= rt->arena_budget_limit_bytes;
}

static void arena_after_capacity_change(prometheus_runtime* rt, prom_typed_arena* arena, uint64_t new_capacity) {
  if (rt == NULL || arena == NULL) {
    return;
  }
  arena->capacity_bytes = new_capacity;
  arena->committed_live_bytes = new_capacity;
  arena->generation += 1u;
  rt->slot_diag.p11_m3_total_committed_bytes = arena_total_committed_bytes(rt);
  rt->slot_diag.p11_m3_budget_limit_bytes = rt->arena_budget_limit_bytes;
}

static int arena_compute_shrink_target(prometheus_runtime* rt, prom_typed_arena* arena, uint64_t* out_shrink_target) {
  uint64_t shrink_target = 0u;
  uint32_t low_usage_eligible = 0u;
  if (out_shrink_target == NULL) {
    return 0;
  }
  *out_shrink_target = 0u;
  if (rt == NULL || arena == NULL || arena->valid == 0u) {
    return 0;
  }
  low_usage_eligible = (arena->required_bytes > 0u && arena->capacity_bytes > (2u * arena->required_bytes)) ? 1u : 0u;
  if (arena->in_flight != 0u) {
    if (low_usage_eligible != 0u) {
      arena->ownership_rejection_count += 1u;
    }
    arena->low_usage_epoch_count = 0u;
    return 0;
  }
  if (arena->shrink_cooldown_epochs > 0u) {
    arena->shrink_cooldown_epochs -= 1u;
  }
  if (low_usage_eligible != 0u) {
    arena->low_usage_epoch_count += 1u;
  } else {
    arena->low_usage_epoch_count = 0u;
  }
  if (arena->low_usage_epoch_count < rt->arena_shrink_low_usage_threshold_epochs ||
      arena->shrink_cooldown_epochs != 0u) {
    return 0;
  }
  shrink_target = arena->required_bytes;
  if (shrink_target < rt->arena_floor_bytes) {
    shrink_target = rt->arena_floor_bytes;
  }
  if (shrink_target >= arena->capacity_bytes) {
    arena->low_usage_epoch_count = 0u;
    return 0;
  }
  *out_shrink_target = shrink_target;
  return 1;
}

static void arena_finish_shrink(prometheus_runtime* rt, prom_typed_arena* arena, uint64_t shrink_target) {
  if (rt == NULL || arena == NULL) {
    return;
  }
  arena->capacity_bytes = shrink_target;
  arena->committed_live_bytes = shrink_target;
  arena->generation += 1u;
  arena->shrink_count += 1u;
  arena->low_usage_epoch_count = 0u;
  arena->shrink_cooldown_epochs = rt->arena_shrink_cooldown_epochs;
  rt->slot_diag.p11_m3_total_committed_bytes = arena_total_committed_bytes(rt);
}

static int arena_shrink_single_buffer(prometheus_runtime* rt,
                                      prom_typed_arena* arena,
                                      prom_vk_buffer* buffer,
                                      VkBufferUsageFlags usage,
                                      VkMemoryPropertyFlags memory_props,
                                      int map_memory) {
  prom_vk_buffer replacement;
  uint64_t shrink_target = 0u;
  VkResult result;
  if (rt == NULL || arena == NULL || buffer == NULL) {
    return 0;
  }
  if (!arena_compute_shrink_target(rt, arena, &shrink_target)) {
    return 1;
  }
  memset(&replacement, 0, sizeof(replacement));
  result = prom_vk_create_buffer(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags,
                         (VkDeviceSize)shrink_target,
                         usage,
                         memory_props,
                         map_memory,
                         &replacement);
  if (result != VK_SUCCESS) {
    arena->last_failure_reason = (int)result;
    return 0;
  }
  prom_vk_destroy_buffer(rt->vulkan.device, buffer);
  *buffer = replacement;
  arena_finish_shrink(rt, arena, shrink_target);
  return 1;
}

static int arena_shrink_paired_buffers(prometheus_runtime* rt,
                                       prom_typed_arena* arena,
                                       uint64_t first_required_bytes,
                                       prom_vk_buffer* first,
                                       VkBufferUsageFlags first_usage,
                                       VkMemoryPropertyFlags first_memory_props,
                                       int first_map_memory,
                                       uint64_t second_required_bytes,
                                       prom_vk_buffer* second,
                                       VkBufferUsageFlags second_usage,
                                       VkMemoryPropertyFlags second_memory_props,
                                       int second_map_memory) {
  prom_vk_buffer replacement_first;
  prom_vk_buffer replacement_second;
  uint64_t shrink_target = 0u;
  VkResult first_result;
  VkResult second_result;
  if (rt == NULL || arena == NULL || first == NULL || second == NULL) {
    return 0;
  }
  if (first_required_bytes != second_required_bytes) {
    /* P11 M3 staging currently models paired upload/device (or device/readback) buffers with symmetric required sizes. */
    arena->last_failure_reason = VK_ERROR_UNKNOWN;
    return 0;
  }
  if (!arena_compute_shrink_target(rt, arena, &shrink_target)) {
    return 1;
  }
  memset(&replacement_first, 0, sizeof(replacement_first));
  memset(&replacement_second, 0, sizeof(replacement_second));
  first_result = prom_vk_create_buffer(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags,
                               (VkDeviceSize)shrink_target,
                               first_usage,
                               first_memory_props,
                               first_map_memory,
                               &replacement_first);
  if (first_result != VK_SUCCESS) {
    arena->last_failure_reason = (int)first_result;
    return 0;
  }
  second_result = prom_vk_create_buffer(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags,
                                (VkDeviceSize)shrink_target,
                                second_usage,
                                second_memory_props,
                                second_map_memory,
                                &replacement_second);
  if (second_result != VK_SUCCESS) {
    prom_vk_destroy_buffer(rt->vulkan.device, &replacement_first);
    arena->last_failure_reason = (int)second_result;
    return 0;
  }
  prom_vk_destroy_buffer(rt->vulkan.device, first);
  prom_vk_destroy_buffer(rt->vulkan.device, second);
  *first = replacement_first;
  *second = replacement_second;
  arena_finish_shrink(rt, arena, shrink_target);
  return 1;
}

static prom_buffer_artifact_key make_artifact_key(prom_buffer_artifact_kind artifact,
                                                   uint32_t m,
                                                   uint32_t n,
                                                   uint32_t k,
                                                   uint32_t compute_or_padded_k,
                                                   uint32_t layout,
                                                   uint32_t precision,
                                                   VkDeviceSize required_bytes) {
  prom_buffer_artifact_key key;
  memset(&key, 0, sizeof(key));
  key.valid = 1u;
  key.layout = layout;
  key.precision = precision;
  key.required_bytes = (uint64_t)required_bytes;
  if (artifact == PROM_BUFFER_ARTIFACT_A) {
    key.m = m;
    key.k = k;
    key.compute_or_padded_k = compute_or_padded_k;
  } else if (artifact == PROM_BUFFER_ARTIFACT_B) {
    key.n = n;
    key.k = k;
    key.compute_or_padded_k = compute_or_padded_k;
  } else {
    key.m = m;
    key.n = n;
  }
  return key;
}

static int artifact_dependency_equal(const prom_buffer_artifact_key* current,
                                     const prom_buffer_artifact_key* required,
                                     prom_buffer_artifact_kind artifact) {
  if (current == NULL || required == NULL || current->valid == 0u || required->valid == 0u) {
    return 0;
  }
  if (current->layout != required->layout || current->precision != required->precision) {
    return 0;
  }
  if (artifact == PROM_BUFFER_ARTIFACT_A) {
    return current->m == required->m && current->k == required->k &&
           current->compute_or_padded_k == required->compute_or_padded_k;
  }
  if (artifact == PROM_BUFFER_ARTIFACT_B) {
    return current->n == required->n && current->k == required->k &&
           current->compute_or_padded_k == required->compute_or_padded_k;
  }
  return current->m == required->m && current->n == required->n;
}

static uint64_t* artifact_counter_ptr(prom_slot_runtime_diag* diag, prom_buffer_artifact_kind artifact, int reuse_counter) {
  if (diag == NULL) {
    return NULL;
  }
  if (reuse_counter != 0) {
    if (artifact == PROM_BUFFER_ARTIFACT_A) {
      return &diag->m14_a_reuse_count;
    }
    if (artifact == PROM_BUFFER_ARTIFACT_B) {
      return &diag->m14_b_reuse_count;
    }
    return &diag->m14_c_reuse_count;
  }
  if (artifact == PROM_BUFFER_ARTIFACT_A) {
    return &diag->m14_a_invalidation_count;
  }
  if (artifact == PROM_BUFFER_ARTIFACT_B) {
    return &diag->m14_b_invalidation_count;
  }
  return &diag->m14_c_invalidation_count;
}

static uint32_t* artifact_last_reason_ptr(prom_slot_runtime_diag* diag, prom_buffer_artifact_kind artifact) {
  if (diag == NULL) {
    return NULL;
  }
  if (artifact == PROM_BUFFER_ARTIFACT_A) {
    return &diag->m14_a_last_invalidation_reason;
  }
  if (artifact == PROM_BUFFER_ARTIFACT_B) {
    return &diag->m14_b_last_invalidation_reason;
  }
  return &diag->m14_c_last_invalidation_reason;
}

static void record_artifact_reuse(prometheus_runtime* rt,
                                  prom_buffer_artifact_kind artifact,
                                  const prom_buffer_artifact_key* required) {
  uint64_t* reuse_counter;
  if (rt == NULL || required == NULL) {
    return;
  }
  reuse_counter = artifact_counter_ptr(&rt->slot_diag, artifact, 1);
  if (reuse_counter != NULL) {
    *reuse_counter += 1u;
  }
  if (rt->last_execution_shape_valid != 0u &&
      (rt->last_execution_m != required->m || rt->last_execution_n != required->n || rt->last_execution_k != required->k)) {
    rt->slot_diag.m14_false_invalidation_avoided_count += 1u;
  }
}

static prom_buffer_invalidation_reason classify_invalidation_reason(const prom_buffer_artifact_key* current,
                                                                    const prom_buffer_artifact_key* required,
                                                                    const prom_vk_buffer* buffer) {
  if (current == NULL || current->valid == 0u || required == NULL) {
    return PROM_BUFFER_INVALIDATION_REASON_UNINITIALIZED;
  }
  if (!ensure_buffer_capacity(buffer, (VkDeviceSize)required->required_bytes)) {
    return PROM_BUFFER_INVALIDATION_REASON_CAPACITY;
  }
  if (current->layout != required->layout || current->precision != required->precision) {
    return PROM_BUFFER_INVALIDATION_REASON_LAYOUT_PRECISION;
  }
  return PROM_BUFFER_INVALIDATION_REASON_DEPENDENCY;
}

static void record_artifact_invalidation(prometheus_runtime* rt,
                                         prom_buffer_artifact_kind artifact,
                                         prom_buffer_invalidation_reason reason) {
  uint64_t* invalidation_counter;
  uint32_t* last_reason;
  if (rt == NULL) {
    return;
  }
  invalidation_counter = artifact_counter_ptr(&rt->slot_diag, artifact, 0);
  if (invalidation_counter != NULL) {
    *invalidation_counter += 1u;
  }
  if (reason == PROM_BUFFER_INVALIDATION_REASON_CAPACITY) {
    rt->slot_diag.m14_capacity_invalidation_count += 1u;
  }
  if (reason == PROM_BUFFER_INVALIDATION_REASON_LAYOUT_PRECISION) {
    rt->slot_diag.m14_layout_precision_invalidation_count += 1u;
  }
  last_reason = artifact_last_reason_ptr(&rt->slot_diag, artifact);
  if (last_reason != NULL) {
    *last_reason = (uint32_t)reason;
  }
}

static int ensure_direct_execution_buffers(prometheus_runtime* rt,
                                           const prom_buffer_artifact_key* a_required,
                                           const prom_buffer_artifact_key* b_required,
                                           const prom_buffer_artifact_key* c_required,
                                           VkResult* out_result) {
  VkResult result;
  int rebuild_a;
  int rebuild_b;
  int rebuild_c;
  int owner_slot_id;
  uint64_t projected = 0u;
  prom_typed_arena* arena_a;
  prom_typed_arena* arena_b;
  prom_typed_arena* arena_c;

  if (out_result == NULL || rt == NULL || a_required == NULL || b_required == NULL || c_required == NULL) {
    return 0;
  }
  *out_result = VK_SUCCESS;
  rt->arena_last_failure_detail = 0;
  owner_slot_id = rt->slot_diag.current_slot_id == UINT32_MAX ? -1 : (int)rt->slot_diag.current_slot_id;
  arena_a = &rt->arenas[PROM_ARENA_ROLE_A];
  arena_b = &rt->arenas[PROM_ARENA_ROLE_B];
  arena_c = &rt->arenas[PROM_ARENA_ROLE_C];
  arena_track_required(arena_a, a_required);
  arena_track_required(arena_b, b_required);
  arena_track_required(arena_c, c_required);
  arena_a->memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  arena_b->memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  arena_c->memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  arena_a->owner_slot_id = owner_slot_id;
  arena_b->owner_slot_id = owner_slot_id;
  arena_c->owner_slot_id = owner_slot_id;
  arena_a->in_flight = (rt->in_flight_submit != 0u || (rt->vulkan.test_flags & PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT) != 0u) ? 1u : 0u;
  arena_b->in_flight = (rt->in_flight_submit != 0u || (rt->vulkan.test_flags & PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT) != 0u) ? 1u : 0u;
  arena_c->in_flight = (rt->in_flight_submit != 0u || (rt->vulkan.test_flags & PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT) != 0u) ? 1u : 0u;

  if (rt->has_staged_buffers != 0u) {
    destroy_all_execution_buffers(rt);
  }

  rebuild_a = !arena_compatible(arena_a, a_required, PROM_ARENA_MEMORY_HOST_VISIBLE, owner_slot_id, 0u) ||
              !artifact_dependency_equal(&rt->direct_a_key, a_required, PROM_BUFFER_ARTIFACT_A) ||
              !ensure_buffer_capacity(&rt->direct_a, (VkDeviceSize)a_required->required_bytes);
  rebuild_b = !arena_compatible(arena_b, b_required, PROM_ARENA_MEMORY_HOST_VISIBLE, owner_slot_id, 0u) ||
              !artifact_dependency_equal(&rt->direct_b_key, b_required, PROM_BUFFER_ARTIFACT_B) ||
              !ensure_buffer_capacity(&rt->direct_b, (VkDeviceSize)b_required->required_bytes);
  rebuild_c = !arena_compatible(arena_c, c_required, PROM_ARENA_MEMORY_HOST_VISIBLE, owner_slot_id, 0u) ||
              !artifact_dependency_equal(&rt->direct_c_key, c_required, PROM_BUFFER_ARTIFACT_C) ||
              !ensure_buffer_capacity(&rt->direct_c, (VkDeviceSize)c_required->required_bytes);
  if (arena_a->valid != 0u && (arena_a->layout_namespace != a_required->layout || arena_a->precision_namespace != a_required->precision)) {
    arena_a->namespace_rejection_count += 1u;
  }
  if (arena_b->valid != 0u && (arena_b->layout_namespace != b_required->layout || arena_b->precision_namespace != b_required->precision)) {
    arena_b->namespace_rejection_count += 1u;
  }
  if (arena_c->valid != 0u && (arena_c->layout_namespace != c_required->layout || arena_c->precision_namespace != c_required->precision)) {
    arena_c->namespace_rejection_count += 1u;
  }

  if (!rebuild_a) {
    record_artifact_reuse(rt, PROM_BUFFER_ARTIFACT_A, a_required);
    arena_a->reuse_count += 1u;
    arena_commit_key(arena_a, a_required);
  } else {
    record_artifact_invalidation(
        rt, PROM_BUFFER_ARTIFACT_A, classify_invalidation_reason(&rt->direct_a_key, a_required, &rt->direct_a));
    if (!arena_budget_allows(rt,
                             PROM_ARENA_ROLE_A,
                             a_required->required_bytes,
                             PROM_ARENA_BUDGET_ROLE_MASK_DIRECT,
                             &projected)) {
      arena_a->budget_rejection_count += 1u;
      rt->slot_diag.p11_m3_projected_committed_bytes = projected;
      rt->arena_last_failure_detail = PROM_DETAIL_ARENA_BUDGET_REJECTED;
      *out_result = VK_ERROR_OUT_OF_DEVICE_MEMORY;
      return 0;
    }
    prom_vk_destroy_buffer(rt->vulkan.device, &rt->direct_a);
    result = prom_vk_create_buffer(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags,
                           (VkDeviceSize)a_required->required_bytes,
                           VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                           VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                           1,
                           &rt->direct_a);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    rt->direct_a_key = *a_required;
    if (arena_a->capacity_bytes < a_required->required_bytes) {
      arena_a->grow_count += 1u;
    } else {
      arena_a->rebuild_count += 1u;
    }
    arena_after_capacity_change(rt, arena_a, a_required->required_bytes);
    arena_commit_key(arena_a, a_required);
  }

  if (!rebuild_b) {
    record_artifact_reuse(rt, PROM_BUFFER_ARTIFACT_B, b_required);
    arena_b->reuse_count += 1u;
    arena_commit_key(arena_b, b_required);
  } else {
    record_artifact_invalidation(
        rt, PROM_BUFFER_ARTIFACT_B, classify_invalidation_reason(&rt->direct_b_key, b_required, &rt->direct_b));
    if (!arena_budget_allows(rt,
                             PROM_ARENA_ROLE_B,
                             b_required->required_bytes,
                             PROM_ARENA_BUDGET_ROLE_MASK_DIRECT,
                             &projected)) {
      arena_b->budget_rejection_count += 1u;
      rt->slot_diag.p11_m3_projected_committed_bytes = projected;
      rt->arena_last_failure_detail = PROM_DETAIL_ARENA_BUDGET_REJECTED;
      *out_result = VK_ERROR_OUT_OF_DEVICE_MEMORY;
      return 0;
    }
    prom_vk_destroy_buffer(rt->vulkan.device, &rt->direct_b);
    result = prom_vk_create_buffer(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags,
                           (VkDeviceSize)b_required->required_bytes,
                           VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                           VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                           1,
                           &rt->direct_b);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    rt->direct_b_key = *b_required;
    if (arena_b->capacity_bytes < b_required->required_bytes) {
      arena_b->grow_count += 1u;
    } else {
      arena_b->rebuild_count += 1u;
    }
    arena_after_capacity_change(rt, arena_b, b_required->required_bytes);
    arena_commit_key(arena_b, b_required);
  }

  if (!rebuild_c) {
    record_artifact_reuse(rt, PROM_BUFFER_ARTIFACT_C, c_required);
    arena_c->reuse_count += 1u;
    arena_commit_key(arena_c, c_required);
  } else {
    record_artifact_invalidation(
        rt, PROM_BUFFER_ARTIFACT_C, classify_invalidation_reason(&rt->direct_c_key, c_required, &rt->direct_c));
    if (!arena_budget_allows(rt,
                             PROM_ARENA_ROLE_C,
                             c_required->required_bytes,
                             PROM_ARENA_BUDGET_ROLE_MASK_DIRECT,
                             &projected)) {
      arena_c->budget_rejection_count += 1u;
      rt->slot_diag.p11_m3_projected_committed_bytes = projected;
      rt->arena_last_failure_detail = PROM_DETAIL_ARENA_BUDGET_REJECTED;
      *out_result = VK_ERROR_OUT_OF_DEVICE_MEMORY;
      return 0;
    }
    prom_vk_destroy_buffer(rt->vulkan.device, &rt->direct_c);
    result = prom_vk_create_buffer(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags,
                           (VkDeviceSize)c_required->required_bytes,
                           VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                           VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                           1,
                           &rt->direct_c);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    rt->direct_c_key = *c_required;
    if (arena_c->capacity_bytes < c_required->required_bytes) {
      arena_c->grow_count += 1u;
    } else {
      arena_c->rebuild_count += 1u;
    }
    arena_after_capacity_change(rt, arena_c, c_required->required_bytes);
    arena_commit_key(arena_c, c_required);
  }

  rt->has_direct_buffers = 1u;
  rt->has_staged_buffers = 0u;
  if (!rebuild_a) {
    (void)arena_shrink_single_buffer(rt,
                                     arena_a,
                                     &rt->direct_a,
                                     VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                     VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                     1);
  }
  if (!rebuild_b) {
    (void)arena_shrink_single_buffer(rt,
                                     arena_b,
                                     &rt->direct_b,
                                     VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                     VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                     1);
  }
  if (!rebuild_c) {
    (void)arena_shrink_single_buffer(rt,
                                     arena_c,
                                     &rt->direct_c,
                                     VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                     VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                     1);
  }
  rt->slot_diag.p11_m3_total_committed_bytes = arena_total_committed_bytes(rt);
  rt->slot_diag.p11_m3_projected_committed_bytes = rt->slot_diag.p11_m3_total_committed_bytes;
  return 1;
}

static int ensure_staged_execution_buffers(prometheus_runtime* rt,
                                           const prom_buffer_artifact_key* a_required,
                                           const prom_buffer_artifact_key* b_required,
                                           const prom_buffer_artifact_key* c_required,
                                           VkResult* out_result) {
  VkResult result;
  int rebuild_a;
  int rebuild_b;
  int rebuild_c;
  int owner_slot_id;
  uint64_t projected = 0u;
  prom_typed_arena* arena_a;
  prom_typed_arena* arena_b;
  prom_typed_arena* arena_c;
  prom_typed_arena* arena_upload;

  if (out_result == NULL || rt == NULL || a_required == NULL || b_required == NULL || c_required == NULL) {
    return 0;
  }

  *out_result = VK_SUCCESS;
  rt->arena_last_failure_detail = 0;
  owner_slot_id = rt->slot_diag.current_slot_id == UINT32_MAX ? -1 : (int)rt->slot_diag.current_slot_id;
  arena_a = &rt->arenas[PROM_ARENA_ROLE_A];
  arena_b = &rt->arenas[PROM_ARENA_ROLE_B];
  arena_c = &rt->arenas[PROM_ARENA_ROLE_C];
  arena_upload = &rt->arenas[PROM_ARENA_ROLE_UPLOAD];
  arena_track_required(arena_a, a_required);
  arena_track_required(arena_b, b_required);
  arena_track_required(arena_c, c_required);
  arena_upload->required_bytes = a_required->required_bytes + b_required->required_bytes;
  arena_upload->layout_namespace = a_required->layout;
  arena_upload->precision_namespace = a_required->precision;
  arena_a->memory_class = PROM_ARENA_MEMORY_DEVICE_LOCAL;
  arena_b->memory_class = PROM_ARENA_MEMORY_DEVICE_LOCAL;
  arena_c->memory_class = PROM_ARENA_MEMORY_DEVICE_LOCAL;
  arena_upload->memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  arena_a->owner_slot_id = owner_slot_id;
  arena_b->owner_slot_id = owner_slot_id;
  arena_c->owner_slot_id = owner_slot_id;
  arena_upload->owner_slot_id = owner_slot_id;
  arena_a->in_flight = (rt->in_flight_submit != 0u || (rt->vulkan.test_flags & PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT) != 0u) ? 1u : 0u;
  arena_b->in_flight = (rt->in_flight_submit != 0u || (rt->vulkan.test_flags & PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT) != 0u) ? 1u : 0u;
  arena_c->in_flight = (rt->in_flight_submit != 0u || (rt->vulkan.test_flags & PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT) != 0u) ? 1u : 0u;
  arena_upload->in_flight = (rt->in_flight_submit != 0u || (rt->vulkan.test_flags & PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT) != 0u) ? 1u : 0u;

  if (rt->has_direct_buffers != 0u) {
    destroy_all_execution_buffers(rt);
  }

  rebuild_a = !arena_compatible(arena_a, a_required, PROM_ARENA_MEMORY_DEVICE_LOCAL, owner_slot_id, 0u) ||
              !artifact_dependency_equal(&rt->staged_a_key, a_required, PROM_BUFFER_ARTIFACT_A) ||
              !ensure_buffer_capacity(&rt->staged_upload_a, (VkDeviceSize)a_required->required_bytes) ||
              !ensure_buffer_capacity(&rt->staged_device_a, (VkDeviceSize)a_required->required_bytes);
  rebuild_b = !arena_compatible(arena_b, b_required, PROM_ARENA_MEMORY_DEVICE_LOCAL, owner_slot_id, 0u) ||
              !artifact_dependency_equal(&rt->staged_b_key, b_required, PROM_BUFFER_ARTIFACT_B) ||
              !ensure_buffer_capacity(&rt->staged_upload_b, (VkDeviceSize)b_required->required_bytes) ||
              !ensure_buffer_capacity(&rt->staged_device_b, (VkDeviceSize)b_required->required_bytes);
  rebuild_c = !arena_compatible(arena_c, c_required, PROM_ARENA_MEMORY_DEVICE_LOCAL, owner_slot_id, 0u) ||
              !artifact_dependency_equal(&rt->staged_c_key, c_required, PROM_BUFFER_ARTIFACT_C) ||
              !ensure_buffer_capacity(&rt->staged_device_c, (VkDeviceSize)c_required->required_bytes) ||
              !ensure_buffer_capacity(&rt->staged_readback_c, (VkDeviceSize)c_required->required_bytes);
  if (arena_a->valid != 0u && (arena_a->layout_namespace != a_required->layout || arena_a->precision_namespace != a_required->precision)) {
    arena_a->namespace_rejection_count += 1u;
  }
  if (arena_b->valid != 0u && (arena_b->layout_namespace != b_required->layout || arena_b->precision_namespace != b_required->precision)) {
    arena_b->namespace_rejection_count += 1u;
  }
  if (arena_c->valid != 0u && (arena_c->layout_namespace != c_required->layout || arena_c->precision_namespace != c_required->precision)) {
    arena_c->namespace_rejection_count += 1u;
  }

  if (!rebuild_a) {
    record_artifact_reuse(rt, PROM_BUFFER_ARTIFACT_A, a_required);
    arena_a->reuse_count += 1u;
    arena_commit_key(arena_a, a_required);
  } else {
    record_artifact_invalidation(
        rt, PROM_BUFFER_ARTIFACT_A, classify_invalidation_reason(&rt->staged_a_key, a_required, &rt->staged_upload_a));
    if (!arena_budget_allows(rt,
                             PROM_ARENA_ROLE_A,
                             a_required->required_bytes,
                             PROM_ARENA_BUDGET_ROLE_MASK_STAGED,
                             &projected)) {
      arena_a->budget_rejection_count += 1u;
      rt->slot_diag.p11_m3_projected_committed_bytes = projected;
      rt->arena_last_failure_detail = PROM_DETAIL_ARENA_BUDGET_REJECTED;
      *out_result = VK_ERROR_OUT_OF_DEVICE_MEMORY;
      return 0;
    }
    prom_vk_destroy_buffer(rt->vulkan.device, &rt->staged_upload_a);
    prom_vk_destroy_buffer(rt->vulkan.device, &rt->staged_device_a);
    result = prom_vk_create_buffer(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags,
                           (VkDeviceSize)a_required->required_bytes,
                           VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                           VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                           1,
                           &rt->staged_upload_a);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    result = prom_vk_create_buffer(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags,
                           (VkDeviceSize)a_required->required_bytes,
                           VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                           VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                           0,
                           &rt->staged_device_a);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    rt->staged_a_key = *a_required;
    if (arena_a->capacity_bytes < a_required->required_bytes) {
      arena_a->grow_count += 1u;
    } else {
      arena_a->rebuild_count += 1u;
    }
    arena_after_capacity_change(rt, arena_a, a_required->required_bytes);
    arena_commit_key(arena_a, a_required);
  }

  if (!rebuild_b) {
    record_artifact_reuse(rt, PROM_BUFFER_ARTIFACT_B, b_required);
    arena_b->reuse_count += 1u;
    arena_commit_key(arena_b, b_required);
  } else {
    record_artifact_invalidation(
        rt, PROM_BUFFER_ARTIFACT_B, classify_invalidation_reason(&rt->staged_b_key, b_required, &rt->staged_upload_b));
    if (!arena_budget_allows(rt,
                             PROM_ARENA_ROLE_B,
                             b_required->required_bytes,
                             PROM_ARENA_BUDGET_ROLE_MASK_STAGED,
                             &projected)) {
      arena_b->budget_rejection_count += 1u;
      rt->slot_diag.p11_m3_projected_committed_bytes = projected;
      rt->arena_last_failure_detail = PROM_DETAIL_ARENA_BUDGET_REJECTED;
      *out_result = VK_ERROR_OUT_OF_DEVICE_MEMORY;
      return 0;
    }
    prom_vk_destroy_buffer(rt->vulkan.device, &rt->staged_upload_b);
    prom_vk_destroy_buffer(rt->vulkan.device, &rt->staged_device_b);
    result = prom_vk_create_buffer(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags,
                           (VkDeviceSize)b_required->required_bytes,
                           VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                           VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                           1,
                           &rt->staged_upload_b);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    result = prom_vk_create_buffer(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags,
                           (VkDeviceSize)b_required->required_bytes,
                           VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                           VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                           0,
                           &rt->staged_device_b);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    rt->staged_b_key = *b_required;
    if (arena_b->capacity_bytes < b_required->required_bytes) {
      arena_b->grow_count += 1u;
    } else {
      arena_b->rebuild_count += 1u;
    }
    arena_after_capacity_change(rt, arena_b, b_required->required_bytes);
    arena_commit_key(arena_b, b_required);
  }

  if (!rebuild_c) {
    record_artifact_reuse(rt, PROM_BUFFER_ARTIFACT_C, c_required);
    arena_c->reuse_count += 1u;
    arena_commit_key(arena_c, c_required);
  } else {
    record_artifact_invalidation(
        rt, PROM_BUFFER_ARTIFACT_C, classify_invalidation_reason(&rt->staged_c_key, c_required, &rt->staged_readback_c));
    if (!arena_budget_allows(rt,
                             PROM_ARENA_ROLE_C,
                             c_required->required_bytes,
                             PROM_ARENA_BUDGET_ROLE_MASK_STAGED,
                             &projected)) {
      arena_c->budget_rejection_count += 1u;
      rt->slot_diag.p11_m3_projected_committed_bytes = projected;
      rt->arena_last_failure_detail = PROM_DETAIL_ARENA_BUDGET_REJECTED;
      *out_result = VK_ERROR_OUT_OF_DEVICE_MEMORY;
      return 0;
    }
    prom_vk_destroy_buffer(rt->vulkan.device, &rt->staged_device_c);
    prom_vk_destroy_buffer(rt->vulkan.device, &rt->staged_readback_c);
    result = prom_vk_create_buffer(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags,
                           (VkDeviceSize)c_required->required_bytes,
                           VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                           VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                           0,
                           &rt->staged_device_c);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    result = prom_vk_create_buffer(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags,
                           (VkDeviceSize)c_required->required_bytes,
                           VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                           VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                           1,
                           &rt->staged_readback_c);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    rt->staged_c_key = *c_required;
    if (arena_c->capacity_bytes < c_required->required_bytes) {
      arena_c->grow_count += 1u;
    } else {
      arena_c->rebuild_count += 1u;
    }
    arena_after_capacity_change(rt, arena_c, c_required->required_bytes);
    arena_commit_key(arena_c, c_required);
  }

  arena_upload->capacity_bytes = rt->staged_upload_a.size + rt->staged_upload_b.size;
  arena_upload->committed_live_bytes = arena_upload->capacity_bytes;
  arena_upload->generation = arena_a->generation + arena_b->generation;
  arena_upload->reuse_count = arena_a->reuse_count + arena_b->reuse_count;
  arena_upload->grow_count = arena_a->grow_count + arena_b->grow_count;
  arena_upload->rebuild_count = arena_a->rebuild_count + arena_b->rebuild_count;
  arena_upload->shrink_count = arena_a->shrink_count + arena_b->shrink_count;
  arena_upload->valid = 1u;

  rt->has_direct_buffers = 0u;
  rt->has_staged_buffers = 1u;
  if (!rebuild_a) {
    (void)arena_shrink_paired_buffers(rt,
                                      arena_a,
                                      a_required->required_bytes,
                                      &rt->staged_upload_a,
                                      VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                      VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                      1,
                                      a_required->required_bytes,
                                      &rt->staged_device_a,
                                      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                                      0);
  }
  if (!rebuild_b) {
    (void)arena_shrink_paired_buffers(rt,
                                      arena_b,
                                      b_required->required_bytes,
                                      &rt->staged_upload_b,
                                      VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                      VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                      1,
                                      b_required->required_bytes,
                                      &rt->staged_device_b,
                                      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                                      0);
  }
  if (!rebuild_c) {
    (void)arena_shrink_paired_buffers(rt,
                                      arena_c,
                                      c_required->required_bytes,
                                      &rt->staged_device_c,
                                      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                                      0,
                                      c_required->required_bytes,
                                      &rt->staged_readback_c,
                                      VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                      VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                      1);
  }
  arena_upload->capacity_bytes = rt->staged_upload_a.size + rt->staged_upload_b.size;
  arena_upload->committed_live_bytes = arena_upload->capacity_bytes;
  arena_upload->shrink_count = arena_a->shrink_count + arena_b->shrink_count;
  rt->slot_diag.p11_m3_total_committed_bytes = arena_total_committed_bytes(rt);
  rt->slot_diag.p11_m3_projected_committed_bytes = rt->slot_diag.p11_m3_total_committed_bytes;
  return 1;
}

static void note_last_execution_shape(prometheus_runtime* rt, uint32_t m, uint32_t n, uint32_t k) {
  if (rt == NULL) {
    return;
  }
  rt->last_execution_shape_valid = 1u;
  rt->last_execution_m = m;
  rt->last_execution_n = n;
  rt->last_execution_k = k;
}

int prom_reactor_runtime_mark_cooperative_matrix_executable(void* handle) {
  prometheus_runtime* rt;
  if (!prom_reactor_runtime_validate_handle(handle)) return PROM_INVALID_HANDLE;
  rt = (prometheus_runtime*)handle;
  if (rt->vulkan.cooperative_matrix_feature_enabled == 0u ||
      rt->vulkan.cooperative_matrix_state < PROM_VK_COOPERATIVE_MATRIX_DEVICE_FEATURE_ENABLED) return PROM_ERROR;
  rt->vulkan.cooperative_matrix_state = PROM_VK_COOPERATIVE_MATRIX_EXECUTABLE;
  return PROM_OK;
}

static uint64_t prom_wall_clock_now_ns(void) {
#if defined(_WIN32)
  static LARGE_INTEGER frequency;
  static uint32_t frequency_initialized = 0u;
  LARGE_INTEGER counter;
  if (frequency_initialized == 0u) {
    if (QueryPerformanceFrequency(&frequency) == 0) {
      frequency.QuadPart = 0;
    }
    frequency_initialized = 1u;
  }
  if (frequency.QuadPart <= 0 || QueryPerformanceCounter(&counter) == 0) {
    return 0u;
  }
  return (uint64_t)((counter.QuadPart * 1000000000ll) / frequency.QuadPart);
#else
  struct timespec ts;
  if (clock_gettime(CLOCK_MONOTONIC, &ts) != 0) {
    return 0u;
  }
  return ((uint64_t)ts.tv_sec * 1000000000ull) + (uint64_t)ts.tv_nsec;
#endif
}

static uint64_t prom_wall_clock_elapsed_ns(uint64_t start_ns, uint64_t end_ns) {
  if (start_ns == 0u || end_ns == 0u || end_ns < start_ns) {
    return 0u;
  }
  return end_ns - start_ns;
}

static void reset_last_runtime_timing_decomposition(prometheus_runtime* rt) {
  if (rt == NULL) {
    return;
  }
  rt->px16_m8_last_upload_wall_ns = 0u;
  rt->px16_m8_last_pre_dispatch_wall_ns = 0u;
  rt->px16_m8_last_command_record_wall_ns = 0u;
  rt->px16_m8_last_dispatch_submit_wall_ns = 0u;
  rt->px16_m8_last_sync_wait_wall_ns = 0u;
  rt->px16_m8_last_post_sync_wall_ns = 0u;
  rt->px16_m8_last_readback_wall_ns = 0u;
  rt->px16_m8_last_post_readback_wall_ns = 0u;
  rt->px16_m8_last_total_wall_ns = 0u;
  rt->px16_m17_last_tolerance_eval_wall_ns = 0u;
  rt->px16_m17_last_tolerance_eval_in_dispatch = 0u;
  rt->px16_m17_last_tolerance_eval_source = 0u;
}

static void reset_last_gpu_timing(prometheus_runtime* rt, uint32_t failure_reason) {
  if (rt == NULL) {
    return;
  }
  rt->last_gpu_timing_valid = 0u;
  rt->last_gpu_duration_ns = 0u;
  rt->last_gpu_timing_failure_reason = failure_reason;
  rt->p14_last_filtered_evidence.valid = 0u;
}

static void vk_runtime_cleanup(prometheus_runtime* rt) {
  if (rt == NULL) {
    return;
  }
  /* Destruction is the one blocking recovery path: no slot-owned Vulkan
     resource is destroyed until quarantined work has been drained or the
     device teardown path owns the failure. */
  if (rt->vulkan.device != VK_NULL_HANDLE) {
    (void)prom_async_reap_quarantined_slots(rt, 1u);
    prom_vk_runtime_wait_idle(&rt->vulkan);
  }
  if (rt->reduction_state != NULL) {
    prom_reactor_runtime_reduction_cleanup_state(rt->reduction_state, rt->vulkan.device);
    rt->reduction_state = NULL;
  }
  for (uint32_t async_index = 0u; async_index < PROM_SGEMM_ASYNC_MAX_TASKS; ++async_index) {
    prom_vk_destroy_buffer(rt->vulkan.device, &rt->async_tasks[async_index].c);
    prom_vk_destroy_buffer(rt->vulkan.device, &rt->async_tasks[async_index].b);
    prom_vk_destroy_buffer(rt->vulkan.device, &rt->async_tasks[async_index].a);
  }
  destroy_all_execution_buffers(rt);
  /* Pipeline handles are mutable instances; descriptors remain immutable. */
  {
    VkPipeline* pipeline_fields[PROM_COMPUTE_PIPELINE_COUNT] = {
        &rt->pipeline, &rt->tiled_pipeline, &rt->memory_conservative_pipeline,
        &rt->sdsl_scalar_plus_pipeline, &rt->sdsl_tile16x16_shared_fp32_pipeline,
        &rt->sdsl_reg2x2_tile16x16_fp32_pipeline, &rt->sdsl_reg2x2_tile16x16_exacttail_fp32_pipeline,
        &rt->sdsl_reg2x2_tile16x16_flowboard_fp32_pipeline, &rt->sdsl_reg2x2_tile16x16_derive_fp32_pipeline,
        &rt->srt_2accum_k_pipeline, &rt->b2x2_row_major_biased_pipeline, &rt->a2x4_row_biased_accum8_pipeline,
        &rt->packed4_pipeline, &rt->fp16_pipeline,
    };
    for (uint32_t i = 0u; i < PROM_COMPUTE_PIPELINE_COUNT; ++i) {
      if (*pipeline_fields[i] != VK_NULL_HANDLE) {
        vkDestroyPipeline(rt->vulkan.device, *pipeline_fields[i], NULL);
        *pipeline_fields[i] = VK_NULL_HANDLE;
      }
    }
    for (uint32_t i = 0u; i < sizeof(rt->compute_pipeline_instances) / sizeof(rt->compute_pipeline_instances[0]); ++i) {
      rt->compute_pipeline_instances[i].pipeline = VK_NULL_HANDLE;
      rt->compute_pipeline_instances[i].status = PROM_PIPELINE_NOT_INITIALIZED;
    }
  }
  if (rt->memory_conservative_shader_module != VK_NULL_HANDLE) {
    vkDestroyShaderModule(rt->vulkan.device, rt->memory_conservative_shader_module, NULL);
    rt->memory_conservative_shader_module = VK_NULL_HANDLE;
  }
  if (rt->sdsl_scalar_plus_shader_module != VK_NULL_HANDLE) {
    vkDestroyShaderModule(rt->vulkan.device, rt->sdsl_scalar_plus_shader_module, NULL);
    rt->sdsl_scalar_plus_shader_module = VK_NULL_HANDLE;
  }
  if (rt->sdsl_reg2x2_tile16x16_fp32_shader_module != VK_NULL_HANDLE) {
    vkDestroyShaderModule(rt->vulkan.device, rt->sdsl_reg2x2_tile16x16_fp32_shader_module, NULL);
    rt->sdsl_reg2x2_tile16x16_fp32_shader_module = VK_NULL_HANDLE;
  }
  if (rt->sdsl_reg2x2_tile16x16_exacttail_fp32_shader_module != VK_NULL_HANDLE) {
    vkDestroyShaderModule(rt->vulkan.device, rt->sdsl_reg2x2_tile16x16_exacttail_fp32_shader_module, NULL);
    rt->sdsl_reg2x2_tile16x16_exacttail_fp32_shader_module = VK_NULL_HANDLE;
  }
  if (rt->sdsl_reg2x2_tile16x16_flowboard_fp32_shader_module != VK_NULL_HANDLE) {
    vkDestroyShaderModule(rt->vulkan.device, rt->sdsl_reg2x2_tile16x16_flowboard_fp32_shader_module, NULL);
    rt->sdsl_reg2x2_tile16x16_flowboard_fp32_shader_module = VK_NULL_HANDLE;
  }
  if (rt->sdsl_reg2x2_tile16x16_derive_fp32_shader_module != VK_NULL_HANDLE) {
    vkDestroyShaderModule(rt->vulkan.device, rt->sdsl_reg2x2_tile16x16_derive_fp32_shader_module, NULL);
    rt->sdsl_reg2x2_tile16x16_derive_fp32_shader_module = VK_NULL_HANDLE;
  }
  if (rt->sdsl_tile16x16_shared_fp32_shader_module != VK_NULL_HANDLE) {
    vkDestroyShaderModule(rt->vulkan.device, rt->sdsl_tile16x16_shared_fp32_shader_module, NULL);
    rt->sdsl_tile16x16_shared_fp32_shader_module = VK_NULL_HANDLE;
  }
  if (rt->pipeline_layout != VK_NULL_HANDLE) {
    vkDestroyPipelineLayout(rt->vulkan.device, rt->pipeline_layout, NULL);
    rt->pipeline_layout = VK_NULL_HANDLE;
  }
  if (rt->descriptor_set_layout != VK_NULL_HANDLE) {
    vkDestroyDescriptorSetLayout(rt->vulkan.device, rt->descriptor_set_layout, NULL);
    rt->descriptor_set_layout = VK_NULL_HANDLE;
  }
  if (rt->descriptor_pool != VK_NULL_HANDLE) {
    vkDestroyDescriptorPool(rt->vulkan.device, rt->descriptor_pool, NULL);
    rt->descriptor_pool = VK_NULL_HANDLE;
  }
  if (rt->sgemm_timestamp_query_pool != VK_NULL_HANDLE) {
    vkDestroyQueryPool(rt->vulkan.device, rt->sgemm_timestamp_query_pool, NULL);
    rt->sgemm_timestamp_query_pool = VK_NULL_HANDLE;
  }
  if (rt->submit_fence != VK_NULL_HANDLE) {
    vkDestroyFence(rt->vulkan.device, rt->submit_fence, NULL);
    rt->submit_fence = VK_NULL_HANDLE;
  }
  {
    uint32_t ring_index;
    for (ring_index = 0u; ring_index < PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH; ++ring_index) {
      if (rt->submission_ring[ring_index].fence != VK_NULL_HANDLE) {
        vkDestroyFence(rt->vulkan.device, rt->submission_ring[ring_index].fence, NULL);
        rt->submission_ring[ring_index].fence = VK_NULL_HANDLE;
      }
      rt->submission_ring[ring_index].command_buffer = VK_NULL_HANDLE;
      rt->submission_ring[ring_index].descriptor_set = VK_NULL_HANDLE;
    }
  }
  if (rt->transfer_submit_fence != VK_NULL_HANDLE) {
    vkDestroyFence(rt->vulkan.device, rt->transfer_submit_fence, NULL);
    rt->transfer_submit_fence = VK_NULL_HANDLE;
  }
  if (rt->transfer_ready_semaphore != VK_NULL_HANDLE) {
    vkDestroySemaphore(rt->vulkan.device, rt->transfer_ready_semaphore, NULL);
    rt->transfer_ready_semaphore = VK_NULL_HANDLE;
  }
  /* SGEMM objects above borrow the device and command pools.  The common
     owner is destroyed only after those operation resources are gone. */
  prom_vk_runtime_cleanup(&rt->vulkan);
}

static VkResult prom_runtime_create_package_module(prometheus_runtime* rt,
                                                   const char* variant_id,
                                                   VkShaderModule* out_module,
                                                   const char** out_entry_point) {
  prom_shader_package_diagnostic diagnostic;
  if (rt == NULL || rt->vulkan.shader_package == NULL || variant_id == NULL ||
      out_module == NULL || out_entry_point == NULL) return VK_ERROR_INITIALIZATION_FAILED;
  if (!prom_shader_package_create_module(rt->vulkan.shader_package, rt->vulkan.device, variant_id,
                                         out_module, out_entry_point, &diagnostic)) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }
  return VK_SUCCESS;
}

/* This deliberately remains separate from production package execution.  The
 * two public audit benchmarks accept caller-owned synthetic SPIR-V to compare
 * compiler artifacts; no production selector or registry entry can reach it. */
static VkResult prom_audit_create_arbitrary_spirv_module(
    VkDevice device, const prom_sgemm_audit_execution_descriptor* descriptor, VkShaderModule* out_module) {
  VkShaderModuleCreateInfo shader_info;
  if (descriptor == NULL || out_module == NULL) return VK_ERROR_INITIALIZATION_FAILED;
  memset(&shader_info, 0, sizeof(shader_info));
  shader_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  shader_info.codeSize = descriptor->spirv_size_bytes;
  shader_info.pCode = descriptor->spirv_words;
  return vkCreateShaderModule(device, &shader_info, NULL, out_module);
}

static VkResult vk_runtime_init(prometheus_runtime* rt) {
  VkResult result;
  VkDescriptorSetLayoutBinding bindings[3];
  VkDescriptorSetLayoutCreateInfo set_layout_info;
  VkPushConstantRange push_range;
  VkPipelineLayoutCreateInfo pipeline_layout_info;
  VkShaderModule shader_module = VK_NULL_HANDLE;
  const char* package_entry_point = NULL;
  VkPipelineShaderStageCreateInfo stage_info;
  VkComputePipelineCreateInfo pipeline_info;
  VkDescriptorPoolSize pool_size;
  VkDescriptorPoolCreateInfo descriptor_pool_info;
  VkDescriptorSetAllocateInfo set_alloc_info;
  VkCommandBufferAllocateInfo cmd_alloc_info;
  VkFenceCreateInfo fence_info;
  VkQueryPoolCreateInfo query_pool_info;

  if (rt == NULL) return VK_ERROR_INITIALIZATION_FAILED;
  result = prom_vk_runtime_init(&rt->vulkan, rt->vulkan.test_flags);
  if (result != VK_SUCCESS) return result;
  rt->timestamp_query_supported = 0u;
  if (rt->vulkan.timestamp_period_ns > 0.0f && rt->vulkan.timestamp_valid_bits > 0u) {
    memset(&query_pool_info, 0, sizeof(query_pool_info));
    query_pool_info.sType = VK_STRUCTURE_TYPE_QUERY_POOL_CREATE_INFO;
    query_pool_info.queryType = VK_QUERY_TYPE_TIMESTAMP;
    /* Legacy async/synchronous timing owns 0/1; M29 slots own 2..9. */
    query_pool_info.queryCount = 2u + (2u * PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH);
    result = vkCreateQueryPool(rt->vulkan.device, &query_pool_info, NULL, &rt->sgemm_timestamp_query_pool);
    if (result == VK_SUCCESS) {
      rt->timestamp_query_supported = 1u;
    } else {
      rt->sgemm_timestamp_query_pool = VK_NULL_HANDLE;
    }
  }
  if (rt->timestamp_query_supported != 0u) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_NONE);
  } else if (rt->vulkan.timestamp_period_ns > 0.0f && rt->vulkan.timestamp_valid_bits > 0u) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_POOL_UNAVAILABLE);
  } else if (rt->vulkan.timestamp_period_ns <= 0.0f) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_PERIOD);
  } else {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_UNSUPPORTED);
  }
  result = prom_vk_runtime_enable_validation(&rt->vulkan);
  if (result != VK_SUCCESS) return result;
  memset(bindings, 0, sizeof(bindings));
  bindings[0].binding = 0u;
  bindings[0].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  bindings[0].descriptorCount = 1u;
  bindings[0].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  bindings[1].binding = 1u;
  bindings[1].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  bindings[1].descriptorCount = 1u;
  bindings[1].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  bindings[2].binding = 2u;
  bindings[2].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  bindings[2].descriptorCount = 1u;
  bindings[2].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;

  memset(&set_layout_info, 0, sizeof(set_layout_info));
  set_layout_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO;
  set_layout_info.bindingCount = 3u;
  set_layout_info.pBindings = bindings;
  result = vkCreateDescriptorSetLayout(rt->vulkan.device, &set_layout_info, NULL, &rt->descriptor_set_layout);
  if (result != VK_SUCCESS) {
    return result;
  }

  pool_size.type = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  pool_size.descriptorCount = 3u * (1u + PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH);
  memset(&descriptor_pool_info, 0, sizeof(descriptor_pool_info));
  descriptor_pool_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO;
  descriptor_pool_info.poolSizeCount = 1u;
  descriptor_pool_info.pPoolSizes = &pool_size;
  descriptor_pool_info.maxSets = 1u + PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH;
  result = vkCreateDescriptorPool(rt->vulkan.device, &descriptor_pool_info, NULL, &rt->descriptor_pool);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&set_alloc_info, 0, sizeof(set_alloc_info));
  set_alloc_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO;
  set_alloc_info.descriptorPool = rt->descriptor_pool;
  set_alloc_info.descriptorSetCount = 1u;
  set_alloc_info.pSetLayouts = &rt->descriptor_set_layout;
  result = vkAllocateDescriptorSets(rt->vulkan.device, &set_alloc_info, &rt->descriptor_set);
  if (result != VK_SUCCESS) {
    return result;
  }
  {
    VkDescriptorSetLayout ring_layouts[PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH];
    VkDescriptorSet ring_sets[PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH];
    uint32_t ring_index;
    for (ring_index = 0u; ring_index < PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH; ++ring_index) {
      ring_layouts[ring_index] = rt->descriptor_set_layout;
    }
    set_alloc_info.descriptorSetCount = PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH;
    set_alloc_info.pSetLayouts = ring_layouts;
    result = vkAllocateDescriptorSets(rt->vulkan.device, &set_alloc_info, ring_sets);
    if (result != VK_SUCCESS) {
      return result;
    }
    for (ring_index = 0u; ring_index < PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH; ++ring_index) {
      rt->submission_ring[ring_index].slot_id = ring_index;
      rt->submission_ring[ring_index].state = PROM_SGEMM_SUBMISSION_SLOT_EMPTY;
      rt->submission_ring[ring_index].descriptor_set = ring_sets[ring_index];
      rt->submission_ring[ring_index].query_base = 2u + ring_index * 2u;
    }
  }

  memset(&push_range, 0, sizeof(push_range));
  push_range.stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  push_range.offset = 0u;
  push_range.size = PROM_VK_SHADER_PUSH_BYTES;

  memset(&pipeline_layout_info, 0, sizeof(pipeline_layout_info));
  pipeline_layout_info.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO;
  pipeline_layout_info.setLayoutCount = 1u;
  pipeline_layout_info.pSetLayouts = &rt->descriptor_set_layout;
  pipeline_layout_info.pushConstantRangeCount = 1u;
  pipeline_layout_info.pPushConstantRanges = &push_range;
  result = vkCreatePipelineLayout(rt->vulkan.device, &pipeline_layout_info, NULL, &rt->pipeline_layout);
  if (result != VK_SUCCESS) {
    return result;
  }

  if ((rt->vulkan.test_flags & PROM_TESTCFG_FAIL_PIPELINE_CREATE) != 0u) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }

  result = prom_runtime_create_package_module(rt, "kernel-3-default",
                                              &rt->memory_conservative_shader_module,
                                              &package_entry_point);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = rt->memory_conservative_shader_module;
  stage_info.pName = package_entry_point;

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->memory_conservative_pipeline);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = prom_runtime_create_package_module(rt, "kernel-4-default",
                                              &rt->sdsl_scalar_plus_shader_module,
                                              &package_entry_point);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = rt->sdsl_scalar_plus_shader_module;
  stage_info.pName = package_entry_point;

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->sdsl_scalar_plus_pipeline);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = prom_runtime_create_package_module(rt, "kernel-5-default",
                                              &rt->sdsl_tile16x16_shared_fp32_shader_module,
                                              &package_entry_point);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = rt->sdsl_tile16x16_shared_fp32_shader_module;
  stage_info.pName = package_entry_point;

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->sdsl_tile16x16_shared_fp32_pipeline);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = prom_runtime_create_package_module(rt, "kernel-6-default",
                                              &rt->sdsl_reg2x2_tile16x16_fp32_shader_module,
                                              &package_entry_point);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = rt->sdsl_reg2x2_tile16x16_fp32_shader_module;
  stage_info.pName = package_entry_point;

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->sdsl_reg2x2_tile16x16_fp32_pipeline);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = prom_runtime_create_package_module(rt, "kernel-7-default",
                                              &rt->sdsl_reg2x2_tile16x16_exacttail_fp32_shader_module,
                                              &package_entry_point);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = rt->sdsl_reg2x2_tile16x16_exacttail_fp32_shader_module;
  stage_info.pName = package_entry_point;

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device,
                                    VK_NULL_HANDLE,
                                    1u,
                                    &pipeline_info,
                                    NULL,
                                    &rt->sdsl_reg2x2_tile16x16_exacttail_fp32_pipeline);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = prom_runtime_create_package_module(rt, "kernel-8-default",
                                              &rt->sdsl_reg2x2_tile16x16_flowboard_fp32_shader_module,
                                              &package_entry_point);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = rt->sdsl_reg2x2_tile16x16_flowboard_fp32_shader_module;
  stage_info.pName = package_entry_point;

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device,
                                    VK_NULL_HANDLE,
                                    1u,
                                    &pipeline_info,
                                    NULL,
                                    &rt->sdsl_reg2x2_tile16x16_flowboard_fp32_pipeline);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = prom_runtime_create_package_module(rt, "kernel-9-default",
                                              &rt->sdsl_reg2x2_tile16x16_derive_fp32_shader_module,
                                              &package_entry_point);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = rt->sdsl_reg2x2_tile16x16_derive_fp32_shader_module;
  stage_info.pName = package_entry_point;

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device,
                                    VK_NULL_HANDLE,
                                    1u,
                                    &pipeline_info,
                                    NULL,
                                    &rt->sdsl_reg2x2_tile16x16_derive_fp32_pipeline);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = prom_runtime_create_package_module(rt, "kernel-10-default", &shader_module,
                                              &package_entry_point);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = package_entry_point;

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->srt_2accum_k_pipeline);
  vkDestroyShaderModule(rt->vulkan.device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = prom_runtime_create_package_module(rt, "kernel-11-default", &shader_module,
                                              &package_entry_point);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = package_entry_point;
  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL,
                                    &rt->b2x2_row_major_biased_pipeline);
  vkDestroyShaderModule(rt->vulkan.device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = prom_runtime_create_package_module(rt, "kernel-12-default", &shader_module,
                                              &package_entry_point);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = package_entry_point;
  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL,
                                    &rt->a2x4_row_biased_accum8_pipeline);
  vkDestroyShaderModule(rt->vulkan.device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = prom_runtime_create_package_module(rt, "kernel-1-default", &shader_module,
                                              &package_entry_point);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = package_entry_point;

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->pipeline);
  vkDestroyShaderModule(rt->vulkan.device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = prom_runtime_create_package_module(rt, "kernel-2-default", &shader_module,
                                              &package_entry_point);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = package_entry_point;

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->tiled_pipeline);
  vkDestroyShaderModule(rt->vulkan.device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = prom_runtime_create_package_module(rt, "kernel-13-default", &shader_module,
                                              &package_entry_point);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = package_entry_point;

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->packed4_pipeline);
  vkDestroyShaderModule(rt->vulkan.device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = prom_runtime_create_package_module(rt, "kernel-14-default", &shader_module,
                                              &package_entry_point);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = package_entry_point;

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->fp16_pipeline);
  vkDestroyShaderModule(rt->vulkan.device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }
  {
    const VkPipeline pipelines[PROM_COMPUTE_PIPELINE_COUNT] = {
        rt->pipeline, rt->tiled_pipeline, rt->memory_conservative_pipeline,
        rt->sdsl_scalar_plus_pipeline, rt->sdsl_tile16x16_shared_fp32_pipeline,
        rt->sdsl_reg2x2_tile16x16_fp32_pipeline, rt->sdsl_reg2x2_tile16x16_exacttail_fp32_pipeline,
        rt->sdsl_reg2x2_tile16x16_flowboard_fp32_pipeline, rt->sdsl_reg2x2_tile16x16_derive_fp32_pipeline,
        rt->srt_2accum_k_pipeline, rt->b2x2_row_major_biased_pipeline, rt->a2x4_row_biased_accum8_pipeline,
        rt->packed4_pipeline, rt->fp16_pipeline,
    };
    if (prom_shader_registry_validate() == 0u) {
      return VK_ERROR_INITIALIZATION_FAILED;
    }
    prom_shader_registry_initialize_pipeline_instances(rt->compute_pipeline_instances,
                                                        sizeof(rt->compute_pipeline_instances) / sizeof(rt->compute_pipeline_instances[0]),
                                                        pipelines,
                                                        PROM_COMPUTE_PIPELINE_COUNT);
  }

  memset(&cmd_alloc_info, 0, sizeof(cmd_alloc_info));
  cmd_alloc_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO;
  cmd_alloc_info.commandPool = rt->vulkan.command_pool;
  cmd_alloc_info.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;
  cmd_alloc_info.commandBufferCount = 1u;
  result = vkAllocateCommandBuffers(rt->vulkan.device, &cmd_alloc_info, &rt->command_buffer);
  if (result != VK_SUCCESS) {
    return result;
  }
  cmd_alloc_info.commandBufferCount = PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH;
  {
    VkCommandBuffer ring_command_buffers[PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH];
    uint32_t ring_index;
    result = vkAllocateCommandBuffers(rt->vulkan.device, &cmd_alloc_info, ring_command_buffers);
    if (result != VK_SUCCESS) {
      return result;
    }
    for (ring_index = 0u; ring_index < PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH; ++ring_index) {
      rt->submission_ring[ring_index].command_buffer = ring_command_buffers[ring_index];
    }
  }
  cmd_alloc_info.commandBufferCount = 1u;
  if (rt->vulkan.transfer_queue_enabled != 0u) {
    cmd_alloc_info.commandPool = rt->vulkan.transfer_command_pool;
    result = vkAllocateCommandBuffers(rt->vulkan.device, &cmd_alloc_info, &rt->transfer_command_buffer);
    if (result != VK_SUCCESS) {
      return result;
    }
  }

  memset(&fence_info, 0, sizeof(fence_info));
  fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
  result = vkCreateFence(rt->vulkan.device, &fence_info, NULL, &rt->submit_fence);
  if (result != VK_SUCCESS) {
    return result;
  }
  {
    uint32_t ring_index;
    for (ring_index = 0u; ring_index < PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH; ++ring_index) {
      result = vkCreateFence(rt->vulkan.device, &fence_info, NULL, &rt->submission_ring[ring_index].fence);
      if (result != VK_SUCCESS) {
        return result;
      }
    }
  }
  if (rt->vulkan.transfer_queue_enabled != 0u) {
    VkSemaphoreCreateInfo semaphore_info;
    result = vkCreateFence(rt->vulkan.device, &fence_info, NULL, &rt->transfer_submit_fence);
    if (result != VK_SUCCESS) {
      return result;
    }
    memset(&semaphore_info, 0, sizeof(semaphore_info));
    semaphore_info.sType = VK_STRUCTURE_TYPE_SEMAPHORE_CREATE_INFO;
    result = vkCreateSemaphore(rt->vulkan.device, &semaphore_info, NULL, &rt->transfer_ready_semaphore);
    if (result != VK_SUCCESS) {
      return result;
    }
  }
  return VK_SUCCESS;
}

// ============================================================================

static int prom_runtime_copy_shader_package_root(prometheus_runtime* runtime,
                                                 const PrometheusReactorConfig* config) {
  size_t length;
  if (runtime == NULL || config == NULL ||
      config->struct_size < offsetof(PrometheusReactorConfig, shader_package_root) +
                                sizeof(config->shader_package_root) ||
      config->shader_package_root == NULL || config->shader_package_root[0] == '\0') {
    return 1;
  }
  length = strlen(config->shader_package_root);
  runtime->vulkan.shader_package_root = (char*)malloc(length + 1u);
  if (runtime->vulkan.shader_package_root == NULL) return 0;
  memcpy(runtime->vulkan.shader_package_root, config->shader_package_root, length + 1u);
  return 1;
}

static int prom_runtime_discover_adjacent_shader_package(prometheus_runtime* runtime) {
#if defined(_WIN32)
  char module_path[MAX_PATH];
  char* separator;
  size_t length;
  if (runtime == NULL || runtime->vulkan.shader_package_root != NULL) return runtime != NULL;
  if (GetModuleFileNameA(NULL, module_path, (DWORD)sizeof(module_path)) == 0u ||
      GetLastError() == ERROR_INSUFFICIENT_BUFFER) return 0;
  separator = strrchr(module_path, '\\');
  if (separator == NULL) return 0;
  *separator = '\0';
  length = strlen(module_path);
  if (length > SIZE_MAX - sizeof("\\shaders")) return 0;
  runtime->vulkan.shader_package_root = (char*)malloc(length + sizeof("\\shaders"));
  if (runtime->vulkan.shader_package_root == NULL) return 0;
  memcpy(runtime->vulkan.shader_package_root, module_path, length);
  memcpy(runtime->vulkan.shader_package_root + length, "\\shaders", sizeof("\\shaders"));
  return 1;
#else
  (void)runtime;
  return 0;
#endif
}

int prom_reactor_runtime_create_impl(void* config, void** out_handle) {
  VkResult result;
  prometheus_runtime* runtime;
  uint32_t legacy_test_without_package = 0u;
  (void)config;

  if (out_handle == NULL) {
    return PROM_ERROR;
  }

  *out_handle = NULL;
  runtime = (prometheus_runtime*)malloc(sizeof(prometheus_runtime));
  if (runtime == NULL) {
    return PROM_INTERNAL_ERROR;
  }
  memset(runtime, 0, sizeof(*runtime));
  runtime->magic = PROMETHEUS_RUNTIME_MAGIC;
  runtime->vulkan.reason_code = PROM_REASON_VULKAN_UNAVAILABLE;
  prom_dom_blackboard_init(&runtime->blackboard);
  prom_dominatus_measurement_filter_init(&runtime->p14_measurement_filter_state, NULL);
  memset(&runtime->p14_last_filtered_evidence, 0, sizeof(runtime->p14_last_filtered_evidence));
  runtime->p14_measurement_tick = 0u;
  prom_dominatus_predictor_init(&runtime->p15_predictor_state, NULL);
  prom_dominatus_shadow_calibration_init(&runtime->p15_shadow_calibration);
  prom_dominatus_shadow_would_act_init(&runtime->p15_shadow_would_act_state);
  runtime->p15_shadow_canary_params = prom_dominatus_shadow_canary_default_params();
  prom_dominatus_shadow_canary_init(&runtime->p15_shadow_canary_state);
  runtime->p15_prestage_params = prom_dominatus_prestage_default_params();
  prom_sgemm_controller_init(&runtime->sgemm_controller);
  prom_slot_hfsm_init(&runtime->slots[0], 0u);
  prom_slot_hfsm_init(&runtime->slots[1], 1u);
  runtime->slot_diag.current_slot_id = UINT32_MAX;
  runtime->slot_diag.next_slot_id = 0u;
  runtime->slot_diag.failure_slot_id = -1;
  runtime->slot_diag.async_slot_id = -1;
  runtime->slot_diag.transfer_queue_family_index = UINT32_MAX;
  invalidate_selector_caches(runtime);
  runtime->slot_diag.compute_queue_family_index = UINT32_MAX;
  runtime->slot_diag.transfer_fallback_reason = PROM_TRANSFER_FALLBACK_REQUIRED;
  runtime->slot_diag.transfer_failure_slot_id = -1;
  runtime->slot_diag.transfer_failure_reason = 0;
  runtime->async_task_id = 0;
  runtime->async_state = PROM_ASYNC_STATE_IDLE;
  runtime->async_stage = PROM_STAGE_NONE;
  runtime->async_failure_detail = 0;
  runtime->async_next_feedback_sequence = 1u;
  runtime->async_next_submission_sequence = 1u;
  runtime->submission_ring_diag.configured_depth = PROM_SGEMM_SUBMISSION_RING_DEFAULT_DEPTH;
  runtime->reduction_ring_depth = 2u;
  commit_slot_runtime_diag_snapshot(runtime, 0);
  stage_commit_async_snapshot(runtime, PROM_DOM_EVENT_NONE, 0);

  if (config != NULL) {
    const PrometheusReactorConfig* cfg = (const PrometheusReactorConfig*)config;
    if (!prom_runtime_copy_shader_package_root(runtime, cfg)) {
      free(runtime);
      return PROM_INTERNAL_ERROR;
    }
    if (cfg->struct_size >= offsetof(PrometheusReactorConfig, batch_ring_depth) + sizeof(cfg->batch_ring_depth) &&
        cfg->batch_ring_depth >= 1u && cfg->batch_ring_depth <= PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH) {
      runtime->submission_ring_diag.configured_depth = cfg->batch_ring_depth;
    }
    if (cfg->struct_size >= offsetof(PrometheusReactorConfig, test_flags) + sizeof(cfg->test_flags)) {
      runtime->vulkan.test_flags = cfg->test_flags;
    }
    if (cfg->struct_size >= offsetof(PrometheusReactorConfig, async_test_flags) + sizeof(cfg->async_test_flags)) {
      runtime->async_test_flags = cfg->async_test_flags;
    }
    if (cfg->struct_size >= offsetof(PrometheusReactorConfig, p15_shadow_canary_enabled) + sizeof(cfg->p15_shadow_canary_enabled)) {
      runtime->p15_shadow_canary_params.enabled = cfg->p15_shadow_canary_enabled != 0u ? 1u : 0u;
      runtime->p15_shadow_canary_state.enabled = runtime->p15_shadow_canary_params.enabled;
      runtime->p15_shadow_authority_gate.authority_enabled = runtime->p15_shadow_canary_params.enabled;
    }
    if (cfg->struct_size >= offsetof(PrometheusReactorConfig, reduction_test_flags) + sizeof(cfg->reduction_test_flags)) {
      runtime->reduction_test_flags = cfg->reduction_test_flags;
    }
    if (cfg->struct_size >= offsetof(PrometheusReactorConfig, reduction_ring_depth) + sizeof(cfg->reduction_ring_depth) &&
        cfg->reduction_ring_depth >= 1u && cfg->reduction_ring_depth <= 4u) {
      runtime->reduction_ring_depth = cfg->reduction_ring_depth;
    }
    legacy_test_without_package =
        cfg->struct_size < offsetof(PrometheusReactorConfig, shader_package_root) + sizeof(cfg->shader_package_root) &&
        (runtime->vulkan.test_flags & PROM_TESTCFG_SKIP_VULKAN_INIT) != 0u;
  }
  /* Older header-sized test configurations predate package selection. Their
     explicit no-Vulkan probe skips discovery without relaxing executable
     admission, which still requires a resolved package. */
  if (runtime->vulkan.shader_package_root == NULL && legacy_test_without_package == 0u &&
      !prom_runtime_discover_adjacent_shader_package(runtime)) {
    free(runtime);
    return PROM_ERROR;
  }
  if (runtime->vulkan.shader_package_root != NULL) {
    prom_shader_package_diagnostic package_diagnostic;
    if (!prom_shader_package_open(runtime->vulkan.shader_package_root, &runtime->vulkan.shader_package, &package_diagnostic)) {
      free(runtime->vulkan.shader_package_root);
      free(runtime);
      return PROM_ERROR;
    }
  }
  runtime->arena_budget_limit_bytes = PROM_ARENA_DEFAULT_BUDGET_BYTES;
  runtime->arena_floor_bytes = PROM_ARENA_DEFAULT_SHRINK_FLOOR_BYTES;
  if (runtime->vulkan.test_flags != 0u) {
    runtime->arena_floor_bytes = 1ull * 1024ull * 1024ull;
    runtime->arena_budget_limit_bytes = 32ull * 1024ull * 1024ull;
  }
  runtime->slot_diag.p11_m3_budget_limit_bytes = runtime->arena_budget_limit_bytes;
  runtime->arena_shrink_low_usage_threshold_epochs = PROM_ARENA_SHRINK_LOW_USAGE_EPOCHS;
  runtime->arena_shrink_cooldown_epochs = PROM_ARENA_SHRINK_COOLDOWN_EPOCHS;
  runtime->arena_last_failure_detail = 0;
  runtime->arenas[PROM_ARENA_ROLE_A].role = PROM_ARENA_ROLE_A;
  runtime->arenas[PROM_ARENA_ROLE_B].role = PROM_ARENA_ROLE_B;
  runtime->arenas[PROM_ARENA_ROLE_C].role = PROM_ARENA_ROLE_C;
  runtime->arenas[PROM_ARENA_ROLE_UPLOAD].role = PROM_ARENA_ROLE_UPLOAD;
  runtime->arenas[PROM_ARENA_ROLE_A].memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  runtime->arenas[PROM_ARENA_ROLE_B].memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  runtime->arenas[PROM_ARENA_ROLE_C].memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  runtime->arenas[PROM_ARENA_ROLE_UPLOAD].memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  runtime->arenas[PROM_ARENA_ROLE_A].owner_slot_id = -1;
  runtime->arenas[PROM_ARENA_ROLE_B].owner_slot_id = -1;
  runtime->arenas[PROM_ARENA_ROLE_C].owner_slot_id = -1;
  runtime->arenas[PROM_ARENA_ROLE_UPLOAD].owner_slot_id = -1;
  runtime->slot_diag.p11_m3_budget_limit_bytes = runtime->arena_budget_limit_bytes;

  if ((runtime->vulkan.test_flags & PROM_TESTCFG_SKIP_VULKAN_INIT) != 0u) {
    runtime->vulkan.available = 0u;
    runtime->vulkan.reason_code = PROM_REASON_VULKAN_UNAVAILABLE;
    runtime->vulkan.init_detail_code = (int)VK_ERROR_INITIALIZATION_FAILED;
  } else {
    result = vk_runtime_init(runtime);
    if (result == VK_SUCCESS) {
      prom_dom_transfer_queue_facts transfer_facts;
      prom_dom_transfer_queue_decision transfer_decision;
      runtime->vulkan.available = 1u;
      runtime->vulkan.reason_code = PROM_REASON_NONE;
      runtime->vulkan.init_detail_code = 0;
      memset(&transfer_facts, 0, sizeof(transfer_facts));
      transfer_facts.dedicated_transfer_available = runtime->vulkan.dedicated_transfer_available;
      transfer_facts.transfer_queue_family_index = runtime->vulkan.transfer_queue_family_index;
      transfer_facts.compute_queue_family_index = runtime->vulkan.queue_family_index;
      transfer_facts.queue_families_differ =
          (runtime->vulkan.dedicated_transfer_available != 0u && runtime->vulkan.transfer_queue_family_index != runtime->vulkan.queue_family_index) ? 1u : 0u;
      transfer_facts.transfer_queue_supported = runtime->vulkan.transfer_queue_enabled;
      transfer_facts.transfer_queue_disabled_by_config = ((runtime->vulkan.test_flags & PROM_TESTCFG_DISABLE_TRANSFER_QUEUE) != 0u) ? 1u : 0u;
      transfer_facts.transfer_workload_large_enough = 1u;
      transfer_facts.transfer_sync_ownership_supported = runtime->vulkan.transfer_queue_enabled;
      transfer_facts.transfer_fallback_available = 1u;
      transfer_facts.upload_only_policy_eligible = 1u;
      transfer_facts.upload_readback_supported = 0u;
      if (prom_dom_sgemm_stage_transfer_queue_facts(&runtime->blackboard, &transfer_facts) != 0u) {
        prom_dom_sgemm_commit(&runtime->blackboard);
      }
      memset(&transfer_decision, 0, sizeof(transfer_decision));
      transfer_decision.transfer_policy_selected = runtime->vulkan.transfer_queue_enabled;
      transfer_decision.selected_transfer_policy = runtime->vulkan.transfer_queue_enabled != 0u ? 1u : 0u;
      transfer_decision.transfer_queue_used = runtime->vulkan.transfer_queue_enabled;
      transfer_decision.transfer_fallback_reason =
          runtime->vulkan.transfer_queue_enabled != 0u ? PROM_TRANSFER_FALLBACK_NONE
                                                : (((runtime->vulkan.test_flags & PROM_TESTCFG_DISABLE_TRANSFER_QUEUE) != 0u)
                                                       ? PROM_TRANSFER_FALLBACK_DISABLED_BY_CONFIG
                                                       : PROM_TRANSFER_FALLBACK_NO_DEDICATED_QUEUE);
      if (prom_dom_sgemm_stage_transfer_queue_decision(&runtime->blackboard, &transfer_decision) != 0u) {
        prom_dom_sgemm_commit(&runtime->blackboard);
      }
      if (prom_dom_sgemm_stage_transfer_handoff(&runtime->blackboard, 0u, 0u, 0) != 0u &&
          prom_dom_sgemm_stage_transfer_wait(&runtime->blackboard, 0u, 0u, 0) != 0u &&
          prom_dom_sgemm_stage_transfer_failure(&runtime->blackboard, -1, 0, 0u) != 0u &&
          prom_dom_sgemm_stage_transfer_complete(&runtime->blackboard, 1u, 0u, 0u, 0) != 0u) {
        prom_dom_sgemm_commit(&runtime->blackboard);
      }
      sync_transfer_diag_from_visible(runtime);
    } else {
      runtime->vulkan.available = 0u;
      runtime->vulkan.reason_code = PROM_REASON_VULKAN_UNAVAILABLE;
      runtime->vulkan.init_detail_code = (int)result;
      vk_runtime_cleanup(runtime);
    }
  }

  if (!registry_add(runtime)) {
    vk_runtime_cleanup(runtime);
    prom_vk_runtime_destroy_package(&runtime->vulkan);
    free(runtime);
    return PROM_INTERNAL_ERROR;
  }

  *out_handle = runtime;
  return PROM_OK;
}

int prom_reactor_runtime_destroy_impl(void* handle) {
  prometheus_runtime* runtime;
  if (handle == NULL) {
    return PROM_OK;
  }
  if (!registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }

  runtime = (prometheus_runtime*)handle;
  if (runtime->magic != PROMETHEUS_RUNTIME_MAGIC) {
    return PROM_INVALID_HANDLE;
  }

  registry_remove(handle);
  /* Persistent ray-query scenes borrow this sole device owner.  Retire them
     before vk_runtime_cleanup tears down their AS backing storage. */
  prom_ray_query_scene_runtime_destroy_all(handle);
  prom_fft_diag_forget_handle(handle);
  vk_runtime_cleanup(runtime);
  prom_vk_runtime_destroy_package(&runtime->vulkan);
  free(runtime);
  return PROM_OK;
}

int prom_reactor_runtime_probe_impl(void* handle, PrometheusCaps* out_caps) {
  prometheus_runtime* runtime;
  if (out_caps == NULL) {
    return PROM_ERROR;
  }
  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }

  runtime = (prometheus_runtime*)handle;
  out_caps->available = runtime->vulkan.available;
  if (runtime->vulkan.available == 0u) {
    out_caps->backend_type = PROM_BACKEND_UNKNOWN;
  } else if (runtime->vulkan.software_vulkan != 0u) {
    out_caps->backend_type = PROM_BACKEND_VULKAN_SOFTWARE;
  } else {
    out_caps->backend_type = PROM_BACKEND_VULKAN;
  }
  out_caps->reason_code = runtime->vulkan.reason_code;
  return PROM_OK;
}

// ============================================================================
// SGEMM Single-Call Execution
// ============================================================================

static uint32_t prom_occ_variant_registered(uint32_t variant) {
  return prom_shader_registry_find_compute_implementation(variant) != NULL ? 1u : 0u;
}

int prom_reactor_runtime_vulkan_device_diagnostics_impl(void* handle, PrometheusVulkanDeviceDiagnostics* out_diag) {
  prometheus_runtime* rt;
  VkPhysicalDeviceProperties props;
  if (out_diag == NULL) return PROM_ERROR;
  memset(out_diag, 0, sizeof(*out_diag));
  if (handle == NULL || !registry_contains(handle)) return PROM_INVALID_HANDLE;
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC || rt->vulkan.physical_device == VK_NULL_HANDLE) return PROM_INVALID_HANDLE;
  vkGetPhysicalDeviceProperties(rt->vulkan.physical_device, &props);
  memcpy(out_diag->device_name, props.deviceName, sizeof(out_diag->device_name) - 1u);
  out_diag->vendor_id = props.vendorID;
  out_diag->device_id = props.deviceID;
  out_diag->device_type = (uint32_t)props.deviceType;
  out_diag->driver_version = props.driverVersion;
  out_diag->api_version = props.apiVersion;
  out_diag->software_vulkan = rt->vulkan.software_vulkan;
  out_diag->compute_queue_family = rt->vulkan.queue_family_index;
  out_diag->transfer_queue_family = rt->vulkan.transfer_queue_family_index;
  return PROM_OK;
}

void* prom_reactor_runtime_reduction_state(void* handle) {
  if (!prom_reactor_runtime_validate_handle(handle)) return NULL;
  return ((prometheus_runtime*)handle)->reduction_state;
}

int prom_reactor_runtime_set_reduction_state(void* handle, void* state) {
  prometheus_runtime* rt;
  if (!prom_reactor_runtime_validate_handle(handle)) return PROM_INVALID_HANDLE;
  rt = (prometheus_runtime*)handle;
  if (rt->reduction_state != NULL && state != NULL && rt->reduction_state != state) return PROM_ERROR;
  rt->reduction_state = state;
  return PROM_OK;
}


static uint32_t prom_occ_variant_is_wired_evt_dispatchable(uint32_t variant) {
  return prom_shader_registry_is_dispatchable(variant);
}

static uint32_t prom_occ_variant_path_status(uint32_t variant) {
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) {
    return PROM_OCCUPANCY_VARIANT_PATH_STATUS_BASELINE;
  }
  if (prom_occ_variant_is_wired_evt_dispatchable(variant) != 0u) {
    return PROM_OCCUPANCY_VARIANT_PATH_STATUS_WIRED;
  }
  return PROM_OCCUPANCY_VARIANT_PATH_STATUS_NOT_WIRED;
}

static uint32_t prom_occ_variant_executed_identity(uint32_t variant) {
  if (prom_occ_variant_is_wired_evt_dispatchable(variant) != 0u) {
    return variant;
  }
  return PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR;
}

static uint32_t prom_occ_variant_executed_identity_for_dispatch(uint32_t variant, uint32_t compute_mode) {
  if (compute_mode == (uint32_t)PROM_VK_COMPUTE_TILED) {
    return prom_occ_variant_executed_identity(variant);
  }
  return PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR;
}

static uint32_t prom_occ_variant_path_id(uint32_t variant) {
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_SCALAR_PLUS) {
    return PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_SCALAR_PLUS;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_TILE16X16_SHARED_FP32) {
    return PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_TILE16X16_SHARED_FP32;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FP32) {
    return PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_REG2X2_TILE16X16_FP32;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32) {
    return PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32) {
    return PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_DERIVE_FP32) {
    return PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_REG2X2_TILE16X16_DERIVE_FP32;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE) {
    return PROM_OCCUPANCY_VARIANT_PATH_ID_MEMORY_CONSERVATIVE;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE) {
    return PROM_OCCUPANCY_VARIANT_PATH_ID_SRT_2ACCUM_K;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4) {
    return PROM_OCCUPANCY_VARIANT_PATH_ID_B2X2_ROW_MAJOR_BIASED;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) {
    return PROM_OCCUPANCY_VARIANT_PATH_ID_A2X4_ROW_BIASED_ACCUM8;
  }
  return PROM_OCCUPANCY_VARIANT_PATH_ID_BASELINE;
}

static uint32_t prom_occ_variant_fallback_reason(uint32_t variant) {
  if (prom_occ_variant_is_wired_evt_dispatchable(variant) != 0u) {
    return PROM_OCCUPANCY_VARIANT_FALLBACK_NONE;
  }
  return PROM_OCCUPANCY_VARIANT_FALLBACK_PATH_NOT_WIRED;
}

static uint32_t prom_sgemm_effective_force_direct_reason(const prom_dom_sgemm_path_compute_snapshot* path_compute_snapshot) {
  if (path_compute_snapshot == NULL) return PROM_SGEMM_FORCE_DIRECT_REASON_NONE;
  if (path_compute_snapshot->facts.force_direct != 0u) {
    return path_compute_snapshot->facts.force_direct_reason;
  }
  if (path_compute_snapshot->decision.selected_path == (uint32_t)PROM_VK_PATH_DIRECT &&
      (path_compute_snapshot->decision.used_fallback_to_direct != 0u ||
       path_compute_snapshot->decision.requested_path != (uint32_t)PROM_VK_PATH_DIRECT)) {
    return PROM_SGEMM_FORCE_DIRECT_REASON_SAFE_CONCRETE_HAZARD;
  }
  return PROM_SGEMM_FORCE_DIRECT_REASON_NONE;
}

static const prom_dominatus_reservation_request* prom_p15_prefer_earlier_reservation(
    const prom_dominatus_reservation_request* current,
    const prom_dominatus_reservation_request* candidate) {
  if (candidate == NULL) return current;
  if (current == NULL || candidate->target_tick < current->target_tick) return candidate;
  return current;
}

static prom_p15_feedforward_reservation_probe prom_p15_probe_feedforward_reservation(
    const prom_dominatus_reservation_state_set* reservations,
    uint32_t dispatch_shape_class,
    uint32_t dispatch_variant_id) {
  uint32_t i;
  prom_p15_feedforward_reservation_probe probe;
  memset(&probe, 0, sizeof(probe));
  if (reservations == NULL) return probe;

  for (i = 0u; i < PROM_DOM_RESERVATION_CAP; ++i) {
    const prom_dominatus_reservation_request* entry = &reservations->entries[i];
    if (entry->valid == 0u) continue;
    probe.present = 1u;
    if (entry->state == PROM_DOM_RESERVATION_MATURED) probe.matured = 1u;
    if (entry->shape_class == dispatch_shape_class && entry->variant_id == dispatch_variant_id) {
      if (entry->state == PROM_DOM_RESERVATION_MATURED) {
        probe.exact_match = prom_p15_prefer_earlier_reservation(probe.exact_match, entry);
      } else if (entry->state == PROM_DOM_RESERVATION_EXPIRED) {
        probe.stale = prom_p15_prefer_earlier_reservation(probe.stale, entry);
      } else if (entry->state == PROM_DOM_RESERVATION_CANCELLED) {
        probe.cancelled = prom_p15_prefer_earlier_reservation(probe.cancelled, entry);
      } else if (entry->state == PROM_DOM_RESERVATION_YIELDED) {
        probe.consumed = prom_p15_prefer_earlier_reservation(probe.consumed, entry);
      } else if (entry->state == PROM_DOM_RESERVATION_REQUESTED || entry->state == PROM_DOM_RESERVATION_RESERVED) {
        probe.pending = prom_p15_prefer_earlier_reservation(probe.pending, entry);
      }
      continue;
    }
    if (entry->state != PROM_DOM_RESERVATION_MATURED) continue;
    if (entry->shape_class == dispatch_shape_class) {
      probe.variant_mismatch = prom_p15_prefer_earlier_reservation(probe.variant_mismatch, entry);
    } else {
      probe.shape_mismatch = prom_p15_prefer_earlier_reservation(probe.shape_mismatch, entry);
    }
  }
  return probe;
}

static void prom_sgemm_publish_final_dispatch_diagnostics(prometheus_runtime* rt,
                                                          uint32_t requested_variant,
                                                          uint32_t policy_mode,
                                                          const prom_dom_sgemm_path_compute_snapshot* path_compute_snapshot) {
  const uint32_t path_status = prom_occ_variant_path_status(requested_variant);
  const uint32_t evt_dispatchable = prom_occ_variant_is_wired_evt_dispatchable(requested_variant);
  const uint32_t selected_compute_mode =
      path_compute_snapshot != NULL ? path_compute_snapshot->decision.compute_mode : (uint32_t)PROM_VK_COMPUTE_BASELINE;
  const uint32_t selected_path =
      path_compute_snapshot != NULL ? path_compute_snapshot->decision.selected_path : (uint32_t)PROM_VK_PATH_DIRECT;
  const uint32_t requested_path =
      path_compute_snapshot != NULL ? path_compute_snapshot->decision.requested_path : (uint32_t)PROM_VK_PATH_DIRECT;
  const uint32_t force_direct_reason = prom_sgemm_effective_force_direct_reason(path_compute_snapshot);
  const uint32_t force_direct_requested = path_compute_snapshot != NULL ? path_compute_snapshot->facts.force_direct : 0u;
  const uint32_t force_direct_applied =
      (selected_path == (uint32_t)PROM_VK_PATH_DIRECT && force_direct_reason != PROM_SGEMM_FORCE_DIRECT_REASON_NONE) ? 1u : 0u;
  rt->slot_diag.p13_m16b1_requested_occupancy_variant = requested_variant;
  rt->slot_diag.p13_m16b1_executed_occupancy_variant =
      prom_occ_variant_executed_identity_for_dispatch(requested_variant, selected_compute_mode);
  rt->slot_diag.p13_m16b1_variant_registered = prom_occ_variant_registered(requested_variant);
  rt->slot_diag.p13_m16b1_variant_benchmark_enabled = rt->slot_diag.p13_m16b1_variant_registered;
  rt->slot_diag.p13_m16b1_variant_dvt_validated =
      (requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) ? 1u : 0u;
  rt->slot_diag.p13_m16b1_variant_pvt_validated =
      (requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) ? 1u : 0u;
  rt->slot_diag.p13_m16b1_variant_production_eligible = evt_dispatchable;
  rt->slot_diag.p13_m16b1_variant_dispatch_enabled = evt_dispatchable;
  rt->slot_diag.p13_m16b1_variant_path_status = path_status;
  rt->slot_diag.p13_m16b1_variant_path_id = prom_occ_variant_path_id(requested_variant);
  rt->slot_diag.p13_m16b1_fallback_reason = prom_occ_variant_fallback_reason(requested_variant);

  rt->slot_diag.px16_m6_selector_recommended_variant = rt->slot_diag.p13_m2_occupancy_unclamped_variant;
  rt->slot_diag.px16_m6_selector_selected_variant = rt->slot_diag.p13_m2_occupancy_selected_variant;
  rt->slot_diag.px16_m6_requested_dispatch_variant = requested_variant;
  rt->slot_diag.px16_m6_executed_dispatch_variant = rt->slot_diag.p13_m16b1_executed_occupancy_variant;
  rt->slot_diag.px16_m6_requested_path = requested_path;
  rt->slot_diag.px16_m6_selected_path = selected_path;
  rt->slot_diag.px16_m6_executed_path = selected_path;
  rt->slot_diag.px16_m6_requested_compute_mode = selected_compute_mode;
  rt->slot_diag.px16_m6_selected_compute_mode = selected_compute_mode;
  rt->slot_diag.px16_m6_executed_compute_mode = selected_compute_mode;
  rt->slot_diag.px16_m6_force_direct_requested = force_direct_requested;
  rt->slot_diag.px16_m6_force_direct_applied = force_direct_applied;
  rt->slot_diag.px16_m6_force_direct_reason = force_direct_reason;
  rt->slot_diag.px16_m6_policy_mode = policy_mode;
  rt->slot_diag.px16_m6_variant_path_status = rt->slot_diag.p13_m16b1_variant_path_status;
  rt->slot_diag.px16_m6_variant_production_eligible = rt->slot_diag.p13_m16b1_variant_production_eligible;
  rt->slot_diag.px16_m6_variant_dispatch_enabled = rt->slot_diag.p13_m16b1_variant_dispatch_enabled;
  rt->slot_diag.px16_m6_variant_dvt_validated = rt->slot_diag.p13_m16b1_variant_dvt_validated;
  rt->slot_diag.px16_m6_variant_pvt_validated = rt->slot_diag.p13_m16b1_variant_pvt_validated;
  rt->slot_diag.px16_m6_variant_lifecycle_telemetry_only = 1u;
  rt->slot_diag.px16_m6_p15_reservation_present = rt->p15_feedforward_dispatch_state.reservation_present;
  rt->slot_diag.px16_m6_p15_reservation_matured = rt->p15_feedforward_dispatch_state.reservation_matured;
  rt->slot_diag.px16_m6_p15_reservation_consumed = rt->p15_feedforward_dispatch_state.reservation_consumed;
  rt->slot_diag.px16_m6_p15_reserved_variant_id = rt->p15_feedforward_dispatch_state.reserved_variant_id;
  rt->slot_diag.px16_m6_p15_live_selected_variant_id = rt->p15_feedforward_dispatch_state.selected_variant_id;
  rt->slot_diag.px16_m6_p15_reconciliation_match = rt->p15_feedforward_dispatch_state.reconciliation_match;
  rt->slot_diag.px16_m6_p15_block_reason = rt->p15_feedforward_dispatch_state.block_reason;
  rt->slot_diag.px16_m6_p15_correction_action = rt->p15_feedforward_dispatch_state.correction_action;
  rt->slot_diag.px16_m6_p15_reservation_stale_or_expired = rt->p15_feedforward_dispatch_state.reservation_stale_or_expired;
  rt->slot_diag.px16_m6_p15_confidence_before = rt->p15_feedforward_dispatch_state.confidence_before;
  rt->slot_diag.px16_m6_p15_confidence_after = rt->p15_feedforward_dispatch_state.confidence_after;
}
/* Promotion seam terms:
 * DVT: local GPU correctness/sanity.
 * PVT: broader cloud/borrowed GPU validation.
 * production_eligible / dispatch_enabled: EVT-wired variants with real paths may dispatch;
 * DVT/PVT lifecycle fields remain telemetry only and do not gate a wired variant.
 */

typedef struct prom_sgemm_audit_execution_override {
  const prom_sgemm_audit_execution_descriptor* descriptor;
  VkPipeline pipeline;
} prom_sgemm_audit_execution_override;

static int prom_reactor_runtime_sgemm_impl_with_variant(void* handle,
                                                        const float* a,
                                                        const float* b,
                                                        float* c,
                                                        uint32_t m,
                                                        uint32_t n,
                                                        uint32_t k,
                                                        uint32_t requested_variant,
                                                        uint32_t selector_controls_dispatch_variant,
                                                        const prom_sgemm_audit_execution_override* audit_override,
                                                        uint32_t* out_stage,
                                                        int* out_detail_code);

int prom_reactor_runtime_sgemm_impl(void* handle,
                                     const float* a,
                                     const float* b,
                                     float* c,
                                     uint32_t m,
                                     uint32_t n,
                                     uint32_t k,
                                     uint32_t* out_stage,
                                     int* out_detail_code) {
  return prom_reactor_runtime_sgemm_impl_with_variant(handle, a, b, c, m, n, k,
                                                      PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR,
                                                      1u,
                                                      NULL,
                                                      out_stage, out_detail_code);
}

int prom_reactor_runtime_sgemm_benchmark_variant_impl(void* handle,
                                                      const float* a,
                                                      const float* b,
                                                      float* c,
                                                      uint32_t m,
                                                      uint32_t n,
                                                      uint32_t k,
                                                      uint32_t requested_variant,
                                                      uint32_t* out_stage,
                                                      int* out_detail_code) {
  return prom_reactor_runtime_sgemm_impl_with_variant(handle, a, b, c, m, n, k, requested_variant,
                                                      0u,
                                                      NULL,
                                                      out_stage, out_detail_code);
}

int prom_reactor_runtime_sgemm_audit_impl(void* handle,
                                          const float* a,
                                          const float* b,
                                          float* c,
                                          uint32_t m,
                                          uint32_t n,
                                          uint32_t k,
                                          const prom_sgemm_audit_execution_descriptor* descriptor,
                                          prom_sgemm_audit_execution_result* out_result) {
  uint64_t sample = 0u;
  return prom_reactor_runtime_sgemm_audit_benchmark_impl(handle, a, b, c, m, n, k, descriptor,
                                                         0u, 1u, &sample, 1u, out_result);
}

int prom_reactor_runtime_sgemm_audit_benchmark_impl(void* handle,
                                                    const float* a,
                                                    const float* b,
                                                    float* c,
                                                    uint32_t m,
                                                    uint32_t n,
                                                    uint32_t k,
                                                    const prom_sgemm_audit_execution_descriptor* descriptor,
                                                    uint32_t warmup,
                                                    uint32_t iterations,
                                                    uint64_t* out_samples_ns,
                                                    uint32_t sample_capacity,
                                                    prom_sgemm_audit_execution_result* out_result) {
  prometheus_runtime* rt;
  VkShaderModule module = VK_NULL_HANDLE;
  VkPipeline pipeline = VK_NULL_HANDLE;
  VkPipelineShaderStageCreateInfo stage_info;
  VkComputePipelineCreateInfo pipeline_info;
  VkResult result;
  uint32_t stage = PROM_STAGE_NONE;
  int detail = 0;
  int execution_result = PROM_ERROR;
  uint32_t dispatch_index;
  uint32_t completed_warmup = 0u;
  uint32_t completed_measured = 0u;

  if (out_result != NULL) memset(out_result, 0, sizeof(*out_result));
  if (handle == NULL || descriptor == NULL || descriptor->spirv_words == NULL || descriptor->spirv_size_bytes == 0u ||
      (descriptor->spirv_size_bytes % sizeof(uint32_t)) != 0u || descriptor->spirv_words[0] != 0x07230203u ||
      descriptor->entry_point == NULL || descriptor->entry_point[0] == '\0' || descriptor->dispatch.threads_x == 0u ||
      descriptor->dispatch.threads_y == 0u || descriptor->dispatch.threads_z == 0u ||
      descriptor->dispatch.outputs_per_invocation_m == 0u || descriptor->dispatch.outputs_per_invocation_n == 0u ||
      iterations == 0u || out_samples_ns == NULL || sample_capacity < iterations || warmup > UINT32_MAX - iterations) {
    if (out_result != NULL) { out_result->stage = PROM_STAGE_INIT; out_result->detail_code = PROM_ERROR; }
    return PROM_ERROR;
  }
  if (!registry_contains(handle)) return PROM_INVALID_HANDLE;
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC || rt->vulkan.available == 0u) return PROM_ERROR;
  if ((rt->vulkan.test_flags & PROM_TESTCFG_SKIP_SUBMIT_WAIT) != 0u) return PROM_ERROR;
  if (descriptor->compute_mode != (uint32_t)PROM_VK_COMPUTE_BASELINE &&
      descriptor->compute_mode != (uint32_t)PROM_VK_COMPUTE_TILED &&
      descriptor->compute_mode != (uint32_t)PROM_VK_COMPUTE_PACKED4_FP32 &&
      descriptor->compute_mode != (uint32_t)PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) return PROM_ERROR;
  if ((descriptor->dispatch.workgroup_output_m == 0u) != (descriptor->dispatch.workgroup_output_n == 0u) ||
    (descriptor->dispatch.workgroup_output_m != 0u &&
    ((m % descriptor->dispatch.workgroup_output_m) != 0u ||
     (n % descriptor->dispatch.workgroup_output_n) != 0u ||
     (descriptor->dispatch.tile_k != 0u && (k % descriptor->dispatch.tile_k) != 0u)))) {
    if (out_result != NULL) { out_result->stage = PROM_STAGE_INIT; out_result->detail_code = VK_ERROR_FEATURE_NOT_PRESENT; }
    return PROM_ERROR;
  }
  if (descriptor->require_full_subgroups != 0u &&
    (rt->vulkan.cooperative_matrix_feature_enabled == 0u || rt->vulkan.subgroup_size != descriptor->dispatch.threads_x)) {
    if (out_result != NULL) { out_result->stage = PROM_STAGE_INIT; out_result->detail_code = VK_ERROR_FEATURE_NOT_PRESENT; }
    return PROM_ERROR;
  }

  result = prom_audit_create_arbitrary_spirv_module(rt->vulkan.device, descriptor, &module);
  if (result != VK_SUCCESS) { if (out_result != NULL) { out_result->stage = PROM_STAGE_INIT; out_result->detail_code = (int)result; } return PROM_ERROR; }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = module;
  stage_info.pName = descriptor->entry_point;
#ifdef VK_PIPELINE_SHADER_STAGE_CREATE_REQUIRE_FULL_SUBGROUPS_BIT
  if (descriptor->require_full_subgroups != 0u) {
    stage_info.flags |= VK_PIPELINE_SHADER_STAGE_CREATE_REQUIRE_FULL_SUBGROUPS_BIT;
  }
#endif
  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->vulkan.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &pipeline);
  if (result != VK_SUCCESS) {
    vkDestroyShaderModule(rt->vulkan.device, module, NULL);
    if (out_result != NULL) { out_result->stage = PROM_STAGE_INIT; out_result->detail_code = (int)result; }
    return PROM_ERROR;
  }
  {
    prom_sgemm_audit_execution_override execution;
    execution.descriptor = descriptor;
    execution.pipeline = pipeline;
    for (dispatch_index = 0u; dispatch_index < warmup + iterations; ++dispatch_index) {
    execution_result = prom_reactor_runtime_sgemm_impl_with_variant(handle, a, b, c, m, n, k,
                                                                      PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR, 0u,
                                                                      &execution, &stage, &detail);
      if (execution_result != PROM_OK) break;
      if (dispatch_index < warmup) {
        completed_warmup += 1u;
      } else {
        if (rt->last_gpu_timing_valid == 0u) {
          execution_result = PROM_ERROR;
          stage = PROM_STAGE_SUBMIT;
          detail = PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_UNAVAILABLE;
          break;
        }
        out_samples_ns[completed_measured] = rt->last_gpu_duration_ns;
        completed_measured += 1u;
      }
    }
  }
  vkDestroyPipeline(rt->vulkan.device, pipeline, NULL);
  vkDestroyShaderModule(rt->vulkan.device, module, NULL);
  if (out_result != NULL) {
    const prom_vk_buffer* result_a;
    const prom_vk_buffer* result_b;
    const prom_vk_buffer* result_c;
    out_result->stage = stage;
    out_result->detail_code = detail;
    out_result->dispatch_geometry = prom_sgemm_dispatch_geometry_for_metadata(m, n, &descriptor->dispatch);
    out_result->gpu_timing_valid = rt->last_gpu_timing_valid;
    out_result->gpu_duration_ns = rt->last_gpu_duration_ns;
    out_result->pipeline_create_count = 1u;
    out_result->warmup_dispatch_count = completed_warmup;
    out_result->measured_dispatch_count = completed_measured;
    out_result->dispatches_per_sample = 1u;
    out_result->timestamp_interval_command_mask = PROM_SGEMM_AUDIT_TIMESTAMP_DISPATCH;
    out_result->query_reset_before_start_timestamp = 1u;
    out_result->fence_wait_before_query_results = 1u;
    out_result->selected_path = rt->slot_diag.px16_m6_selected_path;
    out_result->compute_mode = descriptor->compute_mode;
    out_result->compute_queue_family_index = rt->vulkan.queue_family_index;
    out_result->push_constant_m = m;
    out_result->push_constant_n = n;
    out_result->push_constant_k = descriptor->compute_mode == (uint32_t)PROM_VK_COMPUTE_PACKED4_FP32 ? prom_round_up4_u32(k) : k;
    if (out_result->selected_path == (uint32_t)PROM_VK_PATH_DIRECT) {
      result_a = &rt->direct_a;
      result_b = &rt->direct_b;
      result_c = &rt->direct_c;
    } else {
      result_a = &rt->staged_device_a;
      result_b = &rt->staged_device_b;
      result_c = &rt->staged_device_c;
    }
    out_result->a_memory_type_index = result_a->memory_type_index;
    out_result->b_memory_type_index = result_b->memory_type_index;
    out_result->c_memory_type_index = result_c->memory_type_index;
    out_result->a_memory_property_flags = (uint32_t)result_a->memory_property_flags;
    out_result->b_memory_property_flags = (uint32_t)result_b->memory_property_flags;
    out_result->c_memory_property_flags = (uint32_t)result_c->memory_property_flags;
    out_result->a_usage_flags = (uint32_t)result_a->usage_flags;
    out_result->b_usage_flags = (uint32_t)result_b->usage_flags;
    out_result->c_usage_flags = (uint32_t)result_c->usage_flags;
    out_result->a_buffer_bytes = (uint64_t)result_a->size;
    out_result->b_buffer_bytes = (uint64_t)result_b->size;
    out_result->c_buffer_bytes = (uint64_t)result_c->size;
    out_result->a_memory_alignment = (uint64_t)result_a->memory_alignment;
    out_result->b_memory_alignment = (uint64_t)result_b->memory_alignment;
    out_result->c_memory_alignment = (uint64_t)result_c->memory_alignment;
    out_result->a_memory_offset = (uint64_t)result_a->memory_offset;
    out_result->b_memory_offset = (uint64_t)result_b->memory_offset;
    out_result->c_memory_offset = (uint64_t)result_c->memory_offset;
  }
  if (execution_result == PROM_OK && descriptor->require_full_subgroups != 0u) {
    rt->vulkan.cooperative_matrix_state = PROM_VK_COOPERATIVE_MATRIX_EXECUTABLE;
  }
  return execution_result == PROM_OK && completed_warmup == warmup && completed_measured == iterations ? PROM_OK : PROM_ERROR;
}

typedef struct prom_sgemm_placement_role_buffer {
  prom_vk_buffer storage;
  prom_vk_buffer transfer;
  uint32_t placement;
  uint32_t is_input;
} prom_sgemm_placement_role_buffer;

static void prom_sgemm_placement_destroy_role(VkDevice device, prom_sgemm_placement_role_buffer* role) {
  if (role == NULL) return;
  prom_vk_destroy_buffer(device, &role->transfer);
  prom_vk_destroy_buffer(device, &role->storage);
  memset(role, 0, sizeof(*role));
}

static VkResult prom_sgemm_placement_create_role(prometheus_runtime* rt,
                                                 VkDeviceSize size,
                                                 uint32_t placement,
                                                 uint32_t is_input,
                                                 prom_sgemm_placement_role_buffer* role) {
  VkBufferUsageFlags usage = VK_BUFFER_USAGE_STORAGE_BUFFER_BIT;
  VkResult result;
  if (rt == NULL || role == NULL || placement < PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL ||
      placement > PROM_SGEMM_MEMORY_PLACEMENT_HOST_VISIBLE_COHERENT_DEVICE_LOCAL) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }
  memset(role, 0, sizeof(*role));
  role->placement = placement;
  role->is_input = is_input;
  if (placement == PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL) {
    usage |= is_input != 0u ? VK_BUFFER_USAGE_TRANSFER_DST_BIT : VK_BUFFER_USAGE_TRANSFER_SRC_BIT;
  }
  result = prom_vk_create_buffer_for_placement(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags, size, usage,
                                                placement, placement != PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL,
                                                &role->storage);
  if (result != VK_SUCCESS) return result;
  if (placement == PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL) {
    const VkBufferUsageFlags transfer_usage = is_input != 0u ? VK_BUFFER_USAGE_TRANSFER_SRC_BIT : VK_BUFFER_USAGE_TRANSFER_DST_BIT;
    result = prom_vk_create_buffer_for_placement(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags, size, transfer_usage,
                                                  PROM_SGEMM_MEMORY_PLACEMENT_HOST_VISIBLE_COHERENT_SYSTEM, 1,
                                                  &role->transfer);
    if (result != VK_SUCCESS) {
      prom_sgemm_placement_destroy_role(rt->vulkan.device, role);
    }
  }
  return result;
}

static void prom_sgemm_placement_capture_role(const prometheus_runtime* rt,
                                              const prom_sgemm_placement_role_buffer* role,
                                              uint32_t* out_type,
                                              uint32_t* out_flags,
                                              uint32_t* out_heap) {
  VkPhysicalDeviceMemoryProperties properties;
  if (rt == NULL || role == NULL || out_type == NULL || out_flags == NULL || out_heap == NULL) return;
  vkGetPhysicalDeviceMemoryProperties(rt->vulkan.physical_device, &properties);
  *out_type = role->storage.memory_type_index;
  *out_flags = (uint32_t)role->storage.memory_property_flags;
  *out_heap = role->storage.memory_type_index < properties.memoryTypeCount
                  ? properties.memoryTypes[role->storage.memory_type_index].heapIndex
                  : UINT32_MAX;
}

static VkResult prom_sgemm_placement_allocate_roles(prometheus_runtime* rt,
                                                    VkDeviceSize a_size,
                                                    VkDeviceSize b_size,
                                                    VkDeviceSize c_size,
                                                    const prom_sgemm_placement_benchmark_options* options,
                                                    prom_sgemm_placement_role_buffer* a_role,
                                                    prom_sgemm_placement_role_buffer* b_role,
                                                    prom_sgemm_placement_role_buffer* c_role) {
  VkResult result = prom_sgemm_placement_create_role(rt, a_size, options->a_placement, 1u, a_role);
  if (result == VK_SUCCESS) result = prom_sgemm_placement_create_role(rt, b_size, options->b_placement, 1u, b_role);
  if (result == VK_SUCCESS) result = prom_sgemm_placement_create_role(rt, c_size, options->c_placement, 0u, c_role);
  if (result != VK_SUCCESS) {
    prom_sgemm_placement_destroy_role(rt->vulkan.device, c_role);
    prom_sgemm_placement_destroy_role(rt->vulkan.device, b_role);
    prom_sgemm_placement_destroy_role(rt->vulkan.device, a_role);
  }
  return result;
}

static void prom_sgemm_placement_update_descriptors(prometheus_runtime* rt,
                                                    const prom_sgemm_placement_role_buffer* a_role,
                                                    const prom_sgemm_placement_role_buffer* b_role,
                                                    const prom_sgemm_placement_role_buffer* c_role) {
  VkDescriptorBufferInfo infos[3];
  VkWriteDescriptorSet writes[3];
  uint32_t i;
  memset(infos, 0, sizeof(infos));
  infos[0].buffer = a_role->storage.buffer; infos[0].range = a_role->storage.size;
  infos[1].buffer = b_role->storage.buffer; infos[1].range = b_role->storage.size;
  infos[2].buffer = c_role->storage.buffer; infos[2].range = c_role->storage.size;
  memset(writes, 0, sizeof(writes));
  for (i = 0u; i < 3u; ++i) {
    writes[i].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
    writes[i].dstSet = rt->descriptor_set;
    writes[i].dstBinding = i;
    writes[i].descriptorCount = 1u;
    writes[i].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    writes[i].pBufferInfo = &infos[i];
  }
  vkUpdateDescriptorSets(rt->vulkan.device, 3u, writes, 0u, NULL);
}

static void prom_sgemm_placement_write_payload(prom_sgemm_placement_role_buffer* role,
                                               const void* payload,
                                               size_t size) {
  void* destination;
  if (role == NULL || payload == NULL) return;
  destination = role->placement == PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL
                    ? role->transfer.mapped
                    : role->storage.mapped;
  if (destination != NULL) memcpy(destination, payload, size);
}

int prom_reactor_runtime_sgemm_placement_benchmark_detailed_impl(void* handle,
                                                        const float* a,
                                                        const float* b,
                                                        float* c,
                                                        uint32_t m,
                                                        uint32_t n,
                                                        uint32_t k,
                                                        const prom_sgemm_audit_execution_descriptor* descriptor,
                                                        const prom_sgemm_placement_benchmark_options* options,
                                                        uint64_t* out_gpu_samples_ns,
                                                        uint64_t* out_preparation_samples_ns,
                                                        uint64_t* out_end_to_end_samples_ns,
                                                        uint64_t* out_conversion_samples_ns,
                                                        uint64_t* out_upload_samples_ns,
                                                        uint64_t* out_readback_samples_ns,
                                                        uint32_t sample_capacity,
                                                        prom_sgemm_placement_benchmark_result* out_result) {
  prometheus_runtime* rt;
  prom_sgemm_placement_role_buffer a_role;
  prom_sgemm_placement_role_buffer b_role;
  prom_sgemm_placement_role_buffer c_role;
  prom_vk_buffer perturbation;
  VkShaderModule module = VK_NULL_HANDLE;
  VkPipeline pipeline = VK_NULL_HANDLE;
  VkQueryPool timing_query_pool = VK_NULL_HANDLE;
  VkPipelineShaderStageCreateInfo shader_stage;
  VkComputePipelineCreateInfo pipeline_info;
  VkQueryPoolCreateInfo query_pool_info;
  VkDeviceSize a_size = 0u, b_size = 0u, c_size = 0u;
  size_t a_copy_size = 0u, b_copy_size = 0u, c_copy_size = 0u;
  void* packed_a = NULL;
  void* packed_b = NULL;
  const void* a_payload = a;
  const void* b_payload = b;
  uint32_t compute_k;
  uint32_t dispatch_index;
  uint32_t completed = 0u;
  uint32_t persistent_allocated = 0u;
  uint32_t total_dispatches;
  uint64_t initial_begin;
  VkResult vk_result = VK_SUCCESS;
  int status = PROM_ERROR;
  if (out_result != NULL) memset(out_result, 0, sizeof(*out_result));
  memset(&a_role, 0, sizeof(a_role)); memset(&b_role, 0, sizeof(b_role)); memset(&c_role, 0, sizeof(c_role));
  memset(&perturbation, 0, sizeof(perturbation));
  if (handle == NULL || !registry_contains(handle) || a == NULL || b == NULL || c == NULL || descriptor == NULL ||
      options == NULL || out_gpu_samples_ns == NULL || out_preparation_samples_ns == NULL ||
      out_end_to_end_samples_ns == NULL || options->iterations == 0u || sample_capacity < options->iterations ||
      options->warmup > UINT32_MAX - options->iterations || options->reuse_mode < PROM_SGEMM_PLACEMENT_REUSE_COLD_ALLOCATION ||
      options->reuse_mode > PROM_SGEMM_PLACEMENT_REUSE_PERSISTENT_B_REUPLOAD_A) {
    if (out_result != NULL) { out_result->stage = PROM_STAGE_INIT; out_result->detail_code = PROM_ERROR; }
    return PROM_ERROR;
  }
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC || rt->vulkan.available == 0u || rt->timestamp_query_supported == 0u) return PROM_ERROR;
  if ((descriptor->dispatch.workgroup_output_m == 0u) != (descriptor->dispatch.workgroup_output_n == 0u) ||
    (descriptor->dispatch.workgroup_output_m != 0u &&
     ((m % descriptor->dispatch.workgroup_output_m) != 0u ||
      (n % descriptor->dispatch.workgroup_output_n) != 0u ||
      (descriptor->dispatch.tile_k != 0u && (k % descriptor->dispatch.tile_k) != 0u))) ||
    (descriptor->require_full_subgroups != 0u &&
     (rt->vulkan.cooperative_matrix_feature_enabled == 0u || rt->vulkan.subgroup_size != descriptor->dispatch.threads_x))) {
    if (out_result != NULL) { out_result->stage = PROM_STAGE_INIT; out_result->detail_code = VK_ERROR_FEATURE_NOT_PRESENT; }
    return PROM_ERROR;
  }
  compute_k = descriptor->compute_mode == (uint32_t)PROM_VK_COMPUTE_PACKED4_FP32 ? prom_round_up4_u32(k) : k;
  if (descriptor->compute_mode == (uint32_t)PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
    if (!checked_packed_fp16_buffer_size(m, compute_k, &a_size, &a_copy_size) ||
        !checked_packed_fp16_buffer_size(compute_k, n, &b_size, &b_copy_size)) goto cleanup;
  } else {
    if (!checked_float_buffer_size(m, compute_k, &a_size, &a_copy_size) ||
        !checked_float_buffer_size(descriptor->compute_mode == (uint32_t)PROM_VK_COMPUTE_PACKED4_FP32 ? n : compute_k,
                                   descriptor->compute_mode == (uint32_t)PROM_VK_COMPUTE_PACKED4_FP32 ? compute_k : n,
                                   &b_size, &b_copy_size)) goto cleanup;
  }
  if (!checked_float_buffer_size(m, n, &c_size, &c_copy_size)) goto cleanup;
  if (descriptor->compute_mode == (uint32_t)PROM_VK_COMPUTE_PACKED4_FP32 ||
      descriptor->compute_mode == (uint32_t)PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
    packed_a = malloc(a_copy_size); packed_b = malloc(b_copy_size);
    if (packed_a == NULL || packed_b == NULL) goto cleanup;
    a_payload = packed_a; b_payload = packed_b;
  }
  vk_result = prom_audit_create_arbitrary_spirv_module(rt->vulkan.device, descriptor, &module);
  if (vk_result != VK_SUCCESS) goto cleanup;
  memset(&shader_stage, 0, sizeof(shader_stage));
  shader_stage.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  shader_stage.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  shader_stage.module = module;
  shader_stage.pName = descriptor->entry_point;
#ifdef VK_PIPELINE_SHADER_STAGE_CREATE_REQUIRE_FULL_SUBGROUPS_BIT
  if (descriptor->require_full_subgroups != 0u) {
    shader_stage.flags |= VK_PIPELINE_SHADER_STAGE_CREATE_REQUIRE_FULL_SUBGROUPS_BIT;
  }
#endif
  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = shader_stage;
  pipeline_info.layout = rt->pipeline_layout;
  vk_result = vkCreateComputePipelines(rt->vulkan.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &pipeline);
  if (vk_result != VK_SUCCESS) goto cleanup;
  memset(&query_pool_info, 0, sizeof(query_pool_info));
  query_pool_info.sType = VK_STRUCTURE_TYPE_QUERY_POOL_CREATE_INFO;
  query_pool_info.queryType = VK_QUERY_TYPE_TIMESTAMP;
  query_pool_info.queryCount = 4u;
  vk_result = vkCreateQueryPool(rt->vulkan.device, &query_pool_info, NULL, &timing_query_pool);
  if (vk_result != VK_SUCCESS) goto cleanup;
  initial_begin = prom_wall_clock_now_ns();
  if (options->reuse_mode != PROM_SGEMM_PLACEMENT_REUSE_COLD_ALLOCATION) {
    vk_result = prom_sgemm_placement_allocate_roles(rt, a_size, b_size, c_size, options, &a_role, &b_role, &c_role);
    if (vk_result != VK_SUCCESS) goto unsupported;
    persistent_allocated = 1u;
    prom_sgemm_placement_update_descriptors(rt, &a_role, &b_role, &c_role);
    if (out_result != NULL) { out_result->allocation_count += 3u; out_result->descriptor_update_count += 1u; }
  }
  if (options->perturb_cache != 0u && options->cache_perturbation_bytes >= 4u) {
    vk_result = prom_vk_create_buffer_for_placement(rt->vulkan.physical_device, rt->vulkan.device, rt->vulkan.test_flags,
                                                     (VkDeviceSize)(options->cache_perturbation_bytes & ~3ull),
                                                     VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                                     PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL, 0, &perturbation);
    if (vk_result != VK_SUCCESS) goto unsupported;
  }
  if (out_result != NULL) out_result->initial_preparation_ns = prom_wall_clock_elapsed_ns(initial_begin, prom_wall_clock_now_ns());
  total_dispatches = options->warmup + options->iterations;
  for (dispatch_index = 0u; dispatch_index < total_dispatches; ++dispatch_index) {
    VkCommandBufferBeginInfo begin_info;
    VkSubmitInfo submit_info;
    VkBufferMemoryBarrier barriers[8];
    VkBufferCopy copies[3];
    prom_vk_push push;
    prom_sgemm_dispatch_geometry geometry;
    uint64_t timestamps[4];
    uint64_t e2e_begin = prom_wall_clock_now_ns();
    uint64_t prep_begin = e2e_begin;
    uint64_t prep_end;
    uint64_t readback_begin;
    uint64_t readback_end;
    uint32_t barrier_count = 0u;
    uint32_t write_a = options->reuse_mode == PROM_SGEMM_PLACEMENT_REUSE_REUPLOAD ||
                       options->reuse_mode == PROM_SGEMM_PLACEMENT_REUSE_PERSISTENT_B_REUPLOAD_A ||
                       options->reuse_mode == PROM_SGEMM_PLACEMENT_REUSE_COLD_ALLOCATION || dispatch_index == 0u;
    uint32_t write_b = options->reuse_mode == PROM_SGEMM_PLACEMENT_REUSE_REUPLOAD ||
                       options->reuse_mode == PROM_SGEMM_PLACEMENT_REUSE_COLD_ALLOCATION || dispatch_index == 0u;
    if (options->reuse_mode == PROM_SGEMM_PLACEMENT_REUSE_COLD_ALLOCATION) {
      vk_result = prom_sgemm_placement_allocate_roles(rt, a_size, b_size, c_size, options, &a_role, &b_role, &c_role);
      if (vk_result != VK_SUCCESS) goto unsupported;
      prom_sgemm_placement_update_descriptors(rt, &a_role, &b_role, &c_role);
      if (out_result != NULL) { out_result->allocation_count += 3u; out_result->descriptor_update_count += 1u; }
    }
    if (out_result != NULL && out_result->a_buffer_bytes == 0u) {
      out_result->a_buffer_bytes = (uint64_t)a_size; out_result->b_buffer_bytes = (uint64_t)b_size; out_result->c_buffer_bytes = (uint64_t)c_size;
      prom_sgemm_placement_capture_role(rt, &a_role, &out_result->a_memory_type_index, &out_result->a_memory_property_flags, &out_result->a_heap_index);
      prom_sgemm_placement_capture_role(rt, &b_role, &out_result->b_memory_type_index, &out_result->b_memory_property_flags, &out_result->b_heap_index);
      prom_sgemm_placement_capture_role(rt, &c_role, &out_result->c_memory_type_index, &out_result->c_memory_property_flags, &out_result->c_heap_index);
    }
    if (write_a != 0u || write_b != 0u) {
      if (descriptor->compute_mode == (uint32_t)PROM_VK_COMPUTE_PACKED4_FP32) {
        if (write_a != 0u) prom_pack_a_packed4_rowmajor(a, (float*)packed_a, m, k, compute_k);
        if (write_b != 0u) prom_pack_b_packed4_colmajor(b, (float*)packed_b, n, k, compute_k);
      } else if (descriptor->compute_mode == (uint32_t)PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
        if (write_a != 0u) prom_pack_fp16_pairs(a, m * compute_k, (uint32_t*)packed_a);
        if (write_b != 0u) prom_pack_fp16_pairs(b, compute_k * n, (uint32_t*)packed_b);
      }
      if (write_a != 0u) prom_sgemm_placement_write_payload(&a_role, a_payload, a_copy_size);
      if (write_b != 0u) prom_sgemm_placement_write_payload(&b_role, b_payload, b_copy_size);
    }
    prep_end = prom_wall_clock_now_ns();
    vk_result = vkResetCommandBuffer(rt->command_buffer, 0u);
    if (vk_result != VK_SUCCESS) goto cleanup;
    memset(&begin_info, 0, sizeof(begin_info)); begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    vk_result = vkBeginCommandBuffer(rt->command_buffer, &begin_info);
    if (vk_result != VK_SUCCESS) goto cleanup;
    vkCmdResetQueryPool(rt->command_buffer, timing_query_pool, 0u, 4u);
    vkCmdWriteTimestamp(rt->command_buffer, VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT, timing_query_pool, 0u);
    memset(barriers, 0, sizeof(barriers)); memset(copies, 0, sizeof(copies));
    if (write_a != 0u || write_b != 0u) {
      prom_sgemm_placement_role_buffer* roles[2] = {&a_role, &b_role};
    uint32_t writes[2] = {write_a, write_b};
      uint32_t role_index;
      for (role_index = 0u; role_index < 2u; ++role_index) {
        prom_sgemm_placement_role_buffer* role = roles[role_index];
    if (writes[role_index] == 0u) continue;
        if (role->placement == PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL) {
          VkBufferMemoryBarrier host_barrier;
          VkBufferCopy copy;
          memset(&host_barrier, 0, sizeof(host_barrier));
          host_barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
          host_barrier.srcAccessMask = VK_ACCESS_HOST_WRITE_BIT; host_barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
          host_barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED; host_barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
          host_barrier.buffer = role->transfer.buffer; host_barrier.size = role->transfer.size;
          vkCmdPipelineBarrier(rt->command_buffer, VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
                               0u, 0u, NULL, 1u, &host_barrier, 0u, NULL);
          memset(&copy, 0, sizeof(copy)); copy.size = role->storage.size;
          vkCmdCopyBuffer(rt->command_buffer, role->transfer.buffer, role->storage.buffer, 1u, &copy);
          barriers[barrier_count] = host_barrier;
          barriers[barrier_count].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
          barriers[barrier_count].dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
          barriers[barrier_count].buffer = role->storage.buffer;
          barrier_count += 1u;
        } else {
          barriers[barrier_count].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
          barriers[barrier_count].srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
          barriers[barrier_count].dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
          barriers[barrier_count].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
          barriers[barrier_count].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
          barriers[barrier_count].buffer = role->storage.buffer;
          barriers[barrier_count].size = role->storage.size;
          barrier_count += 1u;
        }
      }
    }
    if (perturbation.buffer != VK_NULL_HANDLE) {
      VkBufferMemoryBarrier perturb_barrier;
      vkCmdFillBuffer(rt->command_buffer, perturbation.buffer, 0u, VK_WHOLE_SIZE, dispatch_index * 2654435761u);
      memset(&perturb_barrier, 0, sizeof(perturb_barrier));
      perturb_barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
      perturb_barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT; perturb_barrier.dstAccessMask = VK_ACCESS_MEMORY_READ_BIT;
      perturb_barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED; perturb_barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
      perturb_barrier.buffer = perturbation.buffer; perturb_barrier.size = perturbation.size;
      vkCmdPipelineBarrier(rt->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           0u, 0u, NULL, 1u, &perturb_barrier, 0u, NULL);
    }
    if (barrier_count != 0u) {
      vkCmdPipelineBarrier(rt->command_buffer,
                           VK_PIPELINE_STAGE_HOST_BIT | VK_PIPELINE_STAGE_TRANSFER_BIT,
                           VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           0u, 0u, NULL, barrier_count, barriers, 0u, NULL);
    }
    vkCmdBindPipeline(rt->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, pipeline);
    vkCmdBindDescriptorSets(rt->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, rt->pipeline_layout,
                            0u, 1u, &rt->descriptor_set, 0u, NULL);
    push.m = m; push.n = n; push.k = compute_k;
    vkCmdPushConstants(rt->command_buffer, rt->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT, 0u, PROM_VK_SHADER_PUSH_BYTES, &push);
    vkCmdWriteTimestamp(rt->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, timing_query_pool, 1u);
    geometry = prom_sgemm_dispatch_geometry_for_metadata(m, n, &descriptor->dispatch);
    vkCmdDispatch(rt->command_buffer, geometry.groups_x, geometry.groups_y, geometry.groups_z);
    vkCmdWriteTimestamp(rt->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, timing_query_pool, 2u);
    memset(&barriers[0], 0, sizeof(barriers[0]));
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED; barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = c_role.storage.buffer; barriers[0].size = c_role.storage.size;
    if (c_role.placement == PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL) {
      barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
      vkCmdPipelineBarrier(rt->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
                           0u, 0u, NULL, 1u, barriers, 0u, NULL);
      copies[2].size = c_role.storage.size;
      vkCmdCopyBuffer(rt->command_buffer, c_role.storage.buffer, c_role.transfer.buffer, 1u, &copies[2]);
      barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT; barriers[0].dstAccessMask = VK_ACCESS_HOST_READ_BIT;
      barriers[0].buffer = c_role.transfer.buffer; barriers[0].size = c_role.transfer.size;
      vkCmdPipelineBarrier(rt->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT,
                           0u, 0u, NULL, 1u, barriers, 0u, NULL);
    } else {
      barriers[0].dstAccessMask = VK_ACCESS_HOST_READ_BIT;
      vkCmdPipelineBarrier(rt->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_HOST_BIT,
                           0u, 0u, NULL, 1u, barriers, 0u, NULL);
    }
    vkCmdWriteTimestamp(rt->command_buffer, VK_PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT, timing_query_pool, 3u);
    vk_result = vkEndCommandBuffer(rt->command_buffer);
    if (vk_result != VK_SUCCESS) goto cleanup;
    vk_result = vkResetFences(rt->vulkan.device, 1u, &rt->submit_fence);
    if (vk_result != VK_SUCCESS) goto cleanup;
    memset(&submit_info, 0, sizeof(submit_info)); submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
    submit_info.commandBufferCount = 1u; submit_info.pCommandBuffers = &rt->command_buffer;
    vk_result = vkQueueSubmit(rt->vulkan.compute_queue, 1u, &submit_info, rt->submit_fence);
    if (vk_result != VK_SUCCESS) goto cleanup;
    vk_result = vkWaitForFences(rt->vulkan.device, 1u, &rt->submit_fence, VK_TRUE, UINT64_MAX);
    if (vk_result != VK_SUCCESS) goto cleanup;
    vk_result = vkGetQueryPoolResults(rt->vulkan.device, timing_query_pool, 0u, 4u, sizeof(timestamps),
                                      timestamps, sizeof(uint64_t), VK_QUERY_RESULT_64_BIT | VK_QUERY_RESULT_WAIT_BIT);
    if (vk_result != VK_SUCCESS || timestamps[1] < timestamps[0] || timestamps[2] <= timestamps[1] ||
        timestamps[3] < timestamps[2]) goto cleanup;
    readback_begin = prom_wall_clock_now_ns();
    memcpy(c, c_role.placement == PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL
                  ? c_role.transfer.mapped : c_role.storage.mapped, c_copy_size);
    readback_end = prom_wall_clock_now_ns();
    if (dispatch_index >= options->warmup) {
      const uint32_t sample_index = dispatch_index - options->warmup;
      const uint64_t gpu_transfer_ticks = (timestamps[1] - timestamps[0]) + (timestamps[3] - timestamps[2]);
      out_gpu_samples_ns[sample_index] = (uint64_t)(((double)(timestamps[2] - timestamps[1])) * rt->vulkan.timestamp_period_ns);
    if (out_conversion_samples_ns != NULL) {
      out_conversion_samples_ns[sample_index] = prom_wall_clock_elapsed_ns(prep_begin, prep_end);
    }
    if (out_upload_samples_ns != NULL) {
      out_upload_samples_ns[sample_index] = (uint64_t)(((double)(timestamps[1] - timestamps[0])) * rt->vulkan.timestamp_period_ns);
    }
    if (out_readback_samples_ns != NULL) {
      out_readback_samples_ns[sample_index] =
        (uint64_t)(((double)(timestamps[3] - timestamps[2])) * rt->vulkan.timestamp_period_ns) +
        prom_wall_clock_elapsed_ns(readback_begin, readback_end);
    }
      out_preparation_samples_ns[sample_index] = prom_wall_clock_elapsed_ns(prep_begin, prep_end) +
          (uint64_t)(((double)gpu_transfer_ticks) * rt->vulkan.timestamp_period_ns) +
          prom_wall_clock_elapsed_ns(readback_begin, readback_end);
      out_end_to_end_samples_ns[sample_index] = prom_wall_clock_elapsed_ns(e2e_begin, prom_wall_clock_now_ns());
      completed += 1u;
      if (out_result != NULL) out_result->correctness_readback_count += 1u;
    }
    if (out_result != NULL) out_result->dispatch_count += 1u;
    if (options->reuse_mode == PROM_SGEMM_PLACEMENT_REUSE_COLD_ALLOCATION) {
      prom_sgemm_placement_destroy_role(rt->vulkan.device, &c_role);
      prom_sgemm_placement_destroy_role(rt->vulkan.device, &b_role);
      prom_sgemm_placement_destroy_role(rt->vulkan.device, &a_role);
    }
  }
  status = PROM_OK;
  if (out_result != NULL) {
    out_result->supported = 1u;
    out_result->completed_iterations = completed;
  }
  goto cleanup;

unsupported:
  if (out_result != NULL) { out_result->supported = 0u; out_result->detail_code = (int)vk_result; out_result->stage = PROM_STAGE_TRANSFER_IN; }
cleanup:
  if (out_result != NULL && status != PROM_OK && out_result->detail_code == 0) {
    out_result->detail_code = (int)(vk_result == VK_SUCCESS ? VK_ERROR_UNKNOWN : vk_result);
    out_result->stage = PROM_STAGE_SUBMIT;
  }
  prom_vk_destroy_buffer(rt != NULL ? rt->vulkan.device : VK_NULL_HANDLE, &perturbation);
  if (rt != NULL) {
    if (persistent_allocated != 0u || options->reuse_mode == PROM_SGEMM_PLACEMENT_REUSE_COLD_ALLOCATION) {
      prom_sgemm_placement_destroy_role(rt->vulkan.device, &c_role);
      prom_sgemm_placement_destroy_role(rt->vulkan.device, &b_role);
      prom_sgemm_placement_destroy_role(rt->vulkan.device, &a_role);
    }
    if (pipeline != VK_NULL_HANDLE) vkDestroyPipeline(rt->vulkan.device, pipeline, NULL);
    if (timing_query_pool != VK_NULL_HANDLE) vkDestroyQueryPool(rt->vulkan.device, timing_query_pool, NULL);
    if (module != VK_NULL_HANDLE) vkDestroyShaderModule(rt->vulkan.device, module, NULL);
  }
  free(packed_b); free(packed_a);
  return status;
}

int prom_reactor_runtime_sgemm_placement_benchmark_impl(void* handle,
                                                        const float* a,
                                                        const float* b,
                                                        float* c,
                                                        uint32_t m,
                                                        uint32_t n,
                                                        uint32_t k,
                                                        const prom_sgemm_audit_execution_descriptor* descriptor,
                                                        const prom_sgemm_placement_benchmark_options* options,
                                                        uint64_t* out_gpu_samples_ns,
                                                        uint64_t* out_preparation_samples_ns,
                                                        uint64_t* out_end_to_end_samples_ns,
                                                        uint32_t sample_capacity,
                                                        prom_sgemm_placement_benchmark_result* out_result) {
  return prom_reactor_runtime_sgemm_placement_benchmark_detailed_impl(
    handle, a, b, c, m, n, k, descriptor, options,
    out_gpu_samples_ns, out_preparation_samples_ns, out_end_to_end_samples_ns,
    NULL, NULL, NULL, sample_capacity, out_result);
}

static int prom_reactor_runtime_sgemm_impl_with_variant(void* handle,
                                     const float* a,
                                     const float* b,
                                     float* c,
                                     uint32_t m,
                                     uint32_t n,
                                     uint32_t k,
                                     uint32_t requested_variant,
                                     uint32_t selector_controls_dispatch_variant,
                                     const prom_sgemm_audit_execution_override* audit_override,
                                     uint32_t* out_stage,
                                     int* out_detail_code) {
  prometheus_runtime* rt;
  VkResult vk_result;
  VkWriteDescriptorSet writes[3];
  VkDescriptorBufferInfo buffer_infos[3];
  VkCommandBufferBeginInfo begin_info;
  VkSubmitInfo submit_info;
  VkSubmitInfo transfer_submit_info;
  VkPipelineStageFlags wait_stage_mask;
  VkBufferMemoryBarrier barriers[4];
  VkBufferCopy copies[3];
  prom_vk_push push;
  prom_vk_buffer* shader_a;
  prom_vk_buffer* shader_b;
  prom_vk_buffer* shader_c;
  VkDeviceSize a_buffer_size = 0;
  VkDeviceSize b_buffer_size = 0;
  VkDeviceSize c_buffer_size = 0;
  size_t a_copy_size = 0u;
  size_t b_copy_size = 0u;
  size_t c_copy_size = 0u;
  uint32_t compute_k;
  uint64_t work_units;
  uint32_t work_units_u32;
  uint32_t mn_product;
  uint32_t can_stage;
  uint32_t can_direct;
  uint32_t tiled_shape;
  uint32_t readback_required;
  uint32_t packed4_waste_permille;
  uint32_t packed4_budget_permille;
  uint32_t packed4_small_shape;
  uint32_t packed4_tail_count;
  uint32_t packed4_padded_lane_count;
  uint32_t fp16_has_special_values;
  int fp16_utility_score;
  prom_policy_mode policy_mode;
  prom_vk_path_mode selected_path;
  prom_vk_compute_mode compute_mode;
  prom_judgment_facts judgment_facts;
  prom_judgment_decision judgment_decision;
  prom_judgment_layout_precision_decision layout_precision_selector_decision;
  prom_judgment_async_facts async_facts;
  prom_judgment_async_decision async_decision;
  prom_buffering_selector_facts buffering_facts;
  prom_dom_sgemm_buffering_projection buffering_projection;
  prom_buffering_selector_decision buffering_decision;
  prom_occupancy_selector_facts occupancy_facts;
  prom_occupancy_selector_decision occupancy_decision;
  prom_dom_transfer_queue_facts transfer_queue_facts;
  prom_dom_transfer_queue_projection transfer_queue_projection;
  prom_dom_transfer_queue_decision transfer_queue_decision;
  prom_dom_transfer_queue_snapshot transfer_queue_snapshot;
  prom_dom_sgemm_layout_precision_facts layout_precision_facts;
  prom_dom_sgemm_layout_precision_projection layout_precision_projection;
  prom_dom_sgemm_layout_precision_decision layout_precision_decision;
  prom_sgemm_dispatch_geometry dispatch_geometry;
  prom_dom_sgemm_path_compute_facts path_compute_facts;
  prom_dom_sgemm_path_compute_projection path_compute_projection;
  prom_dom_sgemm_path_compute_decision path_compute_decision;
  prom_dom_sgemm_path_compute_snapshot path_compute_snapshot;
  prom_buffering_mode buffering_mode = PROM_BUFFERING_MODE_FIXED_DOUBLE_DEFAULT;
  prom_variance_class variance_class = PROM_VARIANCE_MODERATE;
  prom_predictability_class predictability_class = PROM_PREDICTABILITY_STABLE;
  VkPipeline selected_pipeline;
  float* packed_a_upload = NULL;
  float* packed_b_upload = NULL;
  uint32_t* fp16_a_upload = NULL;
  uint32_t* fp16_b_upload = NULL;
  int final_detail = 0;
  int prepare_detail = 0;
  uint32_t use_dedicated_transfer_upload = 0u;
  uint32_t request_async = 0u;
  uint32_t work_slot_id = 0u;
  uint64_t required_capacity_bytes = 0u;
  uint64_t layout_precision_dependency_dirty_mask = 0u;
  uint64_t layout_precision_path_guard_dirty_mask = 0u;
  uint32_t artifact_layout_code = 0u;
  uint32_t artifact_precision_code = 0u;
  prom_buffer_artifact_key artifact_a_key;
  prom_buffer_artifact_key artifact_b_key;
  prom_buffer_artifact_key artifact_c_key;
  prom_resource_lease_facts lease_facts;
  prom_resource_lease_decision lease_decision;
  prom_resource_lease_decision lease_yield_decision;
  uint32_t lease_granted = 0u;
  uint64_t total_wall_begin_ns = 0u;
  uint64_t pre_dispatch_begin_ns = 0u;
  uint64_t upload_begin_ns = 0u;
  uint64_t upload_end_ns = 0u;
  uint64_t command_record_begin_ns = 0u;
  uint64_t dispatch_submit_begin_ns = 0u;
  uint64_t dispatch_submit_end_ns = 0u;
  uint64_t sync_wait_begin_ns = 0u;
  uint64_t sync_wait_end_ns = 0u;
  uint64_t post_sync_begin_ns = 0u;
  uint64_t readback_begin_ns = 0u;
  uint64_t readback_end_ns = 0u;
  uint64_t post_readback_begin_ns = 0u;
  uint64_t function_return_ns = 0u;

  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_NONE, 0);
  total_wall_begin_ns = prom_wall_clock_now_ns();

  if (handle == NULL || !registry_contains(handle)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }

  rt = (prometheus_runtime*)handle;
  request_async = (((rt->vulkan.test_flags & PROM_TESTCFG_SKIP_SUBMIT_WAIT) != 0u) && c == NULL) ? 1u : 0u;
  if (a == NULL || b == NULL || (request_async == 0u && c == NULL)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  if (m == 0u || n == 0u || k == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_ERROR);
    return PROM_ERROR;
  }
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  if (rt->vulkan.available == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, rt->vulkan.init_detail_code);
    return PROM_ERROR;
  }
  if (rt->timestamp_query_supported != 0u && rt->sgemm_timestamp_query_pool != VK_NULL_HANDLE) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_UNAVAILABLE);
  } else if (rt->vulkan.timestamp_period_ns > 0.0f && rt->vulkan.timestamp_valid_bits > 0u) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_POOL_UNAVAILABLE);
  } else if (rt->vulkan.timestamp_period_ns <= 0.0f) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_PERIOD);
  } else {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_UNSUPPORTED);
  }
  reset_last_runtime_timing_decomposition(rt);
  rt->px16_m8_last_executed_explicit_variant_request = selector_controls_dispatch_variant == 0u ? requested_variant : 0u;
  if (!prom_vk_checked_mul_u32(m, k, &work_units_u32) || !prom_vk_checked_mul_u32(k, n, &work_units_u32) ||
      !prom_vk_checked_mul_u32(m, n, &work_units_u32) || !prom_vk_checked_mul_u32(m, n, &mn_product) ||
      !prom_vk_checked_mul_u32(mn_product, k, &work_units_u32)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_DETAIL_SIZE_OVERFLOW);
    return PROM_ERROR;
  }
  pre_dispatch_begin_ns = prom_wall_clock_now_ns();
  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, 0);
  if (rt->async_state == PROM_ASYNC_STATE_CONSUMED) {
    set_async_state(rt, PROM_ASYNC_STATE_IDLE, PROM_STAGE_NONE, 0);
  }
  if (rt->async_state == PROM_ASYNC_STATE_FAILED) {
    prom_vk_set_status(out_stage,
               out_detail_code,
               PROM_STAGE_SUBMIT,
               rt->async_failure_detail != 0 ? rt->async_failure_detail : PROM_DETAIL_ASYNC_FAILED);
    return PROM_ERROR;
  }
  if (rt->async_state == PROM_ASYNC_STATE_SUBMITTED || rt->async_state == PROM_ASYNC_STATE_READY) {
    rt->slot_diag.inflight_rejection_count += 1u;
    commit_slot_runtime_diag_snapshot(rt, PROM_DETAIL_ASYNC_UNCONSUMED);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_ASYNC_UNCONSUMED);
    stage_commit_async_snapshot(rt, PROM_DOM_EVENT_ASYNC_UNCONSUMED_REJECTED, PROM_DETAIL_ASYNC_UNCONSUMED);
    return PROM_ERROR;
  }
  if (rt->in_flight_submit != 0u) {
    vk_result = vkGetFenceStatus(rt->vulkan.device, rt->submit_fence);
    if (vk_result == VK_SUCCESS) {
      rt->in_flight_submit = 0u;
    } else {
      rt->slot_diag.inflight_rejection_count += 1u;
      commit_slot_runtime_diag_snapshot(rt, PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED);
      return PROM_ERROR;
    }
  }

  if ((rt->vulkan.test_flags & PROM_TESTCFG_FAIL_UPLOAD) != 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_DETAIL_INJECTED_UPLOAD_FAILURE);
    return PROM_ERROR;
  }

  can_stage = rt->vulkan.has_device_local_memory;
  can_direct = rt->vulkan.has_host_visible_memory;
  if ((rt->vulkan.test_flags & PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY) != 0u) {
    can_stage = 0u;
  }
  readback_required = ((rt->vulkan.test_flags & PROM_TESTCFG_FORCE_UPLOAD_ONLY) == 0u) ? 1u : 0u;
  work_units = (uint64_t)work_units_u32;
  policy_mode = prom_sgemm_controller_step(&rt->sgemm_controller, m, n, k, work_units, rt->vulkan.software_vulkan);
  packed4_waste_permille = prom_packed4_padding_waste_permille(m, n, k);
  packed4_budget_permille = prom_packed4_mode_budget_permille(policy_mode);
  packed4_small_shape = (m < 4u || n < 4u || k < 4u) ? 1u : 0u;
  packed4_tail_count = prom_packed4_tail_count(m, n, k);
  packed4_padded_lane_count = (uint32_t)((prom_round_up4_u32(k) - k) * (m + n));
  fp16_has_special_values = 0u;
  fp16_utility_score = -1000;
  prom_fp16_prepare_production_tolerance_facts(a, b, m, n, k, &rt->sgemm_controller, &fp16_has_special_values, &fp16_utility_score);
  rt->px16_m17_last_tolerance_eval_wall_ns = 0u;
  rt->px16_m17_last_tolerance_eval_in_dispatch = 0u;
  rt->px16_m17_last_tolerance_eval_source = 1u;
  if ((rt->vulkan.test_flags & PROM_TESTCFG_FORCE_FP16_UTILITY_WIN) != 0u) {
    fp16_utility_score = 1201;
  }
  tiled_shape = (work_units >= (uint64_t)PROM_JUDGMENT_TILED_WORK_THRESHOLD && m >= PROM_VK_LOCAL_SIZE_X &&
                 n >= PROM_VK_LOCAL_SIZE_Y && k >= PROM_VK_TILE_K)
                    ? 1u
                    : 0u;
  memset(&occupancy_facts, 0, sizeof(occupancy_facts));
  occupancy_facts.register_file_class = rt->vulkan.occupancy_register_file_class;
  occupancy_facts.shared_memory_class = rt->vulkan.occupancy_shared_memory_class;
  occupancy_facts.memory_bandwidth_class = rt->vulkan.occupancy_memory_bandwidth_class;
  occupancy_facts.fp32_throughput_class = rt->vulkan.occupancy_fp32_throughput_class;
  occupancy_facts.max_workgroup_class = rt->vulkan.occupancy_max_workgroup_class;
  occupancy_facts.queue_capability_class = rt->vulkan.occupancy_queue_capability_class;
  occupancy_facts.has_exact_profile = rt->vulkan.occupancy_has_exact_profile;
  occupancy_facts.manual_override_enabled = 0u;
  occupancy_facts.manual_override_variant = 0u;
  occupancy_facts.m = m;
  occupancy_facts.n = n;
  occupancy_facts.k = k;
  occupancy_facts.work_units = work_units;
  prom_judgment_engine_select_occupancy_variant(&occupancy_facts, &occupancy_decision);
  (void)prom_dominatus_predictor_advance_reservations(&rt->p15_predictor_state, rt->p14_measurement_tick);
  rt->p15_feedforward_dispatch_state.valid = 1u;
  rt->p15_feedforward_dispatch_state.enabled = rt->p15_shadow_canary_params.enabled != 0u ? 1u : 0u;
  rt->p15_feedforward_dispatch_state.used = 0u;
  rt->p15_feedforward_dispatch_state.source = 0u;
  rt->p15_feedforward_dispatch_state.reservation_present = 0u;
  rt->p15_feedforward_dispatch_state.reservation_matured = 0u;
  rt->p15_feedforward_dispatch_state.block_reason = PROM_P15_SHADOW_FEEDFORWARD_BLOCK_NONE;
  rt->p15_feedforward_dispatch_state.reserved_variant_id = 0u;
  rt->p15_feedforward_dispatch_state.selected_variant_id = occupancy_decision.selected_variant;
  rt->p15_feedforward_dispatch_state.reconciliation_match = 0u;
  rt->p15_feedforward_dispatch_state.correction_action = PROM_DOM_CORRECTION_ACTION_NONE;
  rt->p15_feedforward_dispatch_state.reservation_consumed = 0u;
  rt->p15_feedforward_dispatch_state.reservation_stale_or_expired = 0u;
  rt->p15_feedforward_dispatch_state.confidence_before = rt->p15_predictor_state.prediction_confidence;
  rt->p15_feedforward_dispatch_state.confidence_after = rt->p15_predictor_state.prediction_confidence;
  if (rt->p15_feedforward_dispatch_state.enabled == 0u) {
    rt->p15_feedforward_dispatch_state.block_reason = PROM_P15_SHADOW_FEEDFORWARD_BLOCK_DISABLED;
  } else if (rt->p15_shadow_authority_gate.state != PROM_SHADOW_AUTHORITY_HEALTHY) {
    rt->p15_feedforward_dispatch_state.block_reason = PROM_P15_SHADOW_FEEDFORWARD_BLOCK_NOT_HEALTHY;
    rt->p15_feedforward_dispatch_state.reason_binding_block_count += 1u;
  } else if (occupancy_decision.fallback_used != 0u) {
    rt->p15_feedforward_dispatch_state.block_reason = PROM_P15_SHADOW_FEEDFORWARD_BLOCK_FALLBACK_REQUIRED;
    rt->p15_feedforward_dispatch_state.fallback_to_judgment_count += 1u;
  } else if (rt->p15_shadow_canary_state.healthy_margin_passed == 0u) {
    rt->p15_feedforward_dispatch_state.block_reason = PROM_P15_SHADOW_FEEDFORWARD_BLOCK_MARGIN_FAILED;
    rt->p15_feedforward_dispatch_state.margin_block_count += 1u;
  } else if (rt->p15_shadow_canary_state.reason_binding_passed == 0u) {
    rt->p15_feedforward_dispatch_state.block_reason = PROM_P15_SHADOW_FEEDFORWARD_BLOCK_REASON_BINDING;
    rt->p15_feedforward_dispatch_state.reason_binding_block_count += 1u;
  } else {
    const prom_p15_feedforward_reservation_probe probe =
        prom_p15_probe_feedforward_reservation(&rt->p15_predictor_state.reservations,
                                               occupancy_decision.shape_class,
                                               occupancy_decision.selected_variant);
    rt->p15_feedforward_dispatch_state.reservation_present = probe.present;
    rt->p15_feedforward_dispatch_state.reservation_matured = probe.matured;
    if (probe.exact_match != NULL &&
        prom_occ_variant_is_wired_evt_dispatchable(probe.exact_match->variant_id) == 0u) {
      rt->p15_feedforward_dispatch_state.block_reason = PROM_P15_SHADOW_FEEDFORWARD_BLOCK_CAPABILITY_MISMATCH;
      rt->p15_feedforward_dispatch_state.reserved_variant_id = probe.exact_match->variant_id;
      rt->p15_feedforward_dispatch_state.fallback_to_judgment_count += 1u;
    } else if (probe.exact_match != NULL) {
      prom_dominatus_reservation_decision consume =
          prom_dominatus_reservation_consume_matured(&rt->p15_predictor_state.reservations,
                                                     occupancy_decision.shape_class,
                                                     occupancy_decision.selected_variant);
      if (consume.valid != 0u && consume.yielded != 0u) {
        rt->p15_feedforward_dispatch_state.used = 1u;
        rt->p15_feedforward_dispatch_state.source = 1u;
        rt->p15_feedforward_dispatch_state.reserved_variant_id = probe.exact_match->variant_id;
        rt->p15_feedforward_dispatch_state.reconciliation_match = 1u;
        rt->p15_feedforward_dispatch_state.reservation_consumed = 1u;
        rt->p15_feedforward_dispatch_state.reservation_consumed_count += 1u;
        rt->p15_last_reservation = consume;
      }
    } else if (probe.variant_mismatch != NULL) {
      const prom_dominatus_reservation_decision correction =
          prom_dominatus_predictor_apply_reconciliation_to_reservation(&rt->p15_predictor_state,
                                                                       &rt->p15_predictor_state.reservations,
                                                                       probe.variant_mismatch->request_id,
                                                                       PROM_DOM_CORRECTION_ACTION_LOWER_CONFIDENCE,
                                                                       PROM_P15_SHADOW_FEEDFORWARD_BLOCK_VARIANT_MISMATCH,
                                                                       rt->p14_measurement_tick);
      rt->p15_feedforward_dispatch_state.block_reason = PROM_P15_SHADOW_FEEDFORWARD_BLOCK_VARIANT_MISMATCH;
      rt->p15_feedforward_dispatch_state.reserved_variant_id = probe.variant_mismatch->variant_id;
      rt->p15_feedforward_dispatch_state.correction_action = PROM_DOM_CORRECTION_ACTION_LOWER_CONFIDENCE;
      rt->p15_feedforward_dispatch_state.fallback_to_judgment_count += 1u;
      rt->p15_feedforward_dispatch_state.variant_mismatch_count += 1u;
      if (correction.valid != 0u) {
        rt->p15_last_reservation = correction;
        if (correction.expired != 0u || correction.cancelled != 0u) {
          rt->p15_feedforward_dispatch_state.reservation_stale_or_expired = 1u;
          rt->p15_feedforward_dispatch_state.stale_reservation_count += 1u;
        }
      }
      rt->p15_feedforward_dispatch_state.confidence_after = rt->p15_predictor_state.prediction_confidence;
    } else if (probe.shape_mismatch != NULL) {
      const prom_dominatus_reservation_decision correction =
          prom_dominatus_predictor_apply_reconciliation_to_reservation(&rt->p15_predictor_state,
                                                                       &rt->p15_predictor_state.reservations,
                                                                       probe.shape_mismatch->request_id,
                                                                       PROM_DOM_CORRECTION_ACTION_MARK_STALE,
                                                                       PROM_P15_SHADOW_FEEDFORWARD_BLOCK_SHAPE_MISMATCH,
                                                                       rt->p14_measurement_tick);
      rt->p15_feedforward_dispatch_state.block_reason = PROM_P15_SHADOW_FEEDFORWARD_BLOCK_SHAPE_MISMATCH;
      rt->p15_feedforward_dispatch_state.reserved_variant_id = probe.shape_mismatch->variant_id;
      rt->p15_feedforward_dispatch_state.correction_action = PROM_DOM_CORRECTION_ACTION_MARK_STALE;
      rt->p15_feedforward_dispatch_state.fallback_to_judgment_count += 1u;
      rt->p15_feedforward_dispatch_state.shape_mismatch_count += 1u;
      if (correction.valid != 0u) {
        rt->p15_last_reservation = correction;
        if (correction.expired != 0u || correction.cancelled != 0u) {
          rt->p15_feedforward_dispatch_state.reservation_stale_or_expired = 1u;
          rt->p15_feedforward_dispatch_state.stale_reservation_count += 1u;
        }
      }
      rt->p15_feedforward_dispatch_state.confidence_after = rt->p15_predictor_state.prediction_confidence;
    } else if (probe.stale != NULL) {
      rt->p15_feedforward_dispatch_state.block_reason = PROM_P15_SHADOW_FEEDFORWARD_BLOCK_STALE_RESERVATION;
      rt->p15_feedforward_dispatch_state.reserved_variant_id = probe.stale->variant_id;
      rt->p15_feedforward_dispatch_state.reservation_stale_or_expired = 1u;
      rt->p15_feedforward_dispatch_state.fallback_to_judgment_count += 1u;
      rt->p15_feedforward_dispatch_state.stale_reservation_count += 1u;
    } else if (probe.cancelled != NULL) {
      rt->p15_feedforward_dispatch_state.block_reason = PROM_P15_SHADOW_FEEDFORWARD_BLOCK_CANCELLED_RESERVATION;
      rt->p15_feedforward_dispatch_state.reserved_variant_id = probe.cancelled->variant_id;
      rt->p15_feedforward_dispatch_state.reservation_stale_or_expired = 1u;
      rt->p15_feedforward_dispatch_state.fallback_to_judgment_count += 1u;
    } else if (probe.consumed != NULL) {
      rt->p15_feedforward_dispatch_state.block_reason = PROM_P15_SHADOW_FEEDFORWARD_BLOCK_ALREADY_CONSUMED;
      rt->p15_feedforward_dispatch_state.reserved_variant_id = probe.consumed->variant_id;
      rt->p15_feedforward_dispatch_state.fallback_to_judgment_count += 1u;
      rt->p15_feedforward_dispatch_state.dedup_block_count += 1u;
    } else if (probe.pending != NULL) {
      rt->p15_feedforward_dispatch_state.block_reason = PROM_P15_SHADOW_FEEDFORWARD_BLOCK_RESERVATION_NOT_READY;
      rt->p15_feedforward_dispatch_state.reserved_variant_id = probe.pending->variant_id;
      rt->p15_feedforward_dispatch_state.fallback_to_judgment_count += 1u;
      rt->p15_feedforward_dispatch_state.no_matured_reservation_count += 1u;
    } else {
      rt->p15_feedforward_dispatch_state.block_reason = PROM_P15_SHADOW_FEEDFORWARD_BLOCK_NO_MATURED_RESERVATION;
      rt->p15_feedforward_dispatch_state.fallback_to_judgment_count += 1u;
      rt->p15_feedforward_dispatch_state.no_matured_reservation_count += 1u;
    }
  }
  rt->slot_diag.p13_m2_occupancy_device_band = occupancy_decision.device_band;
  rt->slot_diag.p13_m2_occupancy_shape_class = occupancy_decision.shape_class;
  rt->slot_diag.p13_m2_occupancy_selected_variant = occupancy_decision.selected_variant;
  rt->slot_diag.p13_m2_occupancy_unclamped_variant = occupancy_decision.unclamped_variant;
  rt->slot_diag.p13_m2_occupancy_clamp_reason = occupancy_decision.clamp_reason;
  rt->slot_diag.p13_m2_occupancy_override_used = occupancy_decision.override_used;
  rt->slot_diag.p13_m2_occupancy_fallback_used = occupancy_decision.fallback_used;
  if (selector_controls_dispatch_variant != 0u) {
    requested_variant = occupancy_decision.selected_variant;
  }
  memset(&path_compute_facts, 0, sizeof(path_compute_facts));
  path_compute_facts.m = m;
  path_compute_facts.n = n;
  path_compute_facts.k = k;
  path_compute_facts.work_units = work_units;
  path_compute_facts.can_stage = can_stage;
  path_compute_facts.can_direct = can_direct;
  path_compute_facts.allow_fallback = ((rt->vulkan.test_flags & PROM_TESTCFG_DISABLE_STAGING_FALLBACK) == 0u) ? 1u : 0u;
  path_compute_facts.readback_required = readback_required;
  path_compute_facts.force_direct = ((rt->vulkan.test_flags & PROM_TESTCFG_FORCE_DIRECT_PATH) != 0u) ? 1u : 0u;
  path_compute_facts.force_direct_reason =
      path_compute_facts.force_direct != 0u ? PROM_SGEMM_FORCE_DIRECT_REASON_EXPLICIT_OVERRIDE : PROM_SGEMM_FORCE_DIRECT_REASON_NONE;
  path_compute_facts.force_staged = ((rt->vulkan.test_flags & PROM_TESTCFG_FORCE_STAGED_PATH) != 0u) ? 1u : 0u;
  path_compute_facts.force_tiled = ((rt->vulkan.test_flags & PROM_TESTCFG_FORCE_TILED_PATH) != 0u) ? 1u : 0u;
  path_compute_facts.tiled_shape = tiled_shape;
  path_compute_facts.software_vulkan = rt->vulkan.software_vulkan;
  path_compute_facts.policy_mode = (uint32_t)policy_mode;
  if (prom_dom_sgemm_stage_path_compute_facts(&rt->blackboard, &path_compute_facts) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  if (prom_dom_sgemm_build_path_compute_facts_from_visible(&rt->blackboard, &path_compute_facts, &path_compute_projection) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  memset(&judgment_facts, 0, sizeof(judgment_facts));
  judgment_facts.m = path_compute_projection.facts.m;
  judgment_facts.n = path_compute_projection.facts.n;
  judgment_facts.k = path_compute_projection.facts.k;
  judgment_facts.work_units = path_compute_projection.facts.work_units;
  judgment_facts.can_stage = path_compute_projection.facts.can_stage;
  judgment_facts.can_direct = path_compute_projection.facts.can_direct;
  judgment_facts.allow_fallback = path_compute_projection.facts.allow_fallback;
  judgment_facts.readback_required = path_compute_projection.facts.readback_required;
  judgment_facts.force_direct = path_compute_projection.facts.force_direct;
  judgment_facts.force_staged = path_compute_projection.facts.force_staged;
  judgment_facts.force_tiled = path_compute_projection.facts.force_tiled;
  judgment_facts.tiled_shape = path_compute_projection.facts.tiled_shape;
  judgment_facts.software_vulkan = path_compute_projection.facts.software_vulkan;
  judgment_facts.policy_mode = (prom_policy_mode)path_compute_projection.facts.policy_mode;
  memset(&layout_precision_facts, 0, sizeof(layout_precision_facts));
  layout_precision_facts.packed4_available = 1u;
  layout_precision_facts.packed4_small_shape = packed4_small_shape;
  layout_precision_facts.packed4_padding_waste_permille = packed4_waste_permille;
  layout_precision_facts.packed4_mode_budget_permille = packed4_budget_permille;
  layout_precision_facts.packed4_row_major_valid = 1u;
  layout_precision_facts.packed4_tail_valid = 1u;
  layout_precision_facts.strict_fp32 = ((rt->vulkan.test_flags & PROM_TESTCFG_FORCE_STRICT_FP32) != 0u) ? 1u : 0u;
  layout_precision_facts.tolerance_known = rt->sgemm_controller.fp16_tolerance_known;
  layout_precision_facts.tolerance_pass = rt->sgemm_controller.fp16_tolerance_pass;
  layout_precision_facts.has_special_values = fp16_has_special_values;
  layout_precision_facts.capability_fp16_storage = rt->vulkan.capability_fp16_storage;
  layout_precision_facts.fallback_available = (path_compute_projection.facts.allow_fallback != 0u && path_compute_projection.facts.can_direct != 0u) ? 1u : 0u;
  layout_precision_facts.fp16_utility_score = fp16_utility_score;
  if (prom_dom_sgemm_stage_layout_precision_facts(&rt->blackboard, &layout_precision_facts) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  if (prom_dom_sgemm_build_layout_precision_facts_from_visible(&rt->blackboard, &layout_precision_facts, &layout_precision_projection) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  judgment_facts.packed4_available = layout_precision_projection.facts.packed4_available;
  judgment_facts.packed4_small_shape = layout_precision_projection.facts.packed4_small_shape;
  judgment_facts.packed4_padding_waste_permille = layout_precision_projection.facts.packed4_padding_waste_permille;
  judgment_facts.packed4_mode_budget_permille = layout_precision_projection.facts.packed4_mode_budget_permille;
  judgment_facts.packed4_row_major_valid = layout_precision_projection.facts.packed4_row_major_valid;
  judgment_facts.packed4_tail_valid = layout_precision_projection.facts.packed4_tail_valid;
  judgment_facts.strict_fp32 = layout_precision_projection.facts.strict_fp32;
  judgment_facts.tolerance_known = layout_precision_projection.facts.tolerance_known;
  judgment_facts.tolerance_pass = layout_precision_projection.facts.tolerance_pass;
  judgment_facts.has_special_values = layout_precision_projection.facts.has_special_values;
  judgment_facts.capability_fp16_storage = layout_precision_projection.facts.capability_fp16_storage;
  judgment_facts.fallback_available = layout_precision_projection.facts.fallback_available;
  judgment_facts.fp16_utility_score = layout_precision_projection.facts.fp16_utility_score;
  memset(&transfer_queue_facts, 0, sizeof(transfer_queue_facts));
  transfer_queue_facts.dedicated_transfer_available = rt->vulkan.dedicated_transfer_available;
  transfer_queue_facts.transfer_queue_family_index = rt->vulkan.transfer_queue_family_index;
  transfer_queue_facts.compute_queue_family_index = rt->vulkan.queue_family_index;
  transfer_queue_facts.queue_families_differ =
      (rt->vulkan.dedicated_transfer_available != 0u && rt->vulkan.transfer_queue_family_index != rt->vulkan.queue_family_index) ? 1u : 0u;
  transfer_queue_facts.transfer_queue_supported = rt->vulkan.transfer_queue_enabled;
  transfer_queue_facts.transfer_queue_disabled_by_config = ((rt->vulkan.test_flags & PROM_TESTCFG_DISABLE_TRANSFER_QUEUE) != 0u) ? 1u : 0u;
  transfer_queue_facts.transfer_workload_large_enough = work_units >= (uint64_t)PROM_JUDGMENT_STAGING_WORK_THRESHOLD ? 1u : 0u;
  transfer_queue_facts.transfer_sync_ownership_supported = rt->vulkan.transfer_queue_enabled;
  transfer_queue_facts.transfer_fallback_available = 1u;
  transfer_queue_facts.upload_only_policy_eligible = readback_required == 0u ? 1u : 0u;
  transfer_queue_facts.upload_readback_supported = 0u;
  if (prom_dom_sgemm_stage_transfer_queue_facts(&rt->blackboard, &transfer_queue_facts) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  if (prom_dom_sgemm_build_transfer_queue_facts_from_visible(&rt->blackboard, &transfer_queue_facts, &transfer_queue_projection) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  judgment_facts.transfer_queue_dedicated_available = transfer_queue_projection.facts.dedicated_transfer_available;
  judgment_facts.transfer_queue_families_differ = transfer_queue_projection.facts.queue_families_differ;
  judgment_facts.transfer_queue_supported = transfer_queue_projection.facts.transfer_queue_supported;
  judgment_facts.transfer_overlap_slot_valid = transfer_queue_projection.facts.transfer_sync_ownership_supported;
  judgment_facts.transfer_workload_large_enough = transfer_queue_projection.facts.transfer_workload_large_enough;
  judgment_facts.transfer_fallback_available = transfer_queue_projection.facts.transfer_fallback_available;
  judgment_facts.transfer_queue_disabled_by_config = transfer_queue_projection.facts.transfer_queue_disabled_by_config;
  layout_precision_path_guard_dirty_mask = 0u;
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_SHAPE_M));
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_SHAPE_N));
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_SHAPE_K));
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_WORK_UNITS));
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_CAN_DIRECT));
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_ALLOW_FALLBACK));
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_TILED_SHAPE));
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_POLICY_MODE));
  layout_precision_dependency_dirty_mask =
      layout_precision_projection.dependent_dirty_key_mask_last_commit | layout_precision_path_guard_dirty_mask;

  memset(&layout_precision_selector_decision, 0, sizeof(layout_precision_selector_decision));
  rt->layout_precision_selector_cache.last_dirty_dependency_mask = layout_precision_dependency_dirty_mask;
  rt->layout_precision_selector_cache.dependency_mask = layout_precision_dependency_dirty_mask;
  rt->layout_precision_selector_cache.last_decision_reused = 0u;
  if (selector_cache_enabled(rt) != 0u && layout_precision_projection.from_visible_snapshot != 0u &&
      rt->layout_precision_selector_cache.valid != 0u && layout_precision_dependency_dirty_mask == 0u &&
      rt->layout_precision_selector_cache.layout_precision_invalidation_count_when_computed == rt->slot_diag.m14_layout_precision_invalidation_count) {
    layout_precision_selector_decision = rt->layout_precision_selector_cache.decision;
    rt->layout_precision_selector_cache.reuse_count += 1u;
    rt->layout_precision_selector_cache.last_decision_reused = 1u;
  } else {
    if (rt->layout_precision_selector_cache.valid != 0u &&
        (layout_precision_dependency_dirty_mask != 0u ||
         rt->layout_precision_selector_cache.layout_precision_invalidation_count_when_computed != rt->slot_diag.m14_layout_precision_invalidation_count)) {
      rt->layout_precision_selector_cache.invalidation_count += 1u;
      rt->layout_precision_selector_cache.valid = 0u;
    }
    prom_judgment_engine_select_layout_precision(&judgment_facts, &layout_precision_selector_decision);
    rt->layout_precision_selector_cache.valid = 1u;
    rt->layout_precision_selector_cache.visible_generation_when_computed = layout_precision_projection.visible_generation;
    rt->layout_precision_selector_cache.layout_precision_invalidation_count_when_computed =
        rt->slot_diag.m14_layout_precision_invalidation_count;
    rt->layout_precision_selector_cache.decision = layout_precision_selector_decision;
    rt->layout_precision_selector_cache.recompute_count += 1u;
  }
  prom_judgment_engine_select_sgemm_mode_with_layout_precision(&judgment_facts, &layout_precision_selector_decision, &judgment_decision);
  if (judgment_decision.success == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, judgment_decision.error_detail);
    return PROM_ERROR;
  }
  memset(&path_compute_decision, 0, sizeof(path_compute_decision));
  path_compute_decision.success = judgment_decision.success;
  path_compute_decision.error_detail = judgment_decision.error_detail;
  path_compute_decision.requested_path = (uint32_t)judgment_decision.requested_path;
  path_compute_decision.selected_path = (uint32_t)judgment_decision.selected_path;
  path_compute_decision.compute_mode = (uint32_t)judgment_decision.compute_mode;
  path_compute_decision.final_detail = judgment_decision.final_detail;
  path_compute_decision.used_fallback_to_direct = judgment_decision.used_fallback_to_direct;
  path_compute_decision.winning_candidate_index = judgment_decision.winning_candidate_index;
  path_compute_decision.winning_score = judgment_decision.winning_score;
  if (prom_dom_sgemm_stage_path_compute_decision(&rt->blackboard, &path_compute_decision) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  if (prom_dom_sgemm_read_visible_path_compute_diagnostics(&rt->blackboard, &path_compute_snapshot) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  if (selector_controls_dispatch_variant == 0u &&
      requested_variant != PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR &&
      prom_occ_variant_is_wired_evt_dispatchable(requested_variant) != 0u) {
    path_compute_snapshot.decision.compute_mode = (uint32_t)PROM_VK_COMPUTE_TILED;
  }
  judgment_decision.success = path_compute_snapshot.decision.success;
  judgment_decision.error_detail = path_compute_snapshot.decision.error_detail;
  judgment_decision.requested_path = (prom_vk_path_mode)path_compute_snapshot.decision.requested_path;
  judgment_decision.selected_path = (prom_vk_path_mode)path_compute_snapshot.decision.selected_path;
  judgment_decision.compute_mode = (prom_vk_compute_mode)path_compute_snapshot.decision.compute_mode;
  judgment_decision.final_detail = path_compute_snapshot.decision.final_detail;
  judgment_decision.used_fallback_to_direct = path_compute_snapshot.decision.used_fallback_to_direct;
  judgment_decision.winning_candidate_index = path_compute_snapshot.decision.winning_candidate_index;
  judgment_decision.winning_score = path_compute_snapshot.decision.winning_score;
  selected_path = judgment_decision.selected_path;
  compute_mode = judgment_decision.compute_mode;
  final_detail = judgment_decision.final_detail;
  if (audit_override != NULL && audit_override->descriptor != NULL) {
    /* Audit-only mode selection changes packing after production policy has
       completed. It never feeds a candidate back into policy or registry state. */
    compute_mode = (prom_vk_compute_mode)audit_override->descriptor->compute_mode;
  }
  memset(&transfer_queue_decision, 0, sizeof(transfer_queue_decision));
  rt->transfer_selector_cache.last_dirty_dependency_mask = transfer_queue_projection.dependent_dirty_key_mask_last_commit;
  rt->transfer_selector_cache.dependency_mask = transfer_queue_projection.dependent_dirty_key_mask_last_commit;
  rt->transfer_selector_cache.last_decision_reused = 0u;
  if (selector_cache_enabled(rt) != 0u && transfer_queue_projection.from_visible_snapshot != 0u &&
      rt->transfer_selector_cache.valid != 0u && transfer_queue_projection.dependent_dirty_key_mask_last_commit == 0u &&
      rt->transfer_selector_cache.selected_path == (uint32_t)judgment_decision.selected_path) {
    transfer_queue_decision = rt->transfer_selector_cache.decision;
    rt->transfer_selector_cache.reuse_count += 1u;
    rt->transfer_selector_cache.last_decision_reused = 1u;
  } else {
    if (rt->transfer_selector_cache.valid != 0u && transfer_queue_projection.dependent_dirty_key_mask_last_commit != 0u) {
      rt->transfer_selector_cache.invalidation_count += 1u;
      rt->transfer_selector_cache.valid = 0u;
    }
    select_transfer_queue_policy(&judgment_decision, &transfer_queue_projection.facts, &transfer_queue_decision);
    rt->transfer_selector_cache.valid = 1u;
    rt->transfer_selector_cache.visible_generation_when_computed = transfer_queue_projection.visible_generation;
    rt->transfer_selector_cache.selected_path = (uint32_t)judgment_decision.selected_path;
    rt->transfer_selector_cache.decision = transfer_queue_decision;
    rt->transfer_selector_cache.recompute_count += 1u;
  }
  judgment_decision.use_dedicated_transfer_queue_upload = transfer_queue_decision.transfer_queue_used;
  judgment_decision.transfer_fallback_reason = transfer_queue_decision.transfer_fallback_reason;
  if (prom_dom_sgemm_stage_transfer_queue_decision(&rt->blackboard, &transfer_queue_decision) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  if (prom_dom_sgemm_read_visible_transfer_queue_diagnostics(&rt->blackboard, &transfer_queue_snapshot) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  use_dedicated_transfer_upload = transfer_queue_snapshot.transfer_queue_used;
  sync_transfer_diag_from_visible(rt);
  rt->sgemm_controller.packed4_tail_count_last = packed4_tail_count;
  rt->sgemm_controller.packed4_padded_lane_count_last = packed4_padded_lane_count;
  rt->sgemm_controller.packed4_padding_waste_permille_last = packed4_waste_permille;
  if (judgment_decision.packed4_reject_reason != PROM_PACKED4_REJECT_NONE) {
    prom_packed4_record_fallback(&rt->sgemm_controller, judgment_decision.packed4_reject_reason);
  }
  rt->sgemm_controller.fp16_fallback_reason_detail = prom_fp16_reject_reason_to_detail(judgment_decision.fp16_reject_reason);
  rt->sgemm_controller.fp16_selected_candidate = judgment_decision.fp16_selected != 0u ? 3u : 1u;
  memset(&layout_precision_decision, 0, sizeof(layout_precision_decision));
  layout_precision_decision.packed4_selected = layout_precision_selector_decision.packed4_selected;
  layout_precision_decision.packed4_reject_reason = (uint32_t)layout_precision_selector_decision.packed4_reject_reason;
  layout_precision_decision.fp16_selected = layout_precision_selector_decision.fp16_selected;
  layout_precision_decision.fp16_reject_reason = (uint32_t)layout_precision_selector_decision.fp16_reject_reason;
  layout_precision_decision.packed4_selected_layout_format = rt->sgemm_controller.packed4_selected_layout_format;
  layout_precision_decision.packed4_tail_count_last = rt->sgemm_controller.packed4_tail_count_last;
  layout_precision_decision.packed4_tail_count_total = rt->sgemm_controller.packed4_tail_count_total;
  layout_precision_decision.packed4_padded_lane_count_last = rt->sgemm_controller.packed4_padded_lane_count_last;
  layout_precision_decision.packed4_padded_lane_count_total = rt->sgemm_controller.packed4_padded_lane_count_total;
  layout_precision_decision.packed4_padding_waste_permille_last = rt->sgemm_controller.packed4_padding_waste_permille_last;
  layout_precision_decision.packed4_mode_budget_denials = rt->sgemm_controller.packed4_mode_budget_denials;
  layout_precision_decision.packed4_row_major_check_failures = rt->sgemm_controller.packed4_row_major_check_failures;
  layout_precision_decision.packed4_selection_count = rt->sgemm_controller.packed4_selection_count;
  layout_precision_decision.packed4_fallback_reason_padding_waste = rt->sgemm_controller.packed4_fallback_reason_padding_waste;
  layout_precision_decision.packed4_fallback_reason_small_shape = rt->sgemm_controller.packed4_fallback_reason_small_shape;
  layout_precision_decision.packed4_fallback_reason_capability_missing = rt->sgemm_controller.packed4_fallback_reason_capability_missing;
  layout_precision_decision.packed4_fallback_reason_fallback_required = rt->sgemm_controller.packed4_fallback_reason_fallback_required;
  layout_precision_decision.packed4_fallback_reason_mode_budget_denied = rt->sgemm_controller.packed4_fallback_reason_mode_budget_denied;
  layout_precision_decision.fp16_max_absolute_error = rt->sgemm_controller.fp16_max_absolute_error;
  layout_precision_decision.fp16_max_relative_error = rt->sgemm_controller.fp16_max_relative_error;
  layout_precision_decision.fp16_aggregate_error = rt->sgemm_controller.fp16_aggregate_error;
  layout_precision_decision.fp16_worst_case_element_index = rt->sgemm_controller.fp16_worst_case_element_index;
  layout_precision_decision.fp16_k_error_growth = rt->sgemm_controller.fp16_k_error_growth;
  layout_precision_decision.fp16_cancellation_risk = rt->sgemm_controller.fp16_cancellation_risk;
  layout_precision_decision.fp16_tolerance_known = rt->sgemm_controller.fp16_tolerance_known;
  layout_precision_decision.fp16_tolerance_pass = rt->sgemm_controller.fp16_tolerance_pass;
  layout_precision_decision.fp16_fallback_reason_detail = rt->sgemm_controller.fp16_fallback_reason_detail;
  layout_precision_decision.fp16_selected_candidate = rt->sgemm_controller.fp16_selected_candidate;
  if (prom_dom_sgemm_stage_layout_precision_decision(&rt->blackboard, &layout_precision_decision) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);

  compute_k = compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? prom_round_up4_u32(k) : k;
  if ((compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM &&
       (!checked_packed_fp16_buffer_size(m, compute_k, &a_buffer_size, &a_copy_size) ||
        !checked_packed_fp16_buffer_size(k, n, &b_buffer_size, &b_copy_size))) ||
      (compute_mode != PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM &&
       (!checked_float_buffer_size(m, compute_k, &a_buffer_size, &a_copy_size) ||
        !checked_float_buffer_size(compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? n : k,
                                   compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? compute_k : n,
                                   &b_buffer_size,
                                   &b_copy_size))) ||
      !checked_float_buffer_size(m, n, &c_buffer_size, &c_copy_size)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_DETAIL_SIZE_OVERFLOW);
    return PROM_ERROR;
  }
  artifact_layout_code = prom_slot_compute_layout_code(selected_path, compute_mode);
  artifact_precision_code = (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) ? 16u : 32u;
  artifact_a_key = make_artifact_key(PROM_BUFFER_ARTIFACT_A,
                                     m,
                                     n,
                                     k,
                                     compute_k,
                                     artifact_layout_code,
                                     artifact_precision_code,
                                     a_buffer_size);
  artifact_b_key = make_artifact_key(PROM_BUFFER_ARTIFACT_B,
                                     m,
                                     n,
                                     k,
                                     compute_k,
                                     artifact_layout_code,
                                     artifact_precision_code,
                                     b_buffer_size);
  artifact_c_key = make_artifact_key(PROM_BUFFER_ARTIFACT_C,
                                     m,
                                     n,
                                     k,
                                     compute_k,
                                     (uint32_t)PROM_VK_PATH_DIRECT,
                                     32u,
                                     c_buffer_size);
  required_capacity_bytes = (uint64_t)a_buffer_size + (uint64_t)b_buffer_size + (uint64_t)c_buffer_size;
  memset(&buffering_facts, 0, sizeof(buffering_facts));
  buffering_facts.required_fixed_slots_permille = 2000u;
  buffering_facts.required_pull_lag_peak_slots_permille = 1500u;
  buffering_facts.required_serial_slots_permille = 1000u;
  buffering_facts.fallback_available = judgment_facts.allow_fallback;
  if (judgment_facts.transfer_queue_dedicated_available != 0u && rt->vulkan.software_vulkan == 0u) {
    variance_class = PROM_VARIANCE_LOW;
  } else if (rt->vulkan.software_vulkan != 0u) {
    variance_class = PROM_VARIANCE_HIGH;
  } else {
    variance_class = PROM_VARIANCE_MODERATE;
  }
  if (policy_mode == PROM_POLICY_MODE_RECOVERY) {
    predictability_class = PROM_PREDICTABILITY_UNSTABLE;
  } else if (policy_mode == PROM_POLICY_MODE_SAFE) {
    predictability_class = PROM_PREDICTABILITY_TRACKED;
  } else {
    predictability_class = PROM_PREDICTABILITY_STABLE;
  }
  buffering_facts.transfer_variance_class = variance_class;
  buffering_facts.compute_predictability_class = predictability_class;
  buffering_facts.pull_lag_wip_waste_exceeded = rt->sgemm_controller.pending_waste_units > PROM_SGEMM_WASTE_BUDGET_UNITS ? 1u : 0u;
  buffering_facts.starvation_risk_high = rt->vulkan.software_vulkan != 0u && work_units > (uint64_t)PROM_JUDGMENT_STAGING_WORK_THRESHOLD ? 1u : 0u;
  if (can_stage != 0u && can_direct != 0u) {
    buffering_facts.memory_budget_slots_permille = 2200u;
  } else if (can_stage != 0u || can_direct != 0u) {
    buffering_facts.memory_budget_slots_permille = 1400u;
  } else {
    buffering_facts.memory_budget_slots_permille = 800u;
  }
  if (policy_mode == PROM_POLICY_MODE_SAFE && buffering_facts.memory_budget_slots_permille >= 200u) {
    buffering_facts.memory_budget_slots_permille -= 200u;
  } else if (policy_mode == PROM_POLICY_MODE_RECOVERY && buffering_facts.memory_budget_slots_permille >= 400u) {
    buffering_facts.memory_budget_slots_permille -= 400u;
  }
  buffering_facts.fixed_double_headroom_slots_permille =
      (int32_t)buffering_facts.memory_budget_slots_permille - (int32_t)buffering_facts.required_fixed_slots_permille;
  buffering_facts.pull_lag_headroom_slots_permille =
      (int32_t)buffering_facts.memory_budget_slots_permille - (int32_t)buffering_facts.required_pull_lag_peak_slots_permille;
  buffering_facts.serial_jit_headroom_slots_permille =
      (int32_t)buffering_facts.memory_budget_slots_permille - (int32_t)buffering_facts.required_serial_slots_permille;
  if (prom_dom_sgemm_stage_m35_facts(&rt->blackboard, &buffering_facts) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  if (prom_dom_sgemm_build_buffering_selector_facts_from_visible(&rt->blackboard, &buffering_facts, &buffering_projection) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  rt->m35_selector_cache.last_dirty_dependency_mask = buffering_projection.dependent_dirty_key_mask_last_commit;
  rt->m35_selector_cache.dependency_mask = buffering_projection.dependent_dirty_key_mask_last_commit;
  rt->m35_selector_cache.last_decision_reused = 0u;
  if (selector_cache_enabled(rt) != 0u && buffering_projection.from_visible_snapshot != 0u && rt->m35_selector_cache.valid != 0u &&
      buffering_projection.dependent_dirty_key_mask_last_commit == 0u) {
    buffering_decision = rt->m35_selector_cache.decision;
    rt->m35_selector_cache.reuse_count += 1u;
    rt->m35_selector_cache.last_decision_reused = 1u;
  } else {
    if (rt->m35_selector_cache.valid != 0u && buffering_projection.dependent_dirty_key_mask_last_commit != 0u) {
      rt->m35_selector_cache.invalidation_count += 1u;
      rt->m35_selector_cache.valid = 0u;
    }
    prom_judgment_engine_select_buffering_mode(&buffering_projection.facts, &buffering_decision);
    rt->m35_selector_cache.valid = 1u;
    rt->m35_selector_cache.visible_generation_when_computed = buffering_projection.visible_generation;
    rt->m35_selector_cache.decision = buffering_decision;
    rt->m35_selector_cache.no_feasible_mode_detail = (uint32_t)prom_buffering_reason_to_detail(buffering_decision.final_reason_code);
    rt->m35_selector_cache.recompute_count += 1u;
  }
  if (rt->slot_diag.m35_selected_mode != (uint32_t)buffering_decision.selected_mode) {
    rt->slot_diag.m35_transition_count += 1u;
  }
  if (prom_dom_sgemm_stage_m35_decision(&rt->blackboard, &buffering_decision, rt->m35_selector_cache.no_feasible_mode_detail) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  {
    prom_dom_sgemm_m35_snapshot m35_snapshot;
    if (prom_dom_sgemm_read_visible_m35(&rt->blackboard, &m35_snapshot) == 0u) {
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
      return PROM_ERROR;
    }
    rt->slot_diag.m35_selected_mode = m35_snapshot.selected_mode;
    rt->slot_diag.m35_fixed_feasible = m35_snapshot.fixed_feasible;
    rt->slot_diag.m35_pull_lag_feasible = m35_snapshot.pull_lag_feasible;
    rt->slot_diag.m35_serial_feasible = m35_snapshot.serial_feasible;
    rt->slot_diag.m35_fixed_rejected = m35_snapshot.fixed_rejected;
    rt->slot_diag.m35_pull_lag_rejected = m35_snapshot.pull_lag_rejected;
    rt->slot_diag.m35_serial_rejected = m35_snapshot.serial_rejected;
    rt->slot_diag.m35_fixed_score = m35_snapshot.fixed_score;
    rt->slot_diag.m35_pull_lag_score = m35_snapshot.pull_lag_score;
    rt->slot_diag.m35_serial_score = m35_snapshot.serial_score;
    rt->slot_diag.m35_reason_code = m35_snapshot.reason_code;
    rt->slot_diag.m35_final_reason_code = m35_snapshot.final_reason_code;
    rt->slot_diag.m35_fixed_double_rejection_reason = m35_snapshot.fixed_double_rejection_reason;
    rt->slot_diag.m35_pull_lag_rejection_reason = m35_snapshot.pull_lag_rejection_reason;
    rt->slot_diag.m35_serial_jit_rejection_reason = m35_snapshot.serial_jit_rejection_reason;
    rt->slot_diag.m35_memory_budget_slots_permille = m35_snapshot.memory_budget_slots_permille;
    rt->slot_diag.m35_required_fixed_slots_permille = m35_snapshot.required_fixed_slots_permille;
    rt->slot_diag.m35_required_pull_lag_slots_permille = m35_snapshot.required_pull_lag_peak_slots_permille;
    rt->slot_diag.m35_required_serial_slots_permille = m35_snapshot.required_serial_slots_permille;
    rt->slot_diag.m35_fixed_double_headroom_slots_permille = (int64_t)m35_snapshot.fixed_double_headroom_slots_permille;
    rt->slot_diag.m35_pull_lag_headroom_slots_permille = (int64_t)m35_snapshot.pull_lag_headroom_slots_permille;
    rt->slot_diag.m35_serial_jit_headroom_slots_permille = (int64_t)m35_snapshot.serial_jit_headroom_slots_permille;
  }
  if (buffering_decision.success == 0u) {
    rt->slot_diag.m35_rejection_count += 1u;
    rt->slot_diag.m35_budget_rejection_count += 1u;
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, prom_buffering_reason_to_detail(buffering_decision.reason_code));
    return PROM_ERROR;
  }
  buffering_mode = buffering_decision.selected_mode;
  if (buffering_mode == PROM_BUFFERING_MODE_PULL_LAG_PRESSURE) {
    rt->slot_diag.m35_pull_lag_predicted_demand_proxy_units += work_units;
    rt->slot_diag.m35_pull_lag_transfer_lead_proxy_units += work_units / 4u;
    rt->slot_diag.m35_pull_lag_safety_margin_proxy_units += work_units / 8u;
    rt->slot_diag.m35_pull_lag_stage_start_proxy_units += work_units / 16u;
    rt->slot_diag.m35_pull_lag_stage_complete_proxy_units += work_units / 16u + 1u;
    if (variance_class == PROM_VARIANCE_LOW) {
      rt->slot_diag.m35_pull_lag_early_stage_count += 1u;
      rt->slot_diag.m35_pull_lag_ready_unused_proxy_units += 1u;
    } else {
      rt->slot_diag.m35_pull_lag_late_stage_count += 1u;
      rt->slot_diag.m35_pull_lag_starvation_proxy_units += 1u;
    }
    if (buffering_facts.pull_lag_wip_waste_exceeded != 0u) {
      rt->slot_diag.m35_pull_lag_wip_waste_exceeded_count += 1u;
    }
  }
  if (buffering_mode == PROM_BUFFERING_MODE_SERIAL_JIT_SURVIVAL) {
    const uint32_t peer_slot_id = 1u;
    rt->slot_diag.m35_serial_sequential_step_count += 1u;
    rt->slot_diag.m35_serial_active_slot_count = 1u;
    if (!prom_slot_cleanup_to_empty(rt, &rt->slots[peer_slot_id])) {
      rt->slot_diag.m35_serial_busy_retry_count += 1u;
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED);
      return PROM_ERROR;
    }
    rt->slot_diag.m35_serial_failure_cleanup_count += 1u;
    rt->slot_diag.next_slot_id = 0u;
    rt->slot_diag.m35_serial_wip_depth = prom_slot_wip_depth(rt);
  }
  work_slot_id = rt->slot_diag.next_slot_id < 2u ? rt->slot_diag.next_slot_id : 0u;
  memset(&lease_facts, 0, sizeof(lease_facts));
  lease_facts.worker_id = 0u;
  lease_facts.slot_id = work_slot_id;
  lease_facts.entry_id = 0u;
  lease_facts.selected_recipe_variant = occupancy_decision.selected_variant;
  lease_facts.shape_class = occupancy_decision.shape_class;
  lease_facts.device_band = occupancy_decision.device_band;
  lease_facts.requested_resource_class = PROM_LEASE_RESOURCE_CLASS_COMPUTE;
  lease_facts.current_outstanding_depth = 0u;
  lease_facts.max_outstanding_depth = 1u;
  lease_facts.single_call_mode = 1u;
  lease_facts.lookahead_requested = rt->sgemm_controller.lookahead;
  lease_facts.lookahead_limit = prom_sgemm_default_config().lookahead_max;
  if (work_slot_id < 32u) {
    const uint32_t slot_mask = (1u << work_slot_id);
    /* Single-call path is explicitly preparing this slot for immediate dispatch.
     * Mark it ready/attention to avoid synthetic utility under-scoring. */
    lease_facts.ready_slot_mask = slot_mask;
    lease_facts.slot_attention_mask = slot_mask;
  }
  lease_facts.transfer_overlap_available = 1u;
  lease_facts.true_multi_queue_selected = 1u;
  prom_fill_lease_pressure_classes(rt,
                                   lease_facts.selected_recipe_variant,
                                   lease_facts.shape_class,
                                   lease_facts.device_band,
                                   work_units,
                                   &lease_facts);
  prepare_detail = prom_slot_prepare_for_call(rt,
                                              work_slot_id,
                                              m,
                                              n,
                                              compute_k,
                                              prom_slot_compute_layout_code(selected_path, compute_mode),
                                              (uint32_t)compute_mode,
                                              required_capacity_bytes);
  if (prepare_detail != 0) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, prepare_detail);
    return PROM_ERROR;
  }
  if (!prom_slot_swap_ready_to_current(rt, work_slot_id)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_SWAP_REJECTED);
    return PROM_ERROR;
  }

  upload_begin_ns = prom_wall_clock_now_ns();
  if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
    packed_a_upload = (float*)malloc(a_copy_size);
    packed_b_upload = (float*)malloc(b_copy_size);
    if (packed_a_upload == NULL || packed_b_upload == NULL) {
      free(packed_a_upload);
      free(packed_b_upload);
      prom_slot_mark_failure(rt, work_slot_id, PROM_ERROR);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
      return PROM_ERROR;
    }
    prom_pack_a_packed4_rowmajor(a, packed_a_upload, m, k, compute_k);
    prom_pack_b_packed4_colmajor(b, packed_b_upload, n, k, compute_k);
  } else if (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
    fp16_a_upload = (uint32_t*)malloc(a_copy_size);
    fp16_b_upload = (uint32_t*)malloc(b_copy_size);
    if (fp16_a_upload == NULL || fp16_b_upload == NULL) {
      free(packed_a_upload);
      free(packed_b_upload);
      free(fp16_a_upload);
      free(fp16_b_upload);
      prom_slot_mark_failure(rt, work_slot_id, PROM_ERROR);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
      return PROM_ERROR;
    }
    prom_pack_fp16_pairs(a, m * compute_k, fp16_a_upload);
    prom_pack_fp16_pairs(b, compute_k * n, fp16_b_upload);
  }
  memset(&async_facts, 0, sizeof(async_facts));
  async_facts.request_async = request_async;
  async_facts.in_flight = rt->in_flight_submit;
  async_facts.software_vulkan = rt->vulkan.software_vulkan;
  prom_judgment_engine_select_async_submission(&async_facts, &async_decision);
  if (async_decision.success == 0u) {
    free(packed_a_upload);
    free(packed_b_upload);
    free(fp16_a_upload);
    free(fp16_b_upload);
    prom_slot_mark_failure(rt, work_slot_id, async_decision.reject_detail);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, async_decision.reject_detail);
    return PROM_ERROR;
  }

  if (selected_path == PROM_VK_PATH_DIRECT) {
    if (!ensure_direct_execution_buffers(rt, &artifact_a_key, &artifact_b_key, &artifact_c_key, &vk_result)) {
      const int failure_detail = rt->arena_last_failure_detail != 0 ? rt->arena_last_failure_detail : (int)vk_result;
      prom_slot_mark_failure(rt, work_slot_id, failure_detail);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, failure_detail);
      free(packed_a_upload);
      free(packed_b_upload);
      free(fp16_a_upload);
      free(fp16_b_upload);
      return PROM_ERROR;
    }
    memcpy(rt->direct_a.mapped,
           compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? (const void*)packed_a_upload
           : (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM ? (const void*)fp16_a_upload : (const void*)a),
           a_copy_size);
    memcpy(rt->direct_b.mapped,
           compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? (const void*)packed_b_upload
           : (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM ? (const void*)fp16_b_upload : (const void*)b),
           b_copy_size);
    prom_sgemm_initialize_direct_output(rt->direct_c.mapped, c_copy_size);
    shader_a = &rt->direct_a;
    shader_b = &rt->direct_b;
    shader_c = &rt->direct_c;
  } else {
    if (!ensure_staged_execution_buffers(rt, &artifact_a_key, &artifact_b_key, &artifact_c_key, &vk_result)) {
      const int failure_detail = rt->arena_last_failure_detail != 0 ? rt->arena_last_failure_detail : (int)vk_result;
      prom_slot_mark_failure(rt, work_slot_id, failure_detail);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, failure_detail);
      free(packed_a_upload);
      free(packed_b_upload);
      free(fp16_a_upload);
      free(fp16_b_upload);
      return PROM_ERROR;
    }
    memcpy(rt->staged_upload_a.mapped,
           compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? (const void*)packed_a_upload
           : (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM ? (const void*)fp16_a_upload : (const void*)a),
           a_copy_size);
    memcpy(rt->staged_upload_b.mapped,
           compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? (const void*)packed_b_upload
           : (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM ? (const void*)fp16_b_upload : (const void*)b),
           b_copy_size);
    shader_a = &rt->staged_device_a;
    shader_b = &rt->staged_device_b;
    shader_c = &rt->staged_device_c;
  }
  free(packed_a_upload);
  free(packed_b_upload);
  free(fp16_a_upload);
  free(fp16_b_upload);
  upload_end_ns = prom_wall_clock_now_ns();
  rt->px16_m8_last_upload_wall_ns = prom_wall_clock_elapsed_ns(upload_begin_ns, upload_end_ns);

  if (compute_mode == PROM_VK_COMPUTE_TILED) {
    selected_pipeline = prom_shader_registry_pipeline_for_variant(
        rt->compute_pipeline_instances,
        sizeof(rt->compute_pipeline_instances) / sizeof(rt->compute_pipeline_instances[0]),
        requested_variant);
    if (selected_pipeline == VK_NULL_HANDLE) selected_pipeline = rt->tiled_pipeline;
    if (selected_path == PROM_VK_PATH_DIRECT) {
      final_detail = PROM_DETAIL_PATH_DIRECT_TILED;
    } else if (selected_path == PROM_VK_PATH_STAGED_UPLOAD) {
      final_detail = PROM_DETAIL_PATH_STAGED_UPLOAD_TILED;
    } else {
      final_detail = PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED;
    }
  } else if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
    selected_pipeline = rt->packed4_pipeline;
  } else if (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
    selected_pipeline = rt->fp16_pipeline;
  } else {
    selected_pipeline = rt->pipeline;
  }
  if (audit_override != NULL && audit_override->pipeline != VK_NULL_HANDLE) {
    selected_pipeline = audit_override->pipeline;
  }
  prom_sgemm_publish_final_dispatch_diagnostics(rt, requested_variant, (uint32_t)policy_mode, &path_compute_snapshot);

  memset(buffer_infos, 0, sizeof(buffer_infos));
  buffer_infos[0].buffer = shader_a->buffer;
  buffer_infos[0].offset = 0;
  buffer_infos[0].range = shader_a->size;
  buffer_infos[1].buffer = shader_b->buffer;
  buffer_infos[1].offset = 0;
  buffer_infos[1].range = shader_b->size;
  buffer_infos[2].buffer = shader_c->buffer;
  buffer_infos[2].offset = 0;
  buffer_infos[2].range = shader_c->size;

  memset(writes, 0, sizeof(writes));
  writes[0].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
  writes[0].dstSet = rt->descriptor_set;
  writes[0].dstBinding = 0u;
  writes[0].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  writes[0].descriptorCount = 1u;
  writes[0].pBufferInfo = &buffer_infos[0];
  writes[1] = writes[0];
  writes[1].dstBinding = 1u;
  writes[1].pBufferInfo = &buffer_infos[1];
  writes[2] = writes[0];
  writes[2].dstBinding = 2u;
  writes[2].pBufferInfo = &buffer_infos[2];
  command_record_begin_ns = prom_wall_clock_now_ns();
  rt->px16_m8_last_pre_dispatch_wall_ns =
      prom_wall_clock_elapsed_ns(pre_dispatch_begin_ns, command_record_begin_ns);
  vkUpdateDescriptorSets(rt->vulkan.device, 3u, writes, 0u, NULL);

  if (use_dedicated_transfer_upload != 0u && selected_path == PROM_VK_PATH_STAGED_UPLOAD) {
    stage_transfer_complete_telemetry(rt, 0u, work_slot_id, 0);
    vk_result = vkResetCommandBuffer(rt->transfer_command_buffer, 0u);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      stage_transfer_failure_telemetry(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    memset(&begin_info, 0, sizeof(begin_info));
    begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    vk_result = vkBeginCommandBuffer(rt->transfer_command_buffer, &begin_info);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      stage_transfer_failure_telemetry(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    memset(barriers, 0, sizeof(barriers));
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = rt->staged_upload_a.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->staged_upload_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->staged_upload_b.buffer;
    barriers[1].size = rt->staged_upload_b.size;
    vkCmdPipelineBarrier(rt->transfer_command_buffer, VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT, 0, 0, NULL, 2u, barriers,
                         0, NULL);
    memset(copies, 0, sizeof(copies));
    copies[0].size = rt->staged_upload_a.size;
    copies[1].size = rt->staged_upload_b.size;
    vkCmdCopyBuffer(rt->transfer_command_buffer, rt->staged_upload_a.buffer, rt->staged_device_a.buffer, 1u, &copies[0]);
    vkCmdCopyBuffer(rt->transfer_command_buffer, rt->staged_upload_b.buffer, rt->staged_device_b.buffer, 1u, &copies[1]);
    barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    barriers[0].dstAccessMask = 0u;
    barriers[0].srcQueueFamilyIndex = rt->vulkan.transfer_queue_family_index;
    barriers[0].dstQueueFamilyIndex = rt->vulkan.queue_family_index;
    barriers[0].buffer = rt->staged_device_a.buffer;
    barriers[0].size = rt->staged_device_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->staged_device_b.buffer;
    barriers[1].size = rt->staged_device_b.size;
    vkCmdPipelineBarrier(rt->transfer_command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT, 0, 0, NULL, 2u,
                         barriers, 0, NULL);
    vk_result = vkEndCommandBuffer(rt->transfer_command_buffer);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      stage_transfer_failure_telemetry(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    vk_result = vkResetFences(rt->vulkan.device, 1u, &rt->transfer_submit_fence);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      stage_transfer_failure_telemetry(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    memset(&transfer_submit_info, 0, sizeof(transfer_submit_info));
    transfer_submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
    transfer_submit_info.commandBufferCount = 1u;
    transfer_submit_info.pCommandBuffers = &rt->transfer_command_buffer;
    transfer_submit_info.signalSemaphoreCount = 1u;
    transfer_submit_info.pSignalSemaphores = &rt->transfer_ready_semaphore;
    if ((rt->vulkan.test_flags & PROM_TESTCFG_FAIL_TRANSFER_SUBMIT) != 0u) {
      prom_slot_mark_failure(rt, work_slot_id, VK_ERROR_DEVICE_LOST);
      stage_transfer_failure_telemetry(rt, work_slot_id, VK_ERROR_DEVICE_LOST);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, VK_ERROR_DEVICE_LOST);
      return PROM_ERROR;
    }
    vk_result = vkQueueSubmit(rt->vulkan.transfer_queue, 1u, &transfer_submit_info, rt->transfer_submit_fence);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      stage_transfer_failure_telemetry(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    stage_transfer_handoff_telemetry(rt, work_slot_id, 0, 2u);
    vk_result = vkResetCommandBuffer(rt->command_buffer, 0u);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    memset(&begin_info, 0, sizeof(begin_info));
    begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    vk_result = vkBeginCommandBuffer(rt->command_buffer, &begin_info);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    memset(barriers, 0, sizeof(barriers));
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = 0u;
    barriers[0].dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
    barriers[0].srcQueueFamilyIndex = rt->vulkan.transfer_queue_family_index;
    barriers[0].dstQueueFamilyIndex = rt->vulkan.queue_family_index;
    barriers[0].buffer = rt->staged_device_a.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->staged_device_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->staged_device_b.buffer;
    barriers[1].size = rt->staged_device_b.size;
    barriers[2] = barriers[0];
    barriers[2].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[2].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[2].srcAccessMask = 0u;
    barriers[2].dstAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[2].buffer = rt->staged_device_c.buffer;
    barriers[2].size = rt->staged_device_c.size;
    vkCmdPipelineBarrier(rt->command_buffer, VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0, 0, NULL, 3u, barriers,
                         0, NULL);
  } else {
    stage_transfer_complete_telemetry(rt, 1u, work_slot_id, 0);
    vk_result = vkResetCommandBuffer(rt->command_buffer, 0u);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }

    memset(&begin_info, 0, sizeof(begin_info));
    begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    vk_result = vkBeginCommandBuffer(rt->command_buffer, &begin_info);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }

    memset(barriers, 0, sizeof(barriers));
    if (selected_path == PROM_VK_PATH_DIRECT) {
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = rt->direct_a.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->direct_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->direct_b.buffer;
    barriers[1].size = rt->direct_b.size;
    barriers[2] = barriers[0];
    barriers[2].dstAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[2].buffer = rt->direct_c.buffer;
    barriers[2].size = rt->direct_c.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         0,
                         0,
                         NULL,
                         3u,
                         barriers,
                         0,
                         NULL);
    } else {
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = rt->staged_upload_a.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->staged_upload_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->staged_upload_b.buffer;
    barriers[1].size = rt->staged_upload_b.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         VK_PIPELINE_STAGE_TRANSFER_BIT,
                         0,
                         0,
                         NULL,
                         2u,
                         barriers,
                         0,
                         NULL);

    memset(copies, 0, sizeof(copies));
    copies[0].size = rt->staged_upload_a.size;
    copies[1].size = rt->staged_upload_b.size;
    vkCmdCopyBuffer(rt->command_buffer, rt->staged_upload_a.buffer, rt->staged_device_a.buffer, 1u, &copies[0]);
    vkCmdCopyBuffer(rt->command_buffer, rt->staged_upload_b.buffer, rt->staged_device_b.buffer, 1u, &copies[1]);

    barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
    barriers[0].buffer = rt->staged_device_a.buffer;
    barriers[0].size = rt->staged_device_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->staged_device_b.buffer;
    barriers[1].size = rt->staged_device_b.size;
    barriers[2] = barriers[0];
    barriers[2].dstAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[2].buffer = rt->staged_device_c.buffer;
    barriers[2].size = rt->staged_device_c.size;
    /* Staged device-local C is not pre-zeroed: current SGEMM kernels overwrite every final C element. */
      vkCmdPipelineBarrier(rt->command_buffer,
                           VK_PIPELINE_STAGE_TRANSFER_BIT,
                           VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           0,
                           0,
                           NULL,
                           3u,
                           barriers,
                           0,
                           NULL);
    }
  }

  vkCmdBindPipeline(rt->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, selected_pipeline);
  vkCmdBindDescriptorSets(rt->command_buffer,
                          VK_PIPELINE_BIND_POINT_COMPUTE,
                          rt->pipeline_layout,
                          0u,
                          1u,
                          &rt->descriptor_set,
                          0u,
                          NULL);

  push.m = m;
  push.n = n;
  push.k = compute_k;
  vkCmdPushConstants(rt->command_buffer,
                     rt->pipeline_layout,
                     VK_SHADER_STAGE_COMPUTE_BIT,
                     0u,
                     PROM_VK_SHADER_PUSH_BYTES,
                     &push);

  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, 0);
  if (work_slot_id < 32u) {
    const uint32_t slot_mask = (1u << work_slot_id);
    lease_facts.ready_slot_mask = slot_mask;
    lease_facts.slot_attention_mask = slot_mask;
  }
  lease_facts.failed_slot_mask = 0u;
  lease_facts.invalidated_slot_mask = 0u;
  lease_facts.unsafe_to_reuse = 0u;
  if (prom_runtime_request_resource_lease(rt, &lease_facts, &lease_decision) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_ERROR);
    return PROM_ERROR;
  }
  lease_granted = lease_decision.grant;
  if (lease_granted == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED);
    return PROM_ERROR;
  }
  lease_facts.lease_held = 1u;
  lease_facts.current_outstanding_depth = 1u;
  if ((rt->vulkan.test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_slot_mark_failure(rt, work_slot_id, PROM_DETAIL_INJECTED_DISPATCH_FAILURE);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_INJECTED_DISPATCH_FAILURE);
    return PROM_ERROR;
  }

  if (rt->timestamp_query_supported != 0u && rt->sgemm_timestamp_query_pool != VK_NULL_HANDLE) {
    vkCmdResetQueryPool(rt->command_buffer, rt->sgemm_timestamp_query_pool, 0u, 2u);
    vkCmdWriteTimestamp(rt->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, rt->sgemm_timestamp_query_pool, 0u);
  }

  /* Dispatch/indexing contract: x maps rows (m), y maps columns (n); host and shader must match this. */
  dispatch_geometry = audit_override != NULL && audit_override->descriptor != NULL
                          ? prom_sgemm_dispatch_geometry_for_metadata(m, n, &audit_override->descriptor->dispatch)
                          : prom_sgemm_dispatch_geometry_for_variant(requested_variant, m, n);
  vkCmdDispatch(rt->command_buffer,
                dispatch_geometry.groups_x,
                dispatch_geometry.groups_y,
                dispatch_geometry.groups_z);
  if (rt->timestamp_query_supported != 0u && rt->sgemm_timestamp_query_pool != VK_NULL_HANDLE) {
    vkCmdWriteTimestamp(rt->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, rt->sgemm_timestamp_query_pool, 1u);
  }

  if (selected_path == PROM_VK_PATH_DIRECT) {
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_HOST_READ_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = rt->direct_c.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->direct_c.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         0,
                         0,
                         NULL,
                         1u,
                         barriers,
                         0,
                         NULL);
  } else if (selected_path == PROM_VK_PATH_STAGED_UPLOAD_READBACK) {
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = rt->staged_device_c.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->staged_device_c.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         VK_PIPELINE_STAGE_TRANSFER_BIT,
                         0,
                         0,
                         NULL,
                         1u,
                         barriers,
                         0,
                         NULL);

    copies[2].size = rt->staged_readback_c.size;
    vkCmdCopyBuffer(rt->command_buffer, rt->staged_device_c.buffer, rt->staged_readback_c.buffer, 1u, &copies[2]);

    barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_HOST_READ_BIT;
    barriers[0].buffer = rt->staged_readback_c.buffer;
    barriers[0].size = rt->staged_readback_c.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_TRANSFER_BIT,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         0,
                         0,
                         NULL,
                         1u,
                         barriers,
                         0,
                         NULL);
  }

  if ((rt->vulkan.test_flags & PROM_TESTCFG_FAIL_COMMAND_END) != 0u) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_slot_mark_failure(rt, work_slot_id, VK_ERROR_DEVICE_LOST);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, VK_ERROR_DEVICE_LOST);
    return PROM_ERROR;
  }
  vk_result = vkEndCommandBuffer(rt->command_buffer);
  if (vk_result != VK_SUCCESS) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }
  rt->px16_m8_last_command_record_wall_ns =
      prom_wall_clock_elapsed_ns(command_record_begin_ns, prom_wall_clock_now_ns());

  if ((rt->vulkan.test_flags & PROM_TESTCFG_FAIL_RESET_FENCE) != 0u) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_slot_mark_failure(rt, work_slot_id, VK_ERROR_DEVICE_LOST);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, VK_ERROR_DEVICE_LOST);
    return PROM_ERROR;
  }
  vk_result = vkResetFences(rt->vulkan.device, 1u, &rt->submit_fence);
  if (vk_result != VK_SUCCESS) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }

  memset(&submit_info, 0, sizeof(submit_info));
  submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit_info.commandBufferCount = 1u;
  submit_info.pCommandBuffers = &rt->command_buffer;
  dispatch_submit_begin_ns = prom_wall_clock_now_ns();
  if (use_dedicated_transfer_upload != 0u) {
    wait_stage_mask = VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
    submit_info.waitSemaphoreCount = 1u;
    submit_info.pWaitSemaphores = &rt->transfer_ready_semaphore;
    submit_info.pWaitDstStageMask = &wait_stage_mask;
    stage_transfer_wait_telemetry(rt, work_slot_id, 0);
  }
  if ((rt->vulkan.test_flags & PROM_TESTCFG_FAIL_QUEUE_SUBMIT) != 0u) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_slot_mark_failure(rt, work_slot_id, VK_ERROR_DEVICE_LOST);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, VK_ERROR_DEVICE_LOST);
    return PROM_ERROR;
  }
  vk_result = vkQueueSubmit(rt->vulkan.compute_queue, 1u, &submit_info, rt->submit_fence);
  if (vk_result != VK_SUCCESS) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }
  dispatch_submit_end_ns = prom_wall_clock_now_ns();
  rt->px16_m8_last_dispatch_submit_wall_ns =
      prom_wall_clock_elapsed_ns(dispatch_submit_begin_ns, dispatch_submit_end_ns);
  rt->in_flight_submit = 1u;
  if (!prom_slot_mark_submitted(rt, work_slot_id)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
    prom_slot_mark_failure(rt, work_slot_id, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
    return PROM_ERROR;
  }

  if (async_decision.execute_async != 0u) {
    rt->async_task_id += 1;
    rt->async_m = m;
    rt->async_n = n;
    rt->async_k = k;
    rt->async_c_copy_size = c_copy_size;
    rt->async_selected_path = selected_path;
    rt->async_final_detail = final_detail;
    note_last_execution_shape(rt, m, n, k);
    set_async_state(rt, PROM_ASYNC_STATE_SUBMITTED, PROM_STAGE_SUBMIT, 0);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, final_detail);
    return PROM_OK;
  }

  if ((rt->vulkan.test_flags & PROM_TESTCFG_SKIP_SUBMIT_WAIT) == 0u) {
    sync_wait_begin_ns = prom_wall_clock_now_ns();
    if (use_dedicated_transfer_upload != 0u) {
      vk_result = vkWaitForFences(rt->vulkan.device, 1u, &rt->transfer_submit_fence, VK_TRUE, UINT64_MAX);
      if (vk_result != VK_SUCCESS) {
        prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
        stage_transfer_failure_telemetry(rt, work_slot_id, (int)vk_result);
        prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
        return PROM_ERROR;
      }
      stage_transfer_complete_telemetry(rt, 1u, work_slot_id, 0);
    }
    vk_result = vkWaitForFences(rt->vulkan.device, 1u, &rt->submit_fence, VK_TRUE, UINT64_MAX);
    sync_wait_end_ns = prom_wall_clock_now_ns();
    rt->px16_m8_last_sync_wait_wall_ns = prom_wall_clock_elapsed_ns(sync_wait_begin_ns, sync_wait_end_ns);
    post_sync_begin_ns = sync_wait_end_ns;
    if (vk_result != VK_SUCCESS) {
      reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    if (rt->timestamp_query_supported != 0u && rt->sgemm_timestamp_query_pool != VK_NULL_HANDLE) {
      uint64_t timestamps[2];
      vk_result = vkGetQueryPoolResults(rt->vulkan.device,
                                        rt->sgemm_timestamp_query_pool,
                                        0u,
                                        2u,
                                        sizeof(timestamps),
                                        timestamps,
                                        sizeof(uint64_t),
                                        VK_QUERY_RESULT_64_BIT);
      if (vk_result != VK_SUCCESS) {
        reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_UNAVAILABLE);
      } else if (rt->vulkan.timestamp_period_ns <= 0.0f) {
        reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_PERIOD);
      } else if (timestamps[1] <= timestamps[0]) {
        reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_ORDER);
      } else {
        const double duration = ((double)(timestamps[1] - timestamps[0])) * (double)rt->vulkan.timestamp_period_ns;
        if (duration <= 0.0) {
          reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_ORDER);
        } else {
          rt->last_gpu_timing_valid = 1u;
          rt->last_gpu_timing_failure_reason = PROM_SGEMM_GPU_TIMING_FAILURE_NONE;
          rt->last_gpu_duration_ns = (uint64_t)duration;
          rt->p14_measurement_tick += 1u;
          rt->p14_last_filtered_evidence =
              prom_dominatus_measurement_filter_update(&rt->p14_measurement_filter_state, duration, rt->p14_measurement_tick);
          {
            prom_dominatus_predictor_evidence pe =
                prom_dominatus_predictor_evidence_from_filtered(&rt->p14_last_filtered_evidence);
            prom_dominatus_physical_observation po;
            memset(&po, 0, sizeof(po));
            po.tick = rt->p14_measurement_tick;
            po.actual_ready = 1u;
            po.slot_valid = 1u;
            po.memory_budget_ok = 1u;
            po.outstanding_depth_cap = rt->p15_predictor_state.params.max_outstanding_depth;
            memset(&rt->p15_last_prediction_issued, 0, sizeof(rt->p15_last_prediction_issued));
            rt->p15_last_correction = prom_dominatus_predictor_update(&rt->p15_predictor_state, &pe, &po, po.tick,
                                                                       &rt->p15_last_prediction_issued);
            (void)prom_dominatus_predictor_advance_reservations(&rt->p15_predictor_state, po.tick);
            rt->p15_last_reservation = prom_dominatus_predictor_try_reserve_future(
                &rt->p15_predictor_state,
                &rt->p15_predictor_state.reservations,
                &rt->p15_predictor_state.future_lease_seam.last_request,
                po.tick);
            {
              prom_dominatus_prestage_input pi;
              memset(&pi, 0, sizeof(pi));
              pi.valid = rt->p15_last_reservation.valid;
              pi.request_id = rt->p15_last_reservation.request_id;
              pi.current_tick = po.tick;
              pi.target_tick = rt->p15_last_reservation.target_tick;
              pi.reservation_is_reserved = rt->p15_last_reservation.reserved;
              pi.confidence = rt->p15_last_reservation.confidence;
              pi.warmup = pe.warmup;
              pi.recent_miss_count = (uint32_t)rt->p15_predictor_state.correction_count;
              pi.slot_valid = po.slot_valid;
              pi.memory_budget_ok = po.memory_budget_ok;
              pi.outstanding_depth = po.outstanding_depth;
              pi.outstanding_depth_cap = po.outstanding_depth_cap;
              pi.resource_pressure_low = 1u;
              rt->p15_last_prestage = prom_dominatus_prestage_evaluate(&rt->p15_prestage_params, &pi);
            }
            rt->p15_last_shadow = prom_dominatus_shadow_snapshot_evaluate(&rt->p15_predictor_state,
                                                                           &rt->p15_last_prediction_issued,
                                                                           &rt->p15_last_correction,
                                                                           &rt->p15_last_reservation,
                                                                           rt->p15_last_prestage.allowed,
                                                                           po.tick);
            prom_dominatus_shadow_calibration_update(&rt->p15_shadow_calibration, &rt->p15_last_shadow);
            rt->p15_shadow_authority_gate = prom_dominatus_shadow_authority_gate_evaluate_with_enabled(
                &rt->p15_shadow_calibration, rt->p15_shadow_canary_params.enabled != 0u ? 1u : 0u);
            if (rt->p15_shadow_canary_params.enabled != 0u &&
                rt->p15_shadow_authority_gate.state == PROM_SHADOW_AUTHORITY_HEALTHY &&
                rt->p15_shadow_authority_gate.recommended_lookahead_depth > 0u) {
              uint32_t depth = rt->p15_shadow_authority_gate.recommended_lookahead_depth;
              if (depth < 1u) depth = 1u;
              if (depth > 4u) depth = 4u;
              rt->p15_predictor_state.lookahead_depth = depth;
            }
            prom_dominatus_shadow_would_act_update(
                &rt->p15_shadow_would_act_state, &rt->p15_shadow_authority_gate, &rt->p15_shadow_calibration, &rt->p15_last_shadow);
            if (prom_dominatus_shadow_canary_should_attempt(&rt->p15_shadow_canary_state,
                                                            &rt->p15_shadow_canary_params,
                                                            &rt->p15_shadow_authority_gate,
                                                            &rt->p15_shadow_calibration,
                                                            &rt->p15_last_shadow) != 0u) {
              if (rt->p15_predictor_state.future_lease_seam.last_request.valid == 0u) {
                rt->p15_shadow_canary_state.action_blocked_count += 1u;
                rt->p15_shadow_canary_state.block_no_future_lease_count += 1u;
              } else {
                prom_dominatus_reservation_decision canary_reservation = prom_dominatus_predictor_try_reserve_future(
                    &rt->p15_predictor_state,
                    &rt->p15_predictor_state.reservations,
                    &rt->p15_predictor_state.future_lease_seam.last_request,
                    po.tick);
                rt->p15_shadow_canary_state.reservation_attempt_count += 1u;
                if (canary_reservation.valid != 0u && canary_reservation.reserved != 0u) {
                  rt->p15_shadow_canary_state.action_applied_count += 1u;
                  rt->p15_shadow_canary_state.reservation_success_count += 1u;
                  rt->p15_shadow_canary_state.last_applied_issued_tick = rt->p15_last_shadow.issued_tick;
                  rt->p15_shadow_canary_state.last_applied_target_tick = rt->p15_last_shadow.target_tick;
                  rt->p15_shadow_canary_state.last_applied_predicted_ready_tick = rt->p15_last_shadow.predicted_ready_tick;
                } else {
                  rt->p15_shadow_canary_state.action_blocked_count += 1u;
                  rt->p15_shadow_canary_state.block_reservation_failed_count += 1u;
                  rt->p15_shadow_canary_state.reservation_rejected_count += 1u;
                }
              }
            } else if (rt->p15_shadow_canary_params.enabled == 0u) {
              rt->p15_shadow_canary_state.block_disabled_count += 1u;
            }
          }
        }
      }
    }
    rt->in_flight_submit = 0u;
    if (!prom_slot_mark_complete(rt, work_slot_id)) {
      prom_slot_mark_failure(rt, work_slot_id, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
      return PROM_ERROR;
    }
  } else {
    prom_slot_mark_failure(rt, work_slot_id, PROM_DETAIL_REUSE_IN_FLIGHT);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_REUSE_IN_FLIGHT);
    return PROM_ERROR;
  }

  if ((rt->vulkan.test_flags & PROM_TESTCFG_FAIL_DOWNLOAD) != 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_DETAIL_INJECTED_DOWNLOAD_FAILURE);
    return PROM_ERROR;
  }

  if (selected_path == PROM_VK_PATH_DIRECT) {
    readback_begin_ns = prom_wall_clock_now_ns();
    rt->px16_m8_last_post_sync_wall_ns = prom_wall_clock_elapsed_ns(post_sync_begin_ns, readback_begin_ns);
    if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
      prom_apply_debug_row_major_oracle(rt, a, b, (float*)rt->direct_c.mapped, m, n, k);
    }
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, final_detail);
    memcpy(c, rt->direct_c.mapped, c_copy_size);
    readback_end_ns = prom_wall_clock_now_ns();
  } else if (selected_path == PROM_VK_PATH_STAGED_UPLOAD_READBACK) {
    readback_begin_ns = prom_wall_clock_now_ns();
    rt->px16_m8_last_post_sync_wall_ns = prom_wall_clock_elapsed_ns(post_sync_begin_ns, readback_begin_ns);
    if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
      prom_apply_debug_row_major_oracle(rt, a, b, (float*)rt->staged_readback_c.mapped, m, n, k);
    }
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, final_detail);
    memcpy(c, rt->staged_readback_c.mapped, c_copy_size);
    readback_end_ns = prom_wall_clock_now_ns();
  } else {
    readback_begin_ns = prom_wall_clock_now_ns();
    readback_end_ns = readback_begin_ns;
    rt->px16_m8_last_post_sync_wall_ns = prom_wall_clock_elapsed_ns(post_sync_begin_ns, readback_begin_ns);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, final_detail);
  }
  rt->px16_m8_last_readback_wall_ns = prom_wall_clock_elapsed_ns(readback_begin_ns, readback_end_ns);
  post_readback_begin_ns = readback_end_ns;

  if (out_stage != NULL &&
      out_detail_code != NULL &&
      ((*out_stage == PROM_STAGE_TRANSFER_OUT) || (selected_path == PROM_VK_PATH_STAGED_UPLOAD && *out_stage == PROM_STAGE_SUBMIT))) {
    if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
      rt->sgemm_controller.packed4_selected_layout_format = 2u;
      rt->sgemm_controller.packed4_tail_count_total += (uint64_t)packed4_tail_count;
      rt->sgemm_controller.packed4_padded_lane_count_total += (uint64_t)packed4_padded_lane_count;
      rt->sgemm_controller.packed4_selection_count += 1u;
      rt->sgemm_controller.fp16_selected_candidate = 2u;
    } else if (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
      rt->sgemm_controller.packed4_selected_layout_format = 1u;
      rt->sgemm_controller.fp16_selected_candidate = 3u;
      rt->sgemm_controller.fp16_fallback_reason_detail = 0;
    } else {
      rt->sgemm_controller.packed4_selected_layout_format = 1u;
      rt->sgemm_controller.fp16_selected_candidate = 1u;
    }
    layout_precision_decision.packed4_selected_layout_format = rt->sgemm_controller.packed4_selected_layout_format;
    layout_precision_decision.packed4_tail_count_total = rt->sgemm_controller.packed4_tail_count_total;
    layout_precision_decision.packed4_padded_lane_count_total = rt->sgemm_controller.packed4_padded_lane_count_total;
    layout_precision_decision.packed4_selection_count = rt->sgemm_controller.packed4_selection_count;
    layout_precision_decision.fp16_fallback_reason_detail = rt->sgemm_controller.fp16_fallback_reason_detail;
    layout_precision_decision.fp16_selected_candidate = rt->sgemm_controller.fp16_selected_candidate;
    if ((compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 || compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) &&
        prom_dom_sgemm_stage_layout_precision_decision(&rt->blackboard, &layout_precision_decision) != 0u) {
      prom_dom_sgemm_commit(&rt->blackboard);
    }
    if (lease_granted != 0u) {
      lease_facts.single_call_mode = 1u;
      lease_facts.yield_requested = 1u;
      lease_facts.lease_held = 1u;
      lease_facts.current_outstanding_depth = 1u;
      lease_facts.max_outstanding_depth = 1u;
      if (prom_runtime_request_resource_lease(rt, &lease_facts, &lease_yield_decision) == 0u) {
        prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_CLEANUP, PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED);
        function_return_ns = prom_wall_clock_now_ns();
        rt->px16_m8_last_post_readback_wall_ns =
            prom_wall_clock_elapsed_ns(post_readback_begin_ns, function_return_ns);
        rt->px16_m8_last_total_wall_ns = prom_wall_clock_elapsed_ns(total_wall_begin_ns, function_return_ns);
        return PROM_ERROR;
      }
      lease_facts.lease_held = 0u;
      lease_facts.current_outstanding_depth = 0u;
    }
    note_last_execution_shape(rt, m, n, k);
    function_return_ns = prom_wall_clock_now_ns();
    rt->px16_m8_last_post_readback_wall_ns =
        prom_wall_clock_elapsed_ns(post_readback_begin_ns, function_return_ns);
    rt->px16_m8_last_total_wall_ns = prom_wall_clock_elapsed_ns(total_wall_begin_ns, function_return_ns);
    return PROM_OK;
  }
  function_return_ns = prom_wall_clock_now_ns();
  rt->px16_m8_last_post_readback_wall_ns =
      prom_wall_clock_elapsed_ns(post_readback_begin_ns, function_return_ns);
  rt->px16_m8_last_total_wall_ns = prom_wall_clock_elapsed_ns(total_wall_begin_ns, function_return_ns);
  return PROM_ERROR;
}

static int prom_compare_u64(const void* left, const void* right) {
  const uint64_t a = *(const uint64_t*)left;
  const uint64_t b = *(const uint64_t*)right;
  if (a < b) return -1;
  if (a > b) return 1;
  return 0;
}

static uint64_t prom_percentile_u64(uint64_t* samples, uint32_t count, uint32_t numerator, uint32_t denominator) {
  uint64_t scaled;
  uint32_t index;
  if (samples == NULL || count == 0u || denominator == 0u) {
    return 0u;
  }
  qsort(samples, count, sizeof(uint64_t), prom_compare_u64);
  scaled = ((uint64_t)(count - 1u) * (uint64_t)numerator + (uint64_t)(denominator - 1u)) / (uint64_t)denominator;
  index = scaled > (uint64_t)(count - 1u) ? count - 1u : (uint32_t)scaled;
  return samples[index];
}

static VkPipeline prom_sgemm_pipeline_for_resident_dispatch(prometheus_runtime* rt, uint32_t compute_mode, uint32_t variant) {
  if (rt == NULL) {
    return VK_NULL_HANDLE;
  }
  if (compute_mode == PROM_VK_COMPUTE_TILED) {
    VkPipeline pipeline = prom_shader_registry_pipeline_for_variant(
        rt->compute_pipeline_instances,
        sizeof(rt->compute_pipeline_instances) / sizeof(rt->compute_pipeline_instances[0]), variant);
    return pipeline == VK_NULL_HANDLE ? rt->tiled_pipeline : pipeline;
  }
  if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
    return rt->packed4_pipeline;
  }
  if (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
    return rt->fp16_pipeline;
  }
  return rt->pipeline;
}

/* M29's ring is deliberately private to the resident diagnostic.  The shared
   A/B/C buffers are safe here because each ordered dispatch overwrites C; this
   is not the output-ownership model required by public multi-token async. */
static prom_sgemm_submission_slot* prom_sgemm_ring_acquire_empty(prometheus_runtime* rt) {
  uint32_t offset;
  if (rt == NULL) return NULL;
  for (offset = 0u; offset < rt->submission_ring_diag.configured_depth; ++offset) {
    uint32_t index = (rt->submission_ring_diag.acquire_cursor + offset) % rt->submission_ring_diag.configured_depth;
    prom_sgemm_submission_slot* slot = &rt->submission_ring[index];
    if (slot->state == PROM_SGEMM_SUBMISSION_SLOT_READY) {
      slot->state = PROM_SGEMM_SUBMISSION_SLOT_EMPTY;
      rt->submission_ring_diag.slot_recycle_count += 1u;
    }
    if (slot->state == PROM_SGEMM_SUBMISSION_SLOT_EMPTY) {
      slot->state = PROM_SGEMM_SUBMISSION_SLOT_PREPARING;
      slot->generation += 1u;
      slot->submission_sequence = rt->submission_ring_diag.next_sequence++;
      slot->failure_stage = PROM_STAGE_NONE;
      slot->failure_detail = 0;
      slot->timing_valid = 0u;
      rt->submission_ring_diag.acquire_cursor = (index + 1u) % rt->submission_ring_diag.configured_depth;
      return slot;
    }
  }
  rt->submission_ring_diag.ring_full_count += 1u;
  return NULL;
}

static int prom_sgemm_ring_harvest_slot(prometheus_runtime* rt, prom_sgemm_submission_slot* slot) {
  VkResult result;
  uint64_t timestamps[2];
  if (rt == NULL || slot == NULL || slot->state != PROM_SGEMM_SUBMISSION_SLOT_SUBMITTED) return PROM_ERROR;
  slot->state = PROM_SGEMM_SUBMISSION_SLOT_COMPLETE;
  slot->physical_completion_confirmed = 1u;
  if (rt->timestamp_query_supported != 0u) {
    /* The fence established physical completion before this test-only failure
       is selected.  No GPU resource remains pending and no Vulkan call is
       faked with invalid inputs. */
    if ((rt->async_test_flags & PROM_ASYNC_TESTCFG_FAIL_QUERY_RESULT) != 0u) {
      rt->async_test_flags &= ~PROM_ASYNC_TESTCFG_FAIL_QUERY_RESULT;
      result = VK_ERROR_UNKNOWN;
    } else {
      result = vkGetQueryPoolResults(rt->vulkan.device, rt->sgemm_timestamp_query_pool, slot->query_base, 2u,
                                     sizeof(timestamps), timestamps, sizeof(uint64_t), VK_QUERY_RESULT_64_BIT);
    }
    rt->submission_ring_diag.total_query_harvests += 1u;
    if (result == VK_SUCCESS && rt->vulkan.timestamp_period_ns > 0.0f && timestamps[1] > timestamps[0]) {
      slot->gpu_duration_ns = (uint64_t)(((double)(timestamps[1] - timestamps[0]) * (double)rt->vulkan.timestamp_period_ns));
      slot->timing_valid = slot->gpu_duration_ns > 0u ? 1u : 0u;
      if (slot->timing_valid != 0u) {
        rt->submission_ring_diag.total_gpu_duration_ns += slot->gpu_duration_ns;
        rt->submission_ring_diag.gpu_timing_sample_count += 1u;
      }
    } else if (result != VK_SUCCESS) {
      slot->failure_stage = PROM_STAGE_SUBMIT;
      slot->failure_detail = (int)result;
      slot->state = PROM_SGEMM_SUBMISSION_SLOT_FAILED;
      rt->submission_ring_diag.slot_failure_count += 1u;
      if (rt->submission_ring_diag.outstanding != 0u) rt->submission_ring_diag.outstanding -= 1u;
      return PROM_ERROR;
    }
  }
  slot->state = PROM_SGEMM_SUBMISSION_SLOT_READY;
  rt->submission_ring_diag.outstanding -= 1u;
  return PROM_OK;
}

static int prom_sgemm_ring_poll_slot(prometheus_runtime* rt, prom_sgemm_submission_slot* slot) {
  VkResult result;
  if (rt == NULL || slot == NULL || slot->state != PROM_SGEMM_SUBMISSION_SLOT_SUBMITTED) return PROM_OK;
  rt->submission_ring_diag.total_polls += 1u;
  result = vkGetFenceStatus(rt->vulkan.device, slot->fence);
  if (result == VK_NOT_READY) return PROM_OK;
  if (result != VK_SUCCESS) {
    slot->state = PROM_SGEMM_SUBMISSION_SLOT_FAILED;
    slot->failure_stage = PROM_STAGE_SUBMIT;
    slot->failure_detail = (int)result;
    rt->submission_ring_diag.slot_failure_count += 1u;
    return PROM_ERROR;
  }
  return prom_sgemm_ring_harvest_slot(rt, slot);
}

int prom_sgemm_ring_wait_oldest(prometheus_runtime* rt) {
  prom_sgemm_submission_slot* oldest = NULL;
  uint32_t index;
  VkResult result;
  if (rt == NULL) return PROM_ERROR;
  for (index = 0u; index < rt->submission_ring_diag.configured_depth; ++index) {
    prom_sgemm_submission_slot* candidate = &rt->submission_ring[index];
    if (candidate->state == PROM_SGEMM_SUBMISSION_SLOT_SUBMITTED &&
        (oldest == NULL || candidate->submission_sequence < oldest->submission_sequence)) oldest = candidate;
  }
  if (oldest == NULL) return PROM_OK;
  rt->submission_ring_diag.total_forced_waits += 1u;
  result = vkWaitForFences(rt->vulkan.device, 1u, &oldest->fence, VK_TRUE, UINT64_MAX);
  if (result != VK_SUCCESS) return PROM_ERROR;
  return prom_sgemm_ring_harvest_slot(rt, oldest);
}

static int prom_sgemm_record_resident_slot(prometheus_runtime* rt, prom_sgemm_submission_slot* slot,
                                           uint32_t m, uint32_t n, uint32_t compute_k, uint32_t compute_mode,
                                           uint32_t variant) {
  VkWriteDescriptorSet writes[3]; VkDescriptorBufferInfo infos[3]; VkCommandBufferBeginInfo begin; VkBufferMemoryBarrier barrier;
  prom_vk_push push; VkPipeline pipeline; VkResult result; prom_sgemm_dispatch_geometry geometry;
  if (rt == NULL || slot == NULL || slot->state != PROM_SGEMM_SUBMISSION_SLOT_PREPARING) return PROM_ERROR;
  pipeline = prom_sgemm_pipeline_for_resident_dispatch(rt, compute_mode, variant);
  if (pipeline == VK_NULL_HANDLE) return PROM_ERROR;
  memset(infos, 0, sizeof(infos));
  infos[0].buffer = rt->staged_device_a.buffer; infos[0].range = rt->staged_device_a.size;
  infos[1].buffer = rt->staged_device_b.buffer; infos[1].range = rt->staged_device_b.size;
  infos[2].buffer = rt->staged_device_c.buffer; infos[2].range = rt->staged_device_c.size;
  memset(writes, 0, sizeof(writes));
  for (uint32_t i = 0u; i < 3u; ++i) { writes[i].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET; writes[i].dstSet = slot->descriptor_set; writes[i].dstBinding = i; writes[i].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER; writes[i].descriptorCount = 1u; writes[i].pBufferInfo = &infos[i]; }
  vkUpdateDescriptorSets(rt->vulkan.device, 3u, writes, 0u, NULL);
  result = vkResetCommandBuffer(slot->command_buffer, 0u); if (result != VK_SUCCESS) goto failed;
  memset(&begin, 0, sizeof(begin)); begin.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  result = vkBeginCommandBuffer(slot->command_buffer, &begin); if (result != VK_SUCCESS) goto failed;
  memset(&barrier, 0, sizeof(barrier)); barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER; barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT; barrier.dstAccessMask = VK_ACCESS_SHADER_WRITE_BIT; barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED; barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED; barrier.buffer = rt->staged_device_c.buffer; barrier.size = rt->staged_device_c.size;
  vkCmdPipelineBarrier(slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0, 0, NULL, 1u, &barrier, 0, NULL);
  vkCmdBindPipeline(slot->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, pipeline);
  vkCmdBindDescriptorSets(slot->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, rt->pipeline_layout, 0u, 1u, &slot->descriptor_set, 0u, NULL);
  push.m=m; push.n=n; push.k=compute_k; vkCmdPushConstants(slot->command_buffer, rt->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT, 0u, PROM_VK_SHADER_PUSH_BYTES, &push);
  if (rt->timestamp_query_supported != 0u) { vkCmdResetQueryPool(slot->command_buffer, rt->sgemm_timestamp_query_pool, slot->query_base, 2u); vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, rt->sgemm_timestamp_query_pool, slot->query_base); }
  geometry = prom_sgemm_dispatch_geometry_for_variant(variant, m, n); vkCmdDispatch(slot->command_buffer, geometry.groups_x, geometry.groups_y, geometry.groups_z);
  if (rt->timestamp_query_supported != 0u) vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, rt->sgemm_timestamp_query_pool, slot->query_base + 1u);
  result = vkEndCommandBuffer(slot->command_buffer); if (result != VK_SUCCESS) goto failed;
  slot->m=m; slot->n=n; slot->compute_k=compute_k; slot->compute_mode=compute_mode; slot->variant=variant; slot->state=PROM_SGEMM_SUBMISSION_SLOT_RECORDED; return PROM_OK;
failed:
  slot->state=PROM_SGEMM_SUBMISSION_SLOT_FAILED; slot->failure_stage=PROM_STAGE_SUBMIT; slot->failure_detail=(int)result; rt->submission_ring_diag.slot_failure_count += 1u; return PROM_ERROR;
}

int prom_sgemm_ring_submit_slot(prometheus_runtime* rt, prom_sgemm_submission_slot* slot) {
  VkSubmitInfo info; VkResult result;
  if (rt == NULL || slot == NULL || slot->state != PROM_SGEMM_SUBMISSION_SLOT_RECORDED) return PROM_ERROR;
  result = (rt->vulkan.test_flags & PROM_TESTCFG_FAIL_RESET_FENCE) != 0u ? VK_ERROR_INITIALIZATION_FAILED : vkResetFences(rt->vulkan.device, 1u, &slot->fence);
  if (result != VK_SUCCESS) goto failed;
  memset(&info, 0, sizeof(info)); info.sType=VK_STRUCTURE_TYPE_SUBMIT_INFO; info.commandBufferCount=1u; info.pCommandBuffers=&slot->command_buffer;
  result = (rt->vulkan.test_flags & PROM_TESTCFG_FAIL_QUEUE_SUBMIT) != 0u ? VK_ERROR_INITIALIZATION_FAILED : vkQueueSubmit(rt->vulkan.compute_queue, 1u, &info, slot->fence);
  if (result != VK_SUCCESS) goto failed;
  slot->state=PROM_SGEMM_SUBMISSION_SLOT_SUBMITTED; rt->submission_ring_diag.total_submits += 1u; rt->submission_ring_diag.outstanding += 1u;
  if (rt->submission_ring_diag.outstanding > rt->submission_ring_diag.max_outstanding) rt->submission_ring_diag.max_outstanding=rt->submission_ring_diag.outstanding;
  return PROM_OK;
failed:
  slot->state=PROM_SGEMM_SUBMISSION_SLOT_FAILED; slot->failure_stage=PROM_STAGE_SUBMIT; slot->failure_detail=(int)result; rt->submission_ring_diag.slot_failure_count += 1u; return PROM_ERROR;
}

static int prom_sgemm_resident_ring_dispatches(prometheus_runtime* rt, uint32_t count, uint32_t m, uint32_t n,
                                               uint32_t compute_k, uint32_t compute_mode, uint32_t variant) {
  uint32_t submitted = 0u;
  uint32_t index;
  if (rt == NULL) return PROM_ERROR;
  while (submitted < count) {
    prom_sgemm_submission_slot* slot;
    for (index = 0u; index < rt->submission_ring_diag.configured_depth; ++index) {
      if (prom_sgemm_ring_poll_slot(rt, &rt->submission_ring[index]) != PROM_OK) return PROM_ERROR;
    }
    slot = prom_sgemm_ring_acquire_empty(rt);
    if (slot == NULL) {
      if (prom_sgemm_ring_wait_oldest(rt) != PROM_OK) return PROM_ERROR;
      continue;
    }
    if (prom_sgemm_record_resident_slot(rt, slot, m, n, compute_k, compute_mode, variant) != PROM_OK ||
        prom_sgemm_ring_submit_slot(rt, slot) != PROM_OK) return PROM_ERROR;
    submitted += 1u;
  }
  while (rt->submission_ring_diag.outstanding != 0u) {
    if (prom_sgemm_ring_wait_oldest(rt) != PROM_OK) return PROM_ERROR;
  }
  if (rt->submission_ring_diag.gpu_timing_sample_count != 0u) {
    rt->last_gpu_timing_valid = 1u;
    rt->last_gpu_duration_ns = rt->submission_ring_diag.total_gpu_duration_ns /
                               rt->submission_ring_diag.gpu_timing_sample_count;
    rt->last_gpu_timing_failure_reason = PROM_SGEMM_GPU_TIMING_FAILURE_NONE;
  }
  return PROM_OK;
}

static int prom_sgemm_resident_dispatch_once(prometheus_runtime* rt,
                                             uint32_t m,
                                             uint32_t n,
                                             uint32_t compute_k,
                                             uint32_t compute_mode,
                                             uint32_t variant,
                                             uint32_t dispatch_count,
                                             uint32_t* out_stage,
                                             int* out_detail_code) {
  VkWriteDescriptorSet writes[3];
  VkDescriptorBufferInfo buffer_infos[3];
  VkCommandBufferBeginInfo begin_info;
  VkSubmitInfo submit_info;
  VkBufferMemoryBarrier barrier;
  prom_vk_push push;
  VkPipeline pipeline;
  VkResult vk_result;
  uint64_t command_record_begin_ns;
  uint64_t dispatch_submit_begin_ns;
  uint64_t dispatch_submit_end_ns;
  uint64_t sync_wait_begin_ns;
  uint64_t sync_wait_end_ns;
  prom_sgemm_dispatch_geometry dispatch_geometry;

  if (rt == NULL || rt->has_staged_buffers == 0u ||
      rt->staged_device_a.buffer == VK_NULL_HANDLE ||
      rt->staged_device_b.buffer == VK_NULL_HANDLE ||
      rt->staged_device_c.buffer == VK_NULL_HANDLE) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  pipeline = prom_sgemm_pipeline_for_resident_dispatch(rt, compute_mode, variant);
  if (pipeline == VK_NULL_HANDLE) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_ERROR);
    return PROM_ERROR;
  }

  memset(buffer_infos, 0, sizeof(buffer_infos));
  buffer_infos[0].buffer = rt->staged_device_a.buffer;
  buffer_infos[0].range = rt->staged_device_a.size;
  buffer_infos[1].buffer = rt->staged_device_b.buffer;
  buffer_infos[1].range = rt->staged_device_b.size;
  buffer_infos[2].buffer = rt->staged_device_c.buffer;
  buffer_infos[2].range = rt->staged_device_c.size;
  memset(writes, 0, sizeof(writes));
  writes[0].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
  writes[0].dstSet = rt->descriptor_set;
  writes[0].dstBinding = 0u;
  writes[0].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  writes[0].descriptorCount = 1u;
  writes[0].pBufferInfo = &buffer_infos[0];
  writes[1] = writes[0];
  writes[1].dstBinding = 1u;
  writes[1].pBufferInfo = &buffer_infos[1];
  writes[2] = writes[0];
  writes[2].dstBinding = 2u;
  writes[2].pBufferInfo = &buffer_infos[2];
  command_record_begin_ns = prom_wall_clock_now_ns();
  vkUpdateDescriptorSets(rt->vulkan.device, 3u, writes, 0u, NULL);

  vk_result = vkResetCommandBuffer(rt->command_buffer, 0u);
  if (vk_result != VK_SUCCESS) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  vk_result = vkBeginCommandBuffer(rt->command_buffer, &begin_info);
  if (vk_result != VK_SUCCESS) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }

  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = rt->staged_device_c.buffer;
  barrier.size = rt->staged_device_c.size;
  vkCmdPipelineBarrier(rt->command_buffer,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       0,
                       0,
                       NULL,
                       1u,
                       &barrier,
                       0,
                       NULL);

  vkCmdBindPipeline(rt->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, pipeline);
  vkCmdBindDescriptorSets(rt->command_buffer,
                          VK_PIPELINE_BIND_POINT_COMPUTE,
                          rt->pipeline_layout,
                          0u,
                          1u,
                          &rt->descriptor_set,
                          0u,
                          NULL);
  push.m = m;
  push.n = n;
  push.k = compute_k;
  vkCmdPushConstants(rt->command_buffer,
                     rt->pipeline_layout,
                     VK_SHADER_STAGE_COMPUTE_BIT,
                     0u,
                     PROM_VK_SHADER_PUSH_BYTES,
                     &push);

  if (rt->timestamp_query_supported != 0u && rt->sgemm_timestamp_query_pool != VK_NULL_HANDLE) {
    vkCmdResetQueryPool(rt->command_buffer, rt->sgemm_timestamp_query_pool, 0u, 2u);
    vkCmdWriteTimestamp(rt->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, rt->sgemm_timestamp_query_pool, 0u);
  }
  dispatch_geometry = prom_sgemm_dispatch_geometry_for_variant(variant, m, n);
  /* M28 diagnostic batching deliberately reuses the same resident inputs and
     output.  Each kernel overwrites C, so only the final output is observed;
     this is a feed-path experiment, not a production scheduling path. */
  for (uint32_t dispatch_index = 0u; dispatch_index < dispatch_count; ++dispatch_index) {
    vkCmdDispatch(rt->command_buffer,
                  dispatch_geometry.groups_x,
                  dispatch_geometry.groups_y,
                  dispatch_geometry.groups_z);
  }
  if (rt->timestamp_query_supported != 0u && rt->sgemm_timestamp_query_pool != VK_NULL_HANDLE) {
    vkCmdWriteTimestamp(rt->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, rt->sgemm_timestamp_query_pool, 1u);
  }
  vk_result = vkEndCommandBuffer(rt->command_buffer);
  if (vk_result != VK_SUCCESS) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }
  rt->px16_m8_last_command_record_wall_ns =
      prom_wall_clock_elapsed_ns(command_record_begin_ns, prom_wall_clock_now_ns());

  vk_result = vkResetFences(rt->vulkan.device, 1u, &rt->submit_fence);
  if (vk_result != VK_SUCCESS) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }
  memset(&submit_info, 0, sizeof(submit_info));
  submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit_info.commandBufferCount = 1u;
  submit_info.pCommandBuffers = &rt->command_buffer;
  dispatch_submit_begin_ns = prom_wall_clock_now_ns();
  vk_result = vkQueueSubmit(rt->vulkan.compute_queue, 1u, &submit_info, rt->submit_fence);
  dispatch_submit_end_ns = prom_wall_clock_now_ns();
  rt->px16_m8_last_dispatch_submit_wall_ns =
      prom_wall_clock_elapsed_ns(dispatch_submit_begin_ns, dispatch_submit_end_ns);
  if (vk_result != VK_SUCCESS) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }

  sync_wait_begin_ns = prom_wall_clock_now_ns();
  vk_result = vkWaitForFences(rt->vulkan.device, 1u, &rt->submit_fence, VK_TRUE, UINT64_MAX);
  sync_wait_end_ns = prom_wall_clock_now_ns();
  rt->px16_m8_last_sync_wait_wall_ns = prom_wall_clock_elapsed_ns(sync_wait_begin_ns, sync_wait_end_ns);
  if (vk_result != VK_SUCCESS) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }

  if (rt->timestamp_query_supported != 0u && rt->sgemm_timestamp_query_pool != VK_NULL_HANDLE) {
    uint64_t timestamps[2];
    const uint64_t query_begin_ns = prom_wall_clock_now_ns();
    vk_result = vkGetQueryPoolResults(rt->vulkan.device,
                                      rt->sgemm_timestamp_query_pool,
                                      0u,
                                      2u,
                                      sizeof(timestamps),
                                      timestamps,
                                      sizeof(uint64_t),
                                      VK_QUERY_RESULT_64_BIT);
    rt->px16_m8_last_query_result_wall_ns = prom_wall_clock_elapsed_ns(query_begin_ns, prom_wall_clock_now_ns());
    if (vk_result != VK_SUCCESS) {
      reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_UNAVAILABLE);
    } else if (rt->vulkan.timestamp_period_ns <= 0.0f) {
      reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_PERIOD);
    } else if (timestamps[1] <= timestamps[0]) {
      reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_ORDER);
    } else {
      const double duration = ((double)(timestamps[1] - timestamps[0])) * (double)rt->vulkan.timestamp_period_ns;
      if (duration <= 0.0) {
        reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_ORDER);
      } else {
        rt->last_gpu_timing_valid = 1u;
        rt->last_gpu_timing_failure_reason = PROM_SGEMM_GPU_TIMING_FAILURE_NONE;
        rt->last_gpu_duration_ns = (uint64_t)duration;
      }
    }
  }

  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_PATH_STAGED_UPLOAD_TILED);
  return PROM_OK;
}

static int prom_sgemm_resident_readback_once(prometheus_runtime* rt,
                                             float* c,
                                             size_t c_copy_size,
                                             uint32_t* out_stage,
                                             int* out_detail_code) {
  VkCommandBufferBeginInfo begin_info;
  VkSubmitInfo submit_info;
  VkBufferMemoryBarrier barrier;
  VkBufferCopy copy;
  VkResult vk_result;
  if (rt == NULL || c == NULL || c_copy_size == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_ERROR);
    return PROM_ERROR;
  }
  vk_result = vkResetCommandBuffer(rt->command_buffer, 0u);
  if (vk_result != VK_SUCCESS) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, (int)vk_result);
    return PROM_ERROR;
  }
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  vk_result = vkBeginCommandBuffer(rt->command_buffer, &begin_info);
  if (vk_result != VK_SUCCESS) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, (int)vk_result);
    return PROM_ERROR;
  }
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = rt->staged_device_c.buffer;
  barrier.size = rt->staged_device_c.size;
  vkCmdPipelineBarrier(rt->command_buffer,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT,
                       0,
                       0,
                       NULL,
                       1u,
                       &barrier,
                       0,
                       NULL);
  memset(&copy, 0, sizeof(copy));
  copy.size = (VkDeviceSize)c_copy_size;
  vkCmdCopyBuffer(rt->command_buffer, rt->staged_device_c.buffer, rt->staged_readback_c.buffer, 1u, &copy);
  barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_HOST_READ_BIT;
  barrier.buffer = rt->staged_readback_c.buffer;
  barrier.size = rt->staged_readback_c.size;
  vkCmdPipelineBarrier(rt->command_buffer,
                       VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_HOST_BIT,
                       0,
                       0,
                       NULL,
                       1u,
                       &barrier,
                       0,
                       NULL);
  vk_result = vkEndCommandBuffer(rt->command_buffer);
  if (vk_result != VK_SUCCESS) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, (int)vk_result);
    return PROM_ERROR;
  }
  vk_result = vkResetFences(rt->vulkan.device, 1u, &rt->submit_fence);
  if (vk_result != VK_SUCCESS) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, (int)vk_result);
    return PROM_ERROR;
  }
  memset(&submit_info, 0, sizeof(submit_info));
  submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit_info.commandBufferCount = 1u;
  submit_info.pCommandBuffers = &rt->command_buffer;
  vk_result = vkQueueSubmit(rt->vulkan.compute_queue, 1u, &submit_info, rt->submit_fence);
  if (vk_result != VK_SUCCESS) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, (int)vk_result);
    return PROM_ERROR;
  }
  vk_result = vkWaitForFences(rt->vulkan.device, 1u, &rt->submit_fence, VK_TRUE, UINT64_MAX);
  if (vk_result != VK_SUCCESS) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, (int)vk_result);
    return PROM_ERROR;
  }
  memcpy(c, rt->staged_readback_c.mapped, c_copy_size);
  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED);
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_resident_benchmark_impl(void* handle,
                                                       const PrometheusSgemmResidentBenchmarkRequest* request,
                                                       PrometheusSgemmResidentBenchmarkResult* out_result) {
  prometheus_runtime* rt;
  PrometheusSgemmPolicyDiagnostics diag;
  float* scratch_c = NULL;
  uint64_t* kernel_samples = NULL;
  uint64_t* submit_samples = NULL;
  uint64_t* wait_samples = NULL;
  uint64_t* query_samples = NULL;
  uint32_t stage = PROM_STAGE_NONE;
  int detail_code = 0;
  uint32_t saved_flags;
  uint32_t iterations;
  uint32_t warmup_iterations;
  uint32_t diagnostic_batch_depth;
  uint32_t executed_variant;
  uint32_t compute_mode;
  uint32_t compute_k;
  uint32_t i;
  VkDeviceSize checked_buffer_size;
  size_t checked_copy_size;
  size_t c_copy_size;
  uint64_t setup_begin_ns;
  uint64_t setup_end_ns;
  uint64_t loop_begin_ns;
  uint64_t loop_end_ns;
  uint64_t readback_begin_ns;
  uint64_t readback_end_ns;
  int status;

  if (out_result == NULL) {
    return PROM_ERROR;
  }
  memset(out_result, 0, sizeof(*out_result));
  out_result->struct_size = (uint32_t)sizeof(*out_result);
  if (handle == NULL || !registry_contains(handle)) {
    out_result->setup_stage = PROM_STAGE_INIT;
    out_result->setup_detail_code = PROM_INVALID_HANDLE;
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  out_result->resident_mode_available =
      (rt->vulkan.available != 0u &&
       rt->vulkan.has_device_local_memory != 0u &&
       (rt->vulkan.test_flags & PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY) == 0u)
          ? 1u
          : 0u;
  if (request == NULL || request->a == NULL || request->b == NULL ||
      request->m == 0u || request->n == 0u || request->k == 0u ||
      (request->flags & PROM_SGEMM_RESIDENT_FLAG_VALIDATE_READBACK) != 0u && request->c == NULL) {
    out_result->setup_stage = PROM_STAGE_TRANSFER_IN;
    out_result->setup_detail_code = PROM_ERROR;
    return PROM_ERROR;
  }
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    out_result->setup_stage = PROM_STAGE_INIT;
    out_result->setup_detail_code = PROM_INVALID_HANDLE;
    return PROM_INVALID_HANDLE;
  }
  if (rt->vulkan.available == 0u ||
      rt->vulkan.has_device_local_memory == 0u ||
      (rt->vulkan.test_flags & PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY) != 0u) {
    out_result->setup_stage = PROM_STAGE_INIT;
    out_result->setup_detail_code = rt->vulkan.available == 0u ? rt->vulkan.init_detail_code : PROM_ERROR;
    return PROM_ERROR;
  }
  if (!prom_vk_checked_mul_u32(request->m, request->n, &compute_k) ||
      !checked_float_buffer_size(request->m, request->k, &checked_buffer_size, &checked_copy_size) ||
      !checked_float_buffer_size(request->k, request->n, &checked_buffer_size, &checked_copy_size) ||
      !checked_float_buffer_size(request->m, request->n, &checked_buffer_size, &c_copy_size)) {
    out_result->setup_stage = PROM_STAGE_TRANSFER_IN;
    out_result->setup_detail_code = PROM_DETAIL_SIZE_OVERFLOW;
    return PROM_ERROR;
  }
  scratch_c = (float*)malloc(c_copy_size);
  if (scratch_c == NULL) {
    out_result->setup_stage = PROM_STAGE_TRANSFER_IN;
    out_result->setup_detail_code = PROM_ERROR;
    return PROM_ERROR;
  }
  memset(scratch_c, 0, c_copy_size);
  iterations = request->iterations == 0u ? 1u : request->iterations;
  warmup_iterations = request->warmup_iterations;
  diagnostic_batch_depth = request->diagnostic_batch_depth == 0u ? 1u : request->diagnostic_batch_depth;
  kernel_samples = (uint64_t*)calloc(iterations, sizeof(uint64_t));
  submit_samples = (uint64_t*)calloc(iterations, sizeof(uint64_t));
  wait_samples = (uint64_t*)calloc(iterations, sizeof(uint64_t));
  query_samples = (uint64_t*)calloc(iterations, sizeof(uint64_t));
  if (kernel_samples == NULL || submit_samples == NULL || wait_samples == NULL || query_samples == NULL) {
    free(scratch_c);
    free(kernel_samples);
    free(submit_samples);
    free(wait_samples);
    free(query_samples);
    out_result->setup_stage = PROM_STAGE_TRANSFER_IN;
    out_result->setup_detail_code = PROM_ERROR;
    return PROM_ERROR;
  }

  saved_flags = rt->vulkan.test_flags;
  rt->vulkan.test_flags = saved_flags | PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_UPLOAD_ONLY;
  setup_begin_ns = prom_wall_clock_now_ns();
  if (request->mode == PROM_SGEMM_RESIDENT_MODE_EXPLICIT_VARIANT) {
    status = prom_reactor_runtime_sgemm_benchmark_variant_impl(handle,
                                                               request->a,
                                                               request->b,
                                                               scratch_c,
                                                               request->m,
                                                               request->n,
                                                               request->k,
                                                               request->requested_variant,
                                                               &stage,
                                                               &detail_code);
  } else {
    status = prom_reactor_runtime_sgemm_impl(handle,
                                             request->a,
                                             request->b,
                                             scratch_c,
                                             request->m,
                                             request->n,
                                             request->k,
                                             &stage,
                                             &detail_code);
  }
  setup_end_ns = prom_wall_clock_now_ns();
  rt->vulkan.test_flags = saved_flags;
  out_result->setup_stage = stage;
  out_result->setup_detail_code = detail_code;
  out_result->setup_wall_ns = prom_wall_clock_elapsed_ns(setup_begin_ns, setup_end_ns);
  out_result->upload_once_wall_ns = rt->px16_m8_last_upload_wall_ns;
  if (status != PROM_OK) {
    free(scratch_c);
    free(kernel_samples);
    free(submit_samples);
    free(wait_samples);
    return status;
  }
  if (prom_reactor_runtime_sgemm_policy_diagnostics_impl(handle, &diag) != PROM_OK) {
    free(scratch_c);
    free(kernel_samples);
    free(submit_samples);
    free(wait_samples);
    out_result->final_stage = PROM_STAGE_SUBMIT;
    out_result->final_detail_code = PROM_ERROR;
    return PROM_ERROR;
  }

  out_result->resident_mode_used = 1u;
  out_result->requested_variant = request->mode == PROM_SGEMM_RESIDENT_MODE_EXPLICIT_VARIANT ? request->requested_variant : 0u;
  out_result->production_selected_variant = diag.px16_m6_selector_selected_variant;
  executed_variant = diag.px16_m6_executed_dispatch_variant;
  compute_mode = diag.px16_m6_executed_compute_mode;
  out_result->executed_variant = executed_variant;
  out_result->executed_compute_mode = compute_mode;
  compute_k = (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ||
               compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM)
                  ? prom_round_up4_u32(request->k)
                  : request->k;

  if ((request->flags & PROM_SGEMM_RESIDENT_FLAG_M29_SUBMISSION_RING) != 0u) {
    uint32_t requested_depth = request->diagnostic_batch_depth == 0u
                                   ? PROM_SGEMM_SUBMISSION_RING_DEFAULT_DEPTH
                                   : diagnostic_batch_depth;
    if (requested_depth < 1u || requested_depth > PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH) {
      out_result->final_stage = PROM_STAGE_SUBMIT;
      out_result->final_detail_code = PROM_ERROR;
      free(scratch_c); free(kernel_samples); free(submit_samples); free(wait_samples); free(query_samples);
      return PROM_ERROR;
    }
    memset(&rt->submission_ring_diag, 0, sizeof(rt->submission_ring_diag));
    rt->submission_ring_diag.configured_depth = requested_depth;
    if (warmup_iterations != 0u &&
        prom_sgemm_resident_ring_dispatches(rt, warmup_iterations, request->m, request->n, compute_k, compute_mode, executed_variant) != PROM_OK) {
      out_result->final_stage = PROM_STAGE_SUBMIT; out_result->final_detail_code = PROM_ERROR;
      free(scratch_c); free(kernel_samples); free(submit_samples); free(wait_samples); free(query_samples); return PROM_ERROR;
    }
    loop_begin_ns = prom_wall_clock_now_ns();
    status = prom_sgemm_resident_ring_dispatches(rt, iterations, request->m, request->n, compute_k, compute_mode, executed_variant);
    loop_end_ns = prom_wall_clock_now_ns();
    if (status != PROM_OK) {
      out_result->final_stage = PROM_STAGE_SUBMIT; out_result->final_detail_code = PROM_ERROR;
      free(scratch_c); free(kernel_samples); free(submit_samples); free(wait_samples); free(query_samples); return status;
    }
    for (i = 0u; i < iterations; ++i) kernel_samples[i] = rt->last_gpu_duration_ns;
    out_result->iterations = iterations;
    out_result->total_loop_wall_ns = prom_wall_clock_elapsed_ns(loop_begin_ns, loop_end_ns);
    out_result->diagnostic_batch_depth = requested_depth;
    out_result->queue_submissions = (uint32_t)rt->submission_ring_diag.total_submits;
    out_result->fence_waits = (uint32_t)rt->submission_ring_diag.total_forced_waits;
    out_result->command_buffer_recordings = (uint32_t)rt->submission_ring_diag.total_submits;
    out_result->command_buffer_resets = (uint32_t)rt->submission_ring_diag.total_submits;
    out_result->descriptor_updates = (uint32_t)rt->submission_ring_diag.total_submits;
    out_result->configured_ring_depth = requested_depth;
    out_result->physical_slot_count = PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH;
    out_result->max_in_flight_depth = rt->submission_ring_diag.max_outstanding;
    out_result->ring_poll_count = rt->submission_ring_diag.total_polls;
    out_result->ring_forced_wait_count = rt->submission_ring_diag.total_forced_waits;
    out_result->ring_query_harvest_count = rt->submission_ring_diag.total_query_harvests;
    out_result->ring_full_count = rt->submission_ring_diag.ring_full_count;
    out_result->ring_slot_recycle_count = rt->submission_ring_diag.slot_recycle_count;
    out_result->ring_failure_count = rt->submission_ring_diag.slot_failure_count;
    for (i = 0u; i < PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH; ++i) {
      out_result->ring_final_slot_state[i] = rt->submission_ring[i].state;
    }
    goto resident_timing_complete;
  }

  for (i = 0u; i < warmup_iterations; ++i) {
    status = prom_sgemm_resident_dispatch_once(rt,
                                               request->m,
                                               request->n,
                                               compute_k,
                                               compute_mode,
                                               executed_variant,
                                               diagnostic_batch_depth,
                                               &stage,
                                               &detail_code);
    if (status != PROM_OK) {
      out_result->final_stage = stage;
      out_result->final_detail_code = detail_code;
      free(scratch_c);
      free(kernel_samples);
      free(submit_samples);
      free(wait_samples);
      free(query_samples);
      return status;
    }
  }

  loop_begin_ns = prom_wall_clock_now_ns();
  for (i = 0u; i < iterations; ++i) {
    status = prom_sgemm_resident_dispatch_once(rt,
                                               request->m,
                                               request->n,
                                               compute_k,
                                               compute_mode,
                                               executed_variant,
                                               diagnostic_batch_depth,
                                               &stage,
                                               &detail_code);
    if (status != PROM_OK) {
      out_result->final_stage = stage;
      out_result->final_detail_code = detail_code;
      free(scratch_c);
      free(kernel_samples);
      free(submit_samples);
      free(wait_samples);
      free(query_samples);
      return status;
    }
    submit_samples[i] = rt->px16_m8_last_dispatch_submit_wall_ns;
    wait_samples[i] = rt->px16_m8_last_sync_wait_wall_ns;
    query_samples[i] = rt->px16_m8_last_query_result_wall_ns;
    if (rt->last_gpu_timing_valid != 0u && rt->last_gpu_duration_ns > 0u) {
      kernel_samples[i] = rt->last_gpu_duration_ns;
    }
  }
  loop_end_ns = prom_wall_clock_now_ns();

  out_result->iterations = iterations * diagnostic_batch_depth;
  out_result->total_loop_wall_ns = prom_wall_clock_elapsed_ns(loop_begin_ns, loop_end_ns);
  out_result->dispatch_submit_wall_ns_median = prom_percentile_u64(submit_samples, iterations, 50u, 100u);
  out_result->sync_wait_wall_ns_median = prom_percentile_u64(wait_samples, iterations, 50u, 100u);
  out_result->query_result_wall_ns_median = prom_percentile_u64(query_samples, iterations, 50u, 100u);
  out_result->diagnostic_batch_depth = diagnostic_batch_depth;
  out_result->queue_submissions = iterations;
  out_result->fence_waits = iterations;
  out_result->command_buffer_recordings = iterations;
  out_result->command_buffer_resets = iterations;
  out_result->descriptor_updates = iterations;
  out_result->gpu_timing_failure_reason = rt->last_gpu_timing_failure_reason;
  out_result->gpu_timestamp_valid = 1u;
resident_timing_complete:
  out_result->gpu_timing_failure_reason = rt->last_gpu_timing_failure_reason;
  out_result->gpu_timestamp_valid = 1u;
  for (i = 0u; i < iterations; ++i) {
    if (kernel_samples[i] == 0u) {
      out_result->gpu_timestamp_valid = 0u;
      break;
    }
  }
  if (out_result->gpu_timestamp_valid != 0u) {
    out_result->kernel_min_ns = prom_percentile_u64(kernel_samples, iterations, 0u, 100u);
    out_result->kernel_median_ns = prom_percentile_u64(kernel_samples, iterations, 50u, 100u);
    out_result->kernel_p95_ns = prom_percentile_u64(kernel_samples, iterations, 95u, 100u);
    out_result->gpu_timing_failure_reason = PROM_SGEMM_GPU_TIMING_FAILURE_NONE;
  }

  if ((request->flags & PROM_SGEMM_RESIDENT_FLAG_VALIDATE_READBACK) != 0u) {
    readback_begin_ns = prom_wall_clock_now_ns();
    status = prom_sgemm_resident_readback_once(rt, request->c, c_copy_size, &stage, &detail_code);
    readback_end_ns = prom_wall_clock_now_ns();
    out_result->readback_once_wall_ns = prom_wall_clock_elapsed_ns(readback_begin_ns, readback_end_ns);
    out_result->validation_wall_ns = out_result->readback_once_wall_ns;
    if (status != PROM_OK) {
      out_result->final_stage = stage;
      out_result->final_detail_code = detail_code;
      free(scratch_c);
      free(kernel_samples);
      free(submit_samples);
      free(wait_samples);
      free(query_samples);
      return status;
    }
  }

  out_result->final_stage = PROM_STAGE_SUBMIT;
  out_result->final_detail_code = PROM_DETAIL_PATH_STAGED_UPLOAD_TILED;
  free(scratch_c);
  free(kernel_samples);
  free(submit_samples);
  free(wait_samples);
  free(query_samples);
  return PROM_OK;
}

// ============================================================================
// SGEMM Batch Dispatch / Worker Runtime
// ============================================================================

/* The public v1 word is intentionally narrower than its historical namespace.
   Low eight bits are a logical planning width; bit 8 is the partition policy.
   Every other retained value is a tombstone or an explicit test control. */
#define PROM_BATCH_PRODUCTION_FLAG_MASK (0xffu | PROM_BATCH_FLAG_PARTITION_CONTIGUOUS)

static int prom_sgemm_batch_reject_unsupported_options(prometheus_runtime* rt,
                                                       uint32_t entry_count,
                                                       uint32_t flags,
                                                       uint32_t* out_stage,
                                                       int* out_detail_code) {
  const uint32_t unsupported = flags & ~PROM_BATCH_PRODUCTION_FLAG_MASK;
  memset(&rt->batch_diag, 0, sizeof(rt->batch_diag));
  rt->batch_diag.last_batch_entry_count = entry_count;
  rt->batch_diag.requested_workers = (flags & 0xffu) == 0u ? 1u : (flags & 0xffu);
  rt->batch_diag.effective_workers = rt->batch_diag.requested_workers > 8u
                                       ? 8u
                                       : rt->batch_diag.requested_workers;
  if (entry_count != 0u && rt->batch_diag.effective_workers > entry_count) {
    rt->batch_diag.effective_workers = entry_count;
  }
  rt->batch_diag.partition_policy = (flags & PROM_BATCH_FLAG_PARTITION_CONTIGUOUS) != 0u
                                      ? PROM_BATCH_PARTITION_CONTIGUOUS
                                      : PROM_BATCH_PARTITION_ROUND_ROBIN;
  rt->batch_diag.plan_generation = 1u;
  rt->batch_diag.batch_state = PROM_BATCH_STATE_FAILED;
  rt->batch_diag.failure_stage = PROM_STAGE_INIT;
  rt->batch_diag.failure_detail = PROM_DETAIL_BATCH_UNSUPPORTED_OPTION;
  rt->batch_diag.failure_count = 1u;
  /* No plan, task, ring slot, submit, completion, or caller-output mutation. */
  (void)unsupported;
  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_DETAIL_BATCH_UNSUPPORTED_OPTION);
  return PROM_ERROR;
}

int prom_reactor_runtime_sgemm_batch_impl(void* handle,
                                          const PrometheusSgemmBatchEntry* entries,
                                          uint32_t entry_count,
                                          uint32_t flags,
                                          uint32_t* out_stage,
                                          int* out_detail_code) {
  prometheus_runtime* rt;
  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_NONE, 0);
  if (handle == NULL || !registry_contains(handle)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  if ((flags & ~PROM_BATCH_PRODUCTION_FLAG_MASK) != 0u) {
    return prom_sgemm_batch_reject_unsupported_options(rt, entry_count, flags, out_stage, out_detail_code);
  }
  if (rt->vulkan.available == 0u || rt->submission_ring_diag.configured_depth == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_DETAIL_BATCH_EXECUTION_FAILED);
    return PROM_ERROR;
  }
  return prom_sgemm_batch_execute(rt, entries, entry_count, flags, out_stage, out_detail_code);
}

int prom_reactor_runtime_sgemm_batch_m31_test_impl(void* handle,
                                                   const PrometheusSgemmBatchEntry* entries,
                                                   uint32_t entry_count,
                                                   uint32_t flags,
                                                   uint32_t* out_stage,
                                                   int* out_detail_code) {
  prometheus_runtime* rt;
  if (handle == NULL || !registry_contains(handle)) return PROM_INVALID_HANDLE;
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) return PROM_INVALID_HANDLE;
  return prom_sgemm_batch_execute(rt, entries, entry_count, flags, out_stage, out_detail_code);
}

// ============================================================================
// SGEMM Async Lifecycle
// ============================================================================

static int prom_async_task_id_encode(uint32_t index, uint32_t generation, int* out_id) {
  /* Four table bits leave 27 generation bits and keep public IDs positive. */
  uint32_t id = ((generation & 0x07ffffffu) << 4u) | (index + 1u);
  if (out_id == NULL || id == 0u || (id & 0x80000000u) != 0u) return 0;
  *out_id = (int)id;
  return 1;
}

static prom_sgemm_async_task* prom_async_task_lookup(prometheus_runtime* rt, int task_id) {
  uint32_t low, generation;
  prom_sgemm_async_task* task;
  if (rt == NULL || task_id <= 0) return NULL;
  low = ((uint32_t)task_id & 0x0fu);
  generation = ((uint32_t)task_id >> 4u) & 0x07ffffffu;
  if (low == 0u || low > PROM_SGEMM_ASYNC_MAX_TASKS) return NULL;
  task = &rt->async_tasks[low - 1u];
  if (task->active == 0u || (task->generation & 0x07ffffffu) != generation || task->public_task_id != task_id) return NULL;
  return task;
}

static void prom_async_task_destroy_buffers(prometheus_runtime* rt, prom_sgemm_async_task* task) {
  if (rt == NULL || task == NULL) return;
  prom_vk_destroy_buffer(rt->vulkan.device, &task->c);
  prom_vk_destroy_buffer(rt->vulkan.device, &task->b);
  prom_vk_destroy_buffer(rt->vulkan.device, &task->a);
}

void prom_async_task_release(prometheus_runtime* rt, prom_sgemm_async_task* task) {
  uint32_t generation;
  if (rt == NULL || task == NULL) return;
  generation = task->generation;
  prom_async_task_destroy_buffers(rt, task);
  memset(task, 0, sizeof(*task));
  task->generation = generation;
}

prom_sgemm_async_task* prom_async_task_allocate(prometheus_runtime* rt) {
  uint32_t offset;
  if (rt == NULL) return NULL;
  for (offset = 0u; offset < PROM_SGEMM_ASYNC_MAX_TASKS; ++offset) {
    uint32_t index = (rt->async_task_cursor + offset) % PROM_SGEMM_ASYNC_MAX_TASKS;
    prom_sgemm_async_task* task = &rt->async_tasks[index];
    /* A record is never retired merely because the public caller is done.
       Quarantined slots retain task-owned buffers until the fence is known. */
    if (task->active != 0u && task->lifecycle_state == PROM_ASYNC_STATE_CONSUMED &&
        task->feedback_pending == 0u && task->reap_pending == 0u) prom_async_task_release(rt, task);
    if (task->active == 0u) {
      uint32_t generation = task->generation + 1u;
      memset(task, 0, sizeof(*task));
      task->active = 1u; task->table_index = index; task->generation = generation == 0u ? 1u : generation;
      task->lifecycle_state = PROM_ASYNC_STATE_IDLE;
      if (!prom_async_task_id_encode(index, task->generation, &task->public_task_id)) { task->active = 0u; return NULL; }
      rt->async_task_cursor = (index + 1u) % PROM_SGEMM_ASYNC_MAX_TASKS;
      return task;
    }
  }
  return NULL;
}

static void prom_async_count_quarantine(prometheus_runtime* rt) {
  uint32_t count = 0u;
  if (rt == NULL) return;
  for (uint32_t i = 0u; i < PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH; ++i) {
    if (rt->submission_ring[i].state == PROM_SGEMM_SUBMISSION_SLOT_QUARANTINED) count += 1u;
  }
  if (count > rt->async_max_quarantine_depth) rt->async_max_quarantine_depth = count;
}

static void prom_async_fail_task(prometheus_runtime* rt, prom_sgemm_async_task* task,
                                 uint32_t failure_class, uint32_t stage, int detail,
                                 uint32_t completion_known) {
  prom_sgemm_submission_slot* slot;
  if (rt == NULL || task == NULL) return;
  task->lifecycle_state = PROM_ASYNC_STATE_FAILED;
  task->failure_class = failure_class;
  task->final_stage = stage;
  task->final_detail = detail != 0 ? detail : PROM_DETAIL_ASYNC_FAILED;
  task->feedback_pending = 1u;
  task->physical_completion_confirmed = completion_known;
  if (task->physical_slot_id >= PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH) return;
  slot = &rt->submission_ring[task->physical_slot_id];
  if (completion_known == 0u && slot->generation == task->physical_slot_generation) {
    if (slot->state != PROM_SGEMM_SUBMISSION_SLOT_QUARANTINED) rt->async_quarantine_event_count += 1u;
    slot->state = PROM_SGEMM_SUBMISSION_SLOT_QUARANTINED;
    task->slot_quarantined = 1u;
    task->reap_pending = 1u;
    prom_async_count_quarantine(rt);
  }
  if (failure_class == PROM_ASYNC_FAILURE_DEVICE_LOST) rt->async_runtime_unsafe_to_reuse = 1u;
}

static prom_sgemm_async_task* prom_async_task_for_slot(prometheus_runtime* rt, const prom_sgemm_submission_slot* slot) {
  if (rt == NULL || slot == NULL) return NULL;
  for (uint32_t i = 0u; i < PROM_SGEMM_ASYNC_MAX_TASKS; ++i) {
    prom_sgemm_async_task* task = &rt->async_tasks[i];
    if (task->active != 0u && task->physical_slot_id == slot->slot_id &&
        task->physical_slot_generation == slot->generation) return task;
  }
  return NULL;
}

/* Reaping is the sole path that returns an observation-failed slot to EMPTY.
   Non-blocking callers never wait; destruction can request an explicit drain. */
int prom_async_reap_quarantined_slots(prometheus_runtime* rt, uint32_t allow_wait) {
  if (rt == NULL) return PROM_ERROR;
  for (uint32_t i = 0u; i < PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH; ++i) {
    prom_sgemm_submission_slot* slot = &rt->submission_ring[i];
    prom_sgemm_async_task* task;
    VkResult result;
    if (slot->state != PROM_SGEMM_SUBMISSION_SLOT_QUARANTINED) continue;
    task = prom_async_task_for_slot(rt, slot);
    rt->async_reap_poll_count += 1u;
    result = vkGetFenceStatus(rt->vulkan.device, slot->fence);
    if (result == VK_NOT_READY && allow_wait != 0u) {
      rt->async_reap_wait_count += 1u;
      result = vkWaitForFences(rt->vulkan.device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
    }
    if (result == VK_NOT_READY) continue;
    if (result != VK_SUCCESS) {
      rt->async_reap_failure_count += 1u;
      if (task != NULL) prom_async_fail_task(rt, task,
          result == VK_ERROR_DEVICE_LOST ? PROM_ASYNC_FAILURE_DEVICE_LOST : PROM_ASYNC_FAILURE_OBSERVATION,
          PROM_STAGE_SUBMIT, (int)result, 0u);
      if (result == VK_ERROR_DEVICE_LOST) {
        slot->state = PROM_SGEMM_SUBMISSION_SLOT_FAILED_FATAL;
        rt->async_runtime_unsafe_to_reuse = 1u;
      }
      continue;
    }
    slot->physical_completion_confirmed = 1u;
    if (task != NULL) {
      task->physical_completion_confirmed = 1u;
      task->slot_quarantined = 0u;
      task->reap_pending = 0u;
      task->reap_completed = 1u;
      /* An observation failure already emitted its one skipped feedback event.
         Query results are deliberately discarded: timing cannot repair it. */
      if (task->abandoned != 0u) {
        prom_async_task_destroy_buffers(rt, task);
        task->lifecycle_state = PROM_ASYNC_STATE_CONSUMED;
        task->active = 0u;
      }
    }
    /* Quarantine was entered from a submitted slot whose normal poll path did
       not retire the ring counter.  Reap is its one authoritative retirement
       transition; without this decrement a later reusable batch reports
       phantom in-flight work. */
    if (rt->submission_ring_diag.outstanding != 0u) rt->submission_ring_diag.outstanding -= 1u;
    slot->state = PROM_SGEMM_SUBMISSION_SLOT_EMPTY;
    rt->async_reap_success_count += 1u;
  }
  return PROM_OK;
}

int prom_async_record_slot(prometheus_runtime* rt, prom_sgemm_submission_slot* slot, prom_sgemm_async_task* task) {
  VkWriteDescriptorSet writes[3]; VkDescriptorBufferInfo infos[3]; VkCommandBufferBeginInfo begin; VkBufferMemoryBarrier barriers[3];
  VkResult result; prom_vk_push push; prom_sgemm_dispatch_geometry geometry;
  if (rt == NULL || slot == NULL || task == NULL || slot->state != PROM_SGEMM_SUBMISSION_SLOT_PREPARING) return PROM_ERROR;
  memset(infos, 0, sizeof(infos));
  infos[0].buffer=task->a.buffer; infos[0].range=task->a.size;
  infos[1].buffer=task->b.buffer; infos[1].range=task->b.size;
  infos[2].buffer=task->c.buffer; infos[2].range=task->c.size;
  memset(writes, 0, sizeof(writes));
  for (uint32_t i=0u;i<3u;++i) { writes[i].sType=VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET; writes[i].dstSet=slot->descriptor_set; writes[i].dstBinding=i; writes[i].descriptorType=VK_DESCRIPTOR_TYPE_STORAGE_BUFFER; writes[i].descriptorCount=1u; writes[i].pBufferInfo=&infos[i]; }
  vkUpdateDescriptorSets(rt->vulkan.device, 3u, writes, 0u, NULL);
  result=vkResetCommandBuffer(slot->command_buffer,0u); if(result!=VK_SUCCESS) goto failed;
  memset(&begin,0,sizeof(begin)); begin.sType=VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  result=vkBeginCommandBuffer(slot->command_buffer,&begin); if(result!=VK_SUCCESS) goto failed;
  memset(barriers,0,sizeof(barriers));
  for(uint32_t i=0u;i<3u;++i){ barriers[i].sType=VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER; barriers[i].srcAccessMask=VK_ACCESS_HOST_WRITE_BIT; barriers[i].dstAccessMask=i==2u?VK_ACCESS_SHADER_WRITE_BIT:VK_ACCESS_SHADER_READ_BIT; barriers[i].srcQueueFamilyIndex=VK_QUEUE_FAMILY_IGNORED; barriers[i].dstQueueFamilyIndex=VK_QUEUE_FAMILY_IGNORED; barriers[i].buffer=infos[i].buffer; barriers[i].size=infos[i].range; }
  vkCmdPipelineBarrier(slot->command_buffer,VK_PIPELINE_STAGE_HOST_BIT,VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,0,0,NULL,3u,barriers,0,NULL);
  vkCmdBindPipeline(slot->command_buffer,VK_PIPELINE_BIND_POINT_COMPUTE,rt->pipeline);
  vkCmdBindDescriptorSets(slot->command_buffer,VK_PIPELINE_BIND_POINT_COMPUTE,rt->pipeline_layout,0u,1u,&slot->descriptor_set,0u,NULL);
  push.m=task->m; push.n=task->n; push.k=task->k; vkCmdPushConstants(slot->command_buffer,rt->pipeline_layout,VK_SHADER_STAGE_COMPUTE_BIT,0u,PROM_VK_SHADER_PUSH_BYTES,&push);
  if(rt->timestamp_query_supported!=0u){vkCmdResetQueryPool(slot->command_buffer,rt->sgemm_timestamp_query_pool,slot->query_base,2u);vkCmdWriteTimestamp(slot->command_buffer,VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,rt->sgemm_timestamp_query_pool,slot->query_base);}
  geometry=prom_sgemm_dispatch_geometry_for_variant(0u,task->m,task->n); vkCmdDispatch(slot->command_buffer,geometry.groups_x,geometry.groups_y,geometry.groups_z);
  if(rt->timestamp_query_supported!=0u)vkCmdWriteTimestamp(slot->command_buffer,VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,rt->sgemm_timestamp_query_pool,slot->query_base+1u);
  result=vkEndCommandBuffer(slot->command_buffer);
  if ((rt->vulkan.test_flags & PROM_TESTCFG_FAIL_COMMAND_END) != 0u) result = VK_ERROR_INITIALIZATION_FAILED;
  if(result!=VK_SUCCESS)goto failed;
  slot->m=task->m;slot->n=task->n;slot->compute_k=task->k;slot->compute_mode=PROM_VK_COMPUTE_BASELINE;slot->variant=0u;slot->state=PROM_SGEMM_SUBMISSION_SLOT_RECORDED;return PROM_OK;
failed: slot->state=PROM_SGEMM_SUBMISSION_SLOT_FAILED;slot->failure_stage=PROM_STAGE_SUBMIT;slot->failure_detail=(int)result;return PROM_ERROR;
}

static uint32_t prom_async_outstanding(const prometheus_runtime* rt);

int prom_async_poll_task(prometheus_runtime* rt, prom_sgemm_async_task* task) {
  prom_sgemm_submission_slot* slot; int status;
  if(rt==NULL||task==NULL||task->lifecycle_state!=PROM_ASYNC_STATE_SUBMITTED)return PROM_OK;
  if ((rt->async_test_flags & PROM_ASYNC_TESTCFG_DEVICE_LOST_AFTER_SUBMIT) != 0u) {
    /* Deterministically run the device-lost ownership branch only after at
       least one real submission exists.  We do not pretend the driver lost
       the device; this is an internal classification seam. */
    rt->async_test_flags &= ~PROM_ASYNC_TESTCFG_DEVICE_LOST_AFTER_SUBMIT;
    for (uint32_t i = 0u; i < PROM_SGEMM_ASYNC_MAX_TASKS; ++i) {
      prom_sgemm_async_task* pending = &rt->async_tasks[i];
      if (pending->active != 0u && pending->lifecycle_state == PROM_ASYNC_STATE_SUBMITTED) {
        prom_sgemm_submission_slot* pending_slot = &rt->submission_ring[pending->physical_slot_id];
        prom_async_fail_task(rt, pending, PROM_ASYNC_FAILURE_DEVICE_LOST, PROM_STAGE_SUBMIT, VK_ERROR_DEVICE_LOST, 0u);
        pending_slot->state = PROM_SGEMM_SUBMISSION_SLOT_FAILED_FATAL;
        pending->slot_quarantined = 0u;
        pending->reap_pending = 0u;
      }
    }
    return PROM_ERROR;
  }
  if((rt->vulkan.test_flags&PROM_TESTCFG_FAIL_ASYNC_POLL)!=0u){
    rt->vulkan.test_flags &= ~PROM_TESTCFG_FAIL_ASYNC_POLL;
    prom_async_fail_task(rt,task,PROM_ASYNC_FAILURE_OBSERVATION,PROM_STAGE_SUBMIT,
                         PROM_DETAIL_INJECTED_ASYNC_POLL_FAILURE,0u);
    return PROM_ERROR;
  }
  if(task->physical_slot_id>=PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH)return PROM_ERROR;
  slot=&rt->submission_ring[task->physical_slot_id];
  if(slot->generation!=task->physical_slot_generation){
    prom_async_fail_task(rt,task,PROM_ASYNC_FAILURE_OBSERVATION,PROM_STAGE_SUBMIT,
                         PROM_DETAIL_SLOT_ASYNC_OWNERSHIP,0u);
    return PROM_ERROR;
  }
  status=prom_sgemm_ring_poll_slot(rt,slot);
  if(status!=PROM_OK||slot->state==PROM_SGEMM_SUBMISSION_SLOT_FAILED){
    uint32_t known = slot->state == PROM_SGEMM_SUBMISSION_SLOT_FAILED && slot->physical_completion_confirmed != 0u;
    prom_async_fail_task(rt,task,PROM_ASYNC_FAILURE_QUERY,PROM_STAGE_SUBMIT,
                         slot->failure_detail!=0?slot->failure_detail:PROM_DETAIL_ASYNC_FAILED,known);
    return PROM_ERROR;
  }
  if(slot->state==PROM_SGEMM_SUBMISSION_SLOT_READY){
    task->lifecycle_state=PROM_ASYNC_STATE_READY;task->output_ready=1u;
    task->physical_completion_confirmed=1u;slot->physical_completion_confirmed=1u;
    task->timing_valid=slot->timing_valid;task->gpu_duration_ns=slot->gpu_duration_ns;task->feedback_pending=1u;
  }
  return PROM_OK;
}

/* P14 receives only valid measured duration.  P15 receives the filtered
   evidence derived from that same immutable completion attribution. */
static void prom_sgemm_commit_completion_evidence(prometheus_runtime* rt, prom_sgemm_async_task* task) {
  prom_dominatus_predictor_evidence evidence;
  prom_dominatus_physical_observation observation;
  if (rt == NULL || task == NULL || task->feedback_committed != 0u) return;
  if (task->timing_valid == 0u || task->gpu_duration_ns == 0u || task->lifecycle_state == PROM_ASYNC_STATE_FAILED) {
    task->feedback_committed = 1u;
    rt->async_feedback_skipped_count += 1u;
    return;
  }
  rt->p14_measurement_tick += 1u;
  rt->p14_last_filtered_evidence = prom_dominatus_measurement_filter_update(
      &rt->p14_measurement_filter_state, (double)task->gpu_duration_ns, rt->p14_measurement_tick);
  evidence = prom_dominatus_predictor_evidence_from_filtered(&rt->p14_last_filtered_evidence);
  memset(&observation, 0, sizeof(observation));
  observation.tick = rt->p14_measurement_tick;
  observation.actual_ready = 1u;
  observation.slot_valid = 1u;
  observation.memory_budget_ok = 1u;
  observation.outstanding_depth = prom_async_outstanding(rt);
  observation.outstanding_depth_cap = rt->p15_predictor_state.params.max_outstanding_depth;
  memset(&rt->p15_last_prediction_issued, 0, sizeof(rt->p15_last_prediction_issued));
  rt->p15_last_correction = prom_dominatus_predictor_update(
      &rt->p15_predictor_state, &evidence, &observation, observation.tick, &rt->p15_last_prediction_issued);
  (void)prom_dominatus_predictor_advance_reservations(&rt->p15_predictor_state, observation.tick);
  task->feedback_committed = 1u;
  rt->async_feedback_committed_count += 1u;
}

void prom_async_process_completion_feedback(prometheus_runtime* rt) {
  uint32_t progress = 1u;
  if (rt == NULL) return;
  while (progress != 0u) {
    prom_sgemm_async_task* candidate = NULL;
    progress = 0u;
    for (uint32_t i = 0u; i < PROM_SGEMM_ASYNC_MAX_TASKS; ++i) {
      prom_sgemm_async_task* task = &rt->async_tasks[i];
      if (task->active != 0u && task->submission_sequence == rt->async_next_feedback_sequence) { candidate = task; break; }
    }
    if (candidate == NULL || candidate->feedback_pending == 0u) break;
    if (candidate->lifecycle_state != PROM_ASYNC_STATE_READY && candidate->lifecycle_state != PROM_ASYNC_STATE_FAILED &&
        candidate->lifecycle_state != PROM_ASYNC_STATE_CONSUMED) break;
    prom_sgemm_commit_completion_evidence(rt, candidate);
    candidate->feedback_pending = 0u;
    rt->async_next_feedback_sequence += 1u;
    progress = 1u;
  }
}

static uint32_t prom_async_outstanding(const prometheus_runtime* rt) { uint32_t n=0u; for(uint32_t i=0u;i<PROM_SGEMM_ASYNC_MAX_TASKS;++i)if(rt->async_tasks[i].active!=0u&&rt->async_tasks[i].lifecycle_state!=PROM_ASYNC_STATE_CONSUMED)n++; return n; }

int prom_reactor_runtime_sgemm_submit_async_impl(void* handle,
                                                 const float* a,
                                                 const float* b,
                                                 uint32_t m,
                                                 uint32_t n,
                                                 uint32_t k,
                                                 int* out_task_id,
                                                 uint32_t* out_stage,
                                                 int* out_detail_code) {
  prometheus_runtime* rt;
  prom_sgemm_async_task* task; prom_sgemm_submission_slot* slot; uint32_t a_count,b_count,c_count; VkResult result;

  if (out_task_id == NULL) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_ERROR);
    return PROM_ERROR;
  }
  if (handle == NULL || !registry_contains(handle)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }

  rt = (prometheus_runtime*)handle;
  if(rt->magic!=PROMETHEUS_RUNTIME_MAGIC||rt->vulkan.available==0u||a==NULL||b==NULL||m==0u||n==0u||k==0u||!prom_vk_checked_mul_u32(m,k,&a_count)||!prom_vk_checked_mul_u32(k,n,&b_count)||!prom_vk_checked_mul_u32(m,n,&c_count)){prom_vk_set_status(out_stage,out_detail_code,PROM_STAGE_INIT,PROM_ERROR);return PROM_ERROR;}
  if (rt->async_runtime_unsafe_to_reuse != 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_ASYNC_FAILED);
    return PROM_ERROR;
  }
  /* A failed public task is sticky for its owner, not a global admission lock.
     Its physical slot remains unavailable until this non-blocking reap sees a
     signaled fence. */
  (void)prom_async_reap_quarantined_slots(rt, 0u);
  for(uint32_t i=0u;i<PROM_SGEMM_SUBMISSION_RING_DEFAULT_DEPTH;++i) (void)prom_sgemm_ring_poll_slot(rt,&rt->submission_ring[i]);
  task=prom_async_task_allocate(rt); if(task==NULL){rt->async_queue_full_count++;prom_vk_set_status(out_stage,out_detail_code,PROM_STAGE_SUBMIT,PROM_DETAIL_ASYNC_QUEUE_FULL);return PROM_ERROR;}
  slot=NULL; for(uint32_t i=0u;i<PROM_SGEMM_SUBMISSION_RING_DEFAULT_DEPTH;++i)if(rt->submission_ring[i].state==PROM_SGEMM_SUBMISSION_SLOT_EMPTY){slot=&rt->submission_ring[i];break;}
  if(slot==NULL){prom_async_task_release(rt,task);rt->async_queue_full_count++;prom_vk_set_status(out_stage,out_detail_code,PROM_STAGE_SUBMIT,PROM_DETAIL_ASYNC_QUEUE_FULL);return PROM_ERROR;}
  slot->state=PROM_SGEMM_SUBMISSION_SLOT_PREPARING;slot->generation+=1u;slot->submission_sequence=rt->submission_ring_diag.next_sequence++;slot->physical_completion_confirmed=0u;
  task->m=m;task->n=n;task->k=k;task->compute_k=k;task->physical_slot_id=slot->slot_id;task->physical_slot_generation=slot->generation;task->submission_sequence=rt->async_next_submission_sequence++;task->selected_path=PROM_VK_PATH_DIRECT;task->compute_mode=PROM_VK_COMPUTE_BASELINE;task->final_detail=PROM_DETAIL_PATH_DIRECT;
  result=prom_vk_create_buffer(rt->vulkan.physical_device,rt->vulkan.device,rt->vulkan.test_flags,(VkDeviceSize)a_count*sizeof(float),VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT|VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,1,&task->a); if(result==VK_SUCCESS)result=prom_vk_create_buffer(rt->vulkan.physical_device,rt->vulkan.device,rt->vulkan.test_flags,(VkDeviceSize)b_count*sizeof(float),VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT|VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,1,&task->b); if(result==VK_SUCCESS)result=prom_vk_create_buffer(rt->vulkan.physical_device,rt->vulkan.device,rt->vulkan.test_flags,(VkDeviceSize)c_count*sizeof(float),VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT|VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,1,&task->c);
  if(result!=VK_SUCCESS){slot->state=PROM_SGEMM_SUBMISSION_SLOT_EMPTY;prom_async_task_release(rt,task);prom_vk_set_status(out_stage,out_detail_code,PROM_STAGE_TRANSFER_IN,(int)result);return PROM_ERROR;}
  memcpy(task->a.mapped,a,(size_t)a_count*sizeof(float));memcpy(task->b.mapped,b,(size_t)b_count*sizeof(float));memset(task->c.mapped,0,(size_t)c_count*sizeof(float));
  if(prom_async_record_slot(rt,slot,task)!=PROM_OK||prom_sgemm_ring_submit_slot(rt,slot)!=PROM_OK){slot->state=PROM_SGEMM_SUBMISSION_SLOT_EMPTY;prom_async_task_release(rt,task);prom_vk_set_status(out_stage,out_detail_code,PROM_STAGE_SUBMIT,PROM_ERROR);return PROM_ERROR;}
  task->lifecycle_state=PROM_ASYNC_STATE_SUBMITTED; *out_task_id=task->public_task_id; rt->async_task_id=*out_task_id;rt->async_state=PROM_ASYNC_STATE_SUBMITTED;
  prom_vk_set_status(out_stage,out_detail_code,PROM_STAGE_SUBMIT,0);
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_query_async_impl(void* handle, int task_id, PrometheusAsyncStatus* out_status) {
  prometheus_runtime* rt;
  prom_sgemm_async_task* task;

  if (out_status == NULL) {
    return PROM_ERROR;
  }
  memset(out_status, 0, sizeof(*out_status));

  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    return PROM_INVALID_HANDLE;
  }
  (void)prom_async_reap_quarantined_slots(rt, 0u);
  task=prom_async_task_lookup(rt,task_id); if(task==NULL) {
    out_status->lifecycle_state = PROM_ASYNC_STATE_IDLE;
    out_status->detail_code = PROM_DETAIL_ASYNC_NO_TASK;
    rt->async_stale_reject_count++;
    return PROM_ERROR;
  }
  (void)prom_async_poll_task(rt,task); prom_async_process_completion_feedback(rt); out_status->lifecycle_state=task->lifecycle_state;out_status->stage=task->final_stage==0u?PROM_STAGE_SUBMIT:task->final_stage;out_status->detail_code=task->lifecycle_state==PROM_ASYNC_STATE_SUBMITTED?PROM_DETAIL_ASYNC_NOT_READY:task->final_detail;out_status->ready=task->lifecycle_state==PROM_ASYNC_STATE_READY;out_status->failed=task->lifecycle_state==PROM_ASYNC_STATE_FAILED;out_status->consumed=task->lifecycle_state==PROM_ASYNC_STATE_CONSUMED;out_status->outstanding_tasks=prom_async_outstanding(rt);
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_async_diagnostics_impl(void* handle, PrometheusSgemmAsyncDiagnostics* out_diag) {
  prometheus_runtime* rt;
  if (out_diag == NULL) return PROM_ERROR;
  memset(out_diag, 0, sizeof(*out_diag));
  if (handle == NULL || !registry_contains(handle)) return PROM_INVALID_HANDLE;
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) return PROM_INVALID_HANDLE;
  (void)prom_async_reap_quarantined_slots(rt, 0u);
  out_diag->task_capacity = PROM_SGEMM_ASYNC_MAX_TASKS;
  out_diag->queue_full_count = rt->async_queue_full_count;
  out_diag->stale_id_rejection_count = rt->async_stale_reject_count;
  out_diag->max_in_flight = rt->submission_ring_diag.max_outstanding;
  out_diag->feedback_committed_count = rt->async_feedback_committed_count;
  out_diag->feedback_skipped_count = rt->async_feedback_skipped_count;
  out_diag->next_feedback_sequence = rt->async_next_feedback_sequence;
  out_diag->quarantine_event_count = rt->async_quarantine_event_count;
  out_diag->reap_poll_count = rt->async_reap_poll_count;
  out_diag->reap_success_count = rt->async_reap_success_count;
  out_diag->reap_wait_count = rt->async_reap_wait_count;
  out_diag->reap_failure_count = rt->async_reap_failure_count;
  out_diag->max_quarantine_depth = rt->async_max_quarantine_depth;
  out_diag->runtime_unsafe_to_reuse = rt->async_runtime_unsafe_to_reuse;
  for (uint32_t i = 0u; i < PROM_SGEMM_ASYNC_MAX_TASKS; ++i) {
    const prom_sgemm_async_task* task = &rt->async_tasks[i];
    PrometheusSgemmAsyncTaskDiagnostics* dst = &out_diag->tasks[i];
    if (task->active == 0u) continue;
    out_diag->active_task_count += 1u;
    if (task->lifecycle_state == PROM_ASYNC_STATE_SUBMITTED) out_diag->submitted_count += 1u;
    if (task->lifecycle_state == PROM_ASYNC_STATE_READY) out_diag->ready_count += 1u;
    if (task->lifecycle_state == PROM_ASYNC_STATE_FAILED) out_diag->failed_count += 1u;
    if (task->lifecycle_state == PROM_ASYNC_STATE_CONSUMED) out_diag->consumed_count += 1u;
    if (task->abandoned != 0u) out_diag->abandoned_count += 1u;
    if (task->slot_quarantined != 0u) out_diag->quarantined_slot_count += 1u;
    if (task->feedback_pending != 0u) out_diag->feedback_pending_count += 1u;
    dst->task_id = task->public_task_id; dst->generation = task->generation; dst->lifecycle_state = task->lifecycle_state;
    dst->physical_slot_id = task->physical_slot_id; dst->physical_slot_generation = task->physical_slot_generation;
    dst->submission_sequence = task->submission_sequence; dst->timing_valid = task->timing_valid;
    dst->gpu_duration_ns = task->gpu_duration_ns; dst->feedback_pending = task->feedback_pending;
    dst->feedback_committed = task->feedback_committed; dst->failure_detail = task->final_detail;
    dst->failure_class = task->failure_class; dst->abandoned = task->abandoned;
    dst->physical_slot_state = task->physical_slot_id < PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH ? rt->submission_ring[task->physical_slot_id].state : PROM_ASYNC_PHYSICAL_FAILED;
    dst->quarantined = task->slot_quarantined; dst->physical_completion_confirmed = task->physical_completion_confirmed;
    dst->reap_pending = task->reap_pending; dst->reap_completed = task->reap_completed;
  }
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_consume_async_impl(void* handle,
                                                  int task_id,
                                                  float* c,
                                                  uint32_t c_len,
                                                  uint32_t* out_stage,
                                                  int* out_detail_code) {
  prometheus_runtime* rt;
  uint32_t required_len; prom_sgemm_async_task* task;

  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_NONE, 0);
  if (handle == NULL || !registry_contains(handle)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  (void)prom_async_reap_quarantined_slots(rt, 0u);
  task=prom_async_task_lookup(rt,task_id); if(task==NULL) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_ASYNC_INVALID_TASK);
    stage_commit_async_snapshot(rt, PROM_DOM_EVENT_ASYNC_INVALID_TASK, PROM_DETAIL_ASYNC_INVALID_TASK);
    return PROM_ERROR;
  }
  (void)prom_async_poll_task(rt,task); prom_async_process_completion_feedback(rt); if (task->lifecycle_state == PROM_ASYNC_STATE_SUBMITTED) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_ASYNC_NOT_READY);
    stage_commit_async_snapshot(rt, PROM_DOM_EVENT_ASYNC_NOT_READY, PROM_DETAIL_ASYNC_NOT_READY);
    return PROM_ERROR;
  }
  if (task->lifecycle_state == PROM_ASYNC_STATE_FAILED) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, task->final_detail != 0 ? task->final_detail : PROM_DETAIL_ASYNC_FAILED);
    return PROM_ERROR;
  }
  if (task->lifecycle_state == PROM_ASYNC_STATE_CONSUMED) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_DETAIL_ASYNC_ALREADY_CONSUMED);
    stage_commit_async_snapshot(rt, PROM_DOM_EVENT_ASYNC_ALREADY_CONSUMED, PROM_DETAIL_ASYNC_ALREADY_CONSUMED);
    return PROM_ERROR;
  }
  if (c == NULL) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_ERROR);
    return PROM_ERROR;
  }
  required_len = task->m * task->n;
  if (c_len < required_len) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_ERROR);
    return PROM_ERROR;
  }

  memcpy(c,task->c.mapped,(size_t)required_len*sizeof(float)); task->consumed=1u;task->lifecycle_state=PROM_ASYNC_STATE_CONSUMED;
  rt->submission_ring[task->physical_slot_id].state=PROM_SGEMM_SUBMISSION_SLOT_EMPTY;
  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, task->final_detail);
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_abandon_async_impl(void* handle, int task_id) {
  prometheus_runtime* rt;
  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    return PROM_INVALID_HANDLE;
  }
  (void)prom_async_reap_quarantined_slots(rt, 0u);
  { prom_sgemm_async_task* task=prom_async_task_lookup(rt,task_id); if(task==NULL) {
    return PROM_ERROR;
  } (void)prom_async_poll_task(rt,task); prom_async_process_completion_feedback(rt); if (task->lifecycle_state == PROM_ASYNC_STATE_SUBMITTED) {
    rt->slot_diag.inflight_rejection_count += 1u;
    commit_slot_runtime_diag_snapshot(rt, PROM_DETAIL_ASYNC_UNCONSUMED);
    stage_commit_async_snapshot(rt, PROM_DOM_EVENT_ASYNC_UNCONSUMED_REJECTED, PROM_DETAIL_ASYNC_UNCONSUMED);
    return PROM_ERROR;
  }
  task->abandoned=1u;
  if (task->lifecycle_state == PROM_ASYNC_STATE_FAILED && task->reap_pending != 0u) {
    /* Logical abandonment releases caller interest only.  The task remains
       query-visible as FAILED until a later non-blocking reap retires it. */
    (void)prom_async_reap_quarantined_slots(rt, 0u);
    return PROM_OK;
  }
  rt->submission_ring[task->physical_slot_id].state=PROM_SGEMM_SUBMISSION_SLOT_EMPTY;
  task->lifecycle_state=PROM_ASYNC_STATE_CONSUMED;
  prom_async_process_completion_feedback(rt);
  if (task->feedback_pending == 0u) prom_async_task_release(rt,task);
  }
  return PROM_OK;
}

// ============================================================================
// SGEMM Diagnostics Export
// ============================================================================

int prom_reactor_runtime_p15_test_seed_matured_reservation_impl(void* handle,
                                                          uint32_t shape_class,
                                                          uint32_t variant_id,
                                                          uint64_t target_tick) {
  prom_dominatus_future_lease_request req;
  prom_dominatus_reservation_decision d;
  prometheus_runtime* rt;
  if (handle == NULL || !registry_contains(handle)) return PROM_INVALID_HANDLE;
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) return PROM_INVALID_HANDLE;
  if (rt->p15_shadow_canary_params.enabled == 0u) return PROM_ERROR;
  memset(&req, 0, sizeof(req));
  req.valid = 1u;
  req.request_id = target_tick != 0u ? target_tick : 1u;
  req.target_tick = target_tick;
  req.shape_class = shape_class;
  req.variant_id = variant_id;
  req.lookahead_depth = 1u;
  req.confidence = 0.95;
  d = prom_dominatus_reservation_request_from_future_lease(&rt->p15_predictor_state.reservations,
                                                            &rt->p15_predictor_state.reservation_params,
                                                            &req,
                                                            target_tick > 0u ? target_tick - 1u : 0u);
  if (d.reserved == 0u) return PROM_ERROR;
  d = prom_dominatus_reservation_mature(&rt->p15_predictor_state.reservations, target_tick);
  if (d.matured == 0u) return PROM_ERROR;
  rt->p15_shadow_authority_gate.valid = 1u;
  rt->p15_shadow_authority_gate.state = PROM_SHADOW_AUTHORITY_HEALTHY;
  rt->p15_shadow_authority_gate.authority_enabled = 1u;
  rt->p15_shadow_canary_state.healthy_margin_passed = 1u;
  rt->p15_shadow_canary_state.reason_binding_passed = 1u;
  return PROM_OK;
}

static int prom_reactor_runtime_sgemm_policy_diagnostics_fill(void* handle, PrometheusSgemmPolicyDiagnostics* out_diag) {
  const prom_sgemm_controller_defaults defaults = prom_sgemm_default_config();
  prom_dom_sgemm_m35_snapshot m35_snapshot;
  prom_dom_transfer_queue_snapshot transfer_snapshot;
  prom_dom_sgemm_path_compute_snapshot path_compute_snapshot;
  prom_dom_sgemm_layout_precision_snapshot layout_precision_snapshot;
  prom_dom_slot_commit_snapshot slot_snapshot;
  prom_dom_slot_runtime_diag_snapshot slot_diag_snapshot;
  prom_dom_slot_readiness_snapshot slot_readiness_snapshot;
  prom_dom_sgemm_resource_lease_snapshot lease_snapshot;
  prometheus_runtime* rt;
  if (out_diag == NULL) {
    return PROM_ERROR;
  }
  memset(out_diag, 0, sizeof(*out_diag));
  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    return PROM_INVALID_HANDLE;
  }

  out_diag->current_mode = (uint32_t)rt->sgemm_controller.policy_memory.current_mode;
  out_diag->lookahead = rt->sgemm_controller.lookahead;
  out_diag->outstanding_depth = rt->sgemm_controller.outstanding_depth;
  out_diag->chunk_size = rt->sgemm_controller.chunk_size;
  out_diag->chunk_min = defaults.chunk_min;
  out_diag->chunk_max = defaults.chunk_max;
  out_diag->waste_budget_units = defaults.waste_budget_units;
  out_diag->pending_waste_units = rt->sgemm_controller.pending_waste_units;
  out_diag->wasted_work_units_last = rt->sgemm_controller.wasted_work_units_last;
  out_diag->wasted_work_units_total = rt->sgemm_controller.wasted_work_units_total;
  out_diag->decision_count = rt->sgemm_controller.decision_count;
  out_diag->retreat_count = rt->sgemm_controller.retreat_count;
  out_diag->recovery_count = rt->sgemm_controller.recovery_count;
  out_diag->transition_count = rt->sgemm_controller.transition_count;
  out_diag->instability_count = rt->sgemm_controller.instability_count;
  out_diag->budget_depletion_count = rt->sgemm_controller.budget_depletion_count;
  out_diag->safe_mode_decisions = rt->sgemm_controller.safe_mode_decisions;
  out_diag->aggressive_mode_decisions = rt->sgemm_controller.aggressive_mode_decisions;
  out_diag->recovery_mode_decisions = rt->sgemm_controller.recovery_mode_decisions;
  out_diag->lag_early_warning_count = rt->sgemm_controller.lag_early_warning_count;
  out_diag->burst_dampening_count = rt->sgemm_controller.burst_dampening_count;
  out_diag->bound_violation_count = rt->sgemm_controller.bound_violation_count;
  if (prom_dom_sgemm_read_visible_layout_precision_diagnostics(&rt->blackboard, &layout_precision_snapshot) != 0u) {
    out_diag->packed4_selected_layout_format = layout_precision_snapshot.decision.packed4_selected_layout_format;
    out_diag->packed4_tail_count_last = layout_precision_snapshot.decision.packed4_tail_count_last;
    out_diag->packed4_tail_count_total = layout_precision_snapshot.decision.packed4_tail_count_total;
    out_diag->packed4_padded_lane_count_last = layout_precision_snapshot.decision.packed4_padded_lane_count_last;
    out_diag->packed4_padded_lane_count_total = layout_precision_snapshot.decision.packed4_padded_lane_count_total;
    out_diag->packed4_padding_waste_permille_last = layout_precision_snapshot.decision.packed4_padding_waste_permille_last;
    out_diag->packed4_mode_budget_denials = layout_precision_snapshot.decision.packed4_mode_budget_denials;
    out_diag->packed4_row_major_check_failures = layout_precision_snapshot.decision.packed4_row_major_check_failures;
    out_diag->packed4_selection_count = layout_precision_snapshot.decision.packed4_selection_count;
    out_diag->packed4_fallback_reason_padding_waste = layout_precision_snapshot.decision.packed4_fallback_reason_padding_waste;
    out_diag->packed4_fallback_reason_small_shape = layout_precision_snapshot.decision.packed4_fallback_reason_small_shape;
    out_diag->packed4_fallback_reason_capability_missing = layout_precision_snapshot.decision.packed4_fallback_reason_capability_missing;
    out_diag->packed4_fallback_reason_fallback_required = layout_precision_snapshot.decision.packed4_fallback_reason_fallback_required;
    out_diag->packed4_fallback_reason_mode_budget_denied = layout_precision_snapshot.decision.packed4_fallback_reason_mode_budget_denied;
    out_diag->fp16_max_absolute_error = layout_precision_snapshot.decision.fp16_max_absolute_error;
    out_diag->fp16_max_relative_error = layout_precision_snapshot.decision.fp16_max_relative_error;
    out_diag->fp16_aggregate_error = layout_precision_snapshot.decision.fp16_aggregate_error;
    out_diag->fp16_worst_case_element_index = layout_precision_snapshot.decision.fp16_worst_case_element_index;
    out_diag->fp16_k_error_growth = layout_precision_snapshot.decision.fp16_k_error_growth;
    out_diag->fp16_cancellation_risk = layout_precision_snapshot.decision.fp16_cancellation_risk;
    out_diag->fp16_tolerance_known = layout_precision_snapshot.decision.fp16_tolerance_known;
    out_diag->fp16_tolerance_pass = layout_precision_snapshot.decision.fp16_tolerance_pass;
    out_diag->fp16_fallback_reason_detail = layout_precision_snapshot.decision.fp16_fallback_reason_detail;
    out_diag->fp16_selected_candidate = layout_precision_snapshot.decision.fp16_selected_candidate;
  } else {
    out_diag->packed4_selected_layout_format = rt->sgemm_controller.packed4_selected_layout_format;
    out_diag->packed4_tail_count_last = rt->sgemm_controller.packed4_tail_count_last;
    out_diag->packed4_tail_count_total = rt->sgemm_controller.packed4_tail_count_total;
    out_diag->packed4_padded_lane_count_last = rt->sgemm_controller.packed4_padded_lane_count_last;
    out_diag->packed4_padded_lane_count_total = rt->sgemm_controller.packed4_padded_lane_count_total;
    out_diag->packed4_padding_waste_permille_last = rt->sgemm_controller.packed4_padding_waste_permille_last;
    out_diag->packed4_mode_budget_denials = rt->sgemm_controller.packed4_mode_budget_denials;
    out_diag->packed4_row_major_check_failures = rt->sgemm_controller.packed4_row_major_check_failures;
    out_diag->packed4_selection_count = rt->sgemm_controller.packed4_selection_count;
    out_diag->packed4_fallback_reason_padding_waste = rt->sgemm_controller.packed4_fallback_reason_padding_waste;
    out_diag->packed4_fallback_reason_small_shape = rt->sgemm_controller.packed4_fallback_reason_small_shape;
    out_diag->packed4_fallback_reason_capability_missing = rt->sgemm_controller.packed4_fallback_reason_capability_missing;
    out_diag->packed4_fallback_reason_fallback_required = rt->sgemm_controller.packed4_fallback_reason_fallback_required;
    out_diag->packed4_fallback_reason_mode_budget_denied = rt->sgemm_controller.packed4_fallback_reason_mode_budget_denied;
    out_diag->fp16_max_absolute_error = rt->sgemm_controller.fp16_max_absolute_error;
    out_diag->fp16_max_relative_error = rt->sgemm_controller.fp16_max_relative_error;
    out_diag->fp16_aggregate_error = rt->sgemm_controller.fp16_aggregate_error;
    out_diag->fp16_worst_case_element_index = rt->sgemm_controller.fp16_worst_case_element_index;
    out_diag->fp16_k_error_growth = rt->sgemm_controller.fp16_k_error_growth;
    out_diag->fp16_cancellation_risk = rt->sgemm_controller.fp16_cancellation_risk;
    out_diag->fp16_tolerance_known = rt->sgemm_controller.fp16_tolerance_known;
    out_diag->fp16_tolerance_pass = rt->sgemm_controller.fp16_tolerance_pass;
    out_diag->fp16_fallback_reason_detail = rt->sgemm_controller.fp16_fallback_reason_detail;
    out_diag->fp16_selected_candidate = rt->sgemm_controller.fp16_selected_candidate;
  }
  if (prom_dom_slot_read_visible_runtime_diag(&rt->blackboard, &slot_diag_snapshot) != 0u) {
    out_diag->m29_current_slot_id = slot_diag_snapshot.current_slot_id;
    out_diag->m29_next_slot_id = slot_diag_snapshot.next_slot_id;
    out_diag->m29_slot0_state = slot_diag_snapshot.slot_state[0];
    out_diag->m29_slot1_state = slot_diag_snapshot.slot_state[1];
    out_diag->m29_slot0_generation = slot_diag_snapshot.slot_generation[0];
    out_diag->m29_slot1_generation = slot_diag_snapshot.slot_generation[1];
    out_diag->m29_slot0_valid = slot_diag_snapshot.slot_valid[0];
    out_diag->m29_slot1_valid = slot_diag_snapshot.slot_valid[1];
    out_diag->m29_swap_count = slot_diag_snapshot.swap_count;
    out_diag->m29_max_wip_depth = slot_diag_snapshot.max_wip_depth;
    out_diag->m29_overwrite_rejection_count = slot_diag_snapshot.overwrite_rejection_count;
    out_diag->m29_stale_buffer_rejection_count = slot_diag_snapshot.stale_buffer_rejection_count;
    out_diag->m29_shape_invalidation_count = slot_diag_snapshot.shape_invalidation_count;
    out_diag->m29_layout_invalidation_count = slot_diag_snapshot.layout_invalidation_count;
    out_diag->m29_capacity_invalidation_count = slot_diag_snapshot.capacity_invalidation_count;
    out_diag->m14_a_invalidation_count = rt->slot_diag.m14_a_invalidation_count;
    out_diag->m14_b_invalidation_count = rt->slot_diag.m14_b_invalidation_count;
    out_diag->m14_c_invalidation_count = rt->slot_diag.m14_c_invalidation_count;
    out_diag->m14_a_reuse_count = rt->slot_diag.m14_a_reuse_count;
    out_diag->m14_b_reuse_count = rt->slot_diag.m14_b_reuse_count;
    out_diag->m14_c_reuse_count = rt->slot_diag.m14_c_reuse_count;
    out_diag->m14_false_invalidation_avoided_count = rt->slot_diag.m14_false_invalidation_avoided_count;
    out_diag->m14_capacity_invalidation_count = rt->slot_diag.m14_capacity_invalidation_count;
    out_diag->m14_layout_precision_invalidation_count = rt->slot_diag.m14_layout_precision_invalidation_count;
    out_diag->m14_a_last_invalidation_reason = rt->slot_diag.m14_a_last_invalidation_reason;
    out_diag->m14_b_last_invalidation_reason = rt->slot_diag.m14_b_last_invalidation_reason;
    out_diag->m14_c_last_invalidation_reason = rt->slot_diag.m14_c_last_invalidation_reason;
    out_diag->m29_inflight_rejection_count = slot_diag_snapshot.inflight_rejection_count;
    out_diag->m29_cleanup_success_count = slot_diag_snapshot.cleanup_success_count;
    out_diag->m29_failure_slot_id = slot_diag_snapshot.failure_slot_id;
    out_diag->m29_failure_reason = slot_diag_snapshot.failure_reason;

    rt->slot_diag.current_slot_id = slot_diag_snapshot.current_slot_id;
    rt->slot_diag.next_slot_id = slot_diag_snapshot.next_slot_id;
    rt->slot_diag.swap_count = slot_diag_snapshot.swap_count;
    rt->slot_diag.max_wip_depth = slot_diag_snapshot.max_wip_depth;
    rt->slot_diag.overwrite_rejection_count = slot_diag_snapshot.overwrite_rejection_count;
    rt->slot_diag.stale_buffer_rejection_count = slot_diag_snapshot.stale_buffer_rejection_count;
    rt->slot_diag.shape_invalidation_count = slot_diag_snapshot.shape_invalidation_count;
    rt->slot_diag.layout_invalidation_count = slot_diag_snapshot.layout_invalidation_count;
    rt->slot_diag.capacity_invalidation_count = slot_diag_snapshot.capacity_invalidation_count;
    rt->slot_diag.inflight_rejection_count = slot_diag_snapshot.inflight_rejection_count;
    rt->slot_diag.cleanup_success_count = slot_diag_snapshot.cleanup_success_count;
    rt->slot_diag.failure_slot_id = slot_diag_snapshot.failure_slot_id;
    rt->slot_diag.failure_reason = slot_diag_snapshot.failure_reason;
  } else {
    out_diag->m29_current_slot_id = rt->slot_diag.current_slot_id;
    out_diag->m29_next_slot_id = rt->slot_diag.next_slot_id;
    out_diag->m29_slot0_state = (uint32_t)prom_slot_hfsm_current_state(&rt->slots[0]);
    out_diag->m29_slot1_state = (uint32_t)prom_slot_hfsm_current_state(&rt->slots[1]);
    out_diag->m29_slot0_generation = prom_slot_hfsm_metadata(&rt->slots[0])->generation;
    out_diag->m29_slot1_generation = prom_slot_hfsm_metadata(&rt->slots[1])->generation;
    out_diag->m29_slot0_valid = prom_slot_hfsm_metadata(&rt->slots[0])->valid;
    out_diag->m29_slot1_valid = prom_slot_hfsm_metadata(&rt->slots[1])->valid;
    out_diag->m29_swap_count = rt->slot_diag.swap_count;
    out_diag->m29_max_wip_depth = rt->slot_diag.max_wip_depth;
    out_diag->m29_overwrite_rejection_count = rt->slot_diag.overwrite_rejection_count;
    out_diag->m29_stale_buffer_rejection_count = rt->slot_diag.stale_buffer_rejection_count;
    out_diag->m29_shape_invalidation_count = rt->slot_diag.shape_invalidation_count;
    out_diag->m29_layout_invalidation_count = rt->slot_diag.layout_invalidation_count;
    out_diag->m29_capacity_invalidation_count = rt->slot_diag.capacity_invalidation_count;
    out_diag->m14_a_invalidation_count = rt->slot_diag.m14_a_invalidation_count;
    out_diag->m14_b_invalidation_count = rt->slot_diag.m14_b_invalidation_count;
    out_diag->m14_c_invalidation_count = rt->slot_diag.m14_c_invalidation_count;
    out_diag->m14_a_reuse_count = rt->slot_diag.m14_a_reuse_count;
    out_diag->m14_b_reuse_count = rt->slot_diag.m14_b_reuse_count;
    out_diag->m14_c_reuse_count = rt->slot_diag.m14_c_reuse_count;
    out_diag->m14_false_invalidation_avoided_count = rt->slot_diag.m14_false_invalidation_avoided_count;
    out_diag->m14_capacity_invalidation_count = rt->slot_diag.m14_capacity_invalidation_count;
    out_diag->m14_layout_precision_invalidation_count = rt->slot_diag.m14_layout_precision_invalidation_count;
    out_diag->m14_a_last_invalidation_reason = rt->slot_diag.m14_a_last_invalidation_reason;
    out_diag->m14_b_last_invalidation_reason = rt->slot_diag.m14_b_last_invalidation_reason;
    out_diag->m14_c_last_invalidation_reason = rt->slot_diag.m14_c_last_invalidation_reason;
    out_diag->m29_inflight_rejection_count = rt->slot_diag.inflight_rejection_count;
    out_diag->m29_cleanup_success_count = rt->slot_diag.cleanup_success_count;
    out_diag->m29_failure_slot_id = rt->slot_diag.failure_slot_id;
    out_diag->m29_failure_reason = rt->slot_diag.failure_reason;
  }
  if (prom_dom_sgemm_read_visible_transfer_queue_diagnostics(&rt->blackboard, &transfer_snapshot) != 0u) {
    out_diag->m31_transfer_queue_used = transfer_snapshot.transfer_queue_used;
    out_diag->m31_transfer_policy_selected = transfer_snapshot.transfer_policy_selected;
    out_diag->m31_dedicated_transfer_available = transfer_snapshot.dedicated_transfer_available;
    out_diag->m31_transfer_queue_family_index = transfer_snapshot.transfer_queue_family_index;
    out_diag->m31_compute_queue_family_index = transfer_snapshot.compute_queue_family_index;
    out_diag->m31_queue_families_differ = transfer_snapshot.queue_families_differ;
    out_diag->m31_transfer_fallback_reason = transfer_snapshot.transfer_fallback_reason;
    out_diag->m31_upload_policy_marker = transfer_snapshot.upload_only_policy_eligible;
    out_diag->m31_queue_family_handoff_count = transfer_snapshot.queue_family_handoff_count;
    out_diag->m31_transfer_compute_wait_count = transfer_snapshot.transfer_compute_wait_count;
    out_diag->m31_transfer_failure_slot_id = transfer_snapshot.transfer_failure_slot_id;
    out_diag->m31_transfer_failure_reason = transfer_snapshot.transfer_failure_reason;
    out_diag->m31_async_transfer_complete = transfer_snapshot.async_transfer_complete;
  } else {
    out_diag->m31_transfer_queue_used = rt->slot_diag.transfer_queue_used;
    out_diag->m31_transfer_policy_selected = rt->slot_diag.transfer_policy_selected;
    out_diag->m31_dedicated_transfer_available = rt->slot_diag.dedicated_transfer_available;
    out_diag->m31_transfer_queue_family_index = rt->slot_diag.transfer_queue_family_index;
    out_diag->m31_compute_queue_family_index = rt->slot_diag.compute_queue_family_index;
    out_diag->m31_queue_families_differ = rt->slot_diag.queue_families_differ;
    out_diag->m31_transfer_fallback_reason = rt->slot_diag.transfer_fallback_reason;
    out_diag->m31_upload_policy_marker = 1u;
    out_diag->m31_queue_family_handoff_count = rt->slot_diag.queue_family_handoff_count;
    out_diag->m31_transfer_compute_wait_count = rt->slot_diag.transfer_compute_wait_count;
    out_diag->m31_transfer_failure_slot_id = rt->slot_diag.transfer_failure_slot_id;
    out_diag->m31_transfer_failure_reason = rt->slot_diag.transfer_failure_reason;
    out_diag->m31_async_transfer_complete = rt->slot_diag.async_transfer_complete;
  }
  if (prom_dom_sgemm_read_visible_path_compute_diagnostics(&rt->blackboard, &path_compute_snapshot) != 0u) {
    out_diag->p13_m16b5_force_direct_requested = path_compute_snapshot.facts.force_direct;
    out_diag->p13_m16b5_force_direct_applied =
        (path_compute_snapshot.decision.selected_path == (uint32_t)PROM_VK_PATH_DIRECT &&
         prom_sgemm_effective_force_direct_reason(&path_compute_snapshot) != PROM_SGEMM_FORCE_DIRECT_REASON_NONE)
            ? 1u
            : 0u;
    out_diag->p13_m16b5_force_direct_reason = prom_sgemm_effective_force_direct_reason(&path_compute_snapshot);
    out_diag->p13_m16b5_requested_path = path_compute_snapshot.decision.requested_path;
    out_diag->p13_m16b5_selected_path = path_compute_snapshot.decision.selected_path;
    out_diag->p13_m16b5_compute_mode = path_compute_snapshot.decision.compute_mode;
  }
  if (prom_dom_sgemm_read_visible_m35(&rt->blackboard, &m35_snapshot) != 0u) {
    out_diag->m35_selected_buffering_mode = m35_snapshot.selected_mode;
    out_diag->m35_fixed_feasible = m35_snapshot.fixed_feasible;
    out_diag->m35_pull_lag_feasible = m35_snapshot.pull_lag_feasible;
    out_diag->m35_serial_feasible = m35_snapshot.serial_feasible;
    out_diag->m35_fixed_rejected = m35_snapshot.fixed_rejected;
    out_diag->m35_pull_lag_rejected = m35_snapshot.pull_lag_rejected;
    out_diag->m35_serial_rejected = m35_snapshot.serial_rejected;
    out_diag->m35_fixed_score = (uint32_t)(m35_snapshot.fixed_score < 0 ? 0 : m35_snapshot.fixed_score);
    out_diag->m35_pull_lag_score = (uint32_t)(m35_snapshot.pull_lag_score < 0 ? 0 : m35_snapshot.pull_lag_score);
    out_diag->m35_serial_score = (uint32_t)(m35_snapshot.serial_score < 0 ? 0 : m35_snapshot.serial_score);
    out_diag->m35_reason_code = m35_snapshot.reason_code;
    out_diag->m35_final_reason_code = m35_snapshot.final_reason_code;
    out_diag->m35_fixed_double_rejection_reason = m35_snapshot.fixed_double_rejection_reason;
    out_diag->m35_pull_lag_rejection_reason = m35_snapshot.pull_lag_rejection_reason;
    out_diag->m35_serial_jit_rejection_reason = m35_snapshot.serial_jit_rejection_reason;
    out_diag->m35_memory_budget_slots_permille = m35_snapshot.memory_budget_slots_permille;
    out_diag->m35_required_fixed_slots_permille = m35_snapshot.required_fixed_slots_permille;
    out_diag->m35_required_pull_lag_slots_permille = m35_snapshot.required_pull_lag_peak_slots_permille;
    out_diag->m35_required_serial_slots_permille = m35_snapshot.required_serial_slots_permille;
    out_diag->m35_fixed_double_headroom_slots_permille = (int64_t)m35_snapshot.fixed_double_headroom_slots_permille;
    out_diag->m35_pull_lag_headroom_slots_permille = (int64_t)m35_snapshot.pull_lag_headroom_slots_permille;
    out_diag->m35_serial_jit_headroom_slots_permille = (int64_t)m35_snapshot.serial_jit_headroom_slots_permille;
  } else {
    out_diag->m35_selected_buffering_mode = rt->slot_diag.m35_selected_mode;
    out_diag->m35_fixed_feasible = rt->slot_diag.m35_fixed_feasible;
    out_diag->m35_pull_lag_feasible = rt->slot_diag.m35_pull_lag_feasible;
    out_diag->m35_serial_feasible = rt->slot_diag.m35_serial_feasible;
    out_diag->m35_fixed_rejected = rt->slot_diag.m35_fixed_rejected;
    out_diag->m35_pull_lag_rejected = rt->slot_diag.m35_pull_lag_rejected;
    out_diag->m35_serial_rejected = rt->slot_diag.m35_serial_rejected;
    out_diag->m35_fixed_score = (uint32_t)(rt->slot_diag.m35_fixed_score < 0 ? 0 : rt->slot_diag.m35_fixed_score);
    out_diag->m35_pull_lag_score = (uint32_t)(rt->slot_diag.m35_pull_lag_score < 0 ? 0 : rt->slot_diag.m35_pull_lag_score);
    out_diag->m35_serial_score = (uint32_t)(rt->slot_diag.m35_serial_score < 0 ? 0 : rt->slot_diag.m35_serial_score);
    out_diag->m35_reason_code = rt->slot_diag.m35_reason_code;
    out_diag->m35_final_reason_code = rt->slot_diag.m35_final_reason_code;
    out_diag->m35_fixed_double_rejection_reason = rt->slot_diag.m35_fixed_double_rejection_reason;
    out_diag->m35_pull_lag_rejection_reason = rt->slot_diag.m35_pull_lag_rejection_reason;
    out_diag->m35_serial_jit_rejection_reason = rt->slot_diag.m35_serial_jit_rejection_reason;
    out_diag->m35_memory_budget_slots_permille = rt->slot_diag.m35_memory_budget_slots_permille;
    out_diag->m35_required_fixed_slots_permille = rt->slot_diag.m35_required_fixed_slots_permille;
    out_diag->m35_required_pull_lag_slots_permille = rt->slot_diag.m35_required_pull_lag_slots_permille;
    out_diag->m35_required_serial_slots_permille = rt->slot_diag.m35_required_serial_slots_permille;
    out_diag->m35_fixed_double_headroom_slots_permille = rt->slot_diag.m35_fixed_double_headroom_slots_permille;
    out_diag->m35_pull_lag_headroom_slots_permille = rt->slot_diag.m35_pull_lag_headroom_slots_permille;
    out_diag->m35_serial_jit_headroom_slots_permille = rt->slot_diag.m35_serial_jit_headroom_slots_permille;
  }
  out_diag->m35_transition_count = rt->slot_diag.m35_transition_count;
  out_diag->m35_rejection_count = rt->slot_diag.m35_rejection_count;
  out_diag->m35_budget_rejection_count = rt->slot_diag.m35_budget_rejection_count;
  out_diag->m35_pull_lag_predicted_demand_proxy_units = rt->slot_diag.m35_pull_lag_predicted_demand_proxy_units;
  out_diag->m35_pull_lag_transfer_lead_proxy_units = rt->slot_diag.m35_pull_lag_transfer_lead_proxy_units;
  out_diag->m35_pull_lag_safety_margin_proxy_units = rt->slot_diag.m35_pull_lag_safety_margin_proxy_units;
  out_diag->m35_pull_lag_stage_start_proxy_units = rt->slot_diag.m35_pull_lag_stage_start_proxy_units;
  out_diag->m35_pull_lag_stage_complete_proxy_units = rt->slot_diag.m35_pull_lag_stage_complete_proxy_units;
  out_diag->m35_pull_lag_late_stage_count = rt->slot_diag.m35_pull_lag_late_stage_count;
  out_diag->m35_pull_lag_early_stage_count = rt->slot_diag.m35_pull_lag_early_stage_count;
  out_diag->m35_pull_lag_starvation_proxy_units = rt->slot_diag.m35_pull_lag_starvation_proxy_units;
  out_diag->m35_pull_lag_ready_unused_proxy_units = rt->slot_diag.m35_pull_lag_ready_unused_proxy_units;
  out_diag->m35_pull_lag_wip_waste_exceeded_count = rt->slot_diag.m35_pull_lag_wip_waste_exceeded_count;
  out_diag->m35_serial_active_slot_count = rt->slot_diag.m35_serial_active_slot_count;
  out_diag->m35_serial_wip_depth = rt->slot_diag.m35_serial_wip_depth;
  out_diag->m35_serial_sequential_step_count = rt->slot_diag.m35_serial_sequential_step_count;
  out_diag->m35_serial_busy_retry_count = rt->slot_diag.m35_serial_busy_retry_count;
  out_diag->m35_serial_failure_cleanup_count = rt->slot_diag.m35_serial_failure_cleanup_count;
  out_diag->p13_m2_occupancy_device_band = rt->slot_diag.p13_m2_occupancy_device_band;
  out_diag->p13_m2_occupancy_shape_class = rt->slot_diag.p13_m2_occupancy_shape_class;
  out_diag->p13_m2_occupancy_selected_variant = rt->slot_diag.p13_m2_occupancy_selected_variant;
  out_diag->p13_m2_occupancy_unclamped_variant = rt->slot_diag.p13_m2_occupancy_unclamped_variant;
  out_diag->p13_m2_occupancy_clamp_reason = rt->slot_diag.p13_m2_occupancy_clamp_reason;
  out_diag->p13_m2_occupancy_override_used = rt->slot_diag.p13_m2_occupancy_override_used;
  out_diag->p13_m2_occupancy_fallback_used = rt->slot_diag.p13_m2_occupancy_fallback_used;
  out_diag->p13_m16b1_requested_occupancy_variant = rt->slot_diag.p13_m16b1_requested_occupancy_variant;
  out_diag->p13_m16b1_executed_occupancy_variant = rt->slot_diag.p13_m16b1_executed_occupancy_variant;
  out_diag->p13_m16b1_variant_registered = rt->slot_diag.p13_m16b1_variant_registered;
  out_diag->p13_m16b1_variant_benchmark_enabled = rt->slot_diag.p13_m16b1_variant_benchmark_enabled;
  out_diag->p13_m16b1_variant_dvt_validated = rt->slot_diag.p13_m16b1_variant_dvt_validated;
  out_diag->p13_m16b1_variant_pvt_validated = rt->slot_diag.p13_m16b1_variant_pvt_validated;
  out_diag->p13_m16b1_variant_production_eligible = rt->slot_diag.p13_m16b1_variant_production_eligible;
  out_diag->p13_m16b1_variant_dispatch_enabled = rt->slot_diag.p13_m16b1_variant_dispatch_enabled;
  out_diag->p13_m16b1_variant_path_status = rt->slot_diag.p13_m16b1_variant_path_status;
  out_diag->p13_m16b1_variant_path_id = rt->slot_diag.p13_m16b1_variant_path_id;
  out_diag->p13_m16b1_fallback_reason = rt->slot_diag.p13_m16b1_fallback_reason;
  out_diag->p13_m5_timestamp_available = rt->timestamp_query_supported;
  out_diag->p13_m5_last_gpu_timing_valid = rt->last_gpu_timing_valid;
  out_diag->p13_m5_last_gpu_timing_failure_reason = rt->last_gpu_timing_failure_reason;
  out_diag->p13_m5_last_gpu_duration_ns = rt->last_gpu_duration_ns;
  out_diag->p14_m8_filter_evidence_valid = rt->p14_last_filtered_evidence.valid;
  out_diag->p14_m8_raw_gpu_duration_ns = rt->p14_last_filtered_evidence.raw_value;
  out_diag->p14_m8_filtered_gpu_duration_ns = rt->p14_last_filtered_evidence.filtered_value;
  out_diag->p14_m8_filter_residual = rt->p14_last_filtered_evidence.residual;
  out_diag->p14_m8_filter_confidence = rt->p14_last_filtered_evidence.confidence;
  out_diag->p14_m8_filter_selected_kind = (uint32_t)rt->p14_last_filtered_evidence.selected_filter;
  out_diag->p14_m8_filter_previous_kind = (uint32_t)rt->p14_last_filtered_evidence.previous_filter;
  out_diag->p14_m8_filter_switched = rt->p14_last_filtered_evidence.filter_switched;
  out_diag->p14_m8_filter_warmup = rt->p14_last_filtered_evidence.filter_warmup;
  out_diag->p14_m8_filter_held_by_min_commit = rt->p14_last_filtered_evidence.held_by_min_commit;
  out_diag->p14_m8_filter_held_by_margin = rt->p14_last_filtered_evidence.held_by_margin;
  out_diag->p14_m8_filter_held_by_confidence = rt->p14_last_filtered_evidence.held_by_confidence;
  out_diag->p14_m8_filter_warm_transferred = rt->p14_last_filtered_evidence.warm_transferred;
  out_diag->p14_m8_filter_sample_count = rt->p14_last_filtered_evidence.sample_count;
  out_diag->p14_m8_filter_outlier_count = rt->p14_last_filtered_evidence.outlier_count;
  out_diag->p15_predictor_valid = rt->p14_last_filtered_evidence.valid;
  out_diag->p15_prediction_confidence = rt->p15_predictor_state.prediction_confidence;
  out_diag->p15_lookahead_depth = rt->p15_predictor_state.lookahead_depth;
  out_diag->p15_prediction_issued = rt->p15_last_prediction_issued.active;
  out_diag->p15_prediction_matured = rt->p15_last_correction.prediction_matured;
  out_diag->p15_predicted_ready_tick = rt->p15_last_correction.target_tick;
  out_diag->p15_actual_ready_tick = rt->p15_last_correction.prediction_matured != 0u ? rt->p15_last_correction.tick : 0u;
  out_diag->p15_prediction_error_ticks = rt->p15_last_correction.arrival_error_ticks;
  out_diag->p15_correction_count = rt->p15_predictor_state.correction_count;
  out_diag->p15_correction_action = (uint32_t)rt->p15_last_correction.action;
  out_diag->p15_fallback_active = rt->p15_predictor_state.fallback_active;
  out_diag->p15_fallback_reason = rt->p15_predictor_state.fallback_reason;
  out_diag->p15_future_lease_valid = rt->p15_predictor_state.future_lease_seam.last_request.valid;
  out_diag->p15_future_lease_request_id = rt->p15_predictor_state.future_lease_seam.last_request.request_id;
  out_diag->p15_future_lease_state = (uint32_t)rt->p15_predictor_state.future_lease_seam.last_request.state;
  out_diag->p15_future_lease_target_tick = rt->p15_predictor_state.future_lease_seam.last_request.target_tick;
  out_diag->p15_future_lease_confidence = rt->p15_predictor_state.future_lease_seam.last_request.confidence;
  out_diag->p15_future_lease_reason = rt->p15_predictor_state.future_lease_seam.last_request.cancel_reason;
  out_diag->p15_reservation_valid = rt->p15_last_reservation.valid;
  out_diag->p15_reservation_request_id = rt->p15_last_reservation.request_id;
  out_diag->p15_reservation_state = (uint32_t)rt->p15_last_reservation.new_state;
  out_diag->p15_reservation_reserved = rt->p15_last_reservation.reserved;
  out_diag->p15_reservation_denied = rt->p15_last_reservation.denied;
  out_diag->p15_reservation_cancelled = rt->p15_last_reservation.cancelled;
  out_diag->p15_reservation_matured = rt->p15_last_reservation.matured;
  out_diag->p15_reservation_expired = rt->p15_last_reservation.expired;
  out_diag->p15_reservation_reason = rt->p15_last_reservation.reason;
  out_diag->p15_reservation_active_count = rt->p15_last_reservation.active_count;
  out_diag->p15_prestage_valid = rt->p15_last_prestage.valid;
  out_diag->p15_prestage_state = (uint32_t)rt->p15_last_prestage.state;
  out_diag->p15_prestage_allowed = rt->p15_last_prestage.allowed;
  out_diag->p15_prestage_submitted = rt->p15_last_prestage.submitted;
  out_diag->p15_prestage_block_reasons = rt->p15_last_prestage.block_reasons;
  out_diag->p15_prestage_confidence = rt->p15_last_prestage.confidence;
  out_diag->p15_prestage_target_tick = rt->p15_last_prestage.target_tick;
  out_diag->p15_prestage_lead_ticks = rt->p15_last_prestage.lead_ticks;
  out_diag->p15_prestage_cost_estimate = rt->p15_last_prestage.cost_estimate;
  out_diag->p15_prestage_benefit_estimate = rt->p15_last_prestage.benefit_estimate;
  out_diag->p15_shadow_valid = rt->p15_last_shadow.valid;
  out_diag->p15_shadow_state = (uint32_t)rt->p15_last_shadow.shadow_state;
  out_diag->p15_shadow_physical_state = rt->p15_last_shadow.physical_state;
  out_diag->p15_shadow_issued_tick = rt->p15_last_shadow.issued_tick;
  out_diag->p15_shadow_target_tick = rt->p15_last_shadow.target_tick;
  out_diag->p15_shadow_predicted_ready_tick = rt->p15_last_shadow.predicted_ready_tick;
  out_diag->p15_shadow_actual_ready_tick = rt->p15_last_shadow.actual_ready_tick;
  out_diag->p15_shadow_arrival_error_ticks = rt->p15_last_shadow.arrival_error_ticks;
  out_diag->p15_shadow_prediction_confidence = rt->p15_last_shadow.prediction_confidence;
  out_diag->p15_shadow_mismatch_kind = (uint32_t)rt->p15_last_shadow.mismatch_kind;
  out_diag->p15_shadow_matched = rt->p15_last_shadow.matched;
  out_diag->p15_shadow_stale = rt->p15_last_shadow.stale;
  out_diag->p15_shadow_cancelled = rt->p15_last_shadow.cancelled;
  out_diag->p15_shadow_fallback = rt->p15_last_shadow.fallback;
  out_diag->p15_shadow_correction_action = (uint32_t)rt->p15_last_shadow.correction_action;
  out_diag->p15_shadow_correction_count = rt->p15_last_shadow.correction_count;
  out_diag->p15_shadow_stale_count = rt->p15_last_shadow.stale_count;
  out_diag->p15_shadow_miss_count = rt->p15_last_shadow.miss_count;
  out_diag->p15_shadow_calibration_valid = rt->p15_shadow_calibration.valid;
  out_diag->p15_shadow_calibration_sample_count = rt->p15_shadow_calibration.sample_count;
  out_diag->p15_shadow_calibration_match_count = rt->p15_shadow_calibration.match_count;
  out_diag->p15_shadow_calibration_miss_count = rt->p15_shadow_calibration.miss_count;
  out_diag->p15_shadow_calibration_early_count = rt->p15_shadow_calibration.early_count;
  out_diag->p15_shadow_calibration_late_count = rt->p15_shadow_calibration.late_count;
  out_diag->p15_shadow_calibration_stale_count = rt->p15_shadow_calibration.stale_count;
  out_diag->p15_shadow_calibration_fallback_count = rt->p15_shadow_calibration.fallback_count;
  out_diag->p15_shadow_calibration_confidence = rt->p15_shadow_calibration.confidence;
  out_diag->p15_shadow_calibration_mean_abs_arrival_error_ticks =
      rt->p15_shadow_calibration.sample_count == 0u
          ? 0.0
          : (double)rt->p15_shadow_calibration.total_abs_arrival_error_ticks / (double)rt->p15_shadow_calibration.sample_count;
  out_diag->p15_shadow_calibration_last_mismatch_kind = (uint32_t)rt->p15_shadow_calibration.last_mismatch_kind;
  out_diag->p15_shadow_lookahead_state = (uint32_t)rt->p15_shadow_calibration.lookahead_diagnostic_state;
  out_diag->p15_shadow_authority_valid = rt->p15_shadow_authority_gate.valid;
  out_diag->p15_shadow_authority_state = (uint32_t)rt->p15_shadow_authority_gate.state;
  out_diag->p15_shadow_authority_reason = (uint32_t)rt->p15_shadow_authority_gate.reason;
  out_diag->p15_shadow_authority_canary_allowed = rt->p15_shadow_authority_gate.canary_allowed;
  out_diag->p15_shadow_authority_would_act = rt->p15_shadow_authority_gate.authority_would_act;
  out_diag->p15_shadow_authority_enabled = rt->p15_shadow_authority_gate.authority_enabled;
  out_diag->p15_shadow_authority_recommended_lookahead_depth = rt->p15_shadow_authority_gate.recommended_lookahead_depth;
  out_diag->p15_shadow_authority_confidence_gate_passed = rt->p15_shadow_authority_gate.confidence_gate_passed;
  out_diag->p15_shadow_authority_sample_gate_passed = rt->p15_shadow_authority_gate.sample_gate_passed;
  out_diag->p15_shadow_authority_miss_rate_gate_passed = rt->p15_shadow_authority_gate.miss_rate_gate_passed;
  out_diag->p15_shadow_authority_arrival_error_gate_passed = rt->p15_shadow_authority_gate.arrival_error_gate_passed;
  out_diag->p15_shadow_authority_lookahead_gate_passed = rt->p15_shadow_authority_gate.lookahead_state_gate_passed;
  out_diag->p15_shadow_authority_match_rate = rt->p15_shadow_authority_gate.match_rate;
  out_diag->p15_shadow_authority_miss_rate = rt->p15_shadow_authority_gate.miss_rate;
  out_diag->p15_shadow_authority_mean_abs_arrival_error_ticks = rt->p15_shadow_authority_gate.mean_abs_arrival_error_ticks;
  out_diag->p15_shadow_would_act_valid = rt->p15_shadow_would_act_state.valid;
  out_diag->p15_shadow_would_act_evaluation_count = rt->p15_shadow_would_act_state.evaluation_count;
  out_diag->p15_shadow_would_act_count = rt->p15_shadow_would_act_state.would_act_count;
  out_diag->p15_shadow_would_block_count = rt->p15_shadow_would_act_state.would_block_count;
  out_diag->p15_shadow_would_unknown_count = rt->p15_shadow_would_act_state.would_unknown_count;
  out_diag->p15_shadow_would_disabled_count = rt->p15_shadow_would_act_state.would_disabled_count;
  out_diag->p15_shadow_would_canary_count = rt->p15_shadow_would_act_state.would_canary_count;
  out_diag->p15_shadow_would_healthy_count = rt->p15_shadow_would_act_state.would_healthy_count;
  out_diag->p15_shadow_would_block_low_confidence_count = rt->p15_shadow_would_act_state.blocked_low_confidence_count;
  out_diag->p15_shadow_would_block_high_miss_rate_count = rt->p15_shadow_would_act_state.blocked_high_miss_rate_count;
  out_diag->p15_shadow_would_block_high_arrival_error_count = rt->p15_shadow_would_act_state.blocked_high_arrival_error_count;
  out_diag->p15_shadow_would_block_recent_fallback_count = rt->p15_shadow_would_act_state.blocked_recent_fallback_count;
  out_diag->p15_shadow_would_block_recent_stale_count = rt->p15_shadow_would_act_state.blocked_recent_stale_count;
  out_diag->p15_shadow_would_block_insufficient_samples_count = rt->p15_shadow_would_act_state.blocked_insufficient_samples_count;
  out_diag->p15_shadow_would_block_invalid_calibration_count = rt->p15_shadow_would_act_state.blocked_invalid_calibration_count;
  out_diag->p15_shadow_would_block_lookahead_disabled_count = rt->p15_shadow_would_act_state.blocked_lookahead_disabled_count;
  out_diag->p15_shadow_would_healthy_suppressed_by_recent_fallback_count =
      rt->p15_shadow_would_act_state.healthy_suppressed_by_recent_fallback_count;
  out_diag->p15_shadow_would_healthy_suppressed_by_recent_stale_count =
      rt->p15_shadow_would_act_state.healthy_suppressed_by_recent_stale_count;
  out_diag->p15_shadow_would_healthy_suppressed_by_arrival_error_count =
      rt->p15_shadow_would_act_state.healthy_suppressed_by_arrival_error_count;
  out_diag->p15_shadow_would_last_would_act = rt->p15_shadow_would_act_state.last_would_act;
  out_diag->p15_shadow_would_last_reason = (uint32_t)rt->p15_shadow_would_act_state.last_would_block_reason;
  out_diag->p15_shadow_would_last_gate_state = (uint32_t)rt->p15_shadow_would_act_state.last_gate_state;
  out_diag->p15_shadow_would_last_recommended_lookahead_depth = rt->p15_shadow_would_act_state.last_recommended_lookahead_depth;
  out_diag->p15_shadow_canary_valid = rt->p15_shadow_canary_state.valid;
  out_diag->p15_shadow_canary_enabled = rt->p15_shadow_canary_state.enabled;
  out_diag->p15_shadow_canary_last_action_allowed = rt->p15_shadow_canary_state.last_action_allowed;
  out_diag->p15_shadow_canary_last_action_kind = (uint32_t)rt->p15_shadow_canary_state.last_action_kind;
  out_diag->p15_shadow_canary_last_block_reason = (uint32_t)rt->p15_shadow_canary_state.last_block_reason;
  out_diag->p15_shadow_canary_requested_lookahead_depth = rt->p15_shadow_canary_state.requested_lookahead_depth;
  out_diag->p15_shadow_canary_healthy_margin_passed = rt->p15_shadow_canary_state.healthy_margin_passed;
  out_diag->p15_shadow_canary_reason_binding_passed = rt->p15_shadow_canary_state.reason_binding_passed;
  out_diag->p15_shadow_canary_evaluation_count = rt->p15_shadow_canary_state.evaluation_count;
  out_diag->p15_shadow_canary_action_allowed_count = rt->p15_shadow_canary_state.action_allowed_count;
  out_diag->p15_shadow_canary_action_applied_count = rt->p15_shadow_canary_state.action_applied_count;
  out_diag->p15_shadow_canary_action_blocked_count = rt->p15_shadow_canary_state.action_blocked_count;
  out_diag->p15_shadow_canary_reservation_attempt_count = rt->p15_shadow_canary_state.reservation_attempt_count;
  out_diag->p15_shadow_canary_reservation_success_count = rt->p15_shadow_canary_state.reservation_success_count;
  out_diag->p15_shadow_canary_reservation_rejected_count = rt->p15_shadow_canary_state.reservation_rejected_count;
  out_diag->p15_shadow_canary_block_low_confidence_count = rt->p15_shadow_canary_state.block_low_confidence_count;
  out_diag->p15_shadow_canary_block_high_miss_rate_count = rt->p15_shadow_canary_state.block_high_miss_rate_count;
  out_diag->p15_shadow_canary_block_high_arrival_error_count = rt->p15_shadow_canary_state.block_high_arrival_error_count;
  out_diag->p15_shadow_canary_block_recent_fallback_count = rt->p15_shadow_canary_state.block_recent_fallback_count;
  out_diag->p15_shadow_canary_block_recent_stale_count = rt->p15_shadow_canary_state.block_recent_stale_count;
  out_diag->p15_shadow_canary_block_insufficient_samples_count = rt->p15_shadow_canary_state.block_insufficient_samples_count;
  out_diag->p15_shadow_canary_block_disabled_count = rt->p15_shadow_canary_state.block_disabled_count;
  out_diag->p15_shadow_canary_block_no_future_lease_count = rt->p15_shadow_canary_state.block_no_future_lease_count;
  out_diag->p15_shadow_canary_block_reservation_failed_count = rt->p15_shadow_canary_state.block_reservation_failed_count;
  out_diag->p15_shadow_feedforward_valid = rt->p15_feedforward_dispatch_state.valid;
  out_diag->p15_shadow_feedforward_enabled = rt->p15_feedforward_dispatch_state.enabled;
  out_diag->p15_shadow_feedforward_used = rt->p15_feedforward_dispatch_state.used;
  out_diag->p15_shadow_feedforward_source = rt->p15_feedforward_dispatch_state.source;
  out_diag->p15_shadow_feedforward_reservation_present = rt->p15_feedforward_dispatch_state.reservation_present;
  out_diag->p15_shadow_feedforward_reservation_matured = rt->p15_feedforward_dispatch_state.reservation_matured;
  out_diag->p15_shadow_feedforward_block_reason = rt->p15_feedforward_dispatch_state.block_reason;
  out_diag->p15_shadow_feedforward_reserved_variant_id = rt->p15_feedforward_dispatch_state.reserved_variant_id;
  out_diag->p15_shadow_feedforward_selected_variant_id = rt->p15_feedforward_dispatch_state.selected_variant_id;
  out_diag->p15_shadow_feedforward_reconciliation_match = rt->p15_feedforward_dispatch_state.reconciliation_match;
  out_diag->p15_shadow_feedforward_correction_action = rt->p15_feedforward_dispatch_state.correction_action;
  out_diag->p15_shadow_feedforward_fallback_to_judgment_count = rt->p15_feedforward_dispatch_state.fallback_to_judgment_count;
  out_diag->p15_shadow_feedforward_reservation_consumed_count = rt->p15_feedforward_dispatch_state.reservation_consumed_count;
  out_diag->p15_shadow_feedforward_no_matured_reservation_count = rt->p15_feedforward_dispatch_state.no_matured_reservation_count;
  out_diag->p15_shadow_feedforward_shape_mismatch_count = rt->p15_feedforward_dispatch_state.shape_mismatch_count;
  out_diag->p15_shadow_feedforward_variant_mismatch_count = rt->p15_feedforward_dispatch_state.variant_mismatch_count;
  out_diag->p15_shadow_feedforward_stale_reservation_count = rt->p15_feedforward_dispatch_state.stale_reservation_count;
  out_diag->p15_shadow_feedforward_reason_binding_block_count = rt->p15_feedforward_dispatch_state.reason_binding_block_count;
  out_diag->p15_shadow_feedforward_margin_block_count = rt->p15_feedforward_dispatch_state.margin_block_count;
  out_diag->p15_shadow_feedforward_dedup_block_count = rt->p15_feedforward_dispatch_state.dedup_block_count;
  out_diag->px16_m6_selector_recommended_variant = rt->slot_diag.px16_m6_selector_recommended_variant;
  out_diag->px16_m6_selector_selected_variant = rt->slot_diag.px16_m6_selector_selected_variant;
  out_diag->px16_m6_requested_dispatch_variant = rt->slot_diag.px16_m6_requested_dispatch_variant;
  out_diag->px16_m6_executed_dispatch_variant = rt->slot_diag.px16_m6_executed_dispatch_variant;
  out_diag->px16_m6_requested_path = rt->slot_diag.px16_m6_requested_path;
  out_diag->px16_m6_selected_path = rt->slot_diag.px16_m6_selected_path;
  out_diag->px16_m6_executed_path = rt->slot_diag.px16_m6_executed_path;
  out_diag->px16_m6_requested_compute_mode = rt->slot_diag.px16_m6_requested_compute_mode;
  out_diag->px16_m6_selected_compute_mode = rt->slot_diag.px16_m6_selected_compute_mode;
  out_diag->px16_m6_executed_compute_mode = rt->slot_diag.px16_m6_executed_compute_mode;
  out_diag->px16_m6_force_direct_requested = rt->slot_diag.px16_m6_force_direct_requested;
  out_diag->px16_m6_force_direct_applied = rt->slot_diag.px16_m6_force_direct_applied;
  out_diag->px16_m6_force_direct_reason = rt->slot_diag.px16_m6_force_direct_reason;
  out_diag->px16_m6_policy_mode = rt->slot_diag.px16_m6_policy_mode;
  out_diag->px16_m6_variant_path_status = rt->slot_diag.px16_m6_variant_path_status;
  out_diag->px16_m6_variant_production_eligible = rt->slot_diag.px16_m6_variant_production_eligible;
  out_diag->px16_m6_variant_dispatch_enabled = rt->slot_diag.px16_m6_variant_dispatch_enabled;
  out_diag->px16_m6_variant_dvt_validated = rt->slot_diag.px16_m6_variant_dvt_validated;
  out_diag->px16_m6_variant_pvt_validated = rt->slot_diag.px16_m6_variant_pvt_validated;
  out_diag->px16_m6_variant_lifecycle_telemetry_only = rt->slot_diag.px16_m6_variant_lifecycle_telemetry_only;
  out_diag->px16_m6_p15_reservation_present = rt->slot_diag.px16_m6_p15_reservation_present;
  out_diag->px16_m6_p15_reservation_matured = rt->slot_diag.px16_m6_p15_reservation_matured;
  out_diag->px16_m6_p15_reservation_consumed = rt->slot_diag.px16_m6_p15_reservation_consumed;
  out_diag->px16_m6_p15_reserved_variant_id = rt->slot_diag.px16_m6_p15_reserved_variant_id;
  out_diag->px16_m6_p15_live_selected_variant_id = rt->slot_diag.px16_m6_p15_live_selected_variant_id;
  out_diag->px16_m6_p15_reconciliation_match = rt->slot_diag.px16_m6_p15_reconciliation_match;
  out_diag->px16_m6_p15_block_reason = rt->slot_diag.px16_m6_p15_block_reason;
  out_diag->px16_m6_p15_correction_action = rt->slot_diag.px16_m6_p15_correction_action;
  out_diag->px16_m6_p15_reservation_stale_or_expired = rt->slot_diag.px16_m6_p15_reservation_stale_or_expired;
  out_diag->px16_m6_p15_confidence_before = rt->slot_diag.px16_m6_p15_confidence_before;
  out_diag->px16_m6_p15_confidence_after = rt->slot_diag.px16_m6_p15_confidence_after;
  out_diag->px16_m8_last_upload_wall_ns = rt->px16_m8_last_upload_wall_ns;
  out_diag->px16_m8_last_pre_dispatch_wall_ns = rt->px16_m8_last_pre_dispatch_wall_ns;
  out_diag->px16_m8_last_command_record_wall_ns = rt->px16_m8_last_command_record_wall_ns;
  out_diag->px16_m8_last_dispatch_submit_wall_ns = rt->px16_m8_last_dispatch_submit_wall_ns;
  out_diag->px16_m8_last_sync_wait_wall_ns = rt->px16_m8_last_sync_wait_wall_ns;
  out_diag->px16_m8_last_post_sync_wall_ns = rt->px16_m8_last_post_sync_wall_ns;
  out_diag->px16_m8_last_readback_wall_ns = rt->px16_m8_last_readback_wall_ns;
  out_diag->px16_m8_last_post_readback_wall_ns = rt->px16_m8_last_post_readback_wall_ns;
  out_diag->px16_m8_last_total_wall_ns = rt->px16_m8_last_total_wall_ns;
  out_diag->px16_m8_last_gpu_timestamp_valid = rt->last_gpu_timing_valid;
  out_diag->px16_m8_resident_device_mode_available =
      (rt->vulkan.available != 0u &&
       rt->vulkan.has_device_local_memory != 0u &&
       (rt->vulkan.test_flags & PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY) == 0u)
          ? 1u
          : 0u;
  out_diag->px16_m8_last_executed_explicit_variant_request = rt->px16_m8_last_executed_explicit_variant_request;
  out_diag->px16_m17_last_tolerance_eval_wall_ns = rt->px16_m17_last_tolerance_eval_wall_ns;
  out_diag->px16_m17_last_tolerance_eval_in_dispatch = rt->px16_m17_last_tolerance_eval_in_dispatch;
  out_diag->px16_m17_last_tolerance_eval_source = rt->px16_m17_last_tolerance_eval_source;
  out_diag->p13_m5_timestamp_valid_bits = rt->vulkan.timestamp_valid_bits;
  out_diag->p13_m5_timestamp_period_ns = rt->vulkan.timestamp_period_ns;
  if (prom_dom_slot_read_last_commit(&rt->blackboard, 0u, &slot_snapshot) != 0u && slot_snapshot.committed_event_count > 0u) {
    out_diag->p10_m4_last_slot_event_kind = (uint32_t)slot_snapshot.last_event.kind;
    out_diag->p10_m4_last_slot_event_slot_id = slot_snapshot.last_event.slot_id;
    out_diag->p10_m4_last_slot_event_reason = slot_snapshot.last_event.reason_code;
    out_diag->p10_m4_last_commit_dirty_slot_mask = slot_snapshot.last_commit_dirty_slot_mask;
  }
  if (prom_dom_slot_readiness_read_visible(&rt->blackboard, &slot_readiness_snapshot) != 0u) {
    out_diag->p10_m16_slot_readiness_boundary_generation = slot_readiness_snapshot.boundary_generation;
    out_diag->p10_m16_slot_readiness_dirty_slot_mask = slot_readiness_snapshot.dirty_slot_mask;
    out_diag->p10_m16_slot_readiness_ready_slot_mask = slot_readiness_snapshot.ready_slot_mask;
    out_diag->p10_m16_slot_readiness_failed_slot_mask = slot_readiness_snapshot.failed_slot_mask;
    out_diag->p10_m16_slot_readiness_invalidated_slot_mask = slot_readiness_snapshot.invalidated_slot_mask;
    out_diag->p10_m16_slot_readiness_attention_slot_mask = slot_readiness_snapshot.attention_slot_mask;
    out_diag->p10_m16_slot_readiness_overflow_spill_count = slot_readiness_snapshot.overflow_spill_count;
    out_diag->p10_m16_slot_readiness_duplicate_ready_event_count = slot_readiness_snapshot.duplicate_ready_event_count;
    out_diag->p10_m16_slot_readiness_empty_boundary_commit_count = slot_readiness_snapshot.empty_boundary_commit_count;
  }
  out_diag->p10_m13_m35_selector_cache_enabled = selector_cache_enabled(rt);
  out_diag->p10_m13_m35_selector_cache_valid = rt->m35_selector_cache.valid;
  out_diag->p10_m13_m35_selector_reuse_count = rt->m35_selector_cache.reuse_count;
  out_diag->p10_m13_m35_selector_recompute_count = rt->m35_selector_cache.recompute_count;
  out_diag->p10_m13_m35_selector_invalidation_count = rt->m35_selector_cache.invalidation_count;
  out_diag->p10_m13_m35_selector_last_dirty_dependency_mask = rt->m35_selector_cache.last_dirty_dependency_mask;
  out_diag->p10_m13_m35_selector_last_visible_generation = rt->m35_selector_cache.visible_generation_when_computed;
  out_diag->p10_m13_m35_selector_last_decision_reused = rt->m35_selector_cache.last_decision_reused;
  out_diag->p10_m13_transfer_selector_cache_enabled = selector_cache_enabled(rt);
  out_diag->p10_m13_transfer_selector_cache_valid = rt->transfer_selector_cache.valid;
  out_diag->p10_m13_transfer_selector_reuse_count = rt->transfer_selector_cache.reuse_count;
  out_diag->p10_m13_transfer_selector_recompute_count = rt->transfer_selector_cache.recompute_count;
  out_diag->p10_m13_transfer_selector_invalidation_count = rt->transfer_selector_cache.invalidation_count;
  out_diag->p10_m13_transfer_selector_last_dirty_dependency_mask = rt->transfer_selector_cache.last_dirty_dependency_mask;
  out_diag->p10_m13_transfer_selector_last_visible_generation = rt->transfer_selector_cache.visible_generation_when_computed;
  out_diag->p10_m13_transfer_selector_last_decision_reused = rt->transfer_selector_cache.last_decision_reused;
  out_diag->p10_m15_layout_precision_selector_cache_enabled = selector_cache_enabled(rt);
  out_diag->p10_m15_layout_precision_selector_cache_valid = rt->layout_precision_selector_cache.valid;
  out_diag->p10_m15_layout_precision_selector_reuse_count = rt->layout_precision_selector_cache.reuse_count;
  out_diag->p10_m15_layout_precision_selector_recompute_count = rt->layout_precision_selector_cache.recompute_count;
  out_diag->p10_m15_layout_precision_selector_invalidation_count = rt->layout_precision_selector_cache.invalidation_count;
  out_diag->p10_m15_layout_precision_selector_last_dirty_dependency_mask =
      rt->layout_precision_selector_cache.last_dirty_dependency_mask;
  out_diag->p10_m15_layout_precision_selector_last_visible_generation =
      rt->layout_precision_selector_cache.visible_generation_when_computed;
  out_diag->p10_m15_layout_precision_selector_last_decision_reused = rt->layout_precision_selector_cache.last_decision_reused;
  if (prom_dom_sgemm_read_visible_resource_lease_diagnostics(&rt->blackboard, &lease_snapshot) != 0u) {
    out_diag->p13_m10_lease_request_count = lease_snapshot.granted_count + lease_snapshot.denied_count;
    out_diag->p13_m10_lease_grant_count = lease_snapshot.granted_count;
    out_diag->p13_m10_lease_deny_count = lease_snapshot.denied_count;
    out_diag->p13_m10_lease_yield_count = lease_snapshot.yield_count;
    out_diag->p13_m10_lease_last_state = lease_snapshot.decision.lease_state;
    out_diag->p13_m10_lease_last_deny_reason = lease_snapshot.decision.deny_reason;
    out_diag->p13_m10_lookahead_requested = lease_snapshot.facts.lookahead_requested;
    out_diag->p13_m10_lookahead_allowed = lease_snapshot.decision.lookahead_allowed;
    out_diag->p13_m10_lookahead_blocked_reason = lease_snapshot.lookahead_blocked_reason;
    out_diag->p13_m10_selected_recipe_variant = lease_snapshot.decision.selected_recipe_variant;
  }
  out_diag->p11_m3_arena_a_capacity_bytes = rt->arenas[PROM_ARENA_ROLE_A].capacity_bytes;
  out_diag->p11_m3_arena_b_capacity_bytes = rt->arenas[PROM_ARENA_ROLE_B].capacity_bytes;
  out_diag->p11_m3_arena_c_capacity_bytes = rt->arenas[PROM_ARENA_ROLE_C].capacity_bytes;
  out_diag->p11_m3_arena_upload_capacity_bytes = rt->arenas[PROM_ARENA_ROLE_UPLOAD].capacity_bytes;
  out_diag->p11_m3_arena_a_required_bytes = rt->arenas[PROM_ARENA_ROLE_A].required_bytes;
  out_diag->p11_m3_arena_b_required_bytes = rt->arenas[PROM_ARENA_ROLE_B].required_bytes;
  out_diag->p11_m3_arena_c_required_bytes = rt->arenas[PROM_ARENA_ROLE_C].required_bytes;
  out_diag->p11_m3_arena_upload_required_bytes = rt->arenas[PROM_ARENA_ROLE_UPLOAD].required_bytes;
  out_diag->p11_m3_arena_a_generation = rt->arenas[PROM_ARENA_ROLE_A].generation;
  out_diag->p11_m3_arena_b_generation = rt->arenas[PROM_ARENA_ROLE_B].generation;
  out_diag->p11_m3_arena_c_generation = rt->arenas[PROM_ARENA_ROLE_C].generation;
  out_diag->p11_m3_arena_upload_generation = rt->arenas[PROM_ARENA_ROLE_UPLOAD].generation;
  out_diag->p11_m3_arena_a_reuse_count = rt->arenas[PROM_ARENA_ROLE_A].reuse_count;
  out_diag->p11_m3_arena_b_reuse_count = rt->arenas[PROM_ARENA_ROLE_B].reuse_count;
  out_diag->p11_m3_arena_c_reuse_count = rt->arenas[PROM_ARENA_ROLE_C].reuse_count;
  out_diag->p11_m3_arena_upload_reuse_count = rt->arenas[PROM_ARENA_ROLE_UPLOAD].reuse_count;
  out_diag->p11_m3_arena_a_grow_count = rt->arenas[PROM_ARENA_ROLE_A].grow_count;
  out_diag->p11_m3_arena_b_grow_count = rt->arenas[PROM_ARENA_ROLE_B].grow_count;
  out_diag->p11_m3_arena_c_grow_count = rt->arenas[PROM_ARENA_ROLE_C].grow_count;
  out_diag->p11_m3_arena_upload_grow_count = rt->arenas[PROM_ARENA_ROLE_UPLOAD].grow_count;
  out_diag->p11_m3_arena_a_shrink_count = rt->arenas[PROM_ARENA_ROLE_A].shrink_count;
  out_diag->p11_m3_arena_b_shrink_count = rt->arenas[PROM_ARENA_ROLE_B].shrink_count;
  out_diag->p11_m3_arena_c_shrink_count = rt->arenas[PROM_ARENA_ROLE_C].shrink_count;
  out_diag->p11_m3_arena_upload_shrink_count = rt->arenas[PROM_ARENA_ROLE_UPLOAD].shrink_count;
  out_diag->p11_m3_arena_a_rebuild_count = rt->arenas[PROM_ARENA_ROLE_A].rebuild_count;
  out_diag->p11_m3_arena_b_rebuild_count = rt->arenas[PROM_ARENA_ROLE_B].rebuild_count;
  out_diag->p11_m3_arena_c_rebuild_count = rt->arenas[PROM_ARENA_ROLE_C].rebuild_count;
  out_diag->p11_m3_arena_upload_rebuild_count = rt->arenas[PROM_ARENA_ROLE_UPLOAD].rebuild_count;
  out_diag->p11_m3_arena_grow_count = rt->arenas[PROM_ARENA_ROLE_A].grow_count +
                                       rt->arenas[PROM_ARENA_ROLE_B].grow_count +
                                       rt->arenas[PROM_ARENA_ROLE_C].grow_count +
                                       rt->arenas[PROM_ARENA_ROLE_UPLOAD].grow_count;
  out_diag->p11_m3_arena_shrink_count = rt->arenas[PROM_ARENA_ROLE_A].shrink_count +
                                         rt->arenas[PROM_ARENA_ROLE_B].shrink_count +
                                         rt->arenas[PROM_ARENA_ROLE_C].shrink_count +
                                         rt->arenas[PROM_ARENA_ROLE_UPLOAD].shrink_count;
  out_diag->p11_m3_arena_rebuild_count = rt->arenas[PROM_ARENA_ROLE_A].rebuild_count +
                                          rt->arenas[PROM_ARENA_ROLE_B].rebuild_count +
                                          rt->arenas[PROM_ARENA_ROLE_C].rebuild_count +
                                          rt->arenas[PROM_ARENA_ROLE_UPLOAD].rebuild_count;
  out_diag->p11_m3_arena_budget_rejection_count = rt->arenas[PROM_ARENA_ROLE_A].budget_rejection_count +
                                                   rt->arenas[PROM_ARENA_ROLE_B].budget_rejection_count +
                                                   rt->arenas[PROM_ARENA_ROLE_C].budget_rejection_count +
                                                   rt->arenas[PROM_ARENA_ROLE_UPLOAD].budget_rejection_count;
  out_diag->p11_m3_arena_ownership_rejection_count = rt->arenas[PROM_ARENA_ROLE_A].ownership_rejection_count +
                                                      rt->arenas[PROM_ARENA_ROLE_B].ownership_rejection_count +
                                                      rt->arenas[PROM_ARENA_ROLE_C].ownership_rejection_count +
                                                      rt->arenas[PROM_ARENA_ROLE_UPLOAD].ownership_rejection_count;
  out_diag->p11_m3_arena_namespace_rejection_count = rt->arenas[PROM_ARENA_ROLE_A].namespace_rejection_count +
                                                      rt->arenas[PROM_ARENA_ROLE_B].namespace_rejection_count +
                                                      rt->arenas[PROM_ARENA_ROLE_C].namespace_rejection_count +
                                                      rt->arenas[PROM_ARENA_ROLE_UPLOAD].namespace_rejection_count;
  out_diag->p11_m3_arena_total_committed_bytes = arena_total_committed_bytes(rt);
  out_diag->p11_m3_arena_projected_committed_bytes = rt->slot_diag.p11_m3_projected_committed_bytes;
  out_diag->p11_m3_arena_budget_limit_bytes = rt->arena_budget_limit_bytes;
  out_diag->p11_m3_arena_last_failure_reason = rt->arena_last_failure_detail;
  return PROM_OK;
}


int prom_reactor_runtime_sgemm_policy_diagnostics_sized_impl(void* handle,
                                                             PrometheusSgemmPolicyDiagnostics* out_diag,
                                                             uint32_t out_size) {
  PrometheusSgemmPolicyDiagnostics full_diag;
  size_t copy_size;
  if (out_diag == NULL || out_size == 0u) return PROM_ERROR;
  memset(out_diag, 0, (size_t)out_size);
  if (handle == NULL || !registry_contains(handle)) return PROM_INVALID_HANDLE;
  if (((prometheus_runtime*)handle)->magic != PROMETHEUS_RUNTIME_MAGIC) return PROM_INVALID_HANDLE;
  if (prom_reactor_runtime_sgemm_policy_diagnostics_fill(handle, &full_diag) != PROM_OK) return PROM_ERROR;
  copy_size = (size_t)out_size < sizeof(full_diag) ? (size_t)out_size : sizeof(full_diag);
  memcpy(out_diag, &full_diag, copy_size);
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_policy_diagnostics_impl(void* handle, PrometheusSgemmPolicyDiagnostics* out_diag) {
  return prom_reactor_runtime_sgemm_policy_diagnostics_sized_impl(handle, out_diag, (uint32_t)sizeof(PrometheusSgemmPolicyDiagnostics));
}

int prom_reactor_runtime_sgemm_batch_diagnostics_impl(void* handle, PrometheusSgemmBatchDiagnostics* out_diag) {
  prometheus_runtime* rt;
  if (out_diag == NULL) {
    return PROM_ERROR;
  }
  memset(out_diag, 0, sizeof(*out_diag));
  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    return PROM_INVALID_HANDLE;
  }
  *out_diag = rt->batch_diag;
  return PROM_OK;
}
