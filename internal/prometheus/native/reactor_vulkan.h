#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_H

#include "reactor_api.h"
#include <vulkan/vulkan.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct prom_vk_buffer {
  VkBuffer buffer;
  VkDeviceMemory memory;
  void* mapped;
  VkDeviceSize size;
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

/*
 * Immutable SGEMM batch planning is internal native API, not public ABI.
 * These records contain caller-order logical facts only. They deliberately do
 * not own Vulkan resources, output staging, task records, or completion state.
 */
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

/* Mutable M31 execution facts.  This is deliberately separate from the
 * immutable plan: task records and ring slots may be recycled while this
 * caller-order record remains the batch's stable identity. */
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

void prom_vk_set_status(uint32_t* out_stage, int* out_detail_code, uint32_t stage, int detail);
int prom_vk_checked_mul_u32(uint32_t left, uint32_t right, uint32_t* out_value);
uint32_t prom_vk_find_memory_type(VkPhysicalDevice physical_device, uint32_t type_filter, VkMemoryPropertyFlags properties);
VkResult prom_vk_create_buffer(VkPhysicalDevice physical_device,
                               VkDevice device,
                               uint32_t test_flags,
                               VkDeviceSize size,
                               VkBufferUsageFlags usage,
                               VkMemoryPropertyFlags memory_properties,
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
int prom_reactor_runtime_sgemm_batch_legacy_test_impl(void* handle,
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
