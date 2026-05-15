#include "reactor_dominatus_prestage.h"

#include <string.h>

prom_dominatus_prestage_params prom_dominatus_prestage_default_params(void) {
  prom_dominatus_prestage_params out;
  out.action_enabled = 0u;
  out.confidence_threshold = 0.75;
  out.recent_miss_window = 5u;
  out.max_lead_ticks = 2u;
  out.cost_estimate_low = 0.10;
  out.cost_estimate_medium = 0.25;
  return out;
}

prom_dominatus_prestage_decision prom_dominatus_prestage_evaluate(
    const prom_dominatus_prestage_params* params,
    const prom_dominatus_prestage_input* input) {
  prom_dominatus_prestage_decision out;
  prom_dominatus_prestage_params p;
  uint64_t lead = 0u;
  memset(&out, 0, sizeof(out));
  p = params == NULL ? prom_dominatus_prestage_default_params() : *params;
  if (input == NULL || input->valid == 0u) {
    out.valid = 0u;
    out.state = PROM_DOM_PRESTAGE_BLOCKED;
    out.block_reasons = PROM_DOM_PRESTAGE_BLOCK_INVALID;
    return out;
  }

  out.valid = 1u;
  out.state = PROM_DOM_PRESTAGE_BLOCKED;
  out.request_id = input->request_id;
  out.target_tick = input->target_tick;
  out.confidence = input->confidence;
  out.cost_estimate = p.cost_estimate_low;

  if (input->target_tick > input->current_tick) {
    lead = input->target_tick - input->current_tick;
    out.lead_ticks = (uint32_t)lead;
    out.benefit_estimate = input->confidence;
  }

  if (input->confidence < p.confidence_threshold) out.block_reasons |= PROM_DOM_PRESTAGE_BLOCK_CONFIDENCE;
  if (input->warmup != 0u) out.block_reasons |= PROM_DOM_PRESTAGE_BLOCK_WARMUP;
  if (input->reservation_is_reserved == 0u) out.block_reasons |= PROM_DOM_PRESTAGE_BLOCK_RESERVATION;
  if (p.recent_miss_window != 0u && input->recent_miss_count > 0u) out.block_reasons |= PROM_DOM_PRESTAGE_BLOCK_RECENT_MISS;
  if (input->runtime_unsafe != 0u || input->slot_valid == 0u || input->memory_budget_ok == 0u) out.block_reasons |= PROM_DOM_PRESTAGE_BLOCK_HARD_GATE;
  if ((input->outstanding_depth_cap != 0u && input->outstanding_depth >= input->outstanding_depth_cap) ||
      input->resource_pressure_low == 0u) out.block_reasons |= PROM_DOM_PRESTAGE_BLOCK_RESOURCE_PRESSURE;
  if (input->target_tick <= input->current_tick || lead > p.max_lead_ticks) out.block_reasons |= PROM_DOM_PRESTAGE_BLOCK_LEAD_TIME;

  if (out.block_reasons == 0u) {
    out.allowed = 1u;
    out.state = PROM_DOM_PRESTAGE_ELIGIBLE;
    if (p.action_enabled == 0u) {
      out.block_reasons |= PROM_DOM_PRESTAGE_BLOCK_FEATURE_DISABLED;
      out.cost_estimate = p.cost_estimate_low;
      return out;
    }
    out.submitted = 1u;
    out.state = PROM_DOM_PRESTAGE_SUBMITTED;
    out.cost_estimate = p.cost_estimate_medium;
  }

  return out;
}
