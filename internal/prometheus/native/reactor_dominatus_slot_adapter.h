#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_SLOT_ADAPTER_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_SLOT_ADAPTER_H

#include <stdint.h>

#include "reactor_dominatus_blackboard.h"
#include "reactor_slot_hfsm.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef struct prom_dom_slot_commit_snapshot {
  uint32_t committed_event_count;
  prom_dom_event last_event;
  uint32_t last_commit_dirty_slot_mask;
  uint32_t slot_state;
  uint64_t slot_generation;
  uint32_t slot_valid;
  int32_t slot_failure_reason;
  uint32_t current_slot_id;
  uint32_t next_slot_id;
} prom_dom_slot_commit_snapshot;

typedef struct prom_dom_slot_runtime_diag_snapshot {
  uint32_t current_slot_id;
  uint32_t next_slot_id;
  uint32_t slot_state[2];
  uint64_t slot_generation[2];
  uint32_t slot_valid[2];
  uint64_t swap_count;
  uint64_t max_wip_depth;
  uint64_t overwrite_rejection_count;
  uint64_t stale_buffer_rejection_count;
  uint64_t shape_invalidation_count;
  uint64_t layout_invalidation_count;
  uint64_t capacity_invalidation_count;
  uint64_t inflight_rejection_count;
  uint64_t cleanup_success_count;
  int32_t failure_slot_id;
  int32_t failure_reason;
} prom_dom_slot_runtime_diag_snapshot;

typedef struct prom_dom_slot_readiness_snapshot {
  uint64_t boundary_generation;
  uint32_t dirty_slot_mask;
  uint32_t ready_slot_mask;
  uint32_t failed_slot_mask;
  uint32_t invalidated_slot_mask;
  uint32_t attention_slot_mask;
  uint32_t overflow_spill_count;
  uint64_t duplicate_ready_event_count;
  uint64_t empty_boundary_commit_count;
} prom_dom_slot_readiness_snapshot;

uint32_t prom_dom_slot_stage_lifecycle(prom_dom_blackboard* board,
                                       prom_dom_event_kind event_kind,
                                       uint32_t slot_id,
                                       prom_slot_state slot_state,
                                       const prom_slot_metadata* metadata,
                                       uint32_t has_current_slot,
                                       uint32_t current_slot_id,
                                       uint32_t has_next_slot,
                                       uint32_t next_slot_id,
                                       int32_t reason_code);

void prom_dom_slot_commit(prom_dom_blackboard* board);

uint32_t prom_dom_slot_read_last_commit(const prom_dom_blackboard* board,
                                        uint32_t slot_id,
                                        prom_dom_slot_commit_snapshot* out_snapshot);
uint32_t prom_dom_slot_stage_runtime_diag(prom_dom_blackboard* board,
                                          const prom_dom_slot_runtime_diag_snapshot* snapshot,
                                          int32_t reason_code);
uint32_t prom_dom_slot_read_visible_runtime_diag(const prom_dom_blackboard* board,
                                                 prom_dom_slot_runtime_diag_snapshot* out_snapshot);
uint32_t prom_dom_slot_readiness_read_visible(const prom_dom_blackboard* board,
                                              prom_dom_slot_readiness_snapshot* out_snapshot);
void prom_dom_slot_readiness_clear_boundary(prom_dom_blackboard* board);

#ifdef __cplusplus
}
#endif

#endif
