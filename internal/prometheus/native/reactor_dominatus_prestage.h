#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_PRESTAGE_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_PRESTAGE_H

#include "reactor_dominatus_predictor.h"

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef enum prom_dominatus_prestage_state {
  PROM_DOM_PRESTAGE_NONE = 0,
  PROM_DOM_PRESTAGE_ELIGIBLE,
  PROM_DOM_PRESTAGE_BLOCKED,
  PROM_DOM_PRESTAGE_SUBMITTED,
  PROM_DOM_PRESTAGE_READY,
  PROM_DOM_PRESTAGE_CANCELLED,
  PROM_DOM_PRESTAGE_EXPIRED,
  PROM_DOM_PRESTAGE_WASTED,
  PROM_DOM_PRESTAGE_MATURED
} prom_dominatus_prestage_state;

enum {
  PROM_DOM_PRESTAGE_BLOCK_CONFIDENCE = 1u << 0,
  PROM_DOM_PRESTAGE_BLOCK_WARMUP = 1u << 1,
  PROM_DOM_PRESTAGE_BLOCK_RESERVATION = 1u << 2,
  PROM_DOM_PRESTAGE_BLOCK_RECENT_MISS = 1u << 3,
  PROM_DOM_PRESTAGE_BLOCK_HARD_GATE = 1u << 4,
  PROM_DOM_PRESTAGE_BLOCK_RESOURCE_PRESSURE = 1u << 5,
  PROM_DOM_PRESTAGE_BLOCK_LEAD_TIME = 1u << 6,
  PROM_DOM_PRESTAGE_BLOCK_FEATURE_DISABLED = 1u << 7,
  PROM_DOM_PRESTAGE_BLOCK_INVALID = 1u << 8,
};

typedef struct prom_dominatus_prestage_params {
  uint32_t action_enabled;
  double confidence_threshold;
  uint32_t recent_miss_window;
  uint32_t max_lead_ticks;
  double cost_estimate_low;
  double cost_estimate_medium;
} prom_dominatus_prestage_params;

typedef struct prom_dominatus_prestage_input {
  uint32_t valid;
  uint64_t request_id;
  uint64_t current_tick;
  uint64_t target_tick;
  uint32_t reservation_is_reserved;
  double confidence;
  uint32_t warmup;
  uint32_t recent_miss_count;
  uint32_t runtime_unsafe;
  uint32_t slot_valid;
  uint32_t memory_budget_ok;
  uint32_t outstanding_depth;
  uint32_t outstanding_depth_cap;
  uint32_t resource_pressure_low;
} prom_dominatus_prestage_input;

typedef struct prom_dominatus_prestage_decision {
  uint32_t valid;
  prom_dominatus_prestage_state state;
  uint32_t allowed;
  uint32_t submitted;
  uint32_t ready;
  uint32_t cancelled;
  uint32_t expired;
  uint32_t wasted;
  uint32_t matured;
  uint32_t block_reasons;
  uint64_t request_id;
  uint64_t target_tick;
  uint32_t lead_ticks;
  double confidence;
  double cost_estimate;
  double benefit_estimate;
} prom_dominatus_prestage_decision;

prom_dominatus_prestage_params prom_dominatus_prestage_default_params(void);
prom_dominatus_prestage_decision prom_dominatus_prestage_evaluate(
    const prom_dominatus_prestage_params* params,
    const prom_dominatus_prestage_input* input);

#ifdef __cplusplus
}
#endif

#endif
