#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_PREDICTOR_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_PREDICTOR_H

#include "reactor_dominatus_measurement_filter.h"

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define PROM_DOM_PREDICTION_RING_CAP 16u

typedef enum prom_dominatus_correction_action {
  PROM_DOM_CORRECTION_ACTION_NONE = 0,
  PROM_DOM_CORRECTION_ACTION_LOWER_CONFIDENCE,
  PROM_DOM_CORRECTION_ACTION_REDUCE_DEPTH,
  PROM_DOM_CORRECTION_ACTION_CANCEL_FUTURE_LEASE,
  PROM_DOM_CORRECTION_ACTION_FALLBACK,
  PROM_DOM_CORRECTION_ACTION_MARK_STALE
} prom_dominatus_correction_action;

typedef enum prom_dominatus_future_lease_state {
  PROM_DOM_FUTURE_LEASE_NONE = 0,
  PROM_DOM_FUTURE_LEASE_REQUESTED,
  PROM_DOM_FUTURE_LEASE_GRANTED,
  PROM_DOM_FUTURE_LEASE_DENIED,
  PROM_DOM_FUTURE_LEASE_CANCELLED,
  PROM_DOM_FUTURE_LEASE_MATURED,
  PROM_DOM_FUTURE_LEASE_YIELDED,
  PROM_DOM_FUTURE_LEASE_EXPIRED
} prom_dominatus_future_lease_state;

typedef struct prom_dominatus_predictor_evidence {
  uint32_t valid;
  uint32_t warmup;
  double filtered_value;
  double raw_value;
  double confidence;
  uint32_t sample_count;
  uint32_t outlier_count;
} prom_dominatus_predictor_evidence;

typedef struct prom_dominatus_physical_observation {
  uint64_t tick;
  uint32_t actual_ready;
  uint32_t slot_valid;
  uint32_t runtime_unsafe;
  uint32_t outstanding_depth;
  uint32_t outstanding_depth_cap;
  uint32_t memory_budget_ok;
} prom_dominatus_physical_observation;

typedef struct prom_dominatus_prediction_entry {
  uint32_t active;
  uint64_t issued_tick;
  uint64_t target_tick;
  uint32_t predicted_ready;
  uint32_t predicted_slot_id;
  uint32_t predicted_shape_class;
  uint32_t predicted_variant;
  double predicted_latency;
  double prediction_confidence;
  uint32_t lookahead_depth;
  uint64_t lease_request_id;
  prom_dominatus_future_lease_state future_lease_state;
} prom_dominatus_prediction_entry;

typedef struct prom_dominatus_correction_event {
  uint32_t valid;
  uint64_t tick;
  uint32_t prediction_matured;
  uint32_t predicted_ready;
  uint32_t actual_ready;
  int64_t arrival_error_ticks;
  uint32_t state_mismatch;
  double confidence_delta;
  prom_dominatus_correction_action action;
  uint32_t entry_index;
  uint64_t issued_tick;
  uint64_t target_tick;
} prom_dominatus_correction_event;

typedef struct prom_dominatus_predictor_params {
  double confidence_threshold_depth1;
  double confidence_threshold_depth2;
  double correction_penalty;
  double correction_reward;
  uint32_t max_lookahead_depth;
  uint32_t max_outstanding_depth;
} prom_dominatus_predictor_params;

typedef struct prom_dominatus_predictor_state {
  prom_dominatus_predictor_params params;
  prom_dominatus_prediction_entry ring[PROM_DOM_PREDICTION_RING_CAP];
  uint32_t ring_head;
  uint32_t ring_count;
  uint32_t lookahead_depth;
  double prediction_confidence;
  int64_t last_prediction_error;
  uint64_t correction_count;
  uint64_t stale_prediction_count;
  uint64_t future_lease_requested;
  uint64_t future_lease_granted;
  uint64_t future_lease_cancelled;
  uint32_t predictor_stale;
  uint32_t fallback_active;
  uint32_t fallback_reason;
  uint64_t last_tick;
  uint64_t next_lease_request_id;
  uint32_t initialized;
} prom_dominatus_predictor_state;

prom_dominatus_predictor_evidence prom_dominatus_predictor_evidence_from_filtered(
    const prom_dominatus_filtered_evidence* evidence);
prom_dominatus_predictor_params prom_dominatus_predictor_default_params(void);
void prom_dominatus_predictor_init(prom_dominatus_predictor_state* state,
                                   const prom_dominatus_predictor_params* params);
void prom_dominatus_predictor_reset(prom_dominatus_predictor_state* state);
uint32_t prom_dominatus_predictor_select_depth(const prom_dominatus_predictor_state* state,
                                               const prom_dominatus_predictor_evidence* evidence,
                                               const prom_dominatus_physical_observation* physical);
uint32_t prom_dominatus_predictor_issue(prom_dominatus_predictor_state* state,
                                        const prom_dominatus_predictor_evidence* evidence,
                                        const prom_dominatus_physical_observation* physical,
                                        uint64_t tick,
                                        prom_dominatus_prediction_entry* out_entry);
prom_dominatus_correction_event prom_dominatus_predictor_mature(
    prom_dominatus_predictor_state* state,
    const prom_dominatus_physical_observation* physical,
    uint64_t tick);
prom_dominatus_correction_event prom_dominatus_predictor_update(
    prom_dominatus_predictor_state* state,
    const prom_dominatus_predictor_evidence* evidence,
    const prom_dominatus_physical_observation* physical,
    uint64_t tick,
    prom_dominatus_prediction_entry* out_issued);

#ifdef __cplusplus
}
#endif

#endif
