#include "reactor_vulkan.h"

#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

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

#define PROMETHEUS_RUNTIME_MAGIC 0x50524f4du
#define PROMETHEUS_MAX_TRACKED_HANDLES 256

#define PROM_VK_LOCAL_SIZE_X 8u
#define PROM_VK_LOCAL_SIZE_Y 8u

typedef struct prom_vk_buffer {
  VkBuffer buffer;
  VkDeviceMemory memory;
  void* mapped;
  VkDeviceSize size;
} prom_vk_buffer;

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
  VkQueue compute_queue;
  VkCommandPool command_pool;
  VkDescriptorSetLayout descriptor_set_layout;
  VkDescriptorPool descriptor_pool;
  VkDescriptorSet descriptor_set;
  VkCommandBuffer command_buffer;
  VkFence submit_fence;
  VkPipelineLayout pipeline_layout;
  VkPipeline pipeline;
  prom_vk_buffer reusable_a;
  prom_vk_buffer reusable_b;
  prom_vk_buffer reusable_c;
  uint32_t reusable_m;
  uint32_t reusable_n;
  uint32_t reusable_k;
  uint32_t has_reusable_buffers;
  uint32_t descriptor_bindings_valid;
  uint32_t command_recording_valid;
  uint32_t in_flight_submit;
  uint32_t software_vulkan;
} prometheus_runtime;

typedef struct prom_vk_push {
  uint32_t m;
  uint32_t n;
  uint32_t k;
  uint32_t reserved0;
} prom_vk_push;

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

/* SPIR-V for:
 * #version 450
 * layout(local_size_x=8, local_size_y=8) in;
 * layout(set=0,binding=0) readonly buffer ABuffer{float a[];};
 * layout(set=0,binding=1) readonly buffer BBuffer{float b[];};
 * layout(set=0,binding=2) writeonly buffer CBuffer{float c[];};
 * layout(push_constant) uniform Push{uint m; uint n; uint k;} pc;
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

static void set_status(uint32_t* out_stage, int* out_detail_code, uint32_t stage, int detail) {
  if (out_stage != NULL) {
    *out_stage = stage;
  }
  if (out_detail_code != NULL) {
    *out_detail_code = detail;
  }
}

static int checked_mul_u32(uint32_t left, uint32_t right, uint32_t* out_value) {
  if (out_value == NULL) {
    return 0;
  }
  if (left != 0u && right > UINT32_MAX / left) {
    return 0;
  }
  *out_value = left * right;
  return 1;
}

static int checked_float_buffer_size(uint32_t rows, uint32_t cols, VkDeviceSize* out_vk_size, size_t* out_copy_size) {
  uint32_t elements;
  uint64_t bytes;

  if (out_vk_size == NULL || out_copy_size == NULL) {
    return 0;
  }
  if (!checked_mul_u32(rows, cols, &elements)) {
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

static uint32_t find_memory_type(VkPhysicalDevice physical_device, uint32_t type_filter, VkMemoryPropertyFlags properties) {
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

static VkResult create_host_visible_buffer(prometheus_runtime* rt, VkDeviceSize size, prom_vk_buffer* out_buffer) {
  VkResult result;
  VkBufferCreateInfo buffer_info;
  VkMemoryRequirements requirements;
  VkMemoryAllocateInfo alloc_info;
  uint32_t memory_type_index;

  if (rt == NULL || out_buffer == NULL) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }

  if ((rt->test_flags & PROM_TESTCFG_FAIL_BUFFER_ALLOC) != 0u) {
    return VK_ERROR_OUT_OF_DEVICE_MEMORY;
  }

  memset(out_buffer, 0, sizeof(*out_buffer));
  out_buffer->size = size;

  memset(&buffer_info, 0, sizeof(buffer_info));
  buffer_info.sType = VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO;
  buffer_info.size = size;
  buffer_info.usage = VK_BUFFER_USAGE_STORAGE_BUFFER_BIT;
  buffer_info.sharingMode = VK_SHARING_MODE_EXCLUSIVE;

  result = vkCreateBuffer(rt->device, &buffer_info, NULL, &out_buffer->buffer);
  if (result != VK_SUCCESS) {
    return result;
  }

  vkGetBufferMemoryRequirements(rt->device, out_buffer->buffer, &requirements);
  memory_type_index = find_memory_type(rt->physical_device,
                                       requirements.memoryTypeBits,
                                       VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT);
  if ((rt->test_flags & PROM_TESTCFG_FORCE_NO_MEMORY_TYPE) != 0u) {
    memory_type_index = UINT32_MAX;
  }
  if (memory_type_index == UINT32_MAX) {
    return VK_ERROR_FEATURE_NOT_PRESENT;
  }

  memset(&alloc_info, 0, sizeof(alloc_info));
  alloc_info.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO;
  alloc_info.allocationSize = requirements.size;
  alloc_info.memoryTypeIndex = memory_type_index;

  result = vkAllocateMemory(rt->device, &alloc_info, NULL, &out_buffer->memory);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = vkBindBufferMemory(rt->device, out_buffer->buffer, out_buffer->memory, 0);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = vkMapMemory(rt->device, out_buffer->memory, 0, size, 0, &out_buffer->mapped);
  return result;
}

static void destroy_buffer(prometheus_runtime* rt, prom_vk_buffer* buffer) {
  if (rt == NULL || buffer == NULL || rt->device == VK_NULL_HANDLE) {
    return;
  }
  if (buffer->mapped != NULL) {
    vkUnmapMemory(rt->device, buffer->memory);
    buffer->mapped = NULL;
  }
  if (buffer->buffer != VK_NULL_HANDLE) {
    vkDestroyBuffer(rt->device, buffer->buffer, NULL);
    buffer->buffer = VK_NULL_HANDLE;
  }
  if (buffer->memory != VK_NULL_HANDLE) {
    vkFreeMemory(rt->device, buffer->memory, NULL);
    buffer->memory = VK_NULL_HANDLE;
  }
}

static void destroy_reusable_execution_buffers(prometheus_runtime* rt) {
  if (rt == NULL) {
    return;
  }
  destroy_buffer(rt, &rt->reusable_c);
  destroy_buffer(rt, &rt->reusable_b);
  destroy_buffer(rt, &rt->reusable_a);
  rt->has_reusable_buffers = 0u;
  rt->descriptor_bindings_valid = 0u;
  rt->command_recording_valid = 0u;
  rt->reusable_m = 0u;
  rt->reusable_n = 0u;
  rt->reusable_k = 0u;
}

static int ensure_reusable_execution_buffers(prometheus_runtime* rt,
                                             uint32_t m,
                                             uint32_t n,
                                             uint32_t k,
                                             VkDeviceSize a_buffer_size,
                                             VkDeviceSize b_buffer_size,
                                             VkDeviceSize c_buffer_size,
                                             VkResult* out_result) {
  VkResult result;
  int shape_changed;

  if (out_result == NULL || rt == NULL) {
    return 0;
  }

  *out_result = VK_SUCCESS;
  shape_changed = (rt->has_reusable_buffers == 0u) || (rt->reusable_m != m) || (rt->reusable_n != n) || (rt->reusable_k != k);
  if (!shape_changed) {
    return 1;
  }

  destroy_reusable_execution_buffers(rt);

  result = create_host_visible_buffer(rt, a_buffer_size, &rt->reusable_a);
  if (result != VK_SUCCESS) {
    *out_result = result;
    return 0;
  }
  result = create_host_visible_buffer(rt, b_buffer_size, &rt->reusable_b);
  if (result != VK_SUCCESS) {
    *out_result = result;
    destroy_reusable_execution_buffers(rt);
    return 0;
  }
  result = create_host_visible_buffer(rt, c_buffer_size, &rt->reusable_c);
  if (result != VK_SUCCESS) {
    *out_result = result;
    destroy_reusable_execution_buffers(rt);
    return 0;
  }

  rt->reusable_m = m;
  rt->reusable_n = n;
  rt->reusable_k = k;
  rt->has_reusable_buffers = 1u;
  rt->descriptor_bindings_valid = 0u;
  rt->command_recording_valid = 0u;
  return 1;
}

static void vk_runtime_cleanup(prometheus_runtime* rt) {
  if (rt == NULL) {
    return;
  }
  if (rt->device != VK_NULL_HANDLE) {
    vkDeviceWaitIdle(rt->device);
  }
  destroy_reusable_execution_buffers(rt);
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
  if (rt->submit_fence != VK_NULL_HANDLE) {
    vkDestroyFence(rt->device, rt->submit_fence, NULL);
    rt->submit_fence = VK_NULL_HANDLE;
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
  VkDeviceQueueCreateInfo queue_info;
  VkDeviceCreateInfo device_info;
  float queue_priority = 1.0f;
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
  VkCommandBufferAllocateInfo cmd_alloc_info;
  VkFenceCreateInfo fence_info;

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
    VkPhysicalDeviceProperties props;
    vkGetPhysicalDeviceProperties(rt->physical_device, &props);
    if (props.deviceType == VK_PHYSICAL_DEVICE_TYPE_CPU || text_contains_llvmpipe(props.deviceName)) {
      rt->software_vulkan = 1u;
    } else {
      rt->software_vulkan = 0u;
    }
  }

  memset(&queue_info, 0, sizeof(queue_info));
  queue_info.sType = VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO;
  queue_info.queueFamilyIndex = rt->queue_family_index;
  queue_info.queueCount = 1u;
  queue_info.pQueuePriorities = &queue_priority;

  memset(&device_info, 0, sizeof(device_info));
  device_info.sType = VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO;
  device_info.queueCreateInfoCount = 1u;
  device_info.pQueueCreateInfos = &queue_info;

  if ((rt->test_flags & PROM_TESTCFG_FAIL_DEVICE_CREATE) != 0u) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }

  result = vkCreateDevice(rt->physical_device, &device_info, NULL, &rt->device);
  if (result != VK_SUCCESS) {
    return result;
  }

  vkGetDeviceQueue(rt->device, rt->queue_family_index, 0u, &rt->compute_queue);

  memset(&pool_info, 0, sizeof(pool_info));
  pool_info.sType = VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO;
  pool_info.flags = VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT;
  pool_info.queueFamilyIndex = rt->queue_family_index;
  result = vkCreateCommandPool(rt->device, &pool_info, NULL, &rt->command_pool);
  if (result != VK_SUCCESS) {
    return result;
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
  push_range.size = sizeof(prom_vk_push);

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

  if ((rt->test_flags & PROM_TESTCFG_FAIL_PIPELINE_CREATE) != 0u) {
    vkDestroyShaderModule(rt->device, shader_module, NULL);
    return VK_ERROR_INITIALIZATION_FAILED;
  }

  result = vkCreateComputePipelines(rt->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->pipeline);
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

  memset(&fence_info, 0, sizeof(fence_info));
  fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
  result = vkCreateFence(rt->device, &fence_info, NULL, &rt->submit_fence);
  return result;
}

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

  if (config != NULL) {
    const PrometheusReactorConfig* cfg = (const PrometheusReactorConfig*)config;
    if (cfg->struct_size >= sizeof(PrometheusReactorConfig)) {
      runtime->test_flags = cfg->test_flags;
    }
  }

  if ((runtime->test_flags & PROM_TESTCFG_SKIP_VULKAN_INIT) != 0u) {
    runtime->available = 0u;
    runtime->reason_code = PROM_REASON_VULKAN_UNAVAILABLE;
    runtime->init_detail_code = (int)VK_ERROR_INITIALIZATION_FAILED;
  } else {
    result = vk_runtime_init(runtime);
    if (result == VK_SUCCESS) {
      runtime->available = 1u;
      runtime->reason_code = PROM_REASON_NONE;
      runtime->init_detail_code = 0;
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

int prom_reactor_runtime_sgemm_impl(void* handle,
                                     const float* a,
                                     const float* b,
                                     float* c,
                                     uint32_t m,
                                     uint32_t n,
                                     uint32_t k,
                                     uint32_t* out_stage,
                                     int* out_detail_code) {
  prometheus_runtime* rt;
  VkResult vk_result;
  VkWriteDescriptorSet writes[3];
  VkDescriptorBufferInfo buffer_infos[3];
  VkCommandBufferBeginInfo begin_info;
  VkSubmitInfo submit_info;
  VkBufferMemoryBarrier barrier;
  prom_vk_push push;
  VkDeviceSize a_buffer_size;
  VkDeviceSize b_buffer_size;
  VkDeviceSize c_buffer_size;
  size_t a_copy_size;
  size_t b_copy_size;
  size_t c_copy_size;

  set_status(out_stage, out_detail_code, PROM_STAGE_NONE, 0);

  if (handle == NULL || !registry_contains(handle)) {
    set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }

  rt = (prometheus_runtime*)handle;
  if (a == NULL || b == NULL || c == NULL) {
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  if (m == 0u || n == 0u || k == 0u) {
    set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_ERROR);
    return PROM_ERROR;
  }
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  if (rt->available == 0u) {
    set_status(out_stage, out_detail_code, PROM_STAGE_INIT, rt->init_detail_code);
    return PROM_ERROR;
  }
  if (!checked_float_buffer_size(m, k, &a_buffer_size, &a_copy_size) ||
      !checked_float_buffer_size(k, n, &b_buffer_size, &b_copy_size) ||
      !checked_float_buffer_size(m, n, &c_buffer_size, &c_copy_size)) {
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_DETAIL_SIZE_OVERFLOW);
    return PROM_ERROR;
  }

  set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, 0);
  if (rt->in_flight_submit != 0u) {
    vk_result = vkGetFenceStatus(rt->device, rt->submit_fence);
    if (vk_result == VK_SUCCESS) {
      rt->in_flight_submit = 0u;
    } else {
      set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_REUSE_IN_FLIGHT);
      return PROM_ERROR;
    }
  }

  if (!ensure_reusable_execution_buffers(rt, m, n, k, a_buffer_size, b_buffer_size, c_buffer_size, &vk_result)) {
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, (int)vk_result);
    return PROM_ERROR;
  }

  if ((rt->test_flags & PROM_TESTCFG_FAIL_UPLOAD) != 0u) {
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_DETAIL_INJECTED_UPLOAD_FAILURE);
    return PROM_ERROR;
  }

  memcpy(rt->reusable_a.mapped, a, a_copy_size);
  memcpy(rt->reusable_b.mapped, b, b_copy_size);
  memset(rt->reusable_c.mapped, 0, c_copy_size);

  memset(buffer_infos, 0, sizeof(buffer_infos));
  buffer_infos[0].buffer = rt->reusable_a.buffer;
  buffer_infos[0].offset = 0;
  buffer_infos[0].range = rt->reusable_a.size;
  buffer_infos[1].buffer = rt->reusable_b.buffer;
  buffer_infos[1].offset = 0;
  buffer_infos[1].range = rt->reusable_b.size;
  buffer_infos[2].buffer = rt->reusable_c.buffer;
  buffer_infos[2].offset = 0;
  buffer_infos[2].range = rt->reusable_c.size;

  if (rt->descriptor_bindings_valid == 0u) {
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
    rt->descriptor_bindings_valid = 1u;
    rt->command_recording_valid = 0u;
  }

  if (rt->command_recording_valid == 0u) {
    vk_result = vkResetCommandBuffer(rt->command_buffer, 0u);
    if (vk_result != VK_SUCCESS) {
      set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }

    memset(&begin_info, 0, sizeof(begin_info));
    begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    vk_result = vkBeginCommandBuffer(rt->command_buffer, &begin_info);
    if (vk_result != VK_SUCCESS) {
      set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }

    memset(&barrier, 0, sizeof(barrier));
    barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barrier.srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
    barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
    barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;

    barrier.buffer = rt->reusable_a.buffer;
    barrier.offset = 0;
    barrier.size = rt->reusable_a.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         0,
                         0,
                         NULL,
                         1,
                         &barrier,
                         0,
                         NULL);

    barrier.buffer = rt->reusable_b.buffer;
    barrier.size = rt->reusable_b.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         0,
                         0,
                         NULL,
                         1,
                         &barrier,
                         0,
                         NULL);

    vkCmdBindPipeline(rt->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, rt->pipeline);
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
    push.k = k;
    push.reserved0 = 0u;
    vkCmdPushConstants(rt->command_buffer,
                       rt->pipeline_layout,
                       VK_SHADER_STAGE_COMPUTE_BIT,
                       0u,
                       sizeof(push),
                       &push);

    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, 0);
    if ((rt->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) {
      set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_INJECTED_DISPATCH_FAILURE);
      return PROM_ERROR;
    }

    vkCmdDispatch(rt->command_buffer,
                  (m + (PROM_VK_LOCAL_SIZE_X - 1u)) / PROM_VK_LOCAL_SIZE_X,
                  (n + (PROM_VK_LOCAL_SIZE_Y - 1u)) / PROM_VK_LOCAL_SIZE_Y,
                  1u);

    barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barrier.dstAccessMask = VK_ACCESS_HOST_READ_BIT;
    barrier.buffer = rt->reusable_c.buffer;
    barrier.size = rt->reusable_c.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         0,
                         0,
                         NULL,
                         1,
                         &barrier,
                         0,
                         NULL);

    vk_result = vkEndCommandBuffer(rt->command_buffer);
    if (vk_result != VK_SUCCESS) {
      set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    rt->command_recording_valid = 1u;
  }

  vk_result = vkResetFences(rt->device, 1u, &rt->submit_fence);
  if (vk_result != VK_SUCCESS) {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }

  memset(&submit_info, 0, sizeof(submit_info));
  submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit_info.commandBufferCount = 1u;
  submit_info.pCommandBuffers = &rt->command_buffer;
  vk_result = vkQueueSubmit(rt->compute_queue, 1u, &submit_info, rt->submit_fence);
  if (vk_result != VK_SUCCESS) {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }
  rt->in_flight_submit = 1u;

  if ((rt->test_flags & PROM_TESTCFG_SKIP_SUBMIT_WAIT) == 0u) {
    vk_result = vkWaitForFences(rt->device, 1u, &rt->submit_fence, VK_TRUE, UINT64_MAX);
    if (vk_result != VK_SUCCESS) {
      set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    rt->in_flight_submit = 0u;
  } else {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_REUSE_IN_FLIGHT);
    return PROM_ERROR;
  }

  if ((rt->test_flags & PROM_TESTCFG_FAIL_DOWNLOAD) != 0u) {
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_DETAIL_INJECTED_DOWNLOAD_FAILURE);
    return PROM_ERROR;
  }

  set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, 0);
  memcpy(c, rt->reusable_c.mapped, c_copy_size);

  if (out_stage != NULL && out_detail_code != NULL && *out_stage == PROM_STAGE_TRANSFER_OUT && *out_detail_code == 0) {
    return PROM_OK;
  }
  return PROM_ERROR;
}
