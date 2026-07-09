#include "reactor_judgment_engine.h"

#include <stddef.h>

typedef struct prom_judgment_candidate {
  prom_vk_path_mode path;
  prom_vk_compute_mode compute;
} prom_judgment_candidate;

static int candidate_detail_code(prom_vk_path_mode path, prom_vk_compute_mode compute, uint32_t fallback_used) {
  if (compute == PROM_VK_COMPUTE_PACKED4_FP32) {
    return PROM_DETAIL_PATH_DIRECT_PACKED4_FP32;
  }
  if (compute == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
    return PROM_DETAIL_PATH_DIRECT_FP16_STORAGE_FP32_ACCUM;
  }
  if (compute == PROM_VK_COMPUTE_TILED) {
    if (path == PROM_VK_PATH_DIRECT) {
      return PROM_DETAIL_PATH_DIRECT_TILED;
    }
    if (path == PROM_VK_PATH_STAGED_UPLOAD) {
      return PROM_DETAIL_PATH_STAGED_UPLOAD_TILED;
    }
    return PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED;
  }

  if (path == PROM_VK_PATH_DIRECT) {
    return fallback_used != 0u ? PROM_DETAIL_PATH_FALLBACK_TO_DIRECT : PROM_DETAIL_PATH_DIRECT;
  }
  if (path == PROM_VK_PATH_STAGED_UPLOAD) {
    return PROM_DETAIL_PATH_STAGED_UPLOAD;
  }
  return PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK;
}

static uint32_t candidate_path_feasible(prom_vk_path_mode path, const prom_judgment_facts* facts) {
  if (path == PROM_VK_PATH_DIRECT) {
    return facts->can_direct;
  }
  return facts->can_stage;
}

static uint32_t path_matches_request(prom_vk_path_mode path, prom_vk_path_mode requested) {
  return path == requested ? 1u : 0u;
}

void prom_judgment_engine_select_layout_precision(const prom_judgment_facts* facts,
                                                  prom_judgment_layout_precision_decision* out_decision) {
  if (out_decision == NULL) {
    return;
  }
  out_decision->packed4_selected = 0u;
  out_decision->packed4_reject_reason = PROM_PACKED4_REJECT_NONE;
  out_decision->fp16_selected = 0u;
  out_decision->fp16_reject_reason = PROM_FP16_REJECT_NONE;
  if (facts == NULL) {
    return;
  }

  if (facts->strict_fp32 != 0u) {
    out_decision->fp16_reject_reason = PROM_FP16_REJECT_STRICT_FP32;
  } else if (facts->tolerance_known == 0u) {
    out_decision->fp16_reject_reason = PROM_FP16_REJECT_TOLERANCE_UNKNOWN;
  } else if (facts->tolerance_pass == 0u) {
    out_decision->fp16_reject_reason = PROM_FP16_REJECT_TOLERANCE_EXCEEDED;
  } else if (facts->has_special_values != 0u) {
    out_decision->fp16_reject_reason = PROM_FP16_REJECT_SPECIAL_VALUE;
  } else if (facts->capability_fp16_storage == 0u) {
    out_decision->fp16_reject_reason = PROM_FP16_REJECT_CAPABILITY_MISSING;
  } else if (facts->fallback_available == 0u) {
    out_decision->fp16_reject_reason = PROM_FP16_REJECT_FALLBACK_REQUIRED;
  }

  if (facts->force_direct != 0u || facts->force_staged != 0u || facts->force_tiled != 0u || facts->tiled_shape != 0u) {
    out_decision->packed4_reject_reason = PROM_PACKED4_REJECT_FALLBACK_REQUIRED;
  } else if (facts->packed4_available == 0u) {
    out_decision->packed4_reject_reason = PROM_PACKED4_REJECT_CAPABILITY_MISSING;
  } else if (facts->allow_fallback == 0u || facts->can_direct == 0u) {
    out_decision->packed4_reject_reason = PROM_PACKED4_REJECT_FALLBACK_REQUIRED;
  } else if (facts->packed4_small_shape != 0u) {
    out_decision->packed4_reject_reason = PROM_PACKED4_REJECT_SMALL_SHAPE;
  } else if (facts->packed4_row_major_valid == 0u || facts->packed4_tail_valid == 0u) {
    out_decision->packed4_reject_reason = PROM_PACKED4_REJECT_CAPABILITY_MISSING;
  } else if (facts->packed4_padding_waste_permille > facts->packed4_mode_budget_permille) {
    out_decision->packed4_reject_reason =
        facts->policy_mode == PROM_POLICY_MODE_AGGRESSIVE ? PROM_PACKED4_REJECT_PADDING_WASTE
                                                          : PROM_PACKED4_REJECT_MODE_BUDGET_DENIED;
  } else if (out_decision->fp16_reject_reason == PROM_FP16_REJECT_NONE && facts->fp16_utility_score > 1000) {
    out_decision->fp16_selected = 1u;
  } else {
    if (out_decision->fp16_reject_reason == PROM_FP16_REJECT_NONE) {
      out_decision->fp16_reject_reason = PROM_FP16_REJECT_NOT_TOP_UTILITY;
    }
    out_decision->packed4_selected = 1u;
  }
}

void prom_judgment_engine_select_sgemm_mode_with_layout_precision(
    const prom_judgment_facts* facts,
    const prom_judgment_layout_precision_decision* layout_precision_decision,
    prom_judgment_decision* out_decision) {
  static const prom_judgment_candidate k_candidates[] = {
      {PROM_VK_PATH_DIRECT, PROM_VK_COMPUTE_BASELINE},
      {PROM_VK_PATH_DIRECT, PROM_VK_COMPUTE_TILED},
      {PROM_VK_PATH_STAGED_UPLOAD, PROM_VK_COMPUTE_BASELINE},
      {PROM_VK_PATH_STAGED_UPLOAD, PROM_VK_COMPUTE_TILED},
      {PROM_VK_PATH_STAGED_UPLOAD_READBACK, PROM_VK_COMPUTE_BASELINE},
      {PROM_VK_PATH_STAGED_UPLOAD_READBACK, PROM_VK_COMPUTE_TILED},
  };
  const uint32_t candidate_count = (uint32_t)(sizeof(k_candidates) / sizeof(k_candidates[0]));
  uint32_t desired_tiled;
  prom_vk_path_mode requested_path;
  int best_score;
  uint32_t best_index;
  uint32_t best_fallback;
  uint32_t candidate_index;

  if (out_decision == NULL) {
    return;
  }

  out_decision->success = 0u;
  out_decision->error_detail = PROM_DETAIL_CAPABILITY_MISMATCH;
  out_decision->requested_path = PROM_VK_PATH_DIRECT;
  out_decision->selected_path = PROM_VK_PATH_DIRECT;
  out_decision->compute_mode = PROM_VK_COMPUTE_BASELINE;
  out_decision->final_detail = PROM_DETAIL_CAPABILITY_MISMATCH;
  out_decision->used_fallback_to_direct = 0u;
  out_decision->winning_candidate_index = 0u;
  out_decision->winning_score = -100000;
  out_decision->packed4_selected = 0u;
  out_decision->packed4_reject_reason = PROM_PACKED4_REJECT_NONE;
  out_decision->fp16_selected = 0u;
  out_decision->fp16_reject_reason = PROM_FP16_REJECT_NONE;
  out_decision->use_dedicated_transfer_queue_upload = 0u;
  out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_NONE;

  if (facts == NULL) {
    return;
  }

  if (facts->force_direct != 0u) {
    requested_path = PROM_VK_PATH_DIRECT;
  } else if (facts->force_staged != 0u) {
    requested_path = facts->readback_required != 0u ? PROM_VK_PATH_STAGED_UPLOAD_READBACK : PROM_VK_PATH_STAGED_UPLOAD;
  } else if (facts->can_stage != 0u && facts->work_units >= (uint64_t)PROM_JUDGMENT_STAGING_WORK_THRESHOLD) {
    requested_path = facts->readback_required != 0u ? PROM_VK_PATH_STAGED_UPLOAD_READBACK : PROM_VK_PATH_STAGED_UPLOAD;
  } else {
    requested_path = PROM_VK_PATH_DIRECT;
  }
  out_decision->requested_path = requested_path;

  if (layout_precision_decision != NULL) {
    out_decision->packed4_selected = layout_precision_decision->packed4_selected;
    out_decision->packed4_reject_reason = layout_precision_decision->packed4_reject_reason;
    out_decision->fp16_selected = layout_precision_decision->fp16_selected;
    out_decision->fp16_reject_reason = layout_precision_decision->fp16_reject_reason;
  }

  if (out_decision->fp16_selected != 0u) {
    out_decision->success = 1u;
    out_decision->error_detail = 0;
    out_decision->requested_path = PROM_VK_PATH_DIRECT;
    out_decision->selected_path = PROM_VK_PATH_DIRECT;
    out_decision->compute_mode = PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM;
    out_decision->final_detail = PROM_DETAIL_PATH_DIRECT_FP16_STORAGE_FP32_ACCUM;
    out_decision->used_fallback_to_direct = 0u;
    out_decision->winning_candidate_index = UINT32_MAX;
    out_decision->winning_score = facts->fp16_utility_score;
    out_decision->fp16_selected = 1u;
    return;
  }
  if (out_decision->packed4_selected != 0u) {
    out_decision->success = 1u;
    out_decision->error_detail = 0;
    out_decision->requested_path = PROM_VK_PATH_DIRECT;
    out_decision->selected_path = PROM_VK_PATH_DIRECT;
    out_decision->compute_mode = PROM_VK_COMPUTE_PACKED4_FP32;
    out_decision->final_detail = PROM_DETAIL_PATH_DIRECT_PACKED4_FP32;
    out_decision->used_fallback_to_direct = 0u;
    out_decision->winning_candidate_index = UINT32_MAX;
    out_decision->winning_score = 1000;
    out_decision->packed4_selected = 1u;
    return;
  }

  desired_tiled = (facts->force_tiled != 0u || facts->tiled_shape != 0u) ? 1u : 0u;

  best_score = -100000;
  best_index = 0u;
  best_fallback = 0u;

  for (candidate_index = 0u; candidate_index < candidate_count; ++candidate_index) {
    const prom_judgment_candidate candidate = k_candidates[candidate_index];
    int score = 0;
    uint32_t fallback_used = 0u;

    if (candidate.path == PROM_VK_PATH_STAGED_UPLOAD && facts->readback_required != 0u) {
      continue;
    }
    if (candidate.path == PROM_VK_PATH_STAGED_UPLOAD_READBACK && facts->readback_required == 0u) {
      continue;
    }
    if ((requested_path == PROM_VK_PATH_STAGED_UPLOAD || requested_path == PROM_VK_PATH_STAGED_UPLOAD_READBACK) &&
        facts->can_stage == 0u && candidate.path == PROM_VK_PATH_DIRECT) {
      continue;
    }

    if (candidate_path_feasible(candidate.path, facts) == 0u) {
      if (candidate.path == PROM_VK_PATH_DIRECT) {
        continue;
      }
      if (facts->allow_fallback == 0u || facts->can_direct == 0u) {
        continue;
      }
      fallback_used = 1u;
    }

    if (fallback_used != 0u) {
      score += 200;
    } else {
      score += path_matches_request(candidate.path, requested_path) != 0u ? 500 : 0;
    }

    if (candidate.compute == PROM_VK_COMPUTE_TILED) {
      if (desired_tiled == 0u) {
        continue;
      }
      score += 20;
    } else if (desired_tiled == 0u) {
      score += 20;
    }

    if (candidate.path == PROM_VK_PATH_DIRECT) {
      score += 5;
    } else {
      score += 10;
      if (facts->software_vulkan != 0u) {
        score -= 2;
      }
    }

    if (score > best_score) {
      best_score = score;
      best_index = candidate_index;
      best_fallback = fallback_used;
    }
  }

  if (best_score <= -100000) {
    return;
  }

  out_decision->success = 1u;
  out_decision->error_detail = 0;
  out_decision->winning_candidate_index = best_index;
  out_decision->winning_score = best_score;
  out_decision->used_fallback_to_direct = best_fallback;
  out_decision->compute_mode = k_candidates[best_index].compute;
  out_decision->selected_path = best_fallback != 0u ? PROM_VK_PATH_DIRECT : k_candidates[best_index].path;
  out_decision->final_detail =
      candidate_detail_code(out_decision->selected_path, out_decision->compute_mode, out_decision->used_fallback_to_direct);

  if (out_decision->selected_path != PROM_VK_PATH_STAGED_UPLOAD) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_REQUIRED;
    return;
  }
  if (facts->transfer_queue_disabled_by_config != 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_DISABLED_BY_CONFIG;
    return;
  }
  if (facts->transfer_queue_dedicated_available == 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_NO_DEDICATED_QUEUE;
    return;
  }
  if (facts->transfer_queue_families_differ == 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_PSEUDO_SHARED_QUEUE;
    return;
  }
  if (facts->transfer_queue_supported == 0u || facts->transfer_overlap_slot_valid == 0u) {
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
  out_decision->use_dedicated_transfer_queue_upload = 1u;
  out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_NONE;
}

void prom_judgment_engine_select_sgemm_mode(const prom_judgment_facts* facts, prom_judgment_decision* out_decision) {
  prom_judgment_layout_precision_decision layout_precision_decision;
  prom_judgment_engine_select_layout_precision(facts, &layout_precision_decision);
  prom_judgment_engine_select_sgemm_mode_with_layout_precision(facts, &layout_precision_decision, out_decision);
}

void prom_judgment_engine_select_async_submission(const prom_judgment_async_facts* facts,
                                                  prom_judgment_async_decision* out_decision) {
  if (out_decision == NULL) {
    return;
  }

  out_decision->success = 0u;
  out_decision->execute_async = 0u;
  out_decision->reject_detail = PROM_DETAIL_ASYNC_SUBMIT_REJECTED;

  if (facts == NULL) {
    return;
  }
  if (facts->request_async == 0u) {
    out_decision->success = 1u;
    return;
  }
  if (facts->in_flight != 0u) {
    out_decision->reject_detail = PROM_DETAIL_REUSE_IN_FLIGHT;
    return;
  }
  if (facts->software_vulkan != 0u) {
    out_decision->reject_detail = PROM_DETAIL_ASYNC_SOFTWARE_SUPPRESSED;
    return;
  }

  out_decision->success = 1u;
  out_decision->execute_async = 1u;
  out_decision->reject_detail = 0;
}

void prom_judgment_engine_decide_resource_lease(const prom_resource_lease_facts* facts,
                                                prom_resource_lease_decision* out_decision) {
  uint32_t slot_mask;
  int grant_score;
  int backpressure_score;
  int lookahead_score;
  uint32_t hard_lookahead_blocked;
  if (out_decision == NULL) {
    return;
  }

  out_decision->success = 0u;
  out_decision->lease_state = PROM_LEASE_STATE_FAILED;
  out_decision->grant = 0u;
  out_decision->deny_reason = PROM_LEASE_REASON_FAILED;
  out_decision->resource_class = PROM_LEASE_RESOURCE_CLASS_COMPUTE;
  out_decision->worker_id = 0u;
  out_decision->slot_id = 0u;
  out_decision->entry_id = 0u;
  out_decision->allowed_outstanding_depth = 0u;
  out_decision->lookahead_allowed = 0u;
  out_decision->backpressure_applied = 0u;
  out_decision->yield_required = 0u;
  out_decision->selected_recipe_variant = 0u;
  out_decision->detail = PROM_LEASE_REASON_FAILED;

  if (facts == NULL) {
    return;
  }

  out_decision->success = 1u;
  out_decision->resource_class = facts->requested_resource_class;
  out_decision->worker_id = facts->worker_id;
  out_decision->slot_id = facts->slot_id;
  out_decision->entry_id = facts->entry_id;
  out_decision->allowed_outstanding_depth = facts->max_outstanding_depth;
  out_decision->selected_recipe_variant = facts->selected_recipe_variant;
  out_decision->yield_required = facts->yield_requested;

  if (facts->yield_requested != 0u) {
    if (facts->lease_held == 0u && facts->current_outstanding_depth == 0u && facts->max_outstanding_depth == 0u) {
      out_decision->lease_state = PROM_LEASE_STATE_DENIED;
      out_decision->deny_reason = PROM_LEASE_REASON_DENIED_OUTSTANDING_LIMIT;
      out_decision->backpressure_applied = 1u;
      out_decision->detail = PROM_LEASE_REASON_NO_YIELD_WITHOUT_HELD_LEASE;
      return;
    }
    out_decision->lease_state = PROM_LEASE_STATE_YIELDED;
    out_decision->deny_reason = PROM_LEASE_REASON_YIELDED;
    out_decision->detail = PROM_LEASE_REASON_HARD_YIELD_CRITICAL_SECTION_COMPLETE;
    return;
  }

  slot_mask = facts->slot_id < 32u ? (1u << facts->slot_id) : 0u;
  if (facts->single_call_mode != 0u) {
    if (facts->unsafe_to_reuse != 0u) {
      out_decision->lease_state = PROM_LEASE_STATE_DENIED;
      out_decision->deny_reason = PROM_LEASE_REASON_DENIED_UNSAFE_RUNTIME;
      out_decision->backpressure_applied = 1u;
      out_decision->detail = PROM_LEASE_REASON_HARD_DENY_SAFETY_OR_CAP;
      return;
    }
    if (facts->current_outstanding_depth >= facts->max_outstanding_depth) {
      out_decision->lease_state = PROM_LEASE_STATE_DENIED;
      out_decision->deny_reason = PROM_LEASE_REASON_DENIED_OUTSTANDING_LIMIT;
      out_decision->backpressure_applied = 1u;
      out_decision->detail = PROM_LEASE_REASON_HARD_DENY_SAFETY_OR_CAP;
      return;
    }
    out_decision->lease_state = PROM_LEASE_STATE_GRANTED;
    out_decision->grant = 1u;
    out_decision->deny_reason = PROM_LEASE_REASON_GRANTED;
    out_decision->detail = PROM_LEASE_REASON_UTILITY_GRANT_READY_AND_SAFE;
    return;
  }
  if (slot_mask != 0u && (facts->failed_slot_mask & slot_mask) != 0u) {
    out_decision->lease_state = PROM_LEASE_STATE_DENIED;
    out_decision->deny_reason = PROM_LEASE_REASON_DENIED_SLOT_FAILED;
    out_decision->backpressure_applied = 1u;
    out_decision->detail = PROM_LEASE_REASON_HARD_DENY_SAFETY_OR_CAP;
    return;
  }
  if (slot_mask != 0u && (facts->invalidated_slot_mask & slot_mask) != 0u) {
    out_decision->lease_state = PROM_LEASE_STATE_DENIED;
    out_decision->deny_reason = PROM_LEASE_REASON_DENIED_SLOT_INVALIDATED;
    out_decision->backpressure_applied = 1u;
    out_decision->detail = PROM_LEASE_REASON_HARD_DENY_SAFETY_OR_CAP;
    return;
  }
  if (facts->unsafe_to_reuse != 0u) {
    out_decision->lease_state = PROM_LEASE_STATE_DENIED;
    out_decision->deny_reason = PROM_LEASE_REASON_DENIED_UNSAFE_RUNTIME;
    out_decision->backpressure_applied = 1u;
    out_decision->detail = PROM_LEASE_REASON_HARD_DENY_SAFETY_OR_CAP;
    return;
  }
  if (facts->current_outstanding_depth >= facts->max_outstanding_depth) {
    out_decision->lease_state = PROM_LEASE_STATE_DENIED;
    out_decision->deny_reason = PROM_LEASE_REASON_DENIED_OUTSTANDING_LIMIT;
    out_decision->backpressure_applied = 1u;
    out_decision->detail = PROM_LEASE_REASON_HARD_DENY_SAFETY_OR_CAP;
    return;
  }
  hard_lookahead_blocked = (facts->current_outstanding_depth >= facts->lookahead_limit || facts->unsafe_to_reuse != 0u ||
                            (slot_mask != 0u && (facts->failed_slot_mask & slot_mask) != 0u) ||
                            (slot_mask != 0u && (facts->invalidated_slot_mask & slot_mask) != 0u) ||
                            (facts->requested_resource_class == PROM_LEASE_RESOURCE_CLASS_TRANSFER &&
                             (facts->transfer_overlap_available == 0u || facts->true_multi_queue_selected == 0u)))
                               ? 1u
                               : 0u;

  grant_score = 50 + (facts->ready_slot_mask & slot_mask ? 20 : 0) + (facts->slot_attention_mask & slot_mask ? 15 : 0);
  backpressure_score = 20 + (int)facts->register_pressure_class * 7 + (int)facts->shared_memory_pressure_class * 7 +
                       (int)facts->memory_bandwidth_pressure_class * 6 + (int)facts->compute_pressure_class * 5;
  lookahead_score = facts->lookahead_requested != 0u ? 10 : 0;
  lookahead_score += facts->pipeline_latency_pressure_class >= 3u ? 25 : -5;
  if (facts->requested_resource_class == PROM_LEASE_RESOURCE_CLASS_TRANSFER &&
      (facts->transfer_overlap_available == 0u || facts->true_multi_queue_selected == 0u)) {
    backpressure_score += 40;
  }
  if (facts->device_band == PROM_OCCUPANCY_DEVICE_BAND_REGISTER_CONSTRAINED && facts->register_pressure_class >= 3u) {
    grant_score -= 18;
  } else if (facts->device_band == PROM_OCCUPANCY_DEVICE_BAND_COMPUTE_RICH) {
    grant_score += 10;
  }
  if (facts->shape_class == PROM_OCCUPANCY_SHAPE_CLASS_SMALL_SQUARE) {
    lookahead_score += 10;
  } else if (facts->shape_class == PROM_OCCUPANCY_SHAPE_CLASS_LARGE_SQUARE) {
    grant_score += 8;
  }
  if (facts->selected_recipe_variant == PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) {
    backpressure_score += 12;
  } else if (facts->selected_recipe_variant == PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE) {
    grant_score += 12;
  }
  /* M14 audit follow-up: fairness ownership moved out of judgment engine.
   * Keep this function pure (facts -> decision) and deterministic across threads. */
  if (facts->current_outstanding_depth == 0u) {
    grant_score += 4;
  } else if (facts->current_outstanding_depth >= facts->max_outstanding_depth) {
    grant_score -= 6;
  }

  if (grant_score >= backpressure_score) {
    out_decision->lease_state = PROM_LEASE_STATE_GRANTED;
    out_decision->grant = 1u;
    out_decision->deny_reason = PROM_LEASE_REASON_GRANTED;
    out_decision->detail = PROM_LEASE_REASON_UTILITY_GRANT_READY_AND_SAFE;
  } else {
    out_decision->lease_state = PROM_LEASE_STATE_DENIED;
    out_decision->grant = 0u;
    out_decision->backpressure_applied = 1u;
    out_decision->deny_reason = PROM_LEASE_REASON_DENIED_RESOURCE_PRESSURE;
    out_decision->detail = (backpressure_score >= 40) ? PROM_LEASE_REASON_UTILITY_BACKPRESSURE_PRESSURE_OR_CONTENTION
                                                       : PROM_LEASE_REASON_UTILITY_BACKPRESSURE_DEFAULT;
  }
  if (facts->lookahead_requested != 0u && hard_lookahead_blocked == 0u && lookahead_score > 0) {
    out_decision->lookahead_allowed = 1u;
    if (facts->pipeline_latency_pressure_class >= 3u) {
      out_decision->detail = PROM_LEASE_REASON_UTILITY_ALLOW_LOOKAHEAD_LATENCY_DOMINANT;
    }
  } else if (facts->lookahead_requested != 0u && hard_lookahead_blocked != 0u) {
    out_decision->lookahead_allowed = 0u;
    out_decision->detail = PROM_LEASE_REASON_HARD_BLOCK_LOOKAHEAD_LIMIT_OR_TRANSFER;
  }
}

prom_policy_mode prom_judgment_engine_update_policy_mode(prom_policy_memory* memory,
                                                         const prom_policy_facts* facts,
                                                         const prom_policy_thresholds* thresholds) {
  return prom_policy_memory_update(memory, facts, thresholds);
}

void prom_judgment_engine_select_buffering_mode(const prom_buffering_selector_facts* facts,
                                                prom_buffering_selector_decision* out_decision) {
  if (out_decision == NULL) {
    return;
  }

  out_decision->success = 0u;
  out_decision->selected_mode = PROM_BUFFERING_MODE_NONE;
  out_decision->reason_code = PROM_BUFFERING_REASON_NO_BUFFERING_MODE_FEASIBLE;
  out_decision->final_reason_code = PROM_BUFFERING_REASON_NO_BUFFERING_MODE_FEASIBLE;
  out_decision->fixed_double_rejection_reason = PROM_BUFFERING_REASON_NONE;
  out_decision->pull_lag_rejection_reason = PROM_BUFFERING_REASON_NONE;
  out_decision->serial_jit_rejection_reason = PROM_BUFFERING_REASON_NONE;
  out_decision->fixed_feasible = 0u;
  out_decision->pull_lag_feasible = 0u;
  out_decision->serial_feasible = 0u;
  out_decision->fixed_rejected = 1u;
  out_decision->pull_lag_rejected = 1u;
  out_decision->serial_rejected = 1u;
  out_decision->fixed_score = -100000;
  out_decision->pull_lag_score = -100000;
  out_decision->serial_score = -100000;

  if (facts == NULL) {
    return;
  }

  out_decision->fixed_feasible = facts->memory_budget_slots_permille >= facts->required_fixed_slots_permille ? 1u : 0u;
  out_decision->serial_feasible =
      facts->fallback_available != 0u && facts->memory_budget_slots_permille >= facts->required_serial_slots_permille ? 1u : 0u;
  out_decision->pull_lag_feasible = facts->memory_budget_slots_permille >= facts->required_pull_lag_peak_slots_permille ? 1u : 0u;
  if (out_decision->pull_lag_feasible == 0u) {
    out_decision->pull_lag_rejection_reason = PROM_BUFFERING_REASON_PULL_LAG_MEMORY_INSUFFICIENT;
  }
  if (out_decision->pull_lag_feasible != 0u && facts->transfer_variance_class == PROM_VARIANCE_HIGH) {
    out_decision->pull_lag_feasible = 0u;
    out_decision->pull_lag_rejection_reason = PROM_BUFFERING_REASON_PULL_LAG_VARIANCE_MISS;
  }
  if (out_decision->pull_lag_feasible != 0u && facts->compute_predictability_class == PROM_PREDICTABILITY_UNSTABLE) {
    out_decision->pull_lag_feasible = 0u;
    out_decision->pull_lag_rejection_reason = PROM_BUFFERING_REASON_PULL_LAG_COMPUTE_UNSTABLE;
  }
  if (out_decision->pull_lag_feasible != 0u && facts->starvation_risk_high != 0u) {
    out_decision->pull_lag_feasible = 0u;
    out_decision->pull_lag_rejection_reason = PROM_BUFFERING_REASON_PULL_LAG_LATE_STAGE_STARVATION;
  }
  if (out_decision->pull_lag_feasible != 0u && facts->pull_lag_wip_waste_exceeded != 0u) {
    out_decision->pull_lag_feasible = 0u;
    out_decision->pull_lag_rejection_reason = PROM_BUFFERING_REASON_PULL_LAG_WIP_WASTE_EXCEEDED;
  }
  if (out_decision->pull_lag_feasible != 0u && facts->pull_lag_headroom_slots_permille < 0) {
    out_decision->pull_lag_feasible = 0u;
    out_decision->pull_lag_rejection_reason = PROM_BUFFERING_REASON_PULL_LAG_MEMORY_EDGE_REJECTED;
  }

  if (out_decision->fixed_feasible != 0u) {
    out_decision->fixed_score = 1000 + facts->fixed_double_headroom_slots_permille;
    out_decision->fixed_rejected = 0u;
  } else {
    out_decision->fixed_double_rejection_reason = PROM_BUFFERING_REASON_FIXED_DOUBLE_MEMORY_INSUFFICIENT;
  }

  if (out_decision->pull_lag_feasible != 0u) {
    out_decision->pull_lag_score = 700 + facts->pull_lag_headroom_slots_permille;
    out_decision->pull_lag_rejected = 0u;
  }

  if (out_decision->serial_feasible != 0u) {
    out_decision->serial_score = 300 + facts->serial_jit_headroom_slots_permille;
    out_decision->serial_rejected = 0u;
  } else {
    out_decision->serial_jit_rejection_reason = PROM_BUFFERING_REASON_SERIAL_JIT_MEMORY_INSUFFICIENT;
  }

  if (out_decision->fixed_feasible != 0u) {
    out_decision->success = 1u;
    out_decision->selected_mode = PROM_BUFFERING_MODE_FIXED_DOUBLE_DEFAULT;
    out_decision->final_reason_code = PROM_BUFFERING_REASON_FIXED_DOUBLE_SELECTED;
    out_decision->reason_code = out_decision->final_reason_code;
    return;
  }
  if (out_decision->pull_lag_feasible != 0u) {
    out_decision->success = 1u;
    out_decision->selected_mode = PROM_BUFFERING_MODE_PULL_LAG_PRESSURE;
    out_decision->final_reason_code = PROM_BUFFERING_REASON_PULL_LAG_SELECTED;
    out_decision->reason_code = out_decision->final_reason_code;
    return;
  }
  if (out_decision->serial_feasible != 0u) {
    out_decision->success = 1u;
    out_decision->selected_mode = PROM_BUFFERING_MODE_SERIAL_JIT_SURVIVAL;
    out_decision->final_reason_code = PROM_BUFFERING_REASON_SERIAL_JIT_SELECTED;
    out_decision->reason_code = out_decision->final_reason_code;
    return;
  }

  out_decision->success = 0u;
  out_decision->selected_mode = PROM_BUFFERING_MODE_NONE;
  out_decision->final_reason_code = PROM_BUFFERING_REASON_NO_BUFFERING_MODE_FEASIBLE;
  out_decision->reason_code = out_decision->final_reason_code;
}

static uint32_t occupancy_variant_valid(uint32_t variant) {
  return variant >= (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR &&
                 variant <= (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_SCALAR_PLUS
             ? 1u
             : 0u;
}

static uint32_t occupancy_shape_is_small(const prom_occupancy_selector_facts* facts) {
  return (facts->m <= 256u && facts->n <= 256u && facts->k <= 256u) ? 1u : 0u;
}

typedef struct prom_occ_shape_features {
  uint32_t is_wide;
  uint32_t is_tall_or_skinny;
  uint32_t is_low_k;
  uint32_t is_small;
  uint32_t is_medium;
  uint32_t is_large_square;
  uint32_t is_rectangular;
  uint32_t is_awkward_or_odd;
} prom_occ_shape_features;

typedef struct prom_occ_variant_utility_config {
  int32_t baseline_scalar_score;
  int32_t small_register_tile_score;
  int32_t balanced_2x2_score;
  int32_t aggressive_4x4_score;
  int32_t memory_conservative_score;

  int32_t register_constrained_memory_conservative_bonus;
  int32_t register_constrained_small_register_tile_bonus;
  int32_t register_constrained_aggressive_penalty;
  int32_t balanced_small_register_tile_bonus;
  int32_t balanced_2x2_bonus;
  int32_t compute_rich_aggressive_bonus;
  int32_t compute_rich_balanced_bonus;
  int32_t compute_rich_memory_conservative_bonus;
  int32_t memory_rich_balanced_bonus;
  int32_t memory_rich_small_register_tile_bonus;

  int32_t memory_conservative_wide_bonus;
  int32_t memory_conservative_rectangular_bonus;
  int32_t memory_conservative_low_k_bonus;
  int32_t memory_conservative_awkward_bonus;
  int32_t memory_conservative_large_square_penalty;
  int32_t small_register_tile_wide_bonus;
  int32_t small_register_tile_small_shape_bonus;
  int32_t small_register_tile_medium_shape_bonus;
  int32_t balanced_rectangular_bonus;
  int32_t balanced_low_k_bonus;
  int32_t balanced_large_square_bonus;
  int32_t balanced_small_shape_penalty;
  int32_t aggressive_large_square_bonus;
  int32_t aggressive_ffn_like_bonus;
  int32_t aggressive_wide_penalty;
  int32_t aggressive_low_k_penalty;
} prom_occ_variant_utility_config;

/* Px16 M10 DVT-tunable selector policy.
 * These constants are intentionally centralized so real hardware feedback can be
 * folded into production selection without rewriting the selector. RTX 3070 DVT
 * showed MEMORY_CONSERVATIVE is not merely a register-constrained fallback: it
 * can win on high-capability devices for wide, rectangular, low-K, and awkward
 * shapes while square/FFN-like shapes still need the register-blocked variants.
 */
static const prom_occ_variant_utility_config k_occupancy_utility = {
    15,  /* baseline_scalar_score */
    60,  /* small_register_tile_score */
    70,  /* balanced_2x2_score */
    75,  /* aggressive_4x4_score */
    45,  /* memory_conservative_score */

    65,  /* register_constrained_memory_conservative_bonus */
    25,  /* register_constrained_small_register_tile_bonus */
    -80, /* register_constrained_aggressive_penalty */
    20,  /* balanced_small_register_tile_bonus */
    20,  /* balanced_2x2_bonus */
    35,  /* compute_rich_aggressive_bonus */
    25,  /* compute_rich_balanced_bonus */
    10,  /* compute_rich_memory_conservative_bonus */
    35,  /* memory_rich_balanced_bonus */
    15,  /* memory_rich_small_register_tile_bonus */

    90,  /* memory_conservative_wide_bonus */
    35,  /* memory_conservative_rectangular_bonus */
    35,  /* memory_conservative_low_k_bonus */
    45,  /* memory_conservative_awkward_bonus */
    -25, /* memory_conservative_large_square_penalty */
    20,  /* small_register_tile_wide_bonus */
    55,  /* small_register_tile_small_shape_bonus */
    15,  /* small_register_tile_medium_shape_bonus */
    15,  /* balanced_rectangular_bonus */
    55,  /* balanced_low_k_bonus */
    45,  /* balanced_large_square_bonus */
    -15, /* balanced_small_shape_penalty */
    65,  /* aggressive_large_square_bonus */
    45,  /* aggressive_ffn_like_bonus */
    -25, /* aggressive_wide_penalty */
    -35, /* aggressive_low_k_penalty */
};

static prom_occ_shape_features occupancy_shape_features(const prom_occupancy_selector_facts* facts, uint32_t shape_class) {
  prom_occ_shape_features features;
  const uint32_t max_mn = facts->m > facts->n ? facts->m : facts->n;
  const uint32_t min_mn = facts->m < facts->n ? facts->m : facts->n;
  features.is_wide = shape_class == (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_WIDE_SHORT ? 1u : 0u;
  features.is_tall_or_skinny = shape_class == (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_TALL_SKINNY ? 1u : 0u;
  features.is_low_k = facts->k <= 128u || (facts->k * 4u <= max_mn && facts->m >= 256u && facts->n >= 256u) ? 1u : 0u;
  features.is_small = occupancy_shape_is_small(facts);
  features.is_medium = shape_class == (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_MEDIUM_SQUARE ? 1u : 0u;
  features.is_large_square = shape_class == (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_LARGE_SQUARE ? 1u : 0u;
  features.is_rectangular = ((uint64_t)max_mn * 2u >= (uint64_t)min_mn * 3u) ? 1u : 0u;
  features.is_awkward_or_odd =
      ((facts->m & 7u) != 0u || (facts->n & 7u) != 0u || (facts->k & 7u) != 0u) ? 1u : 0u;
  return features;
}

static uint32_t occupancy_shape_classify(const prom_occupancy_selector_facts* facts) {
  const uint32_t max_mn = facts->m > facts->n ? facts->m : facts->n;
  const uint32_t min_mn = facts->m < facts->n ? facts->m : facts->n;
  if (max_mn >= 4u * min_mn && min_mn <= 256u) {
    return facts->m >= facts->n ? (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_TALL_SKINNY
                                : (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_WIDE_SHORT;
  }
  if (facts->k >= 2u * max_mn && facts->m >= 256u && facts->n >= 256u) {
    if ((facts->m >= 128u && facts->n >= 1024u) || (facts->n >= 128u && facts->m >= 1024u)) {
      return (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_ML_FFN_LIKE;
    }
    return (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_K_HEAVY;
  }
  if (occupancy_shape_is_small(facts) != 0u) {
    return (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_SMALL_SQUARE;
  }
  if (facts->m >= 1536u && facts->n >= 1536u) {
    return (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_LARGE_SQUARE;
  }
  if ((facts->m >= 512u && facts->n >= 512u && facts->k >= 1024u) || (facts->m >= 1024u && facts->n >= 512u)) {
    return (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_ML_FFN_LIKE;
  }
  return (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_MEDIUM_SQUARE;
}

static uint32_t occupancy_device_band_classify(const prom_occupancy_selector_facts* facts, uint32_t* fallback_used) {
  const uint32_t known = facts->register_file_class != 0u && facts->shared_memory_class != 0u && facts->memory_bandwidth_class != 0u &&
                         facts->fp32_throughput_class != 0u && facts->max_workgroup_class != 0u && facts->queue_capability_class != 0u;
  const uint32_t register_tolerance = (facts->register_file_class + facts->max_workgroup_class) / 2u;
  const uint32_t shared_tolerance = (facts->shared_memory_class + facts->queue_capability_class) / 2u;
  const int32_t compute_vs_memory_bias = (int32_t)facts->fp32_throughput_class - (int32_t)facts->memory_bandwidth_class;
  if (known == 0u) {
    *fallback_used = 1u;
    return (uint32_t)PROM_OCCUPANCY_DEVICE_BAND_BALANCED;
  }
  if (register_tolerance <= 2u) {
    return (uint32_t)PROM_OCCUPANCY_DEVICE_BAND_REGISTER_CONSTRAINED;
  }
  if (compute_vs_memory_bias >= 2 && register_tolerance >= 4u) {
    return (uint32_t)PROM_OCCUPANCY_DEVICE_BAND_COMPUTE_RICH;
  }
  if (facts->memory_bandwidth_class >= 4u && shared_tolerance >= 4u) {
    return (uint32_t)PROM_OCCUPANCY_DEVICE_BAND_MEMORY_RICH;
  }
  return (uint32_t)PROM_OCCUPANCY_DEVICE_BAND_BALANCED;
}

static int32_t occupancy_variant_utility_score(uint32_t variant,
                                               uint32_t band,
                                               uint32_t shape_class,
                                               const prom_occ_shape_features* features) {
  int32_t score = -100000;
  if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) {
    score = k_occupancy_utility.baseline_scalar_score;
  } else if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE) {
    score = k_occupancy_utility.small_register_tile_score;
  } else if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4) {
    score = k_occupancy_utility.balanced_2x2_score;
  } else if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) {
    score = k_occupancy_utility.aggressive_4x4_score;
  } else if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE) {
    score = k_occupancy_utility.memory_conservative_score;
  } else {
    return score;
  }

  if (band == (uint32_t)PROM_OCCUPANCY_DEVICE_BAND_REGISTER_CONSTRAINED) {
    if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE) {
      score += k_occupancy_utility.register_constrained_memory_conservative_bonus;
    } else if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE) {
      score += k_occupancy_utility.register_constrained_small_register_tile_bonus;
    } else if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) {
      score += k_occupancy_utility.register_constrained_aggressive_penalty;
    }
  } else if (band == (uint32_t)PROM_OCCUPANCY_DEVICE_BAND_COMPUTE_RICH) {
    if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) {
      score += (shape_class == (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_LARGE_SQUARE ||
                shape_class == (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_K_HEAVY ||
                shape_class == (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_ML_FFN_LIKE)
                   ? k_occupancy_utility.compute_rich_aggressive_bonus
                   : 0;
    } else if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4) {
      score += k_occupancy_utility.compute_rich_balanced_bonus;
    } else if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE) {
      score += k_occupancy_utility.compute_rich_memory_conservative_bonus;
    }
  } else if (band == (uint32_t)PROM_OCCUPANCY_DEVICE_BAND_MEMORY_RICH) {
    if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4) {
      score += k_occupancy_utility.memory_rich_balanced_bonus;
    } else if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE) {
      score += k_occupancy_utility.memory_rich_small_register_tile_bonus;
    }
  } else {
    if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE) {
      score += k_occupancy_utility.balanced_small_register_tile_bonus;
    } else if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4) {
      score += k_occupancy_utility.balanced_2x2_bonus;
    }
  }

  if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE) {
    score += (features->is_wide != 0u || features->is_tall_or_skinny != 0u) ? k_occupancy_utility.memory_conservative_wide_bonus : 0;
    score += features->is_rectangular != 0u ? k_occupancy_utility.memory_conservative_rectangular_bonus : 0;
    score += features->is_low_k != 0u ? k_occupancy_utility.memory_conservative_low_k_bonus : 0;
    score += features->is_awkward_or_odd != 0u ? k_occupancy_utility.memory_conservative_awkward_bonus : 0;
    score += features->is_large_square != 0u ? k_occupancy_utility.memory_conservative_large_square_penalty : 0;
  } else if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE) {
    score += (features->is_wide != 0u || features->is_tall_or_skinny != 0u) ? k_occupancy_utility.small_register_tile_wide_bonus : 0;
    score += features->is_small != 0u ? k_occupancy_utility.small_register_tile_small_shape_bonus : 0;
    score += features->is_medium != 0u ? k_occupancy_utility.small_register_tile_medium_shape_bonus : 0;
  } else if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4) {
    score += features->is_rectangular != 0u ? k_occupancy_utility.balanced_rectangular_bonus : 0;
    score += features->is_low_k != 0u ? k_occupancy_utility.balanced_low_k_bonus : 0;
    score += features->is_large_square != 0u ? k_occupancy_utility.balanced_large_square_bonus : 0;
    score += features->is_small != 0u ? k_occupancy_utility.balanced_small_shape_penalty : 0;
  } else if (variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) {
    score += features->is_large_square != 0u ? k_occupancy_utility.aggressive_large_square_bonus : 0;
    score += shape_class == (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_ML_FFN_LIKE ? k_occupancy_utility.aggressive_ffn_like_bonus : 0;
    score += (features->is_wide != 0u || features->is_tall_or_skinny != 0u) ? k_occupancy_utility.aggressive_wide_penalty : 0;
    score += features->is_low_k != 0u ? k_occupancy_utility.aggressive_low_k_penalty : 0;
  }

  return score;
}

static uint32_t occupancy_select_unclamped_variant(const prom_occupancy_selector_facts* facts, uint32_t band, uint32_t shape_class) {
  static const uint32_t k_candidates[] = {
      (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR,
      (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE,
      (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4,
      (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8,
      (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE,
  };
  const uint32_t candidate_count = (uint32_t)(sizeof(k_candidates) / sizeof(k_candidates[0]));
  const prom_occ_shape_features features = occupancy_shape_features(facts, shape_class);
  uint32_t best_variant = (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR;
  int32_t best_score = -100000;
  uint32_t candidate_index;
  for (candidate_index = 0u; candidate_index < candidate_count; ++candidate_index) {
    const uint32_t variant = k_candidates[candidate_index];
    const int32_t score = occupancy_variant_utility_score(variant, band, shape_class, &features);
    if (score > best_score) {
      best_score = score;
      best_variant = variant;
    }
  }
  return best_variant;
}

static uint32_t occupancy_apply_safety_clamp(const prom_occupancy_selector_facts* facts,
                                             uint32_t shape_class,
                                             uint32_t requested_variant,
                                             uint32_t* out_reason) {
  uint32_t variant = requested_variant;
  *out_reason = (uint32_t)PROM_OCCUPANCY_REASON_DEFAULT_BAND_SELECTION;
  if (occupancy_shape_is_small(facts) != 0u && variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) {
    variant = (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4;
    *out_reason = (uint32_t)PROM_OCCUPANCY_REASON_SHAPE_SMALL_CLAMP;
  }
  if ((facts->register_file_class <= 2u || facts->max_workgroup_class <= 2u) &&
      variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) {
    variant = (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE;
    *out_reason = (uint32_t)PROM_OCCUPANCY_REASON_LOW_REGISTER_CLAMP;
  }
  if (facts->shared_memory_class <= 2u && variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) {
    variant = (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4;
    *out_reason = (uint32_t)PROM_OCCUPANCY_REASON_SHARED_MEMORY_CLAMP;
  }
  if (shape_class == (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_SMALL_SQUARE &&
      variant == (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4) {
    variant = (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE;
    *out_reason = (uint32_t)PROM_OCCUPANCY_REASON_SHAPE_SMALL_CLAMP;
  }
  return variant;
}

void prom_judgment_engine_select_occupancy_variant(const prom_occupancy_selector_facts* facts,
                                                   prom_occupancy_selector_decision* out_decision) {
  uint32_t fallback_used = 0u;
  uint32_t device_band = (uint32_t)PROM_OCCUPANCY_DEVICE_BAND_BALANCED;
  uint32_t shape_class = (uint32_t)PROM_OCCUPANCY_SHAPE_CLASS_MEDIUM_SQUARE;
  uint32_t unclamped_variant = (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR;
  uint32_t selected_variant = (uint32_t)PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR;
  uint32_t clamp_reason = (uint32_t)PROM_OCCUPANCY_REASON_FALLBACK_BASELINE;
  if (out_decision == NULL) {
    return;
  }
  out_decision->success = 0u;
  out_decision->device_band = device_band;
  out_decision->shape_class = shape_class;
  out_decision->selected_variant = selected_variant;
  out_decision->unclamped_variant = unclamped_variant;
  out_decision->clamp_reason = clamp_reason;
  out_decision->override_used = 0u;
  out_decision->fallback_used = 1u;
  if (facts == NULL || facts->m == 0u || facts->n == 0u || facts->k == 0u || facts->work_units == 0u) {
    return;
  }
  device_band = occupancy_device_band_classify(facts, &fallback_used);
  shape_class = occupancy_shape_classify(facts);
  unclamped_variant = occupancy_select_unclamped_variant(facts, device_band, shape_class);
  selected_variant = occupancy_apply_safety_clamp(facts, shape_class, unclamped_variant, &clamp_reason);
  if (facts->manual_override_enabled != 0u) {
    uint32_t override_reason = (uint32_t)PROM_OCCUPANCY_REASON_NONE;
    if (occupancy_variant_valid(facts->manual_override_variant) != 0u) {
      const uint32_t override_variant =
          occupancy_apply_safety_clamp(facts, shape_class, facts->manual_override_variant, &override_reason);
      if (override_variant == facts->manual_override_variant) {
        unclamped_variant = facts->manual_override_variant;
        selected_variant = facts->manual_override_variant;
        clamp_reason = (uint32_t)PROM_OCCUPANCY_REASON_MANUAL_OVERRIDE_USED;
        out_decision->override_used = 1u;
      } else {
        clamp_reason = (uint32_t)PROM_OCCUPANCY_REASON_OVERRIDE_REJECTED;
      }
    } else {
      clamp_reason = (uint32_t)PROM_OCCUPANCY_REASON_OVERRIDE_REJECTED;
    }
  }
  if (fallback_used != 0u) {
    clamp_reason = (uint32_t)PROM_OCCUPANCY_REASON_UNKNOWN_DEVICE_FALLBACK;
  }
  out_decision->success = 1u;
  out_decision->device_band = device_band;
  out_decision->shape_class = shape_class;
  out_decision->selected_variant = selected_variant;
  out_decision->unclamped_variant = unclamped_variant;
  out_decision->clamp_reason = clamp_reason;
  out_decision->fallback_used = fallback_used;
}
