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
