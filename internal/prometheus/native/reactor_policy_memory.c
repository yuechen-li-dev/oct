#include "reactor_policy_memory.h"

static uint32_t max_u32(uint32_t a, uint32_t b) {
  return a >= b ? a : b;
}

static uint32_t decrement_saturating_u32(uint32_t value) {
  return value == 0u ? 0u : value - 1u;
}

static uint32_t increment_saturating_u32(uint32_t value) {
  return value == UINT32_MAX ? UINT32_MAX : value + 1u;
}

static uint32_t mode_is_valid(prom_policy_mode mode) {
  return mode == PROM_POLICY_MODE_AGGRESSIVE || mode == PROM_POLICY_MODE_SAFE || mode == PROM_POLICY_MODE_RECOVERY;
}

void prom_policy_memory_init(prom_policy_memory* memory, prom_policy_mode initial_mode) {
  if (memory == 0) {
    return;
  }

  memory->current_mode = mode_is_valid(initial_mode) != 0u ? initial_mode : PROM_POLICY_MODE_AGGRESSIVE;
  memory->decisions_in_mode = 0u;
  memory->cooldown_remaining = 0u;
  memory->recovery_cooldown_remaining = 0u;
}

prom_policy_mode prom_policy_memory_update(prom_policy_memory* memory,
                                           const prom_policy_facts* facts,
                                           const prom_policy_thresholds* thresholds) {
  prom_policy_mode current_mode;
  prom_policy_mode next_mode;
  uint32_t metric;
  uint32_t hard_override;
  uint32_t can_transition;
  if (memory == 0 || facts == 0 || thresholds == 0) {
    return PROM_POLICY_MODE_AGGRESSIVE;
  }

  if (mode_is_valid(memory->current_mode) == 0u) {
    prom_policy_memory_init(memory, PROM_POLICY_MODE_AGGRESSIVE);
  }

  memory->cooldown_remaining = decrement_saturating_u32(memory->cooldown_remaining);
  memory->recovery_cooldown_remaining = decrement_saturating_u32(memory->recovery_cooldown_remaining);

  metric = max_u32(facts->waste_ratio_permille, facts->pending_waste_ratio_permille);
  current_mode = memory->current_mode;
  next_mode = current_mode;
  hard_override = facts->hard_retreat_override != 0u || facts->hard_recovery_override != 0u ? 1u : 0u;
  can_transition = hard_override != 0u || memory->decisions_in_mode >= thresholds->min_commit_decisions ? 1u : 0u;

  if (facts->hard_recovery_override != 0u) {
    next_mode = PROM_POLICY_MODE_RECOVERY;
  } else if (facts->hard_retreat_override != 0u) {
    next_mode = PROM_POLICY_MODE_SAFE;
  } else if (current_mode == PROM_POLICY_MODE_AGGRESSIVE) {
    if (metric >= thresholds->retreat_enter_permille) {
      next_mode = PROM_POLICY_MODE_SAFE;
    }
  } else if (current_mode == PROM_POLICY_MODE_SAFE) {
    if (metric >= thresholds->recovery_enter_permille) {
      next_mode = PROM_POLICY_MODE_RECOVERY;
    } else if (metric <= thresholds->retreat_exit_permille && memory->cooldown_remaining == 0u) {
      next_mode = PROM_POLICY_MODE_AGGRESSIVE;
    }
  } else {
    if (metric <= thresholds->recovery_exit_permille && memory->recovery_cooldown_remaining == 0u) {
      next_mode = PROM_POLICY_MODE_SAFE;
    }
  }

  if (can_transition == 0u) {
    next_mode = current_mode;
  }

  if (next_mode != current_mode) {
    memory->current_mode = next_mode;
    memory->decisions_in_mode = 1u;
    if (next_mode == PROM_POLICY_MODE_SAFE) {
      memory->cooldown_remaining = thresholds->retreat_cooldown_decisions;
    } else if (next_mode == PROM_POLICY_MODE_RECOVERY) {
      memory->recovery_cooldown_remaining = thresholds->recovery_hold_decisions;
    }
  } else {
    memory->decisions_in_mode = increment_saturating_u32(memory->decisions_in_mode);
  }

  return memory->current_mode;
}
