#include "reactor_dominatus_slot_adapter.h"

#include <string.h>

static uint32_t slot_mask_for_slot_id(const prom_dom_blackboard* board, uint32_t slot_id) {
  if (board == 0) {
    return 0u;
  }
  if (slot_id < 32u) {
    return 1u << slot_id;
  }
  return 0u;
}

static void apply_slot_readiness_event(prom_dom_blackboard* board, const prom_dom_event* event) {
  uint32_t slot_mask;
  if (board == 0 || event == 0) {
    return;
  }
  if (event->source != PROM_DOM_SOURCE_SLOT_HFSM || event->domain != PROM_DOM_DOMAIN_SLOT) {
    return;
  }

  slot_mask = slot_mask_for_slot_id(board, event->slot_id);
  if (slot_mask == 0u) {
    board->slot_readiness_overflow_spill_count += 1u;
    return;
  }

  board->slot_readiness_dirty_slot_mask |= slot_mask;

  if (event->kind == PROM_DOM_EVENT_SLOT_READY) {
    if ((board->slot_readiness_ready_slot_mask & slot_mask) != 0u) {
      board->slot_readiness_duplicate_ready_event_count += 1u;
    }
    board->slot_readiness_ready_slot_mask |= slot_mask;
    board->slot_readiness_attention_slot_mask |= slot_mask;
    return;
  }

  if (event->kind == PROM_DOM_EVENT_SLOT_SUBMITTED || event->kind == PROM_DOM_EVENT_SLOT_PROMOTED_CURRENT ||
      event->kind == PROM_DOM_EVENT_SLOT_COMPLETE || event->kind == PROM_DOM_EVENT_SLOT_CONSUMED ||
      event->kind == PROM_DOM_EVENT_SLOT_PREPARED) {
    board->slot_readiness_ready_slot_mask &= ~slot_mask;
    board->slot_readiness_attention_slot_mask =
        (board->slot_readiness_ready_slot_mask | board->slot_readiness_failed_slot_mask | board->slot_readiness_invalidated_slot_mask);
    return;
  }

  if (event->kind == PROM_DOM_EVENT_SLOT_FAILED) {
    board->slot_readiness_ready_slot_mask &= ~slot_mask;
    board->slot_readiness_failed_slot_mask |= slot_mask;
    board->slot_readiness_attention_slot_mask |= slot_mask;
    return;
  }

  if (event->kind == PROM_DOM_EVENT_SLOT_INVALIDATED) {
    board->slot_readiness_ready_slot_mask &= ~slot_mask;
    board->slot_readiness_invalidated_slot_mask |= slot_mask;
    board->slot_readiness_attention_slot_mask |= slot_mask;
    return;
  }

  if (event->kind == PROM_DOM_EVENT_SLOT_CLEANUP) {
    board->slot_readiness_ready_slot_mask &= ~slot_mask;
    board->slot_readiness_failed_slot_mask &= ~slot_mask;
    board->slot_readiness_invalidated_slot_mask &= ~slot_mask;
    board->slot_readiness_attention_slot_mask =
        (board->slot_readiness_ready_slot_mask | board->slot_readiness_failed_slot_mask | board->slot_readiness_invalidated_slot_mask);
  }
}

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
  uint32_t prior_committed_event_count;
  uint32_t i;
  prom_dom_event committed_event;

  if (board == 0) {
    return;
  }

  prior_committed_event_count = prom_dom_committed_event_count(board);
  prom_dom_commit(board);

  for (i = prior_committed_event_count; i < prom_dom_committed_event_count(board); ++i) {
    if (prom_dom_committed_event_at(board, i, &committed_event) == 0u) {
      continue;
    }
    apply_slot_readiness_event(board, &committed_event);
  }

  if (prior_committed_event_count == prom_dom_committed_event_count(board)) {
    board->slot_readiness_empty_boundary_commit_count += 1u;
  }
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

uint32_t prom_dom_slot_stage_runtime_diag(prom_dom_blackboard* board,
                                          const prom_dom_slot_runtime_diag_snapshot* snapshot,
                                          int32_t reason_code) {
  if (board == 0 || snapshot == 0) {
    return 0u;
  }

  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_SLOT_HFSM, PROM_DOM_KEY_SLOT_CURRENT_ID, 0u, snapshot->current_slot_id, reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_SLOT_HFSM, PROM_DOM_KEY_SLOT_NEXT_ID, 0u, snapshot->next_slot_id, reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_SLOT_HFSM, PROM_DOM_KEY_SLOT_STATE, 0u, snapshot->slot_state[0], reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u32(board, PROM_DOM_SOURCE_SLOT_HFSM, PROM_DOM_KEY_SLOT_STATE, 1u, snapshot->slot_state[1], reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u64(board, PROM_DOM_SOURCE_SLOT_HFSM, PROM_DOM_KEY_SLOT_GENERATION, 0u, snapshot->slot_generation[0], reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u64(board, PROM_DOM_SOURCE_SLOT_HFSM, PROM_DOM_KEY_SLOT_GENERATION, 1u, snapshot->slot_generation[1], reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_bool(board, PROM_DOM_SOURCE_SLOT_HFSM, PROM_DOM_KEY_SLOT_VALID, 0u, snapshot->slot_valid[0], reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_bool(board, PROM_DOM_SOURCE_SLOT_HFSM, PROM_DOM_KEY_SLOT_VALID, 1u, snapshot->slot_valid[1], reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u64(board, PROM_DOM_SOURCE_SLOT_HFSM, PROM_DOM_KEY_SLOT_SWAP_COUNT, 0u, snapshot->swap_count, reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u64(board, PROM_DOM_SOURCE_SLOT_HFSM, PROM_DOM_KEY_SLOT_MAX_WIP_DEPTH, 0u, snapshot->max_wip_depth, reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u64(board,
                       PROM_DOM_SOURCE_SLOT_HFSM,
                       PROM_DOM_KEY_SLOT_OVERWRITE_REJECTION_COUNT,
                       0u,
                       snapshot->overwrite_rejection_count,
                       reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u64(board,
                       PROM_DOM_SOURCE_SLOT_HFSM,
                       PROM_DOM_KEY_SLOT_STALE_BUFFER_REJECTION_COUNT,
                       0u,
                       snapshot->stale_buffer_rejection_count,
                       reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u64(board,
                       PROM_DOM_SOURCE_SLOT_HFSM,
                       PROM_DOM_KEY_SLOT_SHAPE_INVALIDATION_COUNT,
                       0u,
                       snapshot->shape_invalidation_count,
                       reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u64(board,
                       PROM_DOM_SOURCE_SLOT_HFSM,
                       PROM_DOM_KEY_SLOT_LAYOUT_INVALIDATION_COUNT,
                       0u,
                       snapshot->layout_invalidation_count,
                       reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u64(board,
                       PROM_DOM_SOURCE_SLOT_HFSM,
                       PROM_DOM_KEY_SLOT_CAPACITY_INVALIDATION_COUNT,
                       0u,
                       snapshot->capacity_invalidation_count,
                       reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u64(board,
                       PROM_DOM_SOURCE_SLOT_HFSM,
                       PROM_DOM_KEY_SLOT_INFLIGHT_REJECTION_COUNT,
                       0u,
                       snapshot->inflight_rejection_count,
                       reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_u64(board,
                       PROM_DOM_SOURCE_SLOT_HFSM,
                       PROM_DOM_KEY_SLOT_CLEANUP_SUCCESS_COUNT,
                       0u,
                       snapshot->cleanup_success_count,
                       reason_code) == 0u) {
    return 0u;
  }
  if (prom_dom_set_i32(board, PROM_DOM_SOURCE_SLOT_HFSM, PROM_DOM_KEY_SLOT_FAILURE_SLOT_ID, 0u, snapshot->failure_slot_id, reason_code) == 0u) {
    return 0u;
  }
  return prom_dom_set_i32(board, PROM_DOM_SOURCE_SLOT_HFSM, PROM_DOM_KEY_SLOT_FAILURE_REASON_GLOBAL, 0u, snapshot->failure_reason, reason_code);
}

uint32_t prom_dom_slot_read_visible_runtime_diag(const prom_dom_blackboard* board,
                                                 prom_dom_slot_runtime_diag_snapshot* out_snapshot) {
  if (board == 0 || out_snapshot == 0) {
    return 0u;
  }

  memset(out_snapshot, 0, sizeof(*out_snapshot));
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SLOT_CURRENT_ID, 0u, &out_snapshot->current_slot_id) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SLOT_NEXT_ID, 0u, &out_snapshot->next_slot_id) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SLOT_STATE, 0u, &out_snapshot->slot_state[0]) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u32(board, PROM_DOM_KEY_SLOT_STATE, 1u, &out_snapshot->slot_state[1]) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SLOT_GENERATION, 0u, &out_snapshot->slot_generation[0]) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SLOT_GENERATION, 1u, &out_snapshot->slot_generation[1]) == 0u) {
    return 0u;
  }
  if (prom_dom_get_bool(board, PROM_DOM_KEY_SLOT_VALID, 0u, &out_snapshot->slot_valid[0]) == 0u) {
    return 0u;
  }
  if (prom_dom_get_bool(board, PROM_DOM_KEY_SLOT_VALID, 1u, &out_snapshot->slot_valid[1]) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SLOT_SWAP_COUNT, 0u, &out_snapshot->swap_count) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SLOT_MAX_WIP_DEPTH, 0u, &out_snapshot->max_wip_depth) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SLOT_OVERWRITE_REJECTION_COUNT, 0u, &out_snapshot->overwrite_rejection_count) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SLOT_STALE_BUFFER_REJECTION_COUNT, 0u, &out_snapshot->stale_buffer_rejection_count) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SLOT_SHAPE_INVALIDATION_COUNT, 0u, &out_snapshot->shape_invalidation_count) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SLOT_LAYOUT_INVALIDATION_COUNT, 0u, &out_snapshot->layout_invalidation_count) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SLOT_CAPACITY_INVALIDATION_COUNT, 0u, &out_snapshot->capacity_invalidation_count) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SLOT_INFLIGHT_REJECTION_COUNT, 0u, &out_snapshot->inflight_rejection_count) == 0u) {
    return 0u;
  }
  if (prom_dom_get_u64(board, PROM_DOM_KEY_SLOT_CLEANUP_SUCCESS_COUNT, 0u, &out_snapshot->cleanup_success_count) == 0u) {
    return 0u;
  }
  if (prom_dom_get_i32(board, PROM_DOM_KEY_SLOT_FAILURE_SLOT_ID, 0u, &out_snapshot->failure_slot_id) == 0u) {
    return 0u;
  }
  return prom_dom_get_i32(board, PROM_DOM_KEY_SLOT_FAILURE_REASON_GLOBAL, 0u, &out_snapshot->failure_reason);
}

uint32_t prom_dom_slot_readiness_read_visible(const prom_dom_blackboard* board,
                                              prom_dom_slot_readiness_snapshot* out_snapshot) {
  if (board == 0 || out_snapshot == 0) {
    return 0u;
  }

  memset(out_snapshot, 0, sizeof(*out_snapshot));
  out_snapshot->boundary_generation = board->slot_readiness_boundary_generation;
  out_snapshot->dirty_slot_mask = board->slot_readiness_dirty_slot_mask;
  out_snapshot->ready_slot_mask = board->slot_readiness_ready_slot_mask;
  out_snapshot->failed_slot_mask = board->slot_readiness_failed_slot_mask;
  out_snapshot->invalidated_slot_mask = board->slot_readiness_invalidated_slot_mask;
  out_snapshot->attention_slot_mask = board->slot_readiness_attention_slot_mask;
  out_snapshot->overflow_spill_count = board->slot_readiness_overflow_spill_count;
  out_snapshot->duplicate_ready_event_count = board->slot_readiness_duplicate_ready_event_count;
  out_snapshot->empty_boundary_commit_count = board->slot_readiness_empty_boundary_commit_count;
  return 1u;
}

void prom_dom_slot_readiness_clear_boundary(prom_dom_blackboard* board) {
  if (board == 0) {
    return;
  }

  board->slot_readiness_boundary_generation += 1u;
  board->slot_readiness_dirty_slot_mask = 0u;
  board->slot_readiness_ready_slot_mask = 0u;
  board->slot_readiness_failed_slot_mask = 0u;
  board->slot_readiness_invalidated_slot_mask = 0u;
  board->slot_readiness_attention_slot_mask = 0u;
}
