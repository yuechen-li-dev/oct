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
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_SHAPE_M) != 0u) mask |= 1ull << 13u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_SHAPE_N) != 0u) mask |= 1ull << 14u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_SHAPE_K) != 0u) mask |= 1ull << 15u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_WORK_UNITS) != 0u) mask |= 1ull << 16u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_TILED_SHAPE) != 0u) mask |= 1ull << 17u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_POLICY_MODE) != 0u) mask |= 1ull << 18u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_CAN_DIRECT) != 0u) mask |= 1ull << 19u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_ALLOW_FALLBACK) != 0u) mask |= 1ull << 20u;
  return mask;
}

static uint64_t path_compute_dependency_mask_last_commit(const prom_dom_blackboard* board) {
  uint64_t mask = 0u;
  if (board == 0) return 0u;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_SHAPE_M) != 0u) mask |= 1ull << PROM_DOM_PATH_COMPUTE_DEP_SHAPE_M;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_SHAPE_N) != 0u) mask |= 1ull << PROM_DOM_PATH_COMPUTE_DEP_SHAPE_N;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_SHAPE_K) != 0u) mask |= 1ull << PROM_DOM_PATH_COMPUTE_DEP_SHAPE_K;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_WORK_UNITS) != 0u) mask |= 1ull << PROM_DOM_PATH_COMPUTE_DEP_WORK_UNITS;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_CAN_STAGE) != 0u) mask |= 1ull << PROM_DOM_PATH_COMPUTE_DEP_CAN_STAGE;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_CAN_DIRECT) != 0u) mask |= 1ull << PROM_DOM_PATH_COMPUTE_DEP_CAN_DIRECT;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_ALLOW_FALLBACK) != 0u) mask |= 1ull << PROM_DOM_PATH_COMPUTE_DEP_ALLOW_FALLBACK;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_READBACK_REQUIRED) != 0u) mask |= 1ull << PROM_DOM_PATH_COMPUTE_DEP_READBACK_REQUIRED;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_FORCE_DIRECT) != 0u) mask |= 1ull << PROM_DOM_PATH_COMPUTE_DEP_FORCE_DIRECT;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_FORCE_STAGED) != 0u) mask |= 1ull << PROM_DOM_PATH_COMPUTE_DEP_FORCE_STAGED;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_FORCE_TILED) != 0u) mask |= 1ull << PROM_DOM_PATH_COMPUTE_DEP_FORCE_TILED;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_TILED_SHAPE) != 0u) mask |= 1ull << PROM_DOM_PATH_COMPUTE_DEP_TILED_SHAPE;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_SOFTWARE_VULKAN) != 0u) mask |= 1ull << PROM_DOM_PATH_COMPUTE_DEP_SOFTWARE_VULKAN;
  if (prom_dom_dirty_key_last_commit(board, PROM_DOM_KEY_SGEMM_FACT_POLICY_MODE) != 0u) mask |= 1ull << PROM_DOM_PATH_COMPUTE_DEP_POLICY_MODE;
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

uint32_t prom_dom_sgemm_stage_async_snapshot(prom_dom_blackboard* board,
                                             const prom_dom_async_snapshot* snapshot,
                                             prom_dom_event_kind event_kind,
                                             int32_t reason_code) {
  prom_dom_event event;
  if (board == 0 || snapshot == 0) {
    return 0u;
  }
  if (prom_dom_set_i32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_TASK_ID, 0u, snapshot->task_id, reason_code) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_LIFECYCLE_STATE, 0u, snapshot->lifecycle_state, reason_code) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_STAGE, 0u, snapshot->stage, reason_code) == 0u) return 0u;
  if (prom_dom_set_i32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_DETAIL, 0u, snapshot->detail_code, reason_code) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_READY, 0u, snapshot->ready, reason_code) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_FAILED, 0u, snapshot->failed, reason_code) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_CONSUMED, 0u, snapshot->consumed, reason_code) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_OUTSTANDING_TASKS, 0u, snapshot->outstanding_tasks, reason_code) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_FAILURE_STAGE, 0u, snapshot->failure_stage, reason_code) == 0u) return 0u;
  if (prom_dom_set_i32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_FAILURE_DETAIL, 0u, snapshot->failure_detail, reason_code) == 0u) return 0u;
  if (prom_dom_set_i32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_SUBMIT_DETAIL, 0u, snapshot->submit_detail, reason_code) == 0u) return 0u;
  if (prom_dom_set_i32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_QUERY_DETAIL, 0u, snapshot->query_detail, reason_code) == 0u) return 0u;
  if (prom_dom_set_i32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_SLOT_ID, 0u, snapshot->slot_id, reason_code) == 0u) return 0u;
  if (prom_dom_set_u64(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_SLOT_GENERATION, 0u, snapshot->slot_generation, reason_code) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_OWNS_SLOT, 0u, snapshot->owns_slot, reason_code) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_TRANSFER_COMPLETE, 0u, snapshot->transfer_complete, reason_code) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_COMPUTE_COMPLETE, 0u, snapshot->compute_complete, reason_code) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_ASYNC_READBACK_COMPLETE, 0u, snapshot->readback_complete, reason_code) == 0u) return 0u;
  event.generation = board->staged_generation + 1u;
  event.sequence = board->sequence_counter + 1u;
  event.kind = event_kind;
  event.source = PROM_DOM_SOURCE_REACTOR;
  event.domain = PROM_DOM_DOMAIN_ASYNC;
  event.key = PROM_DOM_KEY_ASYNC_LIFECYCLE_STATE;
  event.slot_id = snapshot->slot_id < 0 ? 0u : (uint32_t)snapshot->slot_id;
  event.reason_code = reason_code;
  return prom_dom_stage_event(board, &event);
}

uint32_t prom_dom_sgemm_read_visible_async_snapshot(const prom_dom_blackboard* board, prom_dom_async_snapshot* out_snapshot) {
  int32_t i32_value;
  uint32_t u32_value;
  uint64_t u64_value;
  if (board == 0 || out_snapshot == 0) {
    return 0u;
  }
  memset(out_snapshot, 0, sizeof(*out_snapshot));
  out_snapshot->task_id = -1;
  out_snapshot->slot_id = -1;
  if (prom_dom_get_i32(board, PROM_DOM_KEY_ASYNC_TASK_ID, 0u, &i32_value) == 0u) return 0u;
  out_snapshot->task_id = i32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_ASYNC_LIFECYCLE_STATE, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->lifecycle_state = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_ASYNC_STAGE, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->stage = u32_value;
  if (prom_dom_get_i32(board, PROM_DOM_KEY_ASYNC_DETAIL, 0u, &i32_value) == 0u) return 0u;
  out_snapshot->detail_code = i32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_ASYNC_READY, 0u, &u32_value) != 0u) out_snapshot->ready = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_ASYNC_FAILED, 0u, &u32_value) != 0u) out_snapshot->failed = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_ASYNC_CONSUMED, 0u, &u32_value) != 0u) out_snapshot->consumed = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_ASYNC_OUTSTANDING_TASKS, 0u, &u32_value) != 0u) out_snapshot->outstanding_tasks = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_ASYNC_FAILURE_STAGE, 0u, &u32_value) != 0u) out_snapshot->failure_stage = u32_value;
  if (prom_dom_get_i32(board, PROM_DOM_KEY_ASYNC_FAILURE_DETAIL, 0u, &i32_value) != 0u) out_snapshot->failure_detail = i32_value;
  if (prom_dom_get_i32(board, PROM_DOM_KEY_ASYNC_SUBMIT_DETAIL, 0u, &i32_value) != 0u) out_snapshot->submit_detail = i32_value;
  if (prom_dom_get_i32(board, PROM_DOM_KEY_ASYNC_QUERY_DETAIL, 0u, &i32_value) != 0u) out_snapshot->query_detail = i32_value;
  if (prom_dom_get_i32(board, PROM_DOM_KEY_ASYNC_SLOT_ID, 0u, &i32_value) != 0u) out_snapshot->slot_id = i32_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_ASYNC_SLOT_GENERATION, 0u, &u64_value) != 0u) out_snapshot->slot_generation = u64_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_ASYNC_OWNS_SLOT, 0u, &u32_value) != 0u) out_snapshot->owns_slot = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_ASYNC_TRANSFER_COMPLETE, 0u, &u32_value) != 0u) out_snapshot->transfer_complete = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_ASYNC_COMPUTE_COMPLETE, 0u, &u32_value) != 0u) out_snapshot->compute_complete = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_ASYNC_READBACK_COMPLETE, 0u, &u32_value) != 0u) out_snapshot->readback_complete = u32_value;
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

uint32_t prom_dom_sgemm_stage_path_compute_facts(prom_dom_blackboard* board,
                                                 const prom_dom_sgemm_path_compute_facts* facts) {
  if (board == 0 || facts == 0) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FACT_SHAPE_M, 0u, facts->m, (int32_t)facts->m) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FACT_SHAPE_N, 0u, facts->n, (int32_t)facts->n) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FACT_SHAPE_K, 0u, facts->k, (int32_t)facts->k) == 0u) return 0u;
  if (prom_dom_set_u64(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FACT_WORK_UNITS, 0u, facts->work_units, (int32_t)facts->k) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FACT_CAN_STAGE, 0u, facts->can_stage, (int32_t)facts->can_stage) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FACT_CAN_DIRECT, 0u, facts->can_direct, (int32_t)facts->can_direct) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FACT_ALLOW_FALLBACK, 0u, facts->allow_fallback, (int32_t)facts->allow_fallback) == 0u) return 0u;
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_FACT_READBACK_REQUIRED,
                       0u,
                       facts->readback_required,
                       (int32_t)facts->readback_required) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FACT_FORCE_DIRECT, 0u, facts->force_direct, (int32_t)facts->force_direct) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FACT_FORCE_STAGED, 0u, facts->force_staged, (int32_t)facts->force_staged) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FACT_FORCE_TILED, 0u, facts->force_tiled, (int32_t)facts->force_tiled) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_FACT_TILED_SHAPE, 0u, facts->tiled_shape, (int32_t)facts->tiled_shape) == 0u) return 0u;
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_FACT_SOFTWARE_VULKAN,
                       0u,
                       facts->software_vulkan,
                       (int32_t)facts->software_vulkan) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_POLICY, PROM_DOM_KEY_SGEMM_FACT_POLICY_MODE, 0u, facts->policy_mode, (int32_t)facts->policy_mode) == 0u) return 0u;
  return 1u;
}

uint32_t prom_dom_sgemm_build_path_compute_facts_from_visible(
    const prom_dom_blackboard* board,
    const prom_dom_sgemm_path_compute_facts* fallback_facts,
    prom_dom_sgemm_path_compute_projection* out_projection) {
  prom_dom_sgemm_path_compute_facts facts;
  uint32_t u32_value;
  uint64_t u64_value;
  if (board == 0 || fallback_facts == 0 || out_projection == 0) return 0u;
  facts = *fallback_facts;
  out_projection->visible_generation = board->visible_generation;
  out_projection->dependent_dirty_key_mask_last_commit = path_compute_dependency_mask_last_commit(board);
  out_projection->from_visible_snapshot = 0u;

  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_SHAPE_M, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.m = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_SHAPE_N, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.n = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_SHAPE_K, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.k = u32_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SGEMM_FACT_WORK_UNITS, 0u, &u64_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.work_units = u64_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_CAN_STAGE, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.can_stage = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_CAN_DIRECT, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.can_direct = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_ALLOW_FALLBACK, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.allow_fallback = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_READBACK_REQUIRED, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.readback_required = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_FORCE_DIRECT, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.force_direct = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_FORCE_STAGED, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.force_staged = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_FORCE_TILED, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.force_tiled = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_TILED_SHAPE, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.tiled_shape = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_SOFTWARE_VULKAN, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.software_vulkan = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_POLICY_MODE, 0u, &u32_value) == 0u) { out_projection->facts = facts; return 1u; }
  facts.policy_mode = u32_value;
  out_projection->from_visible_snapshot = 1u;
  out_projection->facts = facts;
  return 1u;
}

uint32_t prom_dom_sgemm_stage_path_compute_decision(prom_dom_blackboard* board,
                                                    const prom_dom_sgemm_path_compute_decision* decision) {
  if (board == 0 || decision == 0) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_JUDGMENT_SUCCESS, 0u, decision->success, decision->final_detail) == 0u) return 0u;
  if (prom_dom_set_i32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_JUDGMENT_ERROR_DETAIL, 0u, decision->error_detail, decision->error_detail) == 0u) return 0u;
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_JUDGMENT_REQUESTED_PATH,
                       0u,
                       decision->requested_path,
                       decision->final_detail) == 0u) return 0u;
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_JUDGMENT_SELECTED_PATH,
                       0u,
                       decision->selected_path,
                       decision->final_detail) == 0u) return 0u;
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_JUDGMENT_COMPUTE_MODE,
                       0u,
                       decision->compute_mode,
                       decision->final_detail) == 0u) return 0u;
  if (prom_dom_set_i32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_JUDGMENT_FINAL_DETAIL, 0u, decision->final_detail, decision->final_detail) == 0u) return 0u;
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_JUDGMENT_USED_FALLBACK_TO_DIRECT,
                       0u,
                       decision->used_fallback_to_direct,
                       decision->final_detail) == 0u) return 0u;
  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_JUDGMENT_WINNING_CANDIDATE_INDEX,
                       0u,
                       decision->winning_candidate_index,
                       decision->winning_score) == 0u) return 0u;
  if (prom_dom_set_i32(board,
                       PROM_DOM_SOURCE_JUDGMENT,
                       PROM_DOM_KEY_SGEMM_JUDGMENT_WINNING_SCORE,
                       0u,
                       decision->winning_score,
                       decision->winning_score) == 0u) return 0u;
  return 1u;
}

uint32_t prom_dom_sgemm_read_visible_path_compute_diagnostics(const prom_dom_blackboard* board,
                                                              prom_dom_sgemm_path_compute_snapshot* out_snapshot) {
  uint32_t u32_value;
  uint64_t u64_value;
  int32_t i32_value;
  if (board == 0 || out_snapshot == 0) return 0u;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_SHAPE_M, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.m = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_SHAPE_N, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.n = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_SHAPE_K, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.k = u32_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SGEMM_FACT_WORK_UNITS, 0u, &u64_value) == 0u) return 0u;
  out_snapshot->facts.work_units = u64_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_CAN_STAGE, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.can_stage = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_CAN_DIRECT, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.can_direct = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_ALLOW_FALLBACK, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.allow_fallback = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_READBACK_REQUIRED, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.readback_required = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_FORCE_DIRECT, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.force_direct = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_FORCE_STAGED, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.force_staged = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_FORCE_TILED, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.force_tiled = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_TILED_SHAPE, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.tiled_shape = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_SOFTWARE_VULKAN, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.software_vulkan = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FACT_POLICY_MODE, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.policy_mode = u32_value;

  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_JUDGMENT_SUCCESS, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.success = u32_value;
  if (prom_dom_get_i32(board, PROM_DOM_KEY_SGEMM_JUDGMENT_ERROR_DETAIL, 0u, &i32_value) == 0u) return 0u;
  out_snapshot->decision.error_detail = i32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_JUDGMENT_REQUESTED_PATH, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.requested_path = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_JUDGMENT_SELECTED_PATH, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.selected_path = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_JUDGMENT_COMPUTE_MODE, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.compute_mode = u32_value;
  if (prom_dom_get_i32(board, PROM_DOM_KEY_SGEMM_JUDGMENT_FINAL_DETAIL, 0u, &i32_value) == 0u) return 0u;
  out_snapshot->decision.final_detail = i32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_JUDGMENT_USED_FALLBACK_TO_DIRECT, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.used_fallback_to_direct = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_JUDGMENT_WINNING_CANDIDATE_INDEX, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.winning_candidate_index = u32_value;
  if (prom_dom_get_i32(board, PROM_DOM_KEY_SGEMM_JUDGMENT_WINNING_SCORE, 0u, &i32_value) == 0u) return 0u;
  out_snapshot->decision.winning_score = i32_value;

  out_snapshot->packed4_selected = 0u;
  out_snapshot->packed4_reject_reason = 0u;
  out_snapshot->fp16_selected = 0u;
  out_snapshot->fp16_reject_reason = 0u;
  out_snapshot->use_dedicated_transfer_queue_upload = 0u;
  out_snapshot->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_REQUIRED;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_SELECTED, 0u, &u32_value) != 0u) out_snapshot->packed4_selected = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_PACKED4_REJECT_REASON, 0u, &u32_value) != 0u) out_snapshot->packed4_reject_reason = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_SELECTED, 0u, &u32_value) != 0u) out_snapshot->fp16_selected = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_FP16_REJECT_REASON, 0u, &u32_value) != 0u) out_snapshot->fp16_reject_reason = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_QUEUE_USED, 0u, &u32_value) != 0u) {
    out_snapshot->use_dedicated_transfer_queue_upload = u32_value;
  }
  if (prom_dom_get_u32(board, PROM_DOM_KEY_QUEUE_TRANSFER_FALLBACK_REASON, 0u, &u32_value) != 0u) {
    out_snapshot->transfer_fallback_reason = u32_value;
  }
  return 1u;
}

uint32_t prom_dom_sgemm_stage_resource_lease_facts(prom_dom_blackboard* board, const prom_resource_lease_facts* facts) {
  if (board == 0 || facts == 0) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_WORKER_ID, 0u, facts->worker_id, (int32_t)facts->worker_id) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_SLOT_ID, 0u, facts->slot_id, (int32_t)facts->slot_id) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_ENTRY_ID, 0u, facts->entry_id, (int32_t)facts->entry_id) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_SHAPE_CLASS, 0u, facts->shape_class, (int32_t)facts->shape_class) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_DEVICE_BAND, 0u, facts->device_band, (int32_t)facts->device_band) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_SELECTED_RECIPE_VARIANT, 0u, facts->selected_recipe_variant,
                       (int32_t)facts->selected_recipe_variant) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_REQUESTED_RESOURCE_CLASS, 0u, facts->requested_resource_class,
                       (int32_t)facts->requested_resource_class) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_REGISTER_PRESSURE_CLASS, 0u, facts->register_pressure_class,
                       (int32_t)facts->register_pressure_class) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_SHARED_MEMORY_PRESSURE_CLASS, 0u,
                       facts->shared_memory_pressure_class, (int32_t)facts->shared_memory_pressure_class) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_MEMORY_BANDWIDTH_PRESSURE_CLASS, 0u,
                       facts->memory_bandwidth_pressure_class, (int32_t)facts->memory_bandwidth_pressure_class) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_COMPUTE_PRESSURE_CLASS, 0u, facts->compute_pressure_class,
                       (int32_t)facts->compute_pressure_class) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_PIPELINE_LATENCY_PRESSURE_CLASS, 0u,
                       facts->pipeline_latency_pressure_class, (int32_t)facts->pipeline_latency_pressure_class) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_CURRENT_OUTSTANDING_DEPTH, 0u, facts->current_outstanding_depth,
                       (int32_t)facts->current_outstanding_depth) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_MAX_OUTSTANDING_DEPTH, 0u, facts->max_outstanding_depth,
                       (int32_t)facts->max_outstanding_depth) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_LOOKAHEAD_REQUESTED, 0u, facts->lookahead_requested,
                       (int32_t)facts->lookahead_requested) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_LOOKAHEAD_LIMIT, 0u, facts->lookahead_limit,
                       (int32_t)facts->lookahead_limit) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_SLOT_HFSM, PROM_DOM_KEY_SLOT_MAX_WIP_DEPTH, 0u, facts->max_outstanding_depth,
                       (int32_t)facts->max_outstanding_depth) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_TRANSFER_OVERLAP_AVAILABLE, 0u, facts->transfer_overlap_available,
                       (int32_t)facts->transfer_overlap_available) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_TRUE_MULTI_QUEUE_SELECTED, 0u, facts->true_multi_queue_selected,
                       (int32_t)facts->true_multi_queue_selected) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_UNSAFE_TO_REUSE, 0u, facts->unsafe_to_reuse,
                       (int32_t)facts->unsafe_to_reuse) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_YIELD_REQUESTED, 0u, facts->yield_requested,
                       (int32_t)facts->yield_requested) == 0u)
    return 0u;
  return 1u;
}

uint32_t prom_dom_sgemm_build_resource_lease_facts_from_visible(
    const prom_dom_blackboard* board,
    const prom_resource_lease_facts* fallback_facts,
    prom_dom_sgemm_resource_lease_projection* out_projection) {
  if (board == 0 || fallback_facts == 0 || out_projection == 0) return 0u;
  memset(out_projection, 0, sizeof(*out_projection));
  out_projection->facts = *fallback_facts;
  out_projection->visible_generation = board->visible_generation;
  out_projection->from_visible_snapshot = 1u;
  (void)prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_WORKER_ID, 0u, &out_projection->facts.worker_id);
  (void)prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_SLOT_ID, 0u, &out_projection->facts.slot_id);
  (void)prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_ENTRY_ID, 0u, &out_projection->facts.entry_id);
  (void)prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_SELECTED_RECIPE_VARIANT, 0u, &out_projection->facts.selected_recipe_variant);
  (void)prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_CURRENT_OUTSTANDING_DEPTH, 0u, &out_projection->facts.current_outstanding_depth);
  (void)prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_MAX_OUTSTANDING_DEPTH, 0u, &out_projection->facts.max_outstanding_depth);
  (void)prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_LOOKAHEAD_REQUESTED, 0u, &out_projection->facts.lookahead_requested);
  (void)prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_LOOKAHEAD_LIMIT, 0u, &out_projection->facts.lookahead_limit);
  (void)prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_UNSAFE_TO_REUSE, 0u, &out_projection->facts.unsafe_to_reuse);
  out_projection->facts.ready_slot_mask = board->slot_readiness_ready_slot_mask;
  out_projection->facts.failed_slot_mask = board->slot_readiness_failed_slot_mask;
  out_projection->facts.invalidated_slot_mask = board->slot_readiness_invalidated_slot_mask;
  out_projection->facts.slot_attention_mask = board->slot_readiness_attention_slot_mask;
  return 1u;
}

uint32_t prom_dom_sgemm_stage_resource_lease_decision(prom_dom_blackboard* board,
                                                      const prom_resource_lease_decision* decision) {
  prom_dom_event event;
  uint64_t counter = 0u;
  prom_dom_key counter_key = PROM_DOM_KEY_SGEMM_LEASE_DIAG_GRANTED_COUNT;
  prom_dom_event_kind event_kind = PROM_DOM_EVENT_LEASE_GRANTED;
  if (board == 0 || decision == 0) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_DECISION_SUCCESS, 0u, decision->success, decision->detail) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_DECISION_STATE, 0u, decision->lease_state, decision->detail) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_DECISION_GRANT, 0u, decision->grant, decision->detail) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_DECISION_REASON, 0u, decision->deny_reason, decision->detail) == 0u) return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_DECISION_ALLOWED_OUTSTANDING_DEPTH, 0u,
                       decision->allowed_outstanding_depth, decision->detail) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_DECISION_LOOKAHEAD_ALLOWED, 0u, decision->lookahead_allowed,
                       decision->detail) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_DECISION_BACKPRESSURE_APPLIED, 0u,
                       decision->backpressure_applied, decision->detail) == 0u)
    return 0u;
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_LEASE_DECISION_YIELD_REQUIRED, 0u, decision->yield_required,
                       decision->detail) == 0u)
    return 0u;

  if (decision->lease_state == PROM_LEASE_STATE_YIELDED) {
    counter_key = PROM_DOM_KEY_SGEMM_LEASE_DIAG_YIELD_COUNT;
    event_kind = PROM_DOM_EVENT_LEASE_YIELDED;
  } else if (decision->grant == 0u) {
    counter_key = PROM_DOM_KEY_SGEMM_LEASE_DIAG_DENIED_COUNT;
    event_kind = PROM_DOM_EVENT_LEASE_DENIED;
    if (decision->backpressure_applied != 0u && prom_dom_get_u64(board, PROM_DOM_KEY_SGEMM_LEASE_DIAG_BACKPRESSURE_COUNT, 0u, &counter) != 0u) {
      (void)prom_dom_set_u64(board, PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_SGEMM_LEASE_DIAG_BACKPRESSURE_COUNT, 0u, counter + 1u, decision->detail);
    } else if (decision->backpressure_applied != 0u) {
      (void)prom_dom_set_u64(board, PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_SGEMM_LEASE_DIAG_BACKPRESSURE_COUNT, 0u, 1u, decision->detail);
    }
  }

  if (prom_dom_get_u64(board, counter_key, 0u, &counter) == 0u) counter = 0u;
  if (prom_dom_set_u64(board, PROM_DOM_SOURCE_DIAGNOSTICS, counter_key, 0u, counter + 1u, decision->detail) == 0u) return 0u;
  if (decision->lookahead_allowed == 0u) {
    if (prom_dom_set_u32(board, PROM_DOM_SOURCE_DIAGNOSTICS, PROM_DOM_KEY_SGEMM_LEASE_DIAG_LOOKAHEAD_BLOCKED_REASON, 0u, decision->deny_reason,
                         decision->detail) == 0u)
      return 0u;
  }
  memset(&event, 0, sizeof(event));
  event.kind = event_kind;
  event.source = PROM_DOM_SOURCE_JUDGMENT;
  event.domain = PROM_DOM_DOMAIN_SGEMM;
  event.key = PROM_DOM_KEY_SGEMM_LEASE_DECISION_STATE;
  event.slot_id = decision->slot_id;
  event.reason_code = decision->detail;
  return prom_dom_stage_event(board, &event);
}

uint32_t prom_dom_sgemm_read_visible_resource_lease_diagnostics(const prom_dom_blackboard* board,
                                                                prom_dom_sgemm_resource_lease_snapshot* out_snapshot) {
  uint32_t u32_value;
  uint64_t u64_value;
  if (board == 0 || out_snapshot == 0) return 0u;
  memset(out_snapshot, 0, sizeof(*out_snapshot));
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_WORKER_ID, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.worker_id = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_SLOT_ID, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.slot_id = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_ENTRY_ID, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.entry_id = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_SELECTED_RECIPE_VARIANT, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.selected_recipe_variant = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_CURRENT_OUTSTANDING_DEPTH, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.current_outstanding_depth = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_MAX_OUTSTANDING_DEPTH, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.max_outstanding_depth = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_LOOKAHEAD_REQUESTED, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->facts.lookahead_requested = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_DECISION_STATE, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.lease_state = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_DECISION_GRANT, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.grant = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_DECISION_REASON, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.deny_reason = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_DECISION_ALLOWED_OUTSTANDING_DEPTH, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.allowed_outstanding_depth = u32_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_DECISION_LOOKAHEAD_ALLOWED, 0u, &u32_value) == 0u) return 0u;
  out_snapshot->decision.lookahead_allowed = u32_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SGEMM_LEASE_DIAG_GRANTED_COUNT, 0u, &u64_value) != 0u) out_snapshot->granted_count = u64_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SGEMM_LEASE_DIAG_DENIED_COUNT, 0u, &u64_value) != 0u) out_snapshot->denied_count = u64_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SGEMM_LEASE_DIAG_BACKPRESSURE_COUNT, 0u, &u64_value) != 0u) out_snapshot->backpressure_count = u64_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SGEMM_LEASE_DIAG_YIELD_COUNT, 0u, &u64_value) != 0u) out_snapshot->yield_count = u64_value;
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SGEMM_LEASE_DIAG_FAILED_COUNT, 0u, &u64_value) != 0u) out_snapshot->failed_count = u64_value;
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SGEMM_LEASE_DIAG_LOOKAHEAD_BLOCKED_REASON, 0u, &u32_value) != 0u) {
    out_snapshot->lookahead_blocked_reason = u32_value;
  }
  return 1u;
}
