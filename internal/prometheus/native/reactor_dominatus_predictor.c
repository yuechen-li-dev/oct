#include "reactor_dominatus_predictor.h"

#include <stddef.h>
#include <string.h>

#define PROM_DOM_FALLBACK_NONE 0u
#define PROM_DOM_FALLBACK_HARD_GATE 1u
#define PROM_DOM_FALLBACK_RING_FULL 2u

static prom_dominatus_reservation_decision reservation_transition_by_request_id(
    prom_dominatus_reservation_state_set* state,
    uint64_t request_id,
    prom_dominatus_reservation_state next,
    uint32_t reason);

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

static prom_dominatus_future_lease_decision future_lease_transition(
    prom_dominatus_future_lease_seam_state* state,
    uint64_t request_id,
    prom_dominatus_future_lease_state new_state,
    uint32_t reason) {
  prom_dominatus_future_lease_decision out;
  memset(&out, 0, sizeof(out));
  if (state == NULL || state->last_request.valid == 0u || state->last_request.request_id != request_id) return out;
  out.valid = 1u;
  out.request_id = request_id;
  out.previous_state = state->last_request.state;
  out.new_state = new_state;
  out.reason = reason;
  out.target_tick = state->last_request.target_tick;
  out.confidence = state->last_request.confidence;
  state->last_request.state = new_state;
  if (new_state == PROM_DOM_FUTURE_LEASE_GRANTED) {
    out.granted = 1u;
    state->granted_count += 1u;
  } else if (new_state == PROM_DOM_FUTURE_LEASE_DENIED) {
    out.denied = 1u;
    state->denied_count += 1u;
    state->last_request.deny_reason = reason;
  } else if (new_state == PROM_DOM_FUTURE_LEASE_CANCELLED) {
    out.cancelled = 1u;
    state->cancelled_count += 1u;
    state->last_request.cancel_reason = reason;
  } else if (new_state == PROM_DOM_FUTURE_LEASE_MATURED) {
    out.matured = 1u;
    state->matured_count += 1u;
    state->last_matured = state->last_request;
  }
  return out;
}

void prom_dominatus_future_lease_seam_init(prom_dominatus_future_lease_seam_state* state) { prom_dominatus_future_lease_seam_reset(state); }

prom_dominatus_reservation_params prom_dominatus_reservation_default_params(void) {
  prom_dominatus_reservation_params p;
  p.capacity = 16u;
  p.max_lookahead_depth = 2u;
  p.min_confidence = 0.45;
  p.max_future_ticks = 2u;
  p.expiry_slack_ticks = 1u;
  return p;
}

void prom_dominatus_reservation_init(prom_dominatus_reservation_state_set* state,
                                     const prom_dominatus_reservation_params* params) {
  (void)params;
  prom_dominatus_reservation_reset(state);
}
void prom_dominatus_future_lease_seam_reset(prom_dominatus_future_lease_seam_state* state) {
  if (state == NULL) return;
  memset(state, 0, sizeof(*state));
}
prom_dominatus_future_lease_decision prom_dominatus_future_lease_request_issue(
    prom_dominatus_future_lease_seam_state* state,
    const prom_dominatus_prediction_entry* prediction,
    uint64_t tick) {
  prom_dominatus_future_lease_decision out;
  memset(&out, 0, sizeof(out));
  if (state == NULL || prediction == NULL || prediction->active == 0u || prediction->lookahead_depth == 0u) return out;
  state->last_request.valid = 1u;
  state->last_request.request_id = ++state->next_request_id;
  state->last_request.issued_tick = tick;
  state->last_request.target_tick = prediction->target_tick;
  state->last_request.slot_id = prediction->predicted_slot_id;
  state->last_request.shape_class = prediction->predicted_shape_class;
  state->last_request.variant_id = prediction->predicted_variant;
  state->last_request.lookahead_depth = prediction->lookahead_depth;
  state->last_request.confidence = prediction->prediction_confidence;
  state->last_request.state = PROM_DOM_FUTURE_LEASE_REQUESTED;
  state->requested_count += 1u;
  out.valid = 1u;
  out.request_id = state->last_request.request_id;
  out.previous_state = PROM_DOM_FUTURE_LEASE_NONE;
  out.new_state = PROM_DOM_FUTURE_LEASE_REQUESTED;
  out.requested = 1u;
  out.target_tick = state->last_request.target_tick;
  out.confidence = state->last_request.confidence;
  return out;
}
prom_dominatus_future_lease_decision prom_dominatus_future_lease_grant(prom_dominatus_future_lease_seam_state* state,
                                                                        uint64_t request_id,
                                                                        uint64_t tick) {
  (void)tick;
  return future_lease_transition(state, request_id, PROM_DOM_FUTURE_LEASE_GRANTED, 0u);
}
prom_dominatus_future_lease_decision prom_dominatus_future_lease_deny(prom_dominatus_future_lease_seam_state* state,
                                                                       uint64_t request_id,
                                                                       uint32_t reason,
                                                                       uint64_t tick) {
  (void)tick;
  return future_lease_transition(state, request_id, PROM_DOM_FUTURE_LEASE_DENIED, reason);
}
prom_dominatus_future_lease_decision prom_dominatus_future_lease_cancel(prom_dominatus_future_lease_seam_state* state,
                                                                         uint64_t request_id,
                                                                         uint32_t reason,
                                                                         uint64_t tick) {
  (void)tick;
  return future_lease_transition(state, request_id, PROM_DOM_FUTURE_LEASE_CANCELLED, reason);
}
prom_dominatus_future_lease_decision prom_dominatus_future_lease_mature(prom_dominatus_future_lease_seam_state* state,
                                                                         uint64_t request_id,
                                                                         uint64_t tick) {
  (void)tick;
  return future_lease_transition(state, request_id, PROM_DOM_FUTURE_LEASE_MATURED, 0u);
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
  prom_dominatus_future_lease_seam_init(&state->future_lease_seam);
  state->reservation_params = prom_dominatus_reservation_default_params();
  prom_dominatus_reservation_init(&state->reservations, &state->reservation_params);
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
  prom_dominatus_future_lease_seam_reset(&state->future_lease_seam);
  prom_dominatus_reservation_reset(&state->reservations);
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
      state->last_prediction_error <= 0 && state->predictor_stale == 0u) depth = 2u;
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
  state->fallback_active = 0u; state->fallback_reason = PROM_DOM_FALLBACK_NONE;
  depth = prom_dominatus_predictor_select_depth(state, evidence, physical);
  state->lookahead_depth = depth; state->last_tick = tick;
  if (has_hard_gate(physical) != 0u) { state->fallback_active = 1u; state->fallback_reason = PROM_DOM_FALLBACK_HARD_GATE; return 0u; }
  if (depth == 0u) return 0u;
  if (state->ring_count >= PROM_DOM_PREDICTION_RING_CAP) { state->predictor_stale = 1u; state->fallback_active = 1u; state->fallback_reason = PROM_DOM_FALLBACK_RING_FULL; state->stale_prediction_count += 1u; return 0u; }
  memset(&entry,0,sizeof(entry));
  entry.active=1u; entry.issued_tick=tick; entry.target_tick=tick+(uint64_t)depth; entry.predicted_ready=1u; entry.predicted_latency=evidence->filtered_value;
  entry.prediction_confidence=evidence->confidence < state->prediction_confidence ? evidence->confidence : state->prediction_confidence;
  entry.lookahead_depth=depth; entry.future_lease_state=PROM_DOM_FUTURE_LEASE_REQUESTED;
  idx=(state->ring_head + state->ring_count) % PROM_DOM_PREDICTION_RING_CAP; state->ring[idx]=entry; state->ring_count+=1u; state->future_lease_requested+=1u;
  { const prom_dominatus_future_lease_decision d=prom_dominatus_future_lease_request_issue(&state->future_lease_seam,&entry,tick); if (d.valid!=0u){state->ring[idx].lease_request_id=d.request_id; }}
  if (out_entry != NULL) { *out_entry = state->ring[idx]; }
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

  (void)prom_dominatus_predictor_apply_correction_to_reservation(state,
                                                                  &state->reservations,
                                                                  entry,
                                                                  &ev,
                                                                  tick);

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


void prom_dominatus_reservation_reset(prom_dominatus_reservation_state_set* state) {
  if (state == NULL) return;
  memset(state, 0, sizeof(*state));
}

static uint32_t reservation_is_active(prom_dominatus_reservation_state st) {
  return st == PROM_DOM_RESERVATION_RESERVED || st == PROM_DOM_RESERVATION_REQUESTED;
}

static prom_dominatus_reservation_decision reservation_transition(prom_dominatus_reservation_state_set* state,
                                                                  uint64_t idx,
                                                                  prom_dominatus_reservation_state next,
                                                                  uint32_t reason) {
  prom_dominatus_reservation_decision d;
  memset(&d, 0, sizeof(d));
  if (state == NULL || idx >= PROM_DOM_RESERVATION_CAP || state->entries[idx].valid == 0u) return d;
  d.valid = 1u;
  d.request_id = state->entries[idx].request_id;
  d.previous_state = state->entries[idx].state;
  d.new_state = next;
  d.reason = reason;
  d.target_tick = state->entries[idx].target_tick;
  d.confidence = state->entries[idx].confidence;
  if (reservation_is_active(state->entries[idx].state) && !reservation_is_active(next) && state->active_count > 0u) state->active_count -= 1u;
  state->entries[idx].state = next;
  if (next == PROM_DOM_RESERVATION_DENIED) { d.denied = 1u; state->denied_count += 1u; state->entries[idx].deny_reason = reason; }
  if (next == PROM_DOM_RESERVATION_CANCELLED) { d.cancelled = 1u; state->cancelled_count += 1u; state->entries[idx].cancel_reason = reason; }
  if (next == PROM_DOM_RESERVATION_MATURED) { d.matured = 1u; state->matured_count += 1u; }
  if (next == PROM_DOM_RESERVATION_EXPIRED) { d.expired = 1u; state->expired_count += 1u; }
  if (next == PROM_DOM_RESERVATION_YIELDED) { d.yielded = 1u; state->yielded_count += 1u; }
  d.active_count = state->active_count;
  return d;
}

static prom_dominatus_reservation_decision reservation_transition_by_request_id(
    prom_dominatus_reservation_state_set* state,
    uint64_t request_id,
    prom_dominatus_reservation_state next,
    uint32_t reason) {
  uint32_t i;
  prom_dominatus_reservation_decision d;
  memset(&d, 0, sizeof(d));
  if (state == NULL || request_id == 0u) return d;
  for (i = 0u; i < PROM_DOM_RESERVATION_CAP; ++i) {
    if (state->entries[i].valid != 0u && state->entries[i].request_id == request_id &&
        state->entries[i].state == PROM_DOM_RESERVATION_RESERVED) {
      return reservation_transition(state, i, next, reason);
    }
  }
  return d;
}

prom_dominatus_reservation_decision prom_dominatus_predictor_apply_correction_to_reservation(
    prom_dominatus_predictor_state* predictor,
    prom_dominatus_reservation_state_set* reservations,
    const prom_dominatus_prediction_entry* matured_entry,
    const prom_dominatus_correction_event* correction,
    uint64_t tick) {
  prom_dominatus_reservation_decision d;
  memset(&d, 0, sizeof(d));
  if (predictor == NULL || reservations == NULL || matured_entry == NULL || correction == NULL ||
      correction->valid == 0u || matured_entry->lease_request_id == 0u) return d;

  if (correction->action == PROM_DOM_CORRECTION_ACTION_MARK_STALE) {
    d = reservation_transition_by_request_id(reservations, matured_entry->lease_request_id, PROM_DOM_RESERVATION_CANCELLED, 90u);
    if (d.cancelled != 0u) {
      (void)prom_dominatus_future_lease_cancel(&predictor->future_lease_seam, matured_entry->lease_request_id, d.reason, tick);
      predictor->future_lease_cancelled += 1u;
    }
    return d;
  }
  if (correction->state_mismatch == 0u) {
    d = reservation_transition_by_request_id(reservations, matured_entry->lease_request_id, PROM_DOM_RESERVATION_MATURED, 0u);
    if (d.matured != 0u) (void)prom_dominatus_future_lease_mature(&predictor->future_lease_seam, matured_entry->lease_request_id, tick);
    return d;
  }
  if (tick >= matured_entry->target_tick) {
    d = reservation_transition_by_request_id(reservations, matured_entry->lease_request_id, PROM_DOM_RESERVATION_EXPIRED, 91u);
    if (d.expired != 0u) {
      (void)prom_dominatus_future_lease_cancel(&predictor->future_lease_seam, matured_entry->lease_request_id, d.reason, tick);
      predictor->future_lease_cancelled += 1u;
    }
    return d;
  }
  d = reservation_transition_by_request_id(reservations, matured_entry->lease_request_id, PROM_DOM_RESERVATION_CANCELLED, 92u);
  if (d.cancelled != 0u) {
    (void)prom_dominatus_future_lease_cancel(&predictor->future_lease_seam, matured_entry->lease_request_id, d.reason, tick);
    predictor->future_lease_cancelled += 1u;
  }
  return d;
}

prom_dominatus_reservation_decision prom_dominatus_reservation_request_from_future_lease(
    prom_dominatus_reservation_state_set* state,
    const prom_dominatus_reservation_params* params,
    const prom_dominatus_future_lease_request* request,
    uint64_t tick) {
  prom_dominatus_reservation_decision d;
  uint32_t i;
  uint32_t cap;
  memset(&d, 0, sizeof(d));
  if (state == NULL || params == NULL || request == NULL || request->valid == 0u) return d;
  d.valid = 1u; d.request_id = request->request_id; d.new_state = PROM_DOM_RESERVATION_DENIED; d.target_tick = request->target_tick; d.confidence = request->confidence;
  if (request->confidence < params->min_confidence || request->lookahead_depth == 0u || request->lookahead_depth > params->max_lookahead_depth || request->target_tick <= tick || (request->target_tick - tick) > params->max_future_ticks) {
    d.denied = 1u; d.reason = 1u; state->denied_count += 1u; d.active_count = state->active_count; return d;
  }
  cap = params->capacity; if (cap == 0u || cap > PROM_DOM_RESERVATION_CAP) cap = PROM_DOM_RESERVATION_CAP;
  if (state->active_count >= cap) { d.denied = 1u; d.reason = 2u; state->denied_count += 1u; d.active_count = state->active_count; return d; }
  for (i = 0u; i < cap; ++i) {
    if (state->entries[i].valid != 0u && state->entries[i].request_id == request->request_id && reservation_is_active(state->entries[i].state) != 0u) {
      d.denied = 1u; d.reason = 3u; state->denied_count += 1u; d.active_count = state->active_count; return d;
    }
  }
  for (i = 0u; i < cap; ++i) {
    if (state->entries[i].valid == 0u || reservation_is_active(state->entries[i].state) == 0u) {
      memset(&state->entries[i], 0, sizeof(state->entries[i]));
      state->entries[i].valid = 1u;
      state->entries[i].request_id = request->request_id != 0u ? request->request_id : ++state->next_request_id;
      state->entries[i].issued_tick = tick; state->entries[i].target_tick = request->target_tick;
      state->entries[i].resource_kind = request->resource_kind; state->entries[i].slot_id = request->slot_id;
      state->entries[i].shape_class = request->shape_class; state->entries[i].variant_id = request->variant_id;
      state->entries[i].lookahead_depth = request->lookahead_depth; state->entries[i].confidence = request->confidence;
      state->entries[i].state = PROM_DOM_RESERVATION_RESERVED;
      state->requested_count += 1u; state->reserved_count += 1u; state->active_count += 1u;
      d.request_id = state->entries[i].request_id; d.previous_state = PROM_DOM_RESERVATION_NONE; d.new_state = PROM_DOM_RESERVATION_RESERVED; d.reserved = 1u; d.active_count = state->active_count;
      return d;
    }
  }
  d.denied = 1u; d.reason = 4u; state->denied_count += 1u; d.active_count = state->active_count; return d;
}

prom_dominatus_reservation_decision prom_dominatus_reservation_cancel(prom_dominatus_reservation_state_set* state,
                                                                      uint64_t request_id,
                                                                      uint32_t reason,
                                                                      uint64_t tick) {
  uint32_t i; (void)tick;
  for (i = 0u; state != NULL && i < PROM_DOM_RESERVATION_CAP; ++i) if (state->entries[i].valid != 0u && state->entries[i].request_id == request_id && state->entries[i].state == PROM_DOM_RESERVATION_RESERVED) return reservation_transition(state, i, PROM_DOM_RESERVATION_CANCELLED, reason);
  { prom_dominatus_reservation_decision d; memset(&d,0,sizeof(d)); return d; }
}

prom_dominatus_reservation_decision prom_dominatus_reservation_mature(prom_dominatus_reservation_state_set* state,
                                                                      uint64_t tick) {
  uint32_t i; int found=-1; uint64_t best=0;
  for (i = 0u; state != NULL && i < PROM_DOM_RESERVATION_CAP; ++i) if (state->entries[i].valid != 0u && state->entries[i].state == PROM_DOM_RESERVATION_RESERVED && state->entries[i].target_tick <= tick) { if (found < 0 || state->entries[i].target_tick < best) { found=(int)i; best=state->entries[i].target_tick; } }
  if (found >= 0) return reservation_transition(state, (uint64_t)found, PROM_DOM_RESERVATION_MATURED, 0u);
  { prom_dominatus_reservation_decision d; memset(&d,0,sizeof(d)); return d; }
}

prom_dominatus_reservation_decision prom_dominatus_reservation_expire_stale(
    prom_dominatus_reservation_state_set* state,
    const prom_dominatus_reservation_params* params,
    uint64_t tick) {
  uint32_t i; uint64_t slack = params == NULL ? 0u : params->expiry_slack_ticks;
  for (i = 0u; state != NULL && i < PROM_DOM_RESERVATION_CAP; ++i) if (state->entries[i].valid != 0u && state->entries[i].state == PROM_DOM_RESERVATION_RESERVED && tick > state->entries[i].target_tick + slack) return reservation_transition(state, i, PROM_DOM_RESERVATION_EXPIRED, 0u);
  { prom_dominatus_reservation_decision d; memset(&d,0,sizeof(d)); return d; }
}

prom_dominatus_reservation_decision prom_dominatus_predictor_try_reserve_future(
    prom_dominatus_predictor_state* predictor,
    prom_dominatus_reservation_state_set* reservations,
    const prom_dominatus_future_lease_request* future_request,
    uint64_t tick) {
  prom_dominatus_reservation_decision d;
  if (predictor == NULL || reservations == NULL) { memset(&d,0,sizeof(d)); return d; }
  d = prom_dominatus_reservation_request_from_future_lease(reservations, &predictor->reservation_params, future_request, tick);
  if (future_request != NULL && future_request->valid != 0u) {
    if (d.reserved != 0u) (void)prom_dominatus_future_lease_grant(&predictor->future_lease_seam, future_request->request_id, tick);
    else if (d.denied != 0u) (void)prom_dominatus_future_lease_deny(&predictor->future_lease_seam, future_request->request_id, d.reason, tick);
  }
  return d;
}
