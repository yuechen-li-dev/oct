#include "reactor_slot_hfsm.h"

#include <string.h>

static uint32_t state_is_valid(prom_slot_state state) {
  return state >= PROM_SLOT_EMPTY && state <= PROM_SLOT_CLEANUP;
}

static void set_invalid_transition(prom_slot_hfsm* machine, prom_slot_state from, prom_slot_state to) {
  machine->diagnostics.invalid_transition_count += 1u;
  machine->diagnostics.last_invalid_from = from;
  machine->diagnostics.last_invalid_to = to;
}

static void refresh_current_state(prom_slot_hfsm* machine) {
  if (machine->depth == 0u) {
    machine->diagnostics.current_state = PROM_SLOT_EMPTY;
    return;
  }

  machine->diagnostics.current_state = machine->stack[machine->depth - 1u];
}

void prom_slot_hfsm_init(prom_slot_hfsm* machine, uint32_t slot_id) {
  if (machine == 0) {
    return;
  }

  memset(machine, 0, sizeof(*machine));
  machine->depth = 1u;
  machine->stack[0] = PROM_SLOT_EMPTY;
  machine->diagnostics.current_state = PROM_SLOT_EMPTY;
  machine->diagnostics.max_stack_depth_reached = 1u;
  machine->metadata.slot_id = slot_id;
}

void prom_slot_hfsm_reset(prom_slot_hfsm* machine) {
  uint32_t slot_id;
  if (machine == 0) {
    return;
  }

  slot_id = machine->metadata.slot_id;
  prom_slot_hfsm_init(machine, slot_id);
}

prom_slot_state prom_slot_hfsm_current_state(const prom_slot_hfsm* machine) {
  if (machine == 0 || machine->depth == 0u) {
    return PROM_SLOT_EMPTY;
  }

  return machine->stack[machine->depth - 1u];
}

uint32_t prom_slot_hfsm_depth(const prom_slot_hfsm* machine) {
  if (machine == 0) {
    return 0u;
  }

  return machine->depth;
}

uint32_t prom_slot_hfsm_contains(const prom_slot_hfsm* machine, prom_slot_state state) {
  uint32_t i;
  if (machine == 0 || state_is_valid(state) == 0u) {
    return 0u;
  }

  for (i = 0u; i < machine->depth; ++i) {
    if (machine->stack[i] == state) {
      return 1u;
    }
  }

  return 0u;
}

uint32_t prom_slot_hfsm_push_state(prom_slot_hfsm* machine, prom_slot_state state) {
  if (machine == 0 || state_is_valid(state) == 0u) {
    return 0u;
  }
  if (machine->depth >= PROM_SLOT_HFSM_MAX_DEPTH) {
    return 0u;
  }

  machine->diagnostics.previous_state = prom_slot_hfsm_current_state(machine);
  machine->stack[machine->depth] = state;
  machine->depth += 1u;
  refresh_current_state(machine);

  if (machine->depth > machine->diagnostics.max_stack_depth_reached) {
    machine->diagnostics.max_stack_depth_reached = machine->depth;
  }

  return 1u;
}

uint32_t prom_slot_hfsm_pop_state(prom_slot_hfsm* machine) {
  if (machine == 0 || machine->depth <= 1u) {
    return 0u;
  }

  machine->diagnostics.previous_state = prom_slot_hfsm_current_state(machine);
  machine->depth -= 1u;
  refresh_current_state(machine);
  return 1u;
}

uint32_t prom_slot_hfsm_replace_state(prom_slot_hfsm* machine, prom_slot_state state) {
  if (machine == 0 || state_is_valid(state) == 0u || machine->depth == 0u) {
    return 0u;
  }

  machine->diagnostics.previous_state = prom_slot_hfsm_current_state(machine);
  machine->stack[machine->depth - 1u] = state;
  refresh_current_state(machine);
  return 1u;
}

uint32_t prom_slot_hfsm_can_transition(prom_slot_state from, prom_slot_state to) {
  if (state_is_valid(from) == 0u || state_is_valid(to) == 0u) {
    return 0u;
  }

  if (to == PROM_SLOT_FAILED && from != PROM_SLOT_FAILED) {
    return 1u;
  }

  if (from == PROM_SLOT_EMPTY && to == PROM_SLOT_PREPARING) {
    return 1u;
  }
  if (from == PROM_SLOT_PREPARING && to == PROM_SLOT_READY) {
    return 1u;
  }
  if (from == PROM_SLOT_READY && to == PROM_SLOT_CURRENT) {
    return 1u;
  }
  if (from == PROM_SLOT_CURRENT && to == PROM_SLOT_IN_FLIGHT) {
    return 1u;
  }
  if (from == PROM_SLOT_IN_FLIGHT && to == PROM_SLOT_CONSUMED) {
    return 1u;
  }
  if (from == PROM_SLOT_CONSUMED && to == PROM_SLOT_EMPTY) {
    return 1u;
  }
  if (from == PROM_SLOT_FAILED && to == PROM_SLOT_CLEANUP) {
    return 1u;
  }
  if (from == PROM_SLOT_CLEANUP && to == PROM_SLOT_EMPTY) {
    return 1u;
  }

  return 0u;
}

uint32_t prom_slot_hfsm_transition(prom_slot_hfsm* machine, prom_slot_state to) {
  prom_slot_state from;
  if (machine == 0) {
    return 0u;
  }

  from = prom_slot_hfsm_current_state(machine);
  if (prom_slot_hfsm_can_transition(from, to) == 0u) {
    set_invalid_transition(machine, from, to);
    return 0u;
  }

  machine->diagnostics.transition_count += 1u;
  if (to == PROM_SLOT_FAILED) {
    machine->diagnostics.failure_count += 1u;
  }
  if (to == PROM_SLOT_CLEANUP) {
    machine->diagnostics.cleanup_count += 1u;
  }
  return prom_slot_hfsm_replace_state(machine, to);
}

uint32_t prom_slot_hfsm_fail(prom_slot_hfsm* machine, int failure_reason) {
  if (machine == 0) {
    return 0u;
  }

  machine->metadata.failure_reason = failure_reason;
  machine->metadata.valid = 0u;
  return prom_slot_hfsm_transition(machine, PROM_SLOT_FAILED);
}

uint32_t prom_slot_hfsm_cleanup(prom_slot_hfsm* machine) {
  if (machine == 0) {
    return 0u;
  }

  if (prom_slot_hfsm_transition(machine, PROM_SLOT_CLEANUP) == 0u) {
    return 0u;
  }
  if (prom_slot_hfsm_transition(machine, PROM_SLOT_EMPTY) == 0u) {
    return 0u;
  }

  machine->metadata.valid = 0u;
  machine->metadata.required_capacity_bytes = 0u;
  machine->metadata.shape.m = 0u;
  machine->metadata.shape.n = 0u;
  machine->metadata.shape.k = 0u;
  machine->metadata.layout.layout = 0u;
  machine->metadata.layout.precision = 0u;
  machine->metadata.failure_reason = 0;
  machine->metadata.generation += 1u;

  return 1u;
}

void prom_slot_hfsm_set_metadata(prom_slot_hfsm* machine, const prom_slot_metadata* metadata) {
  if (machine == 0 || metadata == 0) {
    return;
  }

  machine->metadata = *metadata;
}

void prom_slot_hfsm_mark_invalidated(prom_slot_hfsm* machine, int failure_reason) {
  if (machine == 0) {
    return;
  }

  machine->metadata.valid = 0u;
  machine->metadata.failure_reason = failure_reason;
}

const prom_slot_metadata* prom_slot_hfsm_metadata(const prom_slot_hfsm* machine) {
  if (machine == 0) {
    return 0;
  }

  return &machine->metadata;
}

const prom_slot_hfsm_diagnostics* prom_slot_hfsm_get_diagnostics(const prom_slot_hfsm* machine) {
  if (machine == 0) {
    return 0;
  }

  return &machine->diagnostics;
}
