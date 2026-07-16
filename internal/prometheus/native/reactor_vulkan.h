#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_H

#include "reactor_api.h"
#include "reactor_batch.h"
#include "reactor_sgemm_dispatch_metadata.h"
#include <vulkan/vulkan.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef enum prom_vk_cooperative_matrix_state {
  PROM_VK_COOPERATIVE_MATRIX_UNAVAILABLE = 0u,
  PROM_VK_COOPERATIVE_MATRIX_EXTENSION_NO_USEFUL_TUPLE = 1u,
  PROM_VK_COOPERATIVE_MATRIX_USEFUL_TUPLE_AVAILABLE = 2u,
  PROM_VK_COOPERATIVE_MATRIX_COMPILER_ROUTE_UNAVAILABLE = 3u,
  PROM_VK_COOPERATIVE_MATRIX_DEVICE_FEATURE_ENABLED = 4u,
  PROM_VK_COOPERATIVE_MATRIX_EXECUTABLE = 5u,
} prom_vk_cooperative_matrix_state;

typedef struct prom_vk_buffer {
  VkBuffer buffer;
  VkDeviceMemory memory;
  void* mapped;
  VkDeviceSize size;
  uint32_t memory_type_index;
  VkMemoryPropertyFlags memory_property_flags;
  VkBufferUsageFlags usage_flags;
  VkSharingMode sharing_mode;
  VkDeviceSize memory_offset;
  VkDeviceSize memory_alignment;
} prom_vk_buffer;

typedef struct prom_vk_runtime_services {
  VkInstance instance;
  VkPhysicalDevice physical_device;
  VkDevice device;
  VkQueue compute_queue;
  uint32_t compute_queue_family_index;
  VkCommandPool compute_command_pool;
  uint32_t backend_available;
  uint32_t backend_reason_code;
  uint32_t test_flags;
  uint32_t reduction_test_flags;
  uint32_t reduction_ring_depth;
  uint32_t timestamp_query_supported;
  uint32_t timestamp_valid_bits;
  float timestamp_period_ns;
  uint32_t validation_enabled;
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
} prom_vk_runtime_services;

/* Test/audit-only request. This is never accepted by policy or the production
   shader registry; it supplies one temporary pipeline to the existing SGEMM
   host path. */
typedef struct prom_sgemm_audit_execution_descriptor {
  const uint32_t* spirv_words;
  size_t spirv_size_bytes;
  const char* entry_point;
  prom_sgemm_kernel_dispatch_metadata dispatch;
  uint32_t compute_mode;
  const char* provenance;
  uint64_t spirv_hash;
  uint32_t require_full_subgroups;
} prom_sgemm_audit_execution_descriptor;

typedef enum prom_sgemm_memory_placement {
  PROM_SGEMM_MEMORY_PLACEMENT_DEFAULT = 0u,
  PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL = 1u,
  PROM_SGEMM_MEMORY_PLACEMENT_HOST_VISIBLE_COHERENT_SYSTEM = 2u,
  PROM_SGEMM_MEMORY_PLACEMENT_HOST_VISIBLE_COHERENT_DEVICE_LOCAL = 3u,
} prom_sgemm_memory_placement;

typedef enum prom_sgemm_placement_reuse_mode {
  PROM_SGEMM_PLACEMENT_REUSE_COLD_ALLOCATION = 1u,
  PROM_SGEMM_PLACEMENT_REUSE_WARM = 2u,
  PROM_SGEMM_PLACEMENT_REUSE_REUPLOAD = 3u,
  PROM_SGEMM_PLACEMENT_REUSE_OUTPUT_TURNOVER = 4u,
  PROM_SGEMM_PLACEMENT_REUSE_PERSISTENT_B_REUPLOAD_A = 5u,
} prom_sgemm_placement_reuse_mode;

typedef struct prom_sgemm_placement_benchmark_options {
  uint32_t a_placement;
  uint32_t b_placement;
  uint32_t c_placement;
  uint32_t reuse_mode;
  uint32_t warmup;
  uint32_t iterations;
  uint32_t perturb_cache;
  uint64_t cache_perturbation_bytes;
} prom_sgemm_placement_benchmark_options;

typedef struct prom_sgemm_placement_benchmark_result {
  uint32_t stage;
  int detail_code;
  uint32_t supported;
  uint32_t completed_iterations;
  uint32_t correctness_readback_count;
  uint32_t allocation_count;
  uint32_t descriptor_update_count;
  uint32_t dispatch_count;
  uint32_t fallback_used;
  uint32_t a_memory_type_index;
  uint32_t b_memory_type_index;
  uint32_t c_memory_type_index;
  uint32_t a_memory_property_flags;
  uint32_t b_memory_property_flags;
  uint32_t c_memory_property_flags;
  uint32_t a_heap_index;
  uint32_t b_heap_index;
  uint32_t c_heap_index;
  uint64_t a_buffer_bytes;
  uint64_t b_buffer_bytes;
  uint64_t c_buffer_bytes;
  uint64_t initial_preparation_ns;
} prom_sgemm_placement_benchmark_result;

typedef struct prom_sgemm_memory_profile {
  uint32_t enabled;
  uint32_t kernel_compute_mode;
  uint32_t vendor_id;
  uint32_t device_id;
  uint32_t driver_version_min;
  uint32_t driver_version_max;
  uint32_t input_placement;
  uint32_t output_placement;
  uint32_t minimum_m;
  uint32_t minimum_n;
  uint32_t minimum_k;
  uint64_t maximum_total_bytes;
  uint64_t minimum_budget_headroom_bytes;
} prom_sgemm_memory_profile;

typedef struct prom_sgemm_memory_profile_facts {
  uint32_t experiment_enabled;
  uint32_t kernel_compute_mode;
  uint32_t vendor_id;
  uint32_t device_id;
  uint32_t driver_version;
  uint32_t mapped_device_local_type_exists;
  uint32_t m;
  uint32_t n;
  uint32_t k;
  uint64_t total_bytes;
  uint64_t heap_budget_bytes;
  uint64_t heap_usage_bytes;
} prom_sgemm_memory_profile_facts;

typedef struct prom_sgemm_memory_profile_decision {
  uint32_t matched;
  uint32_t input_placement;
  uint32_t output_placement;
  uint32_t fallback_placement;
  uint32_t reason;
} prom_sgemm_memory_profile_decision;

enum {
  PROM_SGEMM_MEMORY_PROFILE_REASON_MATCHED = 0u,
  PROM_SGEMM_MEMORY_PROFILE_REASON_DISABLED = 1u,
  PROM_SGEMM_MEMORY_PROFILE_REASON_KERNEL = 2u,
  PROM_SGEMM_MEMORY_PROFILE_REASON_DEVICE = 3u,
  PROM_SGEMM_MEMORY_PROFILE_REASON_DRIVER = 4u,
  PROM_SGEMM_MEMORY_PROFILE_REASON_MEMORY_TYPE = 5u,
  PROM_SGEMM_MEMORY_PROFILE_REASON_SHAPE = 6u,
  PROM_SGEMM_MEMORY_PROFILE_REASON_CAPACITY = 7u,
  PROM_SGEMM_MEMORY_PROFILE_REASON_BUDGET = 8u,
  PROM_SGEMM_MEMORY_PROFILE_REASON_ALLOCATION_FAILURE = 9u,
};

typedef struct prom_sgemm_audit_execution_result {
  uint32_t stage;
  int detail_code;
  prom_sgemm_dispatch_geometry dispatch_geometry;
  uint32_t gpu_timing_valid;
  uint64_t gpu_duration_ns;
  uint32_t pipeline_create_count;
  uint32_t warmup_dispatch_count;
  uint32_t measured_dispatch_count;
  uint32_t dispatches_per_sample;
  uint32_t timestamp_interval_command_mask;
  uint32_t query_reset_before_start_timestamp;
  uint32_t fence_wait_before_query_results;
  uint32_t selected_path;
  uint32_t compute_mode;
  uint32_t compute_queue_family_index;
  uint32_t push_constant_m;
  uint32_t push_constant_n;
  uint32_t push_constant_k;
  uint32_t a_memory_type_index;
  uint32_t b_memory_type_index;
  uint32_t c_memory_type_index;
  uint32_t a_memory_property_flags;
  uint32_t b_memory_property_flags;
  uint32_t c_memory_property_flags;
  uint32_t a_usage_flags;
  uint32_t b_usage_flags;
  uint32_t c_usage_flags;
  uint64_t a_buffer_bytes;
  uint64_t b_buffer_bytes;
  uint64_t c_buffer_bytes;
  uint64_t a_memory_alignment;
  uint64_t b_memory_alignment;
  uint64_t c_memory_alignment;
  uint64_t a_memory_offset;
  uint64_t b_memory_offset;
  uint64_t c_memory_offset;
} prom_sgemm_audit_execution_result;

/* M40b's internal, bounded reactor handoff.  This is intentionally Vulkan-
   facing and is not part of the public Oct ABI. */
typedef enum prom_device_element_type {
  PROM_DEVICE_ELEMENT_F16_PACKED_X2 = 1u,
  PROM_DEVICE_ELEMENT_F32 = 2u,
} prom_device_element_type;

typedef enum prom_device_buffer_layout {
  PROM_DEVICE_LAYOUT_ROW_MAJOR = 1u,
} prom_device_buffer_layout;

typedef enum prom_device_access_state {
  PROM_DEVICE_ACCESS_TRANSFER_WRITE = 1u,
  PROM_DEVICE_ACCESS_COMPUTE_READ = 2u,
  PROM_DEVICE_ACCESS_COMPUTE_WRITE = 3u,
  PROM_DEVICE_ACCESS_HOST_READ = 4u,
  PROM_DEVICE_ACCESS_TRANSFER_READ = 5u,
} prom_device_access_state;

typedef struct prom_device_buffer_view {
  VkBuffer buffer;
  VkDeviceSize offset;
  VkDeviceSize byte_length;
  uint32_t element_type;
  uint32_t logical_rows;
  uint32_t logical_columns;
  uint32_t row_stride_elements;
  uint32_t layout;
  uint32_t producer_access;
  uint32_t required_consumer_access;
  VkDevice owning_device;
  uint64_t owning_lifetime_id;
  uint32_t owning_slot_id;
  uint32_t owning_slot_generation;
} prom_device_buffer_view;

typedef struct prom_m40b_padding_plan {
  uint32_t logical_m;
  uint32_t logical_n;
  uint32_t logical_k;
  uint32_t padded_m;
  uint32_t padded_n;
  uint32_t padded_k;
  uint64_t packed_a_bytes;
  uint64_t packed_b_bytes;
  uint64_t intermediate_c_bytes;
  uint64_t logical_output_bytes;
  uint64_t replay_id;
} prom_m40b_padding_plan;

typedef enum prom_m40b_kernel {
  PROM_M40B_KERNEL_COOPERATIVE = 1u,
  PROM_M40B_KERNEL_A2X4 = 2u,
  PROM_M40B_KERNEL_CONVENTIONAL_FP16 = 3u,
} prom_m40b_kernel;

typedef enum prom_m40b_input_mode {
  PROM_M40B_INPUT_HOST_A_PERSISTENT_B = 1u,
  PROM_M40B_INPUT_DEVICE_A_PERSISTENT_B = 2u,
} prom_m40b_input_mode;

typedef enum prom_m40b_submit_plan {
  PROM_M40B_SUBMIT_ONE_COMMAND_BUFFER = 1u,
  PROM_M40B_SUBMIT_TWO_BOUNDED = 2u,
} prom_m40b_submit_plan;

typedef enum prom_m40b_trace_operation {
  PROM_M40B_TRACE_UPLOAD_A = 1u,
  PROM_M40B_TRACE_BIND_SGEMM_PIPELINE = 2u,
  PROM_M40B_TRACE_BIND_SGEMM_DESCRIPTORS = 3u,
  PROM_M40B_TRACE_PUSH_SGEMM_CONSTANTS = 4u,
  PROM_M40B_TRACE_TIMESTAMP_SGEMM_BEGIN = 5u,
  PROM_M40B_TRACE_DISPATCH_SGEMM = 6u,
  PROM_M40B_TRACE_TIMESTAMP_SGEMM_END = 7u,
  PROM_M40B_TRACE_EXPOSE_DEVICE_C = 8u,
  PROM_M40B_TRACE_COMPUTE_WRITE_TO_READ_BARRIER = 9u,
  PROM_M40B_TRACE_SUBMIT_DEPENDENCY = 10u,
  PROM_M40B_TRACE_TIMESTAMP_SOFTMAX_BEGIN = 11u,
  PROM_M40B_TRACE_BIND_SOFTMAX_PIPELINE = 12u,
  PROM_M40B_TRACE_BIND_SOFTMAX_DESCRIPTORS = 13u,
  PROM_M40B_TRACE_PUSH_SOFTMAX_CONSTANTS = 14u,
  PROM_M40B_TRACE_DISPATCH_SOFTMAX = 15u,
  PROM_M40B_TRACE_SOFTMAX_STAGE_BARRIER = 16u,
  PROM_M40B_TRACE_TIMESTAMP_SOFTMAX_END = 17u,
  PROM_M40B_TRACE_COMPUTE_WRITE_TO_TRANSFER_READ_BARRIER = 18u,
  PROM_M40B_TRACE_COPY_FINAL_READBACK = 19u,
  PROM_M40B_TRACE_TIMESTAMP_READBACK_END = 20u,
} prom_m40b_trace_operation;

#define PROM_M40B_MAX_COMMAND_TRACE_ENTRIES 40u

typedef struct prom_m40b_command_trace_entry {
  uint32_t operation;
  uint32_t submit_index;
  uint32_t reduction_stage_index;
  uint32_t source_stage_mask;
  uint32_t destination_stage_mask;
  uint32_t source_access_mask;
  uint32_t destination_access_mask;
  uint32_t source_queue_family;
  uint32_t destination_queue_family;
} prom_m40b_command_trace_entry;

typedef struct prom_m40b_command_trace {
  uint32_t entry_count;
  uint32_t submit_count;
  uint32_t intermediate_buffer_count;
  uint32_t intermediate_host_copy_count;
  uint32_t final_readback_copy_count;
  uint64_t replay_id;
  prom_m40b_command_trace_entry entries[PROM_M40B_MAX_COMMAND_TRACE_ENTRIES];
} prom_m40b_command_trace;

typedef struct prom_m40b_prepare_request {
  const float* values;
  uint32_t m;
  uint32_t n;
  uint32_t k;
  uint32_t kernel;
  uint64_t generation;
} prom_m40b_prepare_request;

typedef struct prom_m40b_prepare_result {
  uint32_t stage;
  int32_t detail_code;
  uint64_t generation;
  uint64_t conversion_ns;
  uint64_t upload_ns;
  uint64_t retained_bytes;
  uint32_t replaced;
  uint32_t buffer_reused;
  prom_m40b_padding_plan padding;
} prom_m40b_prepare_result;

typedef struct prom_m40b_execution_request {
  const float* host_a;
  float* output;
  uint32_t m;
  uint32_t n;
  uint32_t k;
  uint32_t kernel;
  uint32_t input_mode;
  uint32_t submit_plan;
  uint64_t required_b_generation;
  uint64_t required_a_generation;
} prom_m40b_execution_request;

typedef struct prom_m40b_execution_result {
  uint32_t stage;
  int32_t detail_code;
  uint64_t logical_request_id;
  uint32_t physical_slot_id;
  uint32_t physical_slot_generation;
  uint32_t physical_slot_recyclable;
  uint32_t validation_error_count_before;
  uint32_t validation_error_count_after;
  uint64_t a_conversion_ns;
  uint64_t a_upload_ns;
  uint64_t sgemm_gpu_ns;
  uint64_t handoff_gpu_ns;
  uint64_t softmax_gpu_ns;
  uint64_t combined_gpu_ns;
  uint64_t final_readback_ns;
  uint64_t cpu_submission_ns;
  uint64_t end_to_end_ns;
  uint64_t command_plan_replay_id;
  uint64_t reduction_replay_id;
  uint64_t cooperative_shader_hash;
  uint64_t persistent_b_generation;
  uint64_t resident_a_generation;
  uint64_t retained_bytes;
  uint64_t buffer_allocation_count;
  uint64_t buffer_reuse_count;
  uint64_t descriptor_update_count;
  uint64_t pipeline_create_count;
  uint64_t command_buffer_reuse_count;
  uint32_t reduction_stage_count;
  uint32_t submit_count;
  uint32_t correctness_readback_count;
  uint32_t no_intermediate_host_copy;
  prom_m40b_padding_plan padding;
  prom_device_buffer_view intermediate_c;
  prom_m40b_command_trace command_trace;
} prom_m40b_execution_result;

typedef enum prom_m40b_selector_reason {
  PROM_M40B_SELECTOR_ELIGIBLE = 0u,
  PROM_M40B_SELECTOR_DISABLED = 1u,
  PROM_M40B_SELECTOR_CAPABILITY = 2u,
  PROM_M40B_SELECTOR_TUPLE = 3u,
  PROM_M40B_SELECTOR_PRECISION = 4u,
  PROM_M40B_SELECTOR_SHAPE = 5u,
  PROM_M40B_SELECTOR_PADDING = 6u,
  PROM_M40B_SELECTOR_PERSISTENT_B = 7u,
  PROM_M40B_SELECTOR_RESIDENCY = 8u,
  PROM_M40B_SELECTOR_ROLLBACK = 9u,
} prom_m40b_selector_reason;

typedef struct prom_m40b_selector_facts {
  uint32_t experimental_enabled;
  uint32_t capability_state;
  uint32_t tuple_m;
  uint32_t tuple_n;
  uint32_t tuple_k;
  uint32_t shader_float16;
  uint32_t vulkan_memory_model;
  uint32_t precision_allows_f16_rounded;
  uint32_t m;
  uint32_t n;
  uint32_t k;
  uint32_t padding_supported;
  uint32_t persistent_b_available;
  uint32_t device_resident_composition;
  uint32_t rollback_active;
} prom_m40b_selector_facts;

typedef struct prom_m40b_selector_decision {
  uint32_t eligible;
  uint32_t selected;
  uint32_t reason;
  uint64_t replay_id;
} prom_m40b_selector_decision;

/* M42 is one bounded, one-head forward attention operator.  These types stay
   internal to the Vulkan reactor and expose no raw handles to Oct callers. */
#define PROM_M42_MAX_STAGES 16u
#define PROM_M42_MAX_BUFFERS 16u

typedef enum prom_m42_attention_path {
  PROM_M42_PATH_COOPERATIVE = 1u,
  PROM_M42_PATH_A2X4 = 2u,
  PROM_M42_PATH_CONVENTIONAL_FP16 = 3u,
} prom_m42_attention_path;

typedef enum prom_m42_precision_policy {
  PROM_M42_PRECISION_F16_ROUNDED = 1u,
  PROM_M42_PRECISION_FP32 = 2u,
} prom_m42_precision_policy;

typedef enum prom_m42_input_mode {
  PROM_M42_INPUT_HOST_X = 1u,
  PROM_M42_INPUT_RESIDENT_X = 2u,
} prom_m42_input_mode;

typedef enum prom_m42_k_layout_strategy {
  PROM_M42_K_LAYOUT_PACK_TRANSPOSE_F16 = 1u,
  PROM_M42_K_LAYOUT_TRANSPOSE_F32 = 2u,
} prom_m42_k_layout_strategy;

typedef enum prom_m42_probability_strategy {
  PROM_M42_PROBABILITY_PACK_F16 = 1u,
  PROM_M42_PROBABILITY_F32_DIRECT = 2u,
} prom_m42_probability_strategy;

typedef enum prom_m42_stage_operation {
  PROM_M42_STAGE_UPLOAD_X = 1u,
  PROM_M42_STAGE_PROJECT_Q = 2u,
  PROM_M42_STAGE_PROJECT_K = 3u,
  PROM_M42_STAGE_PROJECT_V = 4u,
  PROM_M42_STAGE_PACK_Q = 5u,
  PROM_M42_STAGE_LAYOUT_K = 6u,
  PROM_M42_STAGE_PACK_V = 7u,
  PROM_M42_STAGE_QK_TRANSPOSE = 8u,
  PROM_M42_STAGE_SCALE = 9u,
  PROM_M42_STAGE_SOFTMAX = 10u,
  PROM_M42_STAGE_PACK_P = 11u,
  PROM_M42_STAGE_PV = 12u,
  PROM_M42_STAGE_FINAL_READBACK = 13u,
} prom_m42_stage_operation;

typedef enum prom_m42_buffer_identity {
  PROM_M42_BUFFER_X = 1u,
  PROM_M42_BUFFER_Q = 2u,
  PROM_M42_BUFFER_K = 3u,
  PROM_M42_BUFFER_V = 4u,
  PROM_M42_BUFFER_Q_PACKED = 5u,
  PROM_M42_BUFFER_K_TRANSPOSED = 6u,
  PROM_M42_BUFFER_V_PACKED = 7u,
  PROM_M42_BUFFER_SCORES = 8u,
  PROM_M42_BUFFER_PROBABILITIES = 9u,
  PROM_M42_BUFFER_P_PACKED = 10u,
  PROM_M42_BUFFER_OUTPUT = 11u,
} prom_m42_buffer_identity;

typedef enum prom_m42_selector_reason {
  PROM_M42_SELECTOR_REQUESTED = 0u,
  PROM_M42_SELECTOR_CAPABILITY_FALLBACK = 1u,
  PROM_M42_SELECTOR_PRECISION_FALLBACK = 2u,
  PROM_M42_SELECTOR_ROLLBACK_FALLBACK = 3u,
  PROM_M42_SELECTOR_EXPLICIT_CONVENTIONAL = 4u,
  PROM_M42_SELECTOR_REJECTED = 5u,
} prom_m42_selector_reason;

typedef enum prom_m42_fault_point {
  PROM_M42_FAULT_NONE = 0u,
  PROM_M42_FAULT_AFTER_Q_PROJECTION = 1u,
  PROM_M42_FAULT_AFTER_QK = 2u,
  PROM_M42_FAULT_AFTER_SOFTMAX = 3u,
  PROM_M42_FAULT_AFTER_PV_SUBMIT = 4u,
} prom_m42_fault_point;

typedef struct prom_m42_stage_plan {
  uint32_t operation;
  uint32_t path;
  uint32_t input_buffer;
  uint32_t auxiliary_buffer;
  uint32_t output_buffer;
  uint32_t dispatch_count;
  uint32_t barrier_before;
  uint32_t barrier_after;
  uint32_t source_stage_mask;
  uint32_t destination_stage_mask;
  uint32_t source_access_mask;
  uint32_t destination_access_mask;
  uint32_t source_queue_family;
  uint32_t destination_queue_family;
} prom_m42_stage_plan;

typedef struct prom_m42_buffer_plan {
  uint32_t identity;
  uint32_t element_type;
  uint32_t logical_rows;
  uint32_t logical_columns;
  uint32_t row_stride_elements;
  uint32_t first_producer_stage;
  uint32_t last_consumer_stage;
  uint64_t retained_bytes;
} prom_m42_buffer_plan;

typedef struct prom_m42_plan_request {
  uint32_t tokens;
  uint32_t model_width;
  uint32_t head_dim;
  uint32_t value_dim;
  float scale;
  uint32_t scale_explicit;
  uint32_t precision_policy;
  uint32_t preferred_path;
  uint32_t allow_fallback;
  uint32_t input_mode;
  uint32_t cooperative_capability_state;
  uint32_t rollback_active;
  uint64_t wq_generation;
  uint64_t wk_generation;
  uint64_t wv_generation;
  uint64_t wq_hash;
  uint64_t wk_hash;
  uint64_t wv_hash;
} prom_m42_plan_request;

typedef struct prom_m42_attention_plan {
  uint32_t tokens;
  uint32_t model_width;
  uint32_t head_dim;
  uint32_t value_dim;
  uint32_t padded_tokens;
  uint32_t padded_model_width;
  uint32_t padded_head_dim;
  float scale;
  uint32_t precision_policy;
  uint32_t preferred_path;
  uint32_t selected_path;
  uint32_t fallback_used;
  uint32_t selector_reason;
  uint32_t k_layout_strategy;
  uint32_t probability_strategy;
  uint32_t reduction_stage_count;
  uint32_t stage_count;
  uint32_t buffer_count;
  uint32_t submit_count;
  uint32_t intermediate_host_copy_count;
  uint32_t final_readback_copy_count;
  uint64_t reduction_replay_id;
  uint64_t command_plan_replay_id;
  uint64_t replay_id;
  prom_m42_stage_plan stages[PROM_M42_MAX_STAGES];
  prom_m42_buffer_plan buffers[PROM_M42_MAX_BUFFERS];
} prom_m42_attention_plan;

typedef struct prom_m42_weight_prepare_request {
  const float* wq;
  const float* wk;
  const float* wv;
  uint32_t model_width;
  uint32_t head_dim;
  uint32_t value_dim;
  uint64_t generation;
} prom_m42_weight_prepare_request;

typedef struct prom_m42_weight_prepare_result {
  uint32_t stage;
  int32_t detail_code;
  uint64_t wq_generation;
  uint64_t wk_generation;
  uint64_t wv_generation;
  uint64_t wq_hash;
  uint64_t wk_hash;
  uint64_t wv_hash;
  uint64_t validation_hash_ns;
  uint64_t upload_and_pack_ns;
  uint64_t gpu_upload_and_pack_ns;
  uint64_t retained_bytes;
  uint32_t replaced;
  uint32_t buffer_reused;
} prom_m42_weight_prepare_result;

typedef struct prom_m42_resident_x_prepare_request {
  const float* x;
  uint32_t tokens;
  uint32_t model_width;
  uint64_t generation;
} prom_m42_resident_x_prepare_request;

typedef struct prom_m42_resident_x_prepare_result {
  uint32_t stage;
  int32_t detail_code;
  uint64_t generation;
  uint64_t hash;
  uint64_t validation_hash_ns;
  uint64_t upload_and_pack_ns;
  uint64_t gpu_upload_and_pack_ns;
  uint64_t retained_bytes;
  uint32_t replaced;
  uint32_t buffer_reused;
} prom_m42_resident_x_prepare_result;

typedef struct prom_m42_attention_request {
  const float* host_x;
  float* output;
  float* audit_q;
  float* audit_k;
  float* audit_v;
  float* audit_scores;
  float* audit_probabilities;
  uint32_t tokens;
  uint32_t model_width;
  uint32_t head_dim;
  uint32_t value_dim;
  float scale;
  uint32_t scale_explicit;
  uint32_t precision_policy;
  uint32_t preferred_path;
  uint32_t allow_fallback;
  uint32_t input_mode;
  uint32_t rollback_active;
  uint32_t fault_point;
  uint32_t audit_intermediates;
  uint64_t required_wq_generation;
  uint64_t required_wk_generation;
  uint64_t required_wv_generation;
  uint64_t required_x_generation;
} prom_m42_attention_request;

typedef struct prom_m42_attention_result {
  uint32_t stage;
  int32_t detail_code;
  uint64_t logical_request_id;
  uint32_t physical_slot_id;
  uint32_t physical_slot_generation;
  uint32_t physical_slot_recyclable;
  uint32_t selected_path;
  uint32_t fallback_used;
  uint32_t selector_reason;
  uint32_t qkv_gpu_producer_dispatch_count;
  uint32_t submit_count;
  uint32_t final_readback_count;
  uint32_t audit_readback_count;
  uint32_t no_intermediate_host_copy;
  uint32_t validation_error_count_before;
  uint32_t validation_error_count_after;
  uint64_t x_conversion_ns;
  uint64_t x_upload_ns;
  uint64_t q_projection_gpu_ns;
  uint64_t k_projection_gpu_ns;
  uint64_t v_projection_gpu_ns;
  uint64_t q_pack_gpu_ns;
  uint64_t k_layout_gpu_ns;
  uint64_t v_pack_gpu_ns;
  uint64_t qk_gpu_ns;
  uint64_t scale_gpu_ns;
  uint64_t softmax_gpu_ns;
  uint64_t p_pack_gpu_ns;
  uint64_t pv_gpu_ns;
  uint64_t total_attention_gpu_ns;
  uint64_t cpu_submission_ns;
  uint64_t final_readback_ns;
  uint64_t audit_readback_ns;
  uint64_t end_to_end_ns;
  uint64_t retained_bytes;
  uint64_t buffer_allocation_count;
  uint64_t buffer_reuse_count;
  uint64_t descriptor_update_count;
  uint64_t pipeline_create_count;
  uint64_t command_buffer_reuse_count;
  uint64_t wq_generation;
  uint64_t wk_generation;
  uint64_t wv_generation;
  uint64_t x_generation;
  prom_m42_attention_plan plan;
  prom_device_buffer_view output_view;
} prom_m42_attention_result;

typedef struct prom_m42_reference_request {
  const float* x;
  const float* wq;
  const float* wk;
  const float* wv;
  float* output;
  float* q;
  float* k;
  float* v;
  float* scores;
  float* probabilities;
  uint32_t tokens;
  uint32_t model_width;
  uint32_t head_dim;
  uint32_t value_dim;
  float scale;
  uint32_t scale_explicit;
  uint32_t precision_policy;
} prom_m42_reference_request;

typedef struct prom_m42_reference_result {
  uint32_t stage;
  int32_t detail_code;
  float resolved_scale;
  float minimum_probability_row_sum;
  float maximum_probability_row_sum;
  uint32_t all_finite;
} prom_m42_reference_result;

typedef struct prom_m42_mismatch {
  uint32_t matched;
  uint32_t stage;
  uint32_t row;
  uint32_t column;
  float expected;
  float actual;
  float absolute_error;
  float relative_error;
  uint32_t logical_rows;
  uint32_t logical_columns;
  uint32_t padded_rows;
  uint32_t padded_columns;
  uint64_t operator_replay_id;
  uint64_t reduction_replay_id;
} prom_m42_mismatch;

/* M43 is one fixed eight-head attention group.  It reuses the M42 stage and
   precision vocabulary, but owns one shared X identity and 24 independent
   persistent weight identities.  These are internal audit/runtime contracts,
   not a public graph or raw-handle API. */
#define PROM_M43_HEAD_COUNT 8u
#define PROM_M43_WEIGHT_KIND_COUNT 3u
#define PROM_M43_MAX_STAGES 104u

typedef enum prom_m43_weight_kind {
  PROM_M43_WEIGHT_Q = 0u,
  PROM_M43_WEIGHT_K = 1u,
  PROM_M43_WEIGHT_V = 2u,
} prom_m43_weight_kind;

typedef enum prom_m43_execution_strategy {
  PROM_M43_STRATEGY_COMPLETE_HEADS = 1u,
  PROM_M43_STRATEGY_PROJECTION_GROUPED = 2u,
  PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42 = 3u,
} prom_m43_execution_strategy;

typedef enum prom_m43_output_layout {
  PROM_M43_OUTPUT_HEAD_MAJOR = 1u,
} prom_m43_output_layout;

typedef enum prom_m43_eligibility_reason {
  PROM_M43_ELIGIBLE = 0u,
  PROM_M43_INELIGIBLE_HEAD_COUNT = 1u,
  PROM_M43_INELIGIBLE_CAPABILITY = 2u,
  PROM_M43_INELIGIBLE_PRECISION = 3u,
  PROM_M43_INELIGIBLE_SHAPE = 4u,
  PROM_M43_INELIGIBLE_PERSISTENT_WEIGHTS = 5u,
  PROM_M43_INELIGIBLE_SHARED_X = 6u,
  PROM_M43_INELIGIBLE_PADDING = 7u,
  PROM_M43_INELIGIBLE_CAPACITY = 8u,
  PROM_M43_INELIGIBLE_ROLLBACK = 9u,
} prom_m43_eligibility_reason;

typedef enum prom_m43_fault_point {
  PROM_M43_FAULT_NONE = 0u,
  PROM_M43_FAULT_SHARED_X_UPLOAD = 1u,
  PROM_M43_FAULT_MID_PROJECTIONS = 2u,
  PROM_M43_FAULT_HEAD_QK = 3u,
  PROM_M43_FAULT_HEAD_SOFTMAX = 4u,
  PROM_M43_FAULT_HEAD_PV_SUBMIT = 5u,
  PROM_M43_FAULT_FINAL_READBACK = 6u,
} prom_m43_fault_point;

typedef struct prom_m43_eligibility_facts {
  uint32_t head_count;
  uint32_t cooperative_capability_state;
  uint32_t precision_policy;
  uint32_t tokens;
  uint32_t model_width;
  uint32_t head_dim;
  uint32_t padding_supported;
  uint32_t persistent_weight_count;
  uint32_t shared_x_available;
  uint32_t generations_valid;
  uint32_t rollback_head_mask;
  uint64_t required_capacity_bytes;
  uint64_t available_capacity_bytes;
} prom_m43_eligibility_facts;

typedef struct prom_m43_eligibility_decision {
  uint32_t eligible;
  uint32_t reason;
  uint64_t replay_id;
} prom_m43_eligibility_decision;

typedef struct prom_m43_stage_plan {
  uint32_t sequence;
  uint32_t head_index;
  uint32_t operation;
  uint32_t selected_path;
  uint32_t dispatch_count;
  uint32_t barrier_call_count;
  uint32_t barrier_buffer_count;
  uint32_t copy_region_count;
  uint32_t descriptor_index;
  uint32_t timestamp_begin;
  uint32_t timestamp_end;
  uint32_t source_queue_family;
  uint32_t destination_queue_family;
} prom_m43_stage_plan;

typedef struct prom_m43_memory_plan {
  uint64_t shared_x_upload_bytes;
  uint64_t shared_x_f32_bytes;
  uint64_t shared_x_packed_bytes;
  uint64_t persistent_weight_upload_bytes;
  uint64_t persistent_weight_f32_bytes;
  uint64_t persistent_weight_packed_bytes;
  uint64_t qkv_f32_bytes;
  uint64_t qkv_packed_bytes;
  uint64_t score_bytes;
  uint64_t probability_bytes;
  uint64_t probability_packed_bytes;
  uint64_t output_bytes;
  uint64_t reduction_temporary_bytes;
  uint64_t grouped_readback_bytes;
  uint64_t exact_retained_bytes;
  uint64_t capacity_limit_bytes;
} prom_m43_memory_plan;

typedef struct prom_m43_plan_request {
  uint32_t head_count;
  uint32_t tokens;
  uint32_t model_width;
  uint32_t head_dim;
  float scale;
  uint32_t scale_explicit;
  uint32_t precision_policy;
  uint32_t allow_fallback;
  uint32_t input_mode;
  uint32_t execution_strategy;
  uint32_t cooperative_capability_state;
  uint32_t preferred_path[PROM_M43_HEAD_COUNT];
  uint32_t rollback_active[PROM_M43_HEAD_COUNT];
  uint64_t shared_x_generation;
  uint64_t shared_x_hash;
  uint64_t weight_generation[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
  uint64_t weight_hash[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
} prom_m43_plan_request;

typedef struct prom_m43_attention_plan {
  uint32_t head_count;
  uint32_t tokens;
  uint32_t model_width;
  uint32_t head_dim;
  uint32_t padded_tokens;
  uint32_t padded_model_width;
  uint32_t padded_head_dim;
  float scale;
  uint32_t precision_policy;
  uint32_t input_mode;
  uint32_t execution_strategy;
  uint32_t output_layout;
  uint32_t stage_count;
  uint32_t dispatch_count;
  uint32_t barrier_call_count;
  uint32_t barrier_buffer_count;
  uint32_t copy_region_count;
  uint32_t submit_count;
  uint32_t shared_x_conversion_count;
  uint32_t shared_x_upload_count;
  uint32_t shared_x_consumer_count;
  uint32_t persistent_weight_count;
  uint32_t qkv_projection_dispatch_count;
  uint32_t intermediate_host_copy_count;
  uint32_t final_readback_count;
  uint32_t selected_path[PROM_M43_HEAD_COUNT];
  uint32_t fallback_used[PROM_M43_HEAD_COUNT];
  uint32_t selector_reason[PROM_M43_HEAD_COUNT];
  uint64_t head_replay_id[PROM_M43_HEAD_COUNT];
  uint64_t shared_x_generation;
  uint64_t shared_x_hash;
  uint64_t weight_generation[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
  uint64_t weight_hash[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
  uint64_t command_plan_replay_id;
  uint64_t aggregate_replay_id;
  prom_m43_eligibility_decision eligibility;
  prom_m43_memory_plan memory;
  prom_m42_attention_plan head_plan[PROM_M43_HEAD_COUNT];
  prom_m43_stage_plan stages[PROM_M43_MAX_STAGES];
} prom_m43_attention_plan;

typedef struct prom_m43_weight_prepare_request {
  const float* values;
  uint64_t element_count;
  uint32_t head_index;
  uint32_t weight_kind;
  uint32_t model_width;
  uint32_t head_dim;
  uint64_t generation;
} prom_m43_weight_prepare_request;

typedef struct prom_m43_weight_prepare_result {
  uint32_t stage;
  int32_t detail_code;
  uint32_t head_index;
  uint32_t weight_kind;
  uint64_t generation;
  uint64_t hash;
  uint64_t validation_hash_ns;
  uint64_t upload_and_pack_ns;
  uint64_t gpu_upload_and_pack_ns;
  uint64_t retained_bytes;
  uint32_t replaced;
  uint32_t buffer_reused;
} prom_m43_weight_prepare_result;

typedef struct prom_m43_resident_x_prepare_request {
  const float* x;
  uint64_t element_count;
  uint32_t tokens;
  uint32_t model_width;
  uint64_t generation;
} prom_m43_resident_x_prepare_request;

typedef struct prom_m43_resident_x_prepare_result {
  uint32_t stage;
  int32_t detail_code;
  uint64_t generation;
  uint64_t hash;
  uint64_t validation_hash_ns;
  uint64_t upload_and_pack_ns;
  uint64_t gpu_upload_and_pack_ns;
  uint64_t retained_bytes;
  uint32_t replaced;
  uint32_t buffer_reused;
} prom_m43_resident_x_prepare_result;

typedef struct prom_m43_attention_group_request {
  const float* host_x;
  float* output;
  uint64_t host_x_element_count;
  uint64_t output_element_count;
  uint32_t head_count;
  uint32_t tokens;
  uint32_t model_width;
  uint32_t head_dim;
  float scale;
  uint32_t scale_explicit;
  uint32_t precision_policy;
  uint32_t allow_fallback;
  uint32_t input_mode;
  uint32_t execution_strategy;
  uint32_t preferred_path[PROM_M43_HEAD_COUNT];
  uint32_t rollback_active[PROM_M43_HEAD_COUNT];
  uint32_t fault_point;
  uint32_t fault_head;
  uint64_t shared_x_generation;
  uint64_t required_weight_generation[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
} prom_m43_attention_group_request;

typedef struct prom_m43_attention_group_result {
  uint32_t stage;
  int32_t detail_code;
  uint64_t logical_request_id;
  uint32_t physical_slot_id;
  uint32_t physical_slot_generation;
  uint32_t physical_slot_recyclable;
  uint32_t submit_count;
  uint32_t final_readback_count;
  uint32_t no_intermediate_host_copy;
  uint32_t shared_x_conversion_count;
  uint32_t shared_x_upload_count;
  uint32_t shared_x_consumer_count;
  uint32_t persistent_weight_count;
  uint32_t qkv_projection_dispatch_count;
  uint32_t validation_error_count_before;
  uint32_t validation_error_count_after;
  uint64_t shared_x_validation_ns;
  uint64_t shared_x_upload_gpu_ns;
  uint64_t shared_x_pack_gpu_ns;
  uint64_t q_projection_gpu_ns[PROM_M43_HEAD_COUNT];
  uint64_t k_projection_gpu_ns[PROM_M43_HEAD_COUNT];
  uint64_t v_projection_gpu_ns[PROM_M43_HEAD_COUNT];
  uint64_t q_pack_gpu_ns[PROM_M43_HEAD_COUNT];
  uint64_t k_layout_gpu_ns[PROM_M43_HEAD_COUNT];
  uint64_t v_pack_gpu_ns[PROM_M43_HEAD_COUNT];
  uint64_t qk_gpu_ns[PROM_M43_HEAD_COUNT];
  uint64_t scale_gpu_ns[PROM_M43_HEAD_COUNT];
  uint64_t softmax_gpu_ns[PROM_M43_HEAD_COUNT];
  uint64_t p_pack_gpu_ns[PROM_M43_HEAD_COUNT];
  uint64_t pv_gpu_ns[PROM_M43_HEAD_COUNT];
  uint64_t q_projection_total_gpu_ns;
  uint64_t k_projection_total_gpu_ns;
  uint64_t v_projection_total_gpu_ns;
  uint64_t q_pack_total_gpu_ns;
  uint64_t k_layout_total_gpu_ns;
  uint64_t v_pack_total_gpu_ns;
  uint64_t qk_total_gpu_ns;
  uint64_t scale_total_gpu_ns;
  uint64_t softmax_total_gpu_ns;
  uint64_t p_pack_total_gpu_ns;
  uint64_t pv_total_gpu_ns;
  uint64_t projection_total_gpu_ns;
  uint64_t post_projection_total_gpu_ns;
  uint64_t grouped_attention_gpu_ns;
  uint64_t cpu_recording_ns;
  uint64_t cpu_submission_ns;
  uint64_t final_readback_ns;
  uint64_t end_to_end_ns;
  uint64_t exact_request_bytes;
  uint64_t retained_bytes;
  uint64_t buffer_allocation_count;
  uint64_t buffer_reuse_count;
  uint64_t descriptor_update_count;
  uint64_t pipeline_create_count;
  uint64_t command_buffer_reuse_count;
  uint64_t shared_x_generation;
  uint64_t weight_generation[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
  prom_m43_attention_plan plan;
  prom_device_buffer_view head_output_view[PROM_M43_HEAD_COUNT];
} prom_m43_attention_group_result;

typedef struct prom_m43_reference_request {
  const float* x;
  const float* weight[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
  float* output;
  uint64_t x_element_count;
  uint64_t weight_element_count;
  uint64_t output_element_count;
  uint32_t head_count;
  uint32_t tokens;
  uint32_t model_width;
  uint32_t head_dim;
  float scale;
  uint32_t scale_explicit;
  uint32_t precision_policy;
} prom_m43_reference_request;

typedef struct prom_m43_reference_result {
  uint32_t stage;
  int32_t detail_code;
  uint32_t head_index;
  float minimum_probability_row_sum;
  float maximum_probability_row_sum;
  uint32_t all_finite;
} prom_m43_reference_result;

typedef struct prom_m43_mismatch {
  uint32_t matched;
  uint32_t head_index;
  prom_m42_mismatch stage_mismatch;
  uint64_t weight_generation[PROM_M43_WEIGHT_KIND_COUNT];
  uint64_t head_replay_id;
  uint64_t aggregate_replay_id;
} prom_m43_mismatch;

/* M44 is one bounded consumer of the fixed M43 head aggregate. It makes the
   token-major concatenation boundary explicit and owns one persistent output
   projection weight. These Vulkan-facing contracts remain internal. */
#define PROM_M44_HEAD_COUNT PROM_M43_HEAD_COUNT
#define PROM_M44_MAX_STAGES 6u

typedef enum prom_m44_aggregation_strategy {
  PROM_M44_AGGREGATION_INTERLEAVE = 1u,
  PROM_M44_AGGREGATION_DIRECT_SEGMENTED = 2u,
} prom_m44_aggregation_strategy;

typedef enum prom_m44_projection_path {
  PROM_M44_PROJECTION_COOPERATIVE = PROM_M42_PATH_COOPERATIVE,
  PROM_M44_PROJECTION_A2X4_FP32 = PROM_M42_PATH_A2X4,
  PROM_M44_PROJECTION_CONVENTIONAL_FP16 = PROM_M42_PATH_CONVENTIONAL_FP16,
  PROM_M44_PROJECTION_DIRECT_SEGMENTED_FP16 = 4u,
} prom_m44_projection_path;

typedef enum prom_m44_submit_plan {
  PROM_M44_SUBMIT_ONE_COMMAND_BUFFER = 1u,
  PROM_M44_SUBMIT_TWO_BOUNDED = 2u,
} prom_m44_submit_plan;

typedef enum prom_m44_stage_operation {
  PROM_M44_STAGE_HEADS_READY = 1u,
  PROM_M44_STAGE_INTERLEAVE = 2u,
  PROM_M44_STAGE_DIRECT_PROJECTION = 3u,
  PROM_M44_STAGE_OUTPUT_PROJECTION = 4u,
  PROM_M44_STAGE_FINAL_READBACK = 5u,
} prom_m44_stage_operation;

typedef enum prom_m44_fault_point {
  PROM_M44_FAULT_NONE = 0u,
  PROM_M44_FAULT_BEFORE_AGGREGATION = 1u,
  PROM_M44_FAULT_DURING_INTERLEAVE = 2u,
  PROM_M44_FAULT_AFTER_INTERLEAVE = 3u,
  PROM_M44_FAULT_MID_DIRECT_PROJECTION = 4u,
  PROM_M44_FAULT_AFTER_PROJECTION_SUBMIT = 5u,
  PROM_M44_FAULT_BEFORE_FINAL_READBACK = 6u,
  PROM_M44_FAULT_UNCERTAIN_COMPLETION = 7u,
} prom_m44_fault_point;

typedef enum prom_m44_eligibility_reason {
  PROM_M44_ELIGIBLE = 0u,
  PROM_M44_INELIGIBLE_HEAD_COUNT = 1u,
  PROM_M44_INELIGIBLE_VIEW = 2u,
  PROM_M44_INELIGIBLE_VIEW_SHAPE = 3u,
  PROM_M44_INELIGIBLE_VIEW_GENERATION = 4u,
  PROM_M44_INELIGIBLE_VIEW_OVERLAP = 5u,
  PROM_M44_INELIGIBLE_WO = 6u,
  PROM_M44_INELIGIBLE_SHAPE = 7u,
  PROM_M44_INELIGIBLE_PRECISION = 8u,
  PROM_M44_INELIGIBLE_CAPABILITY = 9u,
  PROM_M44_INELIGIBLE_PADDING = 10u,
  PROM_M44_INELIGIBLE_CAPACITY = 11u,
  PROM_M44_INELIGIBLE_STRATEGY = 12u,
  PROM_M44_INELIGIBLE_ROLLBACK = 13u,
} prom_m44_eligibility_reason;

typedef struct prom_m44_eligibility_facts {
  uint32_t head_count;
  uint32_t views_valid;
  uint32_t shapes_match;
  uint32_t generations_valid;
  uint32_t non_overlapping;
  uint32_t wo_valid;
  uint32_t shape_valid;
  uint32_t precision_valid;
  uint32_t cooperative_capability_state;
  uint32_t padding_supported;
  uint32_t strategy_supported;
  uint32_t rollback_active;
  uint64_t required_capacity_bytes;
  uint64_t available_capacity_bytes;
} prom_m44_eligibility_facts;

typedef struct prom_m44_eligibility_decision {
  uint32_t eligible;
  uint32_t reason;
  uint64_t replay_id;
} prom_m44_eligibility_decision;

typedef struct prom_m44_stage_plan {
  uint32_t sequence;
  uint32_t operation;
  uint32_t dispatch_count;
  uint32_t barrier_call_count;
  uint32_t barrier_buffer_count;
  uint32_t copy_region_count;
  uint32_t timestamp_begin;
  uint32_t timestamp_end;
  uint32_t source_stage_mask;
  uint32_t destination_stage_mask;
  uint32_t source_access_mask;
  uint32_t destination_access_mask;
  uint32_t source_queue_family;
  uint32_t destination_queue_family;
} prom_m44_stage_plan;

typedef struct prom_m44_memory_plan {
  uint64_t source_head_bytes;
  uint64_t contiguous_f32_bytes;
  uint64_t contiguous_packed_bytes;
  uint64_t partial_output_bytes;
  uint64_t accumulation_bytes;
  uint64_t wo_upload_bytes;
  uint64_t wo_f32_bytes;
  uint64_t wo_packed_bytes;
  uint64_t final_y_bytes;
  uint64_t final_readback_bytes;
  uint64_t exact_request_bytes;
  uint64_t capacity_limit_bytes;
  uint32_t reusable_descriptor_set_count;
  uint32_t descriptor_binding_count;
} prom_m44_memory_plan;

typedef struct prom_m44_plan_request {
  prom_device_buffer_view head_views[PROM_M44_HEAD_COUNT];
  uint32_t head_count;
  uint32_t tokens;
  uint32_t head_dim;
  uint32_t model_width;
  uint32_t precision_policy;
  uint32_t aggregation_strategy;
  uint32_t projection_path;
  uint32_t submit_plan;
  uint32_t cooperative_capability_state;
  uint32_t rollback_active;
  uint64_t wo_generation;
  uint64_t wo_hash;
  uint64_t m43_aggregate_replay_id;
} prom_m44_plan_request;

typedef struct prom_m44_output_projection_plan {
  uint32_t head_count;
  uint32_t tokens;
  uint32_t head_dim;
  uint32_t concatenated_width;
  uint32_t model_width;
  uint32_t padded_tokens;
  uint32_t padded_concatenated_width;
  uint32_t padded_model_width;
  uint32_t head_row_stride;
  uint32_t output_row_stride;
  uint32_t precision_policy;
  uint32_t aggregation_strategy;
  uint32_t projection_path;
  uint32_t submit_plan;
  uint32_t stage_count;
  uint32_t dispatch_count;
  uint32_t barrier_call_count;
  uint32_t barrier_buffer_count;
  uint32_t copy_region_count;
  uint32_t submit_count;
  uint32_t intermediate_host_copy_count;
  uint32_t final_readback_count;
  uint32_t output_element_type;
  uint64_t wo_generation;
  uint64_t wo_hash;
  uint64_t m43_aggregate_replay_id;
  uint64_t command_plan_replay_id;
  uint64_t replay_id;
  prom_m44_eligibility_decision eligibility;
  prom_m44_memory_plan memory;
  prom_m44_stage_plan stages[PROM_M44_MAX_STAGES];
} prom_m44_output_projection_plan;

typedef struct prom_m44_wo_prepare_request {
  const float* values;
  uint64_t element_count;
  uint32_t head_count;
  uint32_t head_dim;
  uint32_t model_width;
  uint64_t generation;
} prom_m44_wo_prepare_request;

typedef struct prom_m44_wo_prepare_result {
  uint32_t stage;
  int32_t detail_code;
  uint64_t generation;
  uint64_t hash;
  uint64_t validation_hash_ns;
  uint64_t upload_and_pack_ns;
  uint64_t gpu_upload_and_pack_ns;
  uint64_t retained_bytes;
  uint32_t replaced;
  uint32_t buffer_reused;
} prom_m44_wo_prepare_result;

typedef struct prom_m44_composed_request {
  prom_m43_attention_group_request attention;
  float* output;
  uint64_t output_element_count;
  uint32_t aggregation_strategy;
  uint32_t projection_path;
  uint32_t submit_plan;
  uint32_t rollback_active;
  uint32_t fault_point;
  uint64_t required_wo_generation;
} prom_m44_composed_request;

typedef struct prom_m44_composed_result {
  uint32_t stage;
  int32_t detail_code;
  uint64_t logical_request_id;
  uint32_t physical_slot_id;
  uint32_t physical_slot_generation;
  uint32_t physical_slot_recyclable;
  uint32_t submit_count;
  uint32_t final_readback_count;
  uint32_t no_intermediate_host_copy;
  uint32_t validation_error_count_before;
  uint32_t validation_error_count_after;
  uint64_t aggregation_gpu_ns;
  uint64_t projection_gpu_ns;
  uint64_t accumulation_gpu_ns;
  uint64_t m44_gpu_ns;
  uint64_t total_m43_m44_gpu_ns;
  uint64_t cpu_recording_ns;
  uint64_t cpu_submission_ns;
  uint64_t final_readback_ns;
  uint64_t end_to_end_ns;
  uint64_t exact_request_bytes;
  uint64_t retained_bytes;
  uint64_t buffer_allocation_count;
  uint64_t buffer_reuse_count;
  uint64_t descriptor_update_count;
  uint64_t pipeline_create_count;
  uint64_t command_buffer_reuse_count;
  uint64_t wo_generation;
  prom_m43_attention_group_result attention;
  prom_m44_output_projection_plan plan;
  prom_device_buffer_view output_view;
} prom_m44_composed_result;

typedef struct prom_m44_host_bounce_request {
  const float* head_major;
  uint64_t head_major_element_count;
  float* output;
  uint64_t output_element_count;
  uint32_t head_count;
  uint32_t tokens;
  uint32_t head_dim;
  uint32_t model_width;
  uint32_t precision_policy;
  uint32_t projection_path;
  uint64_t required_wo_generation;
  uint64_t m43_aggregate_replay_id;
} prom_m44_host_bounce_request;

typedef struct prom_m44_host_bounce_result {
  uint32_t stage;
  int32_t detail_code;
  uint64_t logical_request_id;
  uint32_t physical_slot_id;
  uint32_t physical_slot_generation;
  uint32_t physical_slot_recyclable;
  uint64_t cpu_concatenate_ns;
  uint64_t cpu_pack_ns;
  uint64_t upload_gpu_ns;
  uint64_t projection_gpu_ns;
  uint64_t final_readback_ns;
  uint64_t end_to_end_ns;
  uint64_t retained_bytes;
  uint32_t submit_count;
  uint32_t final_readback_count;
  uint32_t intermediate_host_copy_count;
  uint64_t replay_id;
} prom_m44_host_bounce_result;

typedef struct prom_m44_reference_request {
  const float* head_major;
  const float* wo;
  float* concatenated;
  float* output;
  uint64_t head_major_element_count;
  uint64_t wo_element_count;
  uint64_t output_element_count;
  uint32_t head_count;
  uint32_t tokens;
  uint32_t head_dim;
  uint32_t model_width;
  uint32_t precision_policy;
} prom_m44_reference_request;

typedef struct prom_m44_reference_result {
  uint32_t stage;
  int32_t detail_code;
  uint32_t all_finite;
} prom_m44_reference_result;

typedef struct prom_m44_mismatch {
  uint32_t matched;
  uint32_t strategy;
  uint32_t token;
  uint32_t output_column;
  uint32_t source_head;
  uint32_t source_column;
  float expected;
  float actual;
  float absolute_error;
  float relative_error;
  uint64_t wo_generation;
  uint64_t m43_aggregate_replay_id;
  uint64_t m44_replay_id;
} prom_m44_mismatch;

/* M45 is one bounded ownership transition from the immutable resident X and
   the slot-owned M44 Y to one retained FP32 Z view.  It remains internal to
   the Vulkan reactor and deliberately does not define a graph abstraction. */
#define PROM_M45_MAX_STAGES 4u
#define PROM_M45_MAX_BARRIERS 4u

typedef enum prom_m45_residual_strategy {
  PROM_M45_STRATEGY_SEPARATE_OUTPUT = 1u,
  PROM_M45_STRATEGY_IN_PLACE_Y = 2u,
  PROM_M45_STRATEGY_IN_PLACE_X_AUDIT = 3u,
} prom_m45_residual_strategy;

typedef enum prom_m45_submit_policy {
  PROM_M45_SUBMIT_ONE_COMMAND_BUFFER = 1u,
  PROM_M45_SUBMIT_TWO_BOUNDED = 2u,
} prom_m45_submit_policy;

typedef enum prom_m45_precision_policy {
  PROM_M45_PRECISION_FP32 = 1u,
} prom_m45_precision_policy;

typedef enum prom_m45_buffer_identity {
  PROM_M45_BUFFER_X = 1u,
  PROM_M45_BUFFER_Y = 2u,
  PROM_M45_BUFFER_Z = 3u,
  PROM_M45_BUFFER_READBACK = 4u,
} prom_m45_buffer_identity;

typedef enum prom_m45_stage_operation {
  PROM_M45_STAGE_X_READY = 1u,
  PROM_M45_STAGE_Y_READY = 2u,
  PROM_M45_STAGE_RESIDUAL_ADD = 3u,
  PROM_M45_STAGE_FINAL_READBACK = 4u,
} prom_m45_stage_operation;

typedef enum prom_m45_fault_point {
  PROM_M45_FAULT_NONE = 0u,
  PROM_M45_FAULT_BEFORE_RESIDUAL_BARRIERS = 1u,
  PROM_M45_FAULT_AFTER_X_BARRIER = 2u,
  PROM_M45_FAULT_AFTER_Y_BARRIER = 3u,
  PROM_M45_FAULT_DURING_RESIDUAL_DISPATCH = 4u,
  PROM_M45_FAULT_AFTER_RESIDUAL_SUBMISSION = 5u,
  PROM_M45_FAULT_BEFORE_FINAL_READBACK = 6u,
  PROM_M45_FAULT_UNCERTAIN_COMPLETION = 7u,
} prom_m45_fault_point;

typedef enum prom_m45_eligibility_reason {
  PROM_M45_ELIGIBLE = 0u,
  PROM_M45_INELIGIBLE_VIEW = 1u,
  PROM_M45_INELIGIBLE_SHAPE = 2u,
  PROM_M45_INELIGIBLE_STRIDE = 3u,
  PROM_M45_INELIGIBLE_GENERATION = 4u,
  PROM_M45_INELIGIBLE_DEVICE = 5u,
  PROM_M45_INELIGIBLE_ALIAS = 6u,
  PROM_M45_INELIGIBLE_EXCLUSIVITY = 7u,
  PROM_M45_INELIGIBLE_PRECISION = 8u,
  PROM_M45_INELIGIBLE_CAPACITY = 9u,
  PROM_M45_INELIGIBLE_STRATEGY = 10u,
  PROM_M45_INELIGIBLE_IN_PLACE_X = 11u,
} prom_m45_eligibility_reason;

typedef struct prom_m45_barrier_trace {
  uint32_t sequence;
  uint32_t buffer_identity;
  uint64_t byte_offset;
  uint64_t byte_length;
  uint32_t source_stage_mask;
  uint32_t destination_stage_mask;
  uint32_t source_access_mask;
  uint32_t destination_access_mask;
  uint32_t source_queue_family;
  uint32_t destination_queue_family;
} prom_m45_barrier_trace;

typedef struct prom_m45_stage_plan {
  uint32_t sequence;
  uint32_t operation;
  uint32_t dispatch_count;
  uint32_t barrier_begin;
  uint32_t barrier_count;
  uint32_t copy_region_count;
  uint32_t timestamp_begin;
  uint32_t timestamp_end;
} prom_m45_stage_plan;

typedef struct prom_m45_memory_plan {
  uint64_t x_view_bytes;
  uint64_t y_view_bytes;
  uint64_t z_device_bytes;
  uint64_t z_readback_bytes;
  uint64_t exact_request_bytes;
  uint64_t in_place_y_saved_bytes;
  uint64_t capacity_limit_bytes;
  uint32_t reusable_descriptor_set_count;
  uint32_t descriptor_binding_count;
} prom_m45_memory_plan;

typedef struct prom_m45_eligibility_decision {
  uint32_t eligible;
  uint32_t reason;
  uint64_t replay_id;
} prom_m45_eligibility_decision;

typedef struct prom_m45_plan_request {
  prom_device_buffer_view x_view;
  prom_device_buffer_view y_view;
  uint32_t tokens;
  uint32_t model_width;
  uint32_t strategy;
  uint32_t submit_policy;
  uint32_t precision_policy;
  uint32_t y_exclusive;
  uint32_t pre_residual_y_consumer_count;
  uint32_t final_readback;
  uint64_t expected_x_generation;
  uint64_t expected_y_generation;
  uint64_t m44_replay_id;
} prom_m45_plan_request;

typedef struct prom_m45_residual_plan {
  uint32_t tokens;
  uint32_t model_width;
  uint32_t x_row_stride;
  uint32_t y_row_stride;
  uint32_t z_row_stride;
  uint32_t strategy;
  uint32_t submit_policy;
  uint32_t precision_policy;
  uint32_t physical_alias_plan;
  uint32_t stage_count;
  uint32_t barrier_count;
  uint32_t dispatch_count;
  uint32_t copy_region_count;
  uint32_t submit_count;
  uint32_t intermediate_host_copy_count;
  uint32_t final_readback_count;
  uint64_t x_generation;
  uint64_t y_generation;
  uint64_t z_generation;
  uint64_t shader_hash;
  uint64_t m44_replay_id;
  uint64_t command_plan_replay_id;
  uint64_t replay_id;
  prom_m45_eligibility_decision eligibility;
  prom_m45_memory_plan memory;
  prom_m45_stage_plan stages[PROM_M45_MAX_STAGES];
  prom_m45_barrier_trace barriers[PROM_M45_MAX_BARRIERS];
} prom_m45_residual_plan;

typedef struct prom_m45_composed_request {
  prom_m43_attention_group_request attention;
  float* output;
  uint64_t output_element_count;
  uint32_t aggregation_strategy;
  uint32_t projection_path;
  uint32_t residual_strategy;
  uint32_t submit_policy;
  uint32_t rollback_active;
  uint32_t fault_point;
  uint64_t required_wo_generation;
} prom_m45_composed_request;

typedef struct prom_m45_composed_result {
  uint32_t stage;
  int32_t detail_code;
  uint64_t logical_request_id;
  uint32_t physical_slot_id;
  uint32_t physical_slot_generation;
  uint32_t physical_slot_recyclable;
  uint32_t submit_count;
  uint32_t final_readback_count;
  uint32_t no_intermediate_host_copy;
  uint32_t validation_error_count_before;
  uint32_t validation_error_count_after;
  uint64_t aggregation_gpu_ns;
  uint64_t projection_gpu_ns;
  uint64_t m44_gpu_ns;
  uint64_t residual_gpu_ns;
  uint64_t total_m43_m44_m45_gpu_ns;
  uint64_t cpu_recording_ns;
  uint64_t cpu_submission_ns;
  uint64_t final_readback_ns;
  uint64_t end_to_end_ns;
  uint64_t exact_request_bytes;
  uint64_t retained_bytes;
  uint64_t buffer_allocation_count;
  uint64_t buffer_reuse_count;
  uint64_t descriptor_update_count;
  uint64_t pipeline_create_count;
  uint64_t command_buffer_reuse_count;
  uint64_t x_generation;
  uint64_t y_generation;
  uint64_t z_generation;
  prom_m43_attention_group_result attention;
  prom_m44_output_projection_plan projection_plan;
  prom_m45_residual_plan residual_plan;
  prom_device_buffer_view x_view;
  prom_device_buffer_view y_view;
  prom_device_buffer_view z_view;
} prom_m45_composed_result;

typedef struct prom_m45_reference_request {
  const float* x;
  const float* y;
  float* z;
  uint64_t x_element_count;
  uint64_t y_element_count;
  uint64_t z_element_count;
  uint32_t tokens;
  uint32_t model_width;
  uint32_t x_row_stride;
  uint32_t y_row_stride;
  uint32_t z_row_stride;
} prom_m45_reference_request;

typedef struct prom_m45_mismatch {
  uint32_t matched;
  uint32_t strategy;
  uint32_t token;
  uint32_t column;
  float expected;
  float actual;
  float absolute_error;
  float relative_error;
  uint64_t x_generation;
  uint64_t y_generation;
  uint64_t z_generation;
  uint64_t m44_replay_id;
  uint64_t m45_replay_id;
} prom_m45_mismatch;

typedef struct prom_m45_resident_x_readback_request {
  float* output;
  uint64_t output_element_count;
  uint32_t tokens;
  uint32_t model_width;
  uint64_t expected_x_generation;
} prom_m45_resident_x_readback_request;

typedef struct prom_m45_resident_x_readback_result {
  uint32_t stage;
  int32_t detail_code;
  uint64_t gpu_readback_ns;
  uint64_t end_to_end_ns;
  uint64_t x_generation;
  uint32_t physical_slot_id;
  uint32_t physical_slot_generation;
  uint32_t physical_slot_recyclable;
} prom_m45_resident_x_readback_result;

enum {
  PROM_M40B_DETAIL_INVALID_REQUEST = -6901,
  PROM_M40B_DETAIL_SIZE_OVERFLOW = -6902,
  PROM_M40B_DETAIL_INVALID_VIEW = -6903,
  PROM_M40B_DETAIL_CROSS_DEVICE = -6904,
  PROM_M40B_DETAIL_STALE_GENERATION = -6905,
  PROM_M40B_DETAIL_CAPABILITY = -6906,
  PROM_M40B_DETAIL_RESOURCE = -6907,
  PROM_M40B_DETAIL_COMMAND = -6908,
  PROM_M40B_DETAIL_SUBMIT = -6909,
  PROM_M40B_DETAIL_COMPLETION_UNCERTAIN = -6910,
  PROM_M40B_DETAIL_QUERY = -6911,
  PROM_M40B_DETAIL_READBACK = -6912,
};

enum {
  PROM_M42_DETAIL_INVALID_REQUEST = -7001,
  PROM_M42_DETAIL_SIZE_OVERFLOW = -7002,
  PROM_M42_DETAIL_NONFINITE_INPUT = -7003,
  PROM_M42_DETAIL_STALE_WEIGHT_GENERATION = -7004,
  PROM_M42_DETAIL_STALE_X_GENERATION = -7005,
  PROM_M42_DETAIL_CAPABILITY = -7006,
  PROM_M42_DETAIL_RESOURCE = -7007,
  PROM_M42_DETAIL_COMMAND = -7008,
  PROM_M42_DETAIL_SUBMIT = -7009,
  PROM_M42_DETAIL_COMPLETION_UNCERTAIN = -7010,
  PROM_M42_DETAIL_QUERY = -7011,
  PROM_M42_DETAIL_READBACK = -7012,
  PROM_M42_DETAIL_FAULT_INJECTED = -7013,
  PROM_M42_DETAIL_MISMATCH = -7014,
};

enum {
  PROM_M43_DETAIL_INVALID_REQUEST = -7101,
  PROM_M43_DETAIL_HEAD_COUNT = -7102,
  PROM_M43_DETAIL_SIZE_OVERFLOW = -7103,
  PROM_M43_DETAIL_NONFINITE_INPUT = -7104,
  PROM_M43_DETAIL_STALE_WEIGHT_GENERATION = -7105,
  PROM_M43_DETAIL_STALE_X_GENERATION = -7106,
  PROM_M43_DETAIL_CAPABILITY = -7107,
  PROM_M43_DETAIL_CAPACITY = -7108,
  PROM_M43_DETAIL_RESOURCE = -7109,
  PROM_M43_DETAIL_COMMAND = -7110,
  PROM_M43_DETAIL_SUBMIT = -7111,
  PROM_M43_DETAIL_COMPLETION_UNCERTAIN = -7112,
  PROM_M43_DETAIL_QUERY = -7113,
  PROM_M43_DETAIL_READBACK = -7114,
  PROM_M43_DETAIL_FAULT_INJECTED = -7115,
  PROM_M43_DETAIL_MISMATCH = -7116,
};

enum {
  PROM_M44_DETAIL_INVALID_REQUEST = -7201,
  PROM_M44_DETAIL_HEAD_COUNT = -7202,
  PROM_M44_DETAIL_INVALID_VIEW = -7203,
  PROM_M44_DETAIL_SIZE_OVERFLOW = -7204,
  PROM_M44_DETAIL_NONFINITE_INPUT = -7205,
  PROM_M44_DETAIL_STALE_WO_GENERATION = -7206,
  PROM_M44_DETAIL_CAPABILITY = -7207,
  PROM_M44_DETAIL_CAPACITY = -7208,
  PROM_M44_DETAIL_RESOURCE = -7209,
  PROM_M44_DETAIL_COMMAND = -7210,
  PROM_M44_DETAIL_SUBMIT = -7211,
  PROM_M44_DETAIL_COMPLETION_UNCERTAIN = -7212,
  PROM_M44_DETAIL_QUERY = -7213,
  PROM_M44_DETAIL_READBACK = -7214,
  PROM_M44_DETAIL_FAULT_INJECTED = -7215,
  PROM_M44_DETAIL_MISMATCH = -7216,
};

enum {
  PROM_M45_DETAIL_INVALID_REQUEST = -7301,
  PROM_M45_DETAIL_INVALID_VIEW = -7302,
  PROM_M45_DETAIL_SHAPE = -7303,
  PROM_M45_DETAIL_STRIDE = -7304,
  PROM_M45_DETAIL_STALE_X_GENERATION = -7305,
  PROM_M45_DETAIL_STALE_Y_GENERATION = -7306,
  PROM_M45_DETAIL_CROSS_DEVICE = -7307,
  PROM_M45_DETAIL_ALIAS = -7308,
  PROM_M45_DETAIL_EXCLUSIVITY = -7309,
  PROM_M45_DETAIL_SIZE_OVERFLOW = -7310,
  PROM_M45_DETAIL_CAPACITY = -7311,
  PROM_M45_DETAIL_RESOURCE = -7312,
  PROM_M45_DETAIL_COMMAND = -7313,
  PROM_M45_DETAIL_SUBMIT = -7314,
  PROM_M45_DETAIL_COMPLETION_UNCERTAIN = -7315,
  PROM_M45_DETAIL_QUERY = -7316,
  PROM_M45_DETAIL_READBACK = -7317,
  PROM_M45_DETAIL_FAULT_INJECTED = -7318,
  PROM_M45_DETAIL_NONFINITE_INPUT = -7319,
  PROM_M45_DETAIL_MISMATCH = -7320,
  PROM_M45_DETAIL_IN_PLACE_X_REJECTED = -7321,
};

enum {
  PROM_SGEMM_AUDIT_TIMESTAMP_RESET_QUERY = 1u << 0u,
  PROM_SGEMM_AUDIT_TIMESTAMP_START = 1u << 1u,
  PROM_SGEMM_AUDIT_TIMESTAMP_DISPATCH = 1u << 2u,
  PROM_SGEMM_AUDIT_TIMESTAMP_END = 1u << 3u,
};

void prom_vk_set_status(uint32_t* out_stage, int* out_detail_code, uint32_t stage, int detail);
int prom_vk_checked_mul_u32(uint32_t left, uint32_t right, uint32_t* out_value);
uint32_t prom_vk_find_memory_type(VkPhysicalDevice physical_device, uint32_t type_filter, VkMemoryPropertyFlags properties);
uint32_t prom_vk_find_memory_type_for_placement(const VkPhysicalDeviceMemoryProperties* memory_properties,
                                                uint32_t type_filter,
                                                uint32_t placement);
void prom_sgemm_memory_profile_select(const prom_sgemm_memory_profile* profile,
                                      const prom_sgemm_memory_profile_facts* facts,
                                      prom_sgemm_memory_profile_decision* out_decision);
void prom_sgemm_memory_profile_allocation_failed(prom_sgemm_memory_profile_decision* decision);
VkResult prom_vk_create_buffer(VkPhysicalDevice physical_device,
                               VkDevice device,
                               uint32_t test_flags,
                               VkDeviceSize size,
                               VkBufferUsageFlags usage,
                               VkMemoryPropertyFlags memory_properties,
                               int map_memory,
                               prom_vk_buffer* out_buffer);
VkResult prom_vk_create_buffer_for_placement(VkPhysicalDevice physical_device,
                                             VkDevice device,
                                             uint32_t test_flags,
                                             VkDeviceSize size,
                                             VkBufferUsageFlags usage,
                                             uint32_t placement,
                                             int map_memory,
                                             prom_vk_buffer* out_buffer);
void prom_vk_destroy_buffer(VkDevice device, prom_vk_buffer* buffer);

int prom_reactor_runtime_create_impl(void* config, void** out_handle);

int prom_reactor_runtime_validate_handle(void* handle);
int prom_reactor_runtime_get_vk_services(void* handle, prom_vk_runtime_services* out_services);
int prom_reactor_runtime_mark_cooperative_matrix_executable(void* handle);

int prom_reactor_runtime_fft_impl(void* handle,
                                  const PrometheusFftRequest* request,
                                  uint32_t* out_stage,
                                  int* out_detail_code);
int prom_reactor_runtime_fft_benchmark_variant_impl(void* handle,
                                                    const PrometheusFftRequest* request,
                                                    uint32_t requested_variant,
                                                    uint32_t* out_stage,
                                                    int* out_detail_code);
int prom_reactor_runtime_fft_diagnostics_impl(void* handle, PrometheusFftDiagnostics* out_diag);
int prom_reactor_runtime_fft_diagnostics_sized_impl(void* handle,
                                                    PrometheusFftDiagnostics* out_diag,
                                                    uint32_t out_size);
void prom_fft_diag_forget_handle(void* handle);

int prom_reactor_reduction_plan_impl(const PrometheusReductionRequest* request,
                                     PrometheusReductionPlan* out_plan);
int prom_reactor_runtime_reduction_impl(void* handle,
                                        const PrometheusReductionRequest* request,
                                        PrometheusReductionExecutionResult* out_result);
int prom_reactor_runtime_reduction_diagnostics_impl(void* handle,
                                                    PrometheusReductionDiagnostics* out_diag);
int prom_reactor_runtime_reduction_benchmark_impl(void* handle,
                                                  const PrometheusReductionBenchmarkRequest* request,
                                                  PrometheusReductionBenchmarkResult* out_result);
int prom_reduction_validate_plan_for_test(const PrometheusReductionPlan* plan,
                                          uint64_t available_temporary_bytes,
                                          int32_t* out_detail);
int prom_reduction_cpu_reference(const PrometheusReductionRequest* request,
                                 float* output,
                                 int32_t* out_detail);
int prom_reduction_compare(const PrometheusReductionRequest* request,
                           const float* expected,
                           const float* actual,
                           PrometheusReductionBenchmarkResult* out_result);
int prom_m40b_calculate_padding_plan(uint32_t m, uint32_t n, uint32_t k,
                                    prom_m40b_padding_plan* out_plan);
int prom_m40b_validate_device_buffer_view(const prom_device_buffer_view* view,
                                          VkDevice expected_device,
                                          uint32_t expected_element_type,
                                          uint32_t expected_rows,
                                          uint32_t expected_columns,
                                          uint32_t expected_consumer_access,
                                          int32_t* out_detail);
void prom_m40b_plan_command_trace(uint32_t input_mode,
                                  uint32_t submit_plan,
                                  uint32_t reduction_stage_count,
                                  prom_m40b_command_trace* out_trace);
void prom_m40b_selector_evaluate(const prom_m40b_selector_facts* facts,
                                 prom_m40b_selector_decision* out_decision);
int prom_reactor_runtime_m40b_prepare_persistent_b(void* handle,
                                                   const prom_m40b_prepare_request* request,
                                                   prom_m40b_prepare_result* out_result);
int prom_reactor_runtime_m40b_prepare_resident_a(void* handle,
                                                 const prom_m40b_prepare_request* request,
                                                 prom_m40b_prepare_result* out_result);
int prom_reactor_runtime_m40b_execute(void* handle,
                                      const prom_m40b_execution_request* request,
                                      prom_m40b_execution_result* out_result);
int prom_m42_attention_plan_build(const prom_m42_plan_request* request,
                                  prom_m42_attention_plan* out_plan);
int prom_m42_attention_cpu_reference(const prom_m42_reference_request* request,
                                     prom_m42_reference_result* out_result);
int prom_m42_attention_compare(uint32_t stage,
                               const float* expected,
                               const float* actual,
                               uint32_t rows,
                               uint32_t columns,
                               uint32_t padded_rows,
                               uint32_t padded_columns,
                               float absolute_tolerance,
                               float relative_tolerance,
                               uint64_t operator_replay_id,
                               uint64_t reduction_replay_id,
                               prom_m42_mismatch* out_mismatch);
uint64_t prom_m42_k_transpose_index(uint32_t token,
                                    uint32_t head_column,
                                    uint32_t tokens,
                                    uint32_t padded_tokens);
int prom_reactor_runtime_m42_prepare_weights(void* handle,
                                             const prom_m42_weight_prepare_request* request,
                                             prom_m42_weight_prepare_result* out_result);
int prom_reactor_runtime_m42_prepare_resident_x(void* handle,
                                                const prom_m42_resident_x_prepare_request* request,
                                                prom_m42_resident_x_prepare_result* out_result);
int prom_reactor_runtime_m42_execute(void* handle,
                                     const prom_m42_attention_request* request,
                                     prom_m42_attention_result* out_result);
void prom_m43_eligibility_evaluate(const prom_m43_eligibility_facts* facts,
                                   prom_m43_eligibility_decision* out_decision);
int prom_m43_attention_plan_build(const prom_m43_plan_request* request,
                                  prom_m43_attention_plan* out_plan);
int prom_m43_attention_cpu_reference(const prom_m43_reference_request* request,
                                     prom_m43_reference_result* out_result);
int prom_m43_attention_compare(const float* expected,
                               const float* actual,
                               uint32_t head_count,
                               uint32_t tokens,
                               uint32_t head_dim,
                               float absolute_tolerance,
                               float relative_tolerance,
                               const prom_m43_attention_plan* plan,
                               prom_m43_mismatch* out_mismatch);
uint64_t prom_m43_output_index(uint32_t head,
                               uint32_t token,
                               uint32_t column,
                               uint32_t tokens,
                               uint32_t head_dim);
int prom_reactor_runtime_m43_prepare_weight(void* handle,
                                            const prom_m43_weight_prepare_request* request,
                                            prom_m43_weight_prepare_result* out_result);
int prom_reactor_runtime_m43_prepare_resident_x(void* handle,
                                                const prom_m43_resident_x_prepare_request* request,
                                                prom_m43_resident_x_prepare_result* out_result);
int prom_reactor_runtime_m43_execute(void* handle,
                                     const prom_m43_attention_group_request* request,
                                     prom_m43_attention_group_result* out_result);
void prom_m44_eligibility_evaluate(const prom_m44_eligibility_facts* facts,
                                   prom_m44_eligibility_decision* out_decision);
int prom_m44_output_projection_plan_build(const prom_m44_plan_request* request,
                                          prom_m44_output_projection_plan* out_plan);
uint64_t prom_m44_concat_index(uint32_t token,
                               uint32_t head,
                               uint32_t column,
                               uint32_t tokens,
                               uint32_t head_dim);
int prom_m44_output_projection_cpu_reference(const prom_m44_reference_request* request,
                                             prom_m44_reference_result* out_result);
int prom_m44_output_projection_compare(const float* expected,
                                       const float* actual,
                                       uint32_t tokens,
                                       uint32_t model_width,
                                       float absolute_tolerance,
                                       float relative_tolerance,
                                       uint32_t strategy,
                                       uint64_t wo_generation,
                                       uint64_t m43_aggregate_replay_id,
                                       uint64_t m44_replay_id,
                                       prom_m44_mismatch* out_mismatch);
int prom_reactor_runtime_m44_prepare_wo(void* handle,
                                       const prom_m44_wo_prepare_request* request,
                                       prom_m44_wo_prepare_result* out_result);
int prom_reactor_runtime_m44_execute_composed(void* handle,
                                              const prom_m44_composed_request* request,
                                              prom_m44_composed_result* out_result);
int prom_reactor_runtime_m44_execute_host_bounce(void* handle,
                                                 const prom_m44_host_bounce_request* request,
                                                 prom_m44_host_bounce_result* out_result);
int prom_m45_residual_plan_build(const prom_m45_plan_request* request,
                                 prom_m45_residual_plan* out_plan);
int prom_m45_residual_cpu_reference(const prom_m45_reference_request* request);
int prom_m45_residual_compare(const float* expected,
                              const float* actual,
                              uint32_t tokens,
                              uint32_t model_width,
                              float absolute_tolerance,
                              float relative_tolerance,
                              const prom_m45_residual_plan* plan,
                              prom_m45_mismatch* out_mismatch);
int prom_reactor_runtime_m45_execute_composed(void* handle,
                                              const prom_m45_composed_request* request,
                                              prom_m45_composed_result* out_result);
int prom_reactor_runtime_m45_read_resident_x(void* handle,
                                             const prom_m45_resident_x_readback_request* request,
                                             prom_m45_resident_x_readback_result* out_result);
uint16_t prom_sgemm_float32_to_fp16_bits(float value);
float prom_sgemm_fp16_bits_to_float32(uint16_t value);
void prom_reactor_runtime_reduction_cleanup_state(void* state, VkDevice device);
void* prom_reactor_runtime_reduction_state(void* handle);
int prom_reactor_runtime_set_reduction_state(void* handle, void* state);

int prom_reactor_runtime_destroy_impl(void* handle);
int prom_reactor_runtime_probe_impl(void* handle, PrometheusCaps* out_caps);
int prom_reactor_runtime_vulkan_device_diagnostics_impl(void* handle, PrometheusVulkanDeviceDiagnostics* out_diag);
int prom_reactor_runtime_sgemm_impl(void* handle,
                                    const float* a,
                                    const float* b,
                                    float* c,
                                    uint32_t m,
                                    uint32_t n,
                                    uint32_t k,
                                    uint32_t* out_stage,
                                    int* out_detail_code);
int prom_reactor_runtime_sgemm_benchmark_variant_impl(void* handle,
                                                      const float* a,
                                                      const float* b,
                                                      float* c,
                                                      uint32_t m,
                                                      uint32_t n,
                                                      uint32_t k,
                                                      uint32_t requested_variant,
                                                      uint32_t* out_stage,
                                                      int* out_detail_code);
int prom_reactor_runtime_sgemm_audit_impl(void* handle,
                                          const float* a,
                                          const float* b,
                                          float* c,
                                          uint32_t m,
                                          uint32_t n,
                                          uint32_t k,
                                          const prom_sgemm_audit_execution_descriptor* descriptor,
                                          prom_sgemm_audit_execution_result* out_result);
int prom_reactor_runtime_sgemm_audit_benchmark_impl(void* handle,
                                                    const float* a,
                                                    const float* b,
                                                    float* c,
                                                    uint32_t m,
                                                    uint32_t n,
                                                    uint32_t k,
                                                    const prom_sgemm_audit_execution_descriptor* descriptor,
                                                    uint32_t warmup,
                                                    uint32_t iterations,
                                                    uint64_t* out_samples_ns,
                                                    uint32_t sample_capacity,
                                                    prom_sgemm_audit_execution_result* out_result);
int prom_reactor_runtime_sgemm_placement_benchmark_impl(void* handle,
                                                        const float* a,
                                                        const float* b,
                                                        float* c,
                                                        uint32_t m,
                                                        uint32_t n,
                                                        uint32_t k,
                                                        const prom_sgemm_audit_execution_descriptor* descriptor,
                                                        const prom_sgemm_placement_benchmark_options* options,
                                                        uint64_t* out_gpu_samples_ns,
                                                        uint64_t* out_preparation_samples_ns,
                                                        uint64_t* out_end_to_end_samples_ns,
                                                        uint32_t sample_capacity,
                                                        prom_sgemm_placement_benchmark_result* out_result);
int prom_reactor_runtime_sgemm_placement_benchmark_detailed_impl(
    void* handle, const float* a, const float* b, float* c,
    uint32_t m, uint32_t n, uint32_t k,
    const prom_sgemm_audit_execution_descriptor* descriptor,
    const prom_sgemm_placement_benchmark_options* options,
    uint64_t* out_kernel_samples_ns, uint64_t* out_preparation_samples_ns,
    uint64_t* out_end_to_end_samples_ns, uint64_t* out_conversion_samples_ns,
    uint64_t* out_upload_samples_ns, uint64_t* out_readback_samples_ns,
    uint32_t sample_capacity, prom_sgemm_placement_benchmark_result* out_result);
int prom_reactor_runtime_sgemm_resident_benchmark_impl(void* handle,
                                                       const PrometheusSgemmResidentBenchmarkRequest* request,
                                                       PrometheusSgemmResidentBenchmarkResult* out_result);
int prom_reactor_runtime_sgemm_batch_impl(void* handle,
                                          const PrometheusSgemmBatchEntry* entries,
                                          uint32_t entry_count,
                                          uint32_t flags,
                                          uint32_t* out_stage,
                                          int* out_detail_code);
int prom_reactor_runtime_sgemm_batch_m31_test_impl(void* handle,
                                                   const PrometheusSgemmBatchEntry* entries,
                                                   uint32_t entry_count,
                                                   uint32_t flags,
                                                   uint32_t* out_stage,
                                                   int* out_detail_code);
int prom_reactor_runtime_sgemm_submit_async_impl(void* handle,
                                                 const float* a,
                                                 const float* b,
                                                 uint32_t m,
                                                 uint32_t n,
                                                 uint32_t k,
                                                 int* out_task_id,
                                                 uint32_t* out_stage,
                                                 int* out_detail_code);
int prom_reactor_runtime_sgemm_query_async_impl(void* handle, int task_id, PrometheusAsyncStatus* out_status);
int prom_reactor_runtime_sgemm_async_diagnostics_impl(void* handle, PrometheusSgemmAsyncDiagnostics* out_diag);
int prom_reactor_runtime_sgemm_consume_async_impl(void* handle,
                                                  int task_id,
                                                  float* c,
                                                  uint32_t c_len,
                                                  uint32_t* out_stage,
                                                  int* out_detail_code);
int prom_reactor_runtime_sgemm_abandon_async_impl(void* handle, int task_id);
int prom_reactor_runtime_sgemm_policy_diagnostics_impl(void* handle, PrometheusSgemmPolicyDiagnostics* out_diag);
int prom_reactor_runtime_sgemm_policy_diagnostics_sized_impl(void* handle,
                                                             PrometheusSgemmPolicyDiagnostics* out_diag,
                                                             uint32_t out_size);
int prom_reactor_runtime_p15_test_seed_matured_reservation_impl(void* handle, uint32_t shape_class, uint32_t variant_id, uint64_t target_tick);
int prom_reactor_runtime_sgemm_batch_diagnostics_impl(void* handle, PrometheusSgemmBatchDiagnostics* out_diag);

#ifdef __cplusplus
}
#endif

#endif
