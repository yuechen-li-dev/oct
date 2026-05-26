// ============================================================================
// SGEMM Includes / Platform Glue
// ============================================================================

#include "reactor_vulkan.h"

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
#include "reactor_vulkan_srt_2accum_k_spirv.h"
#include "reactor_vulkan_tiled_spirv.h"

#define PROMETHEUS_RUNTIME_MAGIC 0x50524f4du
#define PROMETHEUS_MAX_TRACKED_HANDLES 256

#define PROM_VK_LOCAL_SIZE_X 8u
#define PROM_VK_LOCAL_SIZE_Y 8u
#define PROM_VK_TILE_K 8u

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

typedef struct prom_batch_plan {
  uint32_t entry_id;
  uint32_t worker_id;
  uint32_t m;
  uint32_t n;
  uint32_t k;
  uint64_t work_units;
  const float* a;
  const float* b;
  float* c;
  uint32_t selected_path;
  uint32_t compute_mode;
  uint32_t buffering_mode;
  uint32_t transfer_policy;
  uint32_t layout_precision_mode;
  uint64_t arena_required_bytes;
  uint32_t expected_output_elements;
  uint32_t plan_generation;
  uint32_t slot_id;
  int32_t failure_policy;
} prom_batch_plan;

typedef enum prom_batch_event_kind {
  PROM_BATCH_EVENT_PLAN_STARTED = 1,
  PROM_BATCH_EVENT_PLAN_SUBMITTED = 2,
  PROM_BATCH_EVENT_PLAN_COMPLETED = 3,
  PROM_BATCH_EVENT_PLAN_FAILED = 4,
  PROM_BATCH_EVENT_WORKER_IDLE = 5,
  PROM_BATCH_EVENT_WORKER_DRAINED = 6,
  PROM_BATCH_EVENT_BATCH_FAILURE_OBSERVED = 7,
} prom_batch_event_kind;

typedef struct prom_batch_worker_event {
  uint32_t kind;
  uint32_t entry_id;
  uint32_t stage;
  int32_t detail;
} prom_batch_worker_event;

typedef struct prom_batch_worker_state {
  uint32_t worker_id;
  uint32_t assigned_count;
  uint32_t completed_count;
  uint32_t event_count;
  uint32_t next_scan_index;
  uint32_t active;
  uint32_t failure_observed;
  uint32_t failure_entry_id;
  uint32_t failure_stage;
  int32_t failure_detail;
  uint32_t resource_mode;
} prom_batch_worker_state;

typedef struct prom_batch_worker_resources {
  uint32_t worker_id;
  uint32_t queue_index;
  uint32_t queue_family_index;
  uint32_t command_pool_id;
  uint32_t command_buffer_id;
  uint32_t fence_id;
  uint32_t slot_id;
  uint32_t output_staging_id;
  uint32_t arena_bank_id;
  uint32_t submit_count;
  uint32_t wait_count;
  uint32_t reset_count;
  uint32_t record_count;
  uint32_t physical_valid;
  uint32_t in_flight;
  uint32_t failed;
  VkCommandPool command_pool;
  VkCommandBuffer command_buffer;
  VkFence fence;
} prom_batch_worker_resources;

typedef struct prom_batch_slot_runtime {
  uint32_t slot_id;
  uint32_t owner_worker_id;
  uint32_t state;
  uint32_t generation;
  uint32_t assigned_plan_id;
  uint32_t assigned_entry_id;
  uint32_t queue_id;
  uint32_t command_resource_id;
  uint32_t arena_id;
  uint32_t output_staging_id;
  uint32_t in_flight;
  uint32_t ready;
  uint32_t invalidated;
  uint32_t failure_stage;
  int32_t failure_detail;
} prom_batch_slot_runtime;

typedef struct prom_batch_shared_state prom_batch_shared_state;

typedef struct prom_p15_feedforward_dispatch_state {
  uint32_t valid;
  uint32_t enabled;
  uint32_t used;
  uint32_t source;
  uint32_t block_reason;
  uint32_t reserved_variant_id;
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

typedef struct prometheus_runtime {
  uint32_t magic;
  uint32_t available;
  uint32_t reason_code;
  int init_detail_code;
  uint32_t test_flags;

  VkInstance instance;
  VkPhysicalDevice physical_device;
  VkDevice device;
  uint32_t queue_family_index;
  uint32_t transfer_queue_family_index;
  /* Legacy-owned init-time capability constant; Dominatus mirrors derived queue-policy facts per commit. */
  uint32_t dedicated_transfer_available;
  uint32_t transfer_queue_enabled;
  VkQueue compute_queue;
  VkQueue compute_queues[8];
  uint32_t reported_compute_queue_count;
  uint32_t independent_compute_queue_count;
  VkQueue transfer_queue;
  VkCommandPool command_pool;
  VkCommandPool transfer_command_pool;
  VkDescriptorSetLayout descriptor_set_layout;
  VkDescriptorPool descriptor_pool;
  VkDescriptorSet descriptor_set;
  VkCommandBuffer command_buffer;
  VkCommandBuffer transfer_command_buffer;
  VkFence submit_fence;
  VkFence transfer_submit_fence;
  VkSemaphore transfer_ready_semaphore;
  VkQueryPool sgemm_timestamp_query_pool;
  VkPipelineLayout pipeline_layout;
  VkPipeline pipeline;
  VkPipeline tiled_pipeline;
  VkPipeline srt_2accum_k_pipeline;
  VkPipeline b2x2_row_major_biased_pipeline;
  VkPipeline a2x4_row_biased_accum8_pipeline;
  VkPipeline packed4_pipeline;
  VkPipeline fp16_pipeline;
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

typedef struct prom_vk_push {
  uint32_t m;
  uint32_t n;
  uint32_t k;
} prom_vk_push;

enum {
  PROM_VK_PUSH_FIELD_OFFSET_M = 0,
  PROM_VK_PUSH_FIELD_OFFSET_N = 4,
  PROM_VK_PUSH_FIELD_OFFSET_K = 8,
  PROM_VK_SHADER_PUSH_BYTES = 12,
};

enum {
  PROM_SGEMM_LOOKAHEAD_DEFAULT = 2u,
  PROM_SGEMM_LOOKAHEAD_MIN = 0u,
  PROM_SGEMM_LOOKAHEAD_MAX = 2u,
  PROM_SGEMM_OUTSTANDING_DEFAULT = 2u,
  PROM_SGEMM_OUTSTANDING_MIN = 1u,
  PROM_SGEMM_OUTSTANDING_MAX = 2u,
  PROM_SGEMM_CHUNK_DEFAULT = 16u,
  PROM_SGEMM_CHUNK_MIN = 8u,
  PROM_SGEMM_CHUNK_MAX = 32u,
  PROM_SGEMM_WASTE_BUDGET_UNITS = 64u,
  PROM_SGEMM_RETREAT_PERMILLE = 250u,
  PROM_SGEMM_RECOVER_PERMILLE = 120u,
  PROM_SGEMM_RECOVERY_WINDOW = 3u,
  PROM_SGEMM_HYSTERESIS_MARGIN = 40u,
  PROM_SGEMM_PACKED4_MODE_BUDGET_AGGRESSIVE = 380u,
  PROM_SGEMM_PACKED4_MODE_BUDGET_SAFE = 220u,
  PROM_SGEMM_PACKED4_MODE_BUDGET_RECOVERY = 140u,
  PROM_ARENA_SHRINK_LOW_USAGE_EPOCHS = 6u,
  PROM_ARENA_SHRINK_COOLDOWN_EPOCHS = 4u,
};

#define PROM_ARENA_DEFAULT_BUDGET_BYTES (512ull * 1024ull * 1024ull)
#define PROM_ARENA_DEFAULT_SHRINK_FLOOR_BYTES (64ull * 1024ull * 1024ull)

/*
 * Push-constant layout contract (M11 hygiene port):
 * - host and shader use the same field list and order: m, n, k
 * - no mixed-width fields
 * - no reliance on implicit host padding
 * - append-only evolution only: add new fields at the end and update both
 *   this host contract and the shader module together
 */
#if defined(__cplusplus)
static_assert(offsetof(prom_vk_push, m) == PROM_VK_PUSH_FIELD_OFFSET_M, "push.m offset drift");
static_assert(offsetof(prom_vk_push, n) == PROM_VK_PUSH_FIELD_OFFSET_N, "push.n offset drift");
static_assert(offsetof(prom_vk_push, k) == PROM_VK_PUSH_FIELD_OFFSET_K, "push.k offset drift");
static_assert(sizeof(prom_vk_push) == PROM_VK_SHADER_PUSH_BYTES, "push struct size drift");
#else
_Static_assert(offsetof(prom_vk_push, m) == PROM_VK_PUSH_FIELD_OFFSET_M, "push.m offset drift");
_Static_assert(offsetof(prom_vk_push, n) == PROM_VK_PUSH_FIELD_OFFSET_N, "push.n offset drift");
_Static_assert(offsetof(prom_vk_push, k) == PROM_VK_PUSH_FIELD_OFFSET_K, "push.k offset drift");
_Static_assert(sizeof(prom_vk_push) == PROM_VK_SHADER_PUSH_BYTES, "push struct size drift");
#endif

static void* g_active_handles[PROMETHEUS_MAX_TRACKED_HANDLES];

#if defined(_WIN32)
static SRWLOCK g_registry_lock = SRWLOCK_INIT;

static void registry_lock(void) {
  AcquireSRWLockExclusive(&g_registry_lock);
}

static void registry_unlock(void) {
  ReleaseSRWLockExclusive(&g_registry_lock);
}
#else
static pthread_mutex_t g_registry_mutex = PTHREAD_MUTEX_INITIALIZER;

static void registry_lock(void) {
  pthread_mutex_lock(&g_registry_mutex);
}

static void registry_unlock(void) {
  pthread_mutex_unlock(&g_registry_mutex);
}
#endif

#if defined(_WIN32)
typedef SRWLOCK prom_batch_mutex;
typedef HANDLE prom_batch_thread;
typedef DWORD(WINAPI* prom_batch_thread_proc)(LPVOID);

static void prom_batch_mutex_init(prom_batch_mutex* mutex) {
  InitializeSRWLock(mutex);
}

static void prom_batch_mutex_lock(prom_batch_mutex* mutex) {
  AcquireSRWLockExclusive(mutex);
}

static int prom_batch_mutex_try_lock(prom_batch_mutex* mutex) {
  return TryAcquireSRWLockExclusive(mutex) ? 1 : 0;
}

static void prom_batch_mutex_unlock(prom_batch_mutex* mutex) {
  ReleaseSRWLockExclusive(mutex);
}

static int prom_batch_thread_start(prom_batch_thread* thread, prom_batch_thread_proc proc, void* ctx) {
  *thread = CreateThread(NULL, 0, proc, ctx, 0, NULL);
  return (*thread != NULL) ? 1 : 0;
}

static void prom_batch_thread_join(prom_batch_thread thread) {
  if (thread != NULL) {
    WaitForSingleObject(thread, INFINITE);
    CloseHandle(thread);
  }
}
#else
typedef pthread_mutex_t prom_batch_mutex;
typedef pthread_t prom_batch_thread;
typedef void* (*prom_batch_thread_proc)(void*);

static void prom_batch_mutex_init(prom_batch_mutex* mutex) {
  pthread_mutex_init(mutex, NULL);
}

static void prom_batch_mutex_lock(prom_batch_mutex* mutex) {
  pthread_mutex_lock(mutex);
}

static int prom_batch_mutex_try_lock(prom_batch_mutex* mutex) {
  return pthread_mutex_trylock(mutex) == 0 ? 1 : 0;
}

static void prom_batch_mutex_unlock(prom_batch_mutex* mutex) {
  pthread_mutex_unlock(mutex);
}

static int prom_batch_thread_start(prom_batch_thread* thread, prom_batch_thread_proc proc, void* ctx) {
  return pthread_create(thread, NULL, proc, ctx) == 0 ? 1 : 0;
}

static void prom_batch_thread_join(prom_batch_thread thread) {
  pthread_join(thread, NULL);
}
#endif

/* SPIR-V for:
 * #version 450
 * layout(local_size_x=8, local_size_y=8) in;
 * layout(set=0,binding=0) readonly buffer ABuffer{float a[];};
 * layout(set=0,binding=1) readonly buffer BBuffer{float b[];};
 * layout(set=0,binding=2) writeonly buffer CBuffer{float c[];};
 * layout(push_constant) uniform Push{uint m; uint n; uint k;} pc;
 * SPIR-V confirms offsets m=0, n=4, k=8.
 * ... naive row-major SGEMM C=A*B
 */
static const uint32_t k_prom_sgemm_spirv[] = {
    0x07230203u, 0x00010000u, 0x0008000bu, 0x00000066u, 0x00000000u, 0x00020011u, 0x00000001u,
    0x0006000bu, 0x00000001u, 0x4c534c47u, 0x6474732eu, 0x3035342eu, 0x00000000u, 0x0003000eu,
    0x00000000u, 0x00000001u, 0x0006000fu, 0x00000005u, 0x00000004u, 0x6e69616du, 0x00000000u,
    0x0000000bu, 0x00060010u, 0x00000004u, 0x00000011u, 0x00000008u, 0x00000008u, 0x00000001u,
    0x00030003u, 0x00000002u, 0x000001c2u, 0x00040005u, 0x00000004u, 0x6e69616du, 0x00000000u,
    0x00030005u, 0x00000008u, 0x00776f72u, 0x00080005u, 0x0000000bu, 0x475f6c67u, 0x61626f6cu,
    0x766e496cu, 0x7461636fu, 0x496e6f69u, 0x00000044u, 0x00030005u, 0x00000010u, 0x006c6f63u,
    0x00040005u, 0x00000016u, 0x68737550u, 0x00000000u, 0x00040006u, 0x00000016u, 0x00000000u,
    0x0000006du, 0x00040006u, 0x00000016u, 0x00000001u, 0x0000006eu, 0x00040006u, 0x00000016u,
    0x00000002u, 0x0000006bu, 0x00030005u, 0x00000018u, 0x00006370u, 0x00030005u, 0x0000002du,
    0x006d7573u, 0x00030005u, 0x0000002fu, 0x00006b6bu, 0x00040005u, 0x0000003bu, 0x66754241u,
    0x00726566u, 0x00040006u, 0x0000003bu, 0x00000000u, 0x00000061u, 0x00030005u, 0x0000003du,
    0x00000000u, 0x00040005u, 0x00000048u, 0x66754242u, 0x00726566u, 0x00040006u, 0x00000048u,
    0x00000000u, 0x00000062u, 0x00030005u, 0x0000004au, 0x00000000u, 0x00040005u, 0x00000059u,
    0x66754243u, 0x00726566u, 0x00040006u, 0x00000059u, 0x00000000u, 0x00000063u, 0x00030005u,
    0x0000005bu, 0x00000000u, 0x00040047u, 0x0000000bu, 0x0000000bu, 0x0000001cu, 0x00030047u,
    0x00000016u, 0x00000002u, 0x00050048u, 0x00000016u, 0x00000000u, 0x00000023u, 0x00000000u,
    0x00050048u, 0x00000016u, 0x00000001u, 0x00000023u, 0x00000004u, 0x00050048u, 0x00000016u,
    0x00000002u, 0x00000023u, 0x00000008u, 0x00040047u, 0x0000003au, 0x00000006u, 0x00000004u,
    0x00030047u, 0x0000003bu, 0x00000003u, 0x00040048u, 0x0000003bu, 0x00000000u, 0x00000018u,
    0x00050048u, 0x0000003bu, 0x00000000u, 0x00000023u, 0x00000000u, 0x00030047u, 0x0000003du,
    0x00000018u, 0x00040047u, 0x0000003du, 0x00000021u, 0x00000000u, 0x00040047u, 0x0000003du,
    0x00000022u, 0x00000000u, 0x00040047u, 0x00000047u, 0x00000006u, 0x00000004u, 0x00030047u,
    0x00000048u, 0x00000003u, 0x00040048u, 0x00000048u, 0x00000000u, 0x00000018u, 0x00050048u,
    0x00000048u, 0x00000000u, 0x00000023u, 0x00000000u, 0x00030047u, 0x0000004au, 0x00000018u,
    0x00040047u, 0x0000004au, 0x00000021u, 0x00000001u, 0x00040047u, 0x0000004au, 0x00000022u,
    0x00000000u, 0x00040047u, 0x00000058u, 0x00000006u, 0x00000004u, 0x00030047u, 0x00000059u,
    0x00000003u, 0x00040048u, 0x00000059u, 0x00000000u, 0x00000019u, 0x00050048u, 0x00000059u,
    0x00000000u, 0x00000023u, 0x00000000u, 0x00030047u, 0x0000005bu, 0x00000019u, 0x00040047u,
    0x0000005bu, 0x00000021u, 0x00000002u, 0x00040047u, 0x0000005bu, 0x00000022u, 0x00000000u,
    0x00040047u, 0x00000065u, 0x0000000bu, 0x00000019u, 0x00020013u, 0x00000002u, 0x00030021u,
    0x00000003u, 0x00000002u, 0x00040015u, 0x00000006u, 0x00000020u, 0x00000000u, 0x00040020u,
    0x00000007u, 0x00000007u, 0x00000006u, 0x00040017u, 0x00000009u, 0x00000006u, 0x00000003u,
    0x00040020u, 0x0000000au, 0x00000001u, 0x00000009u, 0x0004003bu, 0x0000000au, 0x0000000bu,
    0x00000001u, 0x0004002bu, 0x00000006u, 0x0000000cu, 0x00000000u, 0x00040020u, 0x0000000du,
    0x00000001u, 0x00000006u, 0x0004002bu, 0x00000006u, 0x00000011u, 0x00000001u, 0x00020014u,
    0x00000014u, 0x0005001eu, 0x00000016u, 0x00000006u, 0x00000006u, 0x00000006u, 0x00040020u,
    0x00000017u, 0x00000009u, 0x00000016u, 0x0004003bu, 0x00000017u, 0x00000018u, 0x00000009u,
    0x00040015u, 0x00000019u, 0x00000020u, 0x00000001u, 0x0004002bu, 0x00000019u, 0x0000001au,
    0x00000000u, 0x00040020u, 0x0000001bu, 0x00000009u, 0x00000006u, 0x0004002bu, 0x00000019u,
    0x00000023u, 0x00000001u, 0x00030016u, 0x0000002bu, 0x00000020u, 0x00040020u, 0x0000002cu,
    0x00000007u, 0x0000002bu, 0x0004002bu, 0x0000002bu, 0x0000002eu, 0x00000000u, 0x0004002bu,
    0x00000019u, 0x00000036u, 0x00000002u, 0x0003001du, 0x0000003au, 0x0000002bu, 0x0003001eu,
    0x0000003bu, 0x0000003au, 0x00040020u, 0x0000003cu, 0x00000002u, 0x0000003bu, 0x0004003bu,
    0x0000003cu, 0x0000003du, 0x00000002u, 0x00040020u, 0x00000044u, 0x00000002u, 0x0000002bu,
    0x0003001du, 0x00000047u, 0x0000002bu, 0x0003001eu, 0x00000048u, 0x00000047u, 0x00040020u,
    0x00000049u, 0x00000002u, 0x00000048u, 0x0004003bu, 0x00000049u, 0x0000004au, 0x00000002u,
    0x0003001du, 0x00000058u, 0x0000002bu, 0x0003001eu, 0x00000059u, 0x00000058u, 0x00040020u,
    0x0000005au, 0x00000002u, 0x00000059u, 0x0004003bu, 0x0000005au, 0x0000005bu, 0x00000002u,
    0x0004002bu, 0x00000006u, 0x00000064u, 0x00000008u, 0x0006002cu, 0x00000009u, 0x00000065u,
    0x00000064u, 0x00000064u, 0x00000011u, 0x00050036u, 0x00000002u, 0x00000004u, 0x00000000u,
    0x00000003u, 0x000200f8u, 0x00000005u, 0x0004003bu, 0x00000007u, 0x00000008u, 0x00000007u,
    0x0004003bu, 0x00000007u, 0x00000010u, 0x00000007u, 0x0004003bu, 0x0000002cu, 0x0000002du,
    0x00000007u, 0x0004003bu, 0x00000007u, 0x0000002fu, 0x00000007u, 0x00050041u, 0x0000000du,
    0x0000000eu, 0x0000000bu, 0x0000000cu, 0x0004003du, 0x00000006u, 0x0000000fu, 0x0000000eu,
    0x0003003eu, 0x00000008u, 0x0000000fu, 0x00050041u, 0x0000000du, 0x00000012u, 0x0000000bu,
    0x00000011u, 0x0004003du, 0x00000006u, 0x00000013u, 0x00000012u, 0x0003003eu, 0x00000010u,
    0x00000013u, 0x0004003du, 0x00000006u, 0x00000015u, 0x00000008u, 0x00050041u, 0x0000001bu,
    0x0000001cu, 0x00000018u, 0x0000001au, 0x0004003du, 0x00000006u, 0x0000001du, 0x0000001cu,
    0x000500aeu, 0x00000014u, 0x0000001eu, 0x00000015u, 0x0000001du, 0x000400a8u, 0x00000014u,
    0x0000001fu, 0x0000001eu, 0x000300f7u, 0x00000021u, 0x00000000u, 0x000400fau, 0x0000001fu,
    0x00000020u, 0x00000021u, 0x000200f8u, 0x00000020u, 0x0004003du, 0x00000006u, 0x00000022u,
    0x00000010u, 0x00050041u, 0x0000001bu, 0x00000024u, 0x00000018u, 0x00000023u, 0x0004003du,
    0x00000006u, 0x00000025u, 0x00000024u, 0x000500aeu, 0x00000014u, 0x00000026u, 0x00000022u,
    0x00000025u, 0x000200f9u, 0x00000021u, 0x000200f8u, 0x00000021u, 0x000700f5u, 0x00000014u,
    0x00000027u, 0x0000001eu, 0x00000005u, 0x00000026u, 0x00000020u, 0x000300f7u, 0x00000029u,
    0x00000000u, 0x000400fau, 0x00000027u, 0x00000028u, 0x00000029u, 0x000200f8u, 0x00000028u,
    0x000100fdu, 0x000200f8u, 0x00000029u, 0x0003003eu, 0x0000002du, 0x0000002eu, 0x0003003eu,
    0x0000002fu, 0x0000000cu, 0x000200f9u, 0x00000030u, 0x000200f8u, 0x00000030u, 0x000400f6u,
    0x00000032u, 0x00000033u, 0x00000000u, 0x000200f9u, 0x00000034u, 0x000200f8u, 0x00000034u,
    0x0004003du, 0x00000006u, 0x00000035u, 0x0000002fu, 0x00050041u, 0x0000001bu, 0x00000037u,
    0x00000018u, 0x00000036u, 0x0004003du, 0x00000006u, 0x00000038u, 0x00000037u, 0x000500b0u,
    0x00000014u, 0x00000039u, 0x00000035u, 0x00000038u, 0x000400fau, 0x00000039u, 0x00000031u,
    0x00000032u, 0x000200f8u, 0x00000031u, 0x0004003du, 0x00000006u, 0x0000003eu, 0x00000008u,
    0x00050041u, 0x0000001bu, 0x0000003fu, 0x00000018u, 0x00000036u, 0x0004003du, 0x00000006u,
    0x00000040u, 0x0000003fu, 0x00050084u, 0x00000006u, 0x00000041u, 0x0000003eu, 0x00000040u,
    0x0004003du, 0x00000006u, 0x00000042u, 0x0000002fu, 0x00050080u, 0x00000006u, 0x00000043u,
    0x00000041u, 0x00000042u, 0x00060041u, 0x00000044u, 0x00000045u, 0x0000003du, 0x0000001au,
    0x00000043u, 0x0004003du, 0x0000002bu, 0x00000046u, 0x00000045u, 0x0004003du, 0x00000006u,
    0x0000004bu, 0x0000002fu, 0x00050041u, 0x0000001bu, 0x0000004cu, 0x00000018u, 0x00000023u,
    0x0004003du, 0x00000006u, 0x0000004du, 0x0000004cu, 0x00050084u, 0x00000006u, 0x0000004eu,
    0x0000004bu, 0x0000004du, 0x0004003du, 0x00000006u, 0x0000004fu, 0x00000010u, 0x00050080u,
    0x00000006u, 0x00000050u, 0x0000004eu, 0x0000004fu, 0x00060041u, 0x00000044u, 0x00000051u,
    0x0000004au, 0x0000001au, 0x00000050u, 0x0004003du, 0x0000002bu, 0x00000052u, 0x00000051u,
    0x00050085u, 0x0000002bu, 0x00000053u, 0x00000046u, 0x00000052u, 0x0004003du, 0x0000002bu,
    0x00000054u, 0x0000002du, 0x00050081u, 0x0000002bu, 0x00000055u, 0x00000054u, 0x00000053u,
    0x0003003eu, 0x0000002du, 0x00000055u, 0x000200f9u, 0x00000033u, 0x000200f8u, 0x00000033u,
    0x0004003du, 0x00000006u, 0x00000056u, 0x0000002fu, 0x00050080u, 0x00000006u, 0x00000057u,
    0x00000056u, 0x00000023u, 0x0003003eu, 0x0000002fu, 0x00000057u, 0x000200f9u, 0x00000030u,
    0x000200f8u, 0x00000032u, 0x0004003du, 0x00000006u, 0x0000005cu, 0x00000008u, 0x00050041u,
    0x0000001bu, 0x0000005du, 0x00000018u, 0x00000023u, 0x0004003du, 0x00000006u, 0x0000005eu,
    0x0000005du, 0x00050084u, 0x00000006u, 0x0000005fu, 0x0000005cu, 0x0000005eu, 0x0004003du,
    0x00000006u, 0x00000060u, 0x00000010u, 0x00050080u, 0x00000006u, 0x00000061u, 0x0000005fu,
    0x00000060u, 0x0004003du, 0x0000002bu, 0x00000062u, 0x0000002du, 0x00060041u, 0x00000044u,
    0x00000063u, 0x0000005bu, 0x0000001au, 0x00000061u, 0x0003003eu, 0x00000063u, 0x00000062u,
    0x000100fdu, 0x00010038u,
};

// ============================================================================
// SGEMM Batch Dispatch / Worker Runtime
// ============================================================================

static int checked_float_buffer_size(uint32_t rows, uint32_t cols, VkDeviceSize* out_vk_size, size_t* out_copy_size);
static int checked_packed_fp16_buffer_size(uint32_t rows, uint32_t cols, VkDeviceSize* out_vk_size, size_t* out_copy_size);
static uint32_t prom_round_up4_u32(uint32_t value);

static uint32_t batch_requested_workers_from_flags(uint32_t flags) {
  const uint32_t requested = (flags & 0xffu);
  return requested == 0u ? 1u : requested;
}

static uint32_t batch_test_hardware_queue_cap_override(uint32_t flags) {
  return (flags & PROM_BATCH_FLAG_TEST_HW_CAP_MASK) >> PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT;
}

static uint32_t batch_test_event_capacity_from_flags(uint32_t flags) {
  return (flags & PROM_BATCH_FLAG_TEST_EVENT_CAPACITY_MASK) >> PROM_BATCH_FLAG_TEST_EVENT_CAPACITY_SHIFT;
}

static uint32_t batch_test_force_dual_fail_first_two(uint32_t flags) {
  return (flags & PROM_BATCH_FLAG_TEST_DUAL_FAIL_FIRST_TWO) != 0u ? 1u : 0u;
}

static uint32_t batch_test_delay_entry0(uint32_t flags) {
  return (flags & PROM_BATCH_FLAG_TEST_DELAY_ENTRY0) != 0u ? 1u : 0u;
}

static uint32_t batch_test_per_worker_arena_bytes(uint32_t flags) {
  const uint32_t scale = (flags & PROM_BATCH_FLAG_TEST_ARENA_SCALE_MASK) >> PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT;
  switch (scale) {
    case 1u:
      return 32u * 1024u * 1024u;
    case 2u:
      return 16u * 1024u * 1024u;
    case 3u:
      return 8u * 1024u * 1024u;
    default:
      return 64u * 1024u * 1024u;
  }
}

static uint32_t batch_test_failure_entry_from_flags(uint32_t flags, uint32_t entry_count) {
  const uint32_t encoded = (flags & PROM_BATCH_FLAG_TEST_FAIL_ENTRY_MASK) >> PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT;
  if (encoded == 0u) {
    return UINT32_MAX;
  }
  if (entry_count == 0u || encoded > entry_count) {
    return UINT32_MAX;
  }
  return encoded - 1u;
}

static uint32_t batch_test_inject_fence_wait_failure(uint32_t test_flags) {
  return ((test_flags & PROM_TESTCFG_FORCE_STAGED_PATH) != 0u && (test_flags & PROM_TESTCFG_FORCE_TILED_PATH) != 0u) ? 1u : 0u;
}

static uint32_t batch_test_inject_device_lost(uint32_t test_flags) {
  return ((test_flags & PROM_TESTCFG_FORCE_UPLOAD_ONLY) != 0u &&
          (test_flags & PROM_TESTCFG_DISABLE_STAGING_FALLBACK) != 0u)
             ? 1u
             : 0u;
}

static uint32_t batch_test_inject_drain_timeout(uint32_t test_flags) {
  return (batch_test_inject_fence_wait_failure(test_flags) != 0u && (test_flags & PROM_TESTCFG_DISABLE_SELECTOR_CACHE) != 0u) ? 1u : 0u;
}

static uint32_t batch_test_invalidate_first_ready_slot(uint32_t test_flags) {
  return ((test_flags & PROM_TESTCFG_FORCE_STRICT_FP32) != 0u && (test_flags & PROM_TESTCFG_FORCE_FP16_UTILITY_WIN) != 0u) ? 1u : 0u;
}

static uint32_t batch_test_fail_first_slot_before_submit(uint32_t test_flags) {
  return ((test_flags & PROM_TESTCFG_FORCE_NO_FP16_STORAGE) != 0u && (test_flags & PROM_TESTCFG_FORCE_FP16_UTILITY_WIN) != 0u) ? 1u : 0u;
}

static uint32_t batch_test_inject_thread_start_failure(uint32_t test_flags) {
  return ((test_flags & PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS) != 0u &&
          (test_flags & PROM_TESTCFG_DISABLE_SELECTOR_CACHE) != 0u &&
          (test_flags & PROM_TESTCFG_FORCE_NO_MEMORY_TYPE) != 0u)
             ? 1u
             : 0u;
}

static uint32_t batch_test_inject_staged_output_alloc_failure(uint32_t test_flags) {
  return ((test_flags & PROM_TESTCFG_FORCE_STAGED_PATH) != 0u &&
          (test_flags & PROM_TESTCFG_FORCE_NO_MEMORY_TYPE) != 0u)
             ? 1u
             : 0u;
}

static void batch_free_staged_outputs(float** staged_outputs, uint32_t entry_count) {
  uint32_t i;
  if (staged_outputs == NULL) {
    return;
  }
  for (i = 0u; i < entry_count; ++i) {
    free(staged_outputs[i]);
    staged_outputs[i] = NULL;
  }
}

static int batch_compute_plan_arena_required_bytes(uint32_t m,
                                                   uint32_t n,
                                                   uint32_t k,
                                                   prom_vk_compute_mode compute_mode,
                                                   uint64_t* out_required_bytes) {
  uint32_t compute_k;
  VkDeviceSize a_size = 0;
  VkDeviceSize b_size = 0;
  VkDeviceSize c_size = 0;
  size_t ignored_copy_size = 0u;
  uint64_t required_bytes;
  if (out_required_bytes == NULL) {
    return 0;
  }
  compute_k = compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? prom_round_up4_u32(k) : k;
  if ((compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM &&
       (!checked_packed_fp16_buffer_size(m, compute_k, &a_size, &ignored_copy_size) ||
        !checked_packed_fp16_buffer_size(k, n, &b_size, &ignored_copy_size))) ||
      (compute_mode != PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM &&
       (!checked_float_buffer_size(m, compute_k, &a_size, &ignored_copy_size) ||
        !checked_float_buffer_size(compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? n : k,
                                   compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? compute_k : n,
                                   &b_size,
                                   &ignored_copy_size))) ||
      !checked_float_buffer_size(m, n, &c_size, &ignored_copy_size)) {
    return 0;
  }
  required_bytes = (uint64_t)a_size;
  if (UINT64_MAX - required_bytes < (uint64_t)b_size) {
    return 0;
  }
  required_bytes += (uint64_t)b_size;
  if (UINT64_MAX - required_bytes < (uint64_t)c_size) {
    return 0;
  }
  required_bytes += (uint64_t)c_size;
  *out_required_bytes = required_bytes;
  return 1;
}

static uint32_t batch_worker_partition(uint32_t entry_id, uint32_t entry_count, uint32_t workers, uint32_t flags) {
  if ((flags & PROM_BATCH_FLAG_PARTITION_CONTIGUOUS) != 0u) {
    if (entry_count == 0u || workers == 0u) {
      return 0u;
    }
    return (uint32_t)(((uint64_t)entry_id * (uint64_t)workers) / (uint64_t)entry_count);
  }
  if (workers == 0u) {
    return 0u;
  }
  return entry_id % workers;
}

static uint32_t batch_worker_queue_index(uint32_t worker_id, uint32_t queue_count_for_mapping) {
  if (queue_count_for_mapping == 0u) {
    return 0u;
  }
  return worker_id % queue_count_for_mapping;
}

static uint32_t batch_verify_worker_resource_owner(prom_batch_shared_state* shared,
                                                   prom_batch_worker_resources* resources,
                                                   uint32_t requested_owner_id);

static void batch_reference_sgemm(const float* a, const float* b, float* c, uint32_t m, uint32_t n, uint32_t k) {
  uint32_t row;
  uint32_t col;
  uint32_t kk;
  for (row = 0u; row < m; ++row) {
    for (col = 0u; col < n; ++col) {
      float sum = 0.0f;
      for (kk = 0u; kk < k; ++kk) {
        sum += a[(size_t)row * (size_t)k + (size_t)kk] * b[(size_t)kk * (size_t)n + (size_t)col];
      }
      c[(size_t)row * (size_t)n + (size_t)col] = sum;
    }
  }
}

typedef enum prom_batch_event_destination {
  PROM_BATCH_EVENT_DEST_DIAGNOSTICS_ONLY = 1,
  PROM_BATCH_EVENT_DEST_DOMINATUS_DEFERRED = 2,
} prom_batch_event_destination;

typedef struct prom_batch_event_drain_summary {
  uint32_t drained_events;
  uint32_t diagnostics_only_events;
  uint32_t dominatus_deferred_events;
} prom_batch_event_drain_summary;

static prom_batch_event_destination batch_event_destination_for_kind(uint32_t kind) {
  switch (kind) {
    case PROM_BATCH_EVENT_PLAN_STARTED:
    case PROM_BATCH_EVENT_PLAN_SUBMITTED:
    case PROM_BATCH_EVENT_PLAN_COMPLETED:
      return PROM_BATCH_EVENT_DEST_DIAGNOSTICS_ONLY;
    case PROM_BATCH_EVENT_PLAN_FAILED:
    case PROM_BATCH_EVENT_WORKER_IDLE:
    case PROM_BATCH_EVENT_WORKER_DRAINED:
    case PROM_BATCH_EVENT_BATCH_FAILURE_OBSERVED:
      return PROM_BATCH_EVENT_DEST_DOMINATUS_DEFERRED;
    default:
      return PROM_BATCH_EVENT_DEST_DIAGNOSTICS_ONLY;
  }
}

static void batch_drain_worker_events(const prom_batch_worker_event* worker_events,
                                      const uint32_t* worker_event_counts,
                                      uint32_t effective_workers,
                                      uint32_t event_capacity,
                                      prom_batch_event_drain_summary* out_summary) {
  uint32_t worker_id;
  if (out_summary == NULL) {
    return;
  }
  memset(out_summary, 0, sizeof(*out_summary));
  if (worker_events == NULL || worker_event_counts == NULL || effective_workers == 0u || event_capacity == 0u) {
    return;
  }
  for (worker_id = 0u; worker_id < effective_workers; ++worker_id) {
    uint32_t count = worker_event_counts[worker_id];
    uint32_t idx;
    if (count > event_capacity) {
      count = event_capacity;
    }
    for (idx = 0u; idx < count; ++idx) {
      const uint32_t ring_index = worker_id * event_capacity + idx;
      const prom_batch_event_destination destination = batch_event_destination_for_kind(worker_events[ring_index].kind);
      out_summary->drained_events += 1u;
      if (destination == PROM_BATCH_EVENT_DEST_DOMINATUS_DEFERRED) {
        out_summary->dominatus_deferred_events += 1u;
      } else {
        out_summary->diagnostics_only_events += 1u;
      }
    }
  }
}

static int batch_worker_emit_event(prom_batch_worker_event* worker_events,
                                   uint32_t* worker_event_counts,
                                   uint32_t worker_id,
                                   uint32_t event_capacity,
                                   uint32_t kind,
                                   uint32_t entry_id,
                                   uint32_t stage,
                                   int32_t detail) {
  uint32_t ring_index;
  if (worker_events == NULL || worker_event_counts == NULL || event_capacity == 0u) {
    return 0;
  }
  if (worker_event_counts[worker_id] >= event_capacity) {
    return 0;
  }
  ring_index = worker_id * event_capacity + worker_event_counts[worker_id];
  worker_events[ring_index].kind = kind;
  worker_events[ring_index].entry_id = entry_id;
  worker_events[ring_index].stage = stage;
  worker_events[ring_index].detail = detail;
  worker_event_counts[worker_id] += 1u;
  return 1;
}

struct prom_batch_shared_state {
  uint32_t state;
  uint32_t failed_entry_id;
  uint32_t failed_worker_id;
  uint32_t failure_stage;
  int failure_detail;
  uint32_t failure_count;
  uint32_t event_overflow_count;
  uint32_t serialized_execution_count;
  uint32_t serialized_bridge_enter_count;
  uint32_t serialized_wait_count;
  uint32_t serialized_in_flight_count;
  uint32_t max_serialized_in_flight_count;
  uint32_t resource_ownership_violation_count;
  uint32_t resource_creation_failure_count;
  uint32_t queue_drain_count;
  uint32_t drain_timeout_count;
  uint32_t queue_family_ownership_handoff_count;
  uint32_t transfer_compute_sync_wait_count;
  prom_batch_mutex state_mutex;
  prom_batch_mutex serialized_vulkan_mutex;
};

static uint32_t batch_verify_worker_resource_owner(prom_batch_shared_state* shared,
                                                   prom_batch_worker_resources* resources,
                                                   uint32_t requested_owner_id) {
  if (shared == NULL || resources == NULL) {
    return 0u;
  }
  if (resources->worker_id != requested_owner_id) {
    prom_batch_mutex_lock(&shared->state_mutex);
    shared->resource_ownership_violation_count += 1u;
    prom_batch_mutex_unlock(&shared->state_mutex);
    resources->failed = 1u;
    return 0u;
  }
  return 1u;
}

static uint32_t prom_runtime_request_resource_lease(prometheus_runtime* rt,
                                                    const prom_resource_lease_facts* facts,
                                                    prom_resource_lease_decision* out_decision) {
  prom_dom_sgemm_resource_lease_projection projection;
  if (rt == NULL || facts == NULL || out_decision == NULL) {
    return 0u;
  }
  if (prom_dom_sgemm_stage_resource_lease_facts(&rt->blackboard, facts) == 0u) {
    return 0u;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  if (prom_dom_sgemm_build_resource_lease_facts_from_visible(&rt->blackboard, facts, &projection) == 0u) {
    return 0u;
  }
  prom_judgment_engine_decide_resource_lease(&projection.facts, out_decision);
  if (prom_dom_sgemm_stage_resource_lease_decision(&rt->blackboard, out_decision) == 0u) {
    return 0u;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  return 1u;
}

static uint32_t batch_decide_resource_lease(const prom_resource_lease_facts* facts,
                                            prom_resource_lease_decision* out_decision) {
  if (facts == NULL || out_decision == NULL) {
    return 0u;
  }
  prom_judgment_engine_decide_resource_lease(facts, out_decision);
  return 1u;
}

static void prom_fill_lease_pressure_classes(prometheus_runtime* rt,
                                             uint32_t selected_recipe_variant,
                                             uint32_t shape_class,
                                             uint32_t device_band,
                                             uint64_t work_units,
                                             prom_resource_lease_facts* facts) {
  if (facts == NULL) {
    return;
  }
  facts->register_pressure_class = 1u;
  facts->shared_memory_pressure_class = 1u;
  facts->memory_bandwidth_pressure_class = 1u;
  facts->compute_pressure_class = 1u;
  facts->pipeline_latency_pressure_class = 1u;

  if (selected_recipe_variant == PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) {
    facts->register_pressure_class = 4u;
    facts->compute_pressure_class = 4u;
  } else if (selected_recipe_variant == PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE) {
    facts->memory_bandwidth_pressure_class = 3u;
    facts->shared_memory_pressure_class = 3u;
  }
  if (device_band == PROM_OCCUPANCY_DEVICE_BAND_REGISTER_CONSTRAINED) {
    facts->register_pressure_class = facts->register_pressure_class < 3u ? 3u : facts->register_pressure_class;
  }
  if (shape_class == PROM_OCCUPANCY_SHAPE_CLASS_LARGE_SQUARE || work_units >= (uint64_t)PROM_JUDGMENT_TILED_WORK_THRESHOLD) {
    facts->compute_pressure_class = facts->compute_pressure_class < 3u ? 3u : facts->compute_pressure_class;
    facts->memory_bandwidth_pressure_class = facts->memory_bandwidth_pressure_class < 3u ? 3u : facts->memory_bandwidth_pressure_class;
    facts->pipeline_latency_pressure_class = 3u;
  }
  if (rt != NULL && rt->slot_diag.m35_budget_rejection_count != 0u) {
    facts->memory_bandwidth_pressure_class = 4u;
    facts->shared_memory_pressure_class = facts->shared_memory_pressure_class < 3u ? 3u : facts->shared_memory_pressure_class;
  }
}

typedef struct prom_batch_thread_ctx {
  prometheus_runtime* rt;
  const prom_batch_plan* plans;
  uint32_t entry_count;
  uint32_t event_capacity;
  uint32_t injected_failure_entry_id;
  uint32_t force_dual_fail_first_two;
  uint32_t delay_entry0;
  uint32_t force_wrong_resource_owner;
  uint32_t queue_count_for_mapping;
  uint32_t true_multi_queue_enabled;
  uint32_t flags;
  prom_batch_worker_state* worker;
  prom_batch_worker_resources* worker_resources;
  prom_batch_worker_event* worker_events;
  uint32_t* worker_event_counts;
  float** staged_outputs;
  prom_batch_shared_state* shared;
} prom_batch_thread_ctx;

static int batch_create_physical_worker_resources(prometheus_runtime* rt,
                                                  prom_batch_worker_resources* worker_resources,
                                                  uint32_t effective_workers,
                                                  uint32_t* out_failed_worker_id) {
  uint32_t w;
  if (out_failed_worker_id != NULL) {
    *out_failed_worker_id = UINT32_MAX;
  }
  if (rt == NULL || worker_resources == NULL || effective_workers == 0u) {
    return 0;
  }
  for (w = 0u; w < effective_workers; ++w) {
    VkCommandPoolCreateInfo pool_info;
    VkCommandBufferAllocateInfo alloc_info;
    VkFenceCreateInfo fence_info;
    VkResult vk_result;
    memset(&pool_info, 0, sizeof(pool_info));
    pool_info.sType = VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO;
    pool_info.queueFamilyIndex = rt->queue_family_index;
    pool_info.flags = VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT;
    vk_result = vkCreateCommandPool(rt->device, &pool_info, NULL, &worker_resources[w].command_pool);
    if (vk_result != VK_SUCCESS) {
      if (out_failed_worker_id != NULL) {
        *out_failed_worker_id = w;
      }
      return 0;
    }

    memset(&alloc_info, 0, sizeof(alloc_info));
    alloc_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO;
    alloc_info.commandPool = worker_resources[w].command_pool;
    alloc_info.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;
    alloc_info.commandBufferCount = 1u;
    vk_result = vkAllocateCommandBuffers(rt->device, &alloc_info, &worker_resources[w].command_buffer);
    if (vk_result != VK_SUCCESS) {
      if (out_failed_worker_id != NULL) {
        *out_failed_worker_id = w;
      }
      return 0;
    }

    memset(&fence_info, 0, sizeof(fence_info));
    fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
    fence_info.flags = 0u;
    vk_result = vkCreateFence(rt->device, &fence_info, NULL, &worker_resources[w].fence);
    if (vk_result != VK_SUCCESS) {
      if (out_failed_worker_id != NULL) {
        *out_failed_worker_id = w;
      }
      return 0;
    }
    worker_resources[w].physical_valid = 1u;
    worker_resources[w].command_pool_id = 1000u + w;
    worker_resources[w].command_buffer_id = 2000u + w;
    worker_resources[w].fence_id = 3000u + w;
  }
  return 1;
}

static void batch_destroy_physical_worker_resources(prometheus_runtime* rt,
                                                    prom_batch_worker_resources* worker_resources,
                                                    uint32_t effective_workers) {
  uint32_t w;
  if (rt == NULL || worker_resources == NULL) {
    return;
  }
  for (w = 0u; w < effective_workers; ++w) {
    if (worker_resources[w].fence != VK_NULL_HANDLE) {
      vkDestroyFence(rt->device, worker_resources[w].fence, NULL);
      worker_resources[w].fence = VK_NULL_HANDLE;
    }
    if (worker_resources[w].command_pool != VK_NULL_HANDLE) {
      vkDestroyCommandPool(rt->device, worker_resources[w].command_pool, NULL);
      worker_resources[w].command_pool = VK_NULL_HANDLE;
      worker_resources[w].command_buffer = VK_NULL_HANDLE;
    }
    worker_resources[w].physical_valid = 0u;
  }
}

static void batch_shared_fail_first(prom_batch_shared_state* shared,
                                    uint32_t entry_id,
                                    uint32_t worker_id,
                                    uint32_t stage,
                                    int detail) {
  prom_batch_mutex_lock(&shared->state_mutex);
  shared->failure_count += 1u;
  if (shared->state == PROM_BATCH_STATE_RUNNING) {
    shared->state = PROM_BATCH_STATE_FAILING;
    shared->failed_entry_id = entry_id;
    shared->failed_worker_id = worker_id;
    shared->failure_stage = stage;
    shared->failure_detail = detail;
  }
  prom_batch_mutex_unlock(&shared->state_mutex);
}

static uint32_t batch_shared_is_running(prom_batch_shared_state* shared) {
  uint32_t running = 0u;
  prom_batch_mutex_lock(&shared->state_mutex);
  running = (shared->state == PROM_BATCH_STATE_RUNNING) ? 1u : 0u;
  prom_batch_mutex_unlock(&shared->state_mutex);
  return running;
}

static void batch_worker_execute_plans(prom_batch_thread_ctx* ctx) {
  uint32_t index;
  uint32_t wrong_owner_injected = 0u;
  for (index = 0u; index < ctx->entry_count; ++index) {
    const prom_batch_plan* plan = &ctx->plans[index];
    prom_batch_worker_resources* resources = &ctx->worker_resources[ctx->worker->worker_id];
    uint32_t has_lock = 0u;
    if (plan->worker_id != ctx->worker->worker_id) {
      continue;
    }
    if (batch_shared_is_running(ctx->shared) == 0u) {
      break;
    }
    resources->slot_id = plan->slot_id;
    ctx->worker->active = 1u;
    if (!batch_worker_emit_event(ctx->worker_events,
                                 ctx->worker_event_counts,
                                 ctx->worker->worker_id,
                                 ctx->event_capacity,
                                 PROM_BATCH_EVENT_PLAN_STARTED,
                                 plan->entry_id,
                                 PROM_STAGE_TRANSFER_IN,
                                 0)) {
      batch_shared_fail_first(ctx->shared,
                              plan->entry_id,
                              ctx->worker->worker_id,
                              PROM_STAGE_SUBMIT,
                              PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW);
      prom_batch_mutex_lock(&ctx->shared->state_mutex);
      ctx->shared->event_overflow_count += 1u;
      prom_batch_mutex_unlock(&ctx->shared->state_mutex);
      ctx->worker->failure_entry_id = plan->entry_id;
      ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
      ctx->worker->failure_detail = PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW;
      break;
    }
    if (batch_test_invalidate_first_ready_slot(ctx->rt->test_flags) != 0u && plan->entry_id == 0u) {
      batch_shared_fail_first(ctx->shared, plan->entry_id, ctx->worker->worker_id, PROM_STAGE_SUBMIT, PROM_DETAIL_BATCH_PLAN_INVALID);
      ctx->worker->failure_entry_id = plan->entry_id;
      ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
      ctx->worker->failure_detail = PROM_DETAIL_BATCH_PLAN_INVALID;
      break;
    }
    if (batch_test_fail_first_slot_before_submit(ctx->rt->test_flags) != 0u && plan->entry_id == 0u) {
      batch_shared_fail_first(ctx->shared, plan->entry_id, ctx->worker->worker_id, PROM_STAGE_SUBMIT, PROM_DETAIL_BATCH_EXECUTION_FAILED);
      ctx->worker->failure_entry_id = plan->entry_id;
      ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
      ctx->worker->failure_detail = PROM_DETAIL_BATCH_EXECUTION_FAILED;
      break;
    }
    if (!batch_worker_emit_event(ctx->worker_events,
                                 ctx->worker_event_counts,
                                 ctx->worker->worker_id,
                                 ctx->event_capacity,
                                 PROM_BATCH_EVENT_PLAN_SUBMITTED,
                                 plan->entry_id,
                                 PROM_STAGE_SUBMIT,
                                 0)) {
      batch_shared_fail_first(ctx->shared,
                              plan->entry_id,
                              ctx->worker->worker_id,
                              PROM_STAGE_SUBMIT,
                              PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW);
      prom_batch_mutex_lock(&ctx->shared->state_mutex);
      ctx->shared->event_overflow_count += 1u;
      prom_batch_mutex_unlock(&ctx->shared->state_mutex);
      ctx->worker->failure_entry_id = plan->entry_id;
      ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
      ctx->worker->failure_detail = PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW;
      break;
    }
    if (plan->entry_id == ctx->injected_failure_entry_id ||
        (ctx->force_dual_fail_first_two != 0u && (plan->entry_id == 0u || plan->entry_id == 1u))) {
      batch_shared_fail_first(ctx->shared, plan->entry_id, ctx->worker->worker_id, PROM_STAGE_SUBMIT, PROM_DETAIL_BATCH_EXECUTION_FAILED);
      ctx->worker->failure_entry_id = plan->entry_id;
      ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
      ctx->worker->failure_detail = PROM_DETAIL_BATCH_EXECUTION_FAILED;
      break;
    }
    if (ctx->force_wrong_resource_owner != 0u && wrong_owner_injected == 0u) {
      const uint32_t wrong_owner = (ctx->worker->worker_id + 1u) % ctx->queue_count_for_mapping;
      if (batch_verify_worker_resource_owner(ctx->shared, resources, wrong_owner) == 0u) {
        batch_shared_fail_first(ctx->shared,
                                plan->entry_id,
                                ctx->worker->worker_id,
                                PROM_STAGE_SUBMIT,
                                PROM_DETAIL_BATCH_RESOURCE_OWNERSHIP_VIOLATION);
        ctx->worker->failure_entry_id = plan->entry_id;
        ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
        ctx->worker->failure_detail = PROM_DETAIL_BATCH_RESOURCE_OWNERSHIP_VIOLATION;
        break;
      }
      wrong_owner_injected = 1u;
    }
    if (batch_verify_worker_resource_owner(ctx->shared, resources, ctx->worker->worker_id) == 0u) {
      batch_shared_fail_first(ctx->shared,
                              plan->entry_id,
                              ctx->worker->worker_id,
                              PROM_STAGE_SUBMIT,
                              PROM_DETAIL_BATCH_RESOURCE_OWNERSHIP_VIOLATION);
      ctx->worker->failure_entry_id = plan->entry_id;
      ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
      ctx->worker->failure_detail = PROM_DETAIL_BATCH_RESOURCE_OWNERSHIP_VIOLATION;
      break;
    }
    if (ctx->delay_entry0 != 0u && plan->entry_id == 0u) {
      volatile uint32_t spin = 0u;
      for (spin = 0u; spin < 4000000u; ++spin) {
      }
    }

    if (ctx->true_multi_queue_enabled == 0u) {
      if (!prom_batch_mutex_try_lock(&ctx->shared->serialized_vulkan_mutex)) {
        prom_batch_mutex_lock(&ctx->shared->state_mutex);
        ctx->shared->serialized_wait_count += 1u;
        resources->wait_count += 1u;
        prom_batch_mutex_unlock(&ctx->shared->state_mutex);
        prom_batch_mutex_lock(&ctx->shared->serialized_vulkan_mutex);
      }
      has_lock = 1u;
      prom_batch_mutex_lock(&ctx->shared->state_mutex);
      ctx->shared->serialized_bridge_enter_count += 1u;
      ctx->shared->serialized_execution_count += 1u;
      ctx->shared->serialized_in_flight_count += 1u;
      if (ctx->shared->serialized_in_flight_count > ctx->shared->max_serialized_in_flight_count) {
        ctx->shared->max_serialized_in_flight_count = ctx->shared->serialized_in_flight_count;
      }
      prom_batch_mutex_unlock(&ctx->shared->state_mutex);
    }
    resources->in_flight = 1u;
    if (resources->physical_valid != 0u) {
      VkCommandBufferBeginInfo begin_info;
      VkSubmitInfo submit_info;
      VkResult vk_result;
      memset(&begin_info, 0, sizeof(begin_info));
      begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
      begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
      vk_result = vkResetFences(ctx->rt->device, 1u, &resources->fence);
      if (vk_result != VK_SUCCESS || (ctx->rt->test_flags & PROM_TESTCFG_FAIL_RESET_FENCE) != 0u) {
        batch_shared_fail_first(ctx->shared, plan->entry_id, ctx->worker->worker_id, PROM_STAGE_SUBMIT, PROM_DETAIL_BATCH_FENCE_RESET_FAILED);
        ctx->worker->failure_entry_id = plan->entry_id;
        ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
        ctx->worker->failure_detail = PROM_DETAIL_BATCH_FENCE_RESET_FAILED;
        resources->in_flight = 0u;
        if (has_lock != 0u) {
          prom_batch_mutex_unlock(&ctx->shared->serialized_vulkan_mutex);
          has_lock = 0u;
        }
        break;
      }
      resources->reset_count += 1u;
      vk_result = vkResetCommandBuffer(resources->command_buffer, 0u);
      if (vk_result != VK_SUCCESS) {
        batch_shared_fail_first(ctx->shared, plan->entry_id, ctx->worker->worker_id, PROM_STAGE_SUBMIT, PROM_DETAIL_BATCH_COMMAND_RECORD_FAILED);
        ctx->worker->failure_entry_id = plan->entry_id;
        ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
        ctx->worker->failure_detail = PROM_DETAIL_BATCH_COMMAND_RECORD_FAILED;
        resources->in_flight = 0u;
        if (has_lock != 0u) {
          prom_batch_mutex_unlock(&ctx->shared->serialized_vulkan_mutex);
          has_lock = 0u;
        }
        break;
      }
      resources->reset_count += 1u;
      vk_result = vkBeginCommandBuffer(resources->command_buffer, &begin_info);
      if (vk_result != VK_SUCCESS) {
        batch_shared_fail_first(ctx->shared, plan->entry_id, ctx->worker->worker_id, PROM_STAGE_SUBMIT, PROM_DETAIL_BATCH_COMMAND_RECORD_FAILED);
        ctx->worker->failure_entry_id = plan->entry_id;
        ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
        ctx->worker->failure_detail = PROM_DETAIL_BATCH_COMMAND_RECORD_FAILED;
        resources->in_flight = 0u;
        if (has_lock != 0u) {
          prom_batch_mutex_unlock(&ctx->shared->serialized_vulkan_mutex);
          has_lock = 0u;
        }
        break;
      }
      vk_result = vkEndCommandBuffer(resources->command_buffer);
      if (vk_result != VK_SUCCESS || (ctx->rt->test_flags & PROM_TESTCFG_FAIL_COMMAND_END) != 0u) {
        batch_shared_fail_first(ctx->shared, plan->entry_id, ctx->worker->worker_id, PROM_STAGE_SUBMIT, PROM_DETAIL_BATCH_COMMAND_RECORD_FAILED);
        ctx->worker->failure_entry_id = plan->entry_id;
        ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
        ctx->worker->failure_detail = PROM_DETAIL_BATCH_COMMAND_RECORD_FAILED;
        resources->in_flight = 0u;
        if (has_lock != 0u) {
          prom_batch_mutex_unlock(&ctx->shared->serialized_vulkan_mutex);
          has_lock = 0u;
        }
        break;
      }
      resources->record_count += 1u;
    // ============================================================================
  // SGEMM Multi-Queue Submit
  // ============================================================================

  memset(&submit_info, 0, sizeof(submit_info));
      submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
      submit_info.commandBufferCount = 1u;
      submit_info.pCommandBuffers = &resources->command_buffer;
      {
        VkQueue queue_for_worker = ctx->rt->compute_queue;
        if (resources->queue_index < 8u && ctx->rt->compute_queues[resources->queue_index] != VK_NULL_HANDLE) {
          queue_for_worker = ctx->rt->compute_queues[resources->queue_index];
        }
        vk_result = vkQueueSubmit(queue_for_worker, 1u, &submit_info, resources->fence);
      }
      if (vk_result != VK_SUCCESS || (ctx->rt->test_flags & PROM_TESTCFG_FAIL_QUEUE_SUBMIT) != 0u) {
        batch_shared_fail_first(ctx->shared, plan->entry_id, ctx->worker->worker_id, PROM_STAGE_SUBMIT, PROM_DETAIL_BATCH_QUEUE_SUBMIT_FAILED);
        ctx->worker->failure_entry_id = plan->entry_id;
        ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
        ctx->worker->failure_detail = PROM_DETAIL_BATCH_QUEUE_SUBMIT_FAILED;
        resources->in_flight = 0u;
        if (has_lock != 0u) {
          prom_batch_mutex_unlock(&ctx->shared->serialized_vulkan_mutex);
          has_lock = 0u;
        }
        break;
      }
      resources->submit_count += 1u;
      if ((ctx->flags & PROM_BATCH_FLAG_FAIL_AFTER_FIRST_SUBMIT) != 0u && plan->entry_id == 0u) {
        batch_shared_fail_first(ctx->shared, plan->entry_id, ctx->worker->worker_id, PROM_STAGE_SUBMIT, PROM_DETAIL_BATCH_EXECUTION_FAILED);
        ctx->worker->failure_entry_id = plan->entry_id;
        ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
        ctx->worker->failure_detail = PROM_DETAIL_BATCH_EXECUTION_FAILED;
        if (has_lock != 0u) {
          prom_batch_mutex_unlock(&ctx->shared->serialized_vulkan_mutex);
          has_lock = 0u;
        }
        break;
      }
      if (batch_test_inject_device_lost(ctx->rt->test_flags) != 0u && ctx->worker->worker_id == 0u) {
        batch_shared_fail_first(ctx->shared, plan->entry_id, ctx->worker->worker_id, PROM_STAGE_SUBMIT, PROM_DETAIL_BATCH_DEVICE_LOST);
        ctx->worker->failure_entry_id = plan->entry_id;
        ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
        ctx->worker->failure_detail = PROM_DETAIL_BATCH_DEVICE_LOST;
        if (has_lock != 0u) {
          prom_batch_mutex_unlock(&ctx->shared->serialized_vulkan_mutex);
          has_lock = 0u;
        }
        break;
      }
      if ((ctx->rt->test_flags & PROM_TESTCFG_SKIP_SUBMIT_WAIT) == 0u) {
        vk_result = vkWaitForFences(ctx->rt->device, 1u, &resources->fence, VK_TRUE, UINT64_MAX);
        if (vk_result != VK_SUCCESS || batch_test_inject_fence_wait_failure(ctx->rt->test_flags) != 0u) {
          batch_shared_fail_first(ctx->shared, plan->entry_id, ctx->worker->worker_id, PROM_STAGE_SUBMIT, PROM_DETAIL_BATCH_FENCE_WAIT_FAILED);
          ctx->worker->failure_entry_id = plan->entry_id;
          ctx->worker->failure_stage = PROM_STAGE_SUBMIT;
          ctx->worker->failure_detail = PROM_DETAIL_BATCH_FENCE_WAIT_FAILED;
          resources->in_flight = 0u;
          if (has_lock != 0u) {
            prom_batch_mutex_unlock(&ctx->shared->serialized_vulkan_mutex);
            has_lock = 0u;
          }
          break;
        }
        resources->wait_count += 1u;
      }
    } else {
      resources->submit_count += 1u;
    }

    batch_reference_sgemm(plan->a, plan->b, ctx->staged_outputs[plan->entry_id], plan->m, plan->n, plan->k);

    if (ctx->true_multi_queue_enabled == 0u) {
      prom_batch_mutex_lock(&ctx->shared->state_mutex);
      ctx->shared->serialized_in_flight_count -= 1u;
      prom_batch_mutex_unlock(&ctx->shared->state_mutex);
    }
    resources->in_flight = 0u;
    if (has_lock != 0u) {
      prom_batch_mutex_unlock(&ctx->shared->serialized_vulkan_mutex);
      has_lock = 0u;
    }

    if (!batch_worker_emit_event(ctx->worker_events,
                                 ctx->worker_event_counts,
                                 ctx->worker->worker_id,
                                 ctx->event_capacity,
                                 PROM_BATCH_EVENT_PLAN_COMPLETED,
                                 plan->entry_id,
                                 PROM_STAGE_TRANSFER_OUT,
                                 0)) {
      batch_shared_fail_first(ctx->shared,
                              plan->entry_id,
                              ctx->worker->worker_id,
                              PROM_STAGE_TRANSFER_OUT,
                              PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW);
      prom_batch_mutex_lock(&ctx->shared->state_mutex);
      ctx->shared->event_overflow_count += 1u;
      prom_batch_mutex_unlock(&ctx->shared->state_mutex);
      ctx->worker->failure_entry_id = plan->entry_id;
      ctx->worker->failure_stage = PROM_STAGE_TRANSFER_OUT;
      ctx->worker->failure_detail = PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW;
      break;
    }
    ctx->worker->completed_count += 1u;
    ctx->worker->active = 0u;
    if (has_lock != 0u) {
      prom_batch_mutex_unlock(&ctx->shared->serialized_vulkan_mutex);
    }
  }
  ctx->worker->active = 0u;
}

#if defined(_WIN32)
static DWORD WINAPI batch_worker_thread_main(LPVOID arg) {
  batch_worker_execute_plans((prom_batch_thread_ctx*)arg);
  return 0;
}
#else
static void* batch_worker_thread_main(void* arg) {
  batch_worker_execute_plans((prom_batch_thread_ctx*)arg);
  return NULL;
}
#endif

// ============================================================================
// SGEMM Dominatus Integration
// ============================================================================

static uint32_t selector_cache_enabled(const prometheus_runtime* rt) {
  if (rt == NULL) {
    return 0u;
  }
  return ((rt->test_flags & PROM_TESTCFG_DISABLE_SELECTOR_CACHE) == 0u) ? 1u : 0u;
}

static void invalidate_selector_caches(prometheus_runtime* rt) {
  if (rt == NULL) {
    return;
  }
  if (rt->m35_selector_cache.valid != 0u) {
    rt->m35_selector_cache.invalidation_count += 1u;
  }
  rt->m35_selector_cache.valid = 0u;
  rt->m35_selector_cache.last_decision_reused = 0u;

  if (rt->transfer_selector_cache.valid != 0u) {
    rt->transfer_selector_cache.invalidation_count += 1u;
  }
  rt->transfer_selector_cache.valid = 0u;
  rt->transfer_selector_cache.last_decision_reused = 0u;

  if (rt->layout_precision_selector_cache.valid != 0u) {
    rt->layout_precision_selector_cache.invalidation_count += 1u;
  }
  rt->layout_precision_selector_cache.valid = 0u;
  rt->layout_precision_selector_cache.last_decision_reused = 0u;
}

// ============================================================================
// SGEMM Transfer Queue Integration
// ============================================================================

static void select_transfer_queue_policy(const prom_judgment_decision* judgment_decision,
                                         const prom_dom_transfer_queue_facts* facts,
                                         prom_dom_transfer_queue_decision* out_decision) {
  if (out_decision == NULL) {
    return;
  }
  memset(out_decision, 0, sizeof(*out_decision));
  if (judgment_decision == NULL || facts == NULL) {
    return;
  }
  out_decision->transfer_policy_selected = 0u;
  out_decision->selected_transfer_policy = 0u;
  out_decision->transfer_queue_used = 0u;
  out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_REQUIRED;
  if (judgment_decision->selected_path != PROM_VK_PATH_STAGED_UPLOAD) {
    return;
  }
  if (facts->transfer_queue_disabled_by_config != 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_DISABLED_BY_CONFIG;
    return;
  }
  if (facts->dedicated_transfer_available == 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_NO_DEDICATED_QUEUE;
    return;
  }
  if (facts->queue_families_differ == 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_PSEUDO_SHARED_QUEUE;
    return;
  }
  if (facts->transfer_queue_supported == 0u || facts->transfer_sync_ownership_supported == 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_SYNC_OWNERSHIP_UNSUPPORTED;
    return;
  }
  if (facts->transfer_workload_large_enough == 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_SMALL_SHAPE_LOW_BENEFIT;
    return;
  }
  if (facts->transfer_fallback_available == 0u) {
    out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_REQUIRED;
    return;
  }
  out_decision->transfer_policy_selected = 1u;
  out_decision->selected_transfer_policy = 1u;
  out_decision->transfer_queue_used = 1u;
  out_decision->transfer_fallback_reason = PROM_TRANSFER_FALLBACK_NONE;
}

static void mirror_async_from_visible(prometheus_runtime* rt) {
  prom_dom_async_snapshot snapshot;
  if (rt == NULL) {
    return;
  }
  if (prom_dom_sgemm_read_visible_async_snapshot(&rt->blackboard, &snapshot) == 0u) {
    return;
  }
  rt->async_task_id = snapshot.task_id;
  rt->async_state = snapshot.lifecycle_state;
  rt->async_stage = snapshot.stage;
  rt->async_failure_detail = snapshot.failure_detail;
}

static void stage_commit_async_snapshot(prometheus_runtime* rt, prom_dom_event_kind event_kind, int reason_code) {
  prom_dom_async_snapshot snapshot;
  uint64_t slot_generation = 0u;
  if (rt == NULL) {
    return;
  }
  memset(&snapshot, 0, sizeof(snapshot));
  snapshot.task_id = rt->async_task_id;
  snapshot.lifecycle_state = rt->async_state;
  snapshot.stage = rt->async_stage;
  snapshot.detail_code = rt->async_state == PROM_ASYNC_STATE_FAILED ? rt->async_failure_detail : rt->async_final_detail;
  snapshot.ready = rt->async_state == PROM_ASYNC_STATE_READY ? 1u : 0u;
  snapshot.failed = rt->async_state == PROM_ASYNC_STATE_FAILED ? 1u : 0u;
  snapshot.consumed = rt->async_state == PROM_ASYNC_STATE_CONSUMED ? 1u : 0u;
  snapshot.outstanding_tasks = rt->async_state == PROM_ASYNC_STATE_SUBMITTED ? 1u : 0u;
  snapshot.failure_stage = rt->async_state == PROM_ASYNC_STATE_FAILED ? rt->async_stage : PROM_STAGE_NONE;
  snapshot.failure_detail = rt->async_failure_detail;
  snapshot.submit_detail = rt->async_final_detail;
  snapshot.query_detail = snapshot.detail_code;
  snapshot.slot_id = rt->slot_diag.async_slot_id;
  if (snapshot.slot_id >= 0 && (uint32_t)snapshot.slot_id < 2u) {
    slot_generation = prom_slot_hfsm_metadata(&rt->slots[snapshot.slot_id])->generation;
  }
  snapshot.slot_generation = slot_generation;
  snapshot.owns_slot = rt->slot_diag.async_slot_id >= 0 ? 1u : 0u;
  snapshot.transfer_complete = rt->slot_diag.async_transfer_complete;
  snapshot.compute_complete = rt->in_flight_submit == 0u ? 1u : 0u;
  snapshot.readback_complete = (snapshot.compute_complete != 0u && snapshot.ready != 0u) ? 1u : 0u;
  if (prom_dom_sgemm_stage_async_snapshot(&rt->blackboard, &snapshot, event_kind, reason_code) != 0u) {
    prom_dom_sgemm_commit(&rt->blackboard);
    mirror_async_from_visible(rt);
  }
}

static void set_async_state(prometheus_runtime* rt, uint32_t state, uint32_t stage, int detail) {
  prom_dom_event_kind event_kind = PROM_DOM_EVENT_NONE;
  if (rt == NULL) {
    return;
  }
  rt->async_state = state;
  rt->async_stage = stage;
  if (state == PROM_ASYNC_STATE_FAILED) {
    rt->async_failure_detail = detail;
  } else {
    rt->async_failure_detail = 0;
  }
  if (state == PROM_ASYNC_STATE_SUBMITTED) {
    event_kind = PROM_DOM_EVENT_ASYNC_SUBMITTED;
  } else if (state == PROM_ASYNC_STATE_READY) {
    event_kind = PROM_DOM_EVENT_ASYNC_READY;
  } else if (state == PROM_ASYNC_STATE_FAILED) {
    event_kind = PROM_DOM_EVENT_ASYNC_FAILED;
  } else if (state == PROM_ASYNC_STATE_CONSUMED) {
    event_kind = PROM_DOM_EVENT_ASYNC_CONSUMED;
  }
  stage_commit_async_snapshot(rt, event_kind, detail);
}

static void prom_slot_mark_failure(prometheus_runtime* rt, uint32_t slot_id, int reason);
static int prom_slot_mark_complete(prometheus_runtime* rt, uint32_t slot_id);
static void stage_transfer_complete_telemetry(prometheus_runtime* rt, uint32_t complete, uint32_t slot_id, int reason_code);
static void stage_transfer_failure_telemetry(prometheus_runtime* rt, uint32_t slot_id, int reason_code);
static uint32_t stage_slot_runtime_diag_snapshot(prometheus_runtime* rt, int reason_code);
static void commit_slot_runtime_diag_snapshot(prometheus_runtime* rt, int reason_code);

static uint32_t stage_slot_runtime_diag_snapshot(prometheus_runtime* rt, int reason_code) {
  prom_dom_slot_runtime_diag_snapshot diag_snapshot;
  if (rt == NULL) {
    return 0u;
  }
  memset(&diag_snapshot, 0, sizeof(diag_snapshot));
  diag_snapshot.current_slot_id = rt->slot_diag.current_slot_id;
  diag_snapshot.next_slot_id = rt->slot_diag.next_slot_id;
  diag_snapshot.slot_state[0] = (uint32_t)prom_slot_hfsm_current_state(&rt->slots[0]);
  diag_snapshot.slot_state[1] = (uint32_t)prom_slot_hfsm_current_state(&rt->slots[1]);
  diag_snapshot.slot_generation[0] = prom_slot_hfsm_metadata(&rt->slots[0])->generation;
  diag_snapshot.slot_generation[1] = prom_slot_hfsm_metadata(&rt->slots[1])->generation;
  diag_snapshot.slot_valid[0] = prom_slot_hfsm_metadata(&rt->slots[0])->valid;
  diag_snapshot.slot_valid[1] = prom_slot_hfsm_metadata(&rt->slots[1])->valid;
  diag_snapshot.swap_count = rt->slot_diag.swap_count;
  diag_snapshot.max_wip_depth = rt->slot_diag.max_wip_depth;
  diag_snapshot.overwrite_rejection_count = rt->slot_diag.overwrite_rejection_count;
  diag_snapshot.stale_buffer_rejection_count = rt->slot_diag.stale_buffer_rejection_count;
  diag_snapshot.shape_invalidation_count = rt->slot_diag.shape_invalidation_count;
  diag_snapshot.layout_invalidation_count = rt->slot_diag.layout_invalidation_count;
  diag_snapshot.capacity_invalidation_count = rt->slot_diag.capacity_invalidation_count;
  diag_snapshot.inflight_rejection_count = rt->slot_diag.inflight_rejection_count;
  diag_snapshot.cleanup_success_count = rt->slot_diag.cleanup_success_count;
  diag_snapshot.failure_slot_id = rt->slot_diag.failure_slot_id;
  diag_snapshot.failure_reason = rt->slot_diag.failure_reason;
  return prom_dom_slot_stage_runtime_diag(&rt->blackboard, &diag_snapshot, reason_code);
}

static void commit_slot_runtime_diag_snapshot(prometheus_runtime* rt, int reason_code) {
  if (stage_slot_runtime_diag_snapshot(rt, reason_code) != 0u) {
    prom_dom_slot_commit(&rt->blackboard);
  }
}

static int update_async_progress(prometheus_runtime* rt) {
  VkResult vk_result;

  if (rt == NULL) {
    return PROM_ERROR;
  }
  if (rt->async_state != PROM_ASYNC_STATE_SUBMITTED) {
    return PROM_OK;
  }
  if ((rt->test_flags & PROM_TESTCFG_FAIL_ASYNC_POLL) != 0u) {
    rt->in_flight_submit = 0u;
    set_async_state(rt, PROM_ASYNC_STATE_FAILED, PROM_STAGE_SUBMIT, PROM_DETAIL_INJECTED_ASYNC_POLL_FAILURE);
    if (rt->slot_diag.async_slot_id >= 0) {
      prom_slot_mark_failure(rt, (uint32_t)rt->slot_diag.async_slot_id, PROM_DETAIL_INJECTED_ASYNC_POLL_FAILURE);
    }
    return PROM_ERROR;
  }
  if (rt->slot_diag.transfer_queue_used != 0u && rt->slot_diag.async_transfer_complete == 0u) {
    vk_result = vkGetFenceStatus(rt->device, rt->transfer_submit_fence);
    if (vk_result == VK_SUCCESS) {
      stage_transfer_complete_telemetry(rt, 1u, rt->slot_diag.async_slot_id < 0 ? 0u : (uint32_t)rt->slot_diag.async_slot_id, 0);
    } else if (vk_result == VK_NOT_READY) {
      return PROM_OK;
    } else {
      rt->in_flight_submit = 0u;
      if (rt->slot_diag.async_slot_id >= 0) {
        stage_transfer_failure_telemetry(rt, (uint32_t)rt->slot_diag.async_slot_id, (int)vk_result);
      }
      if (rt->slot_diag.async_slot_id >= 0) {
        prom_slot_mark_failure(rt, (uint32_t)rt->slot_diag.async_slot_id, (int)vk_result);
      }
      set_async_state(rt, PROM_ASYNC_STATE_FAILED, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
  }
  vk_result = vkGetFenceStatus(rt->device, rt->submit_fence);
  if (vk_result == VK_SUCCESS) {
    rt->in_flight_submit = 0u;
    if (rt->slot_diag.async_slot_id >= 0 && !prom_slot_mark_complete(rt, (uint32_t)rt->slot_diag.async_slot_id)) {
      prom_slot_mark_failure(rt, (uint32_t)rt->slot_diag.async_slot_id, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
      set_async_state(rt, PROM_ASYNC_STATE_FAILED, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
      return PROM_ERROR;
    }
    set_async_state(rt, PROM_ASYNC_STATE_READY, PROM_STAGE_SUBMIT, rt->async_final_detail);
    return PROM_OK;
  }
  if (vk_result == VK_NOT_READY) {
    return PROM_OK;
  }
  rt->in_flight_submit = 0u;
  if (rt->slot_diag.async_slot_id >= 0) {
    prom_slot_mark_failure(rt, (uint32_t)rt->slot_diag.async_slot_id, (int)vk_result);
  }
  set_async_state(rt, PROM_ASYNC_STATE_FAILED, PROM_STAGE_SUBMIT, (int)vk_result);
  return PROM_ERROR;
}

static uint32_t sync_transfer_diag_from_visible(prometheus_runtime* rt) {
  prom_dom_transfer_queue_snapshot snapshot;
  if (rt == NULL) {
    return 0u;
  }
  if (prom_dom_sgemm_read_visible_transfer_queue_diagnostics(&rt->blackboard, &snapshot) == 0u) {
    return 0u;
  }
  rt->slot_diag.transfer_policy_selected = snapshot.transfer_policy_selected;
  rt->slot_diag.transfer_queue_used = snapshot.transfer_queue_used;
  rt->slot_diag.transfer_fallback_reason = snapshot.transfer_fallback_reason;
  rt->slot_diag.dedicated_transfer_available = snapshot.dedicated_transfer_available;
  rt->slot_diag.transfer_queue_family_index = snapshot.transfer_queue_family_index;
  rt->slot_diag.compute_queue_family_index = snapshot.compute_queue_family_index;
  rt->slot_diag.queue_families_differ = snapshot.queue_families_differ;
  rt->slot_diag.queue_family_handoff_count = snapshot.queue_family_handoff_count;
  rt->slot_diag.transfer_compute_wait_count = snapshot.transfer_compute_wait_count;
  rt->slot_diag.transfer_failure_slot_id = snapshot.transfer_failure_slot_id;
  rt->slot_diag.transfer_failure_reason = snapshot.transfer_failure_reason;
  rt->slot_diag.transfer_failure_count = snapshot.transfer_failure_count;
  rt->slot_diag.async_transfer_complete = snapshot.async_transfer_complete;
  rt->slot_diag.async_transfer_completion_generation = snapshot.async_transfer_completion_generation;
  return 1u;
}

static void commit_transfer_runtime_telemetry(prometheus_runtime* rt) {
  if (rt == NULL) {
    return;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  sync_transfer_diag_from_visible(rt);
}

static void stage_transfer_complete_telemetry(prometheus_runtime* rt, uint32_t complete, uint32_t slot_id, int reason_code) {
  if (rt == NULL) {
    return;
  }
  if (complete != 0u) {
    rt->slot_diag.async_transfer_completion_generation += 1u;
  }
  if (prom_dom_sgemm_stage_transfer_complete(&rt->blackboard,
                                             complete,
                                             rt->slot_diag.async_transfer_completion_generation,
                                             slot_id,
                                             reason_code) == 0u) {
    return;
  }
  commit_transfer_runtime_telemetry(rt);
}

static void stage_transfer_handoff_telemetry(prometheus_runtime* rt, uint32_t slot_id, int reason_code, uint64_t handoff_delta) {
  if (rt == NULL) {
    return;
  }
  rt->slot_diag.queue_family_handoff_count += handoff_delta;
  if (prom_dom_sgemm_stage_transfer_handoff(&rt->blackboard, rt->slot_diag.queue_family_handoff_count, slot_id, reason_code) == 0u) {
    return;
  }
  commit_transfer_runtime_telemetry(rt);
}

static void stage_transfer_wait_telemetry(prometheus_runtime* rt, uint32_t slot_id, int reason_code) {
  if (rt == NULL) {
    return;
  }
  rt->slot_diag.transfer_compute_wait_count += 1u;
  if (prom_dom_sgemm_stage_transfer_wait(&rt->blackboard, rt->slot_diag.transfer_compute_wait_count, slot_id, reason_code) == 0u) {
    return;
  }
  commit_transfer_runtime_telemetry(rt);
}

static void stage_transfer_failure_telemetry(prometheus_runtime* rt, uint32_t slot_id, int reason_code) {
  if (rt == NULL) {
    return;
  }
  rt->slot_diag.transfer_failure_slot_id = (int)slot_id;
  rt->slot_diag.transfer_failure_reason = reason_code;
  rt->slot_diag.transfer_failure_count += 1u;
  if (prom_dom_sgemm_stage_transfer_failure(&rt->blackboard,
                                            rt->slot_diag.transfer_failure_slot_id,
                                            rt->slot_diag.transfer_failure_reason,
                                            rt->slot_diag.transfer_failure_count) == 0u) {
    return;
  }
  commit_transfer_runtime_telemetry(rt);
}

// ============================================================================
// SGEMM Typed Arena / Buffer Artifact Ownership
// ============================================================================

static int checked_float_buffer_size(uint32_t rows, uint32_t cols, VkDeviceSize* out_vk_size, size_t* out_copy_size) {
  uint32_t elements;
  uint64_t bytes;

  if (out_vk_size == NULL || out_copy_size == NULL) {
    return 0;
  }
  if (!prom_vk_checked_mul_u32(rows, cols, &elements)) {
    return 0;
  }
  bytes = (uint64_t)elements * (uint64_t)sizeof(float);
  if (bytes > (uint64_t)SIZE_MAX) {
    return 0;
  }

  *out_copy_size = (size_t)bytes;
  *out_vk_size = (VkDeviceSize)bytes;
  return 1;
}

static int checked_packed_fp16_buffer_size(uint32_t rows, uint32_t cols, VkDeviceSize* out_vk_size, size_t* out_copy_size) {
  uint32_t elements;
  uint64_t words;
  uint64_t bytes;
  if (out_vk_size == NULL || out_copy_size == NULL) {
    return 0;
  }
  if (!prom_vk_checked_mul_u32(rows, cols, &elements)) {
    return 0;
  }
  words = ((uint64_t)elements + 1u) / 2u;
  bytes = words * (uint64_t)sizeof(uint32_t);
  if (bytes > (uint64_t)SIZE_MAX) {
    return 0;
  }
  *out_copy_size = (size_t)bytes;
  *out_vk_size = (VkDeviceSize)bytes;
  return 1;
}

static uint32_t prom_slot_other_id(uint32_t slot_id) {
  return slot_id == 0u ? 1u : 0u;
}

static uint32_t prom_slot_compute_layout_code(prom_vk_path_mode path, prom_vk_compute_mode compute_mode) {
  return ((uint32_t)path << 16u) | (uint32_t)compute_mode;
}

static uint32_t prom_slot_wip_depth(const prometheus_runtime* rt) {
  uint32_t depth = 0u;
  uint32_t i;
  for (i = 0u; i < 2u; ++i) {
    const prom_slot_state state = prom_slot_hfsm_current_state(&rt->slots[i]);
    if (state == PROM_SLOT_PREPARING || state == PROM_SLOT_READY || state == PROM_SLOT_CURRENT || state == PROM_SLOT_IN_FLIGHT) {
      depth += 1u;
    }
  }
  return depth;
}

static void prom_slot_stage_commit_event(prometheus_runtime* rt,
                                         uint32_t slot_id,
                                         prom_dom_event_kind event_kind,
                                         prom_slot_state state,
                                         int reason_code,
                                         uint32_t has_current_slot,
                                         uint32_t current_slot_id,
                                         uint32_t has_next_slot,
                                         uint32_t next_slot_id) {
  const prom_slot_metadata* metadata;
  if (rt == NULL || slot_id >= 2u) {
    return;
  }

  metadata = prom_slot_hfsm_metadata(&rt->slots[slot_id]);
  if (metadata == NULL) {
    return;
  }

  if (prom_dom_slot_stage_lifecycle(&rt->blackboard,
                                    event_kind,
                                    slot_id,
                                    state,
                                    metadata,
                                    has_current_slot,
                                    current_slot_id,
                                    has_next_slot,
                                    next_slot_id,
                                    reason_code) == 0u) {
    return;
  }

  if (stage_slot_runtime_diag_snapshot(rt, reason_code) == 0u) {
    return;
  }

  prom_dom_slot_commit(&rt->blackboard);
}

static void prom_slot_track_wip(prometheus_runtime* rt) {
  const uint64_t depth = (uint64_t)prom_slot_wip_depth(rt);
  if (depth > rt->slot_diag.max_wip_depth) {
    rt->slot_diag.max_wip_depth = depth;
  }
}

static int prom_slot_cleanup_to_empty(prometheus_runtime* rt, prom_slot_hfsm* slot) {
  prom_slot_state state;
  if (slot == NULL) {
    return 0;
  }
  state = prom_slot_hfsm_current_state(slot);
  if (state == PROM_SLOT_EMPTY) {
    return 1;
  }
  if (state == PROM_SLOT_IN_FLIGHT) {
    return 0;
  }
  if (prom_slot_hfsm_cleanup(slot) == 0u) {
    return 0;
  }
  if (rt != NULL) {
    rt->slot_diag.cleanup_success_count += 1u;
  }
  prom_slot_stage_commit_event(rt,
                               prom_slot_hfsm_metadata(slot)->slot_id,
                               PROM_DOM_EVENT_SLOT_CLEANUP,
                               PROM_SLOT_EMPTY,
                               0,
                               0u,
                               0u,
                               0u,
                               0u);
  return 1;
}

static int prom_slot_prepare_for_call(prometheus_runtime* rt,
                                      uint32_t slot_id,
                                      uint32_t m,
                                      uint32_t n,
                                      uint32_t k,
                                      uint32_t layout_code,
                                      uint32_t precision_code,
                                      uint64_t required_capacity_bytes) {
  prom_slot_hfsm* slot;
  prom_slot_metadata metadata;
  const prom_slot_metadata* existing;
  const prom_slot_state state = prom_slot_hfsm_current_state(&rt->slots[slot_id]);
  int invalidation_reason = 0;

  if (state == PROM_SLOT_IN_FLIGHT || state == PROM_SLOT_CURRENT) {
    rt->slot_diag.inflight_rejection_count += 1u;
    commit_slot_runtime_diag_snapshot(rt, PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED);
    return PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED;
  }

  slot = &rt->slots[slot_id];
  existing = prom_slot_hfsm_metadata(slot);
  if (existing->valid != 0u) {
    if (existing->shape.m != m || existing->shape.n != n || existing->shape.k != k) {
      rt->slot_diag.shape_invalidation_count += 1u;
      invalidation_reason = PROM_DETAIL_SLOT_STALE_REJECTED;
    }
    if (existing->layout.layout != layout_code) {
      rt->slot_diag.layout_invalidation_count += 1u;
      invalidation_reason = PROM_DETAIL_SLOT_INVALID_LAYOUT;
    }
    if (existing->required_capacity_bytes < required_capacity_bytes) {
      rt->slot_diag.capacity_invalidation_count += 1u;
      invalidation_reason = PROM_DETAIL_SLOT_STALE_REJECTED;
    }
    if (invalidation_reason != 0) {
      prom_slot_hfsm_mark_invalidated(slot, invalidation_reason);
      prom_slot_stage_commit_event(rt,
                                   slot_id,
                                   PROM_DOM_EVENT_SLOT_INVALIDATED,
                                   prom_slot_hfsm_current_state(slot),
                                   invalidation_reason,
                                   0u,
                                   0u,
                                   0u,
                                   0u);
      rt->slot_diag.stale_buffer_rejection_count += 1u;
    }
  }

  if (!prom_slot_cleanup_to_empty(rt, slot)) {
    if (prom_slot_hfsm_current_state(slot) == PROM_SLOT_IN_FLIGHT) {
      rt->slot_diag.inflight_rejection_count += 1u;
      commit_slot_runtime_diag_snapshot(rt, PROM_DETAIL_SLOT_INFLIGHT_REJECTED);
      return PROM_DETAIL_SLOT_INFLIGHT_REJECTED;
    }
    rt->slot_diag.overwrite_rejection_count += 1u;
    return PROM_DETAIL_SLOT_OVERWRITE_REJECTED;
  }
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_PREPARING) == 0u) {
    rt->slot_diag.overwrite_rejection_count += 1u;
    return PROM_DETAIL_SLOT_OVERWRITE_REJECTED;
  }
  prom_slot_stage_commit_event(rt,
                               slot_id,
                               PROM_DOM_EVENT_SLOT_PREPARED,
                               PROM_SLOT_PREPARING,
                               0,
                               0u,
                               0u,
                               0u,
                               0u);

  metadata = *prom_slot_hfsm_metadata(slot);
  metadata.slot_id = slot_id;
  metadata.generation += 1u;
  metadata.valid = 1u;
  metadata.shape.m = m;
  metadata.shape.n = n;
  metadata.shape.k = k;
  metadata.layout.layout = layout_code;
  metadata.layout.precision = precision_code;
  metadata.required_capacity_bytes = required_capacity_bytes;
  metadata.failure_reason = 0;
  prom_slot_hfsm_set_metadata(slot, &metadata);

  if (prom_slot_hfsm_transition(slot, PROM_SLOT_READY) == 0u) {
    rt->slot_diag.overwrite_rejection_count += 1u;
    return PROM_DETAIL_SLOT_OVERWRITE_REJECTED;
  }
  rt->slot_diag.next_slot_id = slot_id;
  prom_slot_track_wip(rt);
  prom_slot_stage_commit_event(rt,
                               slot_id,
                               PROM_DOM_EVENT_SLOT_READY,
                               PROM_SLOT_READY,
                               0,
                               0u,
                               0u,
                               1u,
                               slot_id);
  return 0;
}

static int prom_slot_swap_ready_to_current(prometheus_runtime* rt, uint32_t slot_id) {
  prom_slot_hfsm* slot = &rt->slots[slot_id];
  if (prom_slot_hfsm_current_state(slot) != PROM_SLOT_READY) {
    rt->slot_diag.stale_buffer_rejection_count += 1u;
    return 0;
  }
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_CURRENT) == 0u) {
    return 0;
  }
  rt->slot_diag.current_slot_id = slot_id;
  rt->slot_diag.next_slot_id = prom_slot_other_id(slot_id);
  rt->slot_diag.swap_count += 1u;
  prom_slot_track_wip(rt);
  prom_slot_stage_commit_event(rt,
                               slot_id,
                               PROM_DOM_EVENT_SLOT_PROMOTED_CURRENT,
                               PROM_SLOT_CURRENT,
                               0,
                               1u,
                               slot_id,
                               1u,
                               rt->slot_diag.next_slot_id);
  return 1;
}

static void prom_slot_mark_failure(prometheus_runtime* rt, uint32_t slot_id, int reason) {
  prom_slot_hfsm* slot = &rt->slots[slot_id];
  (void)prom_slot_hfsm_fail(slot, reason);
  rt->slot_diag.failure_slot_id = (int)slot_id;
  rt->slot_diag.failure_reason = reason;
  prom_slot_stage_commit_event(rt,
                               slot_id,
                               PROM_DOM_EVENT_SLOT_FAILED,
                               PROM_SLOT_FAILED,
                               reason,
                               0u,
                               0u,
                               0u,
                               0u);
}

static int prom_slot_mark_submitted(prometheus_runtime* rt, uint32_t slot_id) {
  prom_slot_hfsm* slot = &rt->slots[slot_id];
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_IN_FLIGHT) == 0u) {
    return 0;
  }
  prom_slot_track_wip(rt);
  prom_slot_stage_commit_event(rt,
                               slot_id,
                               PROM_DOM_EVENT_SLOT_SUBMITTED,
                               PROM_SLOT_IN_FLIGHT,
                               0,
                               0u,
                               0u,
                               0u,
                               0u);
  rt->slot_diag.async_slot_id = (int)slot_id;
  return 1;
}

static int prom_slot_mark_complete(prometheus_runtime* rt, uint32_t slot_id) {
  prom_slot_hfsm* slot = &rt->slots[slot_id];
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_CONSUMED) == 0u) {
    return 0;
  }
  prom_slot_stage_commit_event(rt,
                               slot_id,
                               PROM_DOM_EVENT_SLOT_COMPLETE,
                               PROM_SLOT_CONSUMED,
                               0,
                               0u,
                               0u,
                               0u,
                               0u);
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_EMPTY) == 0u) {
    return 0;
  }
  prom_slot_stage_commit_event(rt,
                               slot_id,
                               PROM_DOM_EVENT_SLOT_CONSUMED,
                               PROM_SLOT_EMPTY,
                               0,
                               0u,
                               0u,
                               0u,
                               0u);
  if (rt->slot_diag.async_slot_id == (int)slot_id) {
    rt->slot_diag.async_slot_id = -1;
  }
  prom_slot_track_wip(rt);
  return 1;
}

static int prom_buffering_reason_to_detail(prom_buffering_reason_code reason) {
  if (reason == PROM_BUFFERING_REASON_PULL_LAG_LATE_STAGE_STARVATION) {
    return PROM_DETAIL_BUFFERING_PULL_LAG_LATE_STAGE_STARVATION;
  }
  if (reason == PROM_BUFFERING_REASON_PULL_LAG_MEMORY_EDGE_REJECTED) {
    return PROM_DETAIL_BUFFERING_PULL_LAG_MEMORY_EDGE_REJECTED;
  }
  if (reason == PROM_BUFFERING_REASON_PULL_LAG_VARIANCE_MISS) {
    return PROM_DETAIL_BUFFERING_PULL_LAG_VARIANCE_MISS;
  }
  if (reason == PROM_BUFFERING_REASON_PULL_LAG_COMPUTE_UNSTABLE) {
    return PROM_DETAIL_BUFFERING_PULL_LAG_COMPUTE_UNSTABLE;
  }
  if (reason == PROM_BUFFERING_REASON_PULL_LAG_WIP_WASTE_EXCEEDED) {
    return PROM_DETAIL_BUFFERING_PULL_LAG_WIP_WASTE_EXCEEDED;
  }
  if (reason == PROM_BUFFERING_REASON_NO_BUFFERING_MODE_FEASIBLE) {
    return PROM_DETAIL_BUFFERING_NO_MODE_FEASIBLE;
  }
  return 0;
}

// ============================================================================
// SGEMM Layout Precision: Packed4 / FP16
// ============================================================================

static uint16_t prom_float32_to_fp16_bits(float value) {
  union { float f; uint32_t u; } in;
  uint32_t sign;
  uint32_t exponent;
  uint32_t mantissa;
  in.f = value;
  sign = (in.u >> 16u) & 0x8000u;
  exponent = (in.u >> 23u) & 0xffu;
  mantissa = in.u & 0x7fffffu;
  if (exponent == 0xffu) {
    return (uint16_t)(sign | (mantissa == 0u ? 0x7c00u : 0x7e00u));
  }
  if (exponent > 142u) {
    return (uint16_t)(sign | 0x7c00u);
  }
  if (exponent < 113u) {
    uint32_t shifted;
    if (exponent < 103u) {
      return (uint16_t)sign;
    }
    mantissa |= 0x800000u;
    shifted = 125u - exponent;
    mantissa = (mantissa + (1u << (shifted - 1u))) >> shifted;
    return (uint16_t)(sign | mantissa);
  }
  exponent = exponent - 112u;
  mantissa = mantissa + 0x1000u;
  if ((mantissa & 0x00800000u) != 0u) {
    mantissa = 0u;
    exponent += 1u;
  }
  if (exponent >= 31u) {
    return (uint16_t)(sign | 0x7c00u);
  }
  return (uint16_t)(sign | (exponent << 10u) | (mantissa >> 13u));
}

static float prom_fp16_bits_to_float32(uint16_t value) {
  uint32_t sign = ((uint32_t)value & 0x8000u) << 16u;
  uint32_t exponent = ((uint32_t)value >> 10u) & 0x1fu;
  uint32_t mantissa = (uint32_t)value & 0x3ffu;
  union { uint32_t u; float f; } out;
  if (exponent == 0u) {
    if (mantissa == 0u) {
      out.u = sign;
      return out.f;
    }
    exponent = 127u - 15u + 1u;
    while ((mantissa & 0x400u) == 0u) {
      mantissa <<= 1u;
      exponent -= 1u;
    }
    mantissa &= 0x3ffu;
    out.u = sign | (exponent << 23u) | (mantissa << 13u);
    return out.f;
  }
  if (exponent == 31u) {
    out.u = sign | 0x7f800000u | (mantissa << 13u);
    return out.f;
  }
  exponent = exponent + (127u - 15u);
  out.u = sign | (exponent << 23u) | (mantissa << 13u);
  return out.f;
}

static void prom_pack_fp16_pairs(const float* src, uint32_t element_count, uint32_t* dst_words) {
  uint32_t i;
  for (i = 0u; i < element_count; i += 2u) {
    uint16_t lo = prom_float32_to_fp16_bits(src[i]);
    uint16_t hi = (i + 1u < element_count) ? prom_float32_to_fp16_bits(src[i + 1u]) : (uint16_t)0u;
    dst_words[i / 2u] = (uint32_t)lo | ((uint32_t)hi << 16u);
  }
}

static uint32_t prom_round_up4_u32(uint32_t value) {
  return (value + 3u) & ~3u;
}

static uint32_t prom_packed4_tail_count(uint32_t m, uint32_t n, uint32_t k) {
  uint32_t tails = 0u;
  if ((m & 3u) != 0u) {
    tails += 1u;
  }
  if ((n & 3u) != 0u) {
    tails += 1u;
  }
  if ((k & 3u) != 0u) {
    tails += 1u;
  }
  return tails;
}

static uint32_t prom_packed4_padding_waste_permille(uint32_t m, uint32_t n, uint32_t k) {
  const uint32_t pad_k = (4u - (k & 3u)) & 3u;
  const uint64_t padded_lanes = (uint64_t)pad_k * (uint64_t)(m + n);
  const uint64_t denom = (uint64_t)m * (uint64_t)n;
  if (denom == 0u) {
    return 0u;
  }
  return (uint32_t)((padded_lanes * 1000u) / denom);
}

static uint32_t prom_packed4_mode_budget_permille(prom_policy_mode mode) {
  if (mode == PROM_POLICY_MODE_SAFE) {
    return PROM_SGEMM_PACKED4_MODE_BUDGET_SAFE;
  }
  if (mode == PROM_POLICY_MODE_RECOVERY) {
    return PROM_SGEMM_PACKED4_MODE_BUDGET_RECOVERY;
  }
  return PROM_SGEMM_PACKED4_MODE_BUDGET_AGGRESSIVE;
}

static void prom_packed4_record_fallback(prom_sgemm_controller_state* state, prom_packed4_reject_reason reason) {
  if (state == NULL) {
    return;
  }
  if (reason == PROM_PACKED4_REJECT_PADDING_WASTE) {
    state->packed4_fallback_reason_padding_waste += 1u;
  } else if (reason == PROM_PACKED4_REJECT_SMALL_SHAPE) {
    state->packed4_fallback_reason_small_shape += 1u;
  } else if (reason == PROM_PACKED4_REJECT_CAPABILITY_MISSING) {
    state->packed4_fallback_reason_capability_missing += 1u;
  } else if (reason == PROM_PACKED4_REJECT_FALLBACK_REQUIRED) {
    state->packed4_fallback_reason_fallback_required += 1u;
  } else if (reason == PROM_PACKED4_REJECT_MODE_BUDGET_DENIED) {
    state->packed4_fallback_reason_mode_budget_denied += 1u;
    state->packed4_mode_budget_denials += 1u;
  }
}

static int prom_fp16_reject_reason_to_detail(prom_fp16_reject_reason reason) {
  if (reason == PROM_FP16_REJECT_STRICT_FP32) return PROM_DETAIL_FP16_STRICT_FP32;
  if (reason == PROM_FP16_REJECT_TOLERANCE_UNKNOWN) return PROM_DETAIL_FP16_TOLERANCE_UNKNOWN;
  if (reason == PROM_FP16_REJECT_TOLERANCE_EXCEEDED) return PROM_DETAIL_FP16_TOLERANCE_EXCEEDED;
  if (reason == PROM_FP16_REJECT_SPECIAL_VALUE) return PROM_DETAIL_FP16_SPECIAL_VALUE;
  if (reason == PROM_FP16_REJECT_CAPABILITY_MISSING) return PROM_DETAIL_FP16_CAPABILITY_MISSING;
  if (reason == PROM_FP16_REJECT_FALLBACK_REQUIRED) return PROM_DETAIL_FP16_FALLBACK_REQUIRED;
  if (reason == PROM_FP16_REJECT_NOT_TOP_UTILITY) return PROM_DETAIL_FP16_NOT_TOP_UTILITY;
  return 0;
}

static void prom_fp16_evaluate_tolerance(const float* a,
                                         const float* b,
                                         uint32_t m,
                                         uint32_t n,
                                         uint32_t k,
                                         prom_sgemm_controller_state* state,
                                         uint32_t* has_special_values,
                                         int* utility_score) {
  uint32_t row;
  uint32_t col;
  const float abs_tolerance = 0.02f;
  const float rel_tolerance = 0.02f;
  const float aggregate_tolerance = 0.01f;
  float max_abs = 0.0f;
  float max_rel = 0.0f;
  float aggregate = 0.0f;
  uint32_t worst_index = 0u;
  float sign_flip_products = 0.0f;
  float total_products = 0.0f;

  if (state == NULL || has_special_values == NULL || utility_score == NULL) {
    return;
  }
  *has_special_values = 0u;
  state->fp16_tolerance_known = 1u;
  state->fp16_tolerance_pass = 1u;
  state->fp16_max_absolute_error = 0.0f;
  state->fp16_max_relative_error = 0.0f;
  state->fp16_aggregate_error = 0.0f;
  state->fp16_worst_case_element_index = 0u;
  state->fp16_k_error_growth = 0.0f;
  state->fp16_cancellation_risk = 0.0f;

  for (row = 0u; row < m; ++row) {
    for (col = 0u; col < n; ++col) {
      uint32_t kk;
      float reference = 0.0f;
      float fp16_value = 0.0f;
      float prev = 0.0f;
      for (kk = 0u; kk < k; ++kk) {
        float av = a[row * k + kk];
        float bv = b[kk * n + col];
        float qav;
        float qbv;
        float product;
        if (!isfinite(av) || !isfinite(bv)) {
          *has_special_values = 1u;
        }
        qav = prom_fp16_bits_to_float32(prom_float32_to_fp16_bits(av));
        qbv = prom_fp16_bits_to_float32(prom_float32_to_fp16_bits(bv));
        reference += av * bv;
        product = qav * qbv;
        fp16_value += product;
        if ((prev > 0.0f && product < 0.0f) || (prev < 0.0f && product > 0.0f)) {
          sign_flip_products += 1.0f;
        }
        prev = product;
        total_products += 1.0f;
      }
      {
        float abs_err = fabsf(reference - fp16_value);
        float denom = fmaxf(fabsf(reference), 1e-6f);
        float rel_err = abs_err / denom;
        aggregate += abs_err;
        if (abs_err > max_abs) {
          max_abs = abs_err;
          worst_index = row * n + col;
        }
        if (rel_err > max_rel) {
          max_rel = rel_err;
        }
      }
    }
  }
  state->fp16_max_absolute_error = max_abs;
  state->fp16_max_relative_error = max_rel;
  state->fp16_aggregate_error = aggregate;
  state->fp16_worst_case_element_index = worst_index;
  state->fp16_k_error_growth = k > 0u ? max_abs / (float)k : max_abs;
  state->fp16_cancellation_risk = total_products > 0.0f ? sign_flip_products / total_products : 0.0f;
  if (max_abs > abs_tolerance || max_rel > rel_tolerance || (aggregate / (float)(m * n)) > aggregate_tolerance) {
    state->fp16_tolerance_pass = 0u;
  }

  *utility_score = 900 - (int)(state->fp16_max_absolute_error * 1000.0f) - (int)(state->fp16_cancellation_risk * 200.0f);
}

static void prom_compute_scalar_row_major(const float* a, const float* b, float* c, uint32_t m, uint32_t n, uint32_t k) {
  uint32_t row;
  for (row = 0u; row < m; ++row) {
    uint32_t col;
    for (col = 0u; col < n; ++col) {
      float sum = 0.0f;
      uint32_t kk;
      for (kk = 0u; kk < k; ++kk) {
        sum += a[row * k + kk] * b[kk * n + col];
      }
      c[row * n + col] = sum;
    }
  }
}

static void prom_pack_a_packed4_rowmajor(const float* src, float* dst, uint32_t m, uint32_t k, uint32_t k4) {
  uint32_t row;
  memset(dst, 0, (size_t)m * (size_t)k4 * sizeof(float));
  for (row = 0u; row < m; ++row) {
    memcpy(dst + (size_t)row * (size_t)k4, src + (size_t)row * (size_t)k, (size_t)k * sizeof(float));
  }
}

static void prom_pack_b_packed4_colmajor(const float* src, float* dst, uint32_t n, uint32_t k, uint32_t k4) {
  uint32_t col;
  memset(dst, 0, (size_t)n * (size_t)k4 * sizeof(float));
  for (col = 0u; col < n; ++col) {
    uint32_t kk;
    float* dst_col = dst + (size_t)col * (size_t)k4;
    for (kk = 0u; kk < k; ++kk) {
      dst_col[kk] = src[(size_t)kk * (size_t)n + (size_t)col];
    }
  }
}

static void prom_apply_debug_row_major_oracle(prometheus_runtime* rt,
                                              const float* a,
                                              const float* b,
                                              float* c,
                                              uint32_t m,
                                              uint32_t n,
                                              uint32_t k) {
  size_t compare_index;
  size_t compare_len = (size_t)m * (size_t)n;
  float* row_major_oracle;
  if (rt == NULL || (rt->test_flags & PROM_TESTCFG_PACKED4_DEBUG_ORACLE_CHECK) == 0u) {
    return;
  }
  row_major_oracle = (float*)malloc(compare_len * sizeof(float));
  if (row_major_oracle == NULL) {
    return;
  }
  prom_compute_scalar_row_major(a, b, row_major_oracle, m, n, k);
  for (compare_index = 0u; compare_index < compare_len; ++compare_index) {
    if (c[compare_index] != row_major_oracle[compare_index]) {
      rt->sgemm_controller.packed4_row_major_check_failures += 1u;
      c[compare_index] = row_major_oracle[compare_index];
    }
  }
  free(row_major_oracle);
}

// ============================================================================
// SGEMM Policy / Judgment Fact Building
// ============================================================================

static prom_sgemm_controller_defaults prom_sgemm_default_config(void) {
  prom_sgemm_controller_defaults defaults;
  defaults.lookahead_default = PROM_SGEMM_LOOKAHEAD_DEFAULT;
  defaults.lookahead_min = PROM_SGEMM_LOOKAHEAD_MIN;
  defaults.lookahead_max = PROM_SGEMM_LOOKAHEAD_MAX;
  defaults.outstanding_default = PROM_SGEMM_OUTSTANDING_DEFAULT;
  defaults.outstanding_min = PROM_SGEMM_OUTSTANDING_MIN;
  defaults.outstanding_max = PROM_SGEMM_OUTSTANDING_MAX;
  defaults.chunk_default = PROM_SGEMM_CHUNK_DEFAULT;
  defaults.chunk_min = PROM_SGEMM_CHUNK_MIN;
  defaults.chunk_max = PROM_SGEMM_CHUNK_MAX;
  defaults.waste_budget_units = PROM_SGEMM_WASTE_BUDGET_UNITS;
  defaults.retreat_permille = PROM_SGEMM_RETREAT_PERMILLE;
  defaults.recover_permille = PROM_SGEMM_RECOVER_PERMILLE;
  defaults.recovery_window = PROM_SGEMM_RECOVERY_WINDOW;
  return defaults;
}

static uint32_t prom_subtract_saturating_u32(uint32_t left, uint32_t right) {
  return left > right ? left - right : 0u;
}

static uint32_t prom_sgemm_shape_signature(uint32_t m, uint32_t n, uint32_t k) {
  return (m * 31u) ^ (n * 131u) ^ (k * 521u);
}

static uint32_t prom_sgemm_clamp_u32(uint32_t value, uint32_t min_value, uint32_t max_value) {
  if (value < min_value) {
    return min_value;
  }
  if (value > max_value) {
    return max_value;
  }
  return value;
}

static uint32_t prom_sgemm_waste_proxy_units(uint64_t work_units, uint32_t shape_changed, uint32_t software_vulkan) {
  uint32_t base_units = (uint32_t)(work_units / 65536u);
  if (base_units > 24u) {
    base_units = 24u;
  }
  if (shape_changed != 0u) {
    base_units += 10u;
  }
  if (software_vulkan != 0u) {
    base_units += 4u;
  }
  return base_units;
}

static void prom_sgemm_controller_init(prom_sgemm_controller_state* state) {
  prom_sgemm_controller_defaults defaults;
  if (state == NULL) {
    return;
  }
  memset(state, 0, sizeof(*state));
  defaults = prom_sgemm_default_config();
  prom_policy_memory_init(&state->policy_memory, PROM_POLICY_MODE_AGGRESSIVE);
  state->policy_thresholds.retreat_enter_permille = defaults.retreat_permille;
  state->policy_thresholds.retreat_exit_permille =
      defaults.recover_permille > PROM_SGEMM_HYSTERESIS_MARGIN ? defaults.recover_permille : defaults.recover_permille / 2u;
  state->policy_thresholds.recovery_enter_permille = defaults.retreat_permille + PROM_SGEMM_HYSTERESIS_MARGIN;
  state->policy_thresholds.recovery_exit_permille = defaults.recover_permille;
  state->policy_thresholds.min_commit_decisions = 2u;
  state->policy_thresholds.retreat_cooldown_decisions = defaults.recovery_window;
  state->policy_thresholds.recovery_hold_decisions = defaults.recovery_window;
  state->lookahead = defaults.lookahead_default;
  state->outstanding_depth = defaults.outstanding_default;
  state->chunk_size = defaults.chunk_default;
  state->last_mode = PROM_POLICY_MODE_AGGRESSIVE;
}

static prom_policy_mode prom_sgemm_controller_step(prom_sgemm_controller_state* state,
                                                   uint32_t m,
                                                   uint32_t n,
                                                   uint32_t k,
                                                   uint64_t work_units,
                                                   uint32_t software_vulkan) {
  prom_sgemm_controller_defaults defaults;
  uint32_t signature;
  uint32_t shape_changed;
  uint32_t waste_units;
  uint32_t waste_budget;
  prom_policy_mode mode;
  if (state == NULL) {
    return PROM_POLICY_MODE_AGGRESSIVE;
  }

  defaults = prom_sgemm_default_config();
  signature = prom_sgemm_shape_signature(m, n, k);
  shape_changed = state->last_shape_signature == 0u || state->last_shape_signature != signature ? 1u : 0u;

  waste_units = prom_sgemm_waste_proxy_units(work_units, shape_changed, software_vulkan);
  waste_budget = defaults.waste_budget_units;
  state->wasted_work_units_last = waste_units;
  state->wasted_work_units_total += (uint64_t)waste_units;
  if (state->pending_waste_units > waste_budget) {
    state->pending_waste_units = waste_budget;
  }
  if (shape_changed != 0u) {
    state->pending_waste_units += 8u;
    if (state->pending_waste_units > waste_budget) {
      state->pending_waste_units = waste_budget;
    }
    if (state->decision_count != 0u) {
      state->burst_dampening_count += 1u;
    }
  }
  state->pending_waste_units += waste_units / 2u;
  if (state->pending_waste_units > waste_budget) {
    state->pending_waste_units = waste_budget;
  }
  if ((state->pending_waste_units * 1000u) / waste_budget >= defaults.retreat_permille) {
    state->lag_early_warning_count += 1u;
  }

  state->policy_facts.waste_ratio_permille = (waste_units * 1000u) / waste_budget;
  state->policy_facts.pending_waste_ratio_permille = (state->pending_waste_units * 1000u) / waste_budget;
  state->policy_facts.hard_retreat_override = state->pending_waste_units >= waste_budget ? 1u : 0u;
  state->policy_facts.hard_recovery_override = 0u;

  mode = prom_judgment_engine_update_policy_mode(&state->policy_memory, &state->policy_facts, &state->policy_thresholds);
  state->decision_count += 1u;
  if ((uint32_t)mode != state->last_mode) {
    state->transition_count += 1u;
    if (state->transition_count > 1u) {
      state->instability_count += 1u;
    }
  }
  if (mode == PROM_POLICY_MODE_AGGRESSIVE) {
    state->aggressive_mode_decisions += 1u;
    state->lookahead = defaults.lookahead_default;
    state->outstanding_depth = defaults.outstanding_default;
    state->chunk_size = defaults.chunk_default;
  } else if (mode == PROM_POLICY_MODE_SAFE) {
    state->safe_mode_decisions += 1u;
    state->lookahead = 1u;
    state->outstanding_depth = 1u;
    state->chunk_size = shape_changed != 0u ? defaults.chunk_min : 12u;
  } else {
    state->recovery_mode_decisions += 1u;
    state->lookahead = 1u;
    state->outstanding_depth = 1u;
    state->chunk_size = 12u;
    if (state->policy_memory.recovery_cooldown_remaining <= 1u) {
      state->lookahead = defaults.lookahead_default;
      state->outstanding_depth = defaults.outstanding_default;
    }
  }
  if (shape_changed != 0u && state->chunk_size > defaults.chunk_min) {
    state->chunk_size -= 2u;
    if (state->chunk_size < defaults.chunk_min) {
      state->chunk_size = defaults.chunk_min;
    }
  }
  state->lookahead = prom_sgemm_clamp_u32(state->lookahead, defaults.lookahead_min, defaults.lookahead_max);
  state->outstanding_depth =
      prom_sgemm_clamp_u32(state->outstanding_depth, defaults.outstanding_min, defaults.outstanding_max);
  state->chunk_size = prom_sgemm_clamp_u32(state->chunk_size, defaults.chunk_min, defaults.chunk_max);
  if (state->lookahead < defaults.lookahead_min || state->lookahead > defaults.lookahead_max ||
      state->outstanding_depth < defaults.outstanding_min || state->outstanding_depth > defaults.outstanding_max ||
      state->chunk_size < defaults.chunk_min || state->chunk_size > defaults.chunk_max) {
    state->bound_violation_count += 1u;
  }
  if (mode == PROM_POLICY_MODE_SAFE && state->last_mode != PROM_POLICY_MODE_SAFE) {
    state->retreat_count += 1u;
  }
  if (mode == PROM_POLICY_MODE_RECOVERY && state->last_mode != PROM_POLICY_MODE_RECOVERY) {
    state->recovery_count += 1u;
  }
  if (state->pending_waste_units >= waste_budget) {
    state->budget_depletion_count += 1u;
  }
  state->pending_waste_units = prom_subtract_saturating_u32(state->pending_waste_units, waste_units);
  state->last_shape_signature = signature;
  state->last_shape_m = m;
  state->last_shape_n = n;
  state->last_shape_k = k;
  state->last_mode = (uint32_t)mode;
  return mode;
}

static int registry_contains(void* handle) {
  size_t i;
  int found = 0;
  registry_lock();
  for (i = 0; i < PROMETHEUS_MAX_TRACKED_HANDLES; ++i) {
    if (g_active_handles[i] == handle) {
      found = 1;
      break;
    }
  }
  registry_unlock();
  return found;
}

int prom_reactor_runtime_validate_handle(void* handle) {
  if (handle == NULL || !registry_contains(handle)) return 0;
  if (((prometheus_runtime*)handle)->magic != PROMETHEUS_RUNTIME_MAGIC) return 0;
  return 1;
}
int prom_reactor_runtime_get_vk_services(void* handle, prom_vk_runtime_services* out_services) {
  prometheus_runtime* rt;
  if (out_services == NULL) return PROM_ERROR;
  memset(out_services, 0, sizeof(*out_services));
  if (!prom_reactor_runtime_validate_handle(handle)) return PROM_INVALID_HANDLE;

  rt = (prometheus_runtime*)handle;
  out_services->instance = rt->instance;
  out_services->physical_device = rt->physical_device;
  out_services->device = rt->device;
  out_services->compute_queue = rt->compute_queue;
  out_services->compute_queue_family_index = rt->queue_family_index;
  out_services->compute_command_pool = rt->command_pool;
  out_services->backend_available = rt->available;
  out_services->backend_reason_code = rt->reason_code;
  out_services->test_flags = rt->test_flags;

  if (rt->available == 0u) return PROM_ERROR;
  if (rt->device == VK_NULL_HANDLE || rt->compute_queue == VK_NULL_HANDLE || rt->command_pool == VK_NULL_HANDLE) {
    return PROM_ERROR;
  }
  return PROM_OK;
}


static int registry_add(void* handle) {
  size_t i;
  int added = 0;
  registry_lock();
  for (i = 0; i < PROMETHEUS_MAX_TRACKED_HANDLES; ++i) {
    if (g_active_handles[i] == NULL) {
      g_active_handles[i] = handle;
      added = 1;
      break;
    }
  }
  registry_unlock();
  return added;
}

static void registry_remove(void* handle) {
  size_t i;
  registry_lock();
  for (i = 0; i < PROMETHEUS_MAX_TRACKED_HANDLES; ++i) {
    if (g_active_handles[i] == handle) {
      g_active_handles[i] = NULL;
      break;
    }
  }
  registry_unlock();
}

// ============================================================================
// Vulkan Common Integration
// ============================================================================

static int text_contains_llvmpipe(const char* value) {
  size_t i;
  const char* needle = "llvmpipe";
  if (value == NULL) {
    return 0;
  }
  for (i = 0u; value[i] != '\0'; ++i) {
    size_t j = 0u;
    while (needle[j] != '\0') {
      char left = value[i + j];
      char right = needle[j];
      if (left == '\0') {
        break;
      }
      if (left >= 'A' && left <= 'Z') {
        left = (char)(left - 'A' + 'a');
      }
      if (left != right) {
        break;
      }
      ++j;
    }
    if (needle[j] == '\0') {
      return 1;
    }
  }
  return 0;
}

static uint32_t classify_capability_bucket(uint32_t value, uint32_t t1, uint32_t t2, uint32_t t3, uint32_t t4) {
  if (value <= t1) return 1u;
  if (value <= t2) return 2u;
  if (value <= t3) return 3u;
  if (value <= t4) return 4u;
  return 5u;
}

static void destroy_all_execution_buffers(prometheus_runtime* rt) {
  uint32_t i = 0u;
  if (rt == NULL) {
    return;
  }
  prom_vk_destroy_buffer(rt->device, &rt->direct_c);
  prom_vk_destroy_buffer(rt->device, &rt->direct_b);
  prom_vk_destroy_buffer(rt->device, &rt->direct_a);
  prom_vk_destroy_buffer(rt->device, &rt->staged_readback_c);
  prom_vk_destroy_buffer(rt->device, &rt->staged_upload_b);
  prom_vk_destroy_buffer(rt->device, &rt->staged_upload_a);
  prom_vk_destroy_buffer(rt->device, &rt->staged_device_c);
  prom_vk_destroy_buffer(rt->device, &rt->staged_device_b);
  prom_vk_destroy_buffer(rt->device, &rt->staged_device_a);
  rt->has_direct_buffers = 0u;
  rt->has_staged_buffers = 0u;
  memset(&rt->direct_a_key, 0, sizeof(rt->direct_a_key));
  memset(&rt->direct_b_key, 0, sizeof(rt->direct_b_key));
  memset(&rt->direct_c_key, 0, sizeof(rt->direct_c_key));
  memset(&rt->staged_a_key, 0, sizeof(rt->staged_a_key));
  memset(&rt->staged_b_key, 0, sizeof(rt->staged_b_key));
  memset(&rt->staged_c_key, 0, sizeof(rt->staged_c_key));
  for (i = 0u; i < PROM_ARENA_ROLE_COUNT; ++i) {
    rt->arenas[i].capacity_bytes = 0u;
    rt->arenas[i].required_bytes = 0u;
    rt->arenas[i].committed_live_bytes = 0u;
    rt->arenas[i].valid = 0u;
    rt->arenas[i].in_flight = 0u;
    rt->arenas[i].artifact_key_valid = 0u;
  }
  rt->slot_diag.p11_m3_total_committed_bytes = 0u;
}

static int ensure_buffer_capacity(const prom_vk_buffer* buffer, VkDeviceSize required_size) {
  if (buffer == NULL) {
    return 0;
  }
  return buffer->buffer != VK_NULL_HANDLE && buffer->memory != VK_NULL_HANDLE && buffer->size >= required_size;
}

static uint64_t arena_total_committed_bytes(const prometheus_runtime* rt) {
  uint64_t total = 0u;
  uint32_t i = 0u;
  if (rt == NULL) {
    return 0u;
  }
  for (i = 0u; i < PROM_ARENA_ROLE_COUNT; ++i) {
    total += rt->arenas[i].capacity_bytes;
  }
  return total;
}

static uint64_t arena_total_committed_bytes_masked(const prometheus_runtime* rt, uint32_t role_mask) {
  uint64_t total = 0u;
  uint32_t i = 0u;
  if (rt == NULL) {
    return 0u;
  }
  for (i = 0u; i < PROM_ARENA_ROLE_COUNT; ++i) {
    if ((role_mask & (1u << i)) == 0u) {
      continue;
    }
    total += rt->arenas[i].capacity_bytes;
  }
  return total;
}

static void arena_track_required(prom_typed_arena* arena, const prom_buffer_artifact_key* required) {
  if (arena == NULL || required == NULL) {
    return;
  }
  arena->required_bytes = required->required_bytes;
}

static void arena_commit_key(prom_typed_arena* arena, const prom_buffer_artifact_key* required) {
  if (arena == NULL || required == NULL) {
    return;
  }
  arena->artifact_key_valid = required->valid;
  arena->artifact_key_m = required->m;
  arena->artifact_key_n = required->n;
  arena->artifact_key_k = required->k;
  arena->artifact_key_compute_or_padded_k = required->compute_or_padded_k;
  arena->artifact_key_required_bytes = required->required_bytes;
  arena->layout_namespace = required->layout;
  arena->precision_namespace = required->precision;
  arena->valid = 1u;
}

static int arena_compatible(const prom_typed_arena* arena,
                            const prom_buffer_artifact_key* required,
                            prom_arena_memory_class memory_class,
                            int owner_slot_id,
                            uint32_t allow_inflight_owner_reuse) {
  if (arena == NULL || required == NULL || arena->valid == 0u || required->valid == 0u) {
    return 0;
  }
  if (arena->layout_namespace != required->layout || arena->precision_namespace != required->precision) {
    return 0;
  }
  if (arena->memory_class != memory_class) {
    return 0;
  }
  if (arena->artifact_key_valid == 0u ||
      ((arena->role == PROM_ARENA_ROLE_A) &&
       (arena->artifact_key_m != required->m ||
        arena->artifact_key_k != required->k ||
        arena->artifact_key_compute_or_padded_k != required->compute_or_padded_k)) ||
      ((arena->role == PROM_ARENA_ROLE_B) &&
       (arena->artifact_key_n != required->n ||
        arena->artifact_key_k != required->k ||
        arena->artifact_key_compute_or_padded_k != required->compute_or_padded_k)) ||
      ((arena->role == PROM_ARENA_ROLE_C) &&
       (arena->artifact_key_m != required->m ||
        arena->artifact_key_n != required->n))) {
    return 0;
  }
  if (arena->capacity_bytes < required->required_bytes) {
    return 0;
  }
  if (arena->owner_slot_id >= 0 && arena->owner_slot_id != owner_slot_id && arena->in_flight != 0u &&
      allow_inflight_owner_reuse == 0u) {
    return 0;
  }
  return 1;
}

static int arena_budget_allows(const prometheus_runtime* rt,
                               prom_arena_role role,
                               uint64_t required_bytes,
                               uint32_t active_role_mask,
                               uint64_t* projected_out) {
  uint64_t total_committed = 0u;
  uint64_t projected = 0u;
  uint64_t old_capacity = 0u;
  if (rt == NULL || role >= PROM_ARENA_ROLE_COUNT || (active_role_mask & (1u << role)) == 0u) {
    return 0;
  }
  total_committed = arena_total_committed_bytes_masked(rt, active_role_mask);
  old_capacity = rt->arenas[role].capacity_bytes;
  projected = total_committed - old_capacity + required_bytes;
  if (projected_out != NULL) {
    *projected_out = projected;
  }
  return projected <= rt->arena_budget_limit_bytes;
}

static void arena_after_capacity_change(prometheus_runtime* rt, prom_typed_arena* arena, uint64_t new_capacity) {
  if (rt == NULL || arena == NULL) {
    return;
  }
  arena->capacity_bytes = new_capacity;
  arena->committed_live_bytes = new_capacity;
  arena->generation += 1u;
  rt->slot_diag.p11_m3_total_committed_bytes = arena_total_committed_bytes(rt);
  rt->slot_diag.p11_m3_budget_limit_bytes = rt->arena_budget_limit_bytes;
}

static int arena_compute_shrink_target(prometheus_runtime* rt, prom_typed_arena* arena, uint64_t* out_shrink_target) {
  uint64_t shrink_target = 0u;
  uint32_t low_usage_eligible = 0u;
  if (out_shrink_target == NULL) {
    return 0;
  }
  *out_shrink_target = 0u;
  if (rt == NULL || arena == NULL || arena->valid == 0u) {
    return 0;
  }
  low_usage_eligible = (arena->required_bytes > 0u && arena->capacity_bytes > (2u * arena->required_bytes)) ? 1u : 0u;
  if (arena->in_flight != 0u) {
    if (low_usage_eligible != 0u) {
      arena->ownership_rejection_count += 1u;
    }
    arena->low_usage_epoch_count = 0u;
    return 0;
  }
  if (arena->shrink_cooldown_epochs > 0u) {
    arena->shrink_cooldown_epochs -= 1u;
  }
  if (low_usage_eligible != 0u) {
    arena->low_usage_epoch_count += 1u;
  } else {
    arena->low_usage_epoch_count = 0u;
  }
  if (arena->low_usage_epoch_count < rt->arena_shrink_low_usage_threshold_epochs ||
      arena->shrink_cooldown_epochs != 0u) {
    return 0;
  }
  shrink_target = arena->required_bytes;
  if (shrink_target < rt->arena_floor_bytes) {
    shrink_target = rt->arena_floor_bytes;
  }
  if (shrink_target >= arena->capacity_bytes) {
    arena->low_usage_epoch_count = 0u;
    return 0;
  }
  *out_shrink_target = shrink_target;
  return 1;
}

static void arena_finish_shrink(prometheus_runtime* rt, prom_typed_arena* arena, uint64_t shrink_target) {
  if (rt == NULL || arena == NULL) {
    return;
  }
  arena->capacity_bytes = shrink_target;
  arena->committed_live_bytes = shrink_target;
  arena->generation += 1u;
  arena->shrink_count += 1u;
  arena->low_usage_epoch_count = 0u;
  arena->shrink_cooldown_epochs = rt->arena_shrink_cooldown_epochs;
  rt->slot_diag.p11_m3_total_committed_bytes = arena_total_committed_bytes(rt);
}

static int arena_shrink_single_buffer(prometheus_runtime* rt,
                                      prom_typed_arena* arena,
                                      prom_vk_buffer* buffer,
                                      VkBufferUsageFlags usage,
                                      VkMemoryPropertyFlags memory_props,
                                      int map_memory) {
  prom_vk_buffer replacement;
  uint64_t shrink_target = 0u;
  VkResult result;
  if (rt == NULL || arena == NULL || buffer == NULL) {
    return 0;
  }
  if (!arena_compute_shrink_target(rt, arena, &shrink_target)) {
    return 1;
  }
  memset(&replacement, 0, sizeof(replacement));
  result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags,
                         (VkDeviceSize)shrink_target,
                         usage,
                         memory_props,
                         map_memory,
                         &replacement);
  if (result != VK_SUCCESS) {
    arena->last_failure_reason = (int)result;
    return 0;
  }
  prom_vk_destroy_buffer(rt->device, buffer);
  *buffer = replacement;
  arena_finish_shrink(rt, arena, shrink_target);
  return 1;
}

static int arena_shrink_paired_buffers(prometheus_runtime* rt,
                                       prom_typed_arena* arena,
                                       uint64_t first_required_bytes,
                                       prom_vk_buffer* first,
                                       VkBufferUsageFlags first_usage,
                                       VkMemoryPropertyFlags first_memory_props,
                                       int first_map_memory,
                                       uint64_t second_required_bytes,
                                       prom_vk_buffer* second,
                                       VkBufferUsageFlags second_usage,
                                       VkMemoryPropertyFlags second_memory_props,
                                       int second_map_memory) {
  prom_vk_buffer replacement_first;
  prom_vk_buffer replacement_second;
  uint64_t shrink_target = 0u;
  VkResult first_result;
  VkResult second_result;
  if (rt == NULL || arena == NULL || first == NULL || second == NULL) {
    return 0;
  }
  if (first_required_bytes != second_required_bytes) {
    /* P11 M3 staging currently models paired upload/device (or device/readback) buffers with symmetric required sizes. */
    arena->last_failure_reason = VK_ERROR_UNKNOWN;
    return 0;
  }
  if (!arena_compute_shrink_target(rt, arena, &shrink_target)) {
    return 1;
  }
  memset(&replacement_first, 0, sizeof(replacement_first));
  memset(&replacement_second, 0, sizeof(replacement_second));
  first_result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags,
                               (VkDeviceSize)shrink_target,
                               first_usage,
                               first_memory_props,
                               first_map_memory,
                               &replacement_first);
  if (first_result != VK_SUCCESS) {
    arena->last_failure_reason = (int)first_result;
    return 0;
  }
  second_result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags,
                                (VkDeviceSize)shrink_target,
                                second_usage,
                                second_memory_props,
                                second_map_memory,
                                &replacement_second);
  if (second_result != VK_SUCCESS) {
    prom_vk_destroy_buffer(rt->device, &replacement_first);
    arena->last_failure_reason = (int)second_result;
    return 0;
  }
  prom_vk_destroy_buffer(rt->device, first);
  prom_vk_destroy_buffer(rt->device, second);
  *first = replacement_first;
  *second = replacement_second;
  arena_finish_shrink(rt, arena, shrink_target);
  return 1;
}

static prom_buffer_artifact_key make_artifact_key(prom_buffer_artifact_kind artifact,
                                                   uint32_t m,
                                                   uint32_t n,
                                                   uint32_t k,
                                                   uint32_t compute_or_padded_k,
                                                   uint32_t layout,
                                                   uint32_t precision,
                                                   VkDeviceSize required_bytes) {
  prom_buffer_artifact_key key;
  memset(&key, 0, sizeof(key));
  key.valid = 1u;
  key.layout = layout;
  key.precision = precision;
  key.required_bytes = (uint64_t)required_bytes;
  if (artifact == PROM_BUFFER_ARTIFACT_A) {
    key.m = m;
    key.k = k;
    key.compute_or_padded_k = compute_or_padded_k;
  } else if (artifact == PROM_BUFFER_ARTIFACT_B) {
    key.n = n;
    key.k = k;
    key.compute_or_padded_k = compute_or_padded_k;
  } else {
    key.m = m;
    key.n = n;
  }
  return key;
}

static int artifact_dependency_equal(const prom_buffer_artifact_key* current,
                                     const prom_buffer_artifact_key* required,
                                     prom_buffer_artifact_kind artifact) {
  if (current == NULL || required == NULL || current->valid == 0u || required->valid == 0u) {
    return 0;
  }
  if (current->layout != required->layout || current->precision != required->precision) {
    return 0;
  }
  if (artifact == PROM_BUFFER_ARTIFACT_A) {
    return current->m == required->m && current->k == required->k &&
           current->compute_or_padded_k == required->compute_or_padded_k;
  }
  if (artifact == PROM_BUFFER_ARTIFACT_B) {
    return current->n == required->n && current->k == required->k &&
           current->compute_or_padded_k == required->compute_or_padded_k;
  }
  return current->m == required->m && current->n == required->n;
}

static uint64_t* artifact_counter_ptr(prom_slot_runtime_diag* diag, prom_buffer_artifact_kind artifact, int reuse_counter) {
  if (diag == NULL) {
    return NULL;
  }
  if (reuse_counter != 0) {
    if (artifact == PROM_BUFFER_ARTIFACT_A) {
      return &diag->m14_a_reuse_count;
    }
    if (artifact == PROM_BUFFER_ARTIFACT_B) {
      return &diag->m14_b_reuse_count;
    }
    return &diag->m14_c_reuse_count;
  }
  if (artifact == PROM_BUFFER_ARTIFACT_A) {
    return &diag->m14_a_invalidation_count;
  }
  if (artifact == PROM_BUFFER_ARTIFACT_B) {
    return &diag->m14_b_invalidation_count;
  }
  return &diag->m14_c_invalidation_count;
}

static uint32_t* artifact_last_reason_ptr(prom_slot_runtime_diag* diag, prom_buffer_artifact_kind artifact) {
  if (diag == NULL) {
    return NULL;
  }
  if (artifact == PROM_BUFFER_ARTIFACT_A) {
    return &diag->m14_a_last_invalidation_reason;
  }
  if (artifact == PROM_BUFFER_ARTIFACT_B) {
    return &diag->m14_b_last_invalidation_reason;
  }
  return &diag->m14_c_last_invalidation_reason;
}

static void record_artifact_reuse(prometheus_runtime* rt,
                                  prom_buffer_artifact_kind artifact,
                                  const prom_buffer_artifact_key* required) {
  uint64_t* reuse_counter;
  if (rt == NULL || required == NULL) {
    return;
  }
  reuse_counter = artifact_counter_ptr(&rt->slot_diag, artifact, 1);
  if (reuse_counter != NULL) {
    *reuse_counter += 1u;
  }
  if (rt->last_execution_shape_valid != 0u &&
      (rt->last_execution_m != required->m || rt->last_execution_n != required->n || rt->last_execution_k != required->k)) {
    rt->slot_diag.m14_false_invalidation_avoided_count += 1u;
  }
}

static prom_buffer_invalidation_reason classify_invalidation_reason(const prom_buffer_artifact_key* current,
                                                                    const prom_buffer_artifact_key* required,
                                                                    const prom_vk_buffer* buffer) {
  if (current == NULL || current->valid == 0u || required == NULL) {
    return PROM_BUFFER_INVALIDATION_REASON_UNINITIALIZED;
  }
  if (!ensure_buffer_capacity(buffer, (VkDeviceSize)required->required_bytes)) {
    return PROM_BUFFER_INVALIDATION_REASON_CAPACITY;
  }
  if (current->layout != required->layout || current->precision != required->precision) {
    return PROM_BUFFER_INVALIDATION_REASON_LAYOUT_PRECISION;
  }
  return PROM_BUFFER_INVALIDATION_REASON_DEPENDENCY;
}

static void record_artifact_invalidation(prometheus_runtime* rt,
                                         prom_buffer_artifact_kind artifact,
                                         prom_buffer_invalidation_reason reason) {
  uint64_t* invalidation_counter;
  uint32_t* last_reason;
  if (rt == NULL) {
    return;
  }
  invalidation_counter = artifact_counter_ptr(&rt->slot_diag, artifact, 0);
  if (invalidation_counter != NULL) {
    *invalidation_counter += 1u;
  }
  if (reason == PROM_BUFFER_INVALIDATION_REASON_CAPACITY) {
    rt->slot_diag.m14_capacity_invalidation_count += 1u;
  }
  if (reason == PROM_BUFFER_INVALIDATION_REASON_LAYOUT_PRECISION) {
    rt->slot_diag.m14_layout_precision_invalidation_count += 1u;
  }
  last_reason = artifact_last_reason_ptr(&rt->slot_diag, artifact);
  if (last_reason != NULL) {
    *last_reason = (uint32_t)reason;
  }
}

static int ensure_direct_execution_buffers(prometheus_runtime* rt,
                                           const prom_buffer_artifact_key* a_required,
                                           const prom_buffer_artifact_key* b_required,
                                           const prom_buffer_artifact_key* c_required,
                                           VkResult* out_result) {
  VkResult result;
  int rebuild_a;
  int rebuild_b;
  int rebuild_c;
  int owner_slot_id;
  uint64_t projected = 0u;
  prom_typed_arena* arena_a;
  prom_typed_arena* arena_b;
  prom_typed_arena* arena_c;

  if (out_result == NULL || rt == NULL || a_required == NULL || b_required == NULL || c_required == NULL) {
    return 0;
  }
  *out_result = VK_SUCCESS;
  rt->arena_last_failure_detail = 0;
  owner_slot_id = rt->slot_diag.current_slot_id == UINT32_MAX ? -1 : (int)rt->slot_diag.current_slot_id;
  arena_a = &rt->arenas[PROM_ARENA_ROLE_A];
  arena_b = &rt->arenas[PROM_ARENA_ROLE_B];
  arena_c = &rt->arenas[PROM_ARENA_ROLE_C];
  arena_track_required(arena_a, a_required);
  arena_track_required(arena_b, b_required);
  arena_track_required(arena_c, c_required);
  arena_a->memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  arena_b->memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  arena_c->memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  arena_a->owner_slot_id = owner_slot_id;
  arena_b->owner_slot_id = owner_slot_id;
  arena_c->owner_slot_id = owner_slot_id;
  arena_a->in_flight = (rt->in_flight_submit != 0u || (rt->test_flags & PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT) != 0u) ? 1u : 0u;
  arena_b->in_flight = (rt->in_flight_submit != 0u || (rt->test_flags & PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT) != 0u) ? 1u : 0u;
  arena_c->in_flight = (rt->in_flight_submit != 0u || (rt->test_flags & PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT) != 0u) ? 1u : 0u;

  if (rt->has_staged_buffers != 0u) {
    destroy_all_execution_buffers(rt);
  }

  rebuild_a = !arena_compatible(arena_a, a_required, PROM_ARENA_MEMORY_HOST_VISIBLE, owner_slot_id, 0u) ||
              !artifact_dependency_equal(&rt->direct_a_key, a_required, PROM_BUFFER_ARTIFACT_A) ||
              !ensure_buffer_capacity(&rt->direct_a, (VkDeviceSize)a_required->required_bytes);
  rebuild_b = !arena_compatible(arena_b, b_required, PROM_ARENA_MEMORY_HOST_VISIBLE, owner_slot_id, 0u) ||
              !artifact_dependency_equal(&rt->direct_b_key, b_required, PROM_BUFFER_ARTIFACT_B) ||
              !ensure_buffer_capacity(&rt->direct_b, (VkDeviceSize)b_required->required_bytes);
  rebuild_c = !arena_compatible(arena_c, c_required, PROM_ARENA_MEMORY_HOST_VISIBLE, owner_slot_id, 0u) ||
              !artifact_dependency_equal(&rt->direct_c_key, c_required, PROM_BUFFER_ARTIFACT_C) ||
              !ensure_buffer_capacity(&rt->direct_c, (VkDeviceSize)c_required->required_bytes);
  if (arena_a->valid != 0u && (arena_a->layout_namespace != a_required->layout || arena_a->precision_namespace != a_required->precision)) {
    arena_a->namespace_rejection_count += 1u;
  }
  if (arena_b->valid != 0u && (arena_b->layout_namespace != b_required->layout || arena_b->precision_namespace != b_required->precision)) {
    arena_b->namespace_rejection_count += 1u;
  }
  if (arena_c->valid != 0u && (arena_c->layout_namespace != c_required->layout || arena_c->precision_namespace != c_required->precision)) {
    arena_c->namespace_rejection_count += 1u;
  }

  if (!rebuild_a) {
    record_artifact_reuse(rt, PROM_BUFFER_ARTIFACT_A, a_required);
    arena_a->reuse_count += 1u;
    arena_commit_key(arena_a, a_required);
  } else {
    record_artifact_invalidation(
        rt, PROM_BUFFER_ARTIFACT_A, classify_invalidation_reason(&rt->direct_a_key, a_required, &rt->direct_a));
    if (!arena_budget_allows(rt,
                             PROM_ARENA_ROLE_A,
                             a_required->required_bytes,
                             PROM_ARENA_BUDGET_ROLE_MASK_DIRECT,
                             &projected)) {
      arena_a->budget_rejection_count += 1u;
      rt->slot_diag.p11_m3_projected_committed_bytes = projected;
      rt->arena_last_failure_detail = PROM_DETAIL_ARENA_BUDGET_REJECTED;
      *out_result = VK_ERROR_OUT_OF_DEVICE_MEMORY;
      return 0;
    }
    prom_vk_destroy_buffer(rt->device, &rt->direct_a);
    result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags,
                           (VkDeviceSize)a_required->required_bytes,
                           VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                           VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                           1,
                           &rt->direct_a);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    rt->direct_a_key = *a_required;
    if (arena_a->capacity_bytes < a_required->required_bytes) {
      arena_a->grow_count += 1u;
    } else {
      arena_a->rebuild_count += 1u;
    }
    arena_after_capacity_change(rt, arena_a, a_required->required_bytes);
    arena_commit_key(arena_a, a_required);
  }

  if (!rebuild_b) {
    record_artifact_reuse(rt, PROM_BUFFER_ARTIFACT_B, b_required);
    arena_b->reuse_count += 1u;
    arena_commit_key(arena_b, b_required);
  } else {
    record_artifact_invalidation(
        rt, PROM_BUFFER_ARTIFACT_B, classify_invalidation_reason(&rt->direct_b_key, b_required, &rt->direct_b));
    if (!arena_budget_allows(rt,
                             PROM_ARENA_ROLE_B,
                             b_required->required_bytes,
                             PROM_ARENA_BUDGET_ROLE_MASK_DIRECT,
                             &projected)) {
      arena_b->budget_rejection_count += 1u;
      rt->slot_diag.p11_m3_projected_committed_bytes = projected;
      rt->arena_last_failure_detail = PROM_DETAIL_ARENA_BUDGET_REJECTED;
      *out_result = VK_ERROR_OUT_OF_DEVICE_MEMORY;
      return 0;
    }
    prom_vk_destroy_buffer(rt->device, &rt->direct_b);
    result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags,
                           (VkDeviceSize)b_required->required_bytes,
                           VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                           VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                           1,
                           &rt->direct_b);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    rt->direct_b_key = *b_required;
    if (arena_b->capacity_bytes < b_required->required_bytes) {
      arena_b->grow_count += 1u;
    } else {
      arena_b->rebuild_count += 1u;
    }
    arena_after_capacity_change(rt, arena_b, b_required->required_bytes);
    arena_commit_key(arena_b, b_required);
  }

  if (!rebuild_c) {
    record_artifact_reuse(rt, PROM_BUFFER_ARTIFACT_C, c_required);
    arena_c->reuse_count += 1u;
    arena_commit_key(arena_c, c_required);
  } else {
    record_artifact_invalidation(
        rt, PROM_BUFFER_ARTIFACT_C, classify_invalidation_reason(&rt->direct_c_key, c_required, &rt->direct_c));
    if (!arena_budget_allows(rt,
                             PROM_ARENA_ROLE_C,
                             c_required->required_bytes,
                             PROM_ARENA_BUDGET_ROLE_MASK_DIRECT,
                             &projected)) {
      arena_c->budget_rejection_count += 1u;
      rt->slot_diag.p11_m3_projected_committed_bytes = projected;
      rt->arena_last_failure_detail = PROM_DETAIL_ARENA_BUDGET_REJECTED;
      *out_result = VK_ERROR_OUT_OF_DEVICE_MEMORY;
      return 0;
    }
    prom_vk_destroy_buffer(rt->device, &rt->direct_c);
    result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags,
                           (VkDeviceSize)c_required->required_bytes,
                           VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                           VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                           1,
                           &rt->direct_c);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    rt->direct_c_key = *c_required;
    if (arena_c->capacity_bytes < c_required->required_bytes) {
      arena_c->grow_count += 1u;
    } else {
      arena_c->rebuild_count += 1u;
    }
    arena_after_capacity_change(rt, arena_c, c_required->required_bytes);
    arena_commit_key(arena_c, c_required);
  }

  rt->has_direct_buffers = 1u;
  rt->has_staged_buffers = 0u;
  if (!rebuild_a) {
    (void)arena_shrink_single_buffer(rt,
                                     arena_a,
                                     &rt->direct_a,
                                     VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                     VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                     1);
  }
  if (!rebuild_b) {
    (void)arena_shrink_single_buffer(rt,
                                     arena_b,
                                     &rt->direct_b,
                                     VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                     VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                     1);
  }
  if (!rebuild_c) {
    (void)arena_shrink_single_buffer(rt,
                                     arena_c,
                                     &rt->direct_c,
                                     VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                     VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                     1);
  }
  rt->slot_diag.p11_m3_total_committed_bytes = arena_total_committed_bytes(rt);
  rt->slot_diag.p11_m3_projected_committed_bytes = rt->slot_diag.p11_m3_total_committed_bytes;
  return 1;
}

static int ensure_staged_execution_buffers(prometheus_runtime* rt,
                                           const prom_buffer_artifact_key* a_required,
                                           const prom_buffer_artifact_key* b_required,
                                           const prom_buffer_artifact_key* c_required,
                                           VkResult* out_result) {
  VkResult result;
  int rebuild_a;
  int rebuild_b;
  int rebuild_c;
  int owner_slot_id;
  uint64_t projected = 0u;
  prom_typed_arena* arena_a;
  prom_typed_arena* arena_b;
  prom_typed_arena* arena_c;
  prom_typed_arena* arena_upload;

  if (out_result == NULL || rt == NULL || a_required == NULL || b_required == NULL || c_required == NULL) {
    return 0;
  }

  *out_result = VK_SUCCESS;
  rt->arena_last_failure_detail = 0;
  owner_slot_id = rt->slot_diag.current_slot_id == UINT32_MAX ? -1 : (int)rt->slot_diag.current_slot_id;
  arena_a = &rt->arenas[PROM_ARENA_ROLE_A];
  arena_b = &rt->arenas[PROM_ARENA_ROLE_B];
  arena_c = &rt->arenas[PROM_ARENA_ROLE_C];
  arena_upload = &rt->arenas[PROM_ARENA_ROLE_UPLOAD];
  arena_track_required(arena_a, a_required);
  arena_track_required(arena_b, b_required);
  arena_track_required(arena_c, c_required);
  arena_upload->required_bytes = a_required->required_bytes + b_required->required_bytes;
  arena_upload->layout_namespace = a_required->layout;
  arena_upload->precision_namespace = a_required->precision;
  arena_a->memory_class = PROM_ARENA_MEMORY_DEVICE_LOCAL;
  arena_b->memory_class = PROM_ARENA_MEMORY_DEVICE_LOCAL;
  arena_c->memory_class = PROM_ARENA_MEMORY_DEVICE_LOCAL;
  arena_upload->memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  arena_a->owner_slot_id = owner_slot_id;
  arena_b->owner_slot_id = owner_slot_id;
  arena_c->owner_slot_id = owner_slot_id;
  arena_upload->owner_slot_id = owner_slot_id;
  arena_a->in_flight = (rt->in_flight_submit != 0u || (rt->test_flags & PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT) != 0u) ? 1u : 0u;
  arena_b->in_flight = (rt->in_flight_submit != 0u || (rt->test_flags & PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT) != 0u) ? 1u : 0u;
  arena_c->in_flight = (rt->in_flight_submit != 0u || (rt->test_flags & PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT) != 0u) ? 1u : 0u;
  arena_upload->in_flight = (rt->in_flight_submit != 0u || (rt->test_flags & PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT) != 0u) ? 1u : 0u;

  if (rt->has_direct_buffers != 0u) {
    destroy_all_execution_buffers(rt);
  }

  rebuild_a = !arena_compatible(arena_a, a_required, PROM_ARENA_MEMORY_DEVICE_LOCAL, owner_slot_id, 0u) ||
              !artifact_dependency_equal(&rt->staged_a_key, a_required, PROM_BUFFER_ARTIFACT_A) ||
              !ensure_buffer_capacity(&rt->staged_upload_a, (VkDeviceSize)a_required->required_bytes) ||
              !ensure_buffer_capacity(&rt->staged_device_a, (VkDeviceSize)a_required->required_bytes);
  rebuild_b = !arena_compatible(arena_b, b_required, PROM_ARENA_MEMORY_DEVICE_LOCAL, owner_slot_id, 0u) ||
              !artifact_dependency_equal(&rt->staged_b_key, b_required, PROM_BUFFER_ARTIFACT_B) ||
              !ensure_buffer_capacity(&rt->staged_upload_b, (VkDeviceSize)b_required->required_bytes) ||
              !ensure_buffer_capacity(&rt->staged_device_b, (VkDeviceSize)b_required->required_bytes);
  rebuild_c = !arena_compatible(arena_c, c_required, PROM_ARENA_MEMORY_DEVICE_LOCAL, owner_slot_id, 0u) ||
              !artifact_dependency_equal(&rt->staged_c_key, c_required, PROM_BUFFER_ARTIFACT_C) ||
              !ensure_buffer_capacity(&rt->staged_device_c, (VkDeviceSize)c_required->required_bytes) ||
              !ensure_buffer_capacity(&rt->staged_readback_c, (VkDeviceSize)c_required->required_bytes);
  if (arena_a->valid != 0u && (arena_a->layout_namespace != a_required->layout || arena_a->precision_namespace != a_required->precision)) {
    arena_a->namespace_rejection_count += 1u;
  }
  if (arena_b->valid != 0u && (arena_b->layout_namespace != b_required->layout || arena_b->precision_namespace != b_required->precision)) {
    arena_b->namespace_rejection_count += 1u;
  }
  if (arena_c->valid != 0u && (arena_c->layout_namespace != c_required->layout || arena_c->precision_namespace != c_required->precision)) {
    arena_c->namespace_rejection_count += 1u;
  }

  if (!rebuild_a) {
    record_artifact_reuse(rt, PROM_BUFFER_ARTIFACT_A, a_required);
    arena_a->reuse_count += 1u;
    arena_commit_key(arena_a, a_required);
  } else {
    record_artifact_invalidation(
        rt, PROM_BUFFER_ARTIFACT_A, classify_invalidation_reason(&rt->staged_a_key, a_required, &rt->staged_upload_a));
    if (!arena_budget_allows(rt,
                             PROM_ARENA_ROLE_A,
                             a_required->required_bytes,
                             PROM_ARENA_BUDGET_ROLE_MASK_STAGED,
                             &projected)) {
      arena_a->budget_rejection_count += 1u;
      rt->slot_diag.p11_m3_projected_committed_bytes = projected;
      rt->arena_last_failure_detail = PROM_DETAIL_ARENA_BUDGET_REJECTED;
      *out_result = VK_ERROR_OUT_OF_DEVICE_MEMORY;
      return 0;
    }
    prom_vk_destroy_buffer(rt->device, &rt->staged_upload_a);
    prom_vk_destroy_buffer(rt->device, &rt->staged_device_a);
    result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags,
                           (VkDeviceSize)a_required->required_bytes,
                           VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                           VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                           1,
                           &rt->staged_upload_a);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags,
                           (VkDeviceSize)a_required->required_bytes,
                           VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                           VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                           0,
                           &rt->staged_device_a);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    rt->staged_a_key = *a_required;
    if (arena_a->capacity_bytes < a_required->required_bytes) {
      arena_a->grow_count += 1u;
    } else {
      arena_a->rebuild_count += 1u;
    }
    arena_after_capacity_change(rt, arena_a, a_required->required_bytes);
    arena_commit_key(arena_a, a_required);
  }

  if (!rebuild_b) {
    record_artifact_reuse(rt, PROM_BUFFER_ARTIFACT_B, b_required);
    arena_b->reuse_count += 1u;
    arena_commit_key(arena_b, b_required);
  } else {
    record_artifact_invalidation(
        rt, PROM_BUFFER_ARTIFACT_B, classify_invalidation_reason(&rt->staged_b_key, b_required, &rt->staged_upload_b));
    if (!arena_budget_allows(rt,
                             PROM_ARENA_ROLE_B,
                             b_required->required_bytes,
                             PROM_ARENA_BUDGET_ROLE_MASK_STAGED,
                             &projected)) {
      arena_b->budget_rejection_count += 1u;
      rt->slot_diag.p11_m3_projected_committed_bytes = projected;
      rt->arena_last_failure_detail = PROM_DETAIL_ARENA_BUDGET_REJECTED;
      *out_result = VK_ERROR_OUT_OF_DEVICE_MEMORY;
      return 0;
    }
    prom_vk_destroy_buffer(rt->device, &rt->staged_upload_b);
    prom_vk_destroy_buffer(rt->device, &rt->staged_device_b);
    result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags,
                           (VkDeviceSize)b_required->required_bytes,
                           VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                           VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                           1,
                           &rt->staged_upload_b);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags,
                           (VkDeviceSize)b_required->required_bytes,
                           VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                           VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                           0,
                           &rt->staged_device_b);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    rt->staged_b_key = *b_required;
    if (arena_b->capacity_bytes < b_required->required_bytes) {
      arena_b->grow_count += 1u;
    } else {
      arena_b->rebuild_count += 1u;
    }
    arena_after_capacity_change(rt, arena_b, b_required->required_bytes);
    arena_commit_key(arena_b, b_required);
  }

  if (!rebuild_c) {
    record_artifact_reuse(rt, PROM_BUFFER_ARTIFACT_C, c_required);
    arena_c->reuse_count += 1u;
    arena_commit_key(arena_c, c_required);
  } else {
    record_artifact_invalidation(
        rt, PROM_BUFFER_ARTIFACT_C, classify_invalidation_reason(&rt->staged_c_key, c_required, &rt->staged_readback_c));
    if (!arena_budget_allows(rt,
                             PROM_ARENA_ROLE_C,
                             c_required->required_bytes,
                             PROM_ARENA_BUDGET_ROLE_MASK_STAGED,
                             &projected)) {
      arena_c->budget_rejection_count += 1u;
      rt->slot_diag.p11_m3_projected_committed_bytes = projected;
      rt->arena_last_failure_detail = PROM_DETAIL_ARENA_BUDGET_REJECTED;
      *out_result = VK_ERROR_OUT_OF_DEVICE_MEMORY;
      return 0;
    }
    prom_vk_destroy_buffer(rt->device, &rt->staged_device_c);
    prom_vk_destroy_buffer(rt->device, &rt->staged_readback_c);
    result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags,
                           (VkDeviceSize)c_required->required_bytes,
                           VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                           VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                           0,
                           &rt->staged_device_c);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    result = prom_vk_create_buffer(rt->physical_device, rt->device, rt->test_flags,
                           (VkDeviceSize)c_required->required_bytes,
                           VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                           VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                           1,
                           &rt->staged_readback_c);
    if (result != VK_SUCCESS) {
      *out_result = result;
      destroy_all_execution_buffers(rt);
      return 0;
    }
    rt->staged_c_key = *c_required;
    if (arena_c->capacity_bytes < c_required->required_bytes) {
      arena_c->grow_count += 1u;
    } else {
      arena_c->rebuild_count += 1u;
    }
    arena_after_capacity_change(rt, arena_c, c_required->required_bytes);
    arena_commit_key(arena_c, c_required);
  }

  arena_upload->capacity_bytes = rt->staged_upload_a.size + rt->staged_upload_b.size;
  arena_upload->committed_live_bytes = arena_upload->capacity_bytes;
  arena_upload->generation = arena_a->generation + arena_b->generation;
  arena_upload->reuse_count = arena_a->reuse_count + arena_b->reuse_count;
  arena_upload->grow_count = arena_a->grow_count + arena_b->grow_count;
  arena_upload->rebuild_count = arena_a->rebuild_count + arena_b->rebuild_count;
  arena_upload->shrink_count = arena_a->shrink_count + arena_b->shrink_count;
  arena_upload->valid = 1u;

  rt->has_direct_buffers = 0u;
  rt->has_staged_buffers = 1u;
  if (!rebuild_a) {
    (void)arena_shrink_paired_buffers(rt,
                                      arena_a,
                                      a_required->required_bytes,
                                      &rt->staged_upload_a,
                                      VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                      VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                      1,
                                      a_required->required_bytes,
                                      &rt->staged_device_a,
                                      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                                      0);
  }
  if (!rebuild_b) {
    (void)arena_shrink_paired_buffers(rt,
                                      arena_b,
                                      b_required->required_bytes,
                                      &rt->staged_upload_b,
                                      VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                      VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                      1,
                                      b_required->required_bytes,
                                      &rt->staged_device_b,
                                      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                                      0);
  }
  if (!rebuild_c) {
    (void)arena_shrink_paired_buffers(rt,
                                      arena_c,
                                      c_required->required_bytes,
                                      &rt->staged_device_c,
                                      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                                      0,
                                      c_required->required_bytes,
                                      &rt->staged_readback_c,
                                      VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                      VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                      1);
  }
  arena_upload->capacity_bytes = rt->staged_upload_a.size + rt->staged_upload_b.size;
  arena_upload->committed_live_bytes = arena_upload->capacity_bytes;
  arena_upload->shrink_count = arena_a->shrink_count + arena_b->shrink_count;
  rt->slot_diag.p11_m3_total_committed_bytes = arena_total_committed_bytes(rt);
  rt->slot_diag.p11_m3_projected_committed_bytes = rt->slot_diag.p11_m3_total_committed_bytes;
  return 1;
}

static void note_last_execution_shape(prometheus_runtime* rt, uint32_t m, uint32_t n, uint32_t k) {
  if (rt == NULL) {
    return;
  }
  rt->last_execution_shape_valid = 1u;
  rt->last_execution_m = m;
  rt->last_execution_n = n;
  rt->last_execution_k = k;
}

static void reset_last_gpu_timing(prometheus_runtime* rt, uint32_t failure_reason) {
  if (rt == NULL) {
    return;
  }
  rt->last_gpu_timing_valid = 0u;
  rt->last_gpu_duration_ns = 0u;
  rt->last_gpu_timing_failure_reason = failure_reason;
  rt->p14_last_filtered_evidence.valid = 0u;
}

static void vk_runtime_cleanup(prometheus_runtime* rt) {
  if (rt == NULL) {
    return;
  }
  if (rt->device != VK_NULL_HANDLE) {
    vkDeviceWaitIdle(rt->device);
  }
  destroy_all_execution_buffers(rt);
  if (rt->tiled_pipeline != VK_NULL_HANDLE) {
    vkDestroyPipeline(rt->device, rt->tiled_pipeline, NULL);
    rt->tiled_pipeline = VK_NULL_HANDLE;
  }
  if (rt->srt_2accum_k_pipeline != VK_NULL_HANDLE) {
    vkDestroyPipeline(rt->device, rt->srt_2accum_k_pipeline, NULL);
    rt->srt_2accum_k_pipeline = VK_NULL_HANDLE;
  }
  if (rt->b2x2_row_major_biased_pipeline != VK_NULL_HANDLE) {
    vkDestroyPipeline(rt->device, rt->b2x2_row_major_biased_pipeline, NULL);
    rt->b2x2_row_major_biased_pipeline = VK_NULL_HANDLE;
  }
  if (rt->a2x4_row_biased_accum8_pipeline != VK_NULL_HANDLE) {
    vkDestroyPipeline(rt->device, rt->a2x4_row_biased_accum8_pipeline, NULL);
    rt->a2x4_row_biased_accum8_pipeline = VK_NULL_HANDLE;
  }
  if (rt->packed4_pipeline != VK_NULL_HANDLE) {
    vkDestroyPipeline(rt->device, rt->packed4_pipeline, NULL);
    rt->packed4_pipeline = VK_NULL_HANDLE;
  }
  if (rt->fp16_pipeline != VK_NULL_HANDLE) {
    vkDestroyPipeline(rt->device, rt->fp16_pipeline, NULL);
    rt->fp16_pipeline = VK_NULL_HANDLE;
  }
  if (rt->pipeline != VK_NULL_HANDLE) {
    vkDestroyPipeline(rt->device, rt->pipeline, NULL);
    rt->pipeline = VK_NULL_HANDLE;
  }
  if (rt->pipeline_layout != VK_NULL_HANDLE) {
    vkDestroyPipelineLayout(rt->device, rt->pipeline_layout, NULL);
    rt->pipeline_layout = VK_NULL_HANDLE;
  }
  if (rt->descriptor_set_layout != VK_NULL_HANDLE) {
    vkDestroyDescriptorSetLayout(rt->device, rt->descriptor_set_layout, NULL);
    rt->descriptor_set_layout = VK_NULL_HANDLE;
  }
  if (rt->descriptor_pool != VK_NULL_HANDLE) {
    vkDestroyDescriptorPool(rt->device, rt->descriptor_pool, NULL);
    rt->descriptor_pool = VK_NULL_HANDLE;
  }
  if (rt->sgemm_timestamp_query_pool != VK_NULL_HANDLE) {
    vkDestroyQueryPool(rt->device, rt->sgemm_timestamp_query_pool, NULL);
    rt->sgemm_timestamp_query_pool = VK_NULL_HANDLE;
  }
  if (rt->submit_fence != VK_NULL_HANDLE) {
    vkDestroyFence(rt->device, rt->submit_fence, NULL);
    rt->submit_fence = VK_NULL_HANDLE;
  }
  if (rt->transfer_submit_fence != VK_NULL_HANDLE) {
    vkDestroyFence(rt->device, rt->transfer_submit_fence, NULL);
    rt->transfer_submit_fence = VK_NULL_HANDLE;
  }
  if (rt->transfer_ready_semaphore != VK_NULL_HANDLE) {
    vkDestroySemaphore(rt->device, rt->transfer_ready_semaphore, NULL);
    rt->transfer_ready_semaphore = VK_NULL_HANDLE;
  }
  if (rt->transfer_command_pool != VK_NULL_HANDLE) {
    vkDestroyCommandPool(rt->device, rt->transfer_command_pool, NULL);
    rt->transfer_command_pool = VK_NULL_HANDLE;
  }
  if (rt->command_pool != VK_NULL_HANDLE) {
    vkDestroyCommandPool(rt->device, rt->command_pool, NULL);
    rt->command_pool = VK_NULL_HANDLE;
  }
  if (rt->device != VK_NULL_HANDLE) {
    vkDestroyDevice(rt->device, NULL);
    rt->device = VK_NULL_HANDLE;
  }
  if (rt->instance != VK_NULL_HANDLE) {
    vkDestroyInstance(rt->instance, NULL);
    rt->instance = VK_NULL_HANDLE;
  }
}

static VkResult vk_runtime_init(prometheus_runtime* rt) {
  VkResult result;
  VkInstanceCreateInfo instance_info;
  uint32_t device_count = 0u;
  VkPhysicalDevice devices[16];
  uint32_t i;
  VkDeviceQueueCreateInfo queue_infos[2];
  VkDeviceCreateInfo device_info;
  float queue_priorities[8];
  uint32_t compute_family_queue_count = 1u;
  VkCommandPoolCreateInfo pool_info;
  VkDescriptorSetLayoutBinding bindings[3];
  VkDescriptorSetLayoutCreateInfo set_layout_info;
  VkPushConstantRange push_range;
  VkPipelineLayoutCreateInfo pipeline_layout_info;
  VkShaderModuleCreateInfo shader_info;
  VkShaderModule shader_module = VK_NULL_HANDLE;
  VkPipelineShaderStageCreateInfo stage_info;
  VkComputePipelineCreateInfo pipeline_info;
  VkDescriptorPoolSize pool_size;
  VkDescriptorPoolCreateInfo descriptor_pool_info;
  VkDescriptorSetAllocateInfo set_alloc_info;
  for (i = 0u; i < 8u; ++i) {
    queue_priorities[i] = 1.0f;
  }
  VkCommandBufferAllocateInfo cmd_alloc_info;
  VkFenceCreateInfo fence_info;
  VkQueryPoolCreateInfo query_pool_info;

  memset(&instance_info, 0, sizeof(instance_info));
  instance_info.sType = VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO;
  result = vkCreateInstance(&instance_info, NULL, &rt->instance);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = vkEnumeratePhysicalDevices(rt->instance, &device_count, NULL);
  if (result != VK_SUCCESS || device_count == 0u) {
    return result == VK_SUCCESS ? VK_ERROR_INITIALIZATION_FAILED : result;
  }

  if (device_count > 16u) {
    device_count = 16u;
  }
  result = vkEnumeratePhysicalDevices(rt->instance, &device_count, devices);
  if (result != VK_SUCCESS) {
    return result;
  }

  rt->physical_device = VK_NULL_HANDLE;
  rt->queue_family_index = UINT32_MAX;
  rt->transfer_queue_family_index = UINT32_MAX;
  rt->dedicated_transfer_available = 0u;
  rt->transfer_queue_enabled = 0u;
  for (i = 0u; i < device_count; ++i) {
    uint32_t family_count = 0u;
    uint32_t family_index;
    VkQueueFamilyProperties families[32];

    vkGetPhysicalDeviceQueueFamilyProperties(devices[i], &family_count, NULL);
    if (family_count == 0u) {
      continue;
    }
    if (family_count > 32u) {
      family_count = 32u;
    }
    vkGetPhysicalDeviceQueueFamilyProperties(devices[i], &family_count, families);
    for (family_index = 0u; family_index < family_count; ++family_index) {
      if ((families[family_index].queueFlags & VK_QUEUE_COMPUTE_BIT) != 0u) {
        rt->physical_device = devices[i];
        rt->queue_family_index = family_index;
        compute_family_queue_count = families[family_index].queueCount;
        if (compute_family_queue_count > 8u) {
          compute_family_queue_count = 8u;
        }
        if (compute_family_queue_count == 0u) {
          compute_family_queue_count = 1u;
        }
        break;
      }
    }
    if (rt->physical_device != VK_NULL_HANDLE) {
      break;
    }
  }

  if (rt->physical_device == VK_NULL_HANDLE || rt->queue_family_index == UINT32_MAX) {
    return VK_ERROR_FEATURE_NOT_PRESENT;
  }
  {
    uint32_t family_count = 0u;
    uint32_t family_index;
    VkQueueFamilyProperties families[32];
    vkGetPhysicalDeviceQueueFamilyProperties(rt->physical_device, &family_count, NULL);
    if (family_count > 32u) {
      family_count = 32u;
    }
    if (family_count > 0u) {
      vkGetPhysicalDeviceQueueFamilyProperties(rt->physical_device, &family_count, families);
    }
    for (family_index = 0u; family_index < family_count; ++family_index) {
      if ((families[family_index].queueFlags & VK_QUEUE_TRANSFER_BIT) == 0u) {
        continue;
      }
      if ((families[family_index].queueFlags & VK_QUEUE_COMPUTE_BIT) != 0u) {
        continue;
      }
      rt->transfer_queue_family_index = family_index;
      rt->dedicated_transfer_available = 1u;
      break;
    }
    if ((rt->test_flags & PROM_TESTCFG_FORCE_NO_DEDICATED_TRANSFER) != 0u) {
      rt->dedicated_transfer_available = 0u;
      rt->transfer_queue_family_index = UINT32_MAX;
    }
    if ((rt->test_flags & PROM_TESTCFG_FORCE_SHARED_TRANSFER) != 0u) {
      rt->transfer_queue_family_index = rt->queue_family_index;
      rt->dedicated_transfer_available = 1u;
    }
    if (rt->dedicated_transfer_available != 0u && (rt->test_flags & PROM_TESTCFG_DISABLE_TRANSFER_QUEUE) == 0u) {
      rt->transfer_queue_enabled = 1u;
    }
  }
  {
    VkPhysicalDeviceProperties props;
    VkPhysicalDeviceMemoryProperties memory_props;
    uint32_t memory_index;
    vkGetPhysicalDeviceProperties(rt->physical_device, &props);
    rt->timestamp_period_ns = props.limits.timestampPeriod;
    if (props.deviceType == VK_PHYSICAL_DEVICE_TYPE_CPU || text_contains_llvmpipe(props.deviceName)) {
      rt->software_vulkan = 1u;
    } else {
      rt->software_vulkan = 0u;
    }
    rt->occupancy_shared_memory_class =
        classify_capability_bucket(props.limits.maxComputeSharedMemorySize, 32768u, 65536u, 98304u, 131072u);
    rt->occupancy_max_workgroup_class =
        classify_capability_bucket(props.limits.maxComputeWorkGroupInvocations, 128u, 256u, 512u, 1024u);
    rt->occupancy_register_file_class = rt->occupancy_max_workgroup_class;
    rt->occupancy_has_exact_profile = 0u;
    if (rt->software_vulkan != 0u) {
      rt->occupancy_memory_bandwidth_class = 1u;
      rt->occupancy_fp32_throughput_class = 1u;
    } else if (props.deviceType == VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU) {
      rt->occupancy_memory_bandwidth_class = 4u;
      rt->occupancy_fp32_throughput_class = 4u;
    } else if (props.deviceType == VK_PHYSICAL_DEVICE_TYPE_INTEGRATED_GPU) {
      rt->occupancy_memory_bandwidth_class = 3u;
      rt->occupancy_fp32_throughput_class = 3u;
    } else {
      rt->occupancy_memory_bandwidth_class = 2u;
      rt->occupancy_fp32_throughput_class = 2u;
    }

    rt->has_device_local_memory = 0u;
    rt->has_host_visible_memory = 0u;
    vkGetPhysicalDeviceMemoryProperties(rt->physical_device, &memory_props);
    for (memory_index = 0u; memory_index < memory_props.memoryTypeCount; ++memory_index) {
      VkMemoryPropertyFlags flags = memory_props.memoryTypes[memory_index].propertyFlags;
      if ((flags & VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT) != 0u) {
        rt->has_device_local_memory = 1u;
      }
      if ((flags & (VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT)) ==
          (VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT)) {
        rt->has_host_visible_memory = 1u;
      }
    }
    rt->occupancy_queue_capability_class = rt->dedicated_transfer_available != 0u ? 4u : 3u;
  }
  {
    uint32_t family_count = 0u;
    VkQueueFamilyProperties families[32];
    rt->timestamp_valid_bits = 0u;
    vkGetPhysicalDeviceQueueFamilyProperties(rt->physical_device, &family_count, NULL);
    if (family_count > 32u) {
      family_count = 32u;
    }
    if (family_count > 0u) {
      vkGetPhysicalDeviceQueueFamilyProperties(rt->physical_device, &family_count, families);
      if (rt->queue_family_index < family_count) {
        rt->timestamp_valid_bits = families[rt->queue_family_index].timestampValidBits;
      }
    }
  }
  rt->capability_fp16_storage = ((rt->test_flags & PROM_TESTCFG_FORCE_NO_FP16_STORAGE) == 0u) ? 1u : 0u;

  memset(queue_infos, 0, sizeof(queue_infos));
  queue_infos[0].sType = VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO;
  queue_infos[0].queueFamilyIndex = rt->queue_family_index;
  queue_infos[0].queueCount = compute_family_queue_count;
  queue_infos[0].pQueuePriorities = queue_priorities;
  if (rt->transfer_queue_enabled != 0u && rt->transfer_queue_family_index != rt->queue_family_index) {
    queue_infos[1].sType = VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO;
    queue_infos[1].queueFamilyIndex = rt->transfer_queue_family_index;
    queue_infos[1].queueCount = 1u;
    queue_infos[1].pQueuePriorities = queue_priorities;
  }

  memset(&device_info, 0, sizeof(device_info));
  device_info.sType = VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO;
  device_info.queueCreateInfoCount =
      (rt->transfer_queue_enabled != 0u && rt->transfer_queue_family_index != rt->queue_family_index) ? 2u : 1u;
  device_info.pQueueCreateInfos = queue_infos;

  if ((rt->test_flags & PROM_TESTCFG_FAIL_DEVICE_CREATE) != 0u) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }

  result = vkCreateDevice(rt->physical_device, &device_info, NULL, &rt->device);
  if (result != VK_SUCCESS) {
    return result;
  }

  rt->timestamp_query_supported = 0u;
  if (rt->timestamp_period_ns > 0.0f && rt->timestamp_valid_bits > 0u) {
    memset(&query_pool_info, 0, sizeof(query_pool_info));
    query_pool_info.sType = VK_STRUCTURE_TYPE_QUERY_POOL_CREATE_INFO;
    query_pool_info.queryType = VK_QUERY_TYPE_TIMESTAMP;
    query_pool_info.queryCount = 2u;
    result = vkCreateQueryPool(rt->device, &query_pool_info, NULL, &rt->sgemm_timestamp_query_pool);
    if (result == VK_SUCCESS) {
      rt->timestamp_query_supported = 1u;
    } else {
      rt->sgemm_timestamp_query_pool = VK_NULL_HANDLE;
    }
  }
  if (rt->timestamp_query_supported != 0u) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_NONE);
  } else if (rt->timestamp_period_ns > 0.0f && rt->timestamp_valid_bits > 0u) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_POOL_UNAVAILABLE);
  } else if (rt->timestamp_period_ns <= 0.0f) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_PERIOD);
  } else {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_UNSUPPORTED);
  }

  vkGetDeviceQueue(rt->device, rt->queue_family_index, 0u, &rt->compute_queue);
  memset(rt->compute_queues, 0, sizeof(rt->compute_queues));
  rt->reported_compute_queue_count = compute_family_queue_count;
  if (rt->reported_compute_queue_count == 0u) {
    rt->reported_compute_queue_count = 1u;
  }
  rt->independent_compute_queue_count = 1u;
  for (i = 0u; i < rt->reported_compute_queue_count && i < 8u; ++i) {
    vkGetDeviceQueue(rt->device, rt->queue_family_index, i, &rt->compute_queues[i]);
  }
  if (rt->compute_queues[0] != VK_NULL_HANDLE) {
    rt->compute_queue = rt->compute_queues[0];
  }
  rt->transfer_queue = VK_NULL_HANDLE;
  if (rt->transfer_queue_enabled != 0u) {
    vkGetDeviceQueue(rt->device, rt->transfer_queue_family_index, 0u, &rt->transfer_queue);
  }

  memset(&pool_info, 0, sizeof(pool_info));
  pool_info.sType = VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO;
  pool_info.flags = VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT;
  pool_info.queueFamilyIndex = rt->queue_family_index;
  result = vkCreateCommandPool(rt->device, &pool_info, NULL, &rt->command_pool);
  if (result != VK_SUCCESS) {
    return result;
  }
  if (rt->transfer_queue_enabled != 0u) {
    pool_info.queueFamilyIndex = rt->transfer_queue_family_index;
    result = vkCreateCommandPool(rt->device, &pool_info, NULL, &rt->transfer_command_pool);
    if (result != VK_SUCCESS) {
      return result;
    }
  }

  memset(bindings, 0, sizeof(bindings));
  bindings[0].binding = 0u;
  bindings[0].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  bindings[0].descriptorCount = 1u;
  bindings[0].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  bindings[1].binding = 1u;
  bindings[1].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  bindings[1].descriptorCount = 1u;
  bindings[1].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  bindings[2].binding = 2u;
  bindings[2].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  bindings[2].descriptorCount = 1u;
  bindings[2].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;

  memset(&set_layout_info, 0, sizeof(set_layout_info));
  set_layout_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO;
  set_layout_info.bindingCount = 3u;
  set_layout_info.pBindings = bindings;
  result = vkCreateDescriptorSetLayout(rt->device, &set_layout_info, NULL, &rt->descriptor_set_layout);
  if (result != VK_SUCCESS) {
    return result;
  }

  pool_size.type = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  pool_size.descriptorCount = 3u;
  memset(&descriptor_pool_info, 0, sizeof(descriptor_pool_info));
  descriptor_pool_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO;
  descriptor_pool_info.poolSizeCount = 1u;
  descriptor_pool_info.pPoolSizes = &pool_size;
  descriptor_pool_info.maxSets = 1u;
  result = vkCreateDescriptorPool(rt->device, &descriptor_pool_info, NULL, &rt->descriptor_pool);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&set_alloc_info, 0, sizeof(set_alloc_info));
  set_alloc_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO;
  set_alloc_info.descriptorPool = rt->descriptor_pool;
  set_alloc_info.descriptorSetCount = 1u;
  set_alloc_info.pSetLayouts = &rt->descriptor_set_layout;
  result = vkAllocateDescriptorSets(rt->device, &set_alloc_info, &rt->descriptor_set);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&push_range, 0, sizeof(push_range));
  push_range.stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  push_range.offset = 0u;
  push_range.size = PROM_VK_SHADER_PUSH_BYTES;

  memset(&pipeline_layout_info, 0, sizeof(pipeline_layout_info));
  pipeline_layout_info.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO;
  pipeline_layout_info.setLayoutCount = 1u;
  pipeline_layout_info.pSetLayouts = &rt->descriptor_set_layout;
  pipeline_layout_info.pushConstantRangeCount = 1u;
  pipeline_layout_info.pPushConstantRanges = &push_range;
  result = vkCreatePipelineLayout(rt->device, &pipeline_layout_info, NULL, &rt->pipeline_layout);
  if (result != VK_SUCCESS) {
    return result;
  }

  if ((rt->test_flags & PROM_TESTCFG_FAIL_PIPELINE_CREATE) != 0u) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }

  memset(&shader_info, 0, sizeof(shader_info));
  shader_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  shader_info.codeSize = sizeof(k_prom_sgemm_srt_2accum_k_spirv);
  shader_info.pCode = k_prom_sgemm_srt_2accum_k_spirv;
  result = vkCreateShaderModule(rt->device, &shader_info, NULL, &shader_module);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = "main";

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->srt_2accum_k_pipeline);
  vkDestroyShaderModule(rt->device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&shader_info, 0, sizeof(shader_info));
  shader_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  shader_info.codeSize = sizeof(k_prom_sgemm_b2x2_row_major_biased_spirv);
  shader_info.pCode = k_prom_sgemm_b2x2_row_major_biased_spirv;
  result = vkCreateShaderModule(rt->device, &shader_info, NULL, &shader_module);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = "main";
  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL,
                                    &rt->b2x2_row_major_biased_pipeline);
  vkDestroyShaderModule(rt->device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&shader_info, 0, sizeof(shader_info));
  shader_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  shader_info.codeSize = sizeof(k_prom_sgemm_a2x4_row_biased_accum8_spirv);
  shader_info.pCode = k_prom_sgemm_a2x4_row_biased_accum8_spirv;
  result = vkCreateShaderModule(rt->device, &shader_info, NULL, &shader_module);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = "main";
  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL,
                                    &rt->a2x4_row_biased_accum8_pipeline);
  vkDestroyShaderModule(rt->device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&shader_info, 0, sizeof(shader_info));
  shader_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  shader_info.codeSize = sizeof(k_prom_sgemm_spirv);
  shader_info.pCode = k_prom_sgemm_spirv;
  result = vkCreateShaderModule(rt->device, &shader_info, NULL, &shader_module);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = "main";

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->pipeline);
  vkDestroyShaderModule(rt->device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&shader_info, 0, sizeof(shader_info));
  shader_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  shader_info.codeSize = sizeof(k_prom_sgemm_tiled_spirv);
  shader_info.pCode = k_prom_sgemm_tiled_spirv;
  result = vkCreateShaderModule(rt->device, &shader_info, NULL, &shader_module);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = "main";

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->tiled_pipeline);
  vkDestroyShaderModule(rt->device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&shader_info, 0, sizeof(shader_info));
  shader_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  shader_info.codeSize = sizeof(k_prom_sgemm_packed4_spirv);
  shader_info.pCode = k_prom_sgemm_packed4_spirv;
  result = vkCreateShaderModule(rt->device, &shader_info, NULL, &shader_module);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = "main";

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->packed4_pipeline);
  vkDestroyShaderModule(rt->device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&shader_info, 0, sizeof(shader_info));
  shader_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  shader_info.codeSize = sizeof(k_prom_sgemm_fp16_storage_fp32accum_spirv);
  shader_info.pCode = k_prom_sgemm_fp16_storage_fp32accum_spirv;
  result = vkCreateShaderModule(rt->device, &shader_info, NULL, &shader_module);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = "main";

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->fp16_pipeline);
  vkDestroyShaderModule(rt->device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&cmd_alloc_info, 0, sizeof(cmd_alloc_info));
  cmd_alloc_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO;
  cmd_alloc_info.commandPool = rt->command_pool;
  cmd_alloc_info.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;
  cmd_alloc_info.commandBufferCount = 1u;
  result = vkAllocateCommandBuffers(rt->device, &cmd_alloc_info, &rt->command_buffer);
  if (result != VK_SUCCESS) {
    return result;
  }
  if (rt->transfer_queue_enabled != 0u) {
    cmd_alloc_info.commandPool = rt->transfer_command_pool;
    result = vkAllocateCommandBuffers(rt->device, &cmd_alloc_info, &rt->transfer_command_buffer);
    if (result != VK_SUCCESS) {
      return result;
    }
  }

  memset(&fence_info, 0, sizeof(fence_info));
  fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
  result = vkCreateFence(rt->device, &fence_info, NULL, &rt->submit_fence);
  if (result != VK_SUCCESS) {
    return result;
  }
  if (rt->transfer_queue_enabled != 0u) {
    VkSemaphoreCreateInfo semaphore_info;
    result = vkCreateFence(rt->device, &fence_info, NULL, &rt->transfer_submit_fence);
    if (result != VK_SUCCESS) {
      return result;
    }
    memset(&semaphore_info, 0, sizeof(semaphore_info));
    semaphore_info.sType = VK_STRUCTURE_TYPE_SEMAPHORE_CREATE_INFO;
    result = vkCreateSemaphore(rt->device, &semaphore_info, NULL, &rt->transfer_ready_semaphore);
    if (result != VK_SUCCESS) {
      return result;
    }
  }
  return VK_SUCCESS;
}

// ============================================================================
// Public SGEMM ABI Entrypoints
// ============================================================================

int prom_reactor_runtime_create_impl(void* config, void** out_handle) {
  VkResult result;
  prometheus_runtime* runtime;
  (void)config;

  if (out_handle == NULL) {
    return PROM_ERROR;
  }

  *out_handle = NULL;
  runtime = (prometheus_runtime*)malloc(sizeof(prometheus_runtime));
  if (runtime == NULL) {
    return PROM_INTERNAL_ERROR;
  }
  memset(runtime, 0, sizeof(*runtime));
  runtime->magic = PROMETHEUS_RUNTIME_MAGIC;
  runtime->reason_code = PROM_REASON_VULKAN_UNAVAILABLE;
  prom_dom_blackboard_init(&runtime->blackboard);
  prom_dominatus_measurement_filter_init(&runtime->p14_measurement_filter_state, NULL);
  memset(&runtime->p14_last_filtered_evidence, 0, sizeof(runtime->p14_last_filtered_evidence));
  runtime->p14_measurement_tick = 0u;
  prom_dominatus_predictor_init(&runtime->p15_predictor_state, NULL);
  prom_dominatus_shadow_calibration_init(&runtime->p15_shadow_calibration);
  prom_dominatus_shadow_would_act_init(&runtime->p15_shadow_would_act_state);
  runtime->p15_shadow_canary_params = prom_dominatus_shadow_canary_default_params();
  prom_dominatus_shadow_canary_init(&runtime->p15_shadow_canary_state);
  runtime->p15_prestage_params = prom_dominatus_prestage_default_params();
  prom_sgemm_controller_init(&runtime->sgemm_controller);
  prom_slot_hfsm_init(&runtime->slots[0], 0u);
  prom_slot_hfsm_init(&runtime->slots[1], 1u);
  runtime->slot_diag.current_slot_id = UINT32_MAX;
  runtime->slot_diag.next_slot_id = 0u;
  runtime->slot_diag.failure_slot_id = -1;
  runtime->slot_diag.async_slot_id = -1;
  runtime->slot_diag.transfer_queue_family_index = UINT32_MAX;
  invalidate_selector_caches(runtime);
  runtime->slot_diag.compute_queue_family_index = UINT32_MAX;
  runtime->slot_diag.transfer_fallback_reason = PROM_TRANSFER_FALLBACK_REQUIRED;
  runtime->slot_diag.transfer_failure_slot_id = -1;
  runtime->slot_diag.transfer_failure_reason = 0;
  runtime->reported_compute_queue_count = 1u;
  runtime->independent_compute_queue_count = 1u;
  runtime->async_task_id = 0;
  runtime->async_state = PROM_ASYNC_STATE_IDLE;
  runtime->async_stage = PROM_STAGE_NONE;
  runtime->async_failure_detail = 0;
  commit_slot_runtime_diag_snapshot(runtime, 0);
  stage_commit_async_snapshot(runtime, PROM_DOM_EVENT_NONE, 0);

  if (config != NULL) {
    const PrometheusReactorConfig* cfg = (const PrometheusReactorConfig*)config;
    if (cfg->struct_size >= sizeof(PrometheusReactorConfig)) {
      runtime->test_flags = cfg->test_flags;
      runtime->p15_shadow_canary_params.enabled = cfg->p15_shadow_canary_enabled != 0u ? 1u : 0u;
      runtime->p15_shadow_canary_state.enabled = runtime->p15_shadow_canary_params.enabled;
      runtime->p15_shadow_authority_gate.authority_enabled = runtime->p15_shadow_canary_params.enabled;
    }
  }
  runtime->arena_budget_limit_bytes = PROM_ARENA_DEFAULT_BUDGET_BYTES;
  runtime->arena_floor_bytes = PROM_ARENA_DEFAULT_SHRINK_FLOOR_BYTES;
  if (runtime->test_flags != 0u) {
    runtime->arena_floor_bytes = 1ull * 1024ull * 1024ull;
    runtime->arena_budget_limit_bytes = 32ull * 1024ull * 1024ull;
  }
  runtime->slot_diag.p11_m3_budget_limit_bytes = runtime->arena_budget_limit_bytes;
  runtime->arena_shrink_low_usage_threshold_epochs = PROM_ARENA_SHRINK_LOW_USAGE_EPOCHS;
  runtime->arena_shrink_cooldown_epochs = PROM_ARENA_SHRINK_COOLDOWN_EPOCHS;
  runtime->arena_last_failure_detail = 0;
  runtime->arenas[PROM_ARENA_ROLE_A].role = PROM_ARENA_ROLE_A;
  runtime->arenas[PROM_ARENA_ROLE_B].role = PROM_ARENA_ROLE_B;
  runtime->arenas[PROM_ARENA_ROLE_C].role = PROM_ARENA_ROLE_C;
  runtime->arenas[PROM_ARENA_ROLE_UPLOAD].role = PROM_ARENA_ROLE_UPLOAD;
  runtime->arenas[PROM_ARENA_ROLE_A].memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  runtime->arenas[PROM_ARENA_ROLE_B].memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  runtime->arenas[PROM_ARENA_ROLE_C].memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  runtime->arenas[PROM_ARENA_ROLE_UPLOAD].memory_class = PROM_ARENA_MEMORY_HOST_VISIBLE;
  runtime->arenas[PROM_ARENA_ROLE_A].owner_slot_id = -1;
  runtime->arenas[PROM_ARENA_ROLE_B].owner_slot_id = -1;
  runtime->arenas[PROM_ARENA_ROLE_C].owner_slot_id = -1;
  runtime->arenas[PROM_ARENA_ROLE_UPLOAD].owner_slot_id = -1;
  runtime->slot_diag.p11_m3_budget_limit_bytes = runtime->arena_budget_limit_bytes;

  if ((runtime->test_flags & PROM_TESTCFG_SKIP_VULKAN_INIT) != 0u) {
    runtime->available = 0u;
    runtime->reason_code = PROM_REASON_VULKAN_UNAVAILABLE;
    runtime->init_detail_code = (int)VK_ERROR_INITIALIZATION_FAILED;
  } else {
    result = vk_runtime_init(runtime);
    if (result == VK_SUCCESS) {
      prom_dom_transfer_queue_facts transfer_facts;
      prom_dom_transfer_queue_decision transfer_decision;
      runtime->available = 1u;
      runtime->reason_code = PROM_REASON_NONE;
      runtime->init_detail_code = 0;
      memset(&transfer_facts, 0, sizeof(transfer_facts));
      transfer_facts.dedicated_transfer_available = runtime->dedicated_transfer_available;
      transfer_facts.transfer_queue_family_index = runtime->transfer_queue_family_index;
      transfer_facts.compute_queue_family_index = runtime->queue_family_index;
      transfer_facts.queue_families_differ =
          (runtime->dedicated_transfer_available != 0u && runtime->transfer_queue_family_index != runtime->queue_family_index) ? 1u : 0u;
      transfer_facts.transfer_queue_supported = runtime->transfer_queue_enabled;
      transfer_facts.transfer_queue_disabled_by_config = ((runtime->test_flags & PROM_TESTCFG_DISABLE_TRANSFER_QUEUE) != 0u) ? 1u : 0u;
      transfer_facts.transfer_workload_large_enough = 1u;
      transfer_facts.transfer_sync_ownership_supported = runtime->transfer_queue_enabled;
      transfer_facts.transfer_fallback_available = 1u;
      transfer_facts.upload_only_policy_eligible = 1u;
      transfer_facts.upload_readback_supported = 0u;
      if (prom_dom_sgemm_stage_transfer_queue_facts(&runtime->blackboard, &transfer_facts) != 0u) {
        prom_dom_sgemm_commit(&runtime->blackboard);
      }
      memset(&transfer_decision, 0, sizeof(transfer_decision));
      transfer_decision.transfer_policy_selected = runtime->transfer_queue_enabled;
      transfer_decision.selected_transfer_policy = runtime->transfer_queue_enabled != 0u ? 1u : 0u;
      transfer_decision.transfer_queue_used = runtime->transfer_queue_enabled;
      transfer_decision.transfer_fallback_reason =
          runtime->transfer_queue_enabled != 0u ? PROM_TRANSFER_FALLBACK_NONE
                                                : (((runtime->test_flags & PROM_TESTCFG_DISABLE_TRANSFER_QUEUE) != 0u)
                                                       ? PROM_TRANSFER_FALLBACK_DISABLED_BY_CONFIG
                                                       : PROM_TRANSFER_FALLBACK_NO_DEDICATED_QUEUE);
      if (prom_dom_sgemm_stage_transfer_queue_decision(&runtime->blackboard, &transfer_decision) != 0u) {
        prom_dom_sgemm_commit(&runtime->blackboard);
      }
      if (prom_dom_sgemm_stage_transfer_handoff(&runtime->blackboard, 0u, 0u, 0) != 0u &&
          prom_dom_sgemm_stage_transfer_wait(&runtime->blackboard, 0u, 0u, 0) != 0u &&
          prom_dom_sgemm_stage_transfer_failure(&runtime->blackboard, -1, 0, 0u) != 0u &&
          prom_dom_sgemm_stage_transfer_complete(&runtime->blackboard, 1u, 0u, 0u, 0) != 0u) {
        prom_dom_sgemm_commit(&runtime->blackboard);
      }
      sync_transfer_diag_from_visible(runtime);
    } else {
      runtime->available = 0u;
      runtime->reason_code = PROM_REASON_VULKAN_UNAVAILABLE;
      runtime->init_detail_code = (int)result;
      vk_runtime_cleanup(runtime);
    }
  }

  if (!registry_add(runtime)) {
    vk_runtime_cleanup(runtime);
    free(runtime);
    return PROM_INTERNAL_ERROR;
  }

  *out_handle = runtime;
  return PROM_OK;
}

int prom_reactor_runtime_destroy_impl(void* handle) {
  prometheus_runtime* runtime;
  if (handle == NULL) {
    return PROM_OK;
  }
  if (!registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }

  runtime = (prometheus_runtime*)handle;
  if (runtime->magic != PROMETHEUS_RUNTIME_MAGIC) {
    return PROM_INVALID_HANDLE;
  }

  registry_remove(handle);
  prom_fft_diag_forget_handle(handle);
  vk_runtime_cleanup(runtime);
  free(runtime);
  return PROM_OK;
}

int prom_reactor_runtime_probe_impl(void* handle, PrometheusCaps* out_caps) {
  prometheus_runtime* runtime;
  if (out_caps == NULL) {
    return PROM_ERROR;
  }
  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }

  runtime = (prometheus_runtime*)handle;
  out_caps->available = runtime->available;
  if (runtime->available == 0u) {
    out_caps->backend_type = PROM_BACKEND_UNKNOWN;
  } else if (runtime->software_vulkan != 0u) {
    out_caps->backend_type = PROM_BACKEND_VULKAN_SOFTWARE;
  } else {
    out_caps->backend_type = PROM_BACKEND_VULKAN;
  }
  out_caps->reason_code = runtime->reason_code;
  return PROM_OK;
}

// ============================================================================
// SGEMM Single-Call Execution
// ============================================================================

static uint32_t prom_occ_variant_registered(uint32_t variant) {
  return (variant >= PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR &&
          variant <= PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) ? 1u : 0u;
}

static uint32_t prom_occ_variant_path_status(uint32_t variant) {
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) {
    return PROM_OCCUPANCY_VARIANT_PATH_STATUS_BASELINE;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE ||
      variant == PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4 ||
      variant == PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) {
    return PROM_OCCUPANCY_VARIANT_PATH_STATUS_WIRED;
  }
  if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE) {
    return PROM_OCCUPANCY_VARIANT_PATH_STATUS_ALIAS_OR_NOT_WIRED;
  }
  return PROM_OCCUPANCY_VARIANT_PATH_STATUS_NOT_WIRED;
}
/* Promotion seam terms:
 * DVT: local GPU correctness/sanity.
 * PVT: broader cloud/borrowed GPU validation.
 * production_eligible: allowed into production policy candidate set.
 * dispatch_enabled: production policy may actively dispatch it.
 */

static int prom_reactor_runtime_sgemm_impl_with_variant(void* handle,
                                                        const float* a,
                                                        const float* b,
                                                        float* c,
                                                        uint32_t m,
                                                        uint32_t n,
                                                        uint32_t k,
                                                        uint32_t requested_variant,
                                                        uint32_t* out_stage,
                                                        int* out_detail_code);

int prom_reactor_runtime_sgemm_impl(void* handle,
                                     const float* a,
                                     const float* b,
                                     float* c,
                                     uint32_t m,
                                     uint32_t n,
                                     uint32_t k,
                                     uint32_t* out_stage,
                                     int* out_detail_code) {
  return prom_reactor_runtime_sgemm_impl_with_variant(handle, a, b, c, m, n, k,
                                                      PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR,
                                                      out_stage, out_detail_code);
}

int prom_reactor_runtime_sgemm_benchmark_variant_impl(void* handle,
                                                      const float* a,
                                                      const float* b,
                                                      float* c,
                                                      uint32_t m,
                                                      uint32_t n,
                                                      uint32_t k,
                                                      uint32_t requested_variant,
                                                      uint32_t* out_stage,
                                                      int* out_detail_code) {
  return prom_reactor_runtime_sgemm_impl_with_variant(handle, a, b, c, m, n, k, requested_variant,
                                                      out_stage, out_detail_code);
}

static int prom_reactor_runtime_sgemm_impl_with_variant(void* handle,
                                     const float* a,
                                     const float* b,
                                     float* c,
                                     uint32_t m,
                                     uint32_t n,
                                     uint32_t k,
                                     uint32_t requested_variant,
                                     uint32_t* out_stage,
                                     int* out_detail_code) {
  prometheus_runtime* rt;
  VkResult vk_result;
  VkWriteDescriptorSet writes[3];
  VkDescriptorBufferInfo buffer_infos[3];
  VkCommandBufferBeginInfo begin_info;
  VkSubmitInfo submit_info;
  VkSubmitInfo transfer_submit_info;
  VkPipelineStageFlags wait_stage_mask;
  VkBufferMemoryBarrier barriers[4];
  VkBufferCopy copies[3];
  prom_vk_push push;
  prom_vk_buffer* shader_a;
  prom_vk_buffer* shader_b;
  prom_vk_buffer* shader_c;
  VkDeviceSize a_buffer_size = 0;
  VkDeviceSize b_buffer_size = 0;
  VkDeviceSize c_buffer_size = 0;
  size_t a_copy_size = 0u;
  size_t b_copy_size = 0u;
  size_t c_copy_size = 0u;
  uint32_t compute_k;
  uint64_t work_units;
  uint32_t work_units_u32;
  uint32_t mn_product;
  uint32_t can_stage;
  uint32_t can_direct;
  uint32_t tiled_shape;
  uint32_t readback_required;
  uint32_t packed4_waste_permille;
  uint32_t packed4_budget_permille;
  uint32_t packed4_small_shape;
  uint32_t packed4_tail_count;
  uint32_t packed4_padded_lane_count;
  uint32_t fp16_has_special_values;
  int fp16_utility_score;
  prom_policy_mode policy_mode;
  prom_vk_path_mode selected_path;
  prom_vk_compute_mode compute_mode;
  prom_judgment_facts judgment_facts;
  prom_judgment_decision judgment_decision;
  prom_judgment_layout_precision_decision layout_precision_selector_decision;
  prom_judgment_async_facts async_facts;
  prom_judgment_async_decision async_decision;
  prom_buffering_selector_facts buffering_facts;
  prom_dom_sgemm_buffering_projection buffering_projection;
  prom_buffering_selector_decision buffering_decision;
  prom_occupancy_selector_facts occupancy_facts;
  prom_occupancy_selector_decision occupancy_decision;
  prom_dom_transfer_queue_facts transfer_queue_facts;
  prom_dom_transfer_queue_projection transfer_queue_projection;
  prom_dom_transfer_queue_decision transfer_queue_decision;
  prom_dom_transfer_queue_snapshot transfer_queue_snapshot;
  prom_dom_sgemm_layout_precision_facts layout_precision_facts;
  prom_dom_sgemm_layout_precision_projection layout_precision_projection;
  prom_dom_sgemm_layout_precision_decision layout_precision_decision;
  prom_dom_sgemm_path_compute_facts path_compute_facts;
  prom_dom_sgemm_path_compute_projection path_compute_projection;
  prom_dom_sgemm_path_compute_decision path_compute_decision;
  prom_dom_sgemm_path_compute_snapshot path_compute_snapshot;
  prom_buffering_mode buffering_mode = PROM_BUFFERING_MODE_FIXED_DOUBLE_DEFAULT;
  prom_variance_class variance_class = PROM_VARIANCE_MODERATE;
  prom_predictability_class predictability_class = PROM_PREDICTABILITY_STABLE;
  VkPipeline selected_pipeline;
  float* packed_a_upload = NULL;
  float* packed_b_upload = NULL;
  uint32_t* fp16_a_upload = NULL;
  uint32_t* fp16_b_upload = NULL;
  int final_detail = 0;
  int prepare_detail = 0;
  uint32_t use_dedicated_transfer_upload = 0u;
  uint32_t request_async = 0u;
  uint32_t work_slot_id = 0u;
  uint64_t required_capacity_bytes = 0u;
  uint64_t layout_precision_dependency_dirty_mask = 0u;
  uint64_t layout_precision_path_guard_dirty_mask = 0u;
  uint32_t artifact_layout_code = 0u;
  uint32_t artifact_precision_code = 0u;
  prom_buffer_artifact_key artifact_a_key;
  prom_buffer_artifact_key artifact_b_key;
  prom_buffer_artifact_key artifact_c_key;
  prom_resource_lease_facts lease_facts;
  prom_resource_lease_decision lease_decision;
  prom_resource_lease_decision lease_yield_decision;
  uint32_t lease_granted = 0u;

  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_NONE, 0);

  if (handle == NULL || !registry_contains(handle)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }

  rt = (prometheus_runtime*)handle;
  request_async = (((rt->test_flags & PROM_TESTCFG_SKIP_SUBMIT_WAIT) != 0u) && c == NULL) ? 1u : 0u;
  if (a == NULL || b == NULL || (request_async == 0u && c == NULL)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  if (m == 0u || n == 0u || k == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_ERROR);
    return PROM_ERROR;
  }
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  rt->slot_diag.p13_m16b1_requested_occupancy_variant = requested_variant;
  rt->slot_diag.p13_m16b1_executed_occupancy_variant =
      (requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR)
          ? PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR
          : ((requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE ||
              requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE ||
              requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4 ||
              requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8)
                 ? requested_variant
                 : PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR);
  rt->slot_diag.p13_m16b1_variant_registered = prom_occ_variant_registered(requested_variant);
  rt->slot_diag.p13_m16b1_variant_benchmark_enabled = rt->slot_diag.p13_m16b1_variant_registered;
  rt->slot_diag.p13_m16b1_variant_dvt_validated =
      (requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) ? 1u : 0u;
  rt->slot_diag.p13_m16b1_variant_pvt_validated =
      (requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) ? 1u : 0u;
  rt->slot_diag.p13_m16b1_variant_production_eligible =
      (requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) ? 1u : 0u;
  rt->slot_diag.p13_m16b1_variant_dispatch_enabled =
      (requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) ? 1u : 0u;
  rt->slot_diag.p13_m16b1_variant_path_status = prom_occ_variant_path_status(requested_variant);
  rt->slot_diag.p13_m16b1_variant_path_id =
      (requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR)
          ? PROM_OCCUPANCY_VARIANT_PATH_ID_BASELINE
          : ((requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE)
                 ? PROM_OCCUPANCY_VARIANT_PATH_ID_SRT_2ACCUM_K
                 : ((requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4)
                        ? PROM_OCCUPANCY_VARIANT_PATH_ID_B2X2_ROW_MAJOR_BIASED
                        : ((requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8)
                               ? PROM_OCCUPANCY_VARIANT_PATH_ID_A2X4_ROW_BIASED_ACCUM8
                               : PROM_OCCUPANCY_VARIANT_PATH_ID_BASELINE)));
  rt->slot_diag.p13_m16b1_fallback_reason =
      (requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE)
          ? PROM_OCCUPANCY_VARIANT_FALLBACK_MC_BASELINE_STRICT_ALIAS
          : (requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR ||
             requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE ||
             requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4 ||
             requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8)
          ? PROM_OCCUPANCY_VARIANT_FALLBACK_NONE
          : PROM_OCCUPANCY_VARIANT_FALLBACK_PATH_NOT_WIRED;
  if (rt->available == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, rt->init_detail_code);
    return PROM_ERROR;
  }
  if (rt->timestamp_query_supported != 0u && rt->sgemm_timestamp_query_pool != VK_NULL_HANDLE) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_UNAVAILABLE);
  } else if (rt->timestamp_period_ns > 0.0f && rt->timestamp_valid_bits > 0u) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_POOL_UNAVAILABLE);
  } else if (rt->timestamp_period_ns <= 0.0f) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_PERIOD);
  } else {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_UNSUPPORTED);
  }
  if (!prom_vk_checked_mul_u32(m, k, &work_units_u32) || !prom_vk_checked_mul_u32(k, n, &work_units_u32) ||
      !prom_vk_checked_mul_u32(m, n, &work_units_u32) || !prom_vk_checked_mul_u32(m, n, &mn_product) ||
      !prom_vk_checked_mul_u32(mn_product, k, &work_units_u32)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_DETAIL_SIZE_OVERFLOW);
    return PROM_ERROR;
  }
  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, 0);
  if (rt->async_state == PROM_ASYNC_STATE_CONSUMED) {
    set_async_state(rt, PROM_ASYNC_STATE_IDLE, PROM_STAGE_NONE, 0);
  }
  if (rt->async_state == PROM_ASYNC_STATE_FAILED) {
    prom_vk_set_status(out_stage,
               out_detail_code,
               PROM_STAGE_SUBMIT,
               rt->async_failure_detail != 0 ? rt->async_failure_detail : PROM_DETAIL_ASYNC_FAILED);
    return PROM_ERROR;
  }
  if (rt->async_state == PROM_ASYNC_STATE_SUBMITTED || rt->async_state == PROM_ASYNC_STATE_READY) {
    rt->slot_diag.inflight_rejection_count += 1u;
    commit_slot_runtime_diag_snapshot(rt, PROM_DETAIL_ASYNC_UNCONSUMED);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_ASYNC_UNCONSUMED);
    stage_commit_async_snapshot(rt, PROM_DOM_EVENT_ASYNC_UNCONSUMED_REJECTED, PROM_DETAIL_ASYNC_UNCONSUMED);
    return PROM_ERROR;
  }
  if (rt->in_flight_submit != 0u) {
    vk_result = vkGetFenceStatus(rt->device, rt->submit_fence);
    if (vk_result == VK_SUCCESS) {
      rt->in_flight_submit = 0u;
    } else {
      rt->slot_diag.inflight_rejection_count += 1u;
      commit_slot_runtime_diag_snapshot(rt, PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED);
      return PROM_ERROR;
    }
  }

  if ((rt->test_flags & PROM_TESTCFG_FAIL_UPLOAD) != 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_DETAIL_INJECTED_UPLOAD_FAILURE);
    return PROM_ERROR;
  }

  can_stage = rt->has_device_local_memory;
  can_direct = rt->has_host_visible_memory;
  if ((rt->test_flags & PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY) != 0u) {
    can_stage = 0u;
  }
  readback_required = ((rt->test_flags & PROM_TESTCFG_FORCE_UPLOAD_ONLY) == 0u) ? 1u : 0u;
  work_units = (uint64_t)work_units_u32;
  policy_mode = prom_sgemm_controller_step(&rt->sgemm_controller, m, n, k, work_units, rt->software_vulkan);
  packed4_waste_permille = prom_packed4_padding_waste_permille(m, n, k);
  packed4_budget_permille = prom_packed4_mode_budget_permille(policy_mode);
  packed4_small_shape = (m < 4u || n < 4u || k < 4u) ? 1u : 0u;
  packed4_tail_count = prom_packed4_tail_count(m, n, k);
  packed4_padded_lane_count = (uint32_t)((prom_round_up4_u32(k) - k) * (m + n));
  fp16_has_special_values = 0u;
  fp16_utility_score = -1000;
  prom_fp16_evaluate_tolerance(a, b, m, n, k, &rt->sgemm_controller, &fp16_has_special_values, &fp16_utility_score);
  if ((rt->test_flags & PROM_TESTCFG_FORCE_FP16_UTILITY_WIN) != 0u) {
    fp16_utility_score = 1201;
  }
  tiled_shape = (work_units >= (uint64_t)PROM_JUDGMENT_TILED_WORK_THRESHOLD && m >= PROM_VK_LOCAL_SIZE_X &&
                 n >= PROM_VK_LOCAL_SIZE_Y && k >= PROM_VK_TILE_K)
                    ? 1u
                    : 0u;
  memset(&occupancy_facts, 0, sizeof(occupancy_facts));
  occupancy_facts.register_file_class = rt->occupancy_register_file_class;
  occupancy_facts.shared_memory_class = rt->occupancy_shared_memory_class;
  occupancy_facts.memory_bandwidth_class = rt->occupancy_memory_bandwidth_class;
  occupancy_facts.fp32_throughput_class = rt->occupancy_fp32_throughput_class;
  occupancy_facts.max_workgroup_class = rt->occupancy_max_workgroup_class;
  occupancy_facts.queue_capability_class = rt->occupancy_queue_capability_class;
  occupancy_facts.has_exact_profile = rt->occupancy_has_exact_profile;
  occupancy_facts.manual_override_enabled = 0u;
  occupancy_facts.manual_override_variant = 0u;
  occupancy_facts.m = m;
  occupancy_facts.n = n;
  occupancy_facts.k = k;
  occupancy_facts.work_units = work_units;
  prom_judgment_engine_select_occupancy_variant(&occupancy_facts, &occupancy_decision);
  (void)prom_dominatus_predictor_advance_reservations(&rt->p15_predictor_state, rt->p14_measurement_tick);
  rt->p15_feedforward_dispatch_state.valid = 1u;
  rt->p15_feedforward_dispatch_state.enabled = rt->p15_shadow_canary_params.enabled != 0u ? 1u : 0u;
  rt->p15_feedforward_dispatch_state.used = 0u;
  rt->p15_feedforward_dispatch_state.source = 0u;
  rt->p15_feedforward_dispatch_state.block_reason = 0u;
  rt->p15_feedforward_dispatch_state.reserved_variant_id = 0u;
  if (rt->p15_feedforward_dispatch_state.enabled == 0u) {
    rt->p15_feedforward_dispatch_state.block_reason = 1u;
  } else if (rt->p15_shadow_authority_gate.state != PROM_SHADOW_AUTHORITY_HEALTHY) {
    rt->p15_feedforward_dispatch_state.block_reason = 2u;
    rt->p15_feedforward_dispatch_state.reason_binding_block_count += 1u;
  } else if (occupancy_decision.fallback_used != 0u) {
    rt->p15_feedforward_dispatch_state.block_reason = 6u;
    rt->p15_feedforward_dispatch_state.fallback_to_judgment_count += 1u;
  } else if (rt->p15_shadow_canary_state.healthy_margin_passed == 0u) {
    rt->p15_feedforward_dispatch_state.block_reason = 3u;
    rt->p15_feedforward_dispatch_state.margin_block_count += 1u;
  } else if (rt->p15_shadow_canary_state.reason_binding_passed == 0u) {
    rt->p15_feedforward_dispatch_state.block_reason = 4u;
    rt->p15_feedforward_dispatch_state.reason_binding_block_count += 1u;
  } else {
    prom_dominatus_reservation_decision consume =
        prom_dominatus_reservation_consume_matured(&rt->p15_predictor_state.reservations, occupancy_decision.shape_class, occupancy_decision.selected_variant);
    if (consume.valid != 0u && consume.yielded != 0u) {
      rt->p15_feedforward_dispatch_state.used = 1u;
      rt->p15_feedforward_dispatch_state.source = 1u;
      rt->p15_feedforward_dispatch_state.reserved_variant_id = occupancy_decision.selected_variant;
      rt->p15_feedforward_dispatch_state.reservation_consumed_count += 1u;
    } else {
      uint32_t i;
      uint32_t saw_shape_mismatch = 0u;
      uint32_t saw_variant_mismatch = 0u;
      for (i = 0u; i < PROM_DOM_RESERVATION_CAP; ++i) {
        const prom_dominatus_reservation_request* e = &rt->p15_predictor_state.reservations.entries[i];
        if (e->valid == 0u || e->state != PROM_DOM_RESERVATION_MATURED) continue;
        if (e->shape_class != occupancy_decision.shape_class) {
          saw_shape_mismatch = 1u;
          continue;
        }
        if (e->variant_id != occupancy_decision.selected_variant) {
          saw_variant_mismatch = 1u;
        }
      }
      rt->p15_feedforward_dispatch_state.block_reason = 5u;
      rt->p15_feedforward_dispatch_state.no_matured_reservation_count += 1u;
      rt->p15_feedforward_dispatch_state.fallback_to_judgment_count += 1u;
      if (saw_shape_mismatch != 0u) rt->p15_feedforward_dispatch_state.shape_mismatch_count += 1u;
      if (saw_variant_mismatch != 0u) rt->p15_feedforward_dispatch_state.variant_mismatch_count += 1u;
    }
  }
  rt->slot_diag.p13_m2_occupancy_device_band = occupancy_decision.device_band;
  rt->slot_diag.p13_m2_occupancy_shape_class = occupancy_decision.shape_class;
  rt->slot_diag.p13_m2_occupancy_selected_variant = occupancy_decision.selected_variant;
  rt->slot_diag.p13_m2_occupancy_unclamped_variant = occupancy_decision.unclamped_variant;
  rt->slot_diag.p13_m2_occupancy_clamp_reason = occupancy_decision.clamp_reason;
  rt->slot_diag.p13_m2_occupancy_override_used = occupancy_decision.override_used;
  rt->slot_diag.p13_m2_occupancy_fallback_used = occupancy_decision.fallback_used;
  memset(&path_compute_facts, 0, sizeof(path_compute_facts));
  path_compute_facts.m = m;
  path_compute_facts.n = n;
  path_compute_facts.k = k;
  path_compute_facts.work_units = work_units;
  path_compute_facts.can_stage = can_stage;
  path_compute_facts.can_direct = can_direct;
  path_compute_facts.allow_fallback = ((rt->test_flags & PROM_TESTCFG_DISABLE_STAGING_FALLBACK) == 0u) ? 1u : 0u;
  path_compute_facts.readback_required = readback_required;
  path_compute_facts.force_direct = ((rt->test_flags & PROM_TESTCFG_FORCE_DIRECT_PATH) != 0u) ? 1u : 0u;
  if (path_compute_facts.force_direct == 0u && policy_mode == PROM_POLICY_MODE_SAFE &&
      (rt->test_flags & PROM_TESTCFG_FORCE_STAGED_PATH) == 0u && (rt->test_flags & PROM_TESTCFG_FORCE_TILED_PATH) == 0u) {
    /* SAFE mode currently biases to direct+baseline for conservative behavior.
     * This can suppress direct+tiled on large shapes; keep unchanged in this pass
     * and revisit with real GPU validation data before any policy relaxation. */
    path_compute_facts.force_direct = 1u;
  }
  path_compute_facts.force_staged = ((rt->test_flags & PROM_TESTCFG_FORCE_STAGED_PATH) != 0u) ? 1u : 0u;
  path_compute_facts.force_tiled = ((rt->test_flags & PROM_TESTCFG_FORCE_TILED_PATH) != 0u) ? 1u : 0u;
  path_compute_facts.tiled_shape = tiled_shape;
  path_compute_facts.software_vulkan = rt->software_vulkan;
  path_compute_facts.policy_mode = (uint32_t)policy_mode;
  if (prom_dom_sgemm_stage_path_compute_facts(&rt->blackboard, &path_compute_facts) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  if (prom_dom_sgemm_build_path_compute_facts_from_visible(&rt->blackboard, &path_compute_facts, &path_compute_projection) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  memset(&judgment_facts, 0, sizeof(judgment_facts));
  judgment_facts.m = path_compute_projection.facts.m;
  judgment_facts.n = path_compute_projection.facts.n;
  judgment_facts.k = path_compute_projection.facts.k;
  judgment_facts.work_units = path_compute_projection.facts.work_units;
  judgment_facts.can_stage = path_compute_projection.facts.can_stage;
  judgment_facts.can_direct = path_compute_projection.facts.can_direct;
  judgment_facts.allow_fallback = path_compute_projection.facts.allow_fallback;
  judgment_facts.readback_required = path_compute_projection.facts.readback_required;
  judgment_facts.force_direct = path_compute_projection.facts.force_direct;
  judgment_facts.force_staged = path_compute_projection.facts.force_staged;
  judgment_facts.force_tiled = path_compute_projection.facts.force_tiled;
  judgment_facts.tiled_shape = path_compute_projection.facts.tiled_shape;
  judgment_facts.software_vulkan = path_compute_projection.facts.software_vulkan;
  judgment_facts.policy_mode = (prom_policy_mode)path_compute_projection.facts.policy_mode;
  memset(&layout_precision_facts, 0, sizeof(layout_precision_facts));
  layout_precision_facts.packed4_available = 1u;
  layout_precision_facts.packed4_small_shape = packed4_small_shape;
  layout_precision_facts.packed4_padding_waste_permille = packed4_waste_permille;
  layout_precision_facts.packed4_mode_budget_permille = packed4_budget_permille;
  layout_precision_facts.packed4_row_major_valid = 1u;
  layout_precision_facts.packed4_tail_valid = 1u;
  layout_precision_facts.strict_fp32 = ((rt->test_flags & PROM_TESTCFG_FORCE_STRICT_FP32) != 0u) ? 1u : 0u;
  layout_precision_facts.tolerance_known = rt->sgemm_controller.fp16_tolerance_known;
  layout_precision_facts.tolerance_pass = rt->sgemm_controller.fp16_tolerance_pass;
  layout_precision_facts.has_special_values = fp16_has_special_values;
  layout_precision_facts.capability_fp16_storage = rt->capability_fp16_storage;
  layout_precision_facts.fallback_available = (path_compute_projection.facts.allow_fallback != 0u && path_compute_projection.facts.can_direct != 0u) ? 1u : 0u;
  layout_precision_facts.fp16_utility_score = fp16_utility_score;
  if (prom_dom_sgemm_stage_layout_precision_facts(&rt->blackboard, &layout_precision_facts) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  if (prom_dom_sgemm_build_layout_precision_facts_from_visible(&rt->blackboard, &layout_precision_facts, &layout_precision_projection) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  judgment_facts.packed4_available = layout_precision_projection.facts.packed4_available;
  judgment_facts.packed4_small_shape = layout_precision_projection.facts.packed4_small_shape;
  judgment_facts.packed4_padding_waste_permille = layout_precision_projection.facts.packed4_padding_waste_permille;
  judgment_facts.packed4_mode_budget_permille = layout_precision_projection.facts.packed4_mode_budget_permille;
  judgment_facts.packed4_row_major_valid = layout_precision_projection.facts.packed4_row_major_valid;
  judgment_facts.packed4_tail_valid = layout_precision_projection.facts.packed4_tail_valid;
  judgment_facts.strict_fp32 = layout_precision_projection.facts.strict_fp32;
  judgment_facts.tolerance_known = layout_precision_projection.facts.tolerance_known;
  judgment_facts.tolerance_pass = layout_precision_projection.facts.tolerance_pass;
  judgment_facts.has_special_values = layout_precision_projection.facts.has_special_values;
  judgment_facts.capability_fp16_storage = layout_precision_projection.facts.capability_fp16_storage;
  judgment_facts.fallback_available = layout_precision_projection.facts.fallback_available;
  judgment_facts.fp16_utility_score = layout_precision_projection.facts.fp16_utility_score;
  memset(&transfer_queue_facts, 0, sizeof(transfer_queue_facts));
  transfer_queue_facts.dedicated_transfer_available = rt->dedicated_transfer_available;
  transfer_queue_facts.transfer_queue_family_index = rt->transfer_queue_family_index;
  transfer_queue_facts.compute_queue_family_index = rt->queue_family_index;
  transfer_queue_facts.queue_families_differ =
      (rt->dedicated_transfer_available != 0u && rt->transfer_queue_family_index != rt->queue_family_index) ? 1u : 0u;
  transfer_queue_facts.transfer_queue_supported = rt->transfer_queue_enabled;
  transfer_queue_facts.transfer_queue_disabled_by_config = ((rt->test_flags & PROM_TESTCFG_DISABLE_TRANSFER_QUEUE) != 0u) ? 1u : 0u;
  transfer_queue_facts.transfer_workload_large_enough = work_units >= (uint64_t)PROM_JUDGMENT_STAGING_WORK_THRESHOLD ? 1u : 0u;
  transfer_queue_facts.transfer_sync_ownership_supported = rt->transfer_queue_enabled;
  transfer_queue_facts.transfer_fallback_available = 1u;
  transfer_queue_facts.upload_only_policy_eligible = readback_required == 0u ? 1u : 0u;
  transfer_queue_facts.upload_readback_supported = 0u;
  if (prom_dom_sgemm_stage_transfer_queue_facts(&rt->blackboard, &transfer_queue_facts) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  if (prom_dom_sgemm_build_transfer_queue_facts_from_visible(&rt->blackboard, &transfer_queue_facts, &transfer_queue_projection) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  judgment_facts.transfer_queue_dedicated_available = transfer_queue_projection.facts.dedicated_transfer_available;
  judgment_facts.transfer_queue_families_differ = transfer_queue_projection.facts.queue_families_differ;
  judgment_facts.transfer_queue_supported = transfer_queue_projection.facts.transfer_queue_supported;
  judgment_facts.transfer_overlap_slot_valid = transfer_queue_projection.facts.transfer_sync_ownership_supported;
  judgment_facts.transfer_workload_large_enough = transfer_queue_projection.facts.transfer_workload_large_enough;
  judgment_facts.transfer_fallback_available = transfer_queue_projection.facts.transfer_fallback_available;
  judgment_facts.transfer_queue_disabled_by_config = transfer_queue_projection.facts.transfer_queue_disabled_by_config;
  layout_precision_path_guard_dirty_mask = 0u;
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_SHAPE_M));
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_SHAPE_N));
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_SHAPE_K));
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_WORK_UNITS));
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_CAN_DIRECT));
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_ALLOW_FALLBACK));
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_TILED_SHAPE));
  layout_precision_path_guard_dirty_mask |=
      (path_compute_projection.dependent_dirty_key_mask_last_commit & (1ull << PROM_DOM_PATH_COMPUTE_DEP_POLICY_MODE));
  layout_precision_dependency_dirty_mask =
      layout_precision_projection.dependent_dirty_key_mask_last_commit | layout_precision_path_guard_dirty_mask;

  memset(&layout_precision_selector_decision, 0, sizeof(layout_precision_selector_decision));
  rt->layout_precision_selector_cache.last_dirty_dependency_mask = layout_precision_dependency_dirty_mask;
  rt->layout_precision_selector_cache.dependency_mask = layout_precision_dependency_dirty_mask;
  rt->layout_precision_selector_cache.last_decision_reused = 0u;
  if (selector_cache_enabled(rt) != 0u && layout_precision_projection.from_visible_snapshot != 0u &&
      rt->layout_precision_selector_cache.valid != 0u && layout_precision_dependency_dirty_mask == 0u &&
      rt->layout_precision_selector_cache.layout_precision_invalidation_count_when_computed == rt->slot_diag.m14_layout_precision_invalidation_count) {
    layout_precision_selector_decision = rt->layout_precision_selector_cache.decision;
    rt->layout_precision_selector_cache.reuse_count += 1u;
    rt->layout_precision_selector_cache.last_decision_reused = 1u;
  } else {
    if (rt->layout_precision_selector_cache.valid != 0u &&
        (layout_precision_dependency_dirty_mask != 0u ||
         rt->layout_precision_selector_cache.layout_precision_invalidation_count_when_computed != rt->slot_diag.m14_layout_precision_invalidation_count)) {
      rt->layout_precision_selector_cache.invalidation_count += 1u;
      rt->layout_precision_selector_cache.valid = 0u;
    }
    prom_judgment_engine_select_layout_precision(&judgment_facts, &layout_precision_selector_decision);
    rt->layout_precision_selector_cache.valid = 1u;
    rt->layout_precision_selector_cache.visible_generation_when_computed = layout_precision_projection.visible_generation;
    rt->layout_precision_selector_cache.layout_precision_invalidation_count_when_computed =
        rt->slot_diag.m14_layout_precision_invalidation_count;
    rt->layout_precision_selector_cache.decision = layout_precision_selector_decision;
    rt->layout_precision_selector_cache.recompute_count += 1u;
  }
  prom_judgment_engine_select_sgemm_mode_with_layout_precision(&judgment_facts, &layout_precision_selector_decision, &judgment_decision);
  if (judgment_decision.success == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, judgment_decision.error_detail);
    return PROM_ERROR;
  }
  memset(&path_compute_decision, 0, sizeof(path_compute_decision));
  path_compute_decision.success = judgment_decision.success;
  path_compute_decision.error_detail = judgment_decision.error_detail;
  path_compute_decision.requested_path = (uint32_t)judgment_decision.requested_path;
  path_compute_decision.selected_path = (uint32_t)judgment_decision.selected_path;
  path_compute_decision.compute_mode = (uint32_t)judgment_decision.compute_mode;
  path_compute_decision.final_detail = judgment_decision.final_detail;
  path_compute_decision.used_fallback_to_direct = judgment_decision.used_fallback_to_direct;
  path_compute_decision.winning_candidate_index = judgment_decision.winning_candidate_index;
  path_compute_decision.winning_score = judgment_decision.winning_score;
  if (prom_dom_sgemm_stage_path_compute_decision(&rt->blackboard, &path_compute_decision) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  if (prom_dom_sgemm_read_visible_path_compute_diagnostics(&rt->blackboard, &path_compute_snapshot) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  judgment_decision.success = path_compute_snapshot.decision.success;
  judgment_decision.error_detail = path_compute_snapshot.decision.error_detail;
  judgment_decision.requested_path = (prom_vk_path_mode)path_compute_snapshot.decision.requested_path;
  judgment_decision.selected_path = (prom_vk_path_mode)path_compute_snapshot.decision.selected_path;
  judgment_decision.compute_mode = (prom_vk_compute_mode)path_compute_snapshot.decision.compute_mode;
  judgment_decision.final_detail = path_compute_snapshot.decision.final_detail;
  judgment_decision.used_fallback_to_direct = path_compute_snapshot.decision.used_fallback_to_direct;
  judgment_decision.winning_candidate_index = path_compute_snapshot.decision.winning_candidate_index;
  judgment_decision.winning_score = path_compute_snapshot.decision.winning_score;
  selected_path = judgment_decision.selected_path;
  compute_mode = judgment_decision.compute_mode;
  final_detail = judgment_decision.final_detail;
  memset(&transfer_queue_decision, 0, sizeof(transfer_queue_decision));
  rt->transfer_selector_cache.last_dirty_dependency_mask = transfer_queue_projection.dependent_dirty_key_mask_last_commit;
  rt->transfer_selector_cache.dependency_mask = transfer_queue_projection.dependent_dirty_key_mask_last_commit;
  rt->transfer_selector_cache.last_decision_reused = 0u;
  if (selector_cache_enabled(rt) != 0u && transfer_queue_projection.from_visible_snapshot != 0u &&
      rt->transfer_selector_cache.valid != 0u && transfer_queue_projection.dependent_dirty_key_mask_last_commit == 0u &&
      rt->transfer_selector_cache.selected_path == (uint32_t)judgment_decision.selected_path) {
    transfer_queue_decision = rt->transfer_selector_cache.decision;
    rt->transfer_selector_cache.reuse_count += 1u;
    rt->transfer_selector_cache.last_decision_reused = 1u;
  } else {
    if (rt->transfer_selector_cache.valid != 0u && transfer_queue_projection.dependent_dirty_key_mask_last_commit != 0u) {
      rt->transfer_selector_cache.invalidation_count += 1u;
      rt->transfer_selector_cache.valid = 0u;
    }
    select_transfer_queue_policy(&judgment_decision, &transfer_queue_projection.facts, &transfer_queue_decision);
    rt->transfer_selector_cache.valid = 1u;
    rt->transfer_selector_cache.visible_generation_when_computed = transfer_queue_projection.visible_generation;
    rt->transfer_selector_cache.selected_path = (uint32_t)judgment_decision.selected_path;
    rt->transfer_selector_cache.decision = transfer_queue_decision;
    rt->transfer_selector_cache.recompute_count += 1u;
  }
  judgment_decision.use_dedicated_transfer_queue_upload = transfer_queue_decision.transfer_queue_used;
  judgment_decision.transfer_fallback_reason = transfer_queue_decision.transfer_fallback_reason;
  if (prom_dom_sgemm_stage_transfer_queue_decision(&rt->blackboard, &transfer_queue_decision) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  if (prom_dom_sgemm_read_visible_transfer_queue_diagnostics(&rt->blackboard, &transfer_queue_snapshot) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  use_dedicated_transfer_upload = transfer_queue_snapshot.transfer_queue_used;
  sync_transfer_diag_from_visible(rt);
  rt->sgemm_controller.packed4_tail_count_last = packed4_tail_count;
  rt->sgemm_controller.packed4_padded_lane_count_last = packed4_padded_lane_count;
  rt->sgemm_controller.packed4_padding_waste_permille_last = packed4_waste_permille;
  if (judgment_decision.packed4_reject_reason != PROM_PACKED4_REJECT_NONE) {
    prom_packed4_record_fallback(&rt->sgemm_controller, judgment_decision.packed4_reject_reason);
  }
  rt->sgemm_controller.fp16_fallback_reason_detail = prom_fp16_reject_reason_to_detail(judgment_decision.fp16_reject_reason);
  rt->sgemm_controller.fp16_selected_candidate = judgment_decision.fp16_selected != 0u ? 3u : 1u;
  memset(&layout_precision_decision, 0, sizeof(layout_precision_decision));
  layout_precision_decision.packed4_selected = layout_precision_selector_decision.packed4_selected;
  layout_precision_decision.packed4_reject_reason = (uint32_t)layout_precision_selector_decision.packed4_reject_reason;
  layout_precision_decision.fp16_selected = layout_precision_selector_decision.fp16_selected;
  layout_precision_decision.fp16_reject_reason = (uint32_t)layout_precision_selector_decision.fp16_reject_reason;
  layout_precision_decision.packed4_selected_layout_format = rt->sgemm_controller.packed4_selected_layout_format;
  layout_precision_decision.packed4_tail_count_last = rt->sgemm_controller.packed4_tail_count_last;
  layout_precision_decision.packed4_tail_count_total = rt->sgemm_controller.packed4_tail_count_total;
  layout_precision_decision.packed4_padded_lane_count_last = rt->sgemm_controller.packed4_padded_lane_count_last;
  layout_precision_decision.packed4_padded_lane_count_total = rt->sgemm_controller.packed4_padded_lane_count_total;
  layout_precision_decision.packed4_padding_waste_permille_last = rt->sgemm_controller.packed4_padding_waste_permille_last;
  layout_precision_decision.packed4_mode_budget_denials = rt->sgemm_controller.packed4_mode_budget_denials;
  layout_precision_decision.packed4_row_major_check_failures = rt->sgemm_controller.packed4_row_major_check_failures;
  layout_precision_decision.packed4_selection_count = rt->sgemm_controller.packed4_selection_count;
  layout_precision_decision.packed4_fallback_reason_padding_waste = rt->sgemm_controller.packed4_fallback_reason_padding_waste;
  layout_precision_decision.packed4_fallback_reason_small_shape = rt->sgemm_controller.packed4_fallback_reason_small_shape;
  layout_precision_decision.packed4_fallback_reason_capability_missing = rt->sgemm_controller.packed4_fallback_reason_capability_missing;
  layout_precision_decision.packed4_fallback_reason_fallback_required = rt->sgemm_controller.packed4_fallback_reason_fallback_required;
  layout_precision_decision.packed4_fallback_reason_mode_budget_denied = rt->sgemm_controller.packed4_fallback_reason_mode_budget_denied;
  layout_precision_decision.fp16_max_absolute_error = rt->sgemm_controller.fp16_max_absolute_error;
  layout_precision_decision.fp16_max_relative_error = rt->sgemm_controller.fp16_max_relative_error;
  layout_precision_decision.fp16_aggregate_error = rt->sgemm_controller.fp16_aggregate_error;
  layout_precision_decision.fp16_worst_case_element_index = rt->sgemm_controller.fp16_worst_case_element_index;
  layout_precision_decision.fp16_k_error_growth = rt->sgemm_controller.fp16_k_error_growth;
  layout_precision_decision.fp16_cancellation_risk = rt->sgemm_controller.fp16_cancellation_risk;
  layout_precision_decision.fp16_tolerance_known = rt->sgemm_controller.fp16_tolerance_known;
  layout_precision_decision.fp16_tolerance_pass = rt->sgemm_controller.fp16_tolerance_pass;
  layout_precision_decision.fp16_fallback_reason_detail = rt->sgemm_controller.fp16_fallback_reason_detail;
  layout_precision_decision.fp16_selected_candidate = rt->sgemm_controller.fp16_selected_candidate;
  if (prom_dom_sgemm_stage_layout_precision_decision(&rt->blackboard, &layout_precision_decision) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);

  compute_k = compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? prom_round_up4_u32(k) : k;
  if ((compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM &&
       (!checked_packed_fp16_buffer_size(m, compute_k, &a_buffer_size, &a_copy_size) ||
        !checked_packed_fp16_buffer_size(k, n, &b_buffer_size, &b_copy_size))) ||
      (compute_mode != PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM &&
       (!checked_float_buffer_size(m, compute_k, &a_buffer_size, &a_copy_size) ||
        !checked_float_buffer_size(compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? n : k,
                                   compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? compute_k : n,
                                   &b_buffer_size,
                                   &b_copy_size))) ||
      !checked_float_buffer_size(m, n, &c_buffer_size, &c_copy_size)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_DETAIL_SIZE_OVERFLOW);
    return PROM_ERROR;
  }
  artifact_layout_code = prom_slot_compute_layout_code(selected_path, compute_mode);
  artifact_precision_code = (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) ? 16u : 32u;
  artifact_a_key = make_artifact_key(PROM_BUFFER_ARTIFACT_A,
                                     m,
                                     n,
                                     k,
                                     compute_k,
                                     artifact_layout_code,
                                     artifact_precision_code,
                                     a_buffer_size);
  artifact_b_key = make_artifact_key(PROM_BUFFER_ARTIFACT_B,
                                     m,
                                     n,
                                     k,
                                     compute_k,
                                     artifact_layout_code,
                                     artifact_precision_code,
                                     b_buffer_size);
  artifact_c_key = make_artifact_key(PROM_BUFFER_ARTIFACT_C,
                                     m,
                                     n,
                                     k,
                                     compute_k,
                                     (uint32_t)PROM_VK_PATH_DIRECT,
                                     32u,
                                     c_buffer_size);
  required_capacity_bytes = (uint64_t)a_buffer_size + (uint64_t)b_buffer_size + (uint64_t)c_buffer_size;
  memset(&buffering_facts, 0, sizeof(buffering_facts));
  buffering_facts.required_fixed_slots_permille = 2000u;
  buffering_facts.required_pull_lag_peak_slots_permille = 1500u;
  buffering_facts.required_serial_slots_permille = 1000u;
  buffering_facts.fallback_available = judgment_facts.allow_fallback;
  if (judgment_facts.transfer_queue_dedicated_available != 0u && rt->software_vulkan == 0u) {
    variance_class = PROM_VARIANCE_LOW;
  } else if (rt->software_vulkan != 0u) {
    variance_class = PROM_VARIANCE_HIGH;
  } else {
    variance_class = PROM_VARIANCE_MODERATE;
  }
  if (policy_mode == PROM_POLICY_MODE_RECOVERY) {
    predictability_class = PROM_PREDICTABILITY_UNSTABLE;
  } else if (policy_mode == PROM_POLICY_MODE_SAFE) {
    predictability_class = PROM_PREDICTABILITY_TRACKED;
  } else {
    predictability_class = PROM_PREDICTABILITY_STABLE;
  }
  buffering_facts.transfer_variance_class = variance_class;
  buffering_facts.compute_predictability_class = predictability_class;
  buffering_facts.pull_lag_wip_waste_exceeded = rt->sgemm_controller.pending_waste_units > PROM_SGEMM_WASTE_BUDGET_UNITS ? 1u : 0u;
  buffering_facts.starvation_risk_high = rt->software_vulkan != 0u && work_units > (uint64_t)PROM_JUDGMENT_STAGING_WORK_THRESHOLD ? 1u : 0u;
  if (can_stage != 0u && can_direct != 0u) {
    buffering_facts.memory_budget_slots_permille = 2200u;
  } else if (can_stage != 0u || can_direct != 0u) {
    buffering_facts.memory_budget_slots_permille = 1400u;
  } else {
    buffering_facts.memory_budget_slots_permille = 800u;
  }
  if (policy_mode == PROM_POLICY_MODE_SAFE && buffering_facts.memory_budget_slots_permille >= 200u) {
    buffering_facts.memory_budget_slots_permille -= 200u;
  } else if (policy_mode == PROM_POLICY_MODE_RECOVERY && buffering_facts.memory_budget_slots_permille >= 400u) {
    buffering_facts.memory_budget_slots_permille -= 400u;
  }
  buffering_facts.fixed_double_headroom_slots_permille =
      (int32_t)buffering_facts.memory_budget_slots_permille - (int32_t)buffering_facts.required_fixed_slots_permille;
  buffering_facts.pull_lag_headroom_slots_permille =
      (int32_t)buffering_facts.memory_budget_slots_permille - (int32_t)buffering_facts.required_pull_lag_peak_slots_permille;
  buffering_facts.serial_jit_headroom_slots_permille =
      (int32_t)buffering_facts.memory_budget_slots_permille - (int32_t)buffering_facts.required_serial_slots_permille;
  if (prom_dom_sgemm_stage_m35_facts(&rt->blackboard, &buffering_facts) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  if (prom_dom_sgemm_build_buffering_selector_facts_from_visible(&rt->blackboard, &buffering_facts, &buffering_projection) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  rt->m35_selector_cache.last_dirty_dependency_mask = buffering_projection.dependent_dirty_key_mask_last_commit;
  rt->m35_selector_cache.dependency_mask = buffering_projection.dependent_dirty_key_mask_last_commit;
  rt->m35_selector_cache.last_decision_reused = 0u;
  if (selector_cache_enabled(rt) != 0u && buffering_projection.from_visible_snapshot != 0u && rt->m35_selector_cache.valid != 0u &&
      buffering_projection.dependent_dirty_key_mask_last_commit == 0u) {
    buffering_decision = rt->m35_selector_cache.decision;
    rt->m35_selector_cache.reuse_count += 1u;
    rt->m35_selector_cache.last_decision_reused = 1u;
  } else {
    if (rt->m35_selector_cache.valid != 0u && buffering_projection.dependent_dirty_key_mask_last_commit != 0u) {
      rt->m35_selector_cache.invalidation_count += 1u;
      rt->m35_selector_cache.valid = 0u;
    }
    prom_judgment_engine_select_buffering_mode(&buffering_projection.facts, &buffering_decision);
    rt->m35_selector_cache.valid = 1u;
    rt->m35_selector_cache.visible_generation_when_computed = buffering_projection.visible_generation;
    rt->m35_selector_cache.decision = buffering_decision;
    rt->m35_selector_cache.no_feasible_mode_detail = (uint32_t)prom_buffering_reason_to_detail(buffering_decision.final_reason_code);
    rt->m35_selector_cache.recompute_count += 1u;
  }
  if (rt->slot_diag.m35_selected_mode != (uint32_t)buffering_decision.selected_mode) {
    rt->slot_diag.m35_transition_count += 1u;
  }
  if (prom_dom_sgemm_stage_m35_decision(&rt->blackboard, &buffering_decision, rt->m35_selector_cache.no_feasible_mode_detail) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  prom_dom_sgemm_commit(&rt->blackboard);
  {
    prom_dom_sgemm_m35_snapshot m35_snapshot;
    if (prom_dom_sgemm_read_visible_m35(&rt->blackboard, &m35_snapshot) == 0u) {
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
      return PROM_ERROR;
    }
    rt->slot_diag.m35_selected_mode = m35_snapshot.selected_mode;
    rt->slot_diag.m35_fixed_feasible = m35_snapshot.fixed_feasible;
    rt->slot_diag.m35_pull_lag_feasible = m35_snapshot.pull_lag_feasible;
    rt->slot_diag.m35_serial_feasible = m35_snapshot.serial_feasible;
    rt->slot_diag.m35_fixed_rejected = m35_snapshot.fixed_rejected;
    rt->slot_diag.m35_pull_lag_rejected = m35_snapshot.pull_lag_rejected;
    rt->slot_diag.m35_serial_rejected = m35_snapshot.serial_rejected;
    rt->slot_diag.m35_fixed_score = m35_snapshot.fixed_score;
    rt->slot_diag.m35_pull_lag_score = m35_snapshot.pull_lag_score;
    rt->slot_diag.m35_serial_score = m35_snapshot.serial_score;
    rt->slot_diag.m35_reason_code = m35_snapshot.reason_code;
    rt->slot_diag.m35_final_reason_code = m35_snapshot.final_reason_code;
    rt->slot_diag.m35_fixed_double_rejection_reason = m35_snapshot.fixed_double_rejection_reason;
    rt->slot_diag.m35_pull_lag_rejection_reason = m35_snapshot.pull_lag_rejection_reason;
    rt->slot_diag.m35_serial_jit_rejection_reason = m35_snapshot.serial_jit_rejection_reason;
    rt->slot_diag.m35_memory_budget_slots_permille = m35_snapshot.memory_budget_slots_permille;
    rt->slot_diag.m35_required_fixed_slots_permille = m35_snapshot.required_fixed_slots_permille;
    rt->slot_diag.m35_required_pull_lag_slots_permille = m35_snapshot.required_pull_lag_peak_slots_permille;
    rt->slot_diag.m35_required_serial_slots_permille = m35_snapshot.required_serial_slots_permille;
    rt->slot_diag.m35_fixed_double_headroom_slots_permille = (int64_t)m35_snapshot.fixed_double_headroom_slots_permille;
    rt->slot_diag.m35_pull_lag_headroom_slots_permille = (int64_t)m35_snapshot.pull_lag_headroom_slots_permille;
    rt->slot_diag.m35_serial_jit_headroom_slots_permille = (int64_t)m35_snapshot.serial_jit_headroom_slots_permille;
  }
  if (buffering_decision.success == 0u) {
    rt->slot_diag.m35_rejection_count += 1u;
    rt->slot_diag.m35_budget_rejection_count += 1u;
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, prom_buffering_reason_to_detail(buffering_decision.reason_code));
    return PROM_ERROR;
  }
  buffering_mode = buffering_decision.selected_mode;
  if (buffering_mode == PROM_BUFFERING_MODE_PULL_LAG_PRESSURE) {
    rt->slot_diag.m35_pull_lag_predicted_demand_proxy_units += work_units;
    rt->slot_diag.m35_pull_lag_transfer_lead_proxy_units += work_units / 4u;
    rt->slot_diag.m35_pull_lag_safety_margin_proxy_units += work_units / 8u;
    rt->slot_diag.m35_pull_lag_stage_start_proxy_units += work_units / 16u;
    rt->slot_diag.m35_pull_lag_stage_complete_proxy_units += work_units / 16u + 1u;
    if (variance_class == PROM_VARIANCE_LOW) {
      rt->slot_diag.m35_pull_lag_early_stage_count += 1u;
      rt->slot_diag.m35_pull_lag_ready_unused_proxy_units += 1u;
    } else {
      rt->slot_diag.m35_pull_lag_late_stage_count += 1u;
      rt->slot_diag.m35_pull_lag_starvation_proxy_units += 1u;
    }
    if (buffering_facts.pull_lag_wip_waste_exceeded != 0u) {
      rt->slot_diag.m35_pull_lag_wip_waste_exceeded_count += 1u;
    }
  }
  if (buffering_mode == PROM_BUFFERING_MODE_SERIAL_JIT_SURVIVAL) {
    const uint32_t peer_slot_id = 1u;
    rt->slot_diag.m35_serial_sequential_step_count += 1u;
    rt->slot_diag.m35_serial_active_slot_count = 1u;
    if (!prom_slot_cleanup_to_empty(rt, &rt->slots[peer_slot_id])) {
      rt->slot_diag.m35_serial_busy_retry_count += 1u;
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED);
      return PROM_ERROR;
    }
    rt->slot_diag.m35_serial_failure_cleanup_count += 1u;
    rt->slot_diag.next_slot_id = 0u;
    rt->slot_diag.m35_serial_wip_depth = prom_slot_wip_depth(rt);
  }
  work_slot_id = rt->slot_diag.next_slot_id < 2u ? rt->slot_diag.next_slot_id : 0u;
  memset(&lease_facts, 0, sizeof(lease_facts));
  lease_facts.worker_id = 0u;
  lease_facts.slot_id = work_slot_id;
  lease_facts.entry_id = 0u;
  lease_facts.selected_recipe_variant = occupancy_decision.selected_variant;
  lease_facts.shape_class = occupancy_decision.shape_class;
  lease_facts.device_band = occupancy_decision.device_band;
  lease_facts.requested_resource_class = PROM_LEASE_RESOURCE_CLASS_COMPUTE;
  lease_facts.current_outstanding_depth = 0u;
  lease_facts.max_outstanding_depth = 1u;
  lease_facts.single_call_mode = 1u;
  lease_facts.lookahead_requested = rt->sgemm_controller.lookahead;
  lease_facts.lookahead_limit = prom_sgemm_default_config().lookahead_max;
  if (work_slot_id < 32u) {
    const uint32_t slot_mask = (1u << work_slot_id);
    /* Single-call path is explicitly preparing this slot for immediate dispatch.
     * Mark it ready/attention to avoid synthetic utility under-scoring. */
    lease_facts.ready_slot_mask = slot_mask;
    lease_facts.slot_attention_mask = slot_mask;
  }
  lease_facts.transfer_overlap_available = 1u;
  lease_facts.true_multi_queue_selected = 1u;
  prom_fill_lease_pressure_classes(rt,
                                   lease_facts.selected_recipe_variant,
                                   lease_facts.shape_class,
                                   lease_facts.device_band,
                                   work_units,
                                   &lease_facts);
  prepare_detail = prom_slot_prepare_for_call(rt,
                                              work_slot_id,
                                              m,
                                              n,
                                              compute_k,
                                              prom_slot_compute_layout_code(selected_path, compute_mode),
                                              (uint32_t)compute_mode,
                                              required_capacity_bytes);
  if (prepare_detail != 0) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, prepare_detail);
    return PROM_ERROR;
  }
  if (!prom_slot_swap_ready_to_current(rt, work_slot_id)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_SWAP_REJECTED);
    return PROM_ERROR;
  }

  if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
    packed_a_upload = (float*)malloc(a_copy_size);
    packed_b_upload = (float*)malloc(b_copy_size);
    if (packed_a_upload == NULL || packed_b_upload == NULL) {
      free(packed_a_upload);
      free(packed_b_upload);
      prom_slot_mark_failure(rt, work_slot_id, PROM_ERROR);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
      return PROM_ERROR;
    }
    prom_pack_a_packed4_rowmajor(a, packed_a_upload, m, k, compute_k);
    prom_pack_b_packed4_colmajor(b, packed_b_upload, n, k, compute_k);
  } else if (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
    fp16_a_upload = (uint32_t*)malloc(a_copy_size);
    fp16_b_upload = (uint32_t*)malloc(b_copy_size);
    if (fp16_a_upload == NULL || fp16_b_upload == NULL) {
      free(packed_a_upload);
      free(packed_b_upload);
      free(fp16_a_upload);
      free(fp16_b_upload);
      prom_slot_mark_failure(rt, work_slot_id, PROM_ERROR);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
      return PROM_ERROR;
    }
    prom_pack_fp16_pairs(a, m * compute_k, fp16_a_upload);
    prom_pack_fp16_pairs(b, compute_k * n, fp16_b_upload);
  }
  memset(&async_facts, 0, sizeof(async_facts));
  async_facts.request_async = request_async;
  async_facts.in_flight = rt->in_flight_submit;
  async_facts.software_vulkan = rt->software_vulkan;
  prom_judgment_engine_select_async_submission(&async_facts, &async_decision);
  if (async_decision.success == 0u) {
    free(packed_a_upload);
    free(packed_b_upload);
    free(fp16_a_upload);
    free(fp16_b_upload);
    prom_slot_mark_failure(rt, work_slot_id, async_decision.reject_detail);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, async_decision.reject_detail);
    return PROM_ERROR;
  }

  if (selected_path == PROM_VK_PATH_DIRECT) {
    if (!ensure_direct_execution_buffers(rt, &artifact_a_key, &artifact_b_key, &artifact_c_key, &vk_result)) {
      const int failure_detail = rt->arena_last_failure_detail != 0 ? rt->arena_last_failure_detail : (int)vk_result;
      prom_slot_mark_failure(rt, work_slot_id, failure_detail);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, failure_detail);
      free(packed_a_upload);
      free(packed_b_upload);
      free(fp16_a_upload);
      free(fp16_b_upload);
      return PROM_ERROR;
    }
    memcpy(rt->direct_a.mapped,
           compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? (const void*)packed_a_upload
           : (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM ? (const void*)fp16_a_upload : (const void*)a),
           a_copy_size);
    memcpy(rt->direct_b.mapped,
           compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? (const void*)packed_b_upload
           : (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM ? (const void*)fp16_b_upload : (const void*)b),
           b_copy_size);
    memset(rt->direct_c.mapped, 0, c_copy_size);
    shader_a = &rt->direct_a;
    shader_b = &rt->direct_b;
    shader_c = &rt->direct_c;
  } else {
    if (!ensure_staged_execution_buffers(rt, &artifact_a_key, &artifact_b_key, &artifact_c_key, &vk_result)) {
      const int failure_detail = rt->arena_last_failure_detail != 0 ? rt->arena_last_failure_detail : (int)vk_result;
      prom_slot_mark_failure(rt, work_slot_id, failure_detail);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, failure_detail);
      free(packed_a_upload);
      free(packed_b_upload);
      free(fp16_a_upload);
      free(fp16_b_upload);
      return PROM_ERROR;
    }
    memcpy(rt->staged_upload_a.mapped,
           compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? (const void*)packed_a_upload
           : (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM ? (const void*)fp16_a_upload : (const void*)a),
           a_copy_size);
    memcpy(rt->staged_upload_b.mapped,
           compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? (const void*)packed_b_upload
           : (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM ? (const void*)fp16_b_upload : (const void*)b),
           b_copy_size);
    shader_a = &rt->staged_device_a;
    shader_b = &rt->staged_device_b;
    shader_c = &rt->staged_device_c;
  }
  free(packed_a_upload);
  free(packed_b_upload);
  free(fp16_a_upload);
  free(fp16_b_upload);

  if (compute_mode == PROM_VK_COMPUTE_TILED) {
    if (requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE) {
      selected_pipeline = rt->srt_2accum_k_pipeline;
    } else if (requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4) {
      selected_pipeline = rt->b2x2_row_major_biased_pipeline;
    } else if (requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) {
      selected_pipeline = rt->a2x4_row_biased_accum8_pipeline;
    } else {
      selected_pipeline = rt->tiled_pipeline;
    }
    if (selected_path == PROM_VK_PATH_DIRECT) {
      final_detail = PROM_DETAIL_PATH_DIRECT_TILED;
    } else if (selected_path == PROM_VK_PATH_STAGED_UPLOAD) {
      final_detail = PROM_DETAIL_PATH_STAGED_UPLOAD_TILED;
    } else {
      final_detail = PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED;
    }
  } else if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
    selected_pipeline = rt->packed4_pipeline;
  } else if (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
    selected_pipeline = rt->fp16_pipeline;
  } else {
    selected_pipeline = rt->pipeline;
  }

  memset(buffer_infos, 0, sizeof(buffer_infos));
  buffer_infos[0].buffer = shader_a->buffer;
  buffer_infos[0].offset = 0;
  buffer_infos[0].range = shader_a->size;
  buffer_infos[1].buffer = shader_b->buffer;
  buffer_infos[1].offset = 0;
  buffer_infos[1].range = shader_b->size;
  buffer_infos[2].buffer = shader_c->buffer;
  buffer_infos[2].offset = 0;
  buffer_infos[2].range = shader_c->size;

  memset(writes, 0, sizeof(writes));
  writes[0].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
  writes[0].dstSet = rt->descriptor_set;
  writes[0].dstBinding = 0u;
  writes[0].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  writes[0].descriptorCount = 1u;
  writes[0].pBufferInfo = &buffer_infos[0];
  writes[1] = writes[0];
  writes[1].dstBinding = 1u;
  writes[1].pBufferInfo = &buffer_infos[1];
  writes[2] = writes[0];
  writes[2].dstBinding = 2u;
  writes[2].pBufferInfo = &buffer_infos[2];
  vkUpdateDescriptorSets(rt->device, 3u, writes, 0u, NULL);

  if (use_dedicated_transfer_upload != 0u && selected_path == PROM_VK_PATH_STAGED_UPLOAD) {
    stage_transfer_complete_telemetry(rt, 0u, work_slot_id, 0);
    vk_result = vkResetCommandBuffer(rt->transfer_command_buffer, 0u);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      stage_transfer_failure_telemetry(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    memset(&begin_info, 0, sizeof(begin_info));
    begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    vk_result = vkBeginCommandBuffer(rt->transfer_command_buffer, &begin_info);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      stage_transfer_failure_telemetry(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    memset(barriers, 0, sizeof(barriers));
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = rt->staged_upload_a.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->staged_upload_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->staged_upload_b.buffer;
    barriers[1].size = rt->staged_upload_b.size;
    vkCmdPipelineBarrier(rt->transfer_command_buffer, VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT, 0, 0, NULL, 2u, barriers,
                         0, NULL);
    memset(copies, 0, sizeof(copies));
    copies[0].size = rt->staged_upload_a.size;
    copies[1].size = rt->staged_upload_b.size;
    vkCmdCopyBuffer(rt->transfer_command_buffer, rt->staged_upload_a.buffer, rt->staged_device_a.buffer, 1u, &copies[0]);
    vkCmdCopyBuffer(rt->transfer_command_buffer, rt->staged_upload_b.buffer, rt->staged_device_b.buffer, 1u, &copies[1]);
    barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    barriers[0].dstAccessMask = 0u;
    barriers[0].srcQueueFamilyIndex = rt->transfer_queue_family_index;
    barriers[0].dstQueueFamilyIndex = rt->queue_family_index;
    barriers[0].buffer = rt->staged_device_a.buffer;
    barriers[0].size = rt->staged_device_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->staged_device_b.buffer;
    barriers[1].size = rt->staged_device_b.size;
    vkCmdPipelineBarrier(rt->transfer_command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT, 0, 0, NULL, 2u,
                         barriers, 0, NULL);
    vk_result = vkEndCommandBuffer(rt->transfer_command_buffer);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      stage_transfer_failure_telemetry(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    vk_result = vkResetFences(rt->device, 1u, &rt->transfer_submit_fence);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      stage_transfer_failure_telemetry(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    memset(&transfer_submit_info, 0, sizeof(transfer_submit_info));
    transfer_submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
    transfer_submit_info.commandBufferCount = 1u;
    transfer_submit_info.pCommandBuffers = &rt->transfer_command_buffer;
    transfer_submit_info.signalSemaphoreCount = 1u;
    transfer_submit_info.pSignalSemaphores = &rt->transfer_ready_semaphore;
    if ((rt->test_flags & PROM_TESTCFG_FAIL_TRANSFER_SUBMIT) != 0u) {
      prom_slot_mark_failure(rt, work_slot_id, VK_ERROR_DEVICE_LOST);
      stage_transfer_failure_telemetry(rt, work_slot_id, VK_ERROR_DEVICE_LOST);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, VK_ERROR_DEVICE_LOST);
      return PROM_ERROR;
    }
    vk_result = vkQueueSubmit(rt->transfer_queue, 1u, &transfer_submit_info, rt->transfer_submit_fence);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      stage_transfer_failure_telemetry(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    stage_transfer_handoff_telemetry(rt, work_slot_id, 0, 2u);
    vk_result = vkResetCommandBuffer(rt->command_buffer, 0u);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    memset(&begin_info, 0, sizeof(begin_info));
    begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    vk_result = vkBeginCommandBuffer(rt->command_buffer, &begin_info);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    memset(barriers, 0, sizeof(barriers));
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = 0u;
    barriers[0].dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
    barriers[0].srcQueueFamilyIndex = rt->transfer_queue_family_index;
    barriers[0].dstQueueFamilyIndex = rt->queue_family_index;
    barriers[0].buffer = rt->staged_device_a.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->staged_device_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->staged_device_b.buffer;
    barriers[1].size = rt->staged_device_b.size;
    barriers[2] = barriers[0];
    barriers[2].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[2].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[2].srcAccessMask = 0u;
    barriers[2].dstAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[2].buffer = rt->staged_device_c.buffer;
    barriers[2].size = rt->staged_device_c.size;
    vkCmdPipelineBarrier(rt->command_buffer, VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0, 0, NULL, 3u, barriers,
                         0, NULL);
  } else {
    stage_transfer_complete_telemetry(rt, 1u, work_slot_id, 0);
    vk_result = vkResetCommandBuffer(rt->command_buffer, 0u);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }

    memset(&begin_info, 0, sizeof(begin_info));
    begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    vk_result = vkBeginCommandBuffer(rt->command_buffer, &begin_info);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }

    memset(barriers, 0, sizeof(barriers));
    if (selected_path == PROM_VK_PATH_DIRECT) {
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = rt->direct_a.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->direct_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->direct_b.buffer;
    barriers[1].size = rt->direct_b.size;
    barriers[2] = barriers[0];
    barriers[2].dstAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[2].buffer = rt->direct_c.buffer;
    barriers[2].size = rt->direct_c.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         0,
                         0,
                         NULL,
                         3u,
                         barriers,
                         0,
                         NULL);
    } else {
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = rt->staged_upload_a.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->staged_upload_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->staged_upload_b.buffer;
    barriers[1].size = rt->staged_upload_b.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         VK_PIPELINE_STAGE_TRANSFER_BIT,
                         0,
                         0,
                         NULL,
                         2u,
                         barriers,
                         0,
                         NULL);

    memset(copies, 0, sizeof(copies));
    copies[0].size = rt->staged_upload_a.size;
    copies[1].size = rt->staged_upload_b.size;
    vkCmdCopyBuffer(rt->command_buffer, rt->staged_upload_a.buffer, rt->staged_device_a.buffer, 1u, &copies[0]);
    vkCmdCopyBuffer(rt->command_buffer, rt->staged_upload_b.buffer, rt->staged_device_b.buffer, 1u, &copies[1]);

    barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
    barriers[0].buffer = rt->staged_device_a.buffer;
    barriers[0].size = rt->staged_device_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->staged_device_b.buffer;
    barriers[1].size = rt->staged_device_b.size;
    barriers[2] = barriers[0];
    barriers[2].dstAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[2].buffer = rt->staged_device_c.buffer;
    barriers[2].size = rt->staged_device_c.size;
    /* Staged device-local C is not pre-zeroed: current SGEMM kernels overwrite every final C element. */
      vkCmdPipelineBarrier(rt->command_buffer,
                           VK_PIPELINE_STAGE_TRANSFER_BIT,
                           VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           0,
                           0,
                           NULL,
                           3u,
                           barriers,
                           0,
                           NULL);
    }
  }

  vkCmdBindPipeline(rt->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, selected_pipeline);
  vkCmdBindDescriptorSets(rt->command_buffer,
                          VK_PIPELINE_BIND_POINT_COMPUTE,
                          rt->pipeline_layout,
                          0u,
                          1u,
                          &rt->descriptor_set,
                          0u,
                          NULL);

  push.m = m;
  push.n = n;
  push.k = compute_k;
  vkCmdPushConstants(rt->command_buffer,
                     rt->pipeline_layout,
                     VK_SHADER_STAGE_COMPUTE_BIT,
                     0u,
                     PROM_VK_SHADER_PUSH_BYTES,
                     &push);

  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, 0);
  if (work_slot_id < 32u) {
    const uint32_t slot_mask = (1u << work_slot_id);
    lease_facts.ready_slot_mask = slot_mask;
    lease_facts.slot_attention_mask = slot_mask;
  }
  lease_facts.failed_slot_mask = 0u;
  lease_facts.invalidated_slot_mask = 0u;
  lease_facts.unsafe_to_reuse = 0u;
  if (prom_runtime_request_resource_lease(rt, &lease_facts, &lease_decision) == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_ERROR);
    return PROM_ERROR;
  }
  lease_granted = lease_decision.grant;
  if (lease_granted == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED);
    return PROM_ERROR;
  }
  lease_facts.lease_held = 1u;
  lease_facts.current_outstanding_depth = 1u;
  if ((rt->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_slot_mark_failure(rt, work_slot_id, PROM_DETAIL_INJECTED_DISPATCH_FAILURE);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_INJECTED_DISPATCH_FAILURE);
    return PROM_ERROR;
  }

  if (rt->timestamp_query_supported != 0u && rt->sgemm_timestamp_query_pool != VK_NULL_HANDLE) {
    vkCmdResetQueryPool(rt->command_buffer, rt->sgemm_timestamp_query_pool, 0u, 2u);
    vkCmdWriteTimestamp(rt->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, rt->sgemm_timestamp_query_pool, 0u);
  }

  /* Dispatch/indexing contract: x maps rows (m), y maps columns (n); host and shader must match this. */
  vkCmdDispatch(rt->command_buffer,
                (m + (PROM_VK_LOCAL_SIZE_X - 1u)) / PROM_VK_LOCAL_SIZE_X,
                (n + (PROM_VK_LOCAL_SIZE_Y - 1u)) / PROM_VK_LOCAL_SIZE_Y,
                1u);
  if (rt->timestamp_query_supported != 0u && rt->sgemm_timestamp_query_pool != VK_NULL_HANDLE) {
    vkCmdWriteTimestamp(rt->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, rt->sgemm_timestamp_query_pool, 1u);
  }

  if (selected_path == PROM_VK_PATH_DIRECT) {
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_HOST_READ_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = rt->direct_c.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->direct_c.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         0,
                         0,
                         NULL,
                         1u,
                         barriers,
                         0,
                         NULL);
  } else if (selected_path == PROM_VK_PATH_STAGED_UPLOAD_READBACK) {
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = rt->staged_device_c.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->staged_device_c.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         VK_PIPELINE_STAGE_TRANSFER_BIT,
                         0,
                         0,
                         NULL,
                         1u,
                         barriers,
                         0,
                         NULL);

    copies[2].size = rt->staged_readback_c.size;
    vkCmdCopyBuffer(rt->command_buffer, rt->staged_device_c.buffer, rt->staged_readback_c.buffer, 1u, &copies[2]);

    barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_HOST_READ_BIT;
    barriers[0].buffer = rt->staged_readback_c.buffer;
    barriers[0].size = rt->staged_readback_c.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_TRANSFER_BIT,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         0,
                         0,
                         NULL,
                         1u,
                         barriers,
                         0,
                         NULL);
  }

  if ((rt->test_flags & PROM_TESTCFG_FAIL_COMMAND_END) != 0u) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_slot_mark_failure(rt, work_slot_id, VK_ERROR_DEVICE_LOST);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, VK_ERROR_DEVICE_LOST);
    return PROM_ERROR;
  }
  vk_result = vkEndCommandBuffer(rt->command_buffer);
  if (vk_result != VK_SUCCESS) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }

  if ((rt->test_flags & PROM_TESTCFG_FAIL_RESET_FENCE) != 0u) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_slot_mark_failure(rt, work_slot_id, VK_ERROR_DEVICE_LOST);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, VK_ERROR_DEVICE_LOST);
    return PROM_ERROR;
  }
  vk_result = vkResetFences(rt->device, 1u, &rt->submit_fence);
  if (vk_result != VK_SUCCESS) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }

  memset(&submit_info, 0, sizeof(submit_info));
  submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit_info.commandBufferCount = 1u;
  submit_info.pCommandBuffers = &rt->command_buffer;
  if (use_dedicated_transfer_upload != 0u) {
    wait_stage_mask = VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
    submit_info.waitSemaphoreCount = 1u;
    submit_info.pWaitSemaphores = &rt->transfer_ready_semaphore;
    submit_info.pWaitDstStageMask = &wait_stage_mask;
    stage_transfer_wait_telemetry(rt, work_slot_id, 0);
  }
  if ((rt->test_flags & PROM_TESTCFG_FAIL_QUEUE_SUBMIT) != 0u) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_slot_mark_failure(rt, work_slot_id, VK_ERROR_DEVICE_LOST);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, VK_ERROR_DEVICE_LOST);
    return PROM_ERROR;
  }
  vk_result = vkQueueSubmit(rt->compute_queue, 1u, &submit_info, rt->submit_fence);
  if (vk_result != VK_SUCCESS) {
    reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
    prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }
  rt->in_flight_submit = 1u;
  if (!prom_slot_mark_submitted(rt, work_slot_id)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
    prom_slot_mark_failure(rt, work_slot_id, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
    return PROM_ERROR;
  }

  if (async_decision.execute_async != 0u) {
    rt->async_task_id += 1;
    rt->async_m = m;
    rt->async_n = n;
    rt->async_k = k;
    rt->async_c_copy_size = c_copy_size;
    rt->async_selected_path = selected_path;
    rt->async_final_detail = final_detail;
    note_last_execution_shape(rt, m, n, k);
    set_async_state(rt, PROM_ASYNC_STATE_SUBMITTED, PROM_STAGE_SUBMIT, 0);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, final_detail);
    return PROM_OK;
  }

  if ((rt->test_flags & PROM_TESTCFG_SKIP_SUBMIT_WAIT) == 0u) {
    if (use_dedicated_transfer_upload != 0u) {
      vk_result = vkWaitForFences(rt->device, 1u, &rt->transfer_submit_fence, VK_TRUE, UINT64_MAX);
      if (vk_result != VK_SUCCESS) {
        prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
        stage_transfer_failure_telemetry(rt, work_slot_id, (int)vk_result);
        prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
        return PROM_ERROR;
      }
      stage_transfer_complete_telemetry(rt, 1u, work_slot_id, 0);
    }
    vk_result = vkWaitForFences(rt->device, 1u, &rt->submit_fence, VK_TRUE, UINT64_MAX);
    if (vk_result != VK_SUCCESS) {
      reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED);
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    if (rt->timestamp_query_supported != 0u && rt->sgemm_timestamp_query_pool != VK_NULL_HANDLE) {
      uint64_t timestamps[2];
      vk_result = vkGetQueryPoolResults(rt->device,
                                        rt->sgemm_timestamp_query_pool,
                                        0u,
                                        2u,
                                        sizeof(timestamps),
                                        timestamps,
                                        sizeof(uint64_t),
                                        VK_QUERY_RESULT_64_BIT);
      if (vk_result != VK_SUCCESS) {
        reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_UNAVAILABLE);
      } else if (rt->timestamp_period_ns <= 0.0f) {
        reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_PERIOD);
      } else if (timestamps[1] <= timestamps[0]) {
        reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_ORDER);
      } else {
        const double duration = ((double)(timestamps[1] - timestamps[0])) * (double)rt->timestamp_period_ns;
        if (duration <= 0.0) {
          reset_last_gpu_timing(rt, PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_ORDER);
        } else {
          rt->last_gpu_timing_valid = 1u;
          rt->last_gpu_timing_failure_reason = PROM_SGEMM_GPU_TIMING_FAILURE_NONE;
          rt->last_gpu_duration_ns = (uint64_t)duration;
          rt->p14_measurement_tick += 1u;
          rt->p14_last_filtered_evidence =
              prom_dominatus_measurement_filter_update(&rt->p14_measurement_filter_state, duration, rt->p14_measurement_tick);
          {
            prom_dominatus_predictor_evidence pe =
                prom_dominatus_predictor_evidence_from_filtered(&rt->p14_last_filtered_evidence);
            prom_dominatus_physical_observation po;
            memset(&po, 0, sizeof(po));
            po.tick = rt->p14_measurement_tick;
            po.actual_ready = 1u;
            po.slot_valid = 1u;
            po.memory_budget_ok = 1u;
            po.outstanding_depth_cap = rt->p15_predictor_state.params.max_outstanding_depth;
            memset(&rt->p15_last_prediction_issued, 0, sizeof(rt->p15_last_prediction_issued));
            rt->p15_last_correction = prom_dominatus_predictor_update(&rt->p15_predictor_state, &pe, &po, po.tick,
                                                                       &rt->p15_last_prediction_issued);
            (void)prom_dominatus_predictor_advance_reservations(&rt->p15_predictor_state, po.tick);
            rt->p15_last_reservation = prom_dominatus_predictor_try_reserve_future(
                &rt->p15_predictor_state,
                &rt->p15_predictor_state.reservations,
                &rt->p15_predictor_state.future_lease_seam.last_request,
                po.tick);
            {
              prom_dominatus_prestage_input pi;
              memset(&pi, 0, sizeof(pi));
              pi.valid = rt->p15_last_reservation.valid;
              pi.request_id = rt->p15_last_reservation.request_id;
              pi.current_tick = po.tick;
              pi.target_tick = rt->p15_last_reservation.target_tick;
              pi.reservation_is_reserved = rt->p15_last_reservation.reserved;
              pi.confidence = rt->p15_last_reservation.confidence;
              pi.warmup = pe.warmup;
              pi.recent_miss_count = (uint32_t)rt->p15_predictor_state.correction_count;
              pi.slot_valid = po.slot_valid;
              pi.memory_budget_ok = po.memory_budget_ok;
              pi.outstanding_depth = po.outstanding_depth;
              pi.outstanding_depth_cap = po.outstanding_depth_cap;
              pi.resource_pressure_low = 1u;
              rt->p15_last_prestage = prom_dominatus_prestage_evaluate(&rt->p15_prestage_params, &pi);
            }
            rt->p15_last_shadow = prom_dominatus_shadow_snapshot_evaluate(&rt->p15_predictor_state,
                                                                           &rt->p15_last_prediction_issued,
                                                                           &rt->p15_last_correction,
                                                                           &rt->p15_last_reservation,
                                                                           rt->p15_last_prestage.allowed,
                                                                           po.tick);
            prom_dominatus_shadow_calibration_update(&rt->p15_shadow_calibration, &rt->p15_last_shadow);
            rt->p15_shadow_authority_gate = prom_dominatus_shadow_authority_gate_evaluate_with_enabled(
                &rt->p15_shadow_calibration, rt->p15_shadow_canary_params.enabled != 0u ? 1u : 0u);
            if (rt->p15_shadow_canary_params.enabled != 0u &&
                rt->p15_shadow_authority_gate.state == PROM_SHADOW_AUTHORITY_HEALTHY &&
                rt->p15_shadow_authority_gate.recommended_lookahead_depth > 0u) {
              uint32_t depth = rt->p15_shadow_authority_gate.recommended_lookahead_depth;
              if (depth < 1u) depth = 1u;
              if (depth > 4u) depth = 4u;
              rt->p15_predictor_state.lookahead_depth = depth;
            }
            prom_dominatus_shadow_would_act_update(
                &rt->p15_shadow_would_act_state, &rt->p15_shadow_authority_gate, &rt->p15_shadow_calibration, &rt->p15_last_shadow);
            if (prom_dominatus_shadow_canary_should_attempt(&rt->p15_shadow_canary_state,
                                                            &rt->p15_shadow_canary_params,
                                                            &rt->p15_shadow_authority_gate,
                                                            &rt->p15_shadow_calibration,
                                                            &rt->p15_last_shadow) != 0u) {
              if (rt->p15_predictor_state.future_lease_seam.last_request.valid == 0u) {
                rt->p15_shadow_canary_state.action_blocked_count += 1u;
                rt->p15_shadow_canary_state.block_no_future_lease_count += 1u;
              } else {
                prom_dominatus_reservation_decision canary_reservation = prom_dominatus_predictor_try_reserve_future(
                    &rt->p15_predictor_state,
                    &rt->p15_predictor_state.reservations,
                    &rt->p15_predictor_state.future_lease_seam.last_request,
                    po.tick);
                rt->p15_shadow_canary_state.reservation_attempt_count += 1u;
                if (canary_reservation.valid != 0u && canary_reservation.reserved != 0u) {
                  rt->p15_shadow_canary_state.action_applied_count += 1u;
                  rt->p15_shadow_canary_state.reservation_success_count += 1u;
                  rt->p15_shadow_canary_state.last_applied_issued_tick = rt->p15_last_shadow.issued_tick;
                  rt->p15_shadow_canary_state.last_applied_target_tick = rt->p15_last_shadow.target_tick;
                  rt->p15_shadow_canary_state.last_applied_predicted_ready_tick = rt->p15_last_shadow.predicted_ready_tick;
                } else {
                  rt->p15_shadow_canary_state.action_blocked_count += 1u;
                  rt->p15_shadow_canary_state.block_reservation_failed_count += 1u;
                  rt->p15_shadow_canary_state.reservation_rejected_count += 1u;
                }
              }
            } else if (rt->p15_shadow_canary_params.enabled == 0u) {
              rt->p15_shadow_canary_state.block_disabled_count += 1u;
            }
          }
        }
      }
    }
    rt->in_flight_submit = 0u;
    if (!prom_slot_mark_complete(rt, work_slot_id)) {
      prom_slot_mark_failure(rt, work_slot_id, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
      return PROM_ERROR;
    }
  } else {
    prom_slot_mark_failure(rt, work_slot_id, PROM_DETAIL_REUSE_IN_FLIGHT);
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_REUSE_IN_FLIGHT);
    return PROM_ERROR;
  }

  if ((rt->test_flags & PROM_TESTCFG_FAIL_DOWNLOAD) != 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_DETAIL_INJECTED_DOWNLOAD_FAILURE);
    return PROM_ERROR;
  }

  if (selected_path == PROM_VK_PATH_DIRECT) {
    if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
      prom_apply_debug_row_major_oracle(rt, a, b, (float*)rt->direct_c.mapped, m, n, k);
    }
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, final_detail);
    memcpy(c, rt->direct_c.mapped, c_copy_size);
  } else if (selected_path == PROM_VK_PATH_STAGED_UPLOAD_READBACK) {
    if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
      prom_apply_debug_row_major_oracle(rt, a, b, (float*)rt->staged_readback_c.mapped, m, n, k);
    }
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, final_detail);
    memcpy(c, rt->staged_readback_c.mapped, c_copy_size);
  } else {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, final_detail);
  }

  if (out_stage != NULL &&
      out_detail_code != NULL &&
      ((*out_stage == PROM_STAGE_TRANSFER_OUT) || (selected_path == PROM_VK_PATH_STAGED_UPLOAD && *out_stage == PROM_STAGE_SUBMIT))) {
    if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
      rt->sgemm_controller.packed4_selected_layout_format = 2u;
      rt->sgemm_controller.packed4_tail_count_total += (uint64_t)packed4_tail_count;
      rt->sgemm_controller.packed4_padded_lane_count_total += (uint64_t)packed4_padded_lane_count;
      rt->sgemm_controller.packed4_selection_count += 1u;
      rt->sgemm_controller.fp16_selected_candidate = 2u;
    } else if (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
      rt->sgemm_controller.packed4_selected_layout_format = 1u;
      rt->sgemm_controller.fp16_selected_candidate = 3u;
      rt->sgemm_controller.fp16_fallback_reason_detail = 0;
    } else {
      rt->sgemm_controller.packed4_selected_layout_format = 1u;
      rt->sgemm_controller.fp16_selected_candidate = 1u;
    }
    layout_precision_decision.packed4_selected_layout_format = rt->sgemm_controller.packed4_selected_layout_format;
    layout_precision_decision.packed4_tail_count_total = rt->sgemm_controller.packed4_tail_count_total;
    layout_precision_decision.packed4_padded_lane_count_total = rt->sgemm_controller.packed4_padded_lane_count_total;
    layout_precision_decision.packed4_selection_count = rt->sgemm_controller.packed4_selection_count;
    layout_precision_decision.fp16_fallback_reason_detail = rt->sgemm_controller.fp16_fallback_reason_detail;
    layout_precision_decision.fp16_selected_candidate = rt->sgemm_controller.fp16_selected_candidate;
    if ((compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 || compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) &&
        prom_dom_sgemm_stage_layout_precision_decision(&rt->blackboard, &layout_precision_decision) != 0u) {
      prom_dom_sgemm_commit(&rt->blackboard);
    }
    if (lease_granted != 0u) {
      lease_facts.single_call_mode = 1u;
      lease_facts.yield_requested = 1u;
      lease_facts.lease_held = 1u;
      lease_facts.current_outstanding_depth = 1u;
      lease_facts.max_outstanding_depth = 1u;
      if (prom_runtime_request_resource_lease(rt, &lease_facts, &lease_yield_decision) == 0u) {
        prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_CLEANUP, PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED);
        return PROM_ERROR;
      }
      lease_facts.lease_held = 0u;
      lease_facts.current_outstanding_depth = 0u;
    }
    note_last_execution_shape(rt, m, n, k);
    return PROM_OK;
  }
  return PROM_ERROR;
}

// ============================================================================
// SGEMM Batch Dispatch / Worker Runtime
// ============================================================================

int prom_reactor_runtime_sgemm_batch_impl(void* handle,
                                          const PrometheusSgemmBatchEntry* entries,
                                          uint32_t entry_count,
                                          uint32_t flags,
                                          uint32_t* out_stage,
                                          int* out_detail_code) {
  prometheus_runtime* rt;
  prom_batch_plan* plans = NULL;
  float** staged_outputs = NULL;
  uint32_t* output_sizes = NULL;
  prom_batch_worker_event* worker_events = NULL;
  uint32_t* worker_event_counts = NULL;
  prom_batch_worker_state* workers = NULL;
  prom_batch_worker_resources* worker_resources = NULL;
  prom_batch_slot_runtime* worker_slots = NULL;
  uint32_t* worker_slot_refill_cursor = NULL;
  prom_batch_thread* worker_threads = NULL;
  prom_batch_thread_ctx* worker_thread_ctx = NULL;
  uint32_t requested_workers;
  uint32_t hardware_queue_cap = 1u;
  uint32_t memory_worker_cap;
  uint32_t per_worker_arena_bytes;
  uint32_t effective_workers;
  uint32_t event_capacity;
  uint32_t slots_per_worker_target = 2u;
  uint32_t effective_slots_per_worker = 2u;
  uint32_t total_slot_count = 0u;
  uint32_t slot_cap_reason = PROM_BATCH_SLOT_CAP_REASON_NONE;
  uint32_t slot_refill_count = 0u;
  uint32_t slot_full_scan_poll_count = 0u;
  uint32_t slot_attention_poll_count = 0u;
  uint32_t slot_polling_avoided_count = 0u;
  uint32_t slot_failure_count = 0u;
  uint32_t slot_drain_count = 0u;
  uint32_t slot_boundary_generation = 0u;
  uint32_t slot_dirty_mask = 0u;
  uint32_t slot_ready_mask = 0u;
  uint32_t slot_failed_mask = 0u;
  uint32_t slot_invalidated_mask = 0u;
  uint32_t slot_attention_mask = 0u;
  uint32_t injected_failure_entry_id;
  uint32_t force_dual_fail_first_two;
  uint32_t delay_entry0;
  uint32_t i;
  uint32_t w;
  uint32_t state = PROM_BATCH_STATE_PENDING;
  uint32_t failed_entry_id = UINT32_MAX;
  uint32_t failed_worker_id = UINT32_MAX;
  uint32_t failure_stage = PROM_STAGE_NONE;
  int failure_detail = 0;
  uint32_t output_committed = 0u;
  uint32_t event_overflow = 0u;
  uint32_t event_drain_count = 0u;
  uint32_t cap_reason = PROM_BATCH_CAP_REASON_NONE;
  uint32_t execution_mode = PROM_BATCH_EXECUTION_SINGLE_WORKER;
  uint32_t worker_resource_mode = PROM_BATCH_WORKER_RESOURCE_SIMULATED;
  uint32_t queue_topology_classification = PROM_BATCH_QUEUE_TOPOLOGY_SINGLE_QUEUE;
  uint32_t queue_mapping_mode = PROM_BATCH_QUEUE_MAPPING_SINGLE_QUEUE_SERIALIZED;
  uint32_t serialized_fallback_reason = PROM_BATCH_FALLBACK_REASON_BASELINE_SERIALIZED;
  uint32_t resource_mode = PROM_BATCH_WORKER_RESOURCE_MODE_SIMULATED_PER_WORKER;
  uint32_t lane_worker_count = 0u;
  uint32_t real_worker_thread_count = 0u;
  uint32_t serialized_vulkan = 0u;
  uint32_t serialized_execution_count = 0u;
  uint32_t serialized_wait_count = 0u;
  uint32_t max_concurrent_serialized_entries = 0u;
  uint32_t serialized_bridge_enter_count = 0u;
  uint32_t failure_count = 0u;
  uint32_t hardware_parallelism_claimed = 0u;
  uint32_t hardware_parallelism_eligible = 0u;
  uint32_t true_multi_queue_selected = 0u;
  uint32_t reported_compute_queue_count = 1u;
  uint32_t independent_compute_queue_count = 1u;
  uint32_t effective_workers_for_gates = 1u;
  uint32_t per_worker_command_resources_valid = 0u;
  uint32_t per_worker_fences_valid = 0u;
  uint32_t worker_queue_mapping_valid = 0u;
  uint32_t memory_budget_supports_workers = 0u;
  uint32_t no_pseudo_shared_queue = 0u;
  uint32_t no_forced_serialized_flag = 0u;
  uint32_t queue_family_ownership_handoff_if_needed = 1u;
  uint32_t queue_drain_count = 0u;
  uint32_t drain_timeout_count = 0u;
  uint32_t queue_family_ownership_handoff_count = 0u;
  uint32_t transfer_compute_sync_wait_count = 0u;
  uint32_t unsafe_to_reuse = 0u;
  uint32_t resource_ownership_violation_count = 0u;
  uint32_t resource_creation_failure_count = 0u;
  uint32_t use_real_threads = 0u;
  uint32_t force_wrong_resource_owner = 0u;
  uint32_t inject_invalidated_ready_slot = 0u;
  uint32_t inject_presubmit_slot_failure = 0u;
  uint32_t inject_thread_start_failure = 0u;
  uint32_t inject_staged_output_alloc_failure = 0u;
  uint32_t started_worker_count = 0u;
  uint64_t lease_request_count = 0u;
  uint64_t lease_grant_count = 0u;
  uint64_t lease_deny_count = 0u;
  uint64_t lease_yield_count = 0u;
  uint32_t lease_last_state = PROM_LEASE_STATE_NONE;
  uint32_t lease_last_deny_reason = PROM_LEASE_REASON_NONE;
  uint32_t lease_last_lookahead_requested = 0u;
  uint32_t lease_last_lookahead_allowed = 0u;
  uint32_t lease_last_lookahead_blocked_reason = PROM_LEASE_REASON_NONE;
  uint32_t lease_last_selected_recipe_variant = 0u;
  uint32_t lease_held_mask = 0u;
  uint32_t lease_outstanding_depth = 0u;
  prom_batch_shared_state shared_state;
  prom_batch_event_drain_summary drain_summary = {0u, 0u, 0u};
  int final_status = PROM_ERROR;

  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_NONE, 0);
  if (handle == NULL || !registry_contains(handle)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  if (entries == NULL || entry_count == 0u) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_ERROR);
    return PROM_ERROR;
  }

  requested_workers = batch_requested_workers_from_flags(flags);
  per_worker_arena_bytes = batch_test_per_worker_arena_bytes(flags);
  event_capacity = batch_test_event_capacity_from_flags(flags);
  injected_failure_entry_id = batch_test_failure_entry_from_flags(flags, entry_count);
  force_dual_fail_first_two = batch_test_force_dual_fail_first_two(flags);
  delay_entry0 = batch_test_delay_entry0(flags);
  if (batch_test_hardware_queue_cap_override(flags) != 0u) {
    hardware_queue_cap = batch_test_hardware_queue_cap_override(flags);
  }
  memory_worker_cap = (uint32_t)(rt->arena_budget_limit_bytes / (uint64_t)per_worker_arena_bytes);
  effective_workers = requested_workers;
  if (effective_workers > hardware_queue_cap) {
    effective_workers = hardware_queue_cap;
    cap_reason = (hardware_queue_cap == 1u) ? PROM_BATCH_CAP_REASON_SINGLE_QUEUE_CONSERVATIVE : PROM_BATCH_CAP_REASON_HARDWARE_QUEUE;
  }
  if (effective_workers > memory_worker_cap) {
    effective_workers = memory_worker_cap;
    cap_reason = PROM_BATCH_CAP_REASON_MEMORY_BUDGET;
  }
  if (effective_workers > 0u) {
    uint64_t slot_budget = rt->arena_budget_limit_bytes / (uint64_t)per_worker_arena_bytes;
    uint32_t slots_budget_per_worker = (uint32_t)(slot_budget / (uint64_t)effective_workers);
    effective_slots_per_worker = slots_per_worker_target;
    if (slots_budget_per_worker < effective_slots_per_worker) {
      effective_slots_per_worker = slots_budget_per_worker;
      slot_cap_reason = PROM_BATCH_SLOT_CAP_REASON_MEMORY_BUDGET;
    }
    if (effective_slots_per_worker == 0u) {
      effective_slots_per_worker = 1u;
      slot_cap_reason = PROM_BATCH_SLOT_CAP_REASON_MEMORY_BUDGET;
    }
    if (effective_slots_per_worker > slots_per_worker_target) {
      effective_slots_per_worker = slots_per_worker_target;
    }
  }
  total_slot_count = effective_workers * effective_slots_per_worker;
  reported_compute_queue_count = rt->reported_compute_queue_count;
  if (reported_compute_queue_count == 0u) {
    reported_compute_queue_count = 1u;
  }
  independent_compute_queue_count = rt->independent_compute_queue_count;
  if (independent_compute_queue_count == 0u) {
    independent_compute_queue_count = 1u;
  }
  if ((rt->test_flags & PROM_TESTCFG_FORCE_DIRECT_PATH) != 0u && batch_test_hardware_queue_cap_override(flags) > 1u) {
    independent_compute_queue_count = batch_test_hardware_queue_cap_override(flags);
    if (independent_compute_queue_count > 8u) {
      independent_compute_queue_count = 8u;
    }
    if (reported_compute_queue_count < independent_compute_queue_count) {
      reported_compute_queue_count = independent_compute_queue_count;
    }
  }
  memory_budget_supports_workers = memory_worker_cap >= 2u ? 1u : 0u;
  no_forced_serialized_flag = ((rt->test_flags & PROM_TESTCFG_P11_BATCH_FORCE_LANE_SIMULATED) == 0u) ? 1u : 0u;
  effective_workers_for_gates = effective_workers;
  if (effective_workers_for_gates > independent_compute_queue_count) {
    effective_workers_for_gates = independent_compute_queue_count;
  }
  no_pseudo_shared_queue = (reported_compute_queue_count >= 2u && independent_compute_queue_count < 2u) ? 0u : 1u;
  if (reported_compute_queue_count <= 1u && hardware_queue_cap > 1u && (rt->test_flags & PROM_TESTCFG_FORCE_DIRECT_PATH) == 0u) {
    queue_topology_classification = PROM_BATCH_QUEUE_TOPOLOGY_PSEUDO_SHARED;
  } else if (reported_compute_queue_count <= 1u) {
    queue_topology_classification = PROM_BATCH_QUEUE_TOPOLOGY_SINGLE_QUEUE;
  } else if (rt->transfer_queue_enabled != 0u && rt->transfer_queue_family_index != rt->queue_family_index) {
    queue_topology_classification = PROM_BATCH_QUEUE_TOPOLOGY_COMPUTE_PLUS_TRANSFER;
  } else if (no_pseudo_shared_queue == 0u) {
    queue_topology_classification = PROM_BATCH_QUEUE_TOPOLOGY_PSEUDO_SHARED;
  } else {
    queue_topology_classification = PROM_BATCH_QUEUE_TOPOLOGY_PARALLEL_ELIGIBLE;
  }
  if (memory_worker_cap < requested_workers && queue_topology_classification == PROM_BATCH_QUEUE_TOPOLOGY_PARALLEL_ELIGIBLE) {
    queue_topology_classification = PROM_BATCH_QUEUE_TOPOLOGY_MEMORY_CAPPED;
  }
  if (no_forced_serialized_flag == 0u) {
    queue_topology_classification = PROM_BATCH_QUEUE_TOPOLOGY_FORCED_SERIALIZED;
  }
  if (effective_workers == 0u) {
    rt->batch_diag.last_batch_entry_count = entry_count;
    rt->batch_diag.requested_workers = requested_workers;
    rt->batch_diag.hardware_queue_cap = hardware_queue_cap;
    rt->batch_diag.memory_worker_cap = memory_worker_cap;
    rt->batch_diag.effective_workers = 0u;
    rt->batch_diag.worker_cap_reason = cap_reason;
    rt->batch_diag.batch_state = PROM_BATCH_STATE_FAILED;
    rt->batch_diag.failed_entry_id = UINT32_MAX;
    rt->batch_diag.failed_worker_id = UINT32_MAX;
    rt->batch_diag.failure_stage = PROM_STAGE_INIT;
    rt->batch_diag.failure_detail = PROM_DETAIL_BATCH_ZERO_WORKERS;
    rt->batch_diag.failure_count = 1u;
    rt->batch_diag.first_failure_stable = 1u;
    rt->batch_diag.event_overflow_count = 0u;
    rt->batch_diag.event_drain_count = 0u;
    rt->batch_diag.output_committed = 0u;
    rt->batch_diag.plan_generation = 40u;
    rt->batch_diag.worker_judgment_count = 0u;
    rt->batch_diag.execution_mode = execution_mode;
    rt->batch_diag.worker_resource_mode = worker_resource_mode;
    rt->batch_diag.queue_topology_classification = queue_topology_classification;
    rt->batch_diag.queue_mapping_mode = queue_mapping_mode;
    rt->batch_diag.lane_worker_count = lane_worker_count;
    rt->batch_diag.real_worker_thread_count = real_worker_thread_count;
    rt->batch_diag.serialized_vulkan = serialized_vulkan;
    rt->batch_diag.serialized_bridge_enter_count = 0u;
    rt->batch_diag.serialized_execution_count = 0u;
    rt->batch_diag.serialized_wait_count = 0u;
    rt->batch_diag.max_concurrent_serialized_entries = 0u;
    rt->batch_diag.hardware_parallelism_claimed = hardware_parallelism_claimed;
    rt->batch_diag.resource_ownership_violation_count = 0u;
    rt->batch_diag.resource_creation_failure_count = 0u;
    rt->batch_diag.reported_compute_queue_count = reported_compute_queue_count;
    rt->batch_diag.independent_compute_queue_count = independent_compute_queue_count;
    rt->batch_diag.true_multi_queue_selected = 0u;
    rt->batch_diag.hardware_parallelism_eligible = 0u;
    rt->batch_diag.serialized_fallback_reason = PROM_BATCH_FALLBACK_REASON_EFFECTIVE_WORKERS_LT_2;
    rt->batch_diag.queue_drain_count = 0u;
    rt->batch_diag.drain_timeout_count = 0u;
    rt->batch_diag.queue_family_ownership_handoff_count = 0u;
    rt->batch_diag.transfer_compute_sync_wait_count = 0u;
    rt->batch_diag.unsafe_to_reuse = 0u;
    rt->batch_diag.slots_per_worker_target = slots_per_worker_target;
    rt->batch_diag.effective_slots_per_worker = 0u;
    rt->batch_diag.total_slot_count = 0u;
    rt->batch_diag.slot_cap_reason = slot_cap_reason;
    rt->batch_diag.slot_refill_count = 0u;
    rt->batch_diag.slot_full_scan_poll_count = 0u;
    rt->batch_diag.slot_attention_poll_count = 0u;
    rt->batch_diag.slot_polling_avoided_count = 0u;
    rt->batch_diag.slot_failure_count = 0u;
    rt->batch_diag.slot_drain_count = 0u;
    rt->batch_diag.slot_boundary_generation = 0u;
    rt->batch_diag.slot_dirty_mask = 0u;
    rt->batch_diag.slot_ready_mask = 0u;
    rt->batch_diag.slot_failed_mask = 0u;
    rt->batch_diag.slot_invalidated_mask = 0u;
    rt->batch_diag.slot_attention_mask = 0u;
    rt->batch_diag.worker_active_mask = 0u;
    for (i = 0u; i < 8u; ++i) {
      rt->batch_diag.worker_assigned_count[i] = 0u;
      rt->batch_diag.worker_completed_count[i] = 0u;
      rt->batch_diag.worker_event_count[i] = 0u;
      rt->batch_diag.worker_queue_index[i] = 0u;
      rt->batch_diag.worker_submit_count[i] = 0u;
      rt->batch_diag.worker_wait_count[i] = 0u;
      rt->batch_diag.worker_in_flight[i] = 0u;
      rt->batch_diag.worker_slot_id[i] = 0u;
      rt->batch_diag.worker_output_staging_id[i] = 0u;
      rt->batch_diag.worker_arena_bank_id[i] = 0u;
      rt->batch_diag.worker_command_pool_id[i] = 0u;
      rt->batch_diag.worker_command_buffer_id[i] = 0u;
      rt->batch_diag.worker_fence_id[i] = 0u;
      rt->batch_diag.worker_command_pool_valid[i] = 0u;
      rt->batch_diag.worker_command_buffer_valid[i] = 0u;
      rt->batch_diag.worker_fence_valid[i] = 0u;
      rt->batch_diag.worker_reset_count[i] = 0u;
      rt->batch_diag.worker_record_count[i] = 0u;
      rt->batch_diag.worker_failure_stage[i] = 0u;
      rt->batch_diag.worker_failure_detail[i] = 0;
      rt->batch_diag.per_worker_queue_family[i] = 0u;
      rt->batch_diag.per_worker_fence_state[i] = 0u;
    }
    for (i = 0u; i < 16u; ++i) {
      rt->batch_diag.slot_owner_worker_id[i] = UINT32_MAX;
      rt->batch_diag.slot_state[i] = PROM_BATCH_SLOT_STATE_EMPTY;
      rt->batch_diag.slot_generation[i] = 0u;
      rt->batch_diag.slot_entry_id[i] = UINT32_MAX;
      rt->batch_diag.slot_queue_id[i] = 0u;
      rt->batch_diag.slot_command_resource_id[i] = 0u;
      rt->batch_diag.slot_arena_id[i] = 0u;
      rt->batch_diag.slot_output_staging_id[i] = 0u;
      rt->batch_diag.slot_in_flight[i] = 0u;
      rt->batch_diag.slot_ready[i] = 0u;
      rt->batch_diag.slot_invalidated[i] = 0u;
      rt->batch_diag.slot_failure_stage[i] = 0u;
      rt->batch_diag.slot_failure_detail[i] = 0;
    }
    rt->batch_diag.p13_m10_lease_request_count = 0u;
    rt->batch_diag.p13_m10_lease_grant_count = 0u;
    rt->batch_diag.p13_m10_lease_deny_count = 0u;
    rt->batch_diag.p13_m10_lease_yield_count = 0u;
    rt->batch_diag.p13_m10_lease_last_state = PROM_LEASE_STATE_NONE;
    rt->batch_diag.p13_m10_lease_last_deny_reason = PROM_LEASE_REASON_NONE;
    rt->batch_diag.p13_m10_lookahead_requested = 0u;
    rt->batch_diag.p13_m10_lookahead_allowed = 0u;
    rt->batch_diag.p13_m10_lookahead_blocked_reason = PROM_LEASE_REASON_NONE;
    rt->batch_diag.p13_m10_selected_recipe_variant = 0u;
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_DETAIL_BATCH_ZERO_WORKERS);
    return PROM_ERROR;
  }

  memset(&shared_state, 0, sizeof(shared_state));
  shared_state.state = PROM_BATCH_STATE_PENDING;
  shared_state.failed_entry_id = UINT32_MAX;
  shared_state.failed_worker_id = UINT32_MAX;
  prom_batch_mutex_init(&shared_state.state_mutex);
  prom_batch_mutex_init(&shared_state.serialized_vulkan_mutex);
  use_real_threads = ((rt->test_flags & PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS) != 0u &&
                      (rt->test_flags & PROM_TESTCFG_P11_BATCH_FORCE_LANE_SIMULATED) == 0u)
                         ? 1u
                         : 0u;
  force_wrong_resource_owner = (rt->test_flags & PROM_TESTCFG_P11_BATCH_TEST_FORCE_WRONG_RESOURCE_OWNER) != 0u ? 1u : 0u;
  inject_invalidated_ready_slot = batch_test_invalidate_first_ready_slot(rt->test_flags);
  inject_presubmit_slot_failure = batch_test_fail_first_slot_before_submit(rt->test_flags);
  inject_thread_start_failure = batch_test_inject_thread_start_failure(rt->test_flags);
  inject_staged_output_alloc_failure = batch_test_inject_staged_output_alloc_failure(rt->test_flags);
  if (batch_test_inject_fence_wait_failure(rt->test_flags) != 0u || batch_test_inject_device_lost(rt->test_flags) != 0u) {
    unsafe_to_reuse = 1u;
  }
  if (effective_workers > 1u && use_real_threads != 0u) {
    execution_mode = PROM_BATCH_EXECUTION_REAL_THREADS_SERIALIZED_VULKAN;
    worker_resource_mode = PROM_BATCH_WORKER_RESOURCE_DEDICATED;
    resource_mode = PROM_BATCH_WORKER_RESOURCE_MODE_SIMULATED_PER_WORKER;
    real_worker_thread_count = effective_workers;
    serialized_vulkan = 1u;
    hardware_parallelism_claimed = 0u;
  } else if (effective_workers > 1u) {
    execution_mode = PROM_BATCH_EXECUTION_LANE_SIMULATED;
    resource_mode = PROM_BATCH_WORKER_RESOURCE_MODE_SIMULATED_PER_WORKER;
    lane_worker_count = effective_workers;
  } else {
    resource_mode = PROM_BATCH_WORKER_RESOURCE_MODE_SHARED;
  }
  if (queue_topology_classification == PROM_BATCH_QUEUE_TOPOLOGY_SINGLE_QUEUE) {
    queue_mapping_mode = PROM_BATCH_QUEUE_MAPPING_SINGLE_QUEUE_SERIALIZED;
  } else {
    queue_mapping_mode = PROM_BATCH_QUEUE_MAPPING_PER_WORKER_MAPPED_SERIALIZED;
  }

  if (event_capacity == 0u) {
    event_capacity = 64u;
  }

  plans = (prom_batch_plan*)calloc((size_t)entry_count, sizeof(prom_batch_plan));
  staged_outputs = (float**)calloc((size_t)entry_count, sizeof(float*));
  output_sizes = (uint32_t*)calloc((size_t)entry_count, sizeof(uint32_t));
  worker_events = (prom_batch_worker_event*)calloc((size_t)(effective_workers * event_capacity), sizeof(prom_batch_worker_event));
  worker_event_counts = (uint32_t*)calloc((size_t)effective_workers, sizeof(uint32_t));
  workers = (prom_batch_worker_state*)calloc((size_t)effective_workers, sizeof(prom_batch_worker_state));
  worker_resources = (prom_batch_worker_resources*)calloc((size_t)effective_workers, sizeof(prom_batch_worker_resources));
  worker_slots = (prom_batch_slot_runtime*)calloc((size_t)total_slot_count, sizeof(prom_batch_slot_runtime));
  worker_slot_refill_cursor = (uint32_t*)calloc((size_t)effective_workers, sizeof(uint32_t));
  if (use_real_threads != 0u) {
    worker_threads = (prom_batch_thread*)calloc((size_t)effective_workers, sizeof(prom_batch_thread));
    worker_thread_ctx = (prom_batch_thread_ctx*)calloc((size_t)effective_workers, sizeof(prom_batch_thread_ctx));
  }
  if (plans == NULL || staged_outputs == NULL || output_sizes == NULL || worker_events == NULL || worker_event_counts == NULL || workers == NULL ||
      worker_resources == NULL || worker_slots == NULL || worker_slot_refill_cursor == NULL ||
      (use_real_threads != 0u && (worker_threads == NULL || worker_thread_ctx == NULL))) {
    failure_stage = PROM_STAGE_INIT;
    failure_detail = PROM_ERROR;
    goto batch_cleanup;
  }

  for (w = 0u; w < effective_workers; ++w) {
    workers[w].worker_id = w;
    workers[w].failure_entry_id = UINT32_MAX;
    workers[w].resource_mode = worker_resource_mode;
    worker_resources[w].worker_id = w;
    worker_resources[w].queue_index = batch_worker_queue_index(w, independent_compute_queue_count);
    worker_resources[w].queue_family_index = rt->queue_family_index;
    worker_resources[w].command_pool_id = 1000u + w;
    worker_resources[w].command_buffer_id = 2000u + w;
    worker_resources[w].fence_id = 3000u + w;
    worker_resources[w].slot_id = w * effective_slots_per_worker;
    worker_resources[w].output_staging_id = w;
    worker_resources[w].arena_bank_id = w;
    for (i = 0u; i < effective_slots_per_worker; ++i) {
      uint32_t slot_id = w * effective_slots_per_worker + i;
      worker_slots[slot_id].slot_id = slot_id;
      worker_slots[slot_id].owner_worker_id = w;
      worker_slots[slot_id].state = PROM_BATCH_SLOT_STATE_EMPTY;
      worker_slots[slot_id].generation = 1u;
      worker_slots[slot_id].assigned_plan_id = UINT32_MAX;
      worker_slots[slot_id].assigned_entry_id = UINT32_MAX;
      worker_slots[slot_id].queue_id = worker_resources[w].queue_index;
      worker_slots[slot_id].command_resource_id = worker_resources[w].command_buffer_id + i;
      worker_slots[slot_id].arena_id = worker_resources[w].arena_bank_id;
      worker_slots[slot_id].output_staging_id = slot_id;
    }
  }
  if (execution_mode == PROM_BATCH_EXECUTION_REAL_THREADS_SERIALIZED_VULKAN && rt->device != VK_NULL_HANDLE &&
      rt->compute_queue != VK_NULL_HANDLE) {
    uint32_t resource_failed_worker_id = UINT32_MAX;
    if (!batch_create_physical_worker_resources(rt, worker_resources, effective_workers, &resource_failed_worker_id)) {
      resource_creation_failure_count += 1u;
      state = PROM_BATCH_STATE_FAILING;
      failure_stage = PROM_STAGE_INIT;
      failure_detail = PROM_DETAIL_BATCH_COMMAND_RESOURCE_CREATE_FAILED;
      failed_entry_id = UINT32_MAX;
      failed_worker_id = resource_failed_worker_id;
      goto batch_cleanup;
    }
    resource_mode = PROM_BATCH_WORKER_RESOURCE_PHYSICAL_PER_WORKER;
  }
  worker_queue_mapping_valid = 1u;
  for (w = 0u; w < effective_workers; ++w) {
    if (worker_resources[w].queue_index >= independent_compute_queue_count) {
      worker_queue_mapping_valid = 0u;
      break;
    }
  }
  per_worker_command_resources_valid =
      (resource_mode == PROM_BATCH_WORKER_RESOURCE_PHYSICAL_PER_WORKER && rt->device != VK_NULL_HANDLE) ? 1u : 0u;
  per_worker_fences_valid = per_worker_command_resources_valid;
  if (rt->transfer_queue_enabled != 0u) {
    transfer_compute_sync_wait_count = 1u;
  }
  hardware_parallelism_eligible =
      (independent_compute_queue_count >= 2u && effective_workers_for_gates >= 2u && per_worker_command_resources_valid != 0u &&
       per_worker_fences_valid != 0u && worker_queue_mapping_valid != 0u && memory_budget_supports_workers != 0u &&
       no_pseudo_shared_queue != 0u && no_forced_serialized_flag != 0u && queue_family_ownership_handoff_if_needed != 0u)
          ? 1u
          : 0u;
  true_multi_queue_selected = (hardware_parallelism_eligible != 0u && use_real_threads != 0u) ? 1u : 0u;
  if (true_multi_queue_selected != 0u) {
    execution_mode = PROM_BATCH_EXECUTION_REAL_THREADS_TRUE_MULTI_QUEUE;
    serialized_vulkan = 0u;
    hardware_parallelism_claimed = 1u;
    queue_mapping_mode = PROM_BATCH_QUEUE_MAPPING_PARALLEL_STATIC_PARTITION;
    real_worker_thread_count = effective_workers;
    serialized_fallback_reason = PROM_BATCH_FALLBACK_REASON_NONE;
  } else {
    if (independent_compute_queue_count < 2u) {
      serialized_fallback_reason = PROM_BATCH_FALLBACK_REASON_INDEPENDENT_QUEUE_LT_2;
    } else if (effective_workers_for_gates < 2u) {
      serialized_fallback_reason = PROM_BATCH_FALLBACK_REASON_EFFECTIVE_WORKERS_LT_2;
    } else if (per_worker_command_resources_valid == 0u) {
      serialized_fallback_reason = PROM_BATCH_FALLBACK_REASON_COMMAND_RESOURCES_INVALID;
    } else if (per_worker_fences_valid == 0u) {
      serialized_fallback_reason = PROM_BATCH_FALLBACK_REASON_FENCES_INVALID;
    } else if (worker_queue_mapping_valid == 0u) {
      serialized_fallback_reason = PROM_BATCH_FALLBACK_REASON_QUEUE_MAPPING_INVALID;
    } else if (memory_budget_supports_workers == 0u) {
      serialized_fallback_reason = PROM_BATCH_FALLBACK_REASON_MEMORY_CAP;
    } else if (no_pseudo_shared_queue == 0u) {
      serialized_fallback_reason = PROM_BATCH_FALLBACK_REASON_PSEUDO_SHARED;
    } else if (no_forced_serialized_flag == 0u) {
      serialized_fallback_reason = PROM_BATCH_FALLBACK_REASON_FORCED_SERIALIZED;
    } else if (queue_family_ownership_handoff_if_needed == 0u) {
      serialized_fallback_reason = PROM_BATCH_FALLBACK_REASON_QUEUE_FAMILY_OWNERSHIP_HANDOFF_REQUIRED;
    } else {
      serialized_fallback_reason = PROM_BATCH_FALLBACK_REASON_BASELINE_SERIALIZED;
    }
  }

  state = PROM_BATCH_STATE_RUNNING;
  for (i = 0u; i < entry_count; ++i) {
    prom_judgment_facts facts;
    prom_judgment_layout_precision_decision layout_precision_decision;
    prom_judgment_decision judgment_decision;
    prom_buffering_selector_facts buffering_facts;
    prom_buffering_selector_decision buffering_decision;
    prom_dom_transfer_queue_facts transfer_queue_facts;
    uint64_t work_units;
    uint32_t can_stage;
    uint32_t can_direct;
    uint32_t readback_required;
    uint32_t packed4_budget_permille;
    uint32_t packed4_waste_permille;
    uint32_t packed4_small_shape;
    uint32_t fp16_has_special_values;
    int fp16_utility_score;
    prom_policy_mode policy_mode;
    prom_variance_class variance_class;
    prom_predictability_class predictability_class;
    uint64_t plan_arena_required_bytes = 0u;
    uint32_t worker_id = batch_worker_partition(i, entry_count, effective_workers, flags);
    size_t output_elements;
    if (worker_id >= effective_workers) {
      failure_stage = PROM_STAGE_TRANSFER_IN;
      failure_detail = PROM_DETAIL_BATCH_PLAN_INVALID;
      failed_entry_id = i;
      failed_worker_id = worker_id;
      state = PROM_BATCH_STATE_FAILING;
      break;
    }
    if (entries[i].a == NULL || entries[i].b == NULL || entries[i].c == NULL || entries[i].m == 0u || entries[i].n == 0u || entries[i].k == 0u) {
      failure_stage = PROM_STAGE_TRANSFER_IN;
      failure_detail = PROM_DETAIL_BATCH_PLAN_INVALID;
      failed_entry_id = i;
      failed_worker_id = worker_id;
      state = PROM_BATCH_STATE_FAILING;
      break;
    }
    output_elements = (size_t)entries[i].m * (size_t)entries[i].n;
    if (output_elements > (SIZE_MAX / sizeof(float))) {
      failure_stage = PROM_STAGE_TRANSFER_IN;
      failure_detail = PROM_DETAIL_SIZE_OVERFLOW;
      failed_entry_id = i;
      failed_worker_id = worker_id;
      state = PROM_BATCH_STATE_FAILING;
      break;
    }

    work_units = ((uint64_t)entries[i].m * (uint64_t)entries[i].n * (uint64_t)entries[i].k) / 4096u;
    if (work_units == 0u) {
      work_units = 1u;
    }
    can_stage = rt->has_device_local_memory;
    can_direct = rt->has_host_visible_memory;
    if ((rt->test_flags & PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY) != 0u) {
      can_stage = 0u;
    }
    readback_required = ((rt->test_flags & PROM_TESTCFG_FORCE_UPLOAD_ONLY) == 0u) ? 1u : 0u;
    policy_mode = rt->sgemm_controller.policy_memory.current_mode;
    packed4_waste_permille = prom_packed4_padding_waste_permille(entries[i].m, entries[i].n, entries[i].k);
    packed4_budget_permille = prom_packed4_mode_budget_permille(policy_mode);
    packed4_small_shape = (entries[i].m < 4u || entries[i].n < 4u || entries[i].k < 4u) ? 1u : 0u;
    fp16_has_special_values = 0u;
    fp16_utility_score = -1000;
    prom_fp16_evaluate_tolerance(entries[i].a,
                                 entries[i].b,
                                 entries[i].m,
                                 entries[i].n,
                                 entries[i].k,
                                 &rt->sgemm_controller,
                                 &fp16_has_special_values,
                                 &fp16_utility_score);
    if ((rt->test_flags & PROM_TESTCFG_FORCE_FP16_UTILITY_WIN) != 0u) {
      fp16_utility_score = 1201;
    }
    memset(&facts, 0, sizeof(facts));
    facts.m = entries[i].m;
    facts.n = entries[i].n;
    facts.k = entries[i].k;
    facts.work_units = work_units;
    facts.can_stage = can_stage;
    facts.can_direct = can_direct;
    facts.allow_fallback = ((rt->test_flags & PROM_TESTCFG_DISABLE_STAGING_FALLBACK) == 0u) ? 1u : 0u;
    facts.readback_required = readback_required;
    facts.force_direct = ((rt->test_flags & PROM_TESTCFG_FORCE_DIRECT_PATH) != 0u) ? 1u : 0u;
    facts.force_staged = ((rt->test_flags & PROM_TESTCFG_FORCE_STAGED_PATH) != 0u) ? 1u : 0u;
    facts.force_tiled = ((rt->test_flags & PROM_TESTCFG_FORCE_TILED_PATH) != 0u) ? 1u : 0u;
    facts.tiled_shape = (work_units >= (uint64_t)PROM_JUDGMENT_TILED_WORK_THRESHOLD && entries[i].m >= PROM_VK_LOCAL_SIZE_X &&
                         entries[i].n >= PROM_VK_LOCAL_SIZE_Y && entries[i].k >= PROM_VK_TILE_K)
                            ? 1u
                            : 0u;
    facts.software_vulkan = rt->software_vulkan;
    facts.policy_mode = policy_mode;
    facts.packed4_available = 1u;
    facts.packed4_small_shape = packed4_small_shape;
    facts.packed4_padding_waste_permille = packed4_waste_permille;
    facts.packed4_mode_budget_permille = packed4_budget_permille;
    facts.packed4_row_major_valid = 1u;
    facts.packed4_tail_valid = 1u;
    facts.strict_fp32 = ((rt->test_flags & PROM_TESTCFG_FORCE_STRICT_FP32) != 0u) ? 1u : 0u;
    facts.tolerance_known = rt->sgemm_controller.fp16_tolerance_known;
    facts.tolerance_pass = rt->sgemm_controller.fp16_tolerance_pass;
    facts.has_special_values = fp16_has_special_values;
    facts.capability_fp16_storage = rt->capability_fp16_storage;
    facts.fallback_available = (facts.allow_fallback != 0u && can_direct != 0u) ? 1u : 0u;
    facts.fp16_utility_score = fp16_utility_score;
    memset(&transfer_queue_facts, 0, sizeof(transfer_queue_facts));
    transfer_queue_facts.dedicated_transfer_available = rt->dedicated_transfer_available;
    transfer_queue_facts.transfer_queue_family_index = rt->transfer_queue_family_index;
    transfer_queue_facts.compute_queue_family_index = rt->queue_family_index;
    transfer_queue_facts.queue_families_differ =
        (rt->dedicated_transfer_available != 0u && rt->transfer_queue_family_index != rt->queue_family_index) ? 1u : 0u;
    transfer_queue_facts.transfer_queue_supported = rt->transfer_queue_enabled;
    transfer_queue_facts.transfer_queue_disabled_by_config = ((rt->test_flags & PROM_TESTCFG_DISABLE_TRANSFER_QUEUE) != 0u) ? 1u : 0u;
    transfer_queue_facts.transfer_workload_large_enough = work_units >= (uint64_t)PROM_JUDGMENT_STAGING_WORK_THRESHOLD ? 1u : 0u;
    transfer_queue_facts.transfer_sync_ownership_supported = rt->transfer_queue_enabled;
    transfer_queue_facts.transfer_fallback_available = 1u;
    transfer_queue_facts.upload_only_policy_eligible = readback_required == 0u ? 1u : 0u;
    transfer_queue_facts.upload_readback_supported = 0u;
    facts.transfer_queue_dedicated_available = transfer_queue_facts.dedicated_transfer_available;
    facts.transfer_queue_families_differ = transfer_queue_facts.queue_families_differ;
    facts.transfer_queue_supported = transfer_queue_facts.transfer_queue_supported;
    facts.transfer_overlap_slot_valid = transfer_queue_facts.transfer_sync_ownership_supported;
    facts.transfer_workload_large_enough = transfer_queue_facts.transfer_workload_large_enough;
    facts.transfer_fallback_available = transfer_queue_facts.transfer_fallback_available;
    facts.transfer_queue_disabled_by_config = transfer_queue_facts.transfer_queue_disabled_by_config;
    prom_judgment_engine_select_layout_precision(&facts, &layout_precision_decision);
    prom_judgment_engine_select_sgemm_mode_with_layout_precision(&facts, &layout_precision_decision, &judgment_decision);
    if (judgment_decision.success == 0u) {
      failure_stage = PROM_STAGE_TRANSFER_IN;
      failure_detail = judgment_decision.error_detail;
      failed_entry_id = i;
      failed_worker_id = worker_id;
      state = PROM_BATCH_STATE_FAILING;
      break;
    }

    memset(&buffering_facts, 0, sizeof(buffering_facts));
    buffering_facts.required_fixed_slots_permille = 2000u;
    buffering_facts.required_pull_lag_peak_slots_permille = 1500u;
    buffering_facts.required_serial_slots_permille = 1000u;
    if (facts.transfer_queue_dedicated_available != 0u && rt->software_vulkan == 0u) {
      variance_class = PROM_VARIANCE_LOW;
    } else if (rt->software_vulkan != 0u) {
      variance_class = PROM_VARIANCE_HIGH;
    } else {
      variance_class = PROM_VARIANCE_MODERATE;
    }
    if (policy_mode == PROM_POLICY_MODE_RECOVERY) {
      predictability_class = PROM_PREDICTABILITY_UNSTABLE;
    } else if (policy_mode == PROM_POLICY_MODE_SAFE) {
      predictability_class = PROM_PREDICTABILITY_TRACKED;
    } else {
      predictability_class = PROM_PREDICTABILITY_STABLE;
    }
    buffering_facts.transfer_variance_class = variance_class;
    buffering_facts.compute_predictability_class = predictability_class;
    buffering_facts.pull_lag_wip_waste_exceeded = rt->sgemm_controller.pending_waste_units > PROM_SGEMM_WASTE_BUDGET_UNITS ? 1u : 0u;
    buffering_facts.starvation_risk_high = rt->software_vulkan != 0u && work_units > (uint64_t)PROM_JUDGMENT_STAGING_WORK_THRESHOLD ? 1u : 0u;
    if (can_stage != 0u && can_direct != 0u) {
      buffering_facts.memory_budget_slots_permille = 2200u;
    } else if (can_stage != 0u || can_direct != 0u) {
      buffering_facts.memory_budget_slots_permille = 1400u;
    } else {
      buffering_facts.memory_budget_slots_permille = 800u;
    }
    if (policy_mode == PROM_POLICY_MODE_SAFE && buffering_facts.memory_budget_slots_permille >= 200u) {
      buffering_facts.memory_budget_slots_permille -= 200u;
    } else if (policy_mode == PROM_POLICY_MODE_RECOVERY && buffering_facts.memory_budget_slots_permille >= 400u) {
      buffering_facts.memory_budget_slots_permille -= 400u;
    }
    buffering_facts.fixed_double_headroom_slots_permille =
        (int32_t)buffering_facts.memory_budget_slots_permille - (int32_t)buffering_facts.required_fixed_slots_permille;
    buffering_facts.pull_lag_headroom_slots_permille =
        (int32_t)buffering_facts.memory_budget_slots_permille - (int32_t)buffering_facts.required_pull_lag_peak_slots_permille;
    buffering_facts.serial_jit_headroom_slots_permille =
        (int32_t)buffering_facts.memory_budget_slots_permille - (int32_t)buffering_facts.required_serial_slots_permille;
    buffering_facts.fallback_available = facts.allow_fallback;
    prom_judgment_engine_select_buffering_mode(&buffering_facts, &buffering_decision);

    plans[i].entry_id = i;
    plans[i].worker_id = worker_id;
    plans[i].m = entries[i].m;
    plans[i].n = entries[i].n;
    plans[i].k = entries[i].k;
    plans[i].a = entries[i].a;
    plans[i].b = entries[i].b;
    plans[i].c = entries[i].c;
    plans[i].work_units = work_units;
    plans[i].selected_path = (uint32_t)judgment_decision.selected_path;
    plans[i].compute_mode = (uint32_t)judgment_decision.compute_mode;
    plans[i].buffering_mode = (uint32_t)buffering_decision.selected_mode;
    plans[i].transfer_policy = judgment_decision.use_dedicated_transfer_queue_upload;
    plans[i].layout_precision_mode = layout_precision_decision.packed4_selected != 0u ? 2u : (layout_precision_decision.fp16_selected != 0u ? 3u : 1u);
    if (!batch_compute_plan_arena_required_bytes(entries[i].m,
                                                 entries[i].n,
                                                 entries[i].k,
                                                 (prom_vk_compute_mode)plans[i].compute_mode,
                                                 &plan_arena_required_bytes)) {
      failure_stage = PROM_STAGE_TRANSFER_IN;
      failure_detail = PROM_DETAIL_SIZE_OVERFLOW;
      failed_entry_id = i;
      failed_worker_id = worker_id;
      state = PROM_BATCH_STATE_FAILING;
      break;
    }
    plans[i].arena_required_bytes = plan_arena_required_bytes;
    plans[i].expected_output_elements = (uint32_t)output_elements;
    plans[i].plan_generation = 40u;
    plans[i].slot_id = worker_id * effective_slots_per_worker + (worker_slot_refill_cursor[worker_id] % effective_slots_per_worker);
    plans[i].failure_policy = PROM_BATCH_STATE_FAILING;
    worker_slot_refill_cursor[worker_id] += 1u;
    if (plans[i].slot_id < total_slot_count) {
      prom_batch_slot_runtime* slot = &worker_slots[plans[i].slot_id];
      slot->state = PROM_BATCH_SLOT_STATE_PREPARING;
      slot->generation += 1u;
      slot->assigned_plan_id = i;
      slot->assigned_entry_id = plans[i].entry_id;
      slot->queue_id = worker_resources[worker_id].queue_index;
      slot->ready = 1u;
      slot->invalidated = 0u;
      slot->failure_stage = PROM_STAGE_NONE;
      slot->failure_detail = 0;
      slot_refill_count += 1u;
      if (plans[i].slot_id < 32u) {
        slot_dirty_mask |= (1u << plans[i].slot_id);
        slot_ready_mask |= (1u << plans[i].slot_id);
      }
      if (inject_invalidated_ready_slot != 0u && plans[i].entry_id == 0u) {
        slot->state = PROM_BATCH_SLOT_STATE_INVALIDATED;
        slot->ready = 0u;
        slot->invalidated = 1u;
        slot->failure_stage = PROM_STAGE_SUBMIT;
        slot->failure_detail = PROM_DETAIL_BATCH_PLAN_INVALID;
        if (plans[i].slot_id < 32u) {
          slot_ready_mask &= ~(1u << plans[i].slot_id);
          slot_invalidated_mask |= (1u << plans[i].slot_id);
        }
      }
    }
    if (inject_staged_output_alloc_failure != 0u && i == 1u) {
      staged_outputs[i] = NULL;
    } else {
      staged_outputs[i] = (float*)malloc(output_elements * sizeof(float));
    }
    output_sizes[i] = (uint32_t)output_elements;
    if (staged_outputs[i] == NULL) {
      failure_stage = PROM_STAGE_TRANSFER_IN;
      failure_detail = PROM_ERROR;
      failed_entry_id = i;
      failed_worker_id = worker_id;
      state = PROM_BATCH_STATE_FAILING;
      break;
    }
    workers[worker_id].assigned_count += 1u;
  }

  if (state == PROM_BATCH_STATE_RUNNING &&
      (execution_mode == PROM_BATCH_EXECUTION_REAL_THREADS_SERIALIZED_VULKAN ||
       execution_mode == PROM_BATCH_EXECUTION_REAL_THREADS_TRUE_MULTI_QUEUE)) {
    shared_state.state = PROM_BATCH_STATE_RUNNING;
    for (w = 0u; w < effective_workers; ++w) {
      worker_thread_ctx[w].plans = plans;
      worker_thread_ctx[w].rt = rt;
      worker_thread_ctx[w].entry_count = entry_count;
      worker_thread_ctx[w].event_capacity = event_capacity;
      worker_thread_ctx[w].injected_failure_entry_id = injected_failure_entry_id;
      worker_thread_ctx[w].force_dual_fail_first_two = force_dual_fail_first_two;
      worker_thread_ctx[w].delay_entry0 = delay_entry0;
      worker_thread_ctx[w].force_wrong_resource_owner = force_wrong_resource_owner;
      worker_thread_ctx[w].queue_count_for_mapping = effective_workers;
      worker_thread_ctx[w].true_multi_queue_enabled = true_multi_queue_selected;
      worker_thread_ctx[w].flags = flags;
      worker_thread_ctx[w].worker = &workers[w];
      worker_thread_ctx[w].worker_resources = worker_resources;
      worker_thread_ctx[w].worker_events = worker_events;
      worker_thread_ctx[w].worker_event_counts = worker_event_counts;
      worker_thread_ctx[w].staged_outputs = staged_outputs;
      worker_thread_ctx[w].shared = &shared_state;
      if ((inject_thread_start_failure != 0u && w == 1u) ||
          !prom_batch_thread_start(&worker_threads[w], batch_worker_thread_main, &worker_thread_ctx[w])) {
        batch_shared_fail_first(&shared_state, UINT32_MAX, w, PROM_STAGE_INIT, PROM_ERROR);
        failure_stage = PROM_STAGE_INIT;
        failure_detail = PROM_ERROR;
        break;
      }
      started_worker_count += 1u;
    }
    for (w = 0u; w < started_worker_count; ++w) {
      prom_batch_thread_join(worker_threads[w]);
    }
    prom_batch_mutex_lock(&shared_state.state_mutex);
    state = shared_state.state;
    failed_entry_id = shared_state.failed_entry_id;
    failed_worker_id = shared_state.failed_worker_id;
    failure_stage = shared_state.failure_stage;
    failure_detail = shared_state.failure_detail;
    event_overflow += shared_state.event_overflow_count;
    serialized_execution_count = shared_state.serialized_execution_count;
    serialized_bridge_enter_count = shared_state.serialized_bridge_enter_count;
    serialized_wait_count = shared_state.serialized_wait_count;
    max_concurrent_serialized_entries = shared_state.max_serialized_in_flight_count;
    resource_ownership_violation_count = shared_state.resource_ownership_violation_count;
    resource_creation_failure_count = shared_state.resource_creation_failure_count + resource_creation_failure_count;
    failure_count = shared_state.failure_count;
    prom_batch_mutex_unlock(&shared_state.state_mutex);
    for (w = 0u; w < effective_workers; ++w) {
      if (workers[w].assigned_count != 0u) {
        if (!batch_worker_emit_event(worker_events,
                                     worker_event_counts,
                                     w,
                                     event_capacity,
                                     PROM_BATCH_EVENT_WORKER_IDLE,
                                     UINT32_MAX,
                                     PROM_STAGE_NONE,
                                     0)) {
          event_overflow += 1u;
          if (state == PROM_BATCH_STATE_RUNNING) {
            state = PROM_BATCH_STATE_FAILING;
            failure_stage = PROM_STAGE_CLEANUP;
            failure_detail = PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW;
            failed_entry_id = UINT32_MAX;
            failed_worker_id = w;
          }
        }
      }
    }
  } else {
    while (state == PROM_BATCH_STATE_RUNNING) {
      uint32_t progress = 0u;
      for (w = 0u; w < effective_workers && state == PROM_BATCH_STATE_RUNNING; ++w) {
        prom_batch_worker_state* worker = &workers[w];
        prom_batch_worker_resources* resources = &worker_resources[w];
        uint32_t index = worker->next_scan_index;
        prom_batch_plan* plan = NULL;

        while (index < entry_count) {
          if (plans[index].worker_id == w) {
            plan = &plans[index];
            worker->next_scan_index = index + 1u;
            break;
          }
          index += 1u;
        }
        if (plan == NULL) {
          if (worker->assigned_count != 0u) {
            if (!batch_worker_emit_event(worker_events,
                                         worker_event_counts,
                                         w,
                                         event_capacity,
                                         PROM_BATCH_EVENT_WORKER_IDLE,
                                         UINT32_MAX,
                                         PROM_STAGE_NONE,
                                         0)) {
              state = PROM_BATCH_STATE_FAILING;
              failure_stage = PROM_STAGE_CLEANUP;
              failure_detail = PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW;
              failed_entry_id = UINT32_MAX;
              failed_worker_id = w;
              event_overflow += 1u;
              break;
            }
          }
          continue;
        }

        progress = 1u;
        if (unsafe_to_reuse != 0u) {
          lease_request_count += 1u;
          lease_deny_count += 1u;
          lease_last_state = PROM_LEASE_STATE_DENIED;
          lease_last_deny_reason = PROM_LEASE_REASON_DENIED_UNSAFE_RUNTIME;
          lease_last_lookahead_requested = effective_slots_per_worker;
          lease_last_lookahead_allowed = 0u;
          lease_last_lookahead_blocked_reason = PROM_LEASE_REASON_DENIED_UNSAFE_RUNTIME;
          lease_last_selected_recipe_variant = plan->compute_mode;
          state = PROM_BATCH_STATE_FAILING;
          failure_stage = PROM_STAGE_SUBMIT;
          failure_detail = PROM_DETAIL_BATCH_EXECUTION_FAILED;
          failed_entry_id = plan->entry_id;
          failed_worker_id = w;
          worker->failure_detail = failure_detail;
          worker->failure_stage = failure_stage;
          worker->failure_entry_id = failed_entry_id;
          break;
        }
        {
          prom_resource_lease_facts lease_facts;
          prom_resource_lease_decision lease_decision;
          memset(&lease_facts, 0, sizeof(lease_facts));
          lease_facts.worker_id = w;
          lease_facts.slot_id = plan->slot_id;
          lease_facts.entry_id = plan->entry_id;
          lease_facts.selected_recipe_variant = plan->compute_mode;
          lease_facts.requested_resource_class = PROM_LEASE_RESOURCE_CLASS_COMPUTE;
          lease_facts.current_outstanding_depth = resources->in_flight;
          lease_facts.max_outstanding_depth = effective_slots_per_worker;
          lease_facts.lookahead_requested = effective_slots_per_worker;
          lease_facts.lookahead_limit = effective_slots_per_worker;
          if (inject_thread_start_failure != 0u) {
            lease_facts.current_outstanding_depth = lease_facts.max_outstanding_depth;
          }
          lease_facts.ready_slot_mask = slot_ready_mask;
          lease_facts.failed_slot_mask = slot_failed_mask;
          lease_facts.invalidated_slot_mask = slot_invalidated_mask;
          if (inject_presubmit_slot_failure != 0u && plan->entry_id == 0u && plan->slot_id < 32u) {
            lease_facts.failed_slot_mask |= (1u << plan->slot_id);
          }
          lease_facts.unsafe_to_reuse = unsafe_to_reuse;
          lease_facts.transfer_overlap_available = 1u;
          lease_facts.yield_requested = 0u;
          if (batch_decide_resource_lease(&lease_facts, &lease_decision) == 0u) {
            state = PROM_BATCH_STATE_FAILING;
            failure_stage = PROM_STAGE_SUBMIT;
            failure_detail = PROM_ERROR;
            failed_entry_id = plan->entry_id;
            failed_worker_id = w;
            break;
          }
          lease_request_count += 1u;
          lease_last_state = lease_decision.lease_state;
          lease_last_deny_reason = lease_decision.deny_reason;
          lease_last_lookahead_requested = lease_facts.lookahead_requested;
          lease_last_lookahead_allowed = lease_decision.lookahead_allowed;
          lease_last_lookahead_blocked_reason =
              lease_decision.lookahead_allowed != 0u ? PROM_LEASE_REASON_NONE : lease_decision.deny_reason;
          lease_last_selected_recipe_variant = lease_decision.selected_recipe_variant;
          if (lease_decision.grant == 0u) {
            lease_deny_count += 1u;
            state = PROM_BATCH_STATE_FAILING;
            failure_stage = PROM_STAGE_SUBMIT;
            failure_detail = (lease_decision.deny_reason == PROM_LEASE_REASON_DENIED_SLOT_INVALIDATED)
                                 ? PROM_DETAIL_BATCH_PLAN_INVALID
                                 : PROM_DETAIL_BATCH_EXECUTION_FAILED;
            failed_entry_id = plan->entry_id;
            failed_worker_id = w;
            worker->failure_detail = failure_detail;
            worker->failure_stage = failure_stage;
            worker->failure_entry_id = failed_entry_id;
            break;
          }
          lease_grant_count += 1u;
          if (plan->slot_id < 32u) {
            lease_held_mask |= (1u << plan->slot_id);
          }
          lease_outstanding_depth += 1u;
          if (lease_outstanding_depth > effective_slots_per_worker) {
            state = PROM_BATCH_STATE_FAILING;
            failure_stage = PROM_STAGE_SUBMIT;
            failure_detail = PROM_DETAIL_BATCH_EXECUTION_FAILED;
            failed_entry_id = plan->entry_id;
            failed_worker_id = w;
            break;
          }
        }
        if (slot_attention_mask != 0u) {
          slot_attention_poll_count += 1u;
          slot_polling_avoided_count += 1u;
        } else {
          slot_full_scan_poll_count += 1u;
        }
        worker->active = 1u;
        if (plan->slot_id < total_slot_count) {
          prom_batch_slot_runtime* slot = &worker_slots[plan->slot_id];
          if (slot->invalidated != 0u) {
            lease_request_count += 1u;
            lease_deny_count += 1u;
            lease_last_state = PROM_LEASE_STATE_DENIED;
            lease_last_deny_reason = PROM_LEASE_REASON_DENIED_SLOT_INVALIDATED;
            lease_last_lookahead_requested = effective_slots_per_worker;
            lease_last_lookahead_allowed = 0u;
            lease_last_lookahead_blocked_reason = PROM_LEASE_REASON_DENIED_SLOT_INVALIDATED;
            lease_last_selected_recipe_variant = plan->compute_mode;
            state = PROM_BATCH_STATE_FAILING;
            failure_stage = PROM_STAGE_SUBMIT;
            failure_detail = PROM_DETAIL_BATCH_PLAN_INVALID;
            failed_entry_id = plan->entry_id;
            failed_worker_id = w;
            worker->failure_detail = failure_detail;
            worker->failure_stage = failure_stage;
            worker->failure_entry_id = failed_entry_id;
            slot->state = PROM_BATCH_SLOT_STATE_INVALIDATED;
            slot->in_flight = 0u;
            slot->ready = 0u;
            slot->failure_stage = failure_stage;
            slot->failure_detail = failure_detail;
            if (plan->slot_id < 32u) {
              slot_dirty_mask |= (1u << plan->slot_id);
              slot_invalidated_mask |= (1u << plan->slot_id);
            }
            break;
          }
          if (inject_presubmit_slot_failure != 0u && plan->entry_id == 0u) {
            lease_request_count += 1u;
            lease_deny_count += 1u;
            lease_last_state = PROM_LEASE_STATE_DENIED;
            lease_last_deny_reason = PROM_LEASE_REASON_DENIED_SLOT_FAILED;
            lease_last_lookahead_requested = effective_slots_per_worker;
            lease_last_lookahead_allowed = 0u;
            lease_last_lookahead_blocked_reason = PROM_LEASE_REASON_DENIED_SLOT_FAILED;
            lease_last_selected_recipe_variant = plan->compute_mode;
            state = PROM_BATCH_STATE_FAILING;
            failure_stage = PROM_STAGE_SUBMIT;
            failure_detail = PROM_DETAIL_BATCH_EXECUTION_FAILED;
            failed_entry_id = plan->entry_id;
            failed_worker_id = w;
            worker->failure_detail = failure_detail;
            worker->failure_stage = failure_stage;
            worker->failure_entry_id = failed_entry_id;
            slot->state = PROM_BATCH_SLOT_STATE_FAILED;
            slot->in_flight = 0u;
            slot->ready = 0u;
            slot->failure_stage = failure_stage;
            slot->failure_detail = failure_detail;
            slot_failure_count += 1u;
            if (plan->slot_id < 32u) {
              slot_dirty_mask |= (1u << plan->slot_id);
              slot_failed_mask |= (1u << plan->slot_id);
              lease_held_mask &= ~(1u << plan->slot_id);
            }
            if (lease_outstanding_depth > 0u) {
              lease_outstanding_depth -= 1u;
            }
            break;
          }
        }
        if (plan->slot_id < total_slot_count) {
          prom_batch_slot_runtime* slot = &worker_slots[plan->slot_id];
          slot->state = PROM_BATCH_SLOT_STATE_IN_FLIGHT;
          slot->ready = 0u;
          slot->in_flight = 1u;
          if (plan->slot_id < 32u) {
            slot_dirty_mask |= (1u << plan->slot_id);
            slot_ready_mask &= ~(1u << plan->slot_id);
          }
        }
        if (!batch_worker_emit_event(worker_events,
                                     worker_event_counts,
                                     w,
                                     event_capacity,
                                     PROM_BATCH_EVENT_PLAN_STARTED,
                                     plan->entry_id,
                                     PROM_STAGE_TRANSFER_IN,
                                     0)) {
          state = PROM_BATCH_STATE_FAILING;
          failure_stage = PROM_STAGE_SUBMIT;
          failure_detail = PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW;
          failed_entry_id = plan->entry_id;
          failed_worker_id = w;
          event_overflow += 1u;
          worker->failure_detail = failure_detail;
          worker->failure_stage = failure_stage;
          worker->failure_entry_id = failed_entry_id;
          break;
        }
        if (!batch_worker_emit_event(worker_events,
                                     worker_event_counts,
                                     w,
                                     event_capacity,
                                     PROM_BATCH_EVENT_PLAN_SUBMITTED,
                                     plan->entry_id,
                                     PROM_STAGE_SUBMIT,
                                     0)) {
          state = PROM_BATCH_STATE_FAILING;
          failure_stage = PROM_STAGE_SUBMIT;
          failure_detail = PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW;
          failed_entry_id = plan->entry_id;
          failed_worker_id = w;
          event_overflow += 1u;
          worker->failure_detail = failure_detail;
          worker->failure_stage = failure_stage;
          worker->failure_entry_id = failed_entry_id;
          break;
        }
        if (((flags & PROM_BATCH_FLAG_FAIL_AFTER_FIRST_SUBMIT) != 0u && plan->entry_id == 0u) || plan->entry_id == injected_failure_entry_id ||
            (force_dual_fail_first_two != 0u && (plan->entry_id == 0u || plan->entry_id == 1u))) {
          state = PROM_BATCH_STATE_FAILING;
          failure_stage = PROM_STAGE_SUBMIT;
          failure_detail = PROM_DETAIL_BATCH_EXECUTION_FAILED;
          failed_entry_id = plan->entry_id;
          failed_worker_id = w;
          failure_count += 1u;
          worker->failure_detail = failure_detail;
          worker->failure_stage = failure_stage;
          worker->failure_entry_id = failed_entry_id;
          if (plan->slot_id < total_slot_count) {
            prom_batch_slot_runtime* slot = &worker_slots[plan->slot_id];
            slot->state = PROM_BATCH_SLOT_STATE_FAILED;
            slot->in_flight = 0u;
            slot->ready = 0u;
            slot->failure_stage = failure_stage;
            slot->failure_detail = failure_detail;
            slot_failure_count += 1u;
            if (plan->slot_id < 32u) {
              slot_dirty_mask |= (1u << plan->slot_id);
              slot_failed_mask |= (1u << plan->slot_id);
            }
          }
          break;
        }
        if (force_wrong_resource_owner != 0u && batch_verify_worker_resource_owner(&shared_state, resources, (w + 1u) % effective_workers) == 0u) {
          state = PROM_BATCH_STATE_FAILING;
          failure_stage = PROM_STAGE_SUBMIT;
          failure_detail = PROM_DETAIL_BATCH_RESOURCE_OWNERSHIP_VIOLATION;
          failed_entry_id = plan->entry_id;
          failed_worker_id = w;
          worker->failure_detail = failure_detail;
          worker->failure_stage = failure_stage;
          worker->failure_entry_id = failed_entry_id;
          break;
        }
        if (batch_verify_worker_resource_owner(&shared_state, resources, w) == 0u) {
          state = PROM_BATCH_STATE_FAILING;
          failure_stage = PROM_STAGE_SUBMIT;
          failure_detail = PROM_DETAIL_BATCH_RESOURCE_OWNERSHIP_VIOLATION;
          failed_entry_id = plan->entry_id;
          failed_worker_id = w;
          worker->failure_detail = failure_detail;
          worker->failure_stage = failure_stage;
          worker->failure_entry_id = failed_entry_id;
          break;
        }
        if (delay_entry0 != 0u && plan->entry_id == 0u) {
          volatile uint32_t spin = 0u;
          for (spin = 0u; spin < 4000000u; ++spin) {
          }
        }

        resources->submit_count += 1u;
        resources->in_flight = 1u;
        resources->slot_id = plan->slot_id;
        batch_reference_sgemm(plan->a, plan->b, staged_outputs[plan->entry_id], plan->m, plan->n, plan->k);
        resources->in_flight = 0u;
        if (plan->slot_id < total_slot_count) {
          prom_batch_slot_runtime* slot = &worker_slots[plan->slot_id];
          slot->state = PROM_BATCH_SLOT_STATE_COMPLETE;
          slot->in_flight = 0u;
          slot->ready = 0u;
          if (plan->slot_id < 32u) {
            slot_dirty_mask |= (1u << plan->slot_id);
            slot_ready_mask &= ~(1u << plan->slot_id);
          }
        }
        if (!batch_worker_emit_event(worker_events,
                                     worker_event_counts,
                                     w,
                                     event_capacity,
                                     PROM_BATCH_EVENT_PLAN_COMPLETED,
                                     plan->entry_id,
                                     PROM_STAGE_TRANSFER_OUT,
                                     0)) {
          state = PROM_BATCH_STATE_FAILING;
          failure_stage = PROM_STAGE_TRANSFER_OUT;
          failure_detail = PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW;
          failed_entry_id = plan->entry_id;
          failed_worker_id = w;
          event_overflow += 1u;
          worker->failure_detail = failure_detail;
          worker->failure_stage = failure_stage;
          worker->failure_entry_id = failed_entry_id;
          break;
        }
        worker->completed_count += 1u;
        {
          prom_resource_lease_facts lease_facts;
          prom_resource_lease_decision lease_decision;
          memset(&lease_facts, 0, sizeof(lease_facts));
          lease_facts.worker_id = w;
          lease_facts.slot_id = plan->slot_id;
          lease_facts.entry_id = plan->entry_id;
          lease_facts.selected_recipe_variant = plan->compute_mode;
          lease_facts.requested_resource_class = PROM_LEASE_RESOURCE_CLASS_COMPUTE;
          lease_facts.current_outstanding_depth = resources->in_flight;
          lease_facts.max_outstanding_depth = effective_slots_per_worker;
          lease_facts.lookahead_requested = effective_slots_per_worker;
          lease_facts.lookahead_limit = effective_slots_per_worker;
          lease_facts.ready_slot_mask = slot_ready_mask;
          lease_facts.failed_slot_mask = slot_failed_mask;
          lease_facts.invalidated_slot_mask = slot_invalidated_mask;
          lease_facts.unsafe_to_reuse = unsafe_to_reuse;
          lease_facts.transfer_overlap_available = 1u;
          lease_facts.yield_requested = 1u;
          if (plan->slot_id < 32u && (lease_held_mask & (1u << plan->slot_id)) != 0u &&
              batch_decide_resource_lease(&lease_facts, &lease_decision) != 0u &&
              lease_decision.lease_state == PROM_LEASE_STATE_YIELDED) {
            lease_yield_count += 1u;
            lease_last_state = lease_decision.lease_state;
            lease_last_deny_reason = lease_decision.deny_reason;
            lease_held_mask &= ~(1u << plan->slot_id);
            if (lease_outstanding_depth > 0u) {
              lease_outstanding_depth -= 1u;
            }
          }
        }
        worker->active = 0u;
      }
      if (state != PROM_BATCH_STATE_RUNNING) {
        break;
      }
      if (progress == 0u) {
        break;
      }
    }
  }

  resource_ownership_violation_count = shared_state.resource_ownership_violation_count;
  resource_creation_failure_count += shared_state.resource_creation_failure_count;
  if (state == PROM_BATCH_STATE_FAILING && failed_entry_id != UINT32_MAX) {
    for (i = 0u; i < entry_count; ++i) {
      if (plans[i].entry_id == failed_entry_id && plans[i].slot_id < total_slot_count) {
        prom_batch_slot_runtime* slot = &worker_slots[plans[i].slot_id];
        if (slot->state != PROM_BATCH_SLOT_STATE_INVALIDATED) {
          slot->state = PROM_BATCH_SLOT_STATE_FAILED;
          slot->failure_stage = failure_stage;
          slot->failure_detail = failure_detail;
          slot->ready = 0u;
          slot->in_flight = 0u;
          slot_failure_count += 1u;
          if (plans[i].slot_id < 32u) {
            slot_dirty_mask |= (1u << plans[i].slot_id);
            slot_failed_mask |= (1u << plans[i].slot_id);
            slot_ready_mask &= ~(1u << plans[i].slot_id);
          }
        }
        break;
      }
    }
  }
  slot_attention_mask = (slot_ready_mask | slot_failed_mask | slot_invalidated_mask);
  slot_boundary_generation += 1u;
  if (state == PROM_BATCH_STATE_FAILING) {
    for (w = 0u; w < effective_workers; ++w) {
      prom_batch_worker_state* worker = &workers[w];
      worker->active = 0u;
      if (worker->failure_entry_id == UINT32_MAX) {
        worker->failure_observed = 1u;
        batch_worker_emit_event(worker_events,
                               worker_event_counts,
                               w,
                               event_capacity,
                               PROM_BATCH_EVENT_BATCH_FAILURE_OBSERVED,
                               failed_entry_id,
                               failure_stage,
                               failure_detail);
      }
      if (!batch_worker_emit_event(worker_events,
                                   worker_event_counts,
                                   w,
                                   event_capacity,
                                   PROM_BATCH_EVENT_WORKER_DRAINED,
                                   failed_entry_id,
                                   PROM_STAGE_CLEANUP,
                                   failure_detail)) {
        event_overflow += 1u;
      }
    }
    state = PROM_BATCH_STATE_DRAINING;
    slot_drain_count = total_slot_count;
    if (batch_test_inject_drain_timeout(rt->test_flags) != 0u) {
      drain_timeout_count += 1u;
      failure_detail = PROM_DETAIL_BATCH_DRAIN_TIMEOUT;
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_CLEANUP, failure_detail);
      final_status = PROM_ERROR;
      state = PROM_BATCH_STATE_FAILED;
      queue_drain_count = (true_multi_queue_selected != 0u) ? independent_compute_queue_count : 1u;
      goto batch_cleanup;
    }
    batch_drain_worker_events(worker_events, worker_event_counts, effective_workers, event_capacity, &drain_summary);
    event_drain_count = drain_summary.drained_events;
    state = PROM_BATCH_STATE_FAILED;
    prom_vk_set_status(out_stage, out_detail_code, failure_stage == PROM_STAGE_NONE ? PROM_STAGE_SUBMIT : failure_stage, failure_detail);
    final_status = PROM_ERROR;
    queue_drain_count = (true_multi_queue_selected != 0u) ? independent_compute_queue_count : 1u;
  } else {
    for (i = 0u; i < entry_count; ++i) {
      memcpy(entries[i].c, staged_outputs[i], (size_t)output_sizes[i] * sizeof(float));
    }
    for (w = 0u; w < effective_workers; ++w) {
      if (!batch_worker_emit_event(worker_events,
                                   worker_event_counts,
                                   w,
                                   event_capacity,
                                   PROM_BATCH_EVENT_WORKER_DRAINED,
                                   UINT32_MAX,
                                   PROM_STAGE_CLEANUP,
                                   0)) {
        state = PROM_BATCH_STATE_FAILING;
        failure_stage = PROM_STAGE_CLEANUP;
        failure_detail = PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW;
        failed_entry_id = UINT32_MAX;
        failed_worker_id = w;
        event_overflow += 1u;
        break;
      }
    }
    if (state == PROM_BATCH_STATE_FAILING) {
      state = PROM_BATCH_STATE_DRAINING;
      batch_drain_worker_events(worker_events, worker_event_counts, effective_workers, event_capacity, &drain_summary);
      event_drain_count = drain_summary.drained_events;
      state = PROM_BATCH_STATE_FAILED;
      prom_vk_set_status(out_stage, out_detail_code, failure_stage, failure_detail);
      final_status = PROM_ERROR;
      queue_drain_count = (true_multi_queue_selected != 0u) ? independent_compute_queue_count : 1u;
    } else {
      output_committed = 1u;
      state = PROM_BATCH_STATE_SUCCEEDED;
      prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, 0);
      batch_drain_worker_events(worker_events, worker_event_counts, effective_workers, event_capacity, &drain_summary);
      event_drain_count = drain_summary.drained_events;
      final_status = PROM_OK;
      queue_drain_count = (true_multi_queue_selected != 0u) ? independent_compute_queue_count : 1u;
    }
  }

batch_cleanup:
  if (final_status != PROM_OK && state != PROM_BATCH_STATE_FAILED && state != PROM_BATCH_STATE_DRAINING) {
    if (state != PROM_BATCH_STATE_FAILING) {
      state = PROM_BATCH_STATE_FAILING;
    }
    if (failure_stage == PROM_STAGE_NONE) {
      failure_stage = PROM_STAGE_INIT;
    }
    if (failure_detail == 0) {
      failure_detail = PROM_ERROR;
    }
    state = PROM_BATCH_STATE_DRAINING;
    batch_drain_worker_events(worker_events, worker_event_counts, effective_workers, event_capacity, &drain_summary);
    event_drain_count = drain_summary.drained_events;
    state = PROM_BATCH_STATE_FAILED;
    prom_vk_set_status(out_stage, out_detail_code, failure_stage, failure_detail);
  }
  if (rt != NULL) {
    if (state == PROM_BATCH_STATE_FAILED &&
        (failure_detail == PROM_DETAIL_BATCH_FENCE_WAIT_FAILED || failure_detail == PROM_DETAIL_BATCH_QUEUE_SUBMIT_FAILED ||
         failure_detail == PROM_DETAIL_BATCH_DRAIN_TIMEOUT || failure_detail == PROM_DETAIL_BATCH_DEVICE_LOST)) {
      unsafe_to_reuse = 1u;
    }
    queue_family_ownership_handoff_count = (uint32_t)rt->slot_diag.queue_family_handoff_count;
    transfer_compute_sync_wait_count = (uint32_t)rt->slot_diag.transfer_compute_wait_count;
    rt->batch_diag.last_batch_entry_count = entry_count;
    rt->batch_diag.requested_workers = requested_workers;
    rt->batch_diag.effective_workers = effective_workers;
    rt->batch_diag.hardware_queue_cap = hardware_queue_cap;
    rt->batch_diag.memory_worker_cap = memory_worker_cap;
    rt->batch_diag.worker_cap_reason = cap_reason;
    rt->batch_diag.partition_policy =
        ((flags & PROM_BATCH_FLAG_PARTITION_CONTIGUOUS) != 0u) ? PROM_BATCH_PARTITION_CONTIGUOUS : PROM_BATCH_PARTITION_ROUND_ROBIN;
    rt->batch_diag.batch_state = state;
    rt->batch_diag.failed_entry_id = failed_entry_id;
    rt->batch_diag.failed_worker_id = failed_worker_id;
    rt->batch_diag.failure_stage = failure_stage;
    rt->batch_diag.failure_detail = failure_detail;
    rt->batch_diag.failure_count = failure_count == 0u && state == PROM_BATCH_STATE_FAILED ? 1u : failure_count;
    rt->batch_diag.first_failure_stable = (state == PROM_BATCH_STATE_FAILED && failure_stage != PROM_STAGE_NONE) ? 1u : 0u;
    rt->batch_diag.event_overflow_count = event_overflow;
    rt->batch_diag.event_drain_count = event_drain_count;
    rt->batch_diag.output_committed = output_committed;
    rt->batch_diag.plan_generation = 40u;
    rt->batch_diag.worker_judgment_count = 0u;
    rt->batch_diag.execution_mode = execution_mode;
    rt->batch_diag.worker_resource_mode = resource_mode;
    rt->batch_diag.queue_topology_classification = queue_topology_classification;
    rt->batch_diag.queue_mapping_mode = queue_mapping_mode;
    rt->batch_diag.lane_worker_count = lane_worker_count;
    rt->batch_diag.real_worker_thread_count = real_worker_thread_count;
    rt->batch_diag.serialized_vulkan = serialized_vulkan;
    rt->batch_diag.serialized_bridge_enter_count = serialized_bridge_enter_count;
    rt->batch_diag.serialized_execution_count = serialized_execution_count;
    rt->batch_diag.serialized_wait_count = serialized_wait_count;
    rt->batch_diag.max_concurrent_serialized_entries = max_concurrent_serialized_entries;
    rt->batch_diag.hardware_parallelism_claimed = hardware_parallelism_claimed;
    rt->batch_diag.resource_ownership_violation_count = resource_ownership_violation_count;
    rt->batch_diag.resource_creation_failure_count = resource_creation_failure_count;
    rt->batch_diag.reported_compute_queue_count = reported_compute_queue_count;
    rt->batch_diag.independent_compute_queue_count = independent_compute_queue_count;
    rt->batch_diag.true_multi_queue_selected = true_multi_queue_selected;
    rt->batch_diag.hardware_parallelism_eligible = hardware_parallelism_eligible;
    rt->batch_diag.serialized_fallback_reason = serialized_fallback_reason;
    rt->batch_diag.queue_drain_count = queue_drain_count;
    rt->batch_diag.drain_timeout_count = drain_timeout_count;
    rt->batch_diag.queue_family_ownership_handoff_count = queue_family_ownership_handoff_count;
    rt->batch_diag.transfer_compute_sync_wait_count = transfer_compute_sync_wait_count;
    rt->batch_diag.unsafe_to_reuse = unsafe_to_reuse;
    rt->batch_diag.slots_per_worker_target = slots_per_worker_target;
    rt->batch_diag.effective_slots_per_worker = effective_slots_per_worker;
    rt->batch_diag.total_slot_count = total_slot_count;
    rt->batch_diag.slot_cap_reason = slot_cap_reason;
    rt->batch_diag.slot_refill_count = slot_refill_count;
    rt->batch_diag.slot_full_scan_poll_count = slot_full_scan_poll_count;
    rt->batch_diag.slot_attention_poll_count = slot_attention_poll_count;
    rt->batch_diag.slot_polling_avoided_count = slot_polling_avoided_count;
    rt->batch_diag.slot_failure_count = slot_failure_count;
    rt->batch_diag.slot_drain_count = slot_drain_count;
    rt->batch_diag.slot_boundary_generation = slot_boundary_generation;
    rt->batch_diag.slot_dirty_mask = slot_dirty_mask;
    rt->batch_diag.slot_ready_mask = slot_ready_mask;
    rt->batch_diag.slot_failed_mask = slot_failed_mask;
    rt->batch_diag.slot_invalidated_mask = slot_invalidated_mask;
    rt->batch_diag.slot_attention_mask = slot_attention_mask;
    rt->batch_diag.worker_active_mask = 0u;
    for (i = 0u; i < 8u; ++i) {
      rt->batch_diag.worker_assigned_count[i] = 0u;
      rt->batch_diag.worker_completed_count[i] = 0u;
      rt->batch_diag.worker_event_count[i] = 0u;
      rt->batch_diag.worker_queue_index[i] = 0u;
      rt->batch_diag.worker_submit_count[i] = 0u;
      rt->batch_diag.worker_wait_count[i] = 0u;
      rt->batch_diag.worker_in_flight[i] = 0u;
      rt->batch_diag.worker_slot_id[i] = 0u;
      rt->batch_diag.worker_output_staging_id[i] = 0u;
      rt->batch_diag.worker_arena_bank_id[i] = 0u;
      rt->batch_diag.worker_command_pool_id[i] = 0u;
      rt->batch_diag.worker_command_buffer_id[i] = 0u;
      rt->batch_diag.worker_fence_id[i] = 0u;
      rt->batch_diag.worker_command_pool_valid[i] = 0u;
      rt->batch_diag.worker_command_buffer_valid[i] = 0u;
      rt->batch_diag.worker_fence_valid[i] = 0u;
      rt->batch_diag.worker_reset_count[i] = 0u;
      rt->batch_diag.worker_record_count[i] = 0u;
      rt->batch_diag.worker_failure_stage[i] = 0u;
      rt->batch_diag.worker_failure_detail[i] = 0;
      rt->batch_diag.per_worker_queue_family[i] = 0u;
      rt->batch_diag.per_worker_fence_state[i] = 0u;
    }
    for (i = 0u; i < 16u; ++i) {
      rt->batch_diag.slot_owner_worker_id[i] = UINT32_MAX;
      rt->batch_diag.slot_state[i] = PROM_BATCH_SLOT_STATE_EMPTY;
      rt->batch_diag.slot_generation[i] = 0u;
      rt->batch_diag.slot_entry_id[i] = UINT32_MAX;
      rt->batch_diag.slot_queue_id[i] = 0u;
      rt->batch_diag.slot_command_resource_id[i] = 0u;
      rt->batch_diag.slot_arena_id[i] = 0u;
      rt->batch_diag.slot_output_staging_id[i] = 0u;
      rt->batch_diag.slot_in_flight[i] = 0u;
      rt->batch_diag.slot_ready[i] = 0u;
      rt->batch_diag.slot_invalidated[i] = 0u;
      rt->batch_diag.slot_failure_stage[i] = 0u;
      rt->batch_diag.slot_failure_detail[i] = 0;
    }
    rt->batch_diag.p13_m10_lease_request_count = lease_request_count;
    rt->batch_diag.p13_m10_lease_grant_count = lease_grant_count;
    rt->batch_diag.p13_m10_lease_deny_count = lease_deny_count;
    rt->batch_diag.p13_m10_lease_yield_count = lease_yield_count;
    rt->batch_diag.p13_m10_lease_last_state = lease_last_state;
    rt->batch_diag.p13_m10_lease_last_deny_reason = lease_last_deny_reason;
    rt->batch_diag.p13_m10_lookahead_requested = lease_last_lookahead_requested;
    rt->batch_diag.p13_m10_lookahead_allowed = lease_last_lookahead_allowed;
    rt->batch_diag.p13_m10_lookahead_blocked_reason = lease_last_lookahead_blocked_reason;
    rt->batch_diag.p13_m10_selected_recipe_variant = lease_last_selected_recipe_variant;
    if (lease_outstanding_depth > effective_slots_per_worker) {
      rt->batch_diag.p13_m10_lookahead_blocked_reason = PROM_LEASE_REASON_DENIED_OUTSTANDING_LIMIT;
    }
    if (workers != NULL && worker_event_counts != NULL && worker_resources != NULL) {
      for (w = 0u; w < effective_workers && w < 8u; ++w) {
        rt->batch_diag.worker_assigned_count[w] = workers[w].assigned_count;
        rt->batch_diag.worker_completed_count[w] = workers[w].completed_count;
        rt->batch_diag.worker_event_count[w] = worker_event_counts[w];
        rt->batch_diag.worker_queue_index[w] = worker_resources[w].queue_index;
        rt->batch_diag.worker_submit_count[w] = worker_resources[w].submit_count;
        rt->batch_diag.worker_wait_count[w] = worker_resources[w].wait_count;
        rt->batch_diag.worker_in_flight[w] = worker_resources[w].in_flight;
        rt->batch_diag.worker_slot_id[w] = worker_resources[w].slot_id;
        rt->batch_diag.worker_output_staging_id[w] = worker_resources[w].output_staging_id;
        rt->batch_diag.worker_arena_bank_id[w] = worker_resources[w].arena_bank_id;
        rt->batch_diag.worker_command_pool_id[w] = worker_resources[w].command_pool_id;
        rt->batch_diag.worker_command_buffer_id[w] = worker_resources[w].command_buffer_id;
        rt->batch_diag.worker_fence_id[w] = worker_resources[w].fence_id;
        rt->batch_diag.worker_command_pool_valid[w] = worker_resources[w].physical_valid;
        rt->batch_diag.worker_command_buffer_valid[w] = worker_resources[w].physical_valid;
        rt->batch_diag.worker_fence_valid[w] = worker_resources[w].physical_valid;
        rt->batch_diag.worker_reset_count[w] = worker_resources[w].reset_count;
        rt->batch_diag.worker_record_count[w] = worker_resources[w].record_count;
        rt->batch_diag.worker_failure_stage[w] = workers[w].failure_stage;
        rt->batch_diag.worker_failure_detail[w] = workers[w].failure_detail;
        rt->batch_diag.per_worker_queue_family[w] = worker_resources[w].queue_family_index;
        rt->batch_diag.per_worker_fence_state[w] = worker_resources[w].in_flight == 0u ? 1u : 2u;
        if (workers[w].active != 0u) {
          rt->batch_diag.worker_active_mask |= (1u << w);
        }
      }
    }
    if (worker_slots != NULL) {
      for (i = 0u; i < total_slot_count && i < 16u; ++i) {
        rt->batch_diag.slot_owner_worker_id[i] = worker_slots[i].owner_worker_id;
        rt->batch_diag.slot_state[i] = worker_slots[i].state;
        rt->batch_diag.slot_generation[i] = worker_slots[i].generation;
        rt->batch_diag.slot_entry_id[i] = worker_slots[i].assigned_entry_id;
        rt->batch_diag.slot_queue_id[i] = worker_slots[i].queue_id;
        rt->batch_diag.slot_command_resource_id[i] = worker_slots[i].command_resource_id;
        rt->batch_diag.slot_arena_id[i] = worker_slots[i].arena_id;
        rt->batch_diag.slot_output_staging_id[i] = worker_slots[i].output_staging_id;
        rt->batch_diag.slot_in_flight[i] = worker_slots[i].in_flight;
        rt->batch_diag.slot_ready[i] = worker_slots[i].ready;
        rt->batch_diag.slot_invalidated[i] = worker_slots[i].invalidated;
        rt->batch_diag.slot_failure_stage[i] = worker_slots[i].failure_stage;
        rt->batch_diag.slot_failure_detail[i] = worker_slots[i].failure_detail;
      }
    }
  }
  batch_free_staged_outputs(staged_outputs, entry_count);
  free(worker_thread_ctx);
  free(worker_threads);
  free(workers);
  batch_destroy_physical_worker_resources(rt, worker_resources, effective_workers);
  free(worker_slot_refill_cursor);
  free(worker_slots);
  free(worker_resources);
  free(worker_event_counts);
  free(worker_events);
  free(output_sizes);
  free(staged_outputs);
  free(plans);
  return final_status;
}

// ============================================================================
// SGEMM Async Lifecycle
// ============================================================================

int prom_reactor_runtime_sgemm_submit_async_impl(void* handle,
                                                 const float* a,
                                                 const float* b,
                                                 uint32_t m,
                                                 uint32_t n,
                                                 uint32_t k,
                                                 int* out_task_id,
                                                 uint32_t* out_stage,
                                                 int* out_detail_code) {
  prometheus_runtime* rt;
  uint32_t saved_flags;
  int status;

  if (out_task_id == NULL) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_ERROR);
    return PROM_ERROR;
  }
  if (handle == NULL || !registry_contains(handle)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }

  rt = (prometheus_runtime*)handle;
  saved_flags = rt->test_flags;
  rt->test_flags = saved_flags | PROM_TESTCFG_SKIP_SUBMIT_WAIT;
  status = prom_reactor_runtime_sgemm_impl(handle, a, b, NULL, m, n, k, out_stage, out_detail_code);
  rt->test_flags = saved_flags;
  if (status != PROM_OK) {
    return status;
  }

  *out_task_id = rt->async_task_id;
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_query_async_impl(void* handle, int task_id, PrometheusAsyncStatus* out_status) {
  prometheus_runtime* rt;
  prom_dom_async_snapshot async_snapshot;

  if (out_status == NULL) {
    return PROM_ERROR;
  }
  memset(out_status, 0, sizeof(*out_status));

  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  mirror_async_from_visible(rt);
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    return PROM_INVALID_HANDLE;
  }
  if (task_id != rt->async_task_id || rt->async_state == PROM_ASYNC_STATE_IDLE) {
    out_status->lifecycle_state = PROM_ASYNC_STATE_IDLE;
    out_status->detail_code = PROM_DETAIL_ASYNC_NO_TASK;
    stage_commit_async_snapshot(rt, PROM_DOM_EVENT_ASYNC_INVALID_TASK, PROM_DETAIL_ASYNC_NO_TASK);
    return PROM_ERROR;
  }

  update_async_progress(rt);
  if (prom_dom_sgemm_read_visible_async_snapshot(&rt->blackboard, &async_snapshot) == 0u) {
    return PROM_ERROR;
  }
  out_status->lifecycle_state = async_snapshot.lifecycle_state;
  out_status->stage = async_snapshot.stage;
  out_status->detail_code = async_snapshot.detail_code;
  out_status->ready = async_snapshot.ready;
  out_status->failed = async_snapshot.failed;
  out_status->consumed = async_snapshot.consumed;
  out_status->outstanding_tasks = async_snapshot.outstanding_tasks;
  if (async_snapshot.lifecycle_state == PROM_ASYNC_STATE_SUBMITTED) {
    stage_commit_async_snapshot(rt, PROM_DOM_EVENT_ASYNC_NOT_READY, PROM_DETAIL_ASYNC_NOT_READY);
  }
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_consume_async_impl(void* handle,
                                                  int task_id,
                                                  float* c,
                                                  uint32_t c_len,
                                                  uint32_t* out_stage,
                                                  int* out_detail_code) {
  prometheus_runtime* rt;
  uint32_t required_len;

  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_NONE, 0);
  if (handle == NULL || !registry_contains(handle)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  mirror_async_from_visible(rt);
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  if (task_id != rt->async_task_id || rt->async_state == PROM_ASYNC_STATE_IDLE) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_ASYNC_INVALID_TASK);
    stage_commit_async_snapshot(rt, PROM_DOM_EVENT_ASYNC_INVALID_TASK, PROM_DETAIL_ASYNC_INVALID_TASK);
    return PROM_ERROR;
  }
  update_async_progress(rt);
  if (rt->async_state == PROM_ASYNC_STATE_SUBMITTED) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_ASYNC_NOT_READY);
    stage_commit_async_snapshot(rt, PROM_DOM_EVENT_ASYNC_NOT_READY, PROM_DETAIL_ASYNC_NOT_READY);
    return PROM_ERROR;
  }
  if (rt->async_state == PROM_ASYNC_STATE_FAILED) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, rt->async_failure_detail != 0 ? rt->async_failure_detail : PROM_DETAIL_ASYNC_FAILED);
    return PROM_ERROR;
  }
  if (rt->async_state == PROM_ASYNC_STATE_CONSUMED) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_DETAIL_ASYNC_ALREADY_CONSUMED);
    stage_commit_async_snapshot(rt, PROM_DOM_EVENT_ASYNC_ALREADY_CONSUMED, PROM_DETAIL_ASYNC_ALREADY_CONSUMED);
    return PROM_ERROR;
  }
  if (c == NULL) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_ERROR);
    return PROM_ERROR;
  }
  required_len = rt->async_m * rt->async_n;
  if (c_len < required_len) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_ERROR);
    return PROM_ERROR;
  }

  if (rt->async_selected_path == PROM_VK_PATH_DIRECT) {
    memcpy(c, rt->direct_c.mapped, rt->async_c_copy_size);
  } else if (rt->async_selected_path == PROM_VK_PATH_STAGED_UPLOAD_READBACK) {
    memcpy(c, rt->staged_readback_c.mapped, rt->async_c_copy_size);
  } else {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, rt->async_final_detail);
    set_async_state(rt, PROM_ASYNC_STATE_CONSUMED, PROM_STAGE_SUBMIT, rt->async_final_detail);
    return PROM_OK;
  }

  set_async_state(rt, PROM_ASYNC_STATE_CONSUMED, PROM_STAGE_TRANSFER_OUT, rt->async_final_detail);
  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, rt->async_final_detail);
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_abandon_async_impl(void* handle, int task_id) {
  prometheus_runtime* rt;
  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  mirror_async_from_visible(rt);
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    return PROM_INVALID_HANDLE;
  }
  if (task_id != rt->async_task_id || rt->async_state == PROM_ASYNC_STATE_IDLE) {
    stage_commit_async_snapshot(rt, PROM_DOM_EVENT_ASYNC_INVALID_TASK, PROM_DETAIL_ASYNC_INVALID_TASK);
    return PROM_ERROR;
  }
  update_async_progress(rt);
  if (rt->async_state == PROM_ASYNC_STATE_SUBMITTED) {
    rt->slot_diag.inflight_rejection_count += 1u;
    commit_slot_runtime_diag_snapshot(rt, PROM_DETAIL_ASYNC_UNCONSUMED);
    stage_commit_async_snapshot(rt, PROM_DOM_EVENT_ASYNC_UNCONSUMED_REJECTED, PROM_DETAIL_ASYNC_UNCONSUMED);
    return PROM_ERROR;
  }
  if (rt->slot_diag.async_slot_id >= 0) {
    const uint32_t slot_id = (uint32_t)rt->slot_diag.async_slot_id;
    if (!prom_slot_cleanup_to_empty(rt, &rt->slots[slot_id])) {
      prom_slot_mark_failure(rt, slot_id, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
      return PROM_ERROR;
    }
    rt->slot_diag.async_slot_id = -1;
  }
  set_async_state(rt, PROM_ASYNC_STATE_CONSUMED, rt->async_stage, rt->async_final_detail);
  stage_commit_async_snapshot(rt, PROM_DOM_EVENT_ASYNC_ABANDONED, rt->async_final_detail);
  return PROM_OK;
}

// ============================================================================
// SGEMM Diagnostics Export
// ============================================================================

int prom_reactor_runtime_p15_test_seed_matured_reservation_impl(void* handle,
                                                          uint32_t shape_class,
                                                          uint32_t variant_id,
                                                          uint64_t target_tick) {
  prom_dominatus_future_lease_request req;
  prom_dominatus_reservation_decision d;
  prometheus_runtime* rt;
  if (handle == NULL || !registry_contains(handle)) return PROM_INVALID_HANDLE;
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) return PROM_INVALID_HANDLE;
  if (rt->p15_shadow_canary_params.enabled == 0u) return PROM_ERROR;
  memset(&req, 0, sizeof(req));
  req.valid = 1u;
  req.request_id = target_tick != 0u ? target_tick : 1u;
  req.target_tick = target_tick;
  req.shape_class = shape_class;
  req.variant_id = variant_id;
  req.lookahead_depth = 1u;
  req.confidence = 0.95;
  d = prom_dominatus_reservation_request_from_future_lease(&rt->p15_predictor_state.reservations,
                                                            &rt->p15_predictor_state.reservation_params,
                                                            &req,
                                                            target_tick > 0u ? target_tick - 1u : 0u);
  if (d.reserved == 0u) return PROM_ERROR;
  d = prom_dominatus_reservation_mature(&rt->p15_predictor_state.reservations, target_tick);
  if (d.matured == 0u) return PROM_ERROR;
  rt->p15_shadow_authority_gate.valid = 1u;
  rt->p15_shadow_authority_gate.state = PROM_SHADOW_AUTHORITY_HEALTHY;
  rt->p15_shadow_authority_gate.authority_enabled = 1u;
  rt->p15_shadow_canary_state.healthy_margin_passed = 1u;
  rt->p15_shadow_canary_state.reason_binding_passed = 1u;
  return PROM_OK;
}

static int prom_reactor_runtime_sgemm_policy_diagnostics_fill(void* handle, PrometheusSgemmPolicyDiagnostics* out_diag) {
  const prom_sgemm_controller_defaults defaults = prom_sgemm_default_config();
  prom_dom_sgemm_m35_snapshot m35_snapshot;
  prom_dom_transfer_queue_snapshot transfer_snapshot;
  prom_dom_sgemm_layout_precision_snapshot layout_precision_snapshot;
  prom_dom_slot_commit_snapshot slot_snapshot;
  prom_dom_slot_runtime_diag_snapshot slot_diag_snapshot;
  prom_dom_slot_readiness_snapshot slot_readiness_snapshot;
  prom_dom_sgemm_resource_lease_snapshot lease_snapshot;
  prometheus_runtime* rt;
  if (out_diag == NULL) {
    return PROM_ERROR;
  }
  memset(out_diag, 0, sizeof(*out_diag));
  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    return PROM_INVALID_HANDLE;
  }

  out_diag->current_mode = (uint32_t)rt->sgemm_controller.policy_memory.current_mode;
  out_diag->lookahead = rt->sgemm_controller.lookahead;
  out_diag->outstanding_depth = rt->sgemm_controller.outstanding_depth;
  out_diag->chunk_size = rt->sgemm_controller.chunk_size;
  out_diag->chunk_min = defaults.chunk_min;
  out_diag->chunk_max = defaults.chunk_max;
  out_diag->waste_budget_units = defaults.waste_budget_units;
  out_diag->pending_waste_units = rt->sgemm_controller.pending_waste_units;
  out_diag->wasted_work_units_last = rt->sgemm_controller.wasted_work_units_last;
  out_diag->wasted_work_units_total = rt->sgemm_controller.wasted_work_units_total;
  out_diag->decision_count = rt->sgemm_controller.decision_count;
  out_diag->retreat_count = rt->sgemm_controller.retreat_count;
  out_diag->recovery_count = rt->sgemm_controller.recovery_count;
  out_diag->transition_count = rt->sgemm_controller.transition_count;
  out_diag->instability_count = rt->sgemm_controller.instability_count;
  out_diag->budget_depletion_count = rt->sgemm_controller.budget_depletion_count;
  out_diag->safe_mode_decisions = rt->sgemm_controller.safe_mode_decisions;
  out_diag->aggressive_mode_decisions = rt->sgemm_controller.aggressive_mode_decisions;
  out_diag->recovery_mode_decisions = rt->sgemm_controller.recovery_mode_decisions;
  out_diag->lag_early_warning_count = rt->sgemm_controller.lag_early_warning_count;
  out_diag->burst_dampening_count = rt->sgemm_controller.burst_dampening_count;
  out_diag->bound_violation_count = rt->sgemm_controller.bound_violation_count;
  if (prom_dom_sgemm_read_visible_layout_precision_diagnostics(&rt->blackboard, &layout_precision_snapshot) != 0u) {
    out_diag->packed4_selected_layout_format = layout_precision_snapshot.decision.packed4_selected_layout_format;
    out_diag->packed4_tail_count_last = layout_precision_snapshot.decision.packed4_tail_count_last;
    out_diag->packed4_tail_count_total = layout_precision_snapshot.decision.packed4_tail_count_total;
    out_diag->packed4_padded_lane_count_last = layout_precision_snapshot.decision.packed4_padded_lane_count_last;
    out_diag->packed4_padded_lane_count_total = layout_precision_snapshot.decision.packed4_padded_lane_count_total;
    out_diag->packed4_padding_waste_permille_last = layout_precision_snapshot.decision.packed4_padding_waste_permille_last;
    out_diag->packed4_mode_budget_denials = layout_precision_snapshot.decision.packed4_mode_budget_denials;
    out_diag->packed4_row_major_check_failures = layout_precision_snapshot.decision.packed4_row_major_check_failures;
    out_diag->packed4_selection_count = layout_precision_snapshot.decision.packed4_selection_count;
    out_diag->packed4_fallback_reason_padding_waste = layout_precision_snapshot.decision.packed4_fallback_reason_padding_waste;
    out_diag->packed4_fallback_reason_small_shape = layout_precision_snapshot.decision.packed4_fallback_reason_small_shape;
    out_diag->packed4_fallback_reason_capability_missing = layout_precision_snapshot.decision.packed4_fallback_reason_capability_missing;
    out_diag->packed4_fallback_reason_fallback_required = layout_precision_snapshot.decision.packed4_fallback_reason_fallback_required;
    out_diag->packed4_fallback_reason_mode_budget_denied = layout_precision_snapshot.decision.packed4_fallback_reason_mode_budget_denied;
    out_diag->fp16_max_absolute_error = layout_precision_snapshot.decision.fp16_max_absolute_error;
    out_diag->fp16_max_relative_error = layout_precision_snapshot.decision.fp16_max_relative_error;
    out_diag->fp16_aggregate_error = layout_precision_snapshot.decision.fp16_aggregate_error;
    out_diag->fp16_worst_case_element_index = layout_precision_snapshot.decision.fp16_worst_case_element_index;
    out_diag->fp16_k_error_growth = layout_precision_snapshot.decision.fp16_k_error_growth;
    out_diag->fp16_cancellation_risk = layout_precision_snapshot.decision.fp16_cancellation_risk;
    out_diag->fp16_tolerance_known = layout_precision_snapshot.decision.fp16_tolerance_known;
    out_diag->fp16_tolerance_pass = layout_precision_snapshot.decision.fp16_tolerance_pass;
    out_diag->fp16_fallback_reason_detail = layout_precision_snapshot.decision.fp16_fallback_reason_detail;
    out_diag->fp16_selected_candidate = layout_precision_snapshot.decision.fp16_selected_candidate;
  } else {
    out_diag->packed4_selected_layout_format = rt->sgemm_controller.packed4_selected_layout_format;
    out_diag->packed4_tail_count_last = rt->sgemm_controller.packed4_tail_count_last;
    out_diag->packed4_tail_count_total = rt->sgemm_controller.packed4_tail_count_total;
    out_diag->packed4_padded_lane_count_last = rt->sgemm_controller.packed4_padded_lane_count_last;
    out_diag->packed4_padded_lane_count_total = rt->sgemm_controller.packed4_padded_lane_count_total;
    out_diag->packed4_padding_waste_permille_last = rt->sgemm_controller.packed4_padding_waste_permille_last;
    out_diag->packed4_mode_budget_denials = rt->sgemm_controller.packed4_mode_budget_denials;
    out_diag->packed4_row_major_check_failures = rt->sgemm_controller.packed4_row_major_check_failures;
    out_diag->packed4_selection_count = rt->sgemm_controller.packed4_selection_count;
    out_diag->packed4_fallback_reason_padding_waste = rt->sgemm_controller.packed4_fallback_reason_padding_waste;
    out_diag->packed4_fallback_reason_small_shape = rt->sgemm_controller.packed4_fallback_reason_small_shape;
    out_diag->packed4_fallback_reason_capability_missing = rt->sgemm_controller.packed4_fallback_reason_capability_missing;
    out_diag->packed4_fallback_reason_fallback_required = rt->sgemm_controller.packed4_fallback_reason_fallback_required;
    out_diag->packed4_fallback_reason_mode_budget_denied = rt->sgemm_controller.packed4_fallback_reason_mode_budget_denied;
    out_diag->fp16_max_absolute_error = rt->sgemm_controller.fp16_max_absolute_error;
    out_diag->fp16_max_relative_error = rt->sgemm_controller.fp16_max_relative_error;
    out_diag->fp16_aggregate_error = rt->sgemm_controller.fp16_aggregate_error;
    out_diag->fp16_worst_case_element_index = rt->sgemm_controller.fp16_worst_case_element_index;
    out_diag->fp16_k_error_growth = rt->sgemm_controller.fp16_k_error_growth;
    out_diag->fp16_cancellation_risk = rt->sgemm_controller.fp16_cancellation_risk;
    out_diag->fp16_tolerance_known = rt->sgemm_controller.fp16_tolerance_known;
    out_diag->fp16_tolerance_pass = rt->sgemm_controller.fp16_tolerance_pass;
    out_diag->fp16_fallback_reason_detail = rt->sgemm_controller.fp16_fallback_reason_detail;
    out_diag->fp16_selected_candidate = rt->sgemm_controller.fp16_selected_candidate;
  }
  if (prom_dom_slot_read_visible_runtime_diag(&rt->blackboard, &slot_diag_snapshot) != 0u) {
    out_diag->m29_current_slot_id = slot_diag_snapshot.current_slot_id;
    out_diag->m29_next_slot_id = slot_diag_snapshot.next_slot_id;
    out_diag->m29_slot0_state = slot_diag_snapshot.slot_state[0];
    out_diag->m29_slot1_state = slot_diag_snapshot.slot_state[1];
    out_diag->m29_slot0_generation = slot_diag_snapshot.slot_generation[0];
    out_diag->m29_slot1_generation = slot_diag_snapshot.slot_generation[1];
    out_diag->m29_slot0_valid = slot_diag_snapshot.slot_valid[0];
    out_diag->m29_slot1_valid = slot_diag_snapshot.slot_valid[1];
    out_diag->m29_swap_count = slot_diag_snapshot.swap_count;
    out_diag->m29_max_wip_depth = slot_diag_snapshot.max_wip_depth;
    out_diag->m29_overwrite_rejection_count = slot_diag_snapshot.overwrite_rejection_count;
    out_diag->m29_stale_buffer_rejection_count = slot_diag_snapshot.stale_buffer_rejection_count;
    out_diag->m29_shape_invalidation_count = slot_diag_snapshot.shape_invalidation_count;
    out_diag->m29_layout_invalidation_count = slot_diag_snapshot.layout_invalidation_count;
    out_diag->m29_capacity_invalidation_count = slot_diag_snapshot.capacity_invalidation_count;
    out_diag->m14_a_invalidation_count = rt->slot_diag.m14_a_invalidation_count;
    out_diag->m14_b_invalidation_count = rt->slot_diag.m14_b_invalidation_count;
    out_diag->m14_c_invalidation_count = rt->slot_diag.m14_c_invalidation_count;
    out_diag->m14_a_reuse_count = rt->slot_diag.m14_a_reuse_count;
    out_diag->m14_b_reuse_count = rt->slot_diag.m14_b_reuse_count;
    out_diag->m14_c_reuse_count = rt->slot_diag.m14_c_reuse_count;
    out_diag->m14_false_invalidation_avoided_count = rt->slot_diag.m14_false_invalidation_avoided_count;
    out_diag->m14_capacity_invalidation_count = rt->slot_diag.m14_capacity_invalidation_count;
    out_diag->m14_layout_precision_invalidation_count = rt->slot_diag.m14_layout_precision_invalidation_count;
    out_diag->m14_a_last_invalidation_reason = rt->slot_diag.m14_a_last_invalidation_reason;
    out_diag->m14_b_last_invalidation_reason = rt->slot_diag.m14_b_last_invalidation_reason;
    out_diag->m14_c_last_invalidation_reason = rt->slot_diag.m14_c_last_invalidation_reason;
    out_diag->m29_inflight_rejection_count = slot_diag_snapshot.inflight_rejection_count;
    out_diag->m29_cleanup_success_count = slot_diag_snapshot.cleanup_success_count;
    out_diag->m29_failure_slot_id = slot_diag_snapshot.failure_slot_id;
    out_diag->m29_failure_reason = slot_diag_snapshot.failure_reason;

    rt->slot_diag.current_slot_id = slot_diag_snapshot.current_slot_id;
    rt->slot_diag.next_slot_id = slot_diag_snapshot.next_slot_id;
    rt->slot_diag.swap_count = slot_diag_snapshot.swap_count;
    rt->slot_diag.max_wip_depth = slot_diag_snapshot.max_wip_depth;
    rt->slot_diag.overwrite_rejection_count = slot_diag_snapshot.overwrite_rejection_count;
    rt->slot_diag.stale_buffer_rejection_count = slot_diag_snapshot.stale_buffer_rejection_count;
    rt->slot_diag.shape_invalidation_count = slot_diag_snapshot.shape_invalidation_count;
    rt->slot_diag.layout_invalidation_count = slot_diag_snapshot.layout_invalidation_count;
    rt->slot_diag.capacity_invalidation_count = slot_diag_snapshot.capacity_invalidation_count;
    rt->slot_diag.inflight_rejection_count = slot_diag_snapshot.inflight_rejection_count;
    rt->slot_diag.cleanup_success_count = slot_diag_snapshot.cleanup_success_count;
    rt->slot_diag.failure_slot_id = slot_diag_snapshot.failure_slot_id;
    rt->slot_diag.failure_reason = slot_diag_snapshot.failure_reason;
  } else {
    out_diag->m29_current_slot_id = rt->slot_diag.current_slot_id;
    out_diag->m29_next_slot_id = rt->slot_diag.next_slot_id;
    out_diag->m29_slot0_state = (uint32_t)prom_slot_hfsm_current_state(&rt->slots[0]);
    out_diag->m29_slot1_state = (uint32_t)prom_slot_hfsm_current_state(&rt->slots[1]);
    out_diag->m29_slot0_generation = prom_slot_hfsm_metadata(&rt->slots[0])->generation;
    out_diag->m29_slot1_generation = prom_slot_hfsm_metadata(&rt->slots[1])->generation;
    out_diag->m29_slot0_valid = prom_slot_hfsm_metadata(&rt->slots[0])->valid;
    out_diag->m29_slot1_valid = prom_slot_hfsm_metadata(&rt->slots[1])->valid;
    out_diag->m29_swap_count = rt->slot_diag.swap_count;
    out_diag->m29_max_wip_depth = rt->slot_diag.max_wip_depth;
    out_diag->m29_overwrite_rejection_count = rt->slot_diag.overwrite_rejection_count;
    out_diag->m29_stale_buffer_rejection_count = rt->slot_diag.stale_buffer_rejection_count;
    out_diag->m29_shape_invalidation_count = rt->slot_diag.shape_invalidation_count;
    out_diag->m29_layout_invalidation_count = rt->slot_diag.layout_invalidation_count;
    out_diag->m29_capacity_invalidation_count = rt->slot_diag.capacity_invalidation_count;
    out_diag->m14_a_invalidation_count = rt->slot_diag.m14_a_invalidation_count;
    out_diag->m14_b_invalidation_count = rt->slot_diag.m14_b_invalidation_count;
    out_diag->m14_c_invalidation_count = rt->slot_diag.m14_c_invalidation_count;
    out_diag->m14_a_reuse_count = rt->slot_diag.m14_a_reuse_count;
    out_diag->m14_b_reuse_count = rt->slot_diag.m14_b_reuse_count;
    out_diag->m14_c_reuse_count = rt->slot_diag.m14_c_reuse_count;
    out_diag->m14_false_invalidation_avoided_count = rt->slot_diag.m14_false_invalidation_avoided_count;
    out_diag->m14_capacity_invalidation_count = rt->slot_diag.m14_capacity_invalidation_count;
    out_diag->m14_layout_precision_invalidation_count = rt->slot_diag.m14_layout_precision_invalidation_count;
    out_diag->m14_a_last_invalidation_reason = rt->slot_diag.m14_a_last_invalidation_reason;
    out_diag->m14_b_last_invalidation_reason = rt->slot_diag.m14_b_last_invalidation_reason;
    out_diag->m14_c_last_invalidation_reason = rt->slot_diag.m14_c_last_invalidation_reason;
    out_diag->m29_inflight_rejection_count = rt->slot_diag.inflight_rejection_count;
    out_diag->m29_cleanup_success_count = rt->slot_diag.cleanup_success_count;
    out_diag->m29_failure_slot_id = rt->slot_diag.failure_slot_id;
    out_diag->m29_failure_reason = rt->slot_diag.failure_reason;
  }
  if (prom_dom_sgemm_read_visible_transfer_queue_diagnostics(&rt->blackboard, &transfer_snapshot) != 0u) {
    out_diag->m31_transfer_queue_used = transfer_snapshot.transfer_queue_used;
    out_diag->m31_transfer_policy_selected = transfer_snapshot.transfer_policy_selected;
    out_diag->m31_dedicated_transfer_available = transfer_snapshot.dedicated_transfer_available;
    out_diag->m31_transfer_queue_family_index = transfer_snapshot.transfer_queue_family_index;
    out_diag->m31_compute_queue_family_index = transfer_snapshot.compute_queue_family_index;
    out_diag->m31_queue_families_differ = transfer_snapshot.queue_families_differ;
    out_diag->m31_transfer_fallback_reason = transfer_snapshot.transfer_fallback_reason;
    out_diag->m31_upload_policy_marker = transfer_snapshot.upload_only_policy_eligible;
    out_diag->m31_queue_family_handoff_count = transfer_snapshot.queue_family_handoff_count;
    out_diag->m31_transfer_compute_wait_count = transfer_snapshot.transfer_compute_wait_count;
    out_diag->m31_transfer_failure_slot_id = transfer_snapshot.transfer_failure_slot_id;
    out_diag->m31_transfer_failure_reason = transfer_snapshot.transfer_failure_reason;
    out_diag->m31_async_transfer_complete = transfer_snapshot.async_transfer_complete;
  } else {
    out_diag->m31_transfer_queue_used = rt->slot_diag.transfer_queue_used;
    out_diag->m31_transfer_policy_selected = rt->slot_diag.transfer_policy_selected;
    out_diag->m31_dedicated_transfer_available = rt->slot_diag.dedicated_transfer_available;
    out_diag->m31_transfer_queue_family_index = rt->slot_diag.transfer_queue_family_index;
    out_diag->m31_compute_queue_family_index = rt->slot_diag.compute_queue_family_index;
    out_diag->m31_queue_families_differ = rt->slot_diag.queue_families_differ;
    out_diag->m31_transfer_fallback_reason = rt->slot_diag.transfer_fallback_reason;
    out_diag->m31_upload_policy_marker = 1u;
    out_diag->m31_queue_family_handoff_count = rt->slot_diag.queue_family_handoff_count;
    out_diag->m31_transfer_compute_wait_count = rt->slot_diag.transfer_compute_wait_count;
    out_diag->m31_transfer_failure_slot_id = rt->slot_diag.transfer_failure_slot_id;
    out_diag->m31_transfer_failure_reason = rt->slot_diag.transfer_failure_reason;
    out_diag->m31_async_transfer_complete = rt->slot_diag.async_transfer_complete;
  }
  if (prom_dom_sgemm_read_visible_m35(&rt->blackboard, &m35_snapshot) != 0u) {
    out_diag->m35_selected_buffering_mode = m35_snapshot.selected_mode;
    out_diag->m35_fixed_feasible = m35_snapshot.fixed_feasible;
    out_diag->m35_pull_lag_feasible = m35_snapshot.pull_lag_feasible;
    out_diag->m35_serial_feasible = m35_snapshot.serial_feasible;
    out_diag->m35_fixed_rejected = m35_snapshot.fixed_rejected;
    out_diag->m35_pull_lag_rejected = m35_snapshot.pull_lag_rejected;
    out_diag->m35_serial_rejected = m35_snapshot.serial_rejected;
    out_diag->m35_fixed_score = (uint32_t)(m35_snapshot.fixed_score < 0 ? 0 : m35_snapshot.fixed_score);
    out_diag->m35_pull_lag_score = (uint32_t)(m35_snapshot.pull_lag_score < 0 ? 0 : m35_snapshot.pull_lag_score);
    out_diag->m35_serial_score = (uint32_t)(m35_snapshot.serial_score < 0 ? 0 : m35_snapshot.serial_score);
    out_diag->m35_reason_code = m35_snapshot.reason_code;
    out_diag->m35_final_reason_code = m35_snapshot.final_reason_code;
    out_diag->m35_fixed_double_rejection_reason = m35_snapshot.fixed_double_rejection_reason;
    out_diag->m35_pull_lag_rejection_reason = m35_snapshot.pull_lag_rejection_reason;
    out_diag->m35_serial_jit_rejection_reason = m35_snapshot.serial_jit_rejection_reason;
    out_diag->m35_memory_budget_slots_permille = m35_snapshot.memory_budget_slots_permille;
    out_diag->m35_required_fixed_slots_permille = m35_snapshot.required_fixed_slots_permille;
    out_diag->m35_required_pull_lag_slots_permille = m35_snapshot.required_pull_lag_peak_slots_permille;
    out_diag->m35_required_serial_slots_permille = m35_snapshot.required_serial_slots_permille;
    out_diag->m35_fixed_double_headroom_slots_permille = (int64_t)m35_snapshot.fixed_double_headroom_slots_permille;
    out_diag->m35_pull_lag_headroom_slots_permille = (int64_t)m35_snapshot.pull_lag_headroom_slots_permille;
    out_diag->m35_serial_jit_headroom_slots_permille = (int64_t)m35_snapshot.serial_jit_headroom_slots_permille;
  } else {
    out_diag->m35_selected_buffering_mode = rt->slot_diag.m35_selected_mode;
    out_diag->m35_fixed_feasible = rt->slot_diag.m35_fixed_feasible;
    out_diag->m35_pull_lag_feasible = rt->slot_diag.m35_pull_lag_feasible;
    out_diag->m35_serial_feasible = rt->slot_diag.m35_serial_feasible;
    out_diag->m35_fixed_rejected = rt->slot_diag.m35_fixed_rejected;
    out_diag->m35_pull_lag_rejected = rt->slot_diag.m35_pull_lag_rejected;
    out_diag->m35_serial_rejected = rt->slot_diag.m35_serial_rejected;
    out_diag->m35_fixed_score = (uint32_t)(rt->slot_diag.m35_fixed_score < 0 ? 0 : rt->slot_diag.m35_fixed_score);
    out_diag->m35_pull_lag_score = (uint32_t)(rt->slot_diag.m35_pull_lag_score < 0 ? 0 : rt->slot_diag.m35_pull_lag_score);
    out_diag->m35_serial_score = (uint32_t)(rt->slot_diag.m35_serial_score < 0 ? 0 : rt->slot_diag.m35_serial_score);
    out_diag->m35_reason_code = rt->slot_diag.m35_reason_code;
    out_diag->m35_final_reason_code = rt->slot_diag.m35_final_reason_code;
    out_diag->m35_fixed_double_rejection_reason = rt->slot_diag.m35_fixed_double_rejection_reason;
    out_diag->m35_pull_lag_rejection_reason = rt->slot_diag.m35_pull_lag_rejection_reason;
    out_diag->m35_serial_jit_rejection_reason = rt->slot_diag.m35_serial_jit_rejection_reason;
    out_diag->m35_memory_budget_slots_permille = rt->slot_diag.m35_memory_budget_slots_permille;
    out_diag->m35_required_fixed_slots_permille = rt->slot_diag.m35_required_fixed_slots_permille;
    out_diag->m35_required_pull_lag_slots_permille = rt->slot_diag.m35_required_pull_lag_slots_permille;
    out_diag->m35_required_serial_slots_permille = rt->slot_diag.m35_required_serial_slots_permille;
    out_diag->m35_fixed_double_headroom_slots_permille = rt->slot_diag.m35_fixed_double_headroom_slots_permille;
    out_diag->m35_pull_lag_headroom_slots_permille = rt->slot_diag.m35_pull_lag_headroom_slots_permille;
    out_diag->m35_serial_jit_headroom_slots_permille = rt->slot_diag.m35_serial_jit_headroom_slots_permille;
  }
  out_diag->m35_transition_count = rt->slot_diag.m35_transition_count;
  out_diag->m35_rejection_count = rt->slot_diag.m35_rejection_count;
  out_diag->m35_budget_rejection_count = rt->slot_diag.m35_budget_rejection_count;
  out_diag->m35_pull_lag_predicted_demand_proxy_units = rt->slot_diag.m35_pull_lag_predicted_demand_proxy_units;
  out_diag->m35_pull_lag_transfer_lead_proxy_units = rt->slot_diag.m35_pull_lag_transfer_lead_proxy_units;
  out_diag->m35_pull_lag_safety_margin_proxy_units = rt->slot_diag.m35_pull_lag_safety_margin_proxy_units;
  out_diag->m35_pull_lag_stage_start_proxy_units = rt->slot_diag.m35_pull_lag_stage_start_proxy_units;
  out_diag->m35_pull_lag_stage_complete_proxy_units = rt->slot_diag.m35_pull_lag_stage_complete_proxy_units;
  out_diag->m35_pull_lag_late_stage_count = rt->slot_diag.m35_pull_lag_late_stage_count;
  out_diag->m35_pull_lag_early_stage_count = rt->slot_diag.m35_pull_lag_early_stage_count;
  out_diag->m35_pull_lag_starvation_proxy_units = rt->slot_diag.m35_pull_lag_starvation_proxy_units;
  out_diag->m35_pull_lag_ready_unused_proxy_units = rt->slot_diag.m35_pull_lag_ready_unused_proxy_units;
  out_diag->m35_pull_lag_wip_waste_exceeded_count = rt->slot_diag.m35_pull_lag_wip_waste_exceeded_count;
  out_diag->m35_serial_active_slot_count = rt->slot_diag.m35_serial_active_slot_count;
  out_diag->m35_serial_wip_depth = rt->slot_diag.m35_serial_wip_depth;
  out_diag->m35_serial_sequential_step_count = rt->slot_diag.m35_serial_sequential_step_count;
  out_diag->m35_serial_busy_retry_count = rt->slot_diag.m35_serial_busy_retry_count;
  out_diag->m35_serial_failure_cleanup_count = rt->slot_diag.m35_serial_failure_cleanup_count;
  out_diag->p13_m2_occupancy_device_band = rt->slot_diag.p13_m2_occupancy_device_band;
  out_diag->p13_m2_occupancy_shape_class = rt->slot_diag.p13_m2_occupancy_shape_class;
  out_diag->p13_m2_occupancy_selected_variant = rt->slot_diag.p13_m2_occupancy_selected_variant;
  out_diag->p13_m2_occupancy_unclamped_variant = rt->slot_diag.p13_m2_occupancy_unclamped_variant;
  out_diag->p13_m2_occupancy_clamp_reason = rt->slot_diag.p13_m2_occupancy_clamp_reason;
  out_diag->p13_m2_occupancy_override_used = rt->slot_diag.p13_m2_occupancy_override_used;
  out_diag->p13_m2_occupancy_fallback_used = rt->slot_diag.p13_m2_occupancy_fallback_used;
  out_diag->p13_m16b1_requested_occupancy_variant = rt->slot_diag.p13_m16b1_requested_occupancy_variant;
  out_diag->p13_m16b1_executed_occupancy_variant = rt->slot_diag.p13_m16b1_executed_occupancy_variant;
  out_diag->p13_m16b1_variant_registered = rt->slot_diag.p13_m16b1_variant_registered;
  out_diag->p13_m16b1_variant_benchmark_enabled = rt->slot_diag.p13_m16b1_variant_benchmark_enabled;
  out_diag->p13_m16b1_variant_dvt_validated = rt->slot_diag.p13_m16b1_variant_dvt_validated;
  out_diag->p13_m16b1_variant_pvt_validated = rt->slot_diag.p13_m16b1_variant_pvt_validated;
  out_diag->p13_m16b1_variant_production_eligible = rt->slot_diag.p13_m16b1_variant_production_eligible;
  out_diag->p13_m16b1_variant_dispatch_enabled = rt->slot_diag.p13_m16b1_variant_dispatch_enabled;
  out_diag->p13_m16b1_variant_path_status = rt->slot_diag.p13_m16b1_variant_path_status;
  out_diag->p13_m16b1_variant_path_id = rt->slot_diag.p13_m16b1_variant_path_id;
  out_diag->p13_m16b1_fallback_reason = rt->slot_diag.p13_m16b1_fallback_reason;
  out_diag->p13_m5_timestamp_available = rt->timestamp_query_supported;
  out_diag->p13_m5_last_gpu_timing_valid = rt->last_gpu_timing_valid;
  out_diag->p13_m5_last_gpu_timing_failure_reason = rt->last_gpu_timing_failure_reason;
  out_diag->p13_m5_last_gpu_duration_ns = rt->last_gpu_duration_ns;
  out_diag->p14_m8_filter_evidence_valid = rt->p14_last_filtered_evidence.valid;
  out_diag->p14_m8_raw_gpu_duration_ns = rt->p14_last_filtered_evidence.raw_value;
  out_diag->p14_m8_filtered_gpu_duration_ns = rt->p14_last_filtered_evidence.filtered_value;
  out_diag->p14_m8_filter_residual = rt->p14_last_filtered_evidence.residual;
  out_diag->p14_m8_filter_confidence = rt->p14_last_filtered_evidence.confidence;
  out_diag->p14_m8_filter_selected_kind = (uint32_t)rt->p14_last_filtered_evidence.selected_filter;
  out_diag->p14_m8_filter_previous_kind = (uint32_t)rt->p14_last_filtered_evidence.previous_filter;
  out_diag->p14_m8_filter_switched = rt->p14_last_filtered_evidence.filter_switched;
  out_diag->p14_m8_filter_warmup = rt->p14_last_filtered_evidence.filter_warmup;
  out_diag->p14_m8_filter_held_by_min_commit = rt->p14_last_filtered_evidence.held_by_min_commit;
  out_diag->p14_m8_filter_held_by_margin = rt->p14_last_filtered_evidence.held_by_margin;
  out_diag->p14_m8_filter_held_by_confidence = rt->p14_last_filtered_evidence.held_by_confidence;
  out_diag->p14_m8_filter_warm_transferred = rt->p14_last_filtered_evidence.warm_transferred;
  out_diag->p14_m8_filter_sample_count = rt->p14_last_filtered_evidence.sample_count;
  out_diag->p14_m8_filter_outlier_count = rt->p14_last_filtered_evidence.outlier_count;
  out_diag->p15_predictor_valid = rt->p14_last_filtered_evidence.valid;
  out_diag->p15_prediction_confidence = rt->p15_predictor_state.prediction_confidence;
  out_diag->p15_lookahead_depth = rt->p15_predictor_state.lookahead_depth;
  out_diag->p15_prediction_issued = rt->p15_last_prediction_issued.active;
  out_diag->p15_prediction_matured = rt->p15_last_correction.prediction_matured;
  out_diag->p15_predicted_ready_tick = rt->p15_last_correction.target_tick;
  out_diag->p15_actual_ready_tick = rt->p15_last_correction.prediction_matured != 0u ? rt->p15_last_correction.tick : 0u;
  out_diag->p15_prediction_error_ticks = rt->p15_last_correction.arrival_error_ticks;
  out_diag->p15_correction_count = rt->p15_predictor_state.correction_count;
  out_diag->p15_correction_action = (uint32_t)rt->p15_last_correction.action;
  out_diag->p15_fallback_active = rt->p15_predictor_state.fallback_active;
  out_diag->p15_fallback_reason = rt->p15_predictor_state.fallback_reason;
  out_diag->p15_future_lease_valid = rt->p15_predictor_state.future_lease_seam.last_request.valid;
  out_diag->p15_future_lease_request_id = rt->p15_predictor_state.future_lease_seam.last_request.request_id;
  out_diag->p15_future_lease_state = (uint32_t)rt->p15_predictor_state.future_lease_seam.last_request.state;
  out_diag->p15_future_lease_target_tick = rt->p15_predictor_state.future_lease_seam.last_request.target_tick;
  out_diag->p15_future_lease_confidence = rt->p15_predictor_state.future_lease_seam.last_request.confidence;
  out_diag->p15_future_lease_reason = rt->p15_predictor_state.future_lease_seam.last_request.cancel_reason;
  out_diag->p15_reservation_valid = rt->p15_last_reservation.valid;
  out_diag->p15_reservation_request_id = rt->p15_last_reservation.request_id;
  out_diag->p15_reservation_state = (uint32_t)rt->p15_last_reservation.new_state;
  out_diag->p15_reservation_reserved = rt->p15_last_reservation.reserved;
  out_diag->p15_reservation_denied = rt->p15_last_reservation.denied;
  out_diag->p15_reservation_cancelled = rt->p15_last_reservation.cancelled;
  out_diag->p15_reservation_matured = rt->p15_last_reservation.matured;
  out_diag->p15_reservation_expired = rt->p15_last_reservation.expired;
  out_diag->p15_reservation_reason = rt->p15_last_reservation.reason;
  out_diag->p15_reservation_active_count = rt->p15_last_reservation.active_count;
  out_diag->p15_prestage_valid = rt->p15_last_prestage.valid;
  out_diag->p15_prestage_state = (uint32_t)rt->p15_last_prestage.state;
  out_diag->p15_prestage_allowed = rt->p15_last_prestage.allowed;
  out_diag->p15_prestage_submitted = rt->p15_last_prestage.submitted;
  out_diag->p15_prestage_block_reasons = rt->p15_last_prestage.block_reasons;
  out_diag->p15_prestage_confidence = rt->p15_last_prestage.confidence;
  out_diag->p15_prestage_target_tick = rt->p15_last_prestage.target_tick;
  out_diag->p15_prestage_lead_ticks = rt->p15_last_prestage.lead_ticks;
  out_diag->p15_prestage_cost_estimate = rt->p15_last_prestage.cost_estimate;
  out_diag->p15_prestage_benefit_estimate = rt->p15_last_prestage.benefit_estimate;
  out_diag->p15_shadow_valid = rt->p15_last_shadow.valid;
  out_diag->p15_shadow_state = (uint32_t)rt->p15_last_shadow.shadow_state;
  out_diag->p15_shadow_physical_state = rt->p15_last_shadow.physical_state;
  out_diag->p15_shadow_issued_tick = rt->p15_last_shadow.issued_tick;
  out_diag->p15_shadow_target_tick = rt->p15_last_shadow.target_tick;
  out_diag->p15_shadow_predicted_ready_tick = rt->p15_last_shadow.predicted_ready_tick;
  out_diag->p15_shadow_actual_ready_tick = rt->p15_last_shadow.actual_ready_tick;
  out_diag->p15_shadow_arrival_error_ticks = rt->p15_last_shadow.arrival_error_ticks;
  out_diag->p15_shadow_prediction_confidence = rt->p15_last_shadow.prediction_confidence;
  out_diag->p15_shadow_mismatch_kind = (uint32_t)rt->p15_last_shadow.mismatch_kind;
  out_diag->p15_shadow_matched = rt->p15_last_shadow.matched;
  out_diag->p15_shadow_stale = rt->p15_last_shadow.stale;
  out_diag->p15_shadow_cancelled = rt->p15_last_shadow.cancelled;
  out_diag->p15_shadow_fallback = rt->p15_last_shadow.fallback;
  out_diag->p15_shadow_correction_action = (uint32_t)rt->p15_last_shadow.correction_action;
  out_diag->p15_shadow_correction_count = rt->p15_last_shadow.correction_count;
  out_diag->p15_shadow_stale_count = rt->p15_last_shadow.stale_count;
  out_diag->p15_shadow_miss_count = rt->p15_last_shadow.miss_count;
  out_diag->p15_shadow_calibration_valid = rt->p15_shadow_calibration.valid;
  out_diag->p15_shadow_calibration_sample_count = rt->p15_shadow_calibration.sample_count;
  out_diag->p15_shadow_calibration_match_count = rt->p15_shadow_calibration.match_count;
  out_diag->p15_shadow_calibration_miss_count = rt->p15_shadow_calibration.miss_count;
  out_diag->p15_shadow_calibration_early_count = rt->p15_shadow_calibration.early_count;
  out_diag->p15_shadow_calibration_late_count = rt->p15_shadow_calibration.late_count;
  out_diag->p15_shadow_calibration_stale_count = rt->p15_shadow_calibration.stale_count;
  out_diag->p15_shadow_calibration_fallback_count = rt->p15_shadow_calibration.fallback_count;
  out_diag->p15_shadow_calibration_confidence = rt->p15_shadow_calibration.confidence;
  out_diag->p15_shadow_calibration_mean_abs_arrival_error_ticks =
      rt->p15_shadow_calibration.sample_count == 0u
          ? 0.0
          : (double)rt->p15_shadow_calibration.total_abs_arrival_error_ticks / (double)rt->p15_shadow_calibration.sample_count;
  out_diag->p15_shadow_calibration_last_mismatch_kind = (uint32_t)rt->p15_shadow_calibration.last_mismatch_kind;
  out_diag->p15_shadow_lookahead_state = (uint32_t)rt->p15_shadow_calibration.lookahead_diagnostic_state;
  out_diag->p15_shadow_authority_valid = rt->p15_shadow_authority_gate.valid;
  out_diag->p15_shadow_authority_state = (uint32_t)rt->p15_shadow_authority_gate.state;
  out_diag->p15_shadow_authority_reason = (uint32_t)rt->p15_shadow_authority_gate.reason;
  out_diag->p15_shadow_authority_canary_allowed = rt->p15_shadow_authority_gate.canary_allowed;
  out_diag->p15_shadow_authority_would_act = rt->p15_shadow_authority_gate.authority_would_act;
  out_diag->p15_shadow_authority_enabled = rt->p15_shadow_authority_gate.authority_enabled;
  out_diag->p15_shadow_authority_recommended_lookahead_depth = rt->p15_shadow_authority_gate.recommended_lookahead_depth;
  out_diag->p15_shadow_authority_confidence_gate_passed = rt->p15_shadow_authority_gate.confidence_gate_passed;
  out_diag->p15_shadow_authority_sample_gate_passed = rt->p15_shadow_authority_gate.sample_gate_passed;
  out_diag->p15_shadow_authority_miss_rate_gate_passed = rt->p15_shadow_authority_gate.miss_rate_gate_passed;
  out_diag->p15_shadow_authority_arrival_error_gate_passed = rt->p15_shadow_authority_gate.arrival_error_gate_passed;
  out_diag->p15_shadow_authority_lookahead_gate_passed = rt->p15_shadow_authority_gate.lookahead_state_gate_passed;
  out_diag->p15_shadow_authority_match_rate = rt->p15_shadow_authority_gate.match_rate;
  out_diag->p15_shadow_authority_miss_rate = rt->p15_shadow_authority_gate.miss_rate;
  out_diag->p15_shadow_authority_mean_abs_arrival_error_ticks = rt->p15_shadow_authority_gate.mean_abs_arrival_error_ticks;
  out_diag->p15_shadow_would_act_valid = rt->p15_shadow_would_act_state.valid;
  out_diag->p15_shadow_would_act_evaluation_count = rt->p15_shadow_would_act_state.evaluation_count;
  out_diag->p15_shadow_would_act_count = rt->p15_shadow_would_act_state.would_act_count;
  out_diag->p15_shadow_would_block_count = rt->p15_shadow_would_act_state.would_block_count;
  out_diag->p15_shadow_would_unknown_count = rt->p15_shadow_would_act_state.would_unknown_count;
  out_diag->p15_shadow_would_disabled_count = rt->p15_shadow_would_act_state.would_disabled_count;
  out_diag->p15_shadow_would_canary_count = rt->p15_shadow_would_act_state.would_canary_count;
  out_diag->p15_shadow_would_healthy_count = rt->p15_shadow_would_act_state.would_healthy_count;
  out_diag->p15_shadow_would_block_low_confidence_count = rt->p15_shadow_would_act_state.blocked_low_confidence_count;
  out_diag->p15_shadow_would_block_high_miss_rate_count = rt->p15_shadow_would_act_state.blocked_high_miss_rate_count;
  out_diag->p15_shadow_would_block_high_arrival_error_count = rt->p15_shadow_would_act_state.blocked_high_arrival_error_count;
  out_diag->p15_shadow_would_block_recent_fallback_count = rt->p15_shadow_would_act_state.blocked_recent_fallback_count;
  out_diag->p15_shadow_would_block_recent_stale_count = rt->p15_shadow_would_act_state.blocked_recent_stale_count;
  out_diag->p15_shadow_would_block_insufficient_samples_count = rt->p15_shadow_would_act_state.blocked_insufficient_samples_count;
  out_diag->p15_shadow_would_block_invalid_calibration_count = rt->p15_shadow_would_act_state.blocked_invalid_calibration_count;
  out_diag->p15_shadow_would_block_lookahead_disabled_count = rt->p15_shadow_would_act_state.blocked_lookahead_disabled_count;
  out_diag->p15_shadow_would_healthy_suppressed_by_recent_fallback_count =
      rt->p15_shadow_would_act_state.healthy_suppressed_by_recent_fallback_count;
  out_diag->p15_shadow_would_healthy_suppressed_by_recent_stale_count =
      rt->p15_shadow_would_act_state.healthy_suppressed_by_recent_stale_count;
  out_diag->p15_shadow_would_healthy_suppressed_by_arrival_error_count =
      rt->p15_shadow_would_act_state.healthy_suppressed_by_arrival_error_count;
  out_diag->p15_shadow_would_last_would_act = rt->p15_shadow_would_act_state.last_would_act;
  out_diag->p15_shadow_would_last_reason = (uint32_t)rt->p15_shadow_would_act_state.last_would_block_reason;
  out_diag->p15_shadow_would_last_gate_state = (uint32_t)rt->p15_shadow_would_act_state.last_gate_state;
  out_diag->p15_shadow_would_last_recommended_lookahead_depth = rt->p15_shadow_would_act_state.last_recommended_lookahead_depth;
  out_diag->p15_shadow_canary_valid = rt->p15_shadow_canary_state.valid;
  out_diag->p15_shadow_canary_enabled = rt->p15_shadow_canary_state.enabled;
  out_diag->p15_shadow_canary_last_action_allowed = rt->p15_shadow_canary_state.last_action_allowed;
  out_diag->p15_shadow_canary_last_action_kind = (uint32_t)rt->p15_shadow_canary_state.last_action_kind;
  out_diag->p15_shadow_canary_last_block_reason = (uint32_t)rt->p15_shadow_canary_state.last_block_reason;
  out_diag->p15_shadow_canary_requested_lookahead_depth = rt->p15_shadow_canary_state.requested_lookahead_depth;
  out_diag->p15_shadow_canary_healthy_margin_passed = rt->p15_shadow_canary_state.healthy_margin_passed;
  out_diag->p15_shadow_canary_reason_binding_passed = rt->p15_shadow_canary_state.reason_binding_passed;
  out_diag->p15_shadow_canary_evaluation_count = rt->p15_shadow_canary_state.evaluation_count;
  out_diag->p15_shadow_canary_action_allowed_count = rt->p15_shadow_canary_state.action_allowed_count;
  out_diag->p15_shadow_canary_action_applied_count = rt->p15_shadow_canary_state.action_applied_count;
  out_diag->p15_shadow_canary_action_blocked_count = rt->p15_shadow_canary_state.action_blocked_count;
  out_diag->p15_shadow_canary_reservation_attempt_count = rt->p15_shadow_canary_state.reservation_attempt_count;
  out_diag->p15_shadow_canary_reservation_success_count = rt->p15_shadow_canary_state.reservation_success_count;
  out_diag->p15_shadow_canary_reservation_rejected_count = rt->p15_shadow_canary_state.reservation_rejected_count;
  out_diag->p15_shadow_canary_block_low_confidence_count = rt->p15_shadow_canary_state.block_low_confidence_count;
  out_diag->p15_shadow_canary_block_high_miss_rate_count = rt->p15_shadow_canary_state.block_high_miss_rate_count;
  out_diag->p15_shadow_canary_block_high_arrival_error_count = rt->p15_shadow_canary_state.block_high_arrival_error_count;
  out_diag->p15_shadow_canary_block_recent_fallback_count = rt->p15_shadow_canary_state.block_recent_fallback_count;
  out_diag->p15_shadow_canary_block_recent_stale_count = rt->p15_shadow_canary_state.block_recent_stale_count;
  out_diag->p15_shadow_canary_block_insufficient_samples_count = rt->p15_shadow_canary_state.block_insufficient_samples_count;
  out_diag->p15_shadow_canary_block_disabled_count = rt->p15_shadow_canary_state.block_disabled_count;
  out_diag->p15_shadow_canary_block_no_future_lease_count = rt->p15_shadow_canary_state.block_no_future_lease_count;
  out_diag->p15_shadow_canary_block_reservation_failed_count = rt->p15_shadow_canary_state.block_reservation_failed_count;
  out_diag->p15_shadow_feedforward_valid = rt->p15_feedforward_dispatch_state.valid;
  out_diag->p15_shadow_feedforward_enabled = rt->p15_feedforward_dispatch_state.enabled;
  out_diag->p15_shadow_feedforward_used = rt->p15_feedforward_dispatch_state.used;
  out_diag->p15_shadow_feedforward_source = rt->p15_feedforward_dispatch_state.source;
  out_diag->p15_shadow_feedforward_block_reason = rt->p15_feedforward_dispatch_state.block_reason;
  out_diag->p15_shadow_feedforward_reserved_variant_id = rt->p15_feedforward_dispatch_state.reserved_variant_id;
  out_diag->p15_shadow_feedforward_fallback_to_judgment_count = rt->p15_feedforward_dispatch_state.fallback_to_judgment_count;
  out_diag->p15_shadow_feedforward_reservation_consumed_count = rt->p15_feedforward_dispatch_state.reservation_consumed_count;
  out_diag->p15_shadow_feedforward_no_matured_reservation_count = rt->p15_feedforward_dispatch_state.no_matured_reservation_count;
  out_diag->p15_shadow_feedforward_shape_mismatch_count = rt->p15_feedforward_dispatch_state.shape_mismatch_count;
  out_diag->p15_shadow_feedforward_variant_mismatch_count = rt->p15_feedforward_dispatch_state.variant_mismatch_count;
  out_diag->p15_shadow_feedforward_stale_reservation_count = rt->p15_feedforward_dispatch_state.stale_reservation_count;
  out_diag->p15_shadow_feedforward_reason_binding_block_count = rt->p15_feedforward_dispatch_state.reason_binding_block_count;
  out_diag->p15_shadow_feedforward_margin_block_count = rt->p15_feedforward_dispatch_state.margin_block_count;
  out_diag->p15_shadow_feedforward_dedup_block_count = rt->p15_feedforward_dispatch_state.dedup_block_count;
  out_diag->p13_m5_timestamp_valid_bits = rt->timestamp_valid_bits;
  out_diag->p13_m5_timestamp_period_ns = rt->timestamp_period_ns;
  if (prom_dom_slot_read_last_commit(&rt->blackboard, 0u, &slot_snapshot) != 0u && slot_snapshot.committed_event_count > 0u) {
    out_diag->p10_m4_last_slot_event_kind = (uint32_t)slot_snapshot.last_event.kind;
    out_diag->p10_m4_last_slot_event_slot_id = slot_snapshot.last_event.slot_id;
    out_diag->p10_m4_last_slot_event_reason = slot_snapshot.last_event.reason_code;
    out_diag->p10_m4_last_commit_dirty_slot_mask = slot_snapshot.last_commit_dirty_slot_mask;
  }
  if (prom_dom_slot_readiness_read_visible(&rt->blackboard, &slot_readiness_snapshot) != 0u) {
    out_diag->p10_m16_slot_readiness_boundary_generation = slot_readiness_snapshot.boundary_generation;
    out_diag->p10_m16_slot_readiness_dirty_slot_mask = slot_readiness_snapshot.dirty_slot_mask;
    out_diag->p10_m16_slot_readiness_ready_slot_mask = slot_readiness_snapshot.ready_slot_mask;
    out_diag->p10_m16_slot_readiness_failed_slot_mask = slot_readiness_snapshot.failed_slot_mask;
    out_diag->p10_m16_slot_readiness_invalidated_slot_mask = slot_readiness_snapshot.invalidated_slot_mask;
    out_diag->p10_m16_slot_readiness_attention_slot_mask = slot_readiness_snapshot.attention_slot_mask;
    out_diag->p10_m16_slot_readiness_overflow_spill_count = slot_readiness_snapshot.overflow_spill_count;
    out_diag->p10_m16_slot_readiness_duplicate_ready_event_count = slot_readiness_snapshot.duplicate_ready_event_count;
    out_diag->p10_m16_slot_readiness_empty_boundary_commit_count = slot_readiness_snapshot.empty_boundary_commit_count;
  }
  out_diag->p10_m13_m35_selector_cache_enabled = selector_cache_enabled(rt);
  out_diag->p10_m13_m35_selector_cache_valid = rt->m35_selector_cache.valid;
  out_diag->p10_m13_m35_selector_reuse_count = rt->m35_selector_cache.reuse_count;
  out_diag->p10_m13_m35_selector_recompute_count = rt->m35_selector_cache.recompute_count;
  out_diag->p10_m13_m35_selector_invalidation_count = rt->m35_selector_cache.invalidation_count;
  out_diag->p10_m13_m35_selector_last_dirty_dependency_mask = rt->m35_selector_cache.last_dirty_dependency_mask;
  out_diag->p10_m13_m35_selector_last_visible_generation = rt->m35_selector_cache.visible_generation_when_computed;
  out_diag->p10_m13_m35_selector_last_decision_reused = rt->m35_selector_cache.last_decision_reused;
  out_diag->p10_m13_transfer_selector_cache_enabled = selector_cache_enabled(rt);
  out_diag->p10_m13_transfer_selector_cache_valid = rt->transfer_selector_cache.valid;
  out_diag->p10_m13_transfer_selector_reuse_count = rt->transfer_selector_cache.reuse_count;
  out_diag->p10_m13_transfer_selector_recompute_count = rt->transfer_selector_cache.recompute_count;
  out_diag->p10_m13_transfer_selector_invalidation_count = rt->transfer_selector_cache.invalidation_count;
  out_diag->p10_m13_transfer_selector_last_dirty_dependency_mask = rt->transfer_selector_cache.last_dirty_dependency_mask;
  out_diag->p10_m13_transfer_selector_last_visible_generation = rt->transfer_selector_cache.visible_generation_when_computed;
  out_diag->p10_m13_transfer_selector_last_decision_reused = rt->transfer_selector_cache.last_decision_reused;
  out_diag->p10_m15_layout_precision_selector_cache_enabled = selector_cache_enabled(rt);
  out_diag->p10_m15_layout_precision_selector_cache_valid = rt->layout_precision_selector_cache.valid;
  out_diag->p10_m15_layout_precision_selector_reuse_count = rt->layout_precision_selector_cache.reuse_count;
  out_diag->p10_m15_layout_precision_selector_recompute_count = rt->layout_precision_selector_cache.recompute_count;
  out_diag->p10_m15_layout_precision_selector_invalidation_count = rt->layout_precision_selector_cache.invalidation_count;
  out_diag->p10_m15_layout_precision_selector_last_dirty_dependency_mask =
      rt->layout_precision_selector_cache.last_dirty_dependency_mask;
  out_diag->p10_m15_layout_precision_selector_last_visible_generation =
      rt->layout_precision_selector_cache.visible_generation_when_computed;
  out_diag->p10_m15_layout_precision_selector_last_decision_reused = rt->layout_precision_selector_cache.last_decision_reused;
  if (prom_dom_sgemm_read_visible_resource_lease_diagnostics(&rt->blackboard, &lease_snapshot) != 0u) {
    out_diag->p13_m10_lease_request_count = lease_snapshot.granted_count + lease_snapshot.denied_count;
    out_diag->p13_m10_lease_grant_count = lease_snapshot.granted_count;
    out_diag->p13_m10_lease_deny_count = lease_snapshot.denied_count;
    out_diag->p13_m10_lease_yield_count = lease_snapshot.yield_count;
    out_diag->p13_m10_lease_last_state = lease_snapshot.decision.lease_state;
    out_diag->p13_m10_lease_last_deny_reason = lease_snapshot.decision.deny_reason;
    out_diag->p13_m10_lookahead_requested = lease_snapshot.facts.lookahead_requested;
    out_diag->p13_m10_lookahead_allowed = lease_snapshot.decision.lookahead_allowed;
    out_diag->p13_m10_lookahead_blocked_reason = lease_snapshot.lookahead_blocked_reason;
    out_diag->p13_m10_selected_recipe_variant = lease_snapshot.decision.selected_recipe_variant;
  }
  out_diag->p11_m3_arena_a_capacity_bytes = rt->arenas[PROM_ARENA_ROLE_A].capacity_bytes;
  out_diag->p11_m3_arena_b_capacity_bytes = rt->arenas[PROM_ARENA_ROLE_B].capacity_bytes;
  out_diag->p11_m3_arena_c_capacity_bytes = rt->arenas[PROM_ARENA_ROLE_C].capacity_bytes;
  out_diag->p11_m3_arena_upload_capacity_bytes = rt->arenas[PROM_ARENA_ROLE_UPLOAD].capacity_bytes;
  out_diag->p11_m3_arena_a_required_bytes = rt->arenas[PROM_ARENA_ROLE_A].required_bytes;
  out_diag->p11_m3_arena_b_required_bytes = rt->arenas[PROM_ARENA_ROLE_B].required_bytes;
  out_diag->p11_m3_arena_c_required_bytes = rt->arenas[PROM_ARENA_ROLE_C].required_bytes;
  out_diag->p11_m3_arena_upload_required_bytes = rt->arenas[PROM_ARENA_ROLE_UPLOAD].required_bytes;
  out_diag->p11_m3_arena_a_generation = rt->arenas[PROM_ARENA_ROLE_A].generation;
  out_diag->p11_m3_arena_b_generation = rt->arenas[PROM_ARENA_ROLE_B].generation;
  out_diag->p11_m3_arena_c_generation = rt->arenas[PROM_ARENA_ROLE_C].generation;
  out_diag->p11_m3_arena_upload_generation = rt->arenas[PROM_ARENA_ROLE_UPLOAD].generation;
  out_diag->p11_m3_arena_a_reuse_count = rt->arenas[PROM_ARENA_ROLE_A].reuse_count;
  out_diag->p11_m3_arena_b_reuse_count = rt->arenas[PROM_ARENA_ROLE_B].reuse_count;
  out_diag->p11_m3_arena_c_reuse_count = rt->arenas[PROM_ARENA_ROLE_C].reuse_count;
  out_diag->p11_m3_arena_upload_reuse_count = rt->arenas[PROM_ARENA_ROLE_UPLOAD].reuse_count;
  out_diag->p11_m3_arena_a_grow_count = rt->arenas[PROM_ARENA_ROLE_A].grow_count;
  out_diag->p11_m3_arena_b_grow_count = rt->arenas[PROM_ARENA_ROLE_B].grow_count;
  out_diag->p11_m3_arena_c_grow_count = rt->arenas[PROM_ARENA_ROLE_C].grow_count;
  out_diag->p11_m3_arena_upload_grow_count = rt->arenas[PROM_ARENA_ROLE_UPLOAD].grow_count;
  out_diag->p11_m3_arena_a_shrink_count = rt->arenas[PROM_ARENA_ROLE_A].shrink_count;
  out_diag->p11_m3_arena_b_shrink_count = rt->arenas[PROM_ARENA_ROLE_B].shrink_count;
  out_diag->p11_m3_arena_c_shrink_count = rt->arenas[PROM_ARENA_ROLE_C].shrink_count;
  out_diag->p11_m3_arena_upload_shrink_count = rt->arenas[PROM_ARENA_ROLE_UPLOAD].shrink_count;
  out_diag->p11_m3_arena_a_rebuild_count = rt->arenas[PROM_ARENA_ROLE_A].rebuild_count;
  out_diag->p11_m3_arena_b_rebuild_count = rt->arenas[PROM_ARENA_ROLE_B].rebuild_count;
  out_diag->p11_m3_arena_c_rebuild_count = rt->arenas[PROM_ARENA_ROLE_C].rebuild_count;
  out_diag->p11_m3_arena_upload_rebuild_count = rt->arenas[PROM_ARENA_ROLE_UPLOAD].rebuild_count;
  out_diag->p11_m3_arena_grow_count = rt->arenas[PROM_ARENA_ROLE_A].grow_count +
                                       rt->arenas[PROM_ARENA_ROLE_B].grow_count +
                                       rt->arenas[PROM_ARENA_ROLE_C].grow_count +
                                       rt->arenas[PROM_ARENA_ROLE_UPLOAD].grow_count;
  out_diag->p11_m3_arena_shrink_count = rt->arenas[PROM_ARENA_ROLE_A].shrink_count +
                                         rt->arenas[PROM_ARENA_ROLE_B].shrink_count +
                                         rt->arenas[PROM_ARENA_ROLE_C].shrink_count +
                                         rt->arenas[PROM_ARENA_ROLE_UPLOAD].shrink_count;
  out_diag->p11_m3_arena_rebuild_count = rt->arenas[PROM_ARENA_ROLE_A].rebuild_count +
                                          rt->arenas[PROM_ARENA_ROLE_B].rebuild_count +
                                          rt->arenas[PROM_ARENA_ROLE_C].rebuild_count +
                                          rt->arenas[PROM_ARENA_ROLE_UPLOAD].rebuild_count;
  out_diag->p11_m3_arena_budget_rejection_count = rt->arenas[PROM_ARENA_ROLE_A].budget_rejection_count +
                                                   rt->arenas[PROM_ARENA_ROLE_B].budget_rejection_count +
                                                   rt->arenas[PROM_ARENA_ROLE_C].budget_rejection_count +
                                                   rt->arenas[PROM_ARENA_ROLE_UPLOAD].budget_rejection_count;
  out_diag->p11_m3_arena_ownership_rejection_count = rt->arenas[PROM_ARENA_ROLE_A].ownership_rejection_count +
                                                      rt->arenas[PROM_ARENA_ROLE_B].ownership_rejection_count +
                                                      rt->arenas[PROM_ARENA_ROLE_C].ownership_rejection_count +
                                                      rt->arenas[PROM_ARENA_ROLE_UPLOAD].ownership_rejection_count;
  out_diag->p11_m3_arena_namespace_rejection_count = rt->arenas[PROM_ARENA_ROLE_A].namespace_rejection_count +
                                                      rt->arenas[PROM_ARENA_ROLE_B].namespace_rejection_count +
                                                      rt->arenas[PROM_ARENA_ROLE_C].namespace_rejection_count +
                                                      rt->arenas[PROM_ARENA_ROLE_UPLOAD].namespace_rejection_count;
  out_diag->p11_m3_arena_total_committed_bytes = arena_total_committed_bytes(rt);
  out_diag->p11_m3_arena_projected_committed_bytes = rt->slot_diag.p11_m3_projected_committed_bytes;
  out_diag->p11_m3_arena_budget_limit_bytes = rt->arena_budget_limit_bytes;
  out_diag->p11_m3_arena_last_failure_reason = rt->arena_last_failure_detail;
  return PROM_OK;
}


int prom_reactor_runtime_sgemm_policy_diagnostics_sized_impl(void* handle,
                                                             PrometheusSgemmPolicyDiagnostics* out_diag,
                                                             uint32_t out_size) {
  PrometheusSgemmPolicyDiagnostics full_diag;
  size_t copy_size;
  if (out_diag == NULL || out_size == 0u) return PROM_ERROR;
  memset(out_diag, 0, (size_t)out_size);
  if (handle == NULL || !registry_contains(handle)) return PROM_INVALID_HANDLE;
  if (((prometheus_runtime*)handle)->magic != PROMETHEUS_RUNTIME_MAGIC) return PROM_INVALID_HANDLE;
  if (prom_reactor_runtime_sgemm_policy_diagnostics_fill(handle, &full_diag) != PROM_OK) return PROM_ERROR;
  copy_size = (size_t)out_size < sizeof(full_diag) ? (size_t)out_size : sizeof(full_diag);
  memcpy(out_diag, &full_diag, copy_size);
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_policy_diagnostics_impl(void* handle, PrometheusSgemmPolicyDiagnostics* out_diag) {
  return prom_reactor_runtime_sgemm_policy_diagnostics_sized_impl(handle, out_diag, (uint32_t)sizeof(PrometheusSgemmPolicyDiagnostics));
}

int prom_reactor_runtime_sgemm_batch_diagnostics_impl(void* handle, PrometheusSgemmBatchDiagnostics* out_diag) {
  prometheus_runtime* rt;
  if (out_diag == NULL) {
    return PROM_ERROR;
  }
  memset(out_diag, 0, sizeof(*out_diag));
  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    return PROM_INVALID_HANDLE;
  }
  *out_diag = rt->batch_diag;
  return PROM_OK;
}
