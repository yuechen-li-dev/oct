#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_SGEMM_INTERNAL_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_SGEMM_INTERNAL_H

// ============================================================================
// SGEMM Includes / Platform Glue
// ============================================================================

#include "reactor_vulkan.h"
#include "reactor_shader_registry.h"

#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>

#if defined(_WIN32)
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN 1
#endif
#ifndef NOMINMAX
#define NOMINMAX 1
#endif
#include <windows.h>
#else
#include <pthread.h>
#include <time.h>
#endif

#include <vulkan/vulkan.h>
#include "reactor_dominatus_blackboard.h"
#include "reactor_dominatus_sgemm_adapter.h"
#include "reactor_dominatus_slot_adapter.h"
#include "reactor_dominatus_measurement_filter.h"
#include "reactor_dominatus_predictor.h"
#include "reactor_dominatus_prestage.h"
#include "reactor_judgment_engine.h"
#include "reactor_slot_hfsm.h"
#include "reactor_vulkan_fp16_spirv.h"
#include "reactor_vulkan_packed4_spirv.h"
#include "reactor_vulkan_b2x2_row_major_biased_spirv.h"
#include "reactor_vulkan_a2x4_row_biased_accum8_spirv.h"
#include "reactor_vulkan_memory_conservative_spirv.h"
#include "reactor_sgemm_dispatch_metadata.h"
#include "reactor_vulkan_sgemm_scalar_plus_spirv.h"
#include "reactor_vulkan_sgemm_reg2x2_tile16x16_fp32_spirv.h"
#include "reactor_vulkan_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv.h"
#include "reactor_vulkan_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv.h"
#include "reactor_vulkan_sgemm_reg2x2_tile16x16_derive_fp32_spirv.h"
#include "reactor_vulkan_sgemm_tile16x16_shared_fp32_spirv.h"
#include "reactor_vulkan_srt_2accum_k_spirv.h"
#include "reactor_vulkan_tiled_spirv.h"

#define PROMETHEUS_RUNTIME_MAGIC 0x50524f4du
#define PROMETHEUS_MAX_TRACKED_HANDLES 256

#define PROM_VK_LOCAL_SIZE_X 8u
#define PROM_VK_LOCAL_SIZE_Y 8u
#define PROM_VK_TILE_K 8u
#define PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH 4u
#define PROM_SGEMM_SUBMISSION_RING_DEFAULT_DEPTH 2u
#define PROM_SGEMM_ASYNC_MAX_TASKS 4u

typedef enum prom_sgemm_submission_slot_state {
  PROM_SGEMM_SUBMISSION_SLOT_EMPTY = 0u,
  PROM_SGEMM_SUBMISSION_SLOT_PREPARING = 1u,
  PROM_SGEMM_SUBMISSION_SLOT_RECORDED = 2u,
  PROM_SGEMM_SUBMISSION_SLOT_SUBMITTED = 3u,
  PROM_SGEMM_SUBMISSION_SLOT_COMPLETE = 4u,
  PROM_SGEMM_SUBMISSION_SLOT_READY = 5u,
  PROM_SGEMM_SUBMISSION_SLOT_FAILED = 6u,
  /* A caller has terminated interest, but the GPU fence has not established
     that command/descriptor/query resources may be touched again. */
  PROM_SGEMM_SUBMISSION_SLOT_QUARANTINED = 7u,
  PROM_SGEMM_SUBMISSION_SLOT_FAILED_FATAL = 8u,
} prom_sgemm_submission_slot_state;

typedef struct prom_sgemm_submission_slot {
  uint32_t slot_id;
  uint32_t generation;
  uint32_t state;
  uint64_t submission_sequence;
  VkCommandBuffer command_buffer;
  VkFence fence;
  VkDescriptorSet descriptor_set;
  uint32_t query_base;
  uint32_t m;
  uint32_t n;
  uint32_t compute_k;
  uint32_t compute_mode;
  uint32_t variant;
  uint32_t timing_valid;
  uint64_t gpu_duration_ns;
  uint32_t physical_completion_confirmed;
  uint32_t failure_stage;
  int32_t failure_detail;
} prom_sgemm_submission_slot;

typedef struct prom_sgemm_submission_ring_diag {
  uint32_t configured_depth;
  uint32_t outstanding;
  uint32_t max_outstanding;
  uint32_t acquire_cursor;
  uint64_t next_sequence;
  uint64_t total_submits;
  uint64_t total_polls;
  uint64_t total_forced_waits;
  uint64_t total_query_harvests;
  uint64_t ring_full_count;
  uint64_t slot_recycle_count;
  uint64_t slot_failure_count;
  uint64_t total_gpu_duration_ns;
  uint64_t gpu_timing_sample_count;
} prom_sgemm_submission_ring_diag;

/* Public async records own their host-visible input/output allocations.  They
   deliberately do not borrow the resident diagnostic buffers: a task may be
   consumed in any order after another task has completed. */
typedef struct prom_sgemm_async_task {
  uint32_t active;
  uint32_t lifecycle_state;
  uint32_t table_index;
  uint32_t generation;
  int32_t public_task_id;
  uint32_t physical_slot_id;
  uint32_t physical_slot_generation;
  /* Batch attribution is copied from immutable planning state. It is never a
     task-table identity and prevents recycled task records being misattributed. */
  uint32_t batch_entry_id;
  uint32_t batch_plan_generation;
  uint64_t submission_sequence;
  uint32_t m, n, k, compute_k;
  uint32_t selected_path, compute_mode, requested_variant, executed_variant;
  uint32_t final_stage;
  int32_t final_detail;
  uint32_t timing_valid;
  uint64_t gpu_duration_ns;
  uint32_t feedback_pending, feedback_committed;
  uint32_t output_ready, consumed, abandoned;
  uint32_t failure_class;
  uint32_t slot_quarantined;
  uint32_t physical_completion_confirmed;
  uint32_t reap_pending, reap_completed;
  prom_vk_buffer a, b, c;
} prom_sgemm_async_task;


static const prom_sgemm_kernel_dispatch_metadata* prom_sgemm_generated_dispatch_metadata_for_variant(uint32_t variant) {
  const prom_sgemm_kernel_dispatch_metadata* registered = prom_shader_registry_dispatch_metadata(variant);
  if (registered != NULL) {
    return registered;
  }
  static prom_sgemm_kernel_dispatch_metadata scalar_plus_metadata;
  static prom_sgemm_kernel_dispatch_metadata reg2x2_metadata;
  static prom_sgemm_kernel_dispatch_metadata reg2x2_exacttail_metadata;
  static prom_sgemm_kernel_dispatch_metadata reg2x2_flowboard_metadata;
  static prom_sgemm_kernel_dispatch_metadata reg2x2_derive_metadata;
  static prom_sgemm_kernel_dispatch_metadata tile16_metadata;
  static uint32_t scalar_plus_metadata_initialized = 0u;
  static uint32_t reg2x2_metadata_initialized = 0u;
  static uint32_t reg2x2_exacttail_metadata_initialized = 0u;
  static uint32_t reg2x2_flowboard_metadata_initialized = 0u;
  static uint32_t reg2x2_derive_metadata_initialized = 0u;
  static uint32_t tile16_metadata_initialized = 0u;
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_SCALAR_PLUS) {
    if (scalar_plus_metadata_initialized == 0u) {
      scalar_plus_metadata.threads_x = k_prom_sgemm_scalar_plus_spirv_numthreads_x;
      scalar_plus_metadata.threads_y = k_prom_sgemm_scalar_plus_spirv_numthreads_y;
      scalar_plus_metadata.threads_z = k_prom_sgemm_scalar_plus_spirv_numthreads_z;
      scalar_plus_metadata.outputs_per_invocation_m = k_prom_sgemm_scalar_plus_spirv_outputs_per_invocation_m;
      scalar_plus_metadata.outputs_per_invocation_n = k_prom_sgemm_scalar_plus_spirv_outputs_per_invocation_n;
      scalar_plus_metadata.tile_m = k_prom_sgemm_scalar_plus_spirv_tile_m;
      scalar_plus_metadata.tile_n = k_prom_sgemm_scalar_plus_spirv_tile_n;
      scalar_plus_metadata.tile_k = 0u;
      scalar_plus_metadata.unroll_k = k_prom_sgemm_scalar_plus_spirv_unroll_k;
      scalar_plus_metadata_initialized = 1u;
    }
    return &scalar_plus_metadata;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FP32) {
    if (reg2x2_metadata_initialized == 0u) {
      reg2x2_metadata.threads_x = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_numthreads_x;
      reg2x2_metadata.threads_y = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_numthreads_y;
      reg2x2_metadata.threads_z = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_numthreads_z;
      reg2x2_metadata.outputs_per_invocation_m = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_outputs_per_invocation_m;
      reg2x2_metadata.outputs_per_invocation_n = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_outputs_per_invocation_n;
      reg2x2_metadata.tile_m = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_tile_m;
      reg2x2_metadata.tile_n = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_tile_n;
      reg2x2_metadata.tile_k = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_tile_k;
      reg2x2_metadata.unroll_k = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_unroll_k;
      reg2x2_metadata_initialized = 1u;
    }
    return &reg2x2_metadata;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32) {
    if (reg2x2_exacttail_metadata_initialized == 0u) {
      reg2x2_exacttail_metadata.threads_x = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_numthreads_x;
      reg2x2_exacttail_metadata.threads_y = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_numthreads_y;
      reg2x2_exacttail_metadata.threads_z = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_numthreads_z;
      reg2x2_exacttail_metadata.outputs_per_invocation_m =
          k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_outputs_per_invocation_m;
      reg2x2_exacttail_metadata.outputs_per_invocation_n =
          k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_outputs_per_invocation_n;
      reg2x2_exacttail_metadata.tile_m = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_tile_m;
      reg2x2_exacttail_metadata.tile_n = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_tile_n;
      reg2x2_exacttail_metadata.tile_k = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_tile_k;
      reg2x2_exacttail_metadata.unroll_k = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_unroll_k;
      reg2x2_exacttail_metadata_initialized = 1u;
    }
    return &reg2x2_exacttail_metadata;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32) {
    if (reg2x2_flowboard_metadata_initialized == 0u) {
      reg2x2_flowboard_metadata.threads_x = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_numthreads_x;
      reg2x2_flowboard_metadata.threads_y = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_numthreads_y;
      reg2x2_flowboard_metadata.threads_z = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_numthreads_z;
      reg2x2_flowboard_metadata.outputs_per_invocation_m =
          k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_outputs_per_invocation_m;
      reg2x2_flowboard_metadata.outputs_per_invocation_n =
          k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_outputs_per_invocation_n;
      reg2x2_flowboard_metadata.tile_m = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_tile_m;
      reg2x2_flowboard_metadata.tile_n = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_tile_n;
      reg2x2_flowboard_metadata.tile_k = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_tile_k;
      reg2x2_flowboard_metadata.unroll_k = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_unroll_k;
      reg2x2_flowboard_metadata_initialized = 1u;
    }
    return &reg2x2_flowboard_metadata;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_DERIVE_FP32) {
    if (reg2x2_derive_metadata_initialized == 0u) {
      reg2x2_derive_metadata.threads_x = k_prom_sgemm_reg2x2_tile16x16_derive_fp32_spirv_numthreads_x;
      reg2x2_derive_metadata.threads_y = k_prom_sgemm_reg2x2_tile16x16_derive_fp32_spirv_numthreads_y;
      reg2x2_derive_metadata.threads_z = k_prom_sgemm_reg2x2_tile16x16_derive_fp32_spirv_numthreads_z;
      reg2x2_derive_metadata.outputs_per_invocation_m =
          k_prom_sgemm_reg2x2_tile16x16_derive_fp32_spirv_outputs_per_invocation_m;
      reg2x2_derive_metadata.outputs_per_invocation_n =
          k_prom_sgemm_reg2x2_tile16x16_derive_fp32_spirv_outputs_per_invocation_n;
      reg2x2_derive_metadata.tile_m = k_prom_sgemm_reg2x2_tile16x16_derive_fp32_spirv_tile_m;
      reg2x2_derive_metadata.tile_n = k_prom_sgemm_reg2x2_tile16x16_derive_fp32_spirv_tile_n;
      reg2x2_derive_metadata.tile_k = k_prom_sgemm_reg2x2_tile16x16_derive_fp32_spirv_tile_k;
      reg2x2_derive_metadata.unroll_k = k_prom_sgemm_reg2x2_tile16x16_derive_fp32_spirv_unroll_k;
      reg2x2_derive_metadata_initialized = 1u;
    }
    return &reg2x2_derive_metadata;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_TILE16X16_SHARED_FP32) {
    if (tile16_metadata_initialized == 0u) {
      tile16_metadata.threads_x = k_prom_sgemm_tile16x16_shared_fp32_spirv_numthreads_x;
      tile16_metadata.threads_y = k_prom_sgemm_tile16x16_shared_fp32_spirv_numthreads_y;
      tile16_metadata.threads_z = k_prom_sgemm_tile16x16_shared_fp32_spirv_numthreads_z;
      tile16_metadata.outputs_per_invocation_m = k_prom_sgemm_tile16x16_shared_fp32_spirv_outputs_per_invocation_m;
      tile16_metadata.outputs_per_invocation_n = k_prom_sgemm_tile16x16_shared_fp32_spirv_outputs_per_invocation_n;
      tile16_metadata.tile_m = k_prom_sgemm_tile16x16_shared_fp32_spirv_tile_m;
      tile16_metadata.tile_n = k_prom_sgemm_tile16x16_shared_fp32_spirv_tile_n;
      tile16_metadata.tile_k = k_prom_sgemm_tile16x16_shared_fp32_spirv_tile_k;
      tile16_metadata.unroll_k = k_prom_sgemm_tile16x16_shared_fp32_spirv_unroll_k;
      tile16_metadata_initialized = 1u;
    }
    return &tile16_metadata;
  }
  return NULL;
}

static prom_sgemm_dispatch_geometry prom_sgemm_dispatch_geometry_for_variant(uint32_t variant, uint32_t m, uint32_t n) {
  const prom_sgemm_kernel_dispatch_metadata* metadata =
      prom_sgemm_generated_dispatch_metadata_for_variant(variant);
  prom_sgemm_dispatch_geometry geometry;
  if (metadata != NULL) {
    return prom_sgemm_dispatch_geometry_for_metadata(m, n, metadata);
  }
  geometry.groups_x = prom_sgemm_ceil_div_u32(m, PROM_VK_LOCAL_SIZE_X);
  geometry.groups_y = prom_sgemm_ceil_div_u32(n, PROM_VK_LOCAL_SIZE_Y);
  geometry.groups_z = 1u;
  geometry.logical_m_per_group = PROM_VK_LOCAL_SIZE_X;
  geometry.logical_n_per_group = PROM_VK_LOCAL_SIZE_Y;
  return geometry;
}

// ============================================================================
// SGEMM Runtime State
// ============================================================================

typedef enum prom_buffer_artifact_kind {
  PROM_BUFFER_ARTIFACT_A = 1,
  PROM_BUFFER_ARTIFACT_B = 2,
  PROM_BUFFER_ARTIFACT_C = 3,
} prom_buffer_artifact_kind;

typedef enum prom_buffer_invalidation_reason {
  PROM_BUFFER_INVALIDATION_REASON_NONE = 0,
  PROM_BUFFER_INVALIDATION_REASON_UNINITIALIZED = 1,
  PROM_BUFFER_INVALIDATION_REASON_DEPENDENCY = 2,
  PROM_BUFFER_INVALIDATION_REASON_LAYOUT_PRECISION = 3,
  PROM_BUFFER_INVALIDATION_REASON_CAPACITY = 4,
} prom_buffer_invalidation_reason;

typedef enum prom_arena_role {
  PROM_ARENA_ROLE_A = 0,
  PROM_ARENA_ROLE_B = 1,
  PROM_ARENA_ROLE_C = 2,
  PROM_ARENA_ROLE_UPLOAD = 3,
  PROM_ARENA_ROLE_COUNT = 4,
} prom_arena_role;

typedef enum prom_arena_budget_role_mask {
  PROM_ARENA_BUDGET_ROLE_MASK_NONE = 0u,
  PROM_ARENA_BUDGET_ROLE_MASK_A = (1u << PROM_ARENA_ROLE_A),
  PROM_ARENA_BUDGET_ROLE_MASK_B = (1u << PROM_ARENA_ROLE_B),
  PROM_ARENA_BUDGET_ROLE_MASK_C = (1u << PROM_ARENA_ROLE_C),
  PROM_ARENA_BUDGET_ROLE_MASK_UPLOAD = (1u << PROM_ARENA_ROLE_UPLOAD),
  PROM_ARENA_BUDGET_ROLE_MASK_DIRECT = (1u << PROM_ARENA_ROLE_A) | (1u << PROM_ARENA_ROLE_B) | (1u << PROM_ARENA_ROLE_C),
  PROM_ARENA_BUDGET_ROLE_MASK_STAGED = (1u << PROM_ARENA_ROLE_A) | (1u << PROM_ARENA_ROLE_B) | (1u << PROM_ARENA_ROLE_C) |
                                       (1u << PROM_ARENA_ROLE_UPLOAD),
} prom_arena_budget_role_mask;

typedef enum prom_arena_memory_class {
  PROM_ARENA_MEMORY_HOST_VISIBLE = 1,
  PROM_ARENA_MEMORY_DEVICE_LOCAL = 2,
} prom_arena_memory_class;

typedef struct prom_typed_arena {
  prom_arena_role role;
  uint64_t required_bytes;
  uint64_t capacity_bytes;
  uint64_t committed_live_bytes;
  uint64_t generation;
  uint32_t artifact_key_valid;
  uint32_t artifact_key_m;
  uint32_t artifact_key_n;
  uint32_t artifact_key_k;
  uint32_t artifact_key_compute_or_padded_k;
  uint64_t artifact_key_required_bytes;
  uint32_t layout_namespace;
  uint32_t precision_namespace;
  prom_arena_memory_class memory_class;
  int owner_slot_id;
  uint32_t valid;
  uint32_t in_flight;
  int last_failure_reason;
  uint32_t low_usage_epoch_count;
  uint32_t shrink_cooldown_epochs;
  uint64_t reuse_count;
  uint64_t grow_count;
  uint64_t shrink_count;
  uint64_t rebuild_count;
  uint64_t budget_rejection_count;
  uint64_t ownership_rejection_count;
  uint64_t namespace_rejection_count;
} prom_typed_arena;

typedef struct prom_buffer_artifact_key {
  uint32_t valid;
  uint32_t m;
  uint32_t n;
  uint32_t k;
  uint32_t compute_or_padded_k;
  uint32_t layout;
  uint32_t precision;
  uint64_t required_bytes;
} prom_buffer_artifact_key;

typedef struct prom_sgemm_controller_defaults {
  uint32_t lookahead_default;
  uint32_t lookahead_min;
  uint32_t lookahead_max;
  uint32_t outstanding_default;
  uint32_t outstanding_min;
  uint32_t outstanding_max;
  uint32_t chunk_default;
  uint32_t chunk_min;
  uint32_t chunk_max;
  uint32_t waste_budget_units;
  uint32_t retreat_permille;
  uint32_t recover_permille;
  uint32_t recovery_window;
} prom_sgemm_controller_defaults;

typedef struct prom_sgemm_controller_state {
  prom_policy_memory policy_memory;
  prom_policy_thresholds policy_thresholds;
  prom_policy_facts policy_facts;
  uint32_t lookahead;
  uint32_t outstanding_depth;
  uint32_t chunk_size;
  uint32_t pending_waste_units;
  uint32_t last_shape_signature;
  uint32_t last_shape_m;
  uint32_t last_shape_n;
  uint32_t last_shape_k;
  uint32_t last_mode;
  uint32_t decision_count;
  uint32_t retreat_count;
  uint32_t recovery_count;
  uint32_t transition_count;
  uint32_t instability_count;
  uint32_t budget_depletion_count;
  uint32_t safe_mode_decisions;
  uint32_t aggressive_mode_decisions;
  uint32_t recovery_mode_decisions;
  uint32_t lag_early_warning_count;
  uint32_t burst_dampening_count;
  uint32_t bound_violation_count;
  uint64_t wasted_work_units_total;
  uint32_t wasted_work_units_last;
  uint32_t packed4_selected_layout_format;
  uint32_t packed4_tail_count_last;
  uint64_t packed4_tail_count_total;
  uint32_t packed4_padded_lane_count_last;
  uint64_t packed4_padded_lane_count_total;
  uint32_t packed4_padding_waste_permille_last;
  uint64_t packed4_mode_budget_denials;
  uint64_t packed4_row_major_check_failures;
  uint64_t packed4_selection_count;
  uint64_t packed4_fallback_reason_padding_waste;
  uint64_t packed4_fallback_reason_small_shape;
  uint64_t packed4_fallback_reason_capability_missing;
  uint64_t packed4_fallback_reason_fallback_required;
  uint64_t packed4_fallback_reason_mode_budget_denied;
  float fp16_max_absolute_error;
  float fp16_max_relative_error;
  float fp16_aggregate_error;
  uint32_t fp16_worst_case_element_index;
  float fp16_k_error_growth;
  float fp16_cancellation_risk;
  uint32_t fp16_tolerance_known;
  uint32_t fp16_tolerance_pass;
  int fp16_fallback_reason_detail;
  uint32_t fp16_selected_candidate;
} prom_sgemm_controller_state;

typedef struct prom_slot_runtime_diag {
  uint32_t current_slot_id;
  uint32_t next_slot_id;
  uint64_t swap_count;
  uint64_t max_wip_depth;
  uint64_t overwrite_rejection_count;
  uint64_t stale_buffer_rejection_count;
  uint64_t shape_invalidation_count;
  uint64_t layout_invalidation_count;
  uint64_t capacity_invalidation_count;
  uint64_t m14_a_invalidation_count;
  uint64_t m14_b_invalidation_count;
  uint64_t m14_c_invalidation_count;
  uint64_t m14_a_reuse_count;
  uint64_t m14_b_reuse_count;
  uint64_t m14_c_reuse_count;
  uint64_t m14_false_invalidation_avoided_count;
  uint64_t m14_capacity_invalidation_count;
  uint64_t m14_layout_precision_invalidation_count;
  uint32_t m14_a_last_invalidation_reason;
  uint32_t m14_b_last_invalidation_reason;
  uint32_t m14_c_last_invalidation_reason;
  uint64_t inflight_rejection_count;
  uint64_t cleanup_success_count;
  int failure_slot_id;
  int failure_reason;
  int async_slot_id;
  uint32_t transfer_queue_used;
  uint32_t transfer_policy_selected;
  uint32_t dedicated_transfer_available;
  uint32_t transfer_queue_family_index;
  uint32_t compute_queue_family_index;
  uint32_t queue_families_differ;
  uint64_t queue_family_handoff_count;
  uint64_t transfer_compute_wait_count;
  uint32_t transfer_fallback_reason;
  int transfer_failure_slot_id;
  int transfer_failure_reason;
  uint64_t transfer_failure_count;
  uint32_t async_transfer_complete;
  uint64_t async_transfer_completion_generation;
  uint32_t m35_selected_mode;
  uint32_t m35_fixed_feasible;
  uint32_t m35_pull_lag_feasible;
  uint32_t m35_serial_feasible;
  uint32_t m35_fixed_rejected;
  uint32_t m35_pull_lag_rejected;
  uint32_t m35_serial_rejected;
  uint32_t m35_reason_code;
  uint32_t m35_final_reason_code;
  uint32_t m35_fixed_double_rejection_reason;
  uint32_t m35_pull_lag_rejection_reason;
  uint32_t m35_serial_jit_rejection_reason;
  uint32_t m35_transition_count;
  uint32_t m35_rejection_count;
  int m35_fixed_score;
  int m35_pull_lag_score;
  int m35_serial_score;
  uint64_t m35_memory_budget_slots_permille;
  uint64_t m35_required_fixed_slots_permille;
  uint64_t m35_required_pull_lag_slots_permille;
  uint64_t m35_required_serial_slots_permille;
  int64_t m35_fixed_double_headroom_slots_permille;
  int64_t m35_pull_lag_headroom_slots_permille;
  int64_t m35_serial_jit_headroom_slots_permille;
  uint64_t m35_budget_rejection_count;
  uint64_t m35_pull_lag_predicted_demand_proxy_units;
  uint64_t m35_pull_lag_transfer_lead_proxy_units;
  uint64_t m35_pull_lag_safety_margin_proxy_units;
  uint64_t m35_pull_lag_stage_start_proxy_units;
  uint64_t m35_pull_lag_stage_complete_proxy_units;
  uint64_t m35_pull_lag_late_stage_count;
  uint64_t m35_pull_lag_early_stage_count;
  uint64_t m35_pull_lag_starvation_proxy_units;
  uint64_t m35_pull_lag_ready_unused_proxy_units;
  uint64_t m35_pull_lag_wip_waste_exceeded_count;
  uint64_t m35_serial_active_slot_count;
  uint64_t m35_serial_wip_depth;
  uint64_t m35_serial_sequential_step_count;
  uint64_t m35_serial_busy_retry_count;
  uint64_t m35_serial_failure_cleanup_count;
  uint32_t p13_m2_occupancy_device_band;
  uint32_t p13_m2_occupancy_shape_class;
  uint32_t p13_m2_occupancy_selected_variant;
  uint32_t p13_m2_occupancy_unclamped_variant;
  uint32_t p13_m2_occupancy_clamp_reason;
  uint32_t p13_m2_occupancy_override_used;
  uint32_t p13_m2_occupancy_fallback_used;
  uint32_t p13_m16b1_requested_occupancy_variant;
  uint32_t p13_m16b1_executed_occupancy_variant;
  uint32_t p13_m16b1_variant_registered;
  uint32_t p13_m16b1_variant_benchmark_enabled;
  uint32_t p13_m16b1_variant_dvt_validated;
  uint32_t p13_m16b1_variant_pvt_validated;
  uint32_t p13_m16b1_variant_production_eligible;
  uint32_t p13_m16b1_variant_dispatch_enabled;
  uint32_t p13_m16b1_variant_path_status;
  uint32_t p13_m16b1_variant_path_id;
  uint32_t p13_m16b1_fallback_reason;
  uint32_t px16_m6_selector_recommended_variant;
  uint32_t px16_m6_selector_selected_variant;
  uint32_t px16_m6_requested_dispatch_variant;
  uint32_t px16_m6_executed_dispatch_variant;
  uint32_t px16_m6_requested_path;
  uint32_t px16_m6_selected_path;
  uint32_t px16_m6_executed_path;
  uint32_t px16_m6_requested_compute_mode;
  uint32_t px16_m6_selected_compute_mode;
  uint32_t px16_m6_executed_compute_mode;
  uint32_t px16_m6_force_direct_requested;
  uint32_t px16_m6_force_direct_applied;
  uint32_t px16_m6_force_direct_reason;
  uint32_t px16_m6_policy_mode;
  uint32_t px16_m6_variant_path_status;
  uint32_t px16_m6_variant_production_eligible;
  uint32_t px16_m6_variant_dispatch_enabled;
  uint32_t px16_m6_variant_dvt_validated;
  uint32_t px16_m6_variant_pvt_validated;
  uint32_t px16_m6_variant_lifecycle_telemetry_only;
  uint32_t px16_m6_p15_reservation_present;
  uint32_t px16_m6_p15_reservation_matured;
  uint32_t px16_m6_p15_reservation_consumed;
  uint32_t px16_m6_p15_reserved_variant_id;
  uint32_t px16_m6_p15_live_selected_variant_id;
  uint32_t px16_m6_p15_reconciliation_match;
  uint32_t px16_m6_p15_block_reason;
  uint32_t px16_m6_p15_correction_action;
  uint32_t px16_m6_p15_reservation_stale_or_expired;
  double px16_m6_p15_confidence_before;
  double px16_m6_p15_confidence_after;
  uint64_t p11_m3_total_committed_bytes;
  uint64_t p11_m3_projected_committed_bytes;
  uint64_t p11_m3_budget_limit_bytes;
  int p11_m3_last_failure_reason;
} prom_slot_runtime_diag;

typedef struct prom_selector_cache_m35 {
  uint32_t valid;
  uint32_t last_decision_reused;
  uint64_t visible_generation_when_computed;
  uint64_t dependency_mask;
  uint64_t last_dirty_dependency_mask;
  uint64_t reuse_count;
  uint64_t recompute_count;
  uint64_t invalidation_count;
  prom_buffering_selector_decision decision;
  uint32_t no_feasible_mode_detail;
} prom_selector_cache_m35;

typedef struct prom_selector_cache_transfer {
  uint32_t valid;
  uint32_t last_decision_reused;
  uint64_t visible_generation_when_computed;
  uint64_t dependency_mask;
  uint64_t last_dirty_dependency_mask;
  uint64_t reuse_count;
  uint64_t recompute_count;
  uint64_t invalidation_count;
  uint32_t selected_path;
  prom_dom_transfer_queue_decision decision;
} prom_selector_cache_transfer;

typedef struct prom_selector_cache_layout_precision {
  uint32_t valid;
  uint32_t last_decision_reused;
  uint64_t visible_generation_when_computed;
  uint64_t dependency_mask;
  uint64_t last_dirty_dependency_mask;
  uint64_t reuse_count;
  uint64_t recompute_count;
  uint64_t invalidation_count;
  uint64_t layout_precision_invalidation_count_when_computed;
  prom_judgment_layout_precision_decision decision;
} prom_selector_cache_layout_precision;

typedef struct prom_p15_feedforward_dispatch_state {
  uint32_t valid;
  uint32_t enabled;
  uint32_t used;
  uint32_t source;
  uint32_t reservation_present;
  uint32_t reservation_matured;
  uint32_t block_reason;
  uint32_t reserved_variant_id;
  uint32_t selected_variant_id;
  uint32_t reconciliation_match;
  uint32_t correction_action;
  uint32_t reservation_consumed;
  uint32_t reservation_stale_or_expired;
  double confidence_before;
  double confidence_after;
  uint64_t fallback_to_judgment_count;
  uint64_t reservation_consumed_count;
  uint64_t no_matured_reservation_count;
  uint64_t shape_mismatch_count;
  uint64_t variant_mismatch_count;
  uint64_t stale_reservation_count;
  uint64_t reason_binding_block_count;
  uint64_t margin_block_count;
  uint64_t dedup_block_count;
} prom_p15_feedforward_dispatch_state;

typedef struct prom_p15_feedforward_reservation_probe {
  const prom_dominatus_reservation_request* exact_match;
  const prom_dominatus_reservation_request* variant_mismatch;
  const prom_dominatus_reservation_request* shape_mismatch;
  const prom_dominatus_reservation_request* stale;
  const prom_dominatus_reservation_request* cancelled;
  const prom_dominatus_reservation_request* consumed;
  const prom_dominatus_reservation_request* pending;
  uint32_t present;
  uint32_t matured;
} prom_p15_feedforward_reservation_probe;

typedef struct prometheus_runtime {
  uint32_t magic;
  uint32_t available;
  uint32_t reason_code;
  int init_detail_code;
  uint32_t test_flags;
  uint32_t async_test_flags;
  uint32_t reduction_test_flags;
  uint32_t reduction_ring_depth;
  void* reduction_state;

  VkInstance instance;
  VkDebugUtilsMessengerEXT validation_debug_messenger;
  uint32_t validation_requested;
  uint32_t validation_available;
  uint32_t validation_enabled;
  uint32_t validation_debug_utils_active;
  uint32_t validation_message_count;
  uint32_t validation_warning_count;
  uint32_t validation_error_count;
	uint32_t cooperative_matrix_state;
	uint32_t cooperative_matrix_extension_spec_version;
	uint32_t cooperative_matrix_feature_enabled;
	uint32_t cooperative_matrix_shader_float16_enabled;
	uint32_t cooperative_matrix_vulkan_memory_model_enabled;
	uint32_t cooperative_matrix_tuple_count;
	uint32_t cooperative_matrix_selected_m;
	uint32_t cooperative_matrix_selected_n;
	uint32_t cooperative_matrix_selected_k;
	uint32_t subgroup_size;
	uint32_t subgroup_supported_stages;
	uint32_t subgroup_supported_operations;
	uint32_t subgroup_compute_supported;
	uint32_t subgroup_arithmetic_supported;
	uint32_t subgroup_basic_supported;
	uint32_t subgroup_fixed_size_32_admitted;
  VkDebugUtilsMessageSeverityFlagBitsEXT validation_last_severity;
  VkDebugUtilsMessageTypeFlagsEXT validation_last_type;
  char validation_last_message_id[128];
  char validation_last_message[512];
  VkPhysicalDevice physical_device;
  VkDevice device;
  uint32_t queue_family_index;
  uint32_t transfer_queue_family_index;
  /* Legacy-owned init-time capability constant; Dominatus mirrors derived queue-policy facts per commit. */
  uint32_t dedicated_transfer_available;
  uint32_t transfer_queue_enabled;
  VkQueue compute_queue;
  VkQueue transfer_queue;
  VkCommandPool command_pool;
  VkCommandPool transfer_command_pool;
  VkDescriptorSetLayout descriptor_set_layout;
  VkDescriptorPool descriptor_pool;
  VkDescriptorSet descriptor_set;
  /* M29 owns distinct resources per physical submission; legacy handles stay
     dedicated to synchronous/async compatibility paths. */
  prom_sgemm_submission_slot submission_ring[PROM_SGEMM_SUBMISSION_RING_MAX_DEPTH];
  prom_sgemm_submission_ring_diag submission_ring_diag;
  prom_sgemm_async_task async_tasks[PROM_SGEMM_ASYNC_MAX_TASKS];
  uint32_t async_task_cursor;
  uint64_t async_next_feedback_sequence;
  uint64_t async_next_submission_sequence;
  uint64_t async_queue_full_count;
  uint64_t async_stale_reject_count;
  uint64_t async_feedback_committed_count;
  uint64_t async_feedback_skipped_count;
  uint64_t async_quarantine_event_count;
  uint64_t async_reap_poll_count;
  uint64_t async_reap_success_count;
  uint64_t async_reap_wait_count;
  uint64_t async_reap_failure_count;
  uint32_t async_max_quarantine_depth;
  uint32_t async_runtime_unsafe_to_reuse;
  VkCommandBuffer command_buffer;
  VkCommandBuffer transfer_command_buffer;
  VkFence submit_fence;
  VkFence transfer_submit_fence;
  VkSemaphore transfer_ready_semaphore;
  VkQueryPool sgemm_timestamp_query_pool;
  VkPipelineLayout pipeline_layout;
  VkPipeline pipeline;
  VkPipeline tiled_pipeline;
  VkShaderModule memory_conservative_shader_module;
  VkPipeline memory_conservative_pipeline;
  VkShaderModule sdsl_scalar_plus_shader_module;
  VkPipeline sdsl_scalar_plus_pipeline;
  VkShaderModule sdsl_reg2x2_tile16x16_fp32_shader_module;
  VkPipeline sdsl_reg2x2_tile16x16_fp32_pipeline;
  VkShaderModule sdsl_reg2x2_tile16x16_exacttail_fp32_shader_module;
  VkPipeline sdsl_reg2x2_tile16x16_exacttail_fp32_pipeline;
  VkShaderModule sdsl_reg2x2_tile16x16_flowboard_fp32_shader_module;
  VkPipeline sdsl_reg2x2_tile16x16_flowboard_fp32_pipeline;
  VkShaderModule sdsl_reg2x2_tile16x16_derive_fp32_shader_module;
  VkPipeline sdsl_reg2x2_tile16x16_derive_fp32_pipeline;
  VkShaderModule sdsl_tile16x16_shared_fp32_shader_module;
  VkPipeline sdsl_tile16x16_shared_fp32_pipeline;
  VkPipeline srt_2accum_k_pipeline;
  VkPipeline b2x2_row_major_biased_pipeline;
  VkPipeline a2x4_row_biased_accum8_pipeline;
  VkPipeline packed4_pipeline;
  VkPipeline fp16_pipeline;
  /* Mutable Vulkan state is separate from immutable registry descriptors. */
  prom_compute_pipeline_instance compute_pipeline_instances[11u];
  prom_vk_buffer direct_a;
  prom_vk_buffer direct_b;
  prom_vk_buffer direct_c;
  prom_vk_buffer staged_device_a;
  prom_vk_buffer staged_device_b;
  prom_vk_buffer staged_device_c;
  prom_vk_buffer staged_upload_a;
  prom_vk_buffer staged_upload_b;
  prom_vk_buffer staged_readback_c;
  prom_buffer_artifact_key direct_a_key;
  prom_buffer_artifact_key direct_b_key;
  prom_buffer_artifact_key direct_c_key;
  prom_buffer_artifact_key staged_a_key;
  prom_buffer_artifact_key staged_b_key;
  prom_buffer_artifact_key staged_c_key;
  uint32_t last_execution_shape_valid;
  uint32_t last_execution_m;
  uint32_t last_execution_n;
  uint32_t last_execution_k;
  uint32_t has_direct_buffers;
  uint32_t has_staged_buffers;
  uint32_t has_device_local_memory;
  uint32_t has_host_visible_memory;
  uint32_t occupancy_register_file_class;
  uint32_t occupancy_shared_memory_class;
  uint32_t occupancy_memory_bandwidth_class;
  uint32_t occupancy_fp32_throughput_class;
  uint32_t occupancy_max_workgroup_class;
  uint32_t occupancy_queue_capability_class;
  uint32_t occupancy_has_exact_profile;
  uint32_t timestamp_query_supported;
  uint32_t timestamp_valid_bits;
  float timestamp_period_ns;
  uint32_t last_gpu_timing_valid;
  uint32_t last_gpu_timing_failure_reason;
  uint64_t last_gpu_duration_ns;
  uint64_t px16_m8_last_upload_wall_ns;
  uint64_t px16_m8_last_pre_dispatch_wall_ns;
  uint64_t px16_m8_last_command_record_wall_ns;
  uint64_t px16_m8_last_dispatch_submit_wall_ns;
  uint64_t px16_m8_last_sync_wait_wall_ns;
  uint64_t px16_m8_last_query_result_wall_ns;
  uint64_t px16_m8_last_post_sync_wall_ns;
  uint64_t px16_m8_last_readback_wall_ns;
  uint64_t px16_m8_last_post_readback_wall_ns;
  uint64_t px16_m8_last_total_wall_ns;
  uint32_t px16_m8_last_executed_explicit_variant_request;
  uint64_t px16_m17_last_tolerance_eval_wall_ns;
  uint32_t px16_m17_last_tolerance_eval_in_dispatch;
  uint32_t px16_m17_last_tolerance_eval_source;
  prom_dominatus_measurement_filter_state p14_measurement_filter_state;
  prom_dominatus_filtered_evidence p14_last_filtered_evidence;
  uint64_t p14_measurement_tick;
  prom_dominatus_predictor_state p15_predictor_state;
  prom_dominatus_prestage_params p15_prestage_params;
  prom_dominatus_correction_event p15_last_correction;
  prom_dominatus_prediction_entry p15_last_prediction_issued;
  prom_dominatus_reservation_decision p15_last_reservation;
  prom_dominatus_prestage_decision p15_last_prestage;
  prom_dominatus_shadow_snapshot p15_last_shadow;
  prom_dominatus_shadow_calibration_state p15_shadow_calibration;
  prom_dominatus_shadow_authority_gate p15_shadow_authority_gate;
  prom_dominatus_shadow_would_act_state p15_shadow_would_act_state;
  prom_dominatus_shadow_canary_params p15_shadow_canary_params;
  prom_dominatus_shadow_canary_state p15_shadow_canary_state;
  prom_p15_feedforward_dispatch_state p15_feedforward_dispatch_state;
  uint32_t in_flight_submit;
  /* Legacy-owned init-time capability constant; Dominatus consumes this via staged SGEMM facts. */
  uint32_t software_vulkan;
  /* Legacy-owned init-time capability constant; Dominatus consumes this via staged layout/precision facts. */
  uint32_t capability_fp16_storage;
  /* Legacy-owned atomic async runtime internals; Dominatus remains the observability/export surface. */
  uint32_t async_state;
  int async_task_id;
  uint32_t async_m;
  uint32_t async_n;
  uint32_t async_k;
  size_t async_c_copy_size;
  prom_vk_path_mode async_selected_path;
  int async_final_detail;
  uint32_t async_stage;
  int async_failure_detail;
  /* Legacy-owned controller integrator internals; Dominatus owns staged facts/decisions emitted from this state. */
  prom_sgemm_controller_state sgemm_controller;
  prom_slot_hfsm slots[2];
  prom_slot_runtime_diag slot_diag;
  prom_selector_cache_m35 m35_selector_cache;
  prom_selector_cache_transfer transfer_selector_cache;
  prom_selector_cache_layout_precision layout_precision_selector_cache;
  prom_typed_arena arenas[PROM_ARENA_ROLE_COUNT];
  uint64_t arena_budget_limit_bytes;
  uint64_t arena_floor_bytes;
  uint32_t arena_shrink_low_usage_threshold_epochs;
  uint32_t arena_shrink_cooldown_epochs;
  int arena_last_failure_detail;
  prom_dom_blackboard blackboard;
  PrometheusSgemmBatchDiagnostics batch_diag;
} prometheus_runtime;

int prom_async_reap_quarantined_slots(prometheus_runtime* rt, uint32_t allow_wait);

prom_sgemm_async_task* prom_async_task_allocate(prometheus_runtime* rt);
void prom_async_task_release(prometheus_runtime* rt, prom_sgemm_async_task* task);
int prom_async_reap_quarantined_slots(prometheus_runtime* rt, uint32_t allow_wait);
int prom_async_record_slot(prometheus_runtime* rt, prom_sgemm_submission_slot* slot, prom_sgemm_async_task* task);
int prom_async_poll_task(prometheus_runtime* rt, prom_sgemm_async_task* task);
void prom_async_process_completion_feedback(prometheus_runtime* rt);
int prom_sgemm_ring_wait_oldest(prometheus_runtime* rt);
int prom_sgemm_ring_submit_slot(prometheus_runtime* rt, prom_sgemm_submission_slot* slot);

#endif
