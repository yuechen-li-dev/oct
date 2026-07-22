#include "reactor_vulkan.h"
#include "reactor_judgment_engine.h"

#include <limits.h>
#include <stddef.h>
#include <string.h>

void prom_vk_set_status(uint32_t* out_stage, int* out_detail_code, uint32_t stage, int detail) {
  if (out_stage != NULL) {
    *out_stage = stage;
  }
  if (out_detail_code != NULL) {
    *out_detail_code = detail;
  }
}

int prom_vk_checked_mul_u32(uint32_t left, uint32_t right, uint32_t* out_value) {
  if (out_value == NULL) {
    return 0;
  }
  if (left != 0u && right > UINT32_MAX / left) {
    return 0;
  }
  *out_value = left * right;
  return 1;
}

uint32_t prom_vk_find_memory_type(VkPhysicalDevice physical_device, uint32_t type_filter, VkMemoryPropertyFlags properties) {
  uint32_t i;
  VkPhysicalDeviceMemoryProperties memory_properties;
  vkGetPhysicalDeviceMemoryProperties(physical_device, &memory_properties);
  for (i = 0u; i < memory_properties.memoryTypeCount; ++i) {
    const uint32_t bit = (1u << i);
    if ((type_filter & bit) != 0u &&
        (memory_properties.memoryTypes[i].propertyFlags & properties) == properties) {
      return i;
    }
  }
  return UINT32_MAX;
}

uint32_t prom_vk_find_memory_type_for_placement(const VkPhysicalDeviceMemoryProperties* memory_properties,
                                                uint32_t type_filter,
                                                uint32_t placement) {
  VkMemoryPropertyFlags required = 0u;
  VkMemoryPropertyFlags forbidden = 0u;
  uint32_t i;
  if (memory_properties == NULL) {
    return UINT32_MAX;
  }
  if (placement == PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL) {
    required = VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT;
    forbidden = VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT;
  } else if (placement == PROM_SGEMM_MEMORY_PLACEMENT_HOST_VISIBLE_COHERENT_SYSTEM) {
    required = VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT;
    forbidden = VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT;
  } else if (placement == PROM_SGEMM_MEMORY_PLACEMENT_HOST_VISIBLE_COHERENT_DEVICE_LOCAL) {
    required = VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT | VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
               VK_MEMORY_PROPERTY_HOST_COHERENT_BIT;
  } else {
    return UINT32_MAX;
  }
  for (i = 0u; i < memory_properties->memoryTypeCount; ++i) {
    const VkMemoryPropertyFlags flags = memory_properties->memoryTypes[i].propertyFlags;
    if ((type_filter & (1u << i)) != 0u && (flags & required) == required && (flags & forbidden) == 0u) {
      return i;
    }
  }
  return UINT32_MAX;
}

void prom_sgemm_memory_profile_select(const prom_sgemm_memory_profile* profile,
                                      const prom_sgemm_memory_profile_facts* facts,
                                      prom_sgemm_memory_profile_decision* out_decision) {
  uint64_t available_budget = 0u;
  if (out_decision == NULL) {
    return;
  }
  memset(out_decision, 0, sizeof(*out_decision));
  out_decision->fallback_placement = PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL;
  if (profile == NULL || facts == NULL || profile->enabled == 0u || facts->experiment_enabled == 0u) {
    out_decision->reason = PROM_SGEMM_MEMORY_PROFILE_REASON_DISABLED;
    return;
  }
  if (profile->kernel_compute_mode != (uint32_t)PROM_VK_COMPUTE_PACKED4_FP32 &&
      profile->kernel_compute_mode != (uint32_t)PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
    out_decision->reason = PROM_SGEMM_MEMORY_PROFILE_REASON_KERNEL;
    return;
  }
  if (profile->kernel_compute_mode != facts->kernel_compute_mode) {
    out_decision->reason = PROM_SGEMM_MEMORY_PROFILE_REASON_KERNEL;
    return;
  }
  if (profile->vendor_id != facts->vendor_id || profile->device_id != facts->device_id) {
    out_decision->reason = PROM_SGEMM_MEMORY_PROFILE_REASON_DEVICE;
    return;
  }
  if (facts->driver_version < profile->driver_version_min ||
      (profile->driver_version_max != 0u && facts->driver_version > profile->driver_version_max)) {
    out_decision->reason = PROM_SGEMM_MEMORY_PROFILE_REASON_DRIVER;
    return;
  }
  if (facts->mapped_device_local_type_exists == 0u) {
    out_decision->reason = PROM_SGEMM_MEMORY_PROFILE_REASON_MEMORY_TYPE;
    return;
  }
  if (facts->m < profile->minimum_m || facts->n < profile->minimum_n || facts->k < profile->minimum_k) {
    out_decision->reason = PROM_SGEMM_MEMORY_PROFILE_REASON_SHAPE;
    return;
  }
  if (profile->maximum_total_bytes != 0u && facts->total_bytes > profile->maximum_total_bytes) {
    out_decision->reason = PROM_SGEMM_MEMORY_PROFILE_REASON_CAPACITY;
    return;
  }
  if (facts->heap_budget_bytes > facts->heap_usage_bytes) {
    available_budget = facts->heap_budget_bytes - facts->heap_usage_bytes;
  }
  if (available_budget < facts->total_bytes ||
      available_budget - facts->total_bytes < profile->minimum_budget_headroom_bytes) {
    out_decision->reason = PROM_SGEMM_MEMORY_PROFILE_REASON_BUDGET;
    return;
  }
  out_decision->matched = 1u;
  out_decision->input_placement = profile->input_placement;
  out_decision->output_placement = profile->output_placement;
  out_decision->reason = PROM_SGEMM_MEMORY_PROFILE_REASON_MATCHED;
}

void prom_sgemm_memory_profile_allocation_failed(prom_sgemm_memory_profile_decision* decision) {
  if (decision == NULL) {
    return;
  }
  decision->matched = 0u;
  decision->input_placement = PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL;
  decision->output_placement = PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL;
  decision->fallback_placement = PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL;
  decision->reason = PROM_SGEMM_MEMORY_PROFILE_REASON_ALLOCATION_FAILURE;
}

VkResult prom_vk_create_buffer(VkPhysicalDevice physical_device,
                               VkDevice device,
                               uint32_t test_flags,
                               VkDeviceSize size,
                               VkBufferUsageFlags usage,
                               VkMemoryPropertyFlags memory_properties,
                               int map_memory,
                               prom_vk_buffer* out_buffer) {
  VkResult result;
  VkBufferCreateInfo buffer_info;
  VkMemoryRequirements requirements;
  VkMemoryAllocateInfo alloc_info;
  VkPhysicalDeviceMemoryProperties physical_memory_properties;
  uint32_t memory_type_index;

  if (device == VK_NULL_HANDLE || out_buffer == NULL) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }

  if ((test_flags & PROM_TESTCFG_FAIL_BUFFER_ALLOC) != 0u) {
    return VK_ERROR_OUT_OF_DEVICE_MEMORY;
  }

  memset(out_buffer, 0, sizeof(*out_buffer));
  out_buffer->size = size;
  out_buffer->usage_flags = usage;
  out_buffer->sharing_mode = VK_SHARING_MODE_EXCLUSIVE;
  out_buffer->memory_offset = 0u;

  memset(&buffer_info, 0, sizeof(buffer_info));
  buffer_info.sType = VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO;
  buffer_info.size = size;
  buffer_info.usage = usage;
  buffer_info.sharingMode = VK_SHARING_MODE_EXCLUSIVE;

  result = vkCreateBuffer(device, &buffer_info, NULL, &out_buffer->buffer);
  if (result != VK_SUCCESS) {
    return result;
  }

  vkGetBufferMemoryRequirements(device, out_buffer->buffer, &requirements);
  out_buffer->memory_alignment = requirements.alignment;
  memory_type_index = prom_vk_find_memory_type(physical_device, requirements.memoryTypeBits, memory_properties);
  if ((test_flags & PROM_TESTCFG_FORCE_NO_MEMORY_TYPE) != 0u ||
      (((test_flags & PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY) != 0u) &&
       (memory_properties & VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT) != 0u)) {
    memory_type_index = UINT32_MAX;
  }
  if (memory_type_index == UINT32_MAX) {
    return VK_ERROR_FEATURE_NOT_PRESENT;
  }
  vkGetPhysicalDeviceMemoryProperties(physical_device, &physical_memory_properties);
  out_buffer->memory_type_index = memory_type_index;
  out_buffer->memory_property_flags = physical_memory_properties.memoryTypes[memory_type_index].propertyFlags;

  memset(&alloc_info, 0, sizeof(alloc_info));
  alloc_info.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO;
  alloc_info.allocationSize = requirements.size;
  alloc_info.memoryTypeIndex = memory_type_index;

  result = vkAllocateMemory(device, &alloc_info, NULL, &out_buffer->memory);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = vkBindBufferMemory(device, out_buffer->buffer, out_buffer->memory, 0);
  if (result != VK_SUCCESS) {
    return result;
  }

  if (map_memory != 0) {
    result = vkMapMemory(device, out_buffer->memory, 0, size, 0, &out_buffer->mapped);
    if (result != VK_SUCCESS) {
      return result;
    }
  }
  return VK_SUCCESS;
}

VkResult prom_vk_create_device_address_buffer(VkPhysicalDevice physical_device,
                                              VkDevice device,
                                              uint32_t test_flags,
                                              VkDeviceSize size,
                                              VkBufferUsageFlags usage,
                                              VkMemoryPropertyFlags memory_properties,
                                              int map_memory,
                                              prom_vk_buffer* out_buffer) {
  VkResult result;
  VkBufferCreateInfo buffer_info;
  VkMemoryRequirements requirements;
  VkMemoryAllocateInfo alloc_info;
  VkMemoryAllocateFlagsInfo address_flags;
  VkPhysicalDeviceMemoryProperties physical_memory_properties;
  uint32_t memory_type_index;

  if (physical_device == VK_NULL_HANDLE || device == VK_NULL_HANDLE || out_buffer == NULL || size == 0u) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }
  if ((test_flags & PROM_TESTCFG_FAIL_BUFFER_ALLOC) != 0u) {
    return VK_ERROR_OUT_OF_DEVICE_MEMORY;
  }

  memset(out_buffer, 0, sizeof(*out_buffer));
  out_buffer->size = size;
  out_buffer->usage_flags = usage | VK_BUFFER_USAGE_SHADER_DEVICE_ADDRESS_BIT;
  out_buffer->sharing_mode = VK_SHARING_MODE_EXCLUSIVE;

  memset(&buffer_info, 0, sizeof(buffer_info));
  buffer_info.sType = VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO;
  buffer_info.size = size;
  buffer_info.usage = out_buffer->usage_flags;
  buffer_info.sharingMode = VK_SHARING_MODE_EXCLUSIVE;
  result = vkCreateBuffer(device, &buffer_info, NULL, &out_buffer->buffer);
  if (result != VK_SUCCESS) {
    return result;
  }

  vkGetBufferMemoryRequirements(device, out_buffer->buffer, &requirements);
  out_buffer->memory_alignment = requirements.alignment;
  memory_type_index = prom_vk_find_memory_type(physical_device, requirements.memoryTypeBits, memory_properties);
  if ((test_flags & PROM_TESTCFG_FORCE_NO_MEMORY_TYPE) != 0u ||
      (((test_flags & PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY) != 0u) &&
       (memory_properties & VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT) != 0u)) {
    memory_type_index = UINT32_MAX;
  }
  if (memory_type_index == UINT32_MAX) {
    prom_vk_destroy_buffer(device, out_buffer);
    return VK_ERROR_FEATURE_NOT_PRESENT;
  }

  vkGetPhysicalDeviceMemoryProperties(physical_device, &physical_memory_properties);
  out_buffer->memory_type_index = memory_type_index;
  out_buffer->memory_property_flags = physical_memory_properties.memoryTypes[memory_type_index].propertyFlags;
  memset(&address_flags, 0, sizeof(address_flags));
  address_flags.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_FLAGS_INFO;
  address_flags.flags = VK_MEMORY_ALLOCATE_DEVICE_ADDRESS_BIT;
  memset(&alloc_info, 0, sizeof(alloc_info));
  alloc_info.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO;
  alloc_info.pNext = &address_flags;
  alloc_info.allocationSize = requirements.size;
  alloc_info.memoryTypeIndex = memory_type_index;
  result = vkAllocateMemory(device, &alloc_info, NULL, &out_buffer->memory);
  if (result == VK_SUCCESS) {
    result = vkBindBufferMemory(device, out_buffer->buffer, out_buffer->memory, 0u);
  }
  if (result == VK_SUCCESS && map_memory != 0) {
    result = vkMapMemory(device, out_buffer->memory, 0u, size, 0u, &out_buffer->mapped);
  }
  if (result != VK_SUCCESS) {
    prom_vk_destroy_buffer(device, out_buffer);
  }
  return result;
}

/* M2 uses concurrent sharing only for its two weight windows.  This avoids
   ownership handoffs for a buffer that alternates between the dedicated
   transfer and compute families; all other reactor buffers retain the
   established exclusive-owner recipe. */
VkResult prom_vk_create_buffer_shared_between_families(
    VkPhysicalDevice physical_device, VkDevice device, uint32_t test_flags, VkDeviceSize size,
    VkBufferUsageFlags usage, VkMemoryPropertyFlags memory_properties, int map_memory,
    uint32_t first_queue_family, uint32_t second_queue_family, prom_vk_buffer* out_buffer) {
  VkResult result;
  VkBufferCreateInfo buffer_info;
  VkMemoryRequirements requirements;
  VkMemoryAllocateInfo alloc_info;
  VkPhysicalDeviceMemoryProperties physical_memory_properties;
  uint32_t memory_type_index;
  uint32_t families[2];
  if (device == VK_NULL_HANDLE || out_buffer == NULL || first_queue_family == UINT32_MAX ||
      second_queue_family == UINT32_MAX || first_queue_family == second_queue_family) return VK_ERROR_INITIALIZATION_FAILED;
  if ((test_flags & PROM_TESTCFG_FAIL_BUFFER_ALLOC) != 0u) return VK_ERROR_OUT_OF_DEVICE_MEMORY;
  memset(out_buffer, 0, sizeof(*out_buffer));
  families[0] = first_queue_family;
  families[1] = second_queue_family;
  out_buffer->size = size;
  out_buffer->usage_flags = usage;
  out_buffer->sharing_mode = VK_SHARING_MODE_CONCURRENT;
  memset(&buffer_info, 0, sizeof(buffer_info));
  buffer_info.sType = VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO;
  buffer_info.size = size;
  buffer_info.usage = usage;
  buffer_info.sharingMode = VK_SHARING_MODE_CONCURRENT;
  buffer_info.queueFamilyIndexCount = 2u;
  buffer_info.pQueueFamilyIndices = families;
  result = vkCreateBuffer(device, &buffer_info, NULL, &out_buffer->buffer);
  if (result != VK_SUCCESS) return result;
  vkGetBufferMemoryRequirements(device, out_buffer->buffer, &requirements);
  out_buffer->memory_alignment = requirements.alignment;
  memory_type_index = prom_vk_find_memory_type(physical_device, requirements.memoryTypeBits, memory_properties);
  if ((test_flags & PROM_TESTCFG_FORCE_NO_MEMORY_TYPE) != 0u ||
      (((test_flags & PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY) != 0u) &&
       (memory_properties & VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT) != 0u)) memory_type_index = UINT32_MAX;
  if (memory_type_index == UINT32_MAX) {
    prom_vk_destroy_buffer(device, out_buffer);
    return VK_ERROR_FEATURE_NOT_PRESENT;
  }
  vkGetPhysicalDeviceMemoryProperties(physical_device, &physical_memory_properties);
  out_buffer->memory_type_index = memory_type_index;
  out_buffer->memory_property_flags = physical_memory_properties.memoryTypes[memory_type_index].propertyFlags;
  memset(&alloc_info, 0, sizeof(alloc_info));
  alloc_info.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO;
  alloc_info.allocationSize = requirements.size;
  alloc_info.memoryTypeIndex = memory_type_index;
  result = vkAllocateMemory(device, &alloc_info, NULL, &out_buffer->memory);
  if (result != VK_SUCCESS) {
    prom_vk_destroy_buffer(device, out_buffer);
    return result;
  }
  result = vkBindBufferMemory(device, out_buffer->buffer, out_buffer->memory, 0);
  if (result != VK_SUCCESS) {
    prom_vk_destroy_buffer(device, out_buffer);
    return result;
  }
  if (map_memory != 0) {
    result = vkMapMemory(device, out_buffer->memory, 0, size, 0, &out_buffer->mapped);
    if (result != VK_SUCCESS) {
      prom_vk_destroy_buffer(device, out_buffer);
      return result;
    }
  }
  return VK_SUCCESS;
}

VkResult prom_vk_create_buffer_for_placement(VkPhysicalDevice physical_device,
                                             VkDevice device,
                                             uint32_t test_flags,
                                             VkDeviceSize size,
                                             VkBufferUsageFlags usage,
                                             uint32_t placement,
                                             int map_memory,
                                             prom_vk_buffer* out_buffer) {
  VkResult result;
  VkBufferCreateInfo buffer_info;
  VkMemoryRequirements requirements;
  VkMemoryAllocateInfo alloc_info;
  VkPhysicalDeviceMemoryProperties memory_properties;
  uint32_t memory_type_index;
  if (device == VK_NULL_HANDLE || out_buffer == NULL) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }
  if ((test_flags & PROM_TESTCFG_FAIL_BUFFER_ALLOC) != 0u) {
    return VK_ERROR_OUT_OF_DEVICE_MEMORY;
  }
  memset(out_buffer, 0, sizeof(*out_buffer));
  out_buffer->size = size;
  out_buffer->usage_flags = usage;
  out_buffer->sharing_mode = VK_SHARING_MODE_EXCLUSIVE;
  memset(&buffer_info, 0, sizeof(buffer_info));
  buffer_info.sType = VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO;
  buffer_info.size = size;
  buffer_info.usage = usage;
  buffer_info.sharingMode = VK_SHARING_MODE_EXCLUSIVE;
  result = vkCreateBuffer(device, &buffer_info, NULL, &out_buffer->buffer);
  if (result != VK_SUCCESS) {
    return result;
  }
  vkGetBufferMemoryRequirements(device, out_buffer->buffer, &requirements);
  out_buffer->memory_alignment = requirements.alignment;
  vkGetPhysicalDeviceMemoryProperties(physical_device, &memory_properties);
  memory_type_index = prom_vk_find_memory_type_for_placement(&memory_properties, requirements.memoryTypeBits, placement);
  if ((test_flags & PROM_TESTCFG_FORCE_NO_MEMORY_TYPE) != 0u ||
      ((test_flags & PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY) != 0u &&
       placement != PROM_SGEMM_MEMORY_PLACEMENT_HOST_VISIBLE_COHERENT_SYSTEM)) {
    memory_type_index = UINT32_MAX;
  }
  if (memory_type_index == UINT32_MAX) {
    prom_vk_destroy_buffer(device, out_buffer);
    return VK_ERROR_FEATURE_NOT_PRESENT;
  }
  out_buffer->memory_type_index = memory_type_index;
  out_buffer->memory_property_flags = memory_properties.memoryTypes[memory_type_index].propertyFlags;
  memset(&alloc_info, 0, sizeof(alloc_info));
  alloc_info.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO;
  alloc_info.allocationSize = requirements.size;
  alloc_info.memoryTypeIndex = memory_type_index;
  result = vkAllocateMemory(device, &alloc_info, NULL, &out_buffer->memory);
  if (result == VK_SUCCESS) {
    result = vkBindBufferMemory(device, out_buffer->buffer, out_buffer->memory, 0u);
  }
  if (result == VK_SUCCESS && map_memory != 0) {
    result = vkMapMemory(device, out_buffer->memory, 0u, size, 0u, &out_buffer->mapped);
  }
  if (result != VK_SUCCESS) {
    prom_vk_destroy_buffer(device, out_buffer);
  }
  return result;
}

void prom_vk_destroy_buffer(VkDevice device, prom_vk_buffer* buffer) {
  if (device == VK_NULL_HANDLE || buffer == NULL) {
    return;
  }
  if (buffer->mapped != NULL) {
    vkUnmapMemory(device, buffer->memory);
    buffer->mapped = NULL;
  }
  if (buffer->buffer != VK_NULL_HANDLE) {
    vkDestroyBuffer(device, buffer->buffer, NULL);
    buffer->buffer = VK_NULL_HANDLE;
  }
  if (buffer->memory != VK_NULL_HANDLE) {
    vkFreeMemory(device, buffer->memory, NULL);
    buffer->memory = VK_NULL_HANDLE;
  }
}
