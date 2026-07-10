#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_SGEMM_BATCH_M31_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_SGEMM_BATCH_M31_H

#include "reactor_api.h"

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct prometheus_runtime prometheus_runtime;

/* Immutable caller-order facts. These records own neither Vulkan resources
 * nor task records; physical state remains separately represented below. */
typedef struct prom_sgemm_batch_entry_plan {
  uint32_t entry_id;
  uint32_t logical_lane;
  uint32_t plan_generation;
  uint32_t m;
  uint32_t n;
  uint32_t k;
  size_t a_element_count;
  size_t b_element_count;
  size_t c_element_count;
  size_t a_byte_count;
  size_t b_byte_count;
  size_t c_byte_count;
  const float* a;
  const float* b;
  float* c;
  uint32_t selected_path;
  uint32_t compute_mode;
  uint32_t requested_variant;
  uint32_t executed_variant;
  uint32_t planning_flags;
} prom_sgemm_batch_entry_plan;

typedef struct prom_sgemm_batch_plan {
  uint32_t entry_count;
  uint32_t requested_logical_width;
  uint32_t planned_logical_width;
  uint32_t partition_policy;
  uint32_t plan_generation;
  prom_sgemm_batch_entry_plan* entries;
} prom_sgemm_batch_plan;

typedef enum prom_batch_entry_state {
  PROM_BATCH_ENTRY_PLANNED = 1u,
  PROM_BATCH_ENTRY_ADMITTED = 2u,
  PROM_BATCH_ENTRY_SUBMITTED = 3u,
  PROM_BATCH_ENTRY_COMPLETED = 4u,
  PROM_BATCH_ENTRY_FAILED = 5u,
  PROM_BATCH_ENTRY_SKIPPED = 6u,
  PROM_BATCH_ENTRY_DRAINED = 7u,
  PROM_BATCH_ENTRY_COMMITTED = 8u,
} prom_batch_entry_state;

typedef struct prom_sgemm_batch_entry_runtime {
  uint32_t entry_id;
  uint32_t plan_generation;
  uint32_t logical_lane;
  prom_batch_entry_state state;
  uint64_t submission_sequence;
  uint32_t physical_slot_id;
  uint32_t physical_slot_generation;
  uint32_t failure_phase;
  int32_t failure_detail;
  uint64_t observation_sequence;
  uint32_t feedback_committed;
  uint32_t feedback_skipped;
} prom_sgemm_batch_entry_runtime;

int prom_sgemm_batch_plan_build(const PrometheusSgemmBatchEntry* entries,
                                uint32_t entry_count,
                                uint32_t flags,
                                prom_sgemm_batch_plan* out_plan,
                                uint32_t* out_failed_entry_id);
void prom_sgemm_batch_plan_destroy(prom_sgemm_batch_plan* plan);

/* The authoritative M31 batch engine.  Its caller supplies the existing
 * private runtime aggregate; this header intentionally exposes neither its
 * state transitions nor M29/M30 lifecycle internals. */
int prom_sgemm_batch_m31_execute(prometheus_runtime* runtime,
                                 const PrometheusSgemmBatchEntry* entries,
                                 uint32_t entry_count,
                                 uint32_t flags,
                                 uint32_t* out_stage,
                                 int* out_detail_code);

#ifdef __cplusplus
}
#endif

#endif
