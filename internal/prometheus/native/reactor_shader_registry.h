#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_SHADER_REGISTRY_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_SHADER_REGISTRY_H

/*
 * Prometheus shader registry atlas
 *
 * This module owns immutable SPIR-V asset facts and immutable, typed compute
 * implementation facts.  It does not own selector scores, request-specific
 * numerical eligibility, or Vulkan handles.  The runtime owns one mutable
 * pipeline instance per compute descriptor; judgment remains the authority
 * that decides which eligible descriptor to request.
 *
 * Shader assets are deliberately shared vocabulary for future graphics work.
 * Compute implementations are deliberately not graphics implementations.
 */

#include <stddef.h>
#include <stdint.h>
#include <vulkan/vulkan.h>

#include "reactor_judgment_engine.h"
#include "reactor_reduction_dispatch_metadata.h"
#include "reactor_sgemm_dispatch_metadata.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef enum prom_shader_stage {
  PROM_SHADER_STAGE_COMPUTE = 1u,
  PROM_SHADER_STAGE_VERTEX = 2u,
  PROM_SHADER_STAGE_FRAGMENT = 3u,
  PROM_SHADER_STAGE_GEOMETRY = 4u,
  PROM_SHADER_STAGE_TESSELLATION_CONTROL = 5u,
  PROM_SHADER_STAGE_TESSELLATION_EVALUATION = 6u,
  PROM_SHADER_STAGE_MESH = 7u,
  PROM_SHADER_STAGE_TASK = 8u,
} prom_shader_stage;

typedef enum prom_shader_source_language {
  PROM_SHADER_SOURCE_UNKNOWN = 0u,
  PROM_SHADER_SOURCE_SDSLV = 1u,
  PROM_SHADER_SOURCE_HLSL = 2u,
  PROM_SHADER_SOURCE_GLSL = 3u,
  PROM_SHADER_SOURCE_SPIRV = 4u,
} prom_shader_source_language;

typedef enum prom_shader_authority {
  PROM_SHADER_AUTHORITY_PRODUCTION = 1u,
  PROM_SHADER_AUTHORITY_EXPERIMENTAL = 2u,
} prom_shader_authority;

typedef enum prom_shader_operation {
  PROM_SHADER_OPERATION_SGEMM = 1u,
  PROM_SHADER_OPERATION_REDUCTION_SUM = 2u,
  PROM_SHADER_OPERATION_REDUCTION_MAX = 3u,
  PROM_SHADER_OPERATION_SOFTMAX = 4u,
} prom_shader_operation;

typedef enum prom_compute_pipeline_family {
  PROM_COMPUTE_PIPELINE_FAMILY_SGEMM = 1u,
  PROM_COMPUTE_PIPELINE_FAMILY_REDUCTION = 2u,
} prom_compute_pipeline_family;

typedef enum prom_compute_pipeline_slot {
  PROM_COMPUTE_PIPELINE_BASELINE = 0u,
  PROM_COMPUTE_PIPELINE_TILED = 1u,
  PROM_COMPUTE_PIPELINE_MEMORY_CONSERVATIVE = 2u,
  PROM_COMPUTE_PIPELINE_SDSL_SCALAR_PLUS = 3u,
  PROM_COMPUTE_PIPELINE_SDSL_TILE16 = 4u,
  PROM_COMPUTE_PIPELINE_SDSL_REG2X2 = 5u,
  PROM_COMPUTE_PIPELINE_SDSL_EXACTTAIL = 6u,
  PROM_COMPUTE_PIPELINE_SDSL_FLOWBOARD = 7u,
  PROM_COMPUTE_PIPELINE_SDSL_DERIVE = 8u,
  PROM_COMPUTE_PIPELINE_SRT = 9u,
  PROM_COMPUTE_PIPELINE_B2X2 = 10u,
  PROM_COMPUTE_PIPELINE_A2X4 = 11u,
  PROM_COMPUTE_PIPELINE_PACKED4 = 12u,
  PROM_COMPUTE_PIPELINE_FP16 = 13u,
  PROM_COMPUTE_PIPELINE_COUNT = 14u,
} prom_compute_pipeline_slot;

typedef struct prom_shader_asset {
  uint32_t shader_id;
  const char* name;
  prom_shader_stage stage;
  const uint32_t* spirv_words;
  size_t spirv_size_bytes;
  const char* entry_point;
  uint64_t capability_mask;
  prom_shader_source_language source_language;
  const char* source_path;
  const char* generated_header_path;
  uint32_t generated;
  uint32_t contains_inline_hlsl;
  uint32_t inline_hlsl_block_count;
  const char* foreign_targets;
  prom_shader_authority authority;
  uint32_t descriptor_binding_count;
  uint32_t push_constant_bytes;
  uint32_t stage_role;
  uint32_t minimum_row_width;
  uint32_t maximum_row_width;
} prom_shader_asset;

typedef struct prom_compute_implementation {
  uint32_t implementation_id; /* Stable public occupancy variant ID. */
  uint32_t operation_id;
  const char* name;
  uint32_t shader_id;
  const prom_sgemm_kernel_dispatch_metadata* dispatch;
  uint64_t capability_mask;
  uint32_t benchmark_enabled;
  uint32_t selector_eligible;
  uint32_t dispatchable;
  prom_compute_pipeline_slot pipeline_slot;
  const prom_reduction_kernel_dispatch_metadata* reduction_dispatch;
  prom_shader_authority authority;
  prom_compute_pipeline_family pipeline_family;
  uint32_t family_pipeline_index;
} prom_compute_implementation;

typedef enum prom_pipeline_init_status {
  PROM_PIPELINE_NOT_INITIALIZED = 0u,
  PROM_PIPELINE_READY = 1u,
  PROM_PIPELINE_UNAVAILABLE = 2u,
  PROM_PIPELINE_FAILED = 3u,
} prom_pipeline_init_status;

typedef struct prom_compute_pipeline_instance {
  const prom_compute_implementation* implementation;
  VkPipeline pipeline;
  prom_pipeline_init_status status;
  int32_t failure_detail;
} prom_compute_pipeline_instance;

const prom_shader_asset* prom_shader_registry_find_shader(uint32_t shader_id);
size_t prom_shader_registry_shader_asset_count(void);
const prom_shader_asset* prom_shader_registry_shader_asset_at(size_t index);
size_t prom_shader_registry_reduction_shader_asset_count(void);
const prom_shader_asset* prom_shader_registry_reduction_shader_asset_at(size_t index);
size_t prom_shader_registry_experimental_shader_asset_count(void);
const prom_shader_asset* prom_shader_registry_experimental_shader_asset_at(size_t index);
const prom_compute_implementation* prom_shader_registry_find_compute_implementation(uint32_t implementation_id);
size_t prom_shader_registry_compute_implementation_count(void);
const prom_compute_implementation* prom_shader_registry_compute_implementation_at(size_t index);
size_t prom_shader_registry_reduction_compute_implementation_count(void);
const prom_compute_implementation* prom_shader_registry_reduction_compute_implementation_at(size_t index);
size_t prom_shader_registry_experimental_compute_implementation_count(void);
const prom_compute_implementation* prom_shader_registry_experimental_compute_implementation_at(size_t index);
const prom_sgemm_kernel_dispatch_metadata* prom_shader_registry_dispatch_metadata(uint32_t implementation_id);
uint32_t prom_shader_registry_is_dispatchable(uint32_t implementation_id);
uint32_t prom_shader_registry_is_selector_eligible(uint32_t implementation_id);
uint32_t prom_shader_registry_validate(void);
uint32_t prom_shader_registry_validate_tables(const prom_shader_asset* assets, size_t asset_count,
                                             const prom_compute_implementation* implementations,
                                             size_t implementation_count);
void prom_shader_registry_initialize_pipeline_instances(prom_compute_pipeline_instance* instances, size_t count,
                                                        const VkPipeline* pipelines, size_t pipeline_count);
VkPipeline prom_shader_registry_pipeline_for_variant(const prom_compute_pipeline_instance* instances, size_t count,
                                                     uint32_t implementation_id);

#ifdef __cplusplus
}
#endif

#endif
