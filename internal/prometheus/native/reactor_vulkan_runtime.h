#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_RUNTIME_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_RUNTIME_H

#include "reactor_vulkan.h"
#include "reactor_shader_package.h"

#ifdef __cplusplus
extern "C" {
#endif

/*
 * The concrete owner for resources shared by all Vulkan operation families.
 * Operation state must borrow these values through prom_vk_runtime_services;
 * it must not add SGEMM, model, FFT, or ray resources here.
 */
typedef struct prom_vk_runtime {
  char* shader_package_root;
  prom_shader_package* shader_package;
  uint32_t test_flags;
  uint32_t available;
  uint32_t reason_code;
  int init_detail_code;

  VkInstance instance;
  VkDebugUtilsMessengerEXT validation_debug_messenger;
  uint32_t validation_requested;
  uint32_t validation_available;
  uint32_t validation_enabled;
  uint32_t validation_debug_utils_active;
  uint32_t validation_message_count;
  uint32_t validation_warning_count;
  uint32_t validation_error_count;
  VkDebugUtilsMessageSeverityFlagBitsEXT validation_last_severity;
  VkDebugUtilsMessageTypeFlagsEXT validation_last_type;
  char validation_last_message_id[128];
  char validation_last_message[512];

  VkPhysicalDevice physical_device;
  VkDevice device;
  uint32_t queue_family_index;
  uint32_t transfer_queue_family_index;
  uint32_t dedicated_transfer_available;
  uint32_t transfer_queue_enabled;
  VkQueue compute_queue;
  VkQueue transfer_queue;
  VkCommandPool command_pool;
  VkCommandPool transfer_command_pool;

  uint32_t software_vulkan;
  uint32_t has_device_local_memory;
  uint32_t has_host_visible_memory;
  uint32_t capability_fp16_storage;
  uint32_t occupancy_register_file_class;
  uint32_t occupancy_shared_memory_class;
  uint32_t occupancy_memory_bandwidth_class;
  uint32_t occupancy_fp32_throughput_class;
  uint32_t occupancy_max_workgroup_class;
  uint32_t occupancy_queue_capability_class;
  uint32_t occupancy_has_exact_profile;
  uint32_t timestamp_valid_bits;
  float timestamp_period_ns;

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
  uint32_t subgroup_shuffle_supported;
  uint32_t subgroup_fixed_size_32_admitted;
  uint32_t subgroup_owned_attention_admitted;
  uint32_t subgroup_owned_attention_topology_proven;
  uint32_t ray_query_state;
  uint32_t ray_query_acceleration_structure_extension_supported;
  uint32_t ray_query_extension_supported;
  uint32_t ray_query_deferred_host_operations_extension_supported;
  uint32_t ray_query_buffer_device_address_supported;
  uint32_t ray_query_acceleration_structure_supported;
  uint32_t ray_query_supported;
  PFN_vkCreateAccelerationStructureKHR create_acceleration_structure;
  PFN_vkDestroyAccelerationStructureKHR destroy_acceleration_structure;
  PFN_vkGetAccelerationStructureBuildSizesKHR get_acceleration_structure_build_sizes;
  PFN_vkCmdBuildAccelerationStructuresKHR cmd_build_acceleration_structures;
  PFN_vkGetAccelerationStructureDeviceAddressKHR get_acceleration_structure_device_address;
} prom_vk_runtime;

VkResult prom_vk_runtime_init(prom_vk_runtime* runtime, uint32_t test_flags);
VkResult prom_vk_runtime_enable_validation(prom_vk_runtime* runtime);
void prom_vk_runtime_wait_idle(prom_vk_runtime* runtime);
void prom_vk_runtime_cleanup(prom_vk_runtime* runtime);
void prom_vk_runtime_destroy_package(prom_vk_runtime* runtime);

#ifdef __cplusplus
}
#endif

#endif
