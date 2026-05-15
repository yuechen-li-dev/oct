#include "reactor_dominatus_predictor.h"

#include <stddef.h>
#include <string.h>

#define PROM_DOM_FALLBACK_NONE 0u
#define PROM_DOM_FALLBACK_HARD_GATE 1u
#define PROM_DOM_FALLBACK_RING_FULL 2u

static double clamp01(double x) {
  if (x < 0.0) return 0.0;
  if (x > 1.0) return 1.0;
  return x;
}

static uint32_t has_hard_gate(const prom_dominatus_physical_observation* physical) {
  if (physical == NULL) return 1u;
  if (physical->runtime_unsafe != 0u) return 1u;
  if (physical->slot_valid == 0u) return 1u;
  if (physical->memory_budget_ok == 0u) return 1u;
  if (physical->outstanding_depth_cap != 0u && physical->outstanding_depth >= physical->outstanding_depth_cap) return 1u;
  return 0u;
}

prom_dominatus_predictor_evidence prom_dominatus_predictor_evidence_from_filtered(
    const prom_dominatus_filtered_evidence* evidence) {
  prom_dominatus_predictor_evidence out;
  memset(&out, 0, sizeof(out));
  if (evidence == NULL) return out;
  out.valid = evidence->valid;
  out.warmup = evidence->filter_warmup;
  out.filtered_value = evidence->filtered_value;
  out.raw_value = evidence->raw_value;
  out.confidence = evidence->confidence;
  out.sample_count = evidence->sample_count;
  out.outlier_count = evidence->outlier_count;
  return out;
}

prom_dominatus_predictor_params prom_dominatus_predictor_default_params(void) {
  prom_dominatus_predictor_params params;
  params.confidence_threshold_depth1 = 0.45;
  params.confidence_threshold_depth2 = 0.75;
  params.correction_penalty = 0.20;
  params.correction_reward = 0.05;
  params.max_lookahead_depth = 2u;
  params.max_outstanding_depth = 2u;
  return params;
}

void prom_dominatus_predictor_init(prom_dominatus_predictor_state* state,
                                   const prom_dominatus_predictor_params* params) {
  if (state == NULL) return;
  memset(state, 0, sizeof(*state));
  state->params = params == NULL ? prom_dominatus_predictor_default_params() : *params;
  state->prediction_confidence = state->params.confidence_threshold_depth1;
  state->initialized = 1u;
}

void prom_dominatus_predictor_reset(prom_dominatus_predictor_state* state) {
  if (state == NULL) return;
  memset(state->ring, 0, sizeof(state->ring));
  state->ring_head = 0u;
  state->ring_count = 0u;
  state->lookahead_depth = 0u;
  state->prediction_confidence = state->params.confidence_threshold_depth1;
  state->last_prediction_error = 0;
  state->correction_count = 0u;
  state->stale_prediction_count = 0u;
  state->future_lease_requested = 0u;
  state->future_lease_granted = 0u;
  state->future_lease_cancelled = 0u;
  state->predictor_stale = 0u;
  state->fallback_active = 0u;
  state->fallback_reason = PROM_DOM_FALLBACK_NONE;
  state->last_tick = 0u;
  state->next_lease_request_id = 0u;
  state->initialized = 0u;
}

uint32_t prom_dominatus_predictor_select_depth(const prom_dominatus_predictor_state* state,
                                               const prom_dominatus_predictor_evidence* evidence,
                                               const prom_dominatus_physical_observation* physical) {
  uint32_t depth = 0u;
  if (state == NULL || evidence == NULL) return 0u;
  if (evidence->valid == 0u || evidence->warmup != 0u) return 0u;
  if (evidence->confidence < state->params.confidence_threshold_depth1) return 0u;
  if (has_hard_gate(physical) != 0u) return 0u;

  depth = 1u;
  if (evidence->confidence >= state->params.confidence_threshold_depth2 && evidence->outlier_count <= 1u &&
      state->last_prediction_error <= 0 && state->predictor_stale == 0u) {
    depth = 2u;
  }
  if (depth > state->params.max_lookahead_depth) depth = state->params.max_lookahead_depth;
  return depth;
}

uint32_t prom_dominatus_predictor_issue(prom_dominatus_predictor_state* state,
                                        const prom_dominatus_predictor_evidence* evidence,
                                        const prom_dominatus_physical_observation* physical,
                                        uint64_t tick,
                                        prom_dominatus_prediction_entry* out_entry) {
  prom_dominatus_prediction_entry entry;
  uint32_t depth;
  uint32_t idx;
  if (out_entry != NULL) memset(out_entry, 0, sizeof(*out_entry));
  if (state == NULL || evidence == NULL) return 0u;

  state->fallback_active = 0u;
  state->fallback_reason = PROM_DOM_FALLBACK_NONE;
  depth = prom_dominatus_predictor_select_depth(state, evidence, physical);
  state->lookahead_depth = depth;
  state->last_tick = tick;

  if (has_hard_gate(physical) != 0u) {
    state->fallback_active = 1u;
    state->fallback_reason = PROM_DOM_FALLBACK_HARD_GATE;
    return 0u;
  }
  if (depth == 0u) return 0u;
  if (state->ring_count >= PROM_DOM_PREDICTION_RING_CAP) {
    state->predictor_stale = 1u;
    state->fallback_active = 1u;
    state->fallback_reason = PROM_DOM_FALLBACK_RING_FULL;
    state->stale_prediction_count += 1u;
    return 0u;
  }

  memset(&entry, 0, sizeof(entry));
  entry.active = 1u;
  entry.issued_tick = tick;
  entry.target_tick = tick + (uint64_t)depth;
  entry.predicted_ready = 1u;
  entry.predicted_latency = evidence->filtered_value;
  entry.prediction_confidence = evidence->confidence < state->prediction_confidence ? evidence->confidence : state->prediction_confidence;
  entry.lookahead_depth = depth;
  entry.lease_request_id = ++state->next_lease_request_id;
  entry.future_lease_state = PROM_DOM_FUTURE_LEASE_REQUESTED;

  idx = (state->ring_head + state->ring_count) % PROM_DOM_PREDICTION_RING_CAP;
  state->ring[idx] = entry;
  state->ring_count += 1u;
  state->future_lease_requested += 1u;

  if (out_entry != NULL) *out_entry = entry;
  return 1u;
}

prom_dominatus_correction_event prom_dominatus_predictor_mature(
    prom_dominatus_predictor_state* state,
    const prom_dominatus_physical_observation* physical,
    uint64_t tick) {
  prom_dominatus_correction_event ev;
  prom_dominatus_prediction_entry* entry;
  memset(&ev, 0, sizeof(ev));
  if (state == NULL || state->ring_count == 0u) return ev;

  entry = &state->ring[state->ring_head];
  if (entry->active == 0u || entry->target_tick > tick) return ev;

  ev.valid = 1u;
  ev.tick = tick;
  ev.prediction_matured = 1u;
  ev.predicted_ready = entry->predicted_ready;
  ev.actual_ready = physical == NULL ? 0u : physical->actual_ready;
  ev.arrival_error_ticks = tick >= entry->target_tick ? (int64_t)(tick - entry->target_tick) : 0;
  ev.state_mismatch = ev.predicted_ready == ev.actual_ready ? 0u : 1u;
  ev.entry_index = state->ring_head;
  ev.issued_tick = entry->issued_tick;
  ev.target_tick = entry->target_tick;

  if (has_hard_gate(physical) != 0u) {
    state->fallback_active = 1u;
    state->fallback_reason = PROM_DOM_FALLBACK_HARD_GATE;
    state->predictor_stale = 1u;
    ev.action = PROM_DOM_CORRECTION_ACTION_MARK_STALE;
  }

  if (ev.state_mismatch == 0u) {
    state->prediction_confidence = clamp01(state->prediction_confidence + state->params.correction_reward);
    ev.confidence_delta = state->params.correction_reward;
    if (ev.action == PROM_DOM_CORRECTION_ACTION_NONE) ev.action = PROM_DOM_CORRECTION_ACTION_NONE;
    state->last_prediction_error = 0;
  } else {
    state->prediction_confidence = clamp01(state->prediction_confidence - state->params.correction_penalty);
    ev.confidence_delta = -state->params.correction_penalty;
    state->correction_count += 1u;
    state->last_prediction_error = ev.arrival_error_ticks;
    if (state->lookahead_depth > 0u) state->lookahead_depth -= 1u;
    if (state->lookahead_depth > 0u) {
      ev.action = PROM_DOM_CORRECTION_ACTION_REDUCE_DEPTH;
    } else {
      ev.action = PROM_DOM_CORRECTION_ACTION_LOWER_CONFIDENCE;
    }
    if (state->prediction_confidence < state->params.confidence_threshold_depth1) {
      state->fallback_active = 1u;
    }
  }

  memset(entry, 0, sizeof(*entry));
  state->ring_head = (state->ring_head + 1u) % PROM_DOM_PREDICTION_RING_CAP;
  state->ring_count -= 1u;
  state->last_tick = tick;
  return ev;
}

prom_dominatus_correction_event prom_dominatus_predictor_update(
    prom_dominatus_predictor_state* state,
    const prom_dominatus_predictor_evidence* evidence,
    const prom_dominatus_physical_observation* physical,
    uint64_t tick,
    prom_dominatus_prediction_entry* out_issued) {
  const prom_dominatus_correction_event ev = prom_dominatus_predictor_mature(state, physical, tick);
  (void)prom_dominatus_predictor_issue(state, evidence, physical, tick, out_issued);
  return ev;
}
