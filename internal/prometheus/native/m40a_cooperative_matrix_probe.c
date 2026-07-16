/*
 * M40a read-only capability probe. This is intentionally a standalone audit
 * executable, not part of the Prometheus production library.
 */
#include <vulkan/vulkan.h>

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifndef VK_KHR_cooperative_matrix
int main(void) {
  puts("{\"schema\":\"prometheus.m40a.cooperative-matrix-probe.v1\",\"header_support\":false}");
  return 0;
}
#else

static const char* component_name(VkComponentTypeKHR value) {
  switch (value) {
    case VK_COMPONENT_TYPE_FLOAT16_KHR: return "float16";
    case VK_COMPONENT_TYPE_FLOAT32_KHR: return "float32";
    case VK_COMPONENT_TYPE_FLOAT64_KHR: return "float64";
    case VK_COMPONENT_TYPE_SINT8_KHR: return "sint8";
    case VK_COMPONENT_TYPE_SINT16_KHR: return "sint16";
    case VK_COMPONENT_TYPE_SINT32_KHR: return "sint32";
    case VK_COMPONENT_TYPE_SINT64_KHR: return "sint64";
    case VK_COMPONENT_TYPE_UINT8_KHR: return "uint8";
    case VK_COMPONENT_TYPE_UINT16_KHR: return "uint16";
    case VK_COMPONENT_TYPE_UINT32_KHR: return "uint32";
    case VK_COMPONENT_TYPE_UINT64_KHR: return "uint64";
    case VK_COMPONENT_TYPE_BFLOAT16_KHR: return "bfloat16";
    case VK_COMPONENT_TYPE_SINT8_PACKED_NV: return "sint8_packed";
    case VK_COMPONENT_TYPE_UINT8_PACKED_NV: return "uint8_packed";
    default: return "unknown";
  }
}

static const char* scope_name(VkScopeKHR value) {
  switch (value) {
    case VK_SCOPE_DEVICE_KHR: return "device";
    case VK_SCOPE_WORKGROUP_KHR: return "workgroup";
    case VK_SCOPE_SUBGROUP_KHR: return "subgroup";
    case VK_SCOPE_QUEUE_FAMILY_KHR: return "queue_family";
    default: return "unknown";
  }
}

static const char* usefulness(const VkCooperativeMatrixPropertiesKHR* tuple) {
  if (tuple->AType == VK_COMPONENT_TYPE_FLOAT16_KHR &&
      tuple->BType == VK_COMPONENT_TYPE_FLOAT16_KHR &&
      tuple->CType == VK_COMPONENT_TYPE_FLOAT32_KHR &&
      tuple->ResultType == VK_COMPONENT_TYPE_FLOAT32_KHR) {
    return "fp16-inference-fp32-accumulation";
  }
  if (tuple->AType == VK_COMPONENT_TYPE_BFLOAT16_KHR &&
      tuple->BType == VK_COMPONENT_TYPE_BFLOAT16_KHR) {
    return "bf16-like-inference";
  }
  if (tuple->AType == VK_COMPONENT_TYPE_FLOAT16_KHR &&
      tuple->BType == VK_COMPONENT_TYPE_FLOAT16_KHR) {
    return "fp16-like-inference";
  }
  if (tuple->AType == VK_COMPONENT_TYPE_SINT8_KHR ||
      tuple->AType == VK_COMPONENT_TYPE_UINT8_KHR) {
    return "integer";
  }
  return "unsupported-by-current-sdslv-scalar-types";
}

static uint32_t extension_spec(const VkExtensionProperties* extensions, uint32_t count, const char* name) {
  uint32_t i;
  for (i = 0u; i < count; ++i) {
    if (strcmp(extensions[i].extensionName, name) == 0) return extensions[i].specVersion;
  }
  return 0u;
}

static int tuple_compare(const void* left, const void* right) {
  const VkCooperativeMatrixPropertiesKHR* a = (const VkCooperativeMatrixPropertiesKHR*)left;
  const VkCooperativeMatrixPropertiesKHR* b = (const VkCooperativeMatrixPropertiesKHR*)right;
#define CMP(FIELD) do { if (a->FIELD < b->FIELD) return -1; if (a->FIELD > b->FIELD) return 1; } while (0)
  CMP(scope); CMP(MSize); CMP(NSize); CMP(KSize); CMP(AType); CMP(BType); CMP(CType); CMP(ResultType);
  CMP(saturatingAccumulation);
#undef CMP
  return 0;
}

static void json_string(const char* text) {
  const unsigned char* p = (const unsigned char*)text;
  putchar('"');
  while (*p != 0u) {
    if (*p == '"' || *p == '\\') putchar('\\');
    if (*p >= 0x20u) putchar((int)*p);
    ++p;
  }
  putchar('"');
}

int main(void) {
  uint32_t loader_api = VK_API_VERSION_1_0;
  VkApplicationInfo app = {VK_STRUCTURE_TYPE_APPLICATION_INFO};
  VkInstanceCreateInfo create = {VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO};
  VkInstance instance = VK_NULL_HANDLE;
  VkPhysicalDevice devices[32];
  VkPhysicalDevice physical = VK_NULL_HANDLE;
  uint32_t device_count = 0u;
  VkPhysicalDeviceProperties properties;
  VkPhysicalDeviceDriverProperties driver = {VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_DRIVER_PROPERTIES};
  VkPhysicalDeviceSubgroupProperties subgroup = {VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_SUBGROUP_PROPERTIES};
  VkPhysicalDeviceCooperativeMatrixPropertiesKHR cooperative = {VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_COOPERATIVE_MATRIX_PROPERTIES_KHR};
  VkPhysicalDeviceCooperativeMatrix2PropertiesNV cooperative2 = {VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_COOPERATIVE_MATRIX_2_PROPERTIES_NV};
  VkPhysicalDeviceProperties2 properties2 = {VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_PROPERTIES_2};
  VkPhysicalDeviceShaderFloat16Int8Features float16 = {VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_SHADER_FLOAT16_INT8_FEATURES};
  VkPhysicalDeviceVulkanMemoryModelFeatures memory_model = {VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_VULKAN_MEMORY_MODEL_FEATURES};
  VkPhysicalDeviceCooperativeMatrixFeaturesKHR khr = {VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_COOPERATIVE_MATRIX_FEATURES_KHR};
  VkPhysicalDeviceCooperativeMatrixFeaturesNV nv = {VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_COOPERATIVE_MATRIX_FEATURES_NV};
  VkPhysicalDeviceCooperativeMatrix2FeaturesNV nv2 = {VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_COOPERATIVE_MATRIX_2_FEATURES_NV};
  VkPhysicalDeviceFeatures2 features2 = {VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_FEATURES_2};
  VkExtensionProperties* extensions = NULL;
  VkCooperativeMatrixPropertiesKHR* tuples = NULL;
  uint32_t extension_count = 0u, tuple_count = 0u, nv_count = 0u, flexible_count = 0u;
  uint32_t khr_spec = 0u, nv_spec = 0u, nv2_spec = 0u, i;
  PFN_vkGetPhysicalDeviceCooperativeMatrixPropertiesKHR get_khr;
  PFN_vkGetPhysicalDeviceCooperativeMatrixPropertiesNV get_nv;
  PFN_vkGetPhysicalDeviceCooperativeMatrixFlexibleDimensionsPropertiesNV get_nv2;
  VkResult result;

  {
    PFN_vkEnumerateInstanceVersion enumerate =
        (PFN_vkEnumerateInstanceVersion)vkGetInstanceProcAddr(VK_NULL_HANDLE, "vkEnumerateInstanceVersion");
    if (enumerate != NULL) (void)enumerate(&loader_api);
  }
  app.pApplicationName = "prometheus-m40a-capability-probe";
  app.pEngineName = "prometheus";
  app.apiVersion = loader_api < VK_API_VERSION_1_3 ? loader_api : VK_API_VERSION_1_3;
  create.pApplicationInfo = &app;
  result = vkCreateInstance(&create, NULL, &instance);
  if (result != VK_SUCCESS) return fprintf(stderr, "vkCreateInstance: %d\n", (int)result), 1;
  result = vkEnumeratePhysicalDevices(instance, &device_count, NULL);
  if (result != VK_SUCCESS || device_count == 0u) return fprintf(stderr, "no Vulkan device\n"), 1;
  if (device_count > 32u) device_count = 32u;
  result = vkEnumeratePhysicalDevices(instance, &device_count, devices);
  if (result != VK_SUCCESS) return 1;
  for (i = 0u; i < device_count; ++i) {
    VkPhysicalDeviceProperties candidate;
    vkGetPhysicalDeviceProperties(devices[i], &candidate);
    if (candidate.vendorID == 0x10deu && candidate.deviceID == 0x2488u) { physical = devices[i]; break; }
  }
  if (physical == VK_NULL_HANDLE) physical = devices[0];
  vkGetPhysicalDeviceProperties(physical, &properties);

  result = vkEnumerateDeviceExtensionProperties(physical, NULL, &extension_count, NULL);
  if (result != VK_SUCCESS) return 1;
  extensions = (VkExtensionProperties*)calloc(extension_count, sizeof(*extensions));
  if (extensions == NULL) return 1;
  result = vkEnumerateDeviceExtensionProperties(physical, NULL, &extension_count, extensions);
  if (result != VK_SUCCESS) return 1;
  khr_spec = extension_spec(extensions, extension_count, VK_KHR_COOPERATIVE_MATRIX_EXTENSION_NAME);
  nv_spec = extension_spec(extensions, extension_count, VK_NV_COOPERATIVE_MATRIX_EXTENSION_NAME);
  nv2_spec = extension_spec(extensions, extension_count, VK_NV_COOPERATIVE_MATRIX_2_EXTENSION_NAME);

  cooperative2.pNext = &cooperative;
  cooperative.pNext = &subgroup;
  subgroup.pNext = &driver;
  properties2.pNext = &cooperative2;
  vkGetPhysicalDeviceProperties2(physical, &properties2);
  float16.pNext = &memory_model;
  memory_model.pNext = &khr;
  khr.pNext = &nv;
  nv.pNext = &nv2;
  features2.pNext = &float16;
  vkGetPhysicalDeviceFeatures2(physical, &features2);

  get_khr = (PFN_vkGetPhysicalDeviceCooperativeMatrixPropertiesKHR)
      vkGetInstanceProcAddr(instance, "vkGetPhysicalDeviceCooperativeMatrixPropertiesKHR");
  get_nv = (PFN_vkGetPhysicalDeviceCooperativeMatrixPropertiesNV)
      vkGetInstanceProcAddr(instance, "vkGetPhysicalDeviceCooperativeMatrixPropertiesNV");
  get_nv2 = (PFN_vkGetPhysicalDeviceCooperativeMatrixFlexibleDimensionsPropertiesNV)
      vkGetInstanceProcAddr(instance, "vkGetPhysicalDeviceCooperativeMatrixFlexibleDimensionsPropertiesNV");
  if (khr_spec != 0u && get_khr != NULL &&
      get_khr(physical, &tuple_count, NULL) == VK_SUCCESS && tuple_count != 0u) {
    tuples = (VkCooperativeMatrixPropertiesKHR*)calloc(tuple_count, sizeof(*tuples));
    if (tuples == NULL) return 1;
    for (i = 0u; i < tuple_count; ++i) tuples[i].sType = VK_STRUCTURE_TYPE_COOPERATIVE_MATRIX_PROPERTIES_KHR;
    if (get_khr(physical, &tuple_count, tuples) != VK_SUCCESS) tuple_count = 0u;
    qsort(tuples, tuple_count, sizeof(*tuples), tuple_compare);
  }
  if (nv_spec != 0u && get_nv != NULL) (void)get_nv(physical, &nv_count, NULL);
  if (nv2_spec != 0u && get_nv2 != NULL) (void)get_nv2(physical, &flexible_count, NULL);

  printf("{\"schema\":\"prometheus.m40a.cooperative-matrix-probe.v1\",");
  printf("\"header_support\":true,\"loader_api_version\":%u,\"instance_api_version\":%u,", loader_api, app.apiVersion);
  printf("\"device\":{\"name\":"); json_string(properties.deviceName);
  printf(",\"api_version\":%u,\"vendor_id\":%u,\"device_id\":%u,\"driver_version\":%u,\"driver_id\":%u,\"driver_name\":",
         properties.apiVersion, properties.vendorID, properties.deviceID, properties.driverVersion, (uint32_t)driver.driverID);
  json_string(driver.driverName); printf(",\"driver_info\":"); json_string(driver.driverInfo); printf("},");
  printf("\"extensions\":{\"VK_KHR_cooperative_matrix\":%u,\"VK_NV_cooperative_matrix\":%u,\"VK_NV_cooperative_matrix2\":%u},",
         khr_spec, nv_spec, nv2_spec);
  printf("\"features\":{\"shader_float16\":%s,\"vulkan_memory_model\":%s,\"khr_cooperative_matrix\":%s,"
         "\"khr_robust_buffer_access\":%s,\"nv_cooperative_matrix\":%s,\"nv_robust_buffer_access\":%s,"
         "\"nv2_workgroup_scope\":%s,\"nv2_flexible_dimensions\":%s,\"nv2_reductions\":%s,"
         "\"nv2_conversions\":%s,\"nv2_per_element_operations\":%s,\"nv2_tensor_addressing\":%s,\"nv2_block_loads\":%s},",
         float16.shaderFloat16 ? "true" : "false", memory_model.vulkanMemoryModel ? "true" : "false",
         khr.cooperativeMatrix ? "true" : "false", khr.cooperativeMatrixRobustBufferAccess ? "true" : "false",
         nv.cooperativeMatrix ? "true" : "false", nv.cooperativeMatrixRobustBufferAccess ? "true" : "false",
         nv2.cooperativeMatrixWorkgroupScope ? "true" : "false", nv2.cooperativeMatrixFlexibleDimensions ? "true" : "false",
         nv2.cooperativeMatrixReductions ? "true" : "false", nv2.cooperativeMatrixConversions ? "true" : "false",
         nv2.cooperativeMatrixPerElementOperations ? "true" : "false", nv2.cooperativeMatrixTensorAddressing ? "true" : "false",
         nv2.cooperativeMatrixBlockLoads ? "true" : "false");
  printf("\"subgroup\":{\"size\":%u,\"supported_stages\":%u,\"supported_operations\":%u,"
         "\"quad_operations_in_all_stages\":%s,\"cooperative_matrix_supported_stages\":%u},",
         subgroup.subgroupSize, subgroup.supportedStages, subgroup.supportedOperations,
         subgroup.quadOperationsInAllStages ? "true" : "false", cooperative.cooperativeMatrixSupportedStages);
  printf("\"function_pointers\":{\"khr_properties\":%s,\"nv_properties\":%s,\"nv2_flexible_properties\":%s},",
         get_khr ? "true" : "false", get_nv ? "true" : "false", get_nv2 ? "true" : "false");
  printf("\"required_contract\":{\"vulkan_extension\":\"VK_KHR_cooperative_matrix\","
         "\"spirv_extension\":\"SPV_KHR_cooperative_matrix\","
         "\"spirv_capabilities\":[\"CooperativeMatrixKHR\",\"Float16\",\"VulkanMemoryModel\"]},");
  printf("\"khr_tuples\":[");
  for (i = 0u; i < tuple_count; ++i) {
    if (i != 0u) putchar(',');
    printf("{\"scope\":\"%s\",\"m\":%u,\"n\":%u,\"k\":%u,\"a_type\":\"%s\",\"b_type\":\"%s\","
           "\"c_type\":\"%s\",\"result_type\":\"%s\",\"saturating_accumulation\":%s,\"mma_usable\":true,\"usefulness\":\"%s\"}",
           scope_name(tuples[i].scope), tuples[i].MSize, tuples[i].NSize, tuples[i].KSize,
           component_name(tuples[i].AType), component_name(tuples[i].BType), component_name(tuples[i].CType),
           component_name(tuples[i].ResultType), tuples[i].saturatingAccumulation ? "true" : "false", usefulness(&tuples[i]));
  }
  printf("],\"nv_tuple_count\":%u,\"nv2\":{\"flexible_tuple_count\":%u,"
         "\"workgroup_scope_max_workgroup_size\":%u,\"flexible_dimensions_max_dimension\":%u,"
         "\"workgroup_scope_reserved_shared_memory\":%u}}\n",
         nv_count, flexible_count, cooperative2.cooperativeMatrixWorkgroupScopeMaxWorkgroupSize,
         cooperative2.cooperativeMatrixFlexibleDimensionsMaxDimension,
         cooperative2.cooperativeMatrixWorkgroupScopeReservedSharedMemory);
  free(tuples);
  free(extensions);
  vkDestroyInstance(instance, NULL);
  return 0;
}
#endif
