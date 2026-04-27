#include "reactor_dominatus_slot_adapter.h"

#include <string.h>

uint32_t prom_dom_slot_stage_lifecycle(prom_dom_blackboard* board,
                                       prom_dom_event_kind event_kind,
                                       uint32_t slot_id,
                                       prom_slot_state slot_state,
                                       const prom_slot_metadata* metadata,
                                       uint32_t has_current_slot,
                                       uint32_t current_slot_id,
                                       uint32_t has_next_slot,
                                       uint32_t next_slot_id,
                                       int32_t reason_code) {
  prom_dom_event event;
  const prom_slot_metadata* resolved_metadata;

  if (board == 0 || metadata == 0) {
    return 0u;
  }

  resolved_metadata = metadata;

  if (prom_dom_set_u32(board,
                       PROM_DOM_SOURCE_SLOT_HFSM,
                       PROM_DOM_KEY_SLOT_STATE,
                       slot_id,
                       (uint32_t)slot_state,
                       reason_code) == 0u) {
    return 0u;
  }

  if (prom_dom_set_u64(board,
                       PROM_DOM_SOURCE_SLOT_HFSM,
                       PROM_DOM_KEY_SLOT_GENERATION,
                       slot_id,
                       resolved_metadata->generation,
                       reason_code) == 0u) {
    return 0u;
  }

  if (prom_dom_set_bool(board,
                        PROM_DOM_SOURCE_SLOT_HFSM,
                        PROM_DOM_KEY_SLOT_VALID,
                        slot_id,
                        resolved_metadata->valid,
                        reason_code) == 0u) {
    return 0u;
  }

  if (prom_dom_set_i32(board,
                       PROM_DOM_SOURCE_SLOT_HFSM,
                       PROM_DOM_KEY_SLOT_FAILURE_REASON,
                       slot_id,
                       resolved_metadata->failure_reason,
                       reason_code) == 0u) {
    return 0u;
  }

  if (has_current_slot != 0u) {
    if (prom_dom_set_u32(board,
                         PROM_DOM_SOURCE_SLOT_HFSM,
                         PROM_DOM_KEY_SLOT_CURRENT_ID,
                         0u,
                         current_slot_id,
                         reason_code) == 0u) {
      return 0u;
    }
  }

  if (has_next_slot != 0u) {
    if (prom_dom_set_u32(board,
                         PROM_DOM_SOURCE_SLOT_HFSM,
                         PROM_DOM_KEY_SLOT_NEXT_ID,
                         0u,
                         next_slot_id,
                         reason_code) == 0u) {
      return 0u;
    }
  }

  memset(&event, 0, sizeof(event));
  event.kind = event_kind;
  event.source = PROM_DOM_SOURCE_SLOT_HFSM;
  event.domain = PROM_DOM_DOMAIN_SLOT;
  event.key = PROM_DOM_KEY_SLOT_STATE;
  event.slot_id = slot_id;
  event.reason_code = reason_code;

  return prom_dom_stage_event(board, &event);
}

void prom_dom_slot_commit(prom_dom_blackboard* board) {
  if (board == 0) {
    return;
  }

  prom_dom_commit(board);
}

uint32_t prom_dom_slot_read_last_commit(const prom_dom_blackboard* board,
                                        uint32_t slot_id,
                                        prom_dom_slot_commit_snapshot* out_snapshot) {
  uint32_t committed_event_count;
  uint32_t i;
  prom_dom_event candidate;

  if (board == 0 || out_snapshot == 0) {
    return 0u;
  }

  memset(out_snapshot, 0, sizeof(*out_snapshot));
  out_snapshot->current_slot_id = UINT32_MAX;
  out_snapshot->next_slot_id = UINT32_MAX;
  committed_event_count = prom_dom_committed_event_count(board);
  out_snapshot->committed_event_count = committed_event_count;
  out_snapshot->last_commit_dirty_slot_mask = prom_dom_dirty_slots_last_commit(board);

  if (prom_dom_get_u32(board, PROM_DOM_KEY_SLOT_STATE, slot_id, &out_snapshot->slot_state) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SLOT_GENERATION, slot_id, &out_snapshot->slot_generation) == 0u) {
    return 0u;
  }
  if (prom_dom_get_bool(board, PROM_DOM_KEY_SLOT_VALID, slot_id, &out_snapshot->slot_valid) == 0u) {
    return 0u;
  }
  if (prom_dom_get_i32(board, PROM_DOM_KEY_SLOT_FAILURE_REASON, slot_id, &out_snapshot->slot_failure_reason) == 0u) {
    return 0u;
  }
  (void)prom_dom_get_u32(board, PROM_DOM_KEY_SLOT_CURRENT_ID, 0u, &out_snapshot->current_slot_id);
  (void)prom_dom_get_u32(board, PROM_DOM_KEY_SLOT_NEXT_ID, 0u, &out_snapshot->next_slot_id);

  if (committed_event_count == 0u) {
    memset(&out_snapshot->last_event, 0, sizeof(out_snapshot->last_event));
    return 1u;
  }

  for (i = committed_event_count; i > 0u; --i) {
    if (prom_dom_committed_event_at(board, i - 1u, &candidate) == 0u) {
      return 0u;
    }
    if (candidate.source == PROM_DOM_SOURCE_SLOT_HFSM && candidate.domain == PROM_DOM_DOMAIN_SLOT) {
      out_snapshot->last_event = candidate;
      if (candidate.slot_id < 32u) {
        out_snapshot->last_commit_dirty_slot_mask = (1u << candidate.slot_id);
      } else {
        out_snapshot->last_commit_dirty_slot_mask = 0u;
      }
      return 1u;
    }
  }

  return prom_dom_committed_event_at(board, committed_event_count - 1u, &out_snapshot->last_event);
}
