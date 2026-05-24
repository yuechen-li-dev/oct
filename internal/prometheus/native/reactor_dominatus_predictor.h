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

typedef enum prom_dominatus_reservation_state {
  PROM_DOM_RESERVATION_NONE = 0,
  PROM_DOM_RESERVATION_REQUESTED,
  PROM_DOM_RESERVATION_RESERVED,
  PROM_DOM_RESERVATION_DENIED,
  PROM_DOM_RESERVATION_CANCELLED,
  PROM_DOM_RESERVATION_MATURED,
  PROM_DOM_RESERVATION_EXPIRED,
  PROM_DOM_RESERVATION_YIELDED
} prom_dominatus_reservation_state;

typedef enum prom_dominatus_shadow_state {
  PROM_DOM_SHADOW_STATE_UNKNOWN = 0,
  PROM_DOM_SHADOW_STATE_IDLE,
  PROM_DOM_SHADOW_STATE_FORECAST_ISSUED,
  PROM_DOM_SHADOW_STATE_FUTURE_LEASE_REQUESTED,
  PROM_DOM_SHADOW_STATE_RESERVED,
  PROM_DOM_SHADOW_STATE_PRESTAGE_ELIGIBLE,
  PROM_DOM_SHADOW_STATE_PREDICTED_READY,
  PROM_DOM_SHADOW_STATE_MATURED,
  PROM_DOM_SHADOW_STATE_CANCELLED,
  PROM_DOM_SHADOW_STATE_STALE,
  PROM_DOM_SHADOW_STATE_FALLBACK
} prom_dominatus_shadow_state;

typedef enum prom_dominatus_shadow_mismatch_kind {
  PROM_DOM_SHADOW_MISMATCH_NONE = 0,
  PROM_DOM_SHADOW_MISMATCH_MATCH,
  PROM_DOM_SHADOW_MISMATCH_LATE,
  PROM_DOM_SHADOW_MISMATCH_EARLY,
  PROM_DOM_SHADOW_MISMATCH_PHYSICAL_NOT_READY,
  PROM_DOM_SHADOW_MISMATCH_SHADOW_NOT_READY,
  PROM_DOM_SHADOW_MISMATCH_CANCELLED,
  PROM_DOM_SHADOW_MISMATCH_STALE,
  PROM_DOM_SHADOW_MISMATCH_FALLBACK,
  PROM_DOM_SHADOW_MISMATCH_HARD_GATE,
  PROM_DOM_SHADOW_MISMATCH_INVALID_PREDICTION
} prom_dominatus_shadow_mismatch_kind;

#define PROM_DOM_RESERVATION_CAP 16u

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

typedef struct prom_dominatus_future_lease_request {
  uint32_t valid;
  uint64_t request_id;
  uint64_t issued_tick;
  uint64_t target_tick;
  uint32_t resource_kind;
  uint32_t slot_id;
  uint32_t shape_class;
  uint32_t variant_id;
  uint32_t lookahead_depth;
  double confidence;
  prom_dominatus_future_lease_state state;
  uint32_t cancel_reason;
  uint32_t deny_reason;
} prom_dominatus_future_lease_request;

typedef struct prom_dominatus_future_lease_decision {
  uint32_t valid;
  uint64_t request_id;
  prom_dominatus_future_lease_state previous_state;
  prom_dominatus_future_lease_state new_state;
  uint32_t requested;
  uint32_t granted;
  uint32_t denied;
  uint32_t cancelled;
  uint32_t matured;
  uint32_t expired;
  uint32_t yielded;
  uint32_t reason;
  uint64_t target_tick;
  double confidence;
} prom_dominatus_future_lease_decision;

typedef struct prom_dominatus_future_lease_seam_state {
  uint64_t next_request_id;
  uint64_t requested_count;
  uint64_t granted_count;
  uint64_t denied_count;
  uint64_t cancelled_count;
  uint64_t matured_count;
  uint64_t expired_count;
  uint64_t yielded_count;
  prom_dominatus_future_lease_request last_request;
  prom_dominatus_future_lease_request last_matured;
} prom_dominatus_future_lease_seam_state;

typedef struct prom_dominatus_reservation_request {
  uint32_t valid;
  uint64_t request_id;
  uint64_t issued_tick;
  uint64_t target_tick;
  uint32_t resource_kind;
  uint32_t slot_id;
  uint32_t shape_class;
  uint32_t variant_id;
  uint32_t lookahead_depth;
  double confidence;
  prom_dominatus_reservation_state state;
  uint32_t deny_reason;
  uint32_t cancel_reason;
} prom_dominatus_reservation_request;

typedef struct prom_dominatus_reservation_state_set {
  prom_dominatus_reservation_request entries[PROM_DOM_RESERVATION_CAP];
  uint64_t next_request_id;
  uint64_t requested_count;
  uint64_t reserved_count;
  uint64_t denied_count;
  uint64_t cancelled_count;
  uint64_t matured_count;
  uint64_t expired_count;
  uint64_t yielded_count;
  uint32_t active_count;
} prom_dominatus_reservation_state_set;

typedef struct prom_dominatus_reservation_params {
  uint32_t capacity;
  uint32_t max_lookahead_depth;
  double min_confidence;
  uint64_t max_future_ticks;
  uint64_t expiry_slack_ticks;
} prom_dominatus_reservation_params;

typedef struct prom_dominatus_reservation_decision {
  uint32_t valid;
  uint64_t request_id;
  prom_dominatus_reservation_state previous_state;
  prom_dominatus_reservation_state new_state;
  uint32_t reserved;
  uint32_t denied;
  uint32_t cancelled;
  uint32_t matured;
  uint32_t expired;
  uint32_t yielded;
  uint32_t reason;
  uint64_t target_tick;
  double confidence;
  uint32_t active_count;
} prom_dominatus_reservation_decision;

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

typedef struct prom_dominatus_shadow_snapshot {
  uint32_t valid;
  prom_dominatus_shadow_state shadow_state;
  uint32_t physical_state;
  uint64_t issued_tick;
  uint64_t target_tick;
  uint64_t predicted_ready_tick;
  uint64_t actual_ready_tick;
  int64_t arrival_error_ticks;
  double prediction_confidence;
  prom_dominatus_shadow_mismatch_kind mismatch_kind;
  uint32_t matched;
  uint32_t stale;
  uint32_t cancelled;
  uint32_t fallback;
  prom_dominatus_correction_action correction_action;
  uint64_t correction_count;
  uint64_t stale_count;
  uint64_t miss_count;
} prom_dominatus_shadow_snapshot;

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
  prom_dominatus_future_lease_seam_state future_lease_seam;
  prom_dominatus_reservation_state_set reservations;
  prom_dominatus_reservation_params reservation_params;
  uint32_t initialized;
} prom_dominatus_predictor_state;

void prom_dominatus_future_lease_seam_init(prom_dominatus_future_lease_seam_state* state);
void prom_dominatus_future_lease_seam_reset(prom_dominatus_future_lease_seam_state* state);
prom_dominatus_future_lease_decision prom_dominatus_future_lease_request_issue(
    prom_dominatus_future_lease_seam_state* state,
    const prom_dominatus_prediction_entry* prediction,
    uint64_t tick);
prom_dominatus_future_lease_decision prom_dominatus_future_lease_grant(
    prom_dominatus_future_lease_seam_state* state,
    uint64_t request_id,
    uint64_t tick);
prom_dominatus_future_lease_decision prom_dominatus_future_lease_deny(
    prom_dominatus_future_lease_seam_state* state,
    uint64_t request_id,
    uint32_t reason,
    uint64_t tick);
prom_dominatus_future_lease_decision prom_dominatus_future_lease_cancel(
    prom_dominatus_future_lease_seam_state* state,
    uint64_t request_id,
    uint32_t reason,
    uint64_t tick);
prom_dominatus_future_lease_decision prom_dominatus_future_lease_mature(
    prom_dominatus_future_lease_seam_state* state,
    uint64_t request_id,
    uint64_t tick);

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
prom_dominatus_reservation_decision prom_dominatus_predictor_apply_correction_to_reservation(
    prom_dominatus_predictor_state* predictor,
    prom_dominatus_reservation_state_set* reservations,
    const prom_dominatus_prediction_entry* matured_entry,
    const prom_dominatus_correction_event* correction,
    uint64_t tick);

prom_dominatus_reservation_params prom_dominatus_reservation_default_params(void);
void prom_dominatus_reservation_init(prom_dominatus_reservation_state_set* state,
                                     const prom_dominatus_reservation_params* params);
void prom_dominatus_reservation_reset(prom_dominatus_reservation_state_set* state);
prom_dominatus_reservation_decision prom_dominatus_reservation_request_from_future_lease(
    prom_dominatus_reservation_state_set* state,
    const prom_dominatus_reservation_params* params,
    const prom_dominatus_future_lease_request* request,
    uint64_t tick);
prom_dominatus_reservation_decision prom_dominatus_reservation_cancel(prom_dominatus_reservation_state_set* state,
                                                                      uint64_t request_id,
                                                                      uint32_t reason,
                                                                      uint64_t tick);
prom_dominatus_reservation_decision prom_dominatus_reservation_mature(prom_dominatus_reservation_state_set* state,
                                                                      uint64_t tick);
prom_dominatus_reservation_decision prom_dominatus_reservation_expire_stale(
    prom_dominatus_reservation_state_set* state,
    const prom_dominatus_reservation_params* params,
    uint64_t tick);
prom_dominatus_reservation_decision prom_dominatus_predictor_try_reserve_future(
    prom_dominatus_predictor_state* predictor,
    prom_dominatus_reservation_state_set* reservations,
    const prom_dominatus_future_lease_request* future_request,
    uint64_t tick);
prom_dominatus_shadow_snapshot prom_dominatus_shadow_snapshot_evaluate(
    const prom_dominatus_predictor_state* predictor,
    const prom_dominatus_prediction_entry* last_issued,
    const prom_dominatus_correction_event* correction,
    const prom_dominatus_reservation_decision* reservation,
    uint32_t prestage_allowed,
    uint64_t current_tick);

#ifdef __cplusplus
}
#endif

#endif
