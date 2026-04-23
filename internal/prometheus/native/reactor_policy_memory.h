#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_POLICY_MEMORY_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_POLICY_MEMORY_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef enum prom_policy_mode {
  PROM_POLICY_MODE_AGGRESSIVE = 1,
  PROM_POLICY_MODE_SAFE = 2,
  PROM_POLICY_MODE_RECOVERY = 3,
} prom_policy_mode;

typedef struct prom_policy_memory {
  prom_policy_mode current_mode;
  uint32_t decisions_in_mode;
  uint32_t cooldown_remaining;
  uint32_t recovery_cooldown_remaining;
} prom_policy_memory;

typedef struct prom_policy_facts {
  uint32_t waste_ratio_permille;
  uint32_t pending_waste_ratio_permille;
  uint32_t hard_retreat_override;
  uint32_t hard_recovery_override;
} prom_policy_facts;

typedef struct prom_policy_thresholds {
  uint32_t retreat_enter_permille;
  uint32_t retreat_exit_permille;
  uint32_t recovery_enter_permille;
  uint32_t recovery_exit_permille;
  uint32_t min_commit_decisions;
  uint32_t retreat_cooldown_decisions;
  uint32_t recovery_hold_decisions;
} prom_policy_thresholds;

void prom_policy_memory_init(prom_policy_memory* memory, prom_policy_mode initial_mode);
prom_policy_mode prom_policy_memory_update(prom_policy_memory* memory,
                                           const prom_policy_facts* facts,
                                           const prom_policy_thresholds* thresholds);

#ifdef __cplusplus
}
#endif

#endif
