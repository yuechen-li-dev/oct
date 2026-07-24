#include "reactor_vulkan_runtime.h"

#include <stdlib.h>
#include <string.h>

static VKAPI_ATTR VkBool32 VKAPI_CALL prom_validation_callback(
    VkDebugUtilsMessageSeverityFlagBitsEXT severity,
    VkDebugUtilsMessageTypeFlagsEXT type,
    const VkDebugUtilsMessengerCallbackDataEXT* callback_data,
    void* user_data) {
  prom_vk_runtime* rt = (prom_vk_runtime*)user_data;
  if (rt == NULL) return VK_FALSE;
  rt->validation_message_count += 1u;
  rt->validation_last_severity = severity;
  rt->validation_last_type = type;
  if (callback_data != NULL) {
    strncpy(rt->validation_last_message_id, callback_data->pMessageIdName == NULL ? "" : callback_data->pMessageIdName, sizeof(rt->validation_last_message_id) - 1u);
    rt->validation_last_message_id[sizeof(rt->validation_last_message_id) - 1u] = '\0';
    strncpy(rt->validation_last_message, callback_data->pMessage == NULL ? "" : callback_data->pMessage, sizeof(rt->validation_last_message) - 1u);
    rt->validation_last_message[sizeof(rt->validation_last_message) - 1u] = '\0';
  }
  if ((severity & VK_DEBUG_UTILS_MESSAGE_SEVERITY_WARNING_BIT_EXT) != 0u) rt->validation_warning_count += 1u;
  if ((severity & VK_DEBUG_UTILS_MESSAGE_SEVERITY_ERROR_BIT_EXT) != 0u) rt->validation_error_count += 1u;
  return VK_FALSE;
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

static uint32_t classify_capability_bucket(uint32_t value, uint32_t t1, uint32_t t2, uint32_t t3, uint32_t t4) {
  if (value <= t1) return 1u;
  if (value <= t2) return 2u;
  if (value <= t3) return 3u;
  if (value <= t4) return 4u;
  return 5u;
}

VkResult prom_vk_runtime_init(prom_vk_runtime* rt, uint32_t test_flags) {
  VkResult result;
  VkApplicationInfo application_info;
  VkInstanceCreateInfo instance_info;
  uint32_t loader_api_version = VK_API_VERSION_1_0;
  uint32_t device_count = 0u;
  VkPhysicalDevice devices[16];
  uint32_t i;
  VkDeviceQueueCreateInfo queue_infos[2];
  VkDeviceCreateInfo device_info;
  float queue_priorities[8];
  VkCommandPoolCreateInfo pool_info;
  const char* validation_layer = "VK_LAYER_KHRONOS_validation";
  const char* debug_extension = VK_EXT_DEBUG_UTILS_EXTENSION_NAME;
  uint32_t layer_count = 0u;
  VkLayerProperties layers[64];
#ifdef VK_KHR_cooperative_matrix
  VkPhysicalDeviceFeatures2 cooperative_features2;
  VkPhysicalDeviceShaderFloat16Int8Features shader_float16_features;
  VkPhysicalDeviceVulkanMemoryModelFeatures vulkan_memory_model_features;
  VkPhysicalDeviceCooperativeMatrixFeaturesKHR cooperative_features;
  VkPhysicalDeviceShaderFloat16Int8Features shader_float16_enable;
  VkPhysicalDeviceVulkanMemoryModelFeatures vulkan_memory_model_enable;
  VkPhysicalDeviceCooperativeMatrixFeaturesKHR cooperative_enable;
  const char* cooperative_device_extensions[1];
  uint32_t cooperative_device_extension_count = 0u;
#endif
#if defined(VK_KHR_acceleration_structure) && defined(VK_KHR_ray_query)
  VkPhysicalDeviceFeatures2 ray_query_features2;
  VkPhysicalDeviceBufferDeviceAddressFeatures ray_query_buffer_device_address_features;
  VkPhysicalDeviceAccelerationStructureFeaturesKHR ray_query_acceleration_structure_features;
  VkPhysicalDeviceRayQueryFeaturesKHR ray_query_features;
  VkPhysicalDeviceBufferDeviceAddressFeatures ray_query_buffer_device_address_enable;
  VkPhysicalDeviceAccelerationStructureFeaturesKHR ray_query_acceleration_structure_enable;
  VkPhysicalDeviceRayQueryFeaturesKHR ray_query_enable;
  const char* ray_query_device_extensions[3];
  uint32_t ray_query_device_extension_count = 0u;
#endif
  const char* optional_device_extensions[4];
  uint32_t optional_device_extension_count = 0u;
  void* optional_device_feature_chain = NULL;

  if (rt == NULL) return VK_ERROR_INITIALIZATION_FAILED;
  rt->test_flags = test_flags;
  for (i = 0u; i < 8u; ++i) {
    queue_priorities[i] = 1.0f;
  }

  memset(&application_info, 0, sizeof(application_info));
  application_info.sType = VK_STRUCTURE_TYPE_APPLICATION_INFO;
  application_info.pApplicationName = "Prometheus";
  application_info.applicationVersion = 1u;
  application_info.pEngineName = "Prometheus";
  application_info.engineVersion = 1u;
#ifdef VK_VERSION_1_1
  {
    PFN_vkEnumerateInstanceVersion enumerate_instance_version =
      (PFN_vkEnumerateInstanceVersion)vkGetInstanceProcAddr(VK_NULL_HANDLE, "vkEnumerateInstanceVersion");
    if (enumerate_instance_version != NULL) (void)enumerate_instance_version(&loader_api_version);
  }
#endif
  /* Production shader artifacts are SPIR-V 1.6 validated under Vulkan 1.4.
     Do not silently lower the runtime contract to the loader's older API. */
  if (loader_api_version < VK_API_VERSION_1_4) {
    return VK_ERROR_INCOMPATIBLE_DRIVER;
  }
  application_info.apiVersion = VK_API_VERSION_1_4;
  memset(&instance_info, 0, sizeof(instance_info));
  instance_info.sType = VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO;
  instance_info.pApplicationInfo = &application_info;
  rt->validation_requested = getenv("PROMETHEUS_VK_VALIDATION") != NULL && strcmp(getenv("PROMETHEUS_VK_VALIDATION"), "1") == 0 ? 1u : 0u;
  if (rt->validation_requested != 0u) {
    result = vkEnumerateInstanceLayerProperties(&layer_count, NULL);
    if (result != VK_SUCCESS) return result;
    if (layer_count > 64u) layer_count = 64u;
    result = vkEnumerateInstanceLayerProperties(&layer_count, layers);
    if (result != VK_SUCCESS) return result;
    for (i = 0u; i < layer_count; ++i) if (strcmp(layers[i].layerName, validation_layer) == 0) rt->validation_available = 1u;
    if (rt->validation_available == 0u) return VK_ERROR_LAYER_NOT_PRESENT;
    instance_info.enabledLayerCount = 1u;
    instance_info.ppEnabledLayerNames = &validation_layer;
    instance_info.enabledExtensionCount = 1u;
    instance_info.ppEnabledExtensionNames = &debug_extension;
  }
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
    VkPhysicalDeviceProperties platform_properties;
    vkGetPhysicalDeviceProperties(rt->physical_device, &platform_properties);
    if (platform_properties.apiVersion < VK_API_VERSION_1_4) {
      return VK_ERROR_INCOMPATIBLE_DRIVER;
    }
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
  rt->cooperative_matrix_state = PROM_VK_COOPERATIVE_MATRIX_UNAVAILABLE;
  rt->cooperative_matrix_extension_spec_version = 0u;
  rt->cooperative_matrix_feature_enabled = 0u;
  rt->cooperative_matrix_shader_float16_enabled = 0u;
  rt->cooperative_matrix_vulkan_memory_model_enabled = 0u;
  rt->cooperative_matrix_tuple_count = 0u;
  rt->cooperative_matrix_selected_m = 0u;
  rt->cooperative_matrix_selected_n = 0u;
  rt->cooperative_matrix_selected_k = 0u;
  rt->subgroup_size = 0u;
  rt->subgroup_supported_stages = 0u;
  rt->subgroup_supported_operations = 0u;
  rt->subgroup_compute_supported = 0u;
  rt->subgroup_arithmetic_supported = 0u;
  rt->subgroup_basic_supported = 0u;
  rt->subgroup_shuffle_supported = 0u;
  rt->subgroup_fixed_size_32_admitted = 0u;
  rt->subgroup_owned_attention_admitted = 0u;
  rt->subgroup_owned_attention_topology_proven = 0u;
  rt->ray_query_state = PROM_VK_RAY_QUERY_UNSUPPORTED;
  rt->ray_query_acceleration_structure_extension_supported = 0u;
  rt->ray_query_extension_supported = 0u;
  rt->ray_query_deferred_host_operations_extension_supported = 0u;
  rt->ray_query_buffer_device_address_supported = 0u;
  rt->ray_query_acceleration_structure_supported = 0u;
  rt->ray_query_supported = 0u;
  {
    VkPhysicalDeviceSubgroupProperties subgroup_properties;
    VkPhysicalDeviceProperties2 properties2;
    memset(&subgroup_properties, 0, sizeof(subgroup_properties));
    subgroup_properties.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_SUBGROUP_PROPERTIES;
    memset(&properties2, 0, sizeof(properties2));
    properties2.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_PROPERTIES_2;
    properties2.pNext = &subgroup_properties;
    vkGetPhysicalDeviceProperties2(rt->physical_device, &properties2);
    rt->subgroup_size = subgroup_properties.subgroupSize;
    rt->subgroup_supported_stages = (uint32_t)subgroup_properties.supportedStages;
    rt->subgroup_supported_operations = (uint32_t)subgroup_properties.supportedOperations;
    rt->subgroup_compute_supported =
        (subgroup_properties.supportedStages & VK_SHADER_STAGE_COMPUTE_BIT) != 0u ? 1u : 0u;
    rt->subgroup_arithmetic_supported =
        (subgroup_properties.supportedOperations & VK_SUBGROUP_FEATURE_ARITHMETIC_BIT) != 0u ? 1u : 0u;
    rt->subgroup_basic_supported =
        (subgroup_properties.supportedOperations & VK_SUBGROUP_FEATURE_BASIC_BIT) != 0u ? 1u : 0u;
    rt->subgroup_shuffle_supported =
        (subgroup_properties.supportedOperations & VK_SUBGROUP_FEATURE_SHUFFLE_BIT) != 0u ? 1u : 0u;
    rt->subgroup_fixed_size_32_admitted =
        (rt->subgroup_size == 32u && rt->subgroup_compute_supported != 0u &&
         rt->subgroup_arithmetic_supported != 0u && rt->subgroup_basic_supported != 0u) ? 1u : 0u;
    rt->subgroup_owned_attention_admitted =
        (rt->subgroup_fixed_size_32_admitted != 0u && rt->subgroup_shuffle_supported != 0u) ? 1u : 0u;
    /* This remains false until the bounded 256-invocation compute/readback
       proof has completed for this physical-device/driver session.  It must
       never be inferred from subgroup size or advertised capabilities. */
    rt->subgroup_owned_attention_topology_proven = 0u;
  }

#ifdef VK_KHR_cooperative_matrix
  {
  if (application_info.apiVersion >= VK_API_VERSION_1_1) {
    uint32_t extension_count = 0u;
    VkExtensionProperties* extensions = NULL;
    uint32_t extension_index;
    VkPhysicalDeviceProperties physical_properties;
    VkPhysicalDeviceSubgroupProperties subgroup_properties;
    VkPhysicalDeviceProperties2 properties2;
    PFN_vkGetPhysicalDeviceCooperativeMatrixPropertiesKHR get_properties = NULL;
    uint32_t tuple_count = 0u;
    VkCooperativeMatrixPropertiesKHR tuples[64];
    const char* disabled = getenv("PROMETHEUS_VK_DISABLE_COOPERATIVE_MATRIX");
    vkGetPhysicalDeviceProperties(rt->physical_device, &physical_properties);
    memset(&subgroup_properties, 0, sizeof(subgroup_properties));
    subgroup_properties.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_SUBGROUP_PROPERTIES;
    memset(&properties2, 0, sizeof(properties2));
    properties2.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_PROPERTIES_2;
    properties2.pNext = &subgroup_properties;
    vkGetPhysicalDeviceProperties2(rt->physical_device, &properties2);
    rt->subgroup_size = subgroup_properties.subgroupSize;
    if (disabled == NULL || strcmp(disabled, "1") != 0) {
      result = vkEnumerateDeviceExtensionProperties(rt->physical_device, NULL, &extension_count, NULL);
      if (result == VK_SUCCESS && extension_count != 0u) {
        extensions = (VkExtensionProperties*)calloc(extension_count, sizeof(*extensions));
        if (extensions == NULL) return VK_ERROR_OUT_OF_HOST_MEMORY;
        result = vkEnumerateDeviceExtensionProperties(rt->physical_device, NULL, &extension_count, extensions);
      }
      if (result == VK_SUCCESS) {
        for (extension_index = 0u; extension_index < extension_count; ++extension_index) {
          if (strcmp(extensions[extension_index].extensionName, VK_KHR_COOPERATIVE_MATRIX_EXTENSION_NAME) == 0) {
            rt->cooperative_matrix_extension_spec_version = extensions[extension_index].specVersion;
            break;
          }
        }
      }
      free(extensions);
    }
    memset(&cooperative_features2, 0, sizeof(cooperative_features2));
    memset(&shader_float16_features, 0, sizeof(shader_float16_features));
    memset(&vulkan_memory_model_features, 0, sizeof(vulkan_memory_model_features));
    memset(&cooperative_features, 0, sizeof(cooperative_features));
    cooperative_features2.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_FEATURES_2;
    shader_float16_features.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_SHADER_FLOAT16_INT8_FEATURES;
    vulkan_memory_model_features.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_VULKAN_MEMORY_MODEL_FEATURES;
    cooperative_features.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_COOPERATIVE_MATRIX_FEATURES_KHR;
    cooperative_features2.pNext = &shader_float16_features;
    shader_float16_features.pNext = &vulkan_memory_model_features;
    vulkan_memory_model_features.pNext = &cooperative_features;
    if (rt->cooperative_matrix_extension_spec_version != 0u) {
      rt->cooperative_matrix_state = PROM_VK_COOPERATIVE_MATRIX_EXTENSION_NO_USEFUL_TUPLE;
      vkGetPhysicalDeviceFeatures2(rt->physical_device, &cooperative_features2);
      get_properties = (PFN_vkGetPhysicalDeviceCooperativeMatrixPropertiesKHR)
        vkGetInstanceProcAddr(rt->instance, "vkGetPhysicalDeviceCooperativeMatrixPropertiesKHR");
      if (get_properties != NULL && get_properties(rt->physical_device, &tuple_count, NULL) == VK_SUCCESS) {
        if (tuple_count > 64u) tuple_count = 64u;
        memset(tuples, 0, sizeof(tuples));
        for (i = 0u; i < tuple_count; ++i) tuples[i].sType = VK_STRUCTURE_TYPE_COOPERATIVE_MATRIX_PROPERTIES_KHR;
        if (get_properties(rt->physical_device, &tuple_count, tuples) != VK_SUCCESS) tuple_count = 0u;
      }
      rt->cooperative_matrix_tuple_count = tuple_count;
      for (i = 0u; i < tuple_count; ++i) {
        if (tuples[i].scope == VK_SCOPE_SUBGROUP_KHR && tuples[i].MSize == 16u && tuples[i].NSize == 16u &&
          tuples[i].KSize == 16u && tuples[i].AType == VK_COMPONENT_TYPE_FLOAT16_KHR &&
          tuples[i].BType == VK_COMPONENT_TYPE_FLOAT16_KHR && tuples[i].CType == VK_COMPONENT_TYPE_FLOAT32_KHR &&
          tuples[i].ResultType == VK_COMPONENT_TYPE_FLOAT32_KHR) {
          rt->cooperative_matrix_selected_m = 16u;
          rt->cooperative_matrix_selected_n = 16u;
          rt->cooperative_matrix_selected_k = 16u;
          rt->cooperative_matrix_state = PROM_VK_COOPERATIVE_MATRIX_USEFUL_TUPLE_AVAILABLE;
          break;
        }
      }
      if (rt->cooperative_matrix_state == PROM_VK_COOPERATIVE_MATRIX_USEFUL_TUPLE_AVAILABLE &&
        cooperative_features.cooperativeMatrix == VK_TRUE && shader_float16_features.shaderFloat16 == VK_TRUE &&
        vulkan_memory_model_features.vulkanMemoryModel == VK_TRUE &&
        physical_properties.apiVersion >= VK_API_VERSION_1_3) {
        cooperative_device_extensions[0] = VK_KHR_COOPERATIVE_MATRIX_EXTENSION_NAME;
        cooperative_device_extension_count = 1u;
      }
    }
  }
  }
#endif

#if defined(VK_KHR_acceleration_structure) && defined(VK_KHR_ray_query)
  {
    uint32_t extension_count = 0u;
    uint32_t extension_index;
    VkExtensionProperties* extensions = NULL;
    result = vkEnumerateDeviceExtensionProperties(rt->physical_device, NULL, &extension_count, NULL);
    if (result == VK_SUCCESS && extension_count != 0u) {
      extensions = (VkExtensionProperties*)calloc(extension_count, sizeof(*extensions));
      if (extensions == NULL) return VK_ERROR_OUT_OF_HOST_MEMORY;
      result = vkEnumerateDeviceExtensionProperties(rt->physical_device, NULL, &extension_count, extensions);
    }
    if (result != VK_SUCCESS) {
      free(extensions);
      return result;
    }
    for (extension_index = 0u; extension_index < extension_count; ++extension_index) {
      const char* name = extensions[extension_index].extensionName;
      if (strcmp(name, VK_KHR_ACCELERATION_STRUCTURE_EXTENSION_NAME) == 0) {
        rt->ray_query_acceleration_structure_extension_supported = 1u;
      } else if (strcmp(name, VK_KHR_RAY_QUERY_EXTENSION_NAME) == 0) {
        rt->ray_query_extension_supported = 1u;
      } else if (strcmp(name, VK_KHR_DEFERRED_HOST_OPERATIONS_EXTENSION_NAME) == 0) {
        rt->ray_query_deferred_host_operations_extension_supported = 1u;
      }
    }
    free(extensions);
    if (rt->ray_query_acceleration_structure_extension_supported == 0u ||
        rt->ray_query_extension_supported == 0u ||
        rt->ray_query_deferred_host_operations_extension_supported == 0u) {
      rt->ray_query_state = PROM_VK_RAY_QUERY_EXTENSION_MISSING;
    } else {
      memset(&ray_query_features2, 0, sizeof(ray_query_features2));
      memset(&ray_query_buffer_device_address_features, 0, sizeof(ray_query_buffer_device_address_features));
      memset(&ray_query_acceleration_structure_features, 0, sizeof(ray_query_acceleration_structure_features));
      memset(&ray_query_features, 0, sizeof(ray_query_features));
      ray_query_features2.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_FEATURES_2;
      ray_query_buffer_device_address_features.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_BUFFER_DEVICE_ADDRESS_FEATURES;
      ray_query_acceleration_structure_features.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_ACCELERATION_STRUCTURE_FEATURES_KHR;
      ray_query_features.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_RAY_QUERY_FEATURES_KHR;
      ray_query_features2.pNext = &ray_query_buffer_device_address_features;
      ray_query_buffer_device_address_features.pNext = &ray_query_acceleration_structure_features;
      ray_query_acceleration_structure_features.pNext = &ray_query_features;
      vkGetPhysicalDeviceFeatures2(rt->physical_device, &ray_query_features2);
      rt->ray_query_buffer_device_address_supported = ray_query_buffer_device_address_features.bufferDeviceAddress == VK_TRUE ? 1u : 0u;
      rt->ray_query_acceleration_structure_supported = ray_query_acceleration_structure_features.accelerationStructure == VK_TRUE ? 1u : 0u;
      rt->ray_query_supported = ray_query_features.rayQuery == VK_TRUE ? 1u : 0u;
      if (rt->ray_query_buffer_device_address_supported == 0u || rt->ray_query_acceleration_structure_supported == 0u || rt->ray_query_supported == 0u) {
        rt->ray_query_state = PROM_VK_RAY_QUERY_FEATURE_MISSING;
      } else {
        ray_query_device_extensions[ray_query_device_extension_count++] = VK_KHR_DEFERRED_HOST_OPERATIONS_EXTENSION_NAME;
        ray_query_device_extensions[ray_query_device_extension_count++] = VK_KHR_ACCELERATION_STRUCTURE_EXTENSION_NAME;
        ray_query_device_extensions[ray_query_device_extension_count++] = VK_KHR_RAY_QUERY_EXTENSION_NAME;
      }
    }
  }
#endif

  memset(queue_infos, 0, sizeof(queue_infos));
  queue_infos[0].sType = VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO;
  queue_infos[0].queueFamilyIndex = rt->queue_family_index;
  queue_infos[0].queueCount = 1u;
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
#ifdef VK_KHR_cooperative_matrix
  if (cooperative_device_extension_count != 0u) {
    memset(&shader_float16_enable, 0, sizeof(shader_float16_enable));
    memset(&vulkan_memory_model_enable, 0, sizeof(vulkan_memory_model_enable));
    memset(&cooperative_enable, 0, sizeof(cooperative_enable));
    shader_float16_enable.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_SHADER_FLOAT16_INT8_FEATURES;
    shader_float16_enable.shaderFloat16 = VK_TRUE;
    vulkan_memory_model_enable.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_VULKAN_MEMORY_MODEL_FEATURES;
    vulkan_memory_model_enable.vulkanMemoryModel = VK_TRUE;
    cooperative_enable.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_COOPERATIVE_MATRIX_FEATURES_KHR;
    cooperative_enable.cooperativeMatrix = VK_TRUE;
    shader_float16_enable.pNext = &vulkan_memory_model_enable;
    vulkan_memory_model_enable.pNext = &cooperative_enable;
    cooperative_enable.pNext = optional_device_feature_chain;
    optional_device_feature_chain = &shader_float16_enable;
    for (i = 0u; i < cooperative_device_extension_count; ++i) {
      optional_device_extensions[optional_device_extension_count++] = cooperative_device_extensions[i];
    }
  }
#endif
#if defined(VK_KHR_acceleration_structure) && defined(VK_KHR_ray_query)
  if (ray_query_device_extension_count != 0u) {
    memset(&ray_query_buffer_device_address_enable, 0, sizeof(ray_query_buffer_device_address_enable));
    memset(&ray_query_acceleration_structure_enable, 0, sizeof(ray_query_acceleration_structure_enable));
    memset(&ray_query_enable, 0, sizeof(ray_query_enable));
    ray_query_buffer_device_address_enable.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_BUFFER_DEVICE_ADDRESS_FEATURES;
    ray_query_buffer_device_address_enable.bufferDeviceAddress = VK_TRUE;
    ray_query_acceleration_structure_enable.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_ACCELERATION_STRUCTURE_FEATURES_KHR;
    ray_query_acceleration_structure_enable.accelerationStructure = VK_TRUE;
    ray_query_enable.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_RAY_QUERY_FEATURES_KHR;
    ray_query_enable.rayQuery = VK_TRUE;
    ray_query_enable.pNext = optional_device_feature_chain;
    ray_query_acceleration_structure_enable.pNext = &ray_query_enable;
    ray_query_buffer_device_address_enable.pNext = &ray_query_acceleration_structure_enable;
    optional_device_feature_chain = &ray_query_buffer_device_address_enable;
    for (i = 0u; i < ray_query_device_extension_count; ++i) {
      optional_device_extensions[optional_device_extension_count++] = ray_query_device_extensions[i];
    }
  }
#endif
  device_info.pNext = optional_device_feature_chain;
  device_info.enabledExtensionCount = optional_device_extension_count;
  device_info.ppEnabledExtensionNames = optional_device_extension_count == 0u ? NULL : optional_device_extensions;

  if ((rt->test_flags & PROM_TESTCFG_FAIL_DEVICE_CREATE) != 0u) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }

  result = vkCreateDevice(rt->physical_device, &device_info, NULL, &rt->device);
  if (result != VK_SUCCESS) {
    return result;
  }
#if defined(VK_KHR_acceleration_structure) && defined(VK_KHR_ray_query)
  if (ray_query_device_extension_count != 0u) {
    rt->create_acceleration_structure = (PFN_vkCreateAccelerationStructureKHR)vkGetDeviceProcAddr(rt->device, "vkCreateAccelerationStructureKHR");
    rt->destroy_acceleration_structure = (PFN_vkDestroyAccelerationStructureKHR)vkGetDeviceProcAddr(rt->device, "vkDestroyAccelerationStructureKHR");
    rt->get_acceleration_structure_build_sizes = (PFN_vkGetAccelerationStructureBuildSizesKHR)vkGetDeviceProcAddr(rt->device, "vkGetAccelerationStructureBuildSizesKHR");
    rt->cmd_build_acceleration_structures = (PFN_vkCmdBuildAccelerationStructuresKHR)vkGetDeviceProcAddr(rt->device, "vkCmdBuildAccelerationStructuresKHR");
    rt->get_acceleration_structure_device_address = (PFN_vkGetAccelerationStructureDeviceAddressKHR)vkGetDeviceProcAddr(rt->device, "vkGetAccelerationStructureDeviceAddressKHR");
    if (rt->create_acceleration_structure == NULL || rt->destroy_acceleration_structure == NULL ||
        rt->get_acceleration_structure_build_sizes == NULL || rt->cmd_build_acceleration_structures == NULL ||
        rt->get_acceleration_structure_device_address == NULL) {
      rt->ray_query_state = PROM_VK_RAY_QUERY_ENTRY_POINT_MISSING;
    } else {
      rt->ray_query_state = PROM_VK_RAY_QUERY_DEVICE_FEATURE_ENABLED;
    }
  }
#endif

  vkGetDeviceQueue(rt->device, rt->queue_family_index, 0u, &rt->compute_queue);
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


  return VK_SUCCESS;
}

VkResult prom_vk_runtime_enable_validation(prom_vk_runtime* rt) {
  VkDebugUtilsMessengerCreateInfoEXT debug_info;
  PFN_vkCreateDebugUtilsMessengerEXT create_debug_messenger;
  VkResult result;
  if (rt == NULL || rt->validation_requested == 0u) return VK_SUCCESS;
  create_debug_messenger = (PFN_vkCreateDebugUtilsMessengerEXT)
      vkGetInstanceProcAddr(rt->instance, "vkCreateDebugUtilsMessengerEXT");
  if (create_debug_messenger == NULL) return VK_ERROR_EXTENSION_NOT_PRESENT;
  memset(&debug_info, 0, sizeof(debug_info));
  debug_info.sType = VK_STRUCTURE_TYPE_DEBUG_UTILS_MESSENGER_CREATE_INFO_EXT;
  debug_info.messageSeverity = VK_DEBUG_UTILS_MESSAGE_SEVERITY_INFO_BIT_EXT |
      VK_DEBUG_UTILS_MESSAGE_SEVERITY_WARNING_BIT_EXT |
      VK_DEBUG_UTILS_MESSAGE_SEVERITY_ERROR_BIT_EXT;
  debug_info.messageType = VK_DEBUG_UTILS_MESSAGE_TYPE_GENERAL_BIT_EXT |
      VK_DEBUG_UTILS_MESSAGE_TYPE_VALIDATION_BIT_EXT |
      VK_DEBUG_UTILS_MESSAGE_TYPE_PERFORMANCE_BIT_EXT;
  debug_info.pfnUserCallback = prom_validation_callback;
  debug_info.pUserData = rt;
  result = create_debug_messenger(rt->instance, &debug_info, NULL,
                                  &rt->validation_debug_messenger);
  if (result != VK_SUCCESS) return result;
  rt->validation_enabled = 1u;
  rt->validation_debug_utils_active = 1u;
  return VK_SUCCESS;
}

void prom_vk_runtime_wait_idle(prom_vk_runtime* rt) {
  if (rt != NULL && rt->device != VK_NULL_HANDLE) {
    (void)vkDeviceWaitIdle(rt->device);
  }
}

void prom_vk_runtime_cleanup(prom_vk_runtime* rt) {
  PFN_vkDestroyDebugUtilsMessengerEXT destroy_debug_messenger;
  if (rt == NULL) return;
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
    destroy_debug_messenger = (PFN_vkDestroyDebugUtilsMessengerEXT)
        vkGetInstanceProcAddr(rt->instance, "vkDestroyDebugUtilsMessengerEXT");
    if (rt->validation_debug_messenger != VK_NULL_HANDLE && destroy_debug_messenger != NULL) {
      destroy_debug_messenger(rt->instance, rt->validation_debug_messenger, NULL);
    }
    rt->validation_debug_messenger = VK_NULL_HANDLE;
    vkDestroyInstance(rt->instance, NULL);
    rt->instance = VK_NULL_HANDLE;
  }
}

void prom_vk_runtime_destroy_package(prom_vk_runtime* rt) {
  if (rt == NULL) return;
  prom_shader_package_destroy(rt->shader_package);
  rt->shader_package = NULL;
  free(rt->shader_package_root);
  rt->shader_package_root = NULL;
}
