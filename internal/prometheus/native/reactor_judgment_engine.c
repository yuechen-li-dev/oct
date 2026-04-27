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
