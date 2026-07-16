#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_H

#include "reactor_api.h"
#include "reactor_batch.h"
#include "reactor_sgemm_dispatch_metadata.h"
#include <vulkan/vulkan.h>

#ifdef __cplusplus
extern "C" {
#endif

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
