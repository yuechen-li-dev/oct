#include "reactor_vulkan.h"

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
  uint32_t memory_type_index;

  if (device == VK_NULL_HANDLE || out_buffer == NULL) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }

  if ((test_flags & PROM_TESTCFG_FAIL_BUFFER_ALLOC) != 0u) {
    return VK_ERROR_OUT_OF_DEVICE_MEMORY;
  }

  memset(out_buffer, 0, sizeof(*out_buffer));
  out_buffer->size = size;

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
  memory_type_index = prom_vk_find_memory_type(physical_device, requirements.memoryTypeBits, memory_properties);
  if ((test_flags & PROM_TESTCFG_FORCE_NO_MEMORY_TYPE) != 0u ||
      (((test_flags & PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY) != 0u) &&
       (memory_properties & VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT) != 0u)) {
    memory_type_index = UINT32_MAX;
  }
  if (memory_type_index == UINT32_MAX) {
    return VK_ERROR_FEATURE_NOT_PRESENT;
  }

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
