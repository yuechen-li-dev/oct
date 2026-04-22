#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_JUDGMENT_ENGINE_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_JUDGMENT_ENGINE_H

#include <stdint.h>

#include "reactor_api.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef enum prom_vk_path_mode {
  PROM_VK_PATH_DIRECT = 1,
  PROM_VK_PATH_STAGED_UPLOAD = 2,
  PROM_VK_PATH_STAGED_UPLOAD_READBACK = 3,
} prom_vk_path_mode;

typedef enum prom_vk_compute_mode {
  PROM_VK_COMPUTE_BASELINE = 1,
  PROM_VK_COMPUTE_TILED = 2,
} prom_vk_compute_mode;

typedef struct prom_judgment_facts {
  uint32_t m;
  uint32_t n;
  uint32_t k;
  uint64_t work_units;
  uint32_t can_stage;
  uint32_t can_direct;
  uint32_t allow_fallback;
  uint32_t readback_required;
  uint32_t force_direct;
  uint32_t force_staged;
  uint32_t force_tiled;
  uint32_t tiled_shape;
  uint32_t software_vulkan;
} prom_judgment_facts;

typedef struct prom_judgment_decision {
  uint32_t success;
  int error_detail;
  prom_vk_path_mode requested_path;
  prom_vk_path_mode selected_path;
  prom_vk_compute_mode compute_mode;
  int final_detail;
  uint32_t used_fallback_to_direct;
  uint32_t winning_candidate_index;
  int winning_score;
} prom_judgment_decision;

typedef struct prom_judgment_async_facts {
  uint32_t request_async;
  uint32_t in_flight;
  uint32_t software_vulkan;
} prom_judgment_async_facts;

typedef struct prom_judgment_async_decision {
  uint32_t success;
  uint32_t execute_async;
  int reject_detail;
} prom_judgment_async_decision;

enum {
  PROM_JUDGMENT_STAGING_WORK_THRESHOLD = 16384u,
  PROM_JUDGMENT_TILED_WORK_THRESHOLD = 131072u,
};

void prom_judgment_engine_select_sgemm_mode(const prom_judgment_facts* facts, prom_judgment_decision* out_decision);
void prom_judgment_engine_select_async_submission(const prom_judgment_async_facts* facts,
                                                  prom_judgment_async_decision* out_decision);

#ifdef __cplusplus
}
#endif

#endif
