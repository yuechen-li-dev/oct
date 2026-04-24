#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_SLOT_HFSM_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_SLOT_HFSM_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

enum {
  PROM_SLOT_HFSM_MAX_DEPTH = 8u,
};

typedef enum prom_slot_state {
  PROM_SLOT_EMPTY = 1,
  PROM_SLOT_PREPARING = 2,
  PROM_SLOT_READY = 3,
  PROM_SLOT_CURRENT = 4,
  PROM_SLOT_IN_FLIGHT = 5,
  PROM_SLOT_CONSUMED = 6,
  PROM_SLOT_FAILED = 7,
  PROM_SLOT_CLEANUP = 8,
} prom_slot_state;

typedef struct prom_slot_shape_metadata {
  uint32_t m;
  uint32_t n;
  uint32_t k;
} prom_slot_shape_metadata;

typedef struct prom_slot_layout_metadata {
  uint32_t layout;
  uint32_t precision;
} prom_slot_layout_metadata;

typedef struct prom_slot_metadata {
  uint32_t slot_id;
  uint64_t generation;
  uint32_t valid;
  prom_slot_shape_metadata shape;
  prom_slot_layout_metadata layout;
  uint64_t required_capacity_bytes;
  int failure_reason;
} prom_slot_metadata;

typedef struct prom_slot_hfsm_diagnostics {
  prom_slot_state current_state;
  prom_slot_state previous_state;
  uint32_t transition_count;
  uint32_t invalid_transition_count;
  uint32_t max_stack_depth_reached;
  prom_slot_state last_invalid_from;
  prom_slot_state last_invalid_to;
  uint32_t failure_count;
  uint32_t cleanup_count;
} prom_slot_hfsm_diagnostics;

typedef struct prom_slot_hfsm {
  prom_slot_state stack[PROM_SLOT_HFSM_MAX_DEPTH];
  uint32_t depth;
  prom_slot_hfsm_diagnostics diagnostics;
  prom_slot_metadata metadata;
} prom_slot_hfsm;

void prom_slot_hfsm_init(prom_slot_hfsm* machine, uint32_t slot_id);
void prom_slot_hfsm_reset(prom_slot_hfsm* machine);

prom_slot_state prom_slot_hfsm_current_state(const prom_slot_hfsm* machine);
uint32_t prom_slot_hfsm_depth(const prom_slot_hfsm* machine);
uint32_t prom_slot_hfsm_contains(const prom_slot_hfsm* machine, prom_slot_state state);

uint32_t prom_slot_hfsm_push_state(prom_slot_hfsm* machine, prom_slot_state state);
uint32_t prom_slot_hfsm_pop_state(prom_slot_hfsm* machine);
uint32_t prom_slot_hfsm_replace_state(prom_slot_hfsm* machine, prom_slot_state state);

uint32_t prom_slot_hfsm_can_transition(prom_slot_state from, prom_slot_state to);
uint32_t prom_slot_hfsm_transition(prom_slot_hfsm* machine, prom_slot_state to);
uint32_t prom_slot_hfsm_fail(prom_slot_hfsm* machine, int failure_reason);
uint32_t prom_slot_hfsm_cleanup(prom_slot_hfsm* machine);

void prom_slot_hfsm_set_metadata(prom_slot_hfsm* machine, const prom_slot_metadata* metadata);
void prom_slot_hfsm_mark_invalidated(prom_slot_hfsm* machine, int failure_reason);
const prom_slot_metadata* prom_slot_hfsm_metadata(const prom_slot_hfsm* machine);
const prom_slot_hfsm_diagnostics* prom_slot_hfsm_get_diagnostics(const prom_slot_hfsm* machine);

#ifdef __cplusplus
}
#endif

#endif
