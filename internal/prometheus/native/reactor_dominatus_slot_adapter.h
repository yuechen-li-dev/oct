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

#ifdef __cplusplus
}
#endif

#endif
