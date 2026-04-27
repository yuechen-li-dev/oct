#include "reactor_dominatus_sgemm_adapter.h"
#include <string.h>

static uint64_t m35_dependency_mask_last_commit(const prom_dom_blackboard* board) {
  uint64_t mask = 0u;
  if (board == 0) {
    return 0u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_MEMORY_BUDGET) != 0u) {
    mask |= 1ull << 0u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_MEMORY_M35_FIXED_HEADROOM) != 0u) {
    mask |= 1ull << 1u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_MEMORY_M35_PULL_LAG_HEADROOM) != 0u) {
    mask |= 1ull << 2u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_MEMORY_M35_SERIAL_HEADROOM) != 0u) {
    mask |= 1ull << 3u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_MEMORY_M35_REQUIRED_FIXED) != 0u) {
    mask |= 1ull << 4u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_MEMORY_M35_REQUIRED_PULL_LAG) != 0u) {
    mask |= 1ull << 5u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_MEMORY_M35_REQUIRED_SERIAL) != 0u) {
    mask |= 1ull << 6u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_M35_TRANSFER_VARIANCE_CLASS) != 0u) {
    mask |= 1ull << 7u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_M35_COMPUTE_PREDICTABILITY_CLASS) != 0u) {
    mask |= 1ull << 8u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_M35_STARVATION_RISK_HIGH) != 0u) {
    mask |= 1ull << 9u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_M35_PULL_LAG_WIP_WASTE_EXCEEDED) != 0u) {
    mask |= 1ull << 10u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_M35_FALLBACK_AVAILABLE) != 0u) {
    mask |= 1ull << 11u;
  }
  return mask;
}

static uint64_t transfer_queue_dependency_mask_last_commit(const prom_dom_blackboard* board) {
  uint64_t mask = 0u;
  if (board == 0) {
    return 0u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_QUEUE_DEDICATED_AVAILABLE) != 0u) {
    mask |= 1ull << 0u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_QUEUE_TRANSFER_FAMILY) != 0u) {
    mask |= 1ull << 1u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_QUEUE_COMPUTE_FAMILY) != 0u) {
    mask |= 1ull << 2u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_QUEUE_FAMILIES_DIFFER) != 0u) {
    mask |= 1ull << 3u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_QUEUE_TRANSFER_SUPPORTED) != 0u) {
    mask |= 1ull << 4u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_QUEUE_TRANSFER_DISABLED_BY_CONFIG) != 0u) {
    mask |= 1ull << 5u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_QUEUE_TRANSFER_WORKLOAD_LARGE_ENOUGH) != 0u) {
    mask |= 1ull << 6u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_QUEUE_TRANSFER_SYNC_OWNERSHIP_SUPPORTED) != 0u) {
    mask |= 1ull << 7u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_QUEUE_TRANSFER_FALLBACK_AVAILABLE) != 0u) {
    mask |= 1ull << 8u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_ONLY_ELIGIBLE) != 0u) {
    mask |= 1ull << 9u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_READBACK_SUPPORTED) != 0u) {
    mask |= 1ull << 10u;
  }
  return mask;
}

static uint64_t layout_precision_dependency_mask_last_commit(const prom_dom_blackboard* board) {
  uint64_t mask = 0u;
  if (board == 0) {
    return 0u;
  }
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_PACKED4_AVAILABLE) != 0u) mask |= 1ull << 0u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_PACKED4_SMALL_SHAPE) != 0u) mask |= 1ull << 1u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_PACKED4_PADDING_WASTE_PERMILLE) != 0u) mask |= 1ull << 2u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_PACKED4_MODE_BUDGET_PERMILLE) != 0u) mask |= 1ull << 3u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_PACKED4_ROW_MAJOR_VALID) != 0u) mask |= 1ull << 4u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_PACKED4_TAIL_VALID) != 0u) mask |= 1ull << 5u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FP16_STRICT_FP32) != 0u) mask |= 1ull << 6u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FP16_TOLERANCE_KNOWN) != 0u) mask |= 1ull << 7u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FP16_TOLERANCE_PASS) != 0u) mask |= 1ull << 8u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FP16_HAS_SPECIAL_VALUES) != 0u) mask |= 1ull << 9u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FP16_CAPABILITY_STORAGE) != 0u) mask |= 1ull << 10u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FP16_FALLBACK_AVAILABLE) != 0u) mask |= 1ull << 11u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FP16_UTILITY_SCORE) != 0u) mask |= 1ull << 12u;
  return mask;
}

static uint32_t float_to_bits(float value) {
  uint32_t bits = 0u;
  memcpy(&bits, &value, sizeof(bits));
  return bits;
}

static float bits_to_float(uint32_t bits) {
  float value = 0.0f;
  memcpy(&value, &bits, sizeof(value));
  return value;
}

uint32_t prom_dom_sgemm_stage_m35_facts(prom_dom_blackboard* board, const prom_buffering_selector_facts* facts) {
  if (board == 0 || facts == 0) {
    return 0u;
  }

  if (prom_dom_set_u64(board,
                       PROM_DOM_SOURCE_MEMORY,
                       PROM_DOM_KEY_MEMORY_BUDGET,
                       0u,
                       (uint64_t)facts->memory_budget_slots_permille,
                       (int32_t)facts->memory_budget_slots_permille) == 0u) {
    return 0u;
  }

  if (prom_dom_set_i32(board,
                       PROM_DOM_SOURCE_MEMORY,
                       PROM_DOM_KEY_MEMORY_M35_FIXED_HEADROOM,
                       0u,
                       facts->fixed_double_headroom_slots_permille,
                       facts->fixed_double_headroom_slots_permille) == 0u) {
    return 0u;
  }
  if (prom_dom_set_i32(board,
                       PROM_DOM_SOURCE_MEMORY,
                       PROM_DOM_KEY_MEMORY_M35_PULL_LAG_HEADROOM,
                       0u,
                       facts->pull_lag_headroom_slots_permille,
                       facts->pull_lag_headroom_slots_permille) == 0u) {
    return 0u;
  }
  if (prom_dom_set_i32(board,
                       PROM_DOM_SOURCE_MEMORY,
                       PROM_DOM_KEY_MEMORY_M35_SERIAL_HEADROOM,
                       0u,
                       facts->serial_jit_headroom_slots_permille,
                       facts->serial_jit_headroom_slots_permille) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_MEMORY,
                       PROM_DOM_KEY_MEMORY_M35_REQUIRED_FIXED,
                       0u,
                       facts->required_fixed_slots_permille,
                       (int32_t)facts->required_fixed_slots_permille) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_MEMORY,
                       PROM_DOM_KEY_MEMORY_M35_REQUIRED_PULL_LAG,
                       0u,
                       facts->required_pull_lag_peak_slots_permille,
                       (int32_t)facts->required_pull_lag_peak_slots_permille) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_MEMORY,
                       PROM_DOM_KEY_MEMORY_M35_REQUIRED_SERIAL,
                       0u,
                       facts->required_serial_slots_permille,
                       (int32_t)facts->required_serial_slots_permille) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_TRANSFER_VARIANCE_CLASS,
                       0u,
                       (uint32_t)facts->transfer_variance_class,
                       (int32_t)facts->transfer_variance_class) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_COMPUTE_PREDICTABILITY_CLASS,
                       0u,
                       (uint32_t)facts->compute_predictability_class,
                       (int32_t)facts->compute_predictability_class) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_STARVATION_RISK_HIGH,
                       0u,
                       facts->starvation_risk_high,
                       (int32_t)facts->starvation_risk_high) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_PULL_LAG_WIP_WASTE_EXCEEDED,
                       0u,
                       facts->pull_lag_wip_waste_exceeded,
                       (int32_t)facts->pull_lag_wip_waste_exceeded) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_FALLBACK_AVAILABLE,
                       0u,
                       facts->fallback_available,
                       (int32_t)facts->fallback_available) == 0u) {
    return 0u;
  }
  return 1u;
}

uint32_t prom_dom_sgemm_stage_m35_decision(prom_dom_blackboard* board,
                                           const prom_buffering_selector_decision* decision,
                                           uint32_t no_feasible_mode_detail) {
  if (board == 0 || decision == 0) {
    return 0u;
  }

  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_SUCCESS,
                       0u,
                       decision->success,
                       (int32_t)decision->final_reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_BUFFERING_MODE,
                       0u,
                       (uint32_t)decision->selected_mode,
                       (int32_t)decision->final_reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_FIXED_FEASIBLE,
                       0u,
                       decision->fixed_feasible,
                       (int32_t)decision->fixed_double_rejection_reason) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_PULL_LAG_FEASIBLE,
                       0u,
                       decision->pull_lag_feasible,
                       (int32_t)decision->pull_lag_rejection_reason) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_SERIAL_FEASIBLE,
                       0u,
                       decision->serial_feasible,
                       (int32_t)decision->serial_jit_rejection_reason) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_FIXED_REJECTED,
                       0u,
                       decision->fixed_rejected,
                       (int32_t)decision->fixed_double_rejection_reason) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_PULL_LAG_REJECTED,
                       0u,
                       decision->pull_lag_rejected,
                       (int32_t)decision->pull_lag_rejection_reason) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_SERIAL_REJECTED,
                       0u,
                       decision->serial_rejected,
                       (int32_t)decision->serial_jit_rejection_reason) == 0u) {
    return 0u;
  }

  if (prom_dom_set_i32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_FIXED_SCORE,
                       0u,
                       (int32_t)decision->fixed_score,
                       (int32_t)decision->fixed_double_rejection_reason) == 0u) {
    return 0u;
  }
  if (prom_dom_set_i32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_PULL_LAG_SCORE,
                       0u,
                       (int32_t)decision->pull_lag_score,
                       (int32_t)decision->pull_lag_rejection_reason) == 0u) {
    return 0u;
  }
  if (prom_dom_set_i32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_SERIAL_SCORE,
                       0u,
                       (int32_t)decision->serial_score,
                       (int32_t)decision->serial_jit_rejection_reason) == 0u) {
    return 0u;
  }

  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_REASON_CODE,
                       0u,
                       (uint32_t)decision->reason_code,
                       (int32_t)decision->reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_FINAL_REASON_CODE,
                       0u,
                       (uint32_t)decision->final_reason_code,
                       (int32_t)decision->final_reason_code) == 0u) {
    return 0u;
  }

  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_FIXED_REJECTION_REASON,
                       0u,
                       (uint32_t)decision->fixed_double_rejection_reason,
                       (int32_t)decision->fixed_double_rejection_reason) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_PULL_LAG_REJECTION_REASON,
                       0u,
                       (uint32_t)decision->pull_lag_rejection_reason,
                       (int32_t)decision->pull_lag_rejection_reason) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_SERIAL_REJECTION_REASON,
                       0u,
                       (uint32_t)decision->serial_jit_rejection_reason,
                       (int32_t)decision->serial_jit_rejection_reason) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_M35_NO_FEASIBLE_DETAIL,
                       0u,
                       no_feasible_mode_detail,
                       (int32_t)decision->final_reason_code) == 0u) {
    return 0u;
  }

  return 1u;
}

uint32_t prom_dom_sgemm_stage_m35(prom_dom_blackboard* board,
                                  const prom_buffering_selector_facts* facts,
                                  const prom_buffering_selector_decision* decision) {
  if (prom_dom_sgemm_stage_m35_facts(board, facts) == 0u) {
    return 0u;
  }
  return prom_dom_sgemm_stage_m35_decision(board, decision, 0u);
}

void prom_dom_sgemm_commit(prom_dom_blackboard* board) {
  if (board == 0) {
    return;
  }
  prom_dom_commit(board);
}

uint32_t prom_dom_sgemm_read_visible_m35(const prom_dom_blackboard* board, prom_dom_sgemm_m35_snapshot* out_snapshot) {
  uint32_t u32_value;
  int32_t i32_value;
  uint64_t u64_value;

  if (board == 0 || out_snapshot == 0) {
    return 0u;
  }

  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_SUCCESS, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->success = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_BUFFERING_MODE, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->selected_mode = u32_value;

  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_FIXED_FEASIBLE, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->fixed_feasible = u32_value;

  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_PULL_LAG_FEASIBLE, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->pull_lag_feasible = u32_value;

  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_SERIAL_FEASIBLE, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->serial_feasible = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_FIXED_REJECTED, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->fixed_rejected = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_PULL_LAG_REJECTED, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->pull_lag_rejected = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_SERIAL_REJECTED, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->serial_rejected = u32_value;

  if (prom_dom_get_i32(board, PROM_DOM_KEY_SGEMM_M35_FIXED_SCORE, 0u, &i32_value) == 0u) {
    return 0u;
  }
  out_snapshot->fixed_score = i32_value;

  if (prom_dom_get_i32(board, PROM_DOM_KEY_SGEMM_M35_PULL_LAG_SCORE, 0u, &i32_value) == 0u) {
    return 0u;
  }
  out_snapshot->pull_lag_score = i32_value;

  if (prom_dom_get_i32(board, PROM_DOM_KEY_SGEMM_M35_SERIAL_SCORE, 0u, &i32_value) == 0u) {
    return 0u;
  }
  out_snapshot->serial_score = i32_value;

  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_REASON_CODE, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->reason_code = u32_value;

  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_FINAL_REASON_CODE, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->final_reason_code = u32_value;

  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_FIXED_REJECTION_REASON, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->fixed_double_rejection_reason = u32_value;

  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_PULL_LAG_REJECTION_REASON, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->pull_lag_rejection_reason = u32_value;

  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_SERIAL_REJECTION_REASON, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->serial_jit_rejection_reason = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_NO_FEASIBLE_DETAIL, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->no_feasible_mode_detail = u32_value;

  if (prom_dom_get_i32(board, PROM_DOM_KEY_MEMORY_M35_FIXED_HEADROOM, 0u, &i32_value) == 0u) {
    return 0u;
  }
  out_snapshot->fixed_double_headroom_slots_permille = i32_value;

  if (prom_dom_get_i32(board, PROM_DOM_KEY_MEMORY_M35_PULL_LAG_HEADROOM, 0u, &i32_value) == 0u) {
    return 0u;
  }
  out_snapshot->pull_lag_headroom_slots_permille = i32_value;

  if (prom_dom_get_i32(board, PROM_DOM_KEY_MEMORY_M35_SERIAL_HEADROOM, 0u, &i32_value) == 0u) {
    return 0u;
  }
  out_snapshot->serial_jit_headroom_slots_permille = i32_value;

  if (prom_dom_get_u64(board, PROM_DOM_KEY_MEMORY_BUDGET, 0u, &u64_value) == 0u) {
    return 0u;
  }
  out_snapshot->memory_budget_slots_permille = u64_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_MEMORY_M35_REQUIRED_FIXED, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->required_fixed_slots_permille = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_MEMORY_M35_REQUIRED_PULL_LAG, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->required_pull_lag_peak_slots_permille = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_MEMORY_M35_REQUIRED_SERIAL, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->required_serial_slots_permille = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_TRANSFER_VARIANCE_CLASS, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->transfer_variance_class = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_COMPUTE_PREDICTABILITY_CLASS, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->compute_predictability_class = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_STARVATION_RISK_HIGH, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->starvation_risk_high = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_PULL_LAG_WIP_WASTE_EXCEEDED, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->pull_lag_wip_waste_exceeded = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_FALLBACK_AVAILABLE, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->fallback_available = u32_value;

  return 1u;
}

uint32_t prom_dom_sgemm_build_buffering_selector_facts_from_visible(
    const prom_dom_blackboard* board,
    const prom_buffering_selector_facts* fallback_facts,
    prom_dom_sgemm_buffering_projection* out_projection) {
  prom_buffering_selector_facts facts;
  uint64_t budget_value;
  uint32_t u32_value;
  int32_t i32_value;

  if (board == 0 || fallback_facts == 0 || out_projection == 0) {
    return 0u;
  }

  facts = *fallback_facts;
  out_projection->visible_generation = board->visible_generation;
  out_projection->dependent_dirty_key_mask_last_commit = m35_dependency_mask_last_commit(board);
  out_projection->from_visible_snapshot = 0u;

  if (prom_dom_get_u64(board, PROM_DOM_KEY_MEMORY_BUDGET, 0u, &budget_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  if (prom_dom_get_i32(board, PROM_DOM_KEY_MEMORY_M35_FIXED_HEADROOM, 0u, &i32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.fixed_double_headroom_slots_permille = i32_value;
  if (prom_dom_get_i32(board, PROM_DOM_KEY_MEMORY_M35_PULL_LAG_HEADROOM, 0u, &i32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.pull_lag_headroom_slots_permille = i32_value;
  if (prom_dom_get_i32(board, PROM_DOM_KEY_MEMORY_M35_SERIAL_HEADROOM, 0u, &i32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  if (prom_dom_get_u32(board, PROM_DOM_KEY_MEMORY_M35_REQUIRED_FIXED, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.required_fixed_slots_permille = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_MEMORY_M35_REQUIRED_PULL_LAG, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.required_pull_lag_peak_slots_permille = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_MEMORY_M35_REQUIRED_SERIAL, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.required_serial_slots_permille = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_TRANSFER_VARIANCE_CLASS, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.transfer_variance_class = (prom_variance_class)u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_COMPUTE_PREDICTABILITY_CLASS, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.compute_predictability_class = (prom_predictability_class)u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_STARVATION_RISK_HIGH, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.starvation_risk_high = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_PULL_LAG_WIP_WASTE_EXCEEDED, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.pull_lag_wip_waste_exceeded = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_M35_FALLBACK_AVAILABLE, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.fallback_available = u32_value;
  facts.serial_jit_headroom_slots_permille = i32_value;
  facts.memory_budget_slots_permille = (uint32_t)budget_value;
  out_projection->from_visible_snapshot = 1u;
  out_projection->facts = facts;
  return 1u;
}

uint32_t prom_dom_sgemm_stage_transfer_queue_facts(prom_dom_blackboard* board, const prom_dom_transfer_queue_facts* facts) {
  if (board == 0 || facts == 0) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_QUEUE,
                       PROM_DOM_KEY_QUEUE_DEDICATED_AVAILABLE,
                       0u,
                       facts->dedicated_transfer_available,
                       (int32_t)facts->dedicated_transfer_available) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_QUEUE,
                       PROM_DOM_KEY_QUEUE_TRANSFER_FAMILY,
                       0u,
                       facts->transfer_queue_family_index,
                       (int32_t)facts->transfer_queue_family_index) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_QUEUE,
                       PROM_DOM_KEY_QUEUE_COMPUTE_FAMILY,
                       0u,
                       facts->compute_queue_family_index,
                       (int32_t)facts->compute_queue_family_index) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_QUEUE,
                       PROM_DOM_KEY_QUEUE_FAMILIES_DIFFER,
                       0u,
                       facts->queue_families_differ,
                       (int32_t)facts->queue_families_differ) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_QUEUE,
                       PROM_DOM_KEY_QUEUE_TRANSFER_SUPPORTED,
                       0u,
                       facts->transfer_queue_supported,
                       (int32_t)facts->transfer_queue_supported) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_POLICY,
                       PROM_DOM_KEY_QUEUE_TRANSFER_DISABLED_BY_CONFIG,
                       0u,
                       facts->transfer_queue_disabled_by_config,
                       (int32_t)facts->transfer_queue_disabled_by_config) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_POLICY,
                       PROM_DOM_KEY_QUEUE_TRANSFER_WORKLOAD_LARGE_ENOUGH,
                       0u,
                       facts->transfer_workload_large_enough,
                       (int32_t)facts->transfer_workload_large_enough) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_POLICY,
                       PROM_DOM_KEY_QUEUE_TRANSFER_SYNC_OWNERSHIP_SUPPORTED,
                       0u,
                       facts->transfer_sync_ownership_supported,
                       (int32_t)facts->transfer_sync_ownership_supported) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_POLICY,
                       PROM_DOM_KEY_QUEUE_TRANSFER_FALLBACK_AVAILABLE,
                       0u,
                       facts->transfer_fallback_available,
                       (int32_t)facts->transfer_fallback_available) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_POLICY,
                       PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_ONLY_ELIGIBLE,
                       0u,
                       facts->upload_only_policy_eligible,
                       (int32_t)facts->upload_only_policy_eligible) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_POLICY,
                       PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_READBACK_SUPPORTED,
                       0u,
                       facts->upload_readback_supported,
                       (int32_t)facts->upload_readback_supported) == 0u) {
    return 0u;
  }
  return 1u;
}

uint32_t prom_dom_sgemm_build_transfer_queue_facts_from_visible(const prom_dom_blackboard* board,
                                                                const prom_dom_transfer_queue_facts* fallback_facts,
                                                                prom_dom_transfer_queue_projection* out_projection) {
  prom_dom_transfer_queue_facts facts;
  uint32_t u32_value;
  if (board == 0 || fallback_facts == 0 || out_projection == 0) {
    return 0u;
  }
  facts = *fallback_facts;
  out_projection->visible_generation = board->visible_generation;
  out_projection->dependent_dirty_key_mask_last_commit = transfer_queue_dependency_mask_last_commit(board);
  out_projection->from_visible_snapshot = 0u;

  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_DEDICATED_AVAILABLE, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.dedicated_transfer_available = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_FAMILY, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.transfer_queue_family_index = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_COMPUTE_FAMILY, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.compute_queue_family_index = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_FAMILIES_DIFFER, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.queue_families_differ = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_SUPPORTED, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.transfer_queue_supported = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_DISABLED_BY_CONFIG, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.transfer_queue_disabled_by_config = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_WORKLOAD_LARGE_ENOUGH, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.transfer_workload_large_enough = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_SYNC_OWNERSHIP_SUPPORTED, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.transfer_sync_ownership_supported = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_FALLBACK_AVAILABLE, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.transfer_fallback_available = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_ONLY_ELIGIBLE, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.upload_only_policy_eligible = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_READBACK_SUPPORTED, 0u, &u32_value) == 0u) {
    out_projection->facts = facts;
    return 1u;
  }
  facts.upload_readback_supported = u32_value;
  out_projection->from_visible_snapshot = 1u;
  out_projection->facts = facts;
  return 1u;
}

uint32_t prom_dom_sgemm_stage_transfer_queue_decision(prom_dom_blackboard* board,
                                                      const prom_dom_transfer_queue_decision* decision) {
  if (board == 0 || decision == 0) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_QUEUE_TRANSFER_POLICY,
                       0u,
                       decision->selected_transfer_policy,
                       (int32_t)decision->transfer_fallback_reason) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_QUEUE_TRANSFER_POLICY_SELECTED,
                       0u,
                       decision->transfer_policy_selected,
                       (int32_t)decision->transfer_fallback_reason) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_QUEUE_TRANSFER_FALLBACK_REASON,
                       0u,
                       decision->transfer_fallback_reason,
                       (int32_t)decision->transfer_fallback_reason) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_QUEUE_TRANSFER_QUEUE_USED,
                       0u,
                       decision->transfer_queue_used,
                       (int32_t)decision->transfer_fallback_reason) == 0u) {
    return 0u;
  }
  return 1u;
}

uint32_t prom_dom_sgemm_read_visible_transfer_queue_diagnostics(const prom_dom_blackboard* board,
                                                                prom_dom_transfer_queue_snapshot* out_snapshot) {
  uint32_t u32_value;
  uint64_t u64_value;
  int32_t i32_value;
  if (board == 0 || out_snapshot == 0) {
    return 0u;
  }
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_POLICY_SELECTED, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->transfer_policy_selected = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_POLICY, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->selected_transfer_policy = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_QUEUE_USED, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->transfer_queue_used = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_FALLBACK_REASON, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->transfer_fallback_reason = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_DEDICATED_AVAILABLE, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->dedicated_transfer_available = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_FAMILY, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->transfer_queue_family_index = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_COMPUTE_FAMILY, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->compute_queue_family_index = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_FAMILIES_DIFFER, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->queue_families_differ = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_SUPPORTED, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->transfer_queue_supported = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_ONLY_ELIGIBLE, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->upload_only_policy_eligible = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_READBACK_SUPPORTED, 0u, &u32_value) == 0u) {
    return 0u;
  }
  out_snapshot->upload_readback_supported = u32_value;
  out_snapshot->queue_family_handoff_count = 0u;
  out_snapshot->transfer_compute_wait_count = 0u;
  out_snapshot->transfer_failure_slot_id = -1;
  out_snapshot->transfer_failure_reason = 0;
  out_snapshot->transfer_failure_count = 0u;
  out_snapshot->async_transfer_complete = 0u;
  out_snapshot->async_transfer_completion_generation = 0u;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_QUEUE_HANDOFF_COUNT, 0u, &u64_value) != 0u) {
    out_snapshot->queue_family_handoff_count = u64_value;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_QUEUE_TRANSFER_COMPUTE_WAIT_COUNT, 0u, &u64_value) != 0u) {
    out_snapshot->transfer_compute_wait_count = u64_value;
  }
  if (prom_dom_get_i32(board, PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_SLOT_ID, 0u, &i32_value) != 0u) {
    out_snapshot->transfer_failure_slot_id = i32_value;
  }
  if (prom_dom_get_i32(board, PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_REASON, 0u, &i32_value) != 0u) {
    out_snapshot->transfer_failure_reason = i32_value;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_COUNT, 0u, &u64_value) != 0u) {
    out_snapshot->transfer_failure_count = u64_value;
  }
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_ASYNC_TRANSFER_COMPLETE, 0u, &u32_value) != 0u) {
    out_snapshot->async_transfer_complete = u32_value;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_QUEUE_ASYNC_TRANSFER_COMPLETION_GENERATION, 0u, &u64_value) != 0u) {
    out_snapshot->async_transfer_completion_generation = u64_value;
  }
  return 1u;
}

uint32_t prom_dom_sgemm_stage_transfer_handoff(prom_dom_blackboard* board,
                                               uint64_t handoff_count,
                                               uint32_t slot_id,
                                               int32_t reason_code) {
  prom_dom_event event;
  if (board == 0) {
    return 0u;
  }
  if (prom_dom_set_u64(board, PROM_DOM_SOURCE_QUEUE, PROM_DOM_KEY_QUEUE_HANDOFF_COUNT, 0u, handoff_count, reason_code) == 0u) {
    return 0u;
  }
  event.generation = board->staged_generation + 1u;
  event.sequence = board->sequence_counter + 1u;
  event.kind = PROM_DOM_EVENT_QUEUE_HANDOFF;
  event.source = PROM_DOM_SOURCE_QUEUE;
  event.domain = PROM_DOM_DOMAIN_QUEUE;
  event.key = PROM_DOM_KEY_QUEUE_HANDOFF_COUNT;
  event.slot_id = slot_id;
  event.reason_code = reason_code;
  return prom_dom_stage_event(board, &event);
}

uint32_t prom_dom_sgemm_stage_transfer_wait(prom_dom_blackboard* board,
                                            uint64_t wait_count,
                                            uint32_t slot_id,
                                            int32_t reason_code) {
  prom_dom_event event;
  if (board == 0) {
    return 0u;
  }
  if (prom_dom_set_u64(board,
                       PROM_DOM_SOURCE_QUEUE,
                       PROM_DOM_KEY_QUEUE_TRANSFER_COMPUTE_WAIT_COUNT,
                       0u,
                       wait_count,
                       reason_code) == 0u) {
    return 0u;
  }
  event.generation = board->staged_generation + 1u;
  event.sequence = board->sequence_counter + 1u;
  event.kind = PROM_DOM_EVENT_TRANSFER_WAIT;
  event.source = PROM_DOM_SOURCE_QUEUE;
  event.domain = PROM_DOM_DOMAIN_QUEUE;
  event.key = PROM_DOM_KEY_QUEUE_TRANSFER_COMPUTE_WAIT_COUNT;
  event.slot_id = slot_id;
  event.reason_code = reason_code;
  return prom_dom_stage_event(board, &event);
}

uint32_t prom_dom_sgemm_stage_transfer_failure(prom_dom_blackboard* board,
                                               int32_t slot_id,
                                               int32_t reason_code,
                                               uint64_t failure_count) {
  prom_dom_event event;
  if (board == 0) {
    return 0u;
  }
  if (prom_dom_set_i32(board, PROM_DOM_SOURCE_QUEUE, PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_SLOT_ID, 0u, slot_id, reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_i32(board, PROM_DOM_SOURCE_QUEUE, PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_REASON, 0u, reason_code, reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u64(board, PROM_DOM_SOURCE_QUEUE, PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_COUNT, 0u, failure_count, reason_code) == 0u) {
    return 0u;
  }
  event.generation = board->staged_generation + 1u;
  event.sequence = board->sequence_counter + 1u;
  event.kind = PROM_DOM_EVENT_TRANSFER_FAILED;
  event.source = PROM_DOM_SOURCE_QUEUE;
  event.domain = PROM_DOM_DOMAIN_QUEUE;
  event.key = PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_REASON;
  event.slot_id = slot_id < 0 ? 0u : (uint32_t)slot_id;
  event.reason_code = reason_code;
  return prom_dom_stage_event(board, &event);
}

uint32_t prom_dom_sgemm_stage_transfer_complete(prom_dom_blackboard* board,
                                                uint32_t async_transfer_complete,
                                                uint64_t completion_generation,
                                                uint32_t slot_id,
                                                int32_t reason_code) {
  prom_dom_event event;
  if (board == 0) {
    return 0u;
  }
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_QUEUE,
                       PROM_DOM_KEY_QUEUE_ASYNC_TRANSFER_COMPLETE,
                       0u,
                       async_transfer_complete,
                       reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u64(board,
                       PROM_DOM_SOURCE_QUEUE,
                       PROM_DOM_KEY_QUEUE_ASYNC_TRANSFER_COMPLETION_GENERATION,
                       0u,
                       completion_generation,
                       reason_code) == 0u) {
    return 0u;
  }
  event.generation = board->staged_generation + 1u;
  event.sequence = board->sequence_counter + 1u;
  event.kind = PROM_DOM_EVENT_TRANSFER_COMPLETE;
  event.source = PROM_DOM_SOURCE_QUEUE;
  event.domain = PROM_DOM_DOMAIN_QUEUE;
  event.key = PROM_DOM_KEY_QUEUE_ASYNC_TRANSFER_COMPLETE;
  event.slot_id = slot_id;
  event.reason_code = reason_code;
  return prom_dom_stage_event(board, &event);
}

uint32_t prom_dom_sgemm_read_visible_transfer_runtime_telemetry(const prom_dom_blackboard* board,
                                                                prom_dom_transfer_runtime_telemetry* out_snapshot) {
  uint64_t u64_value;
  uint32_t u32_value;
  int32_t i32_value;
  if (board == 0 || out_snapshot == 0) {
    return 0u;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_QUEUE_HANDOFF_COUNT, 0u, &u64_value) == 0u) {
    return 0u;
  }
  out_snapshot->queue_family_handoff_count = u64_value;
  out_snapshot->transfer_compute_wait_count = 0u;
  out_snapshot->transfer_failure_slot_id = -1;
  out_snapshot->transfer_failure_reason = 0;
  out_snapshot->transfer_failure_count = 0u;
  out_snapshot->async_transfer_complete = 0u;
  out_snapshot->async_transfer_completion_generation = 0u;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_QUEUE_TRANSFER_COMPUTE_WAIT_COUNT, 0u, &u64_value) != 0u) {
    out_snapshot->transfer_compute_wait_count = u64_value;
  }
  if (prom_dom_get_i32(board, PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_SLOT_ID, 0u, &i32_value) != 0u) {
    out_snapshot->transfer_failure_slot_id = i32_value;
  }
  if (prom_dom_get_i32(board, PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_REASON, 0u, &i32_value) != 0u) {
    out_snapshot->transfer_failure_reason = i32_value;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_COUNT, 0u, &u64_value) != 0u) {
    out_snapshot->transfer_failure_count = u64_value;
  }
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_ASYNC_TRANSFER_COMPLETE, 0u, &u32_value) != 0u) {
    out_snapshot->async_transfer_complete = u32_value;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_QUEUE_ASYNC_TRANSFER_COMPLETION_GENERATION, 0u, &u64_value) != 0u) {
    out_snapshot->async_transfer_completion_generation = u64_value;
  }
  return 1u;
}

uint32_t prom_dom_sgemm_stage_layout_precision_facts(prom_dom_blackboard* board,
                                                     const prom_dom_sgemm_layout_precision_facts* facts) {
  if (board == 0 || facts == 0) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_PACKED4_AVAILABLE, 0u, facts->packed4_available, (int32_t)facts->packed4_available) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_PACKED4_SMALL_SHAPE, 0u, facts->packed4_small_shape, (int32_t)facts->packed4_small_shape) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_PACKED4_PADDING_WASTE_PERMILLE, 0u, facts->packed4_padding_waste_permille, (int32_t)facts->packed4_padding_waste_permille) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_PACKED4_MODE_BUDGET_PERMILLE, 0u, facts->packed4_mode_budget_permille, (int32_t)facts->packed4_mode_budget_permille) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_PACKED4_ROW_MAJOR_VALID, 0u, facts->packed4_row_major_valid, (int32_t)facts->packed4_row_major_valid) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_PACKED4_TAIL_VALID, 0u, facts->packed4_tail_valid, (int32_t)facts->packed4_tail_valid) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FP16_STRICT_FP32, 0u, facts->strict_fp32, (int32_t)facts->strict_fp32) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FP16_TOLERANCE_KNOWN, 0u, facts->tolerance_known, (int32_t)facts->tolerance_known) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FP16_TOLERANCE_PASS, 0u, facts->tolerance_pass, (int32_t)facts->tolerance_pass) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FP16_HAS_SPECIAL_VALUES, 0u, facts->has_special_values, (int32_t)facts->has_special_values) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FP16_CAPABILITY_STORAGE, 0u, facts->capability_fp16_storage, (int32_t)facts->capability_fp16_storage) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FP16_FALLBACK_AVAILABLE, 0u, facts->fallback_available, (int32_t)facts->fallback_available) == 0u) return 0u;
  if (prom_dom_set_i32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FP16_UTILITY_SCORE, 0u, facts->fp16_utility_score, facts->fp16_utility_score) == 0u) return 0u;
  return 1u;
}

uint32_t prom_dom_sgemm_build_layout_precision_facts_from_visible(
    const prom_dom_blackboard* board,
    const prom_dom_sgemm_layout_precision_facts* fallback_facts,
    prom_dom_sgemm_layout_precision_projection* out_projection) {
  prom_dom_sgemm_layout_precision_facts facts;
  uint32_t u32_value;
  int32_t i32_value;
  if (board == 0 || fallback_facts == 0 || out_projection == 0) return 0u;
  facts = *fallback_facts;
  out_projection->visible_generation = board->visible_generation;
  out_projection->dependent_dirty_key_mask_last_commit = layout_precision_dependency_mask_last_commit(board);
  out_projection->from_visible_snapshot = 0u;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_AVAILABLE, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.packed4_available = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_SMALL_SHAPE, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.packed4_small_shape = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_PADDING_WASTE_PERMILLE, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.packed4_padding_waste_permille = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_MODE_BUDGET_PERMILLE, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.packed4_mode_budget_permille = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_ROW_MAJOR_VALID, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.packed4_row_major_valid = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_TAIL_VALID, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.packed4_tail_valid = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_STRICT_FP32, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.strict_fp32 = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_TOLERANCE_KNOWN, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.tolerance_known = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_TOLERANCE_PASS, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.tolerance_pass = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_HAS_SPECIAL_VALUES, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.has_special_values = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_CAPABILITY_STORAGE, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.capability_fp16_storage = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_FALLBACK_AVAILABLE, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.fallback_available = u32_value;
  if (prom_dom_get_i32(board, PROM_DOM_KEY_SGEMM_FP16_UTILITY_SCORE, 0u, &i32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.fp16_utility_score = i32_value;
  out_projection->from_visible_snapshot = 1u;
  out_projection->facts = facts;
  return 1u;
}

uint32_t prom_dom_sgemm_stage_layout_precision_decision(prom_dom_blackboard* board,
                                                        const prom_dom_sgemm_layout_precision_decision* decision) {
  if (board == 0 || decision == 0) return 0u;
#define SETU32(src,key,val,reason) if (prom_dom_set_u32(board, src, key, 0u, val, reason) == 0u) return 0u
#define SETU64(src,key,val,reason) if (prom_dom_set_u64(board, src, key, 0u, val, reason) == 0u) return 0u
  SETU32(PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_PACKED4_SELECTED, decision->packed4_selected, (int32_t)decision->packed4_reject_reason);
  SETU32(PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_PACKED4_REJECT_REASON, decision->packed4_reject_reason, (int32_t)decision->packed4_reject_reason);
  SETU32(PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FP16_SELECTED, decision->fp16_selected, (int32_t)decision->fp16_reject_reason);
  SETU32(PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FP16_REJECT_REASON, decision->fp16_reject_reason, (int32_t)decision->fp16_reject_reason);
  SETU32(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_SELECTED_LAYOUT_FORMAT, decision->packed4_selected_layout_format, (int32_t)decision->packed4_reject_reason);
  SETU32(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_TAIL_COUNT_LAST, decision->packed4_tail_count_last, 0);
  SETU64(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_TAIL_COUNT_TOTAL, decision->packed4_tail_count_total, 0);
  SETU32(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_PADDED_LANE_COUNT_LAST, decision->packed4_padded_lane_count_last, 0);
  SETU64(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_PADDED_LANE_COUNT_TOTAL, decision->packed4_padded_lane_count_total, 0);
  SETU32(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_PADDING_WASTE_PERMILLE_LAST, decision->packed4_padding_waste_permille_last, 0);
  SETU64(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_MODE_BUDGET_DENIALS, decision->packed4_mode_budget_denials, 0);
  SETU64(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_ROW_MAJOR_CHECK_FAILURES, decision->packed4_row_major_check_failures, 0);
  SETU64(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_SELECTION_COUNT, decision->packed4_selection_count, 0);
  SETU64(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_PADDING_WASTE, decision->packed4_fallback_reason_padding_waste, 0);
  SETU64(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_SMALL_SHAPE, decision->packed4_fallback_reason_small_shape, 0);
  SETU64(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_CAPABILITY_MISSING, decision->packed4_fallback_reason_capability_missing, 0);
  SETU64(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_FALLBACK_REQUIRED, decision->packed4_fallback_reason_fallback_required, 0);
  SETU64(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_MODE_BUDGET_DENIED, decision->packed4_fallback_reason_mode_budget_denied, 0);
  SETU32(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_FP16_MAX_ABSOLUTE_ERROR_BITS, float_to_bits(decision->fp16_max_absolute_error), 0);
  SETU32(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_FP16_MAX_RELATIVE_ERROR_BITS, float_to_bits(decision->fp16_max_relative_error), 0);
  SETU32(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_FP16_AGGREGATE_ERROR_BITS, float_to_bits(decision->fp16_aggregate_error), 0);
  SETU32(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_FP16_WORST_CASE_ELEMENT_INDEX, decision->fp16_worst_case_element_index, 0);
  SETU32(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_FP16_K_ERROR_GROWTH_BITS, float_to_bits(decision->fp16_k_error_growth), 0);
  SETU32(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_FP16_CANCELLATION_RISK_BITS, float_to_bits(decision->fp16_cancellation_risk), 0);
  SETU32(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_FP16_TOLERANCE_KNOWN, decision->fp16_tolerance_known, 0);
  SETU32(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_FP16_TOLERANCE_PASS, decision->fp16_tolerance_pass, 0);
  if (prom_dom_set_i32(board, PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_FP16_FALLBACK_REASON_DETAIL, 0u, decision->fp16_fallback_reason_detail, decision->fp16_fallback_reason_detail) == 0u) return 0u;
  SETU32(PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_DIAGNOSTICS_FP16_SELECTED_CANDIDATE, decision->fp16_selected_candidate, decision->fp16_fallback_reason_detail);
#undef SETU32
#undef SETU64
  return 1u;
}

uint32_t prom_dom_sgemm_read_visible_layout_precision_diagnostics(const prom_dom_blackboard* board,
                                                                  prom_dom_sgemm_layout_precision_snapshot* out_snapshot) {
  uint32_t u32_value;
  uint64_t u64_value;
  int32_t i32_value;
  if (board == 0 || out_snapshot == 0) return 0u;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_AVAILABLE, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.packed4_available = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_SMALL_SHAPE, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.packed4_small_shape = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_PADDING_WASTE_PERMILLE, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.packed4_padding_waste_permille = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_MODE_BUDGET_PERMILLE, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.packed4_mode_budget_permille = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_ROW_MAJOR_VALID, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.packed4_row_major_valid = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_TAIL_VALID, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.packed4_tail_valid = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_STRICT_FP32, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.strict_fp32 = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_TOLERANCE_KNOWN, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.tolerance_known = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_TOLERANCE_PASS, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.tolerance_pass = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_HAS_SPECIAL_VALUES, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.has_special_values = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_CAPABILITY_STORAGE, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.capability_fp16_storage = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_FALLBACK_AVAILABLE, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.fallback_available = u32_value;
  if (prom_dom_get_i32(board, PROM_DOM_KEY_SGEMM_FP16_UTILITY_SCORE, 0u, &i32_value) == 0u) return 0u;
  out_snapshot->facts.fp16_utility_score = i32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_SELECTED, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.packed4_selected = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_REJECT_REASON, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.packed4_reject_reason = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_SELECTED, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.fp16_selected = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_REJECT_REASON, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.fp16_reject_reason = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_SELECTED_LAYOUT_FORMAT, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.packed4_selected_layout_format = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_TAIL_COUNT_LAST, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.packed4_tail_count_last = u32_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_TAIL_COUNT_TOTAL, 0u, &u64_value) == 0u) return 0u;
  out_snapshot->decision.packed4_tail_count_total = u64_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_PADDED_LANE_COUNT_LAST, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.packed4_padded_lane_count_last = u32_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_PADDED_LANE_COUNT_TOTAL, 0u, &u64_value) == 0u) return 0u;
  out_snapshot->decision.packed4_padded_lane_count_total = u64_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_PADDING_WASTE_PERMILLE_LAST, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.packed4_padding_waste_permille_last = u32_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_MODE_BUDGET_DENIALS, 0u, &u64_value) == 0u) return 0u;
  out_snapshot->decision.packed4_mode_budget_denials = u64_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_ROW_MAJOR_CHECK_FAILURES, 0u, &u64_value) == 0u) return 0u;
  out_snapshot->decision.packed4_row_major_check_failures = u64_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_SELECTION_COUNT, 0u, &u64_value) == 0u) return 0u;
  out_snapshot->decision.packed4_selection_count = u64_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_PADDING_WASTE, 0u, &u64_value) == 0u) return 0u;
  out_snapshot->decision.packed4_fallback_reason_padding_waste = u64_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_SMALL_SHAPE, 0u, &u64_value) == 0u) return 0u;
  out_snapshot->decision.packed4_fallback_reason_small_shape = u64_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_CAPABILITY_MISSING, 0u, &u64_value) == 0u) return 0u;
  out_snapshot->decision.packed4_fallback_reason_capability_missing = u64_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_FALLBACK_REQUIRED, 0u, &u64_value) == 0u) return 0u;
  out_snapshot->decision.packed4_fallback_reason_fallback_required = u64_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_MODE_BUDGET_DENIED, 0u, &u64_value) == 0u) return 0u;
  out_snapshot->decision.packed4_fallback_reason_mode_budget_denied = u64_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_DIAGNOSTICS_FP16_MAX_ABSOLUTE_ERROR_BITS, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.fp16_max_absolute_error = bits_to_float(u32_value);
  if (prom_dom_get_u32(board, PROM_DOM_KEY_DIAGNOSTICS_FP16_MAX_RELATIVE_ERROR_BITS, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.fp16_max_relative_error = bits_to_float(u32_value);
  if (prom_dom_get_u32(board, PROM_DOM_KEY_DIAGNOSTICS_FP16_AGGREGATE_ERROR_BITS, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.fp16_aggregate_error = bits_to_float(u32_value);
  if (prom_dom_get_u32(board, PROM_DOM_KEY_DIAGNOSTICS_FP16_WORST_CASE_ELEMENT_INDEX, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.fp16_worst_case_element_index = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_DIAGNOSTICS_FP16_K_ERROR_GROWTH_BITS, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.fp16_k_error_growth = bits_to_float(u32_value);
  if (prom_dom_get_u32(board, PROM_DOM_KEY_DIAGNOSTICS_FP16_CANCELLATION_RISK_BITS, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.fp16_cancellation_risk = bits_to_float(u32_value);
  if (prom_dom_get_u32(board, PROM_DOM_KEY_DIAGNOSTICS_FP16_TOLERANCE_KNOWN, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.fp16_tolerance_known = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_DIAGNOSTICS_FP16_TOLERANCE_PASS, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.fp16_tolerance_pass = u32_value;
  if (prom_dom_get_i32(board, PROM_DOM_KEY_DIAGNOSTICS_FP16_FALLBACK_REASON_DETAIL, 0u, &i32_value) == 0u) return 0u;
  out_snapshot->decision.fp16_fallback_reason_detail = i32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_DIAGNOSTICS_FP16_SELECTED_CANDIDATE, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.fp16_selected_candidate = u32_value;
  return 1u;
}
