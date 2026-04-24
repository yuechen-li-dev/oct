#include "reactor_dominatus_sgemm_adapter.h"

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
  return mask;
}

uint32_t prom_dom_sgemm_stage_m35(prom_dom_blackboard* board,
                                  const prom_buffering_selector_facts* facts,
                                  const prom_buffering_selector_decision* decision) {
  if (board == 0 || facts == 0 || decision == 0) {
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

  if (prom_dom_set_u64(board,
                       PROM_DOM_SOURCE_MEMORY,
                       PROM_DOM_KEY_MEMORY_BUDGET,
                       0u,
                       (uint64_t)facts->memory_budget_slots_permille,
                       (int32_t)decision->final_reason_code) == 0u) {
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

  return 1u;
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

  return 1u;
}

uint32_t prom_dom_sgemm_build_buffering_selector_facts_from_visible(
    const prom_dom_blackboard* board,
    const prom_buffering_selector_facts* fallback_facts,
    prom_dom_sgemm_buffering_projection* out_projection) {
  prom_buffering_selector_facts facts;
  uint64_t budget_value;
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
  facts.serial_jit_headroom_slots_permille = i32_value;
  facts.memory_budget_slots_permille = (uint32_t)budget_value;
  out_projection->from_visible_snapshot = 1u;
  out_projection->facts = facts;
  return 1u;
}
