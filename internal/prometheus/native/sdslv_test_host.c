/* Fixed M30 test host. It remains a process executable, not a public
 * Prometheus runtime API: one SPIR-V module, one fixed result buffer, one
 * compiler-owned test input buffer, one selected case. */
#include <ctype.h>
#include <limits.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <vulkan/vulkan.h>

#ifdef _WIN32
#include <windows.h>
#else
#include <unistd.h>
#endif

typedef struct result_record {
  uint32_t abi_version;
  uint32_t failed;
  uint32_t assertion_id;
  uint32_t source_line;
  uint32_t source_column;
  uint32_t invocation_x;
  uint32_t invocation_y;
  uint32_t invocation_z;
  uint32_t value_kind;
  uint32_t component_count;
  uint32_t expected[4];
  uint32_t actual[4];
  uint32_t tolerance[4];
} result_record;

typedef struct push_data {
  uint32_t case_id;
  uint32_t row_id;
  uint32_t width;
  uint32_t height;
} push_data;

typedef struct manifest_input_metadata {
  uint32_t abi_version;
  uint32_t binding;
  uint32_t element_count;
  uint32_t payload_word_count;
  char kind[16];
  uint32_t *payload_words;
} manifest_input_metadata;

typedef struct buffer_allocation {
  VkBuffer buffer;
  VkDeviceMemory memory;
  void *mapped;
  VkDeviceSize size;
} buffer_allocation;

static void pause_ms(void) {
#ifdef _WIN32
  Sleep(10);
#else
  usleep(10000);
#endif
}

static void json_string(const char *value) {
  const unsigned char *p = (const unsigned char *)(value ? value : "");
  putchar('"');
  while (*p != '\0') {
    if (*p == '"' || *p == '\\') {
      putchar('\\');
      putchar(*p);
    } else if (*p < 0x20u) {
      printf("\\u%04x", (unsigned int)*p);
    } else {
      putchar(*p);
    }
    p++;
  }
  putchar('"');
}

static void json_with_reason(const char *status, const char *detail,
                             const char *id, const char *reason,
                             const result_record *r) {
  printf("{\"status\":\"%s\",\"detail\":\"%s\",\"stable_case_id\":\"%s\"",
         status, detail, id ? id : "");
  if (r != NULL) {
    printf(",\"invocation\":[%u,%u,%u],\"assertion_id\":%u,\"source\":[%u,%u],"
           "\"abi_version\":%u,\"failed\":%u,\"value_kind\":%u,"
           "\"component_count\":%u,"
           "\"expected_bits\":[%u,%u,%u,%u],\"actual_bits\":[%u,%u,%u,%u],"
           "\"tolerance_bits\":[%u,%u,%u,%u]",
           r->invocation_x, r->invocation_y, r->invocation_z, r->assertion_id,
           r->source_line, r->source_column, r->abi_version, r->failed,
           r->value_kind, r->component_count, r->expected[0],
           r->expected[1], r->expected[2], r->expected[3], r->actual[0],
           r->actual[1], r->actual[2], r->actual[3], r->tolerance[0],
           r->tolerance[1], r->tolerance[2], r->tolerance[3]);
    if (reason != NULL) {
      printf(",\"reason\":");
      json_string(reason);
    }
  }
  puts("}");
}

#define json(status, detail, id, result) \
  json_with_reason(status, detail, id, NULL, result)

static uint32_t memory_type(VkPhysicalDevice physical, uint32_t bits,
                            VkMemoryPropertyFlags flags) {
  VkPhysicalDeviceMemoryProperties properties;
  uint32_t i;
  vkGetPhysicalDeviceMemoryProperties(physical, &properties);
  for (i = 0; i < properties.memoryTypeCount; i++) {
    if ((bits & (1u << i)) != 0u &&
        (properties.memoryTypes[i].propertyFlags & flags) == flags) {
      return i;
    }
  }
  return UINT32_MAX;
}

static int checked_mul_u32(uint32_t a, uint32_t b, uint32_t *out) {
  if (a != 0u && b > UINT32_MAX / a) {
    return 0;
  }
  *out = a * b;
  return 1;
}

static int checked_mul_size(size_t a, size_t b, size_t *out) {
  if (a != 0u && b > SIZE_MAX / a) {
    return 0;
  }
  *out = a * b;
  return 1;
}

static int load_file_bytes(const char *path, unsigned char **bytes,
                           size_t *byte_count) {
  FILE *file = fopen(path, "rb");
  long file_size;
  unsigned char *data;
  if (file == NULL) {
    return 0;
  }
  if (fseek(file, 0, SEEK_END) != 0) {
    fclose(file);
    return 0;
  }
  file_size = ftell(file);
  if (file_size < 0) {
    fclose(file);
    return 0;
  }
  if (fseek(file, 0, SEEK_SET) != 0) {
    fclose(file);
    return 0;
  }
  data = (unsigned char *)malloc((size_t)file_size + 1u);
  if (data == NULL) {
    fclose(file);
    return 0;
  }
  if (fread(data, 1u, (size_t)file_size, file) != (size_t)file_size) {
    free(data);
    fclose(file);
    return 0;
  }
  fclose(file);
  data[file_size] = '\0';
  *bytes = data;
  *byte_count = (size_t)file_size;
  return 1;
}

static const char *skip_ws(const char *p) {
  while (*p != '\0' && isspace((unsigned char)*p) != 0) {
    p++;
  }
  return p;
}

static const char *skip_json_string(const char *p) {
  if (*p != '"') {
    return NULL;
  }
  p++;
  while (*p != '\0') {
    if (*p == '\\') {
      if (p[1] == '\0') {
        return NULL;
      }
      p += 2;
      continue;
    }
    if (*p == '"') {
      return p + 1;
    }
    p++;
  }
  return NULL;
}

static const char *skip_json_value(const char *p);

static const char *skip_json_compound(const char *p, char open_ch,
                                      char close_ch) {
  int depth = 0;
  if (*p != open_ch) {
    return NULL;
  }
  while (*p != '\0') {
    if (*p == '"') {
      p = skip_json_string(p);
      if (p == NULL) {
        return NULL;
      }
      continue;
    }
    if (*p == open_ch) {
      depth++;
    } else if (*p == close_ch) {
      depth--;
      if (depth == 0) {
        return p + 1;
      }
    }
    p++;
  }
  return NULL;
}

static const char *skip_json_scalar(const char *p) {
  while (*p != '\0' && *p != ',' && *p != '}' && *p != ']' &&
         isspace((unsigned char)*p) == 0) {
    p++;
  }
  return p;
}

static const char *skip_json_value(const char *p) {
  p = skip_ws(p);
  if (*p == '"') {
    return skip_json_string(p);
  }
  if (*p == '{') {
    return skip_json_compound(p, '{', '}');
  }
  if (*p == '[') {
    return skip_json_compound(p, '[', ']');
  }
  return skip_json_scalar(p);
}

static int parse_json_string_value(const char *p, char *out, size_t out_cap,
                                   const char **end_out) {
  const char *end;
  size_t len;
  p = skip_ws(p);
  if (*p != '"') {
    return 0;
  }
  end = skip_json_string(p);
  if (end == NULL) {
    return 0;
  }
  len = (size_t)(end - p - 2);
  if (len + 1u > out_cap) {
    return 0;
  }
  memcpy(out, p + 1, len);
  out[len] = '\0';
  if (end_out != NULL) {
    *end_out = end;
  }
  return 1;
}

static int object_member_range(const char *obj_start, const char *key,
                               const char **value_start,
                               const char **value_end) {
  const char *p = skip_ws(obj_start);
  size_t key_len = strlen(key);
  if (*p != '{') {
    return 0;
  }
  p++;
  while (1) {
    char parsed_key[128];
    const char *after_key;
    p = skip_ws(p);
    if (*p == '}') {
      return 0;
    }
    if (!parse_json_string_value(p, parsed_key, sizeof(parsed_key), &after_key)) {
      return 0;
    }
    p = skip_ws(after_key);
    if (*p != ':') {
      return 0;
    }
    p = skip_ws(p + 1);
    *value_start = p;
    *value_end = skip_json_value(p);
    if (*value_end == '\0' && *p != '\0') {
      return 0;
    }
    if (strlen(parsed_key) == key_len && strcmp(parsed_key, key) == 0) {
      return 1;
    }
    p = skip_ws(*value_end);
    if (*p == ',') {
      p++;
      continue;
    }
    if (*p == '}') {
      return 0;
    }
    return 0;
  }
}

static int object_u32_field(const char *obj_start, const char *key,
                            uint32_t *out) {
  const char *value_start;
  const char *value_end;
  unsigned long value;
  char *parse_end;
  if (!object_member_range(obj_start, key, &value_start, &value_end)) {
    return 0;
  }
  value = strtoul(skip_ws(value_start), &parse_end, 10);
  if ((const char *)parse_end != skip_ws(value_end) || value > UINT32_MAX) {
    return 0;
  }
  *out = (uint32_t)value;
  return 1;
}

static int object_string_field(const char *obj_start, const char *key,
                               char *out, size_t out_cap) {
  const char *value_start;
  const char *value_end;
  if (!object_member_range(obj_start, key, &value_start, &value_end)) {
    return 0;
  }
  return parse_json_string_value(value_start, out, out_cap, &value_end);
}

static int object_u32_array_field(const char *obj_start, const char *key,
                                  uint32_t **out_words, uint32_t *out_count) {
  const char *value_start;
  const char *value_end;
  const char *p;
  uint32_t *words = NULL;
  uint32_t count = 0;
  if (!object_member_range(obj_start, key, &value_start, &value_end)) {
    return 0;
  }
  p = skip_ws(value_start);
  if (strncmp(p, "null", 4) == 0 && skip_ws(p + 4) == skip_ws(value_end)) {
    *out_words = NULL;
    *out_count = 0;
    return 1;
  }
  if (*p != '[') {
    return 0;
  }
  p++;
  while (1) {
    unsigned long value;
    char *parse_end;
    uint32_t *grown;
    p = skip_ws(p);
    if (*p == ']') {
      break;
    }
    value = strtoul(p, &parse_end, 10);
    if ((const char *)parse_end == p || value > UINT32_MAX) {
      free(words);
      return 0;
    }
    grown = (uint32_t *)realloc(words, (size_t)(count + 1u) * sizeof(uint32_t));
    if (grown == NULL) {
      free(words);
      return 0;
    }
    words = grown;
    words[count++] = (uint32_t)value;
    p = skip_ws(parse_end);
    if (*p == ',') {
      p++;
      continue;
    }
    if (*p == ']') {
      break;
    }
    free(words);
    return 0;
  }
  *out_words = words;
  *out_count = count;
  return 1;
}

static int find_case_object(const char *manifest, const char *stable_id,
                            const char **case_start) {
  const char *cases_value_start;
  const char *cases_value_end;
  const char *p;
  if (!object_member_range(manifest, "cases", &cases_value_start,
                           &cases_value_end)) {
    return 0;
  }
  p = skip_ws(cases_value_start);
  if (*p != '[') {
    return 0;
  }
  p++;
  while (1) {
    const char *candidate_end;
    char candidate_id[128];
    p = skip_ws(p);
    if (*p == ']') {
      return 0;
    }
    if (*p != '{') {
      return 0;
    }
    candidate_end = skip_json_value(p);
    if (candidate_end == NULL) {
      return 0;
    }
    if (object_string_field(p, "stable_id", candidate_id, sizeof(candidate_id)) &&
        strcmp(candidate_id, stable_id) == 0) {
      *case_start = p;
      return 1;
    }
    p = skip_ws(candidate_end);
    if (*p == ',') {
      p++;
      continue;
    }
    if (*p == ']') {
      return 0;
    }
    return 0;
  }
}

static void free_manifest_input(manifest_input_metadata *input) {
  free(input->payload_words);
  memset(input, 0, sizeof(*input));
}

static int parse_manifest_input(const char *manifest, const char *stable_id,
                                manifest_input_metadata *out) {
  const char *case_start;
  const char *input_start;
  const char *input_end;
  uint32_t payload_word_count = 0;
  int has_payload_words = 0;
  memset(out, 0, sizeof(*out));
  if (!find_case_object(manifest, stable_id, &case_start)) {
    return 0;
  }
  if (!object_u32_field(case_start, "test_input_binding", &out->binding)) {
    return 0;
  }
  if (!object_member_range(case_start, "test_input", &input_start, &input_end)) {
    return 0;
  }
  if (!object_u32_field(input_start, "abi_version", &out->abi_version) &&
      !object_u32_field(input_start, "ABIVersion", &out->abi_version)) {
    return 0;
  }
  if (!object_string_field(input_start, "kind", out->kind, sizeof(out->kind)) &&
      !object_string_field(input_start, "Kind", out->kind, sizeof(out->kind))) {
    return 0;
  }
  if (!object_u32_field(input_start, "element_count", &out->element_count) &&
      !object_u32_field(input_start, "ElementCount", &out->element_count)) {
    return 0;
  }
  has_payload_words =
      object_u32_array_field(input_start, "payload_words", &out->payload_words,
                             &payload_word_count) ||
      object_u32_array_field(input_start, "PayloadWords", &out->payload_words,
                             &payload_word_count);
  out->payload_word_count = payload_word_count;
  if (out->abi_version != 1u) {
    return 0;
  }
  if (out->binding != 1u) {
    return 0;
  }
  if (strcmp(out->kind, "none") == 0) {
    if (!has_payload_words) {
      out->payload_words = NULL;
      out->payload_word_count = 0u;
    }
    if (out->element_count != 0u || out->payload_word_count != 0u) {
      return 0;
    }
    return 1;
  }
  if (!has_payload_words) {
    return 0;
  }
  if (strcmp(out->kind, "bool") != 0 && strcmp(out->kind, "int") != 0 &&
      strcmp(out->kind, "uint") != 0 && strcmp(out->kind, "float") != 0) {
    return 0;
  }
  if (out->element_count != out->payload_word_count) {
    return 0;
  }
  return 1;
}

static int parse_manifest_assertion_reason(const char *manifest,
                                           const char *stable_id,
                                           uint32_t assertion_id, char *out,
                                           size_t out_cap) {
  const char *case_start;
  const char *assertions;
  const char *assertions_end;
  const char *p;
  uint32_t index = 0u;
  if (!find_case_object(manifest, stable_id, &case_start) ||
      !object_member_range(case_start, "assertions", &assertions,
                           &assertions_end)) {
    return 0;
  }
  p = skip_ws(assertions);
  if (*p != '[') {
    return 0;
  }
  p++;
  while (1) {
    const char *end;
    p = skip_ws(p);
    if (*p == ']') {
      return 0;
    }
    if (*p != '{') {
      return 0;
    }
    end = skip_json_compound(p, '{', '}');
    if (end == NULL) {
      return 0;
    }
    if (index == assertion_id) {
      return object_string_field(p, "reason", out, out_cap) ||
             object_string_field(p, "Reason", out, out_cap);
    }
    index++;
    p = skip_ws(end);
    if (*p == ',') {
      p++;
      continue;
    }
    return *p == ']';
  }
}

static int allocate_host_visible_buffer(VkPhysicalDevice physical, VkDevice device,
                                        VkDeviceSize size, buffer_allocation *out) {
  VkBufferCreateInfo buffer_info = {VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO};
  VkMemoryRequirements requirements;
  VkMemoryAllocateInfo allocation_info = {VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO};
  VkResult vr;
  memset(out, 0, sizeof(*out));
  buffer_info.size = size;
  buffer_info.usage = VK_BUFFER_USAGE_STORAGE_BUFFER_BIT;
  buffer_info.sharingMode = VK_SHARING_MODE_EXCLUSIVE;
  vr = vkCreateBuffer(device, &buffer_info, 0, &out->buffer);
  if (vr != VK_SUCCESS) {
    return 0;
  }
  vkGetBufferMemoryRequirements(device, out->buffer, &requirements);
  allocation_info.allocationSize = requirements.size;
  allocation_info.memoryTypeIndex =
      memory_type(physical, requirements.memoryTypeBits,
                  VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
                      VK_MEMORY_PROPERTY_HOST_COHERENT_BIT);
  if (allocation_info.memoryTypeIndex == UINT32_MAX) {
    return 0;
  }
  vr = vkAllocateMemory(device, &allocation_info, 0, &out->memory);
  if (vr != VK_SUCCESS) {
    return 0;
  }
  vr = vkBindBufferMemory(device, out->buffer, out->memory, 0);
  if (vr != VK_SUCCESS) {
    return 0;
  }
  vr = vkMapMemory(device, out->memory, 0, VK_WHOLE_SIZE, 0, &out->mapped);
  if (vr != VK_SUCCESS) {
    return 0;
  }
  out->size = size;
  return 1;
}

static void destroy_buffer_allocation(VkDevice device, buffer_allocation *buffer) {
  if (buffer->mapped != NULL) {
    vkUnmapMemory(device, buffer->memory);
    buffer->mapped = NULL;
  }
  if (buffer->buffer != VK_NULL_HANDLE) {
    vkDestroyBuffer(device, buffer->buffer, 0);
    buffer->buffer = VK_NULL_HANDLE;
  }
  if (buffer->memory != VK_NULL_HANDLE) {
    vkFreeMemory(device, buffer->memory, 0);
    buffer->memory = VK_NULL_HANDLE;
  }
  buffer->size = 0;
}

int main(int argc, char **argv) {
  const char *manifest_path = NULL;
  const char *spv_path = NULL;
  const char *stable_id = "";
  uint32_t case_id = 0u;
  uint32_t row_id = 0u;
  uint32_t gx = 1u, gy = 1u, gz = 1u;
  uint32_t wgx = 1u, wgy = 1u, wgz = 1u;
  uint32_t invocations = 0u;
  uint32_t width = 0u;
  uint32_t height = 0u;
  uint32_t plane = 0u;
  uint32_t i;
  unsigned char *manifest_bytes = NULL;
  unsigned char *spv_bytes = NULL;
  size_t manifest_byte_count = 0u;
  size_t spv_byte_count = 0u;
  manifest_input_metadata input_metadata;
  VkResult vr;
  VkInstance instance = VK_NULL_HANDLE;
  VkPhysicalDevice physical = VK_NULL_HANDLE;
  VkDevice device = VK_NULL_HANDLE;
  VkQueue queue = VK_NULL_HANDLE;
  uint32_t family = UINT32_MAX;
  uint32_t physical_count = 0u;
  VkCommandPool pool = VK_NULL_HANDLE;
  VkDescriptorSetLayout set_layout = VK_NULL_HANDLE;
  VkPipelineLayout layout = VK_NULL_HANDLE;
  VkShaderModule shader = VK_NULL_HANDLE;
  VkPipeline pipeline = VK_NULL_HANDLE;
  VkDescriptorPool descriptor_pool = VK_NULL_HANDLE;
  VkDescriptorSet set = VK_NULL_HANDLE;
  buffer_allocation result_buffer;
  buffer_allocation input_buffer;
  VkCommandBuffer command = VK_NULL_HANDLE;
  VkFence fence = VK_NULL_HANDLE;
  result_record *records = NULL;
  result_record *failure = NULL;
  char assertion_reason[1024];
  memset(&input_metadata, 0, sizeof(input_metadata));
  memset(&result_buffer, 0, sizeof(result_buffer));
  memset(&input_buffer, 0, sizeof(input_buffer));

  for (i = 1u; i < (uint32_t)argc; i++) {
    if (strcmp(argv[i], "--manifest") == 0 && i + 1u < (uint32_t)argc) {
      manifest_path = argv[++i];
    } else if (strcmp(argv[i], "--spv") == 0 && i + 1u < (uint32_t)argc) {
      spv_path = argv[++i];
    } else if (strcmp(argv[i], "--case") == 0 && i + 1u < (uint32_t)argc) {
      stable_id = argv[++i];
    } else if (strcmp(argv[i], "--case-index") == 0 &&
               i + 1u < (uint32_t)argc) {
      case_id = (uint32_t)strtoul(argv[++i], 0, 10);
    } else if (strcmp(argv[i], "--row-index") == 0 &&
               i + 1u < (uint32_t)argc) {
      row_id = (uint32_t)strtoul(argv[++i], 0, 10);
    } else if (strcmp(argv[i], "--groups") == 0 &&
               i + 3u < (uint32_t)argc) {
      gx = (uint32_t)strtoul(argv[++i], 0, 10);
      gy = (uint32_t)strtoul(argv[++i], 0, 10);
      gz = (uint32_t)strtoul(argv[++i], 0, 10);
    } else if (strcmp(argv[i], "--workgroup") == 0 &&
               i + 3u < (uint32_t)argc) {
      wgx = (uint32_t)strtoul(argv[++i], 0, 10);
      wgy = (uint32_t)strtoul(argv[++i], 0, 10);
      wgz = (uint32_t)strtoul(argv[++i], 0, 10);
    }
  }

  if (manifest_path == NULL || spv_path == NULL) {
    json("HOST_FAILURE", "missing manifest or SPIR-V", stable_id, NULL);
    return 2;
  }
  if (!load_file_bytes(manifest_path, &manifest_bytes, &manifest_byte_count)) {
    json("HOST_FAILURE", "missing manifest or SPIR-V", stable_id, NULL);
    return 2;
  }
  if (!parse_manifest_input((const char *)manifest_bytes, stable_id,
                            &input_metadata)) {
    json("HOST_FAILURE", "malformed test input manifest metadata", stable_id,
         NULL);
    free(manifest_bytes);
    return 2;
  }
  if (!checked_mul_u32(gx, wgx, &width) || !checked_mul_u32(gy, wgy, &height) ||
      !checked_mul_u32(gz, wgz, &plane) ||
      !checked_mul_u32(width, height, &invocations) ||
      !checked_mul_u32(invocations, plane, &invocations) || invocations == 0u) {
    json("HOST_FAILURE", "invalid invocation count", stable_id, NULL);
    free_manifest_input(&input_metadata);
    free(manifest_bytes);
    return 2;
  }
  if (!load_file_bytes(spv_path, &spv_bytes, &spv_byte_count) ||
      spv_byte_count == 0u || (spv_byte_count % sizeof(uint32_t)) != 0u) {
    json("COMPILE_FAILED", "cannot read SPIR-V", stable_id, NULL);
    free_manifest_input(&input_metadata);
    free(manifest_bytes);
    free(spv_bytes);
    return 2;
  }

  {
    VkInstanceCreateInfo create_info = {VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO};
    vr = vkCreateInstance(&create_info, 0, &instance);
    if (vr != VK_SUCCESS) {
      json("HOST_FAILURE", "vkCreateInstance", stable_id, NULL);
      goto done;
    }
  }

  vr = vkEnumeratePhysicalDevices(instance, &physical_count, 0);
  if (vr != VK_SUCCESS || physical_count == 0u) {
    json("HOST_FAILURE", "no Vulkan physical device", stable_id, NULL);
    goto done;
  }
  {
    VkPhysicalDevice *devices = (VkPhysicalDevice *)malloc(
        sizeof(VkPhysicalDevice) * (size_t)physical_count);
    if (devices == NULL) {
      json("HOST_FAILURE", "enumerate devices", stable_id, NULL);
      goto done;
    }
    vr = vkEnumeratePhysicalDevices(instance, &physical_count, devices);
    if (vr != VK_SUCCESS) {
      free(devices);
      json("HOST_FAILURE", "enumerate devices", stable_id, NULL);
      goto done;
    }
    for (i = 0u; i < physical_count && physical == VK_NULL_HANDLE; i++) {
      uint32_t queue_count = 0u;
      uint32_t j;
      VkQueueFamilyProperties *queues;
      vkGetPhysicalDeviceQueueFamilyProperties(devices[i], &queue_count, 0);
      queues = (VkQueueFamilyProperties *)malloc(
          sizeof(VkQueueFamilyProperties) * (size_t)queue_count);
      if (queues == NULL) {
        free(devices);
        json("HOST_FAILURE", "enumerate queues", stable_id, NULL);
        goto done;
      }
      vkGetPhysicalDeviceQueueFamilyProperties(devices[i], &queue_count, queues);
      for (j = 0u; j < queue_count; j++) {
        if ((queues[j].queueFlags & VK_QUEUE_COMPUTE_BIT) != 0u) {
          physical = devices[i];
          family = j;
          break;
        }
      }
      free(queues);
    }
    free(devices);
  }
  if (physical == VK_NULL_HANDLE) {
    json("HOST_FAILURE", "no compute queue", stable_id, NULL);
    goto done;
  }

  {
    float priority = 1.0f;
    VkDeviceQueueCreateInfo queue_info = {
        VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO};
    VkDeviceCreateInfo device_info = {VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO};
    queue_info.queueFamilyIndex = family;
    queue_info.queueCount = 1u;
    queue_info.pQueuePriorities = &priority;
    device_info.queueCreateInfoCount = 1u;
    device_info.pQueueCreateInfos = &queue_info;
    vr = vkCreateDevice(physical, &device_info, 0, &device);
    if (vr != VK_SUCCESS) {
      json("HOST_FAILURE", "vkCreateDevice", stable_id, NULL);
      goto done;
    }
  }
  vkGetDeviceQueue(device, family, 0u, &queue);

  {
    VkCommandPoolCreateInfo pool_info = {
        VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO};
    pool_info.flags = VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT;
    pool_info.queueFamilyIndex = family;
    vr = vkCreateCommandPool(device, &pool_info, 0, &pool);
    if (vr != VK_SUCCESS) {
      json("HOST_FAILURE", "vkCreateCommandPool", stable_id, NULL);
      goto done;
    }
  }

  {
    VkDescriptorSetLayoutBinding bindings[2];
    VkDescriptorSetLayoutCreateInfo layout_info = {
        VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO};
    memset(bindings, 0, sizeof(bindings));
    bindings[0].binding = 0u;
    bindings[0].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    bindings[0].descriptorCount = 1u;
    bindings[0].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
    bindings[1].binding = 1u;
    bindings[1].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    bindings[1].descriptorCount = 1u;
    bindings[1].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
    layout_info.bindingCount = 2u;
    layout_info.pBindings = bindings;
    vr = vkCreateDescriptorSetLayout(device, &layout_info, 0, &set_layout);
    if (vr != VK_SUCCESS) {
      json("PIPELINE_CREATION_FAILED", "descriptor layout", stable_id, NULL);
      goto done;
    }
  }

  {
    VkPushConstantRange range = {VK_SHADER_STAGE_COMPUTE_BIT, 0u,
                                 sizeof(push_data)};
    VkPipelineLayoutCreateInfo layout_info = {
        VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO};
    layout_info.setLayoutCount = 1u;
    layout_info.pSetLayouts = &set_layout;
    layout_info.pushConstantRangeCount = 1u;
    layout_info.pPushConstantRanges = &range;
    vr = vkCreatePipelineLayout(device, &layout_info, 0, &layout);
    if (vr != VK_SUCCESS) {
      json("PIPELINE_CREATION_FAILED", "pipeline layout", stable_id, NULL);
      goto done;
    }
  }

  {
    VkShaderModuleCreateInfo shader_info = {
        VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO};
    VkPipelineShaderStageCreateInfo stage_info = {
        VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO};
    VkComputePipelineCreateInfo pipeline_info = {
        VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO};
    shader_info.codeSize = spv_byte_count;
    shader_info.pCode = (const uint32_t *)spv_bytes;
    vr = vkCreateShaderModule(device, &shader_info, 0, &shader);
    if (vr != VK_SUCCESS) {
      json("PIPELINE_CREATION_FAILED", "shader module", stable_id, NULL);
      goto done;
    }
    stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
    stage_info.module = shader;
    stage_info.pName = "main";
    pipeline_info.stage = stage_info;
    pipeline_info.layout = layout;
    vr = vkCreateComputePipelines(device, 0, 1u, &pipeline_info, 0, &pipeline);
    if (vr != VK_SUCCESS) {
      json("PIPELINE_CREATION_FAILED", "compute pipeline", stable_id, NULL);
      goto done;
    }
  }

  {
    size_t result_bytes;
    if (!checked_mul_size((size_t)invocations, sizeof(result_record),
                          &result_bytes) ||
        !allocate_host_visible_buffer(physical, device, (VkDeviceSize)result_bytes,
                                      &result_buffer)) {
      json("HOST_FAILURE", "result buffer", stable_id, NULL);
      goto done;
    }
    memset(result_buffer.mapped, 0, result_bytes);
  }

  {
    size_t input_word_count =
        input_metadata.element_count == 0u ? 1u : input_metadata.element_count;
    size_t input_bytes;
    if (!checked_mul_size(input_word_count, sizeof(uint32_t), &input_bytes) ||
        !allocate_host_visible_buffer(physical, device, (VkDeviceSize)input_bytes,
                                      &input_buffer)) {
      json("HOST_FAILURE", "test input buffer", stable_id, NULL);
      goto done;
    }
    memset(input_buffer.mapped, 0, input_bytes);
    if (input_metadata.element_count != 0u && input_metadata.payload_words != NULL) {
      memcpy(input_buffer.mapped, input_metadata.payload_words, input_bytes);
    }
  }

  {
    VkDescriptorPoolSize pool_size = {VK_DESCRIPTOR_TYPE_STORAGE_BUFFER, 2u};
    VkDescriptorPoolCreateInfo pool_info = {
        VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO};
    VkDescriptorSetAllocateInfo alloc_info = {
        VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO};
    VkDescriptorBufferInfo buffers[2];
    VkWriteDescriptorSet writes[2];
    pool_info.maxSets = 1u;
    pool_info.poolSizeCount = 1u;
    pool_info.pPoolSizes = &pool_size;
    vr = vkCreateDescriptorPool(device, &pool_info, 0, &descriptor_pool);
    if (vr != VK_SUCCESS) {
      json("HOST_FAILURE", "descriptor pool", stable_id, NULL);
      goto done;
    }
    alloc_info.descriptorPool = descriptor_pool;
    alloc_info.descriptorSetCount = 1u;
    alloc_info.pSetLayouts = &set_layout;
    vr = vkAllocateDescriptorSets(device, &alloc_info, &set);
    if (vr != VK_SUCCESS) {
      json("HOST_FAILURE", "descriptor set", stable_id, NULL);
      goto done;
    }
    memset(buffers, 0, sizeof(buffers));
    buffers[0].buffer = result_buffer.buffer;
    buffers[0].offset = 0u;
    buffers[0].range = result_buffer.size;
    buffers[1].buffer = input_buffer.buffer;
    buffers[1].offset = 0u;
    buffers[1].range = input_buffer.size;
    memset(writes, 0, sizeof(writes));
    writes[0].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
    writes[0].dstSet = set;
    writes[0].dstBinding = 0u;
    writes[0].descriptorCount = 1u;
    writes[0].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    writes[0].pBufferInfo = &buffers[0];
    writes[1].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
    writes[1].dstSet = set;
    writes[1].dstBinding = 1u;
    writes[1].descriptorCount = 1u;
    writes[1].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    writes[1].pBufferInfo = &buffers[1];
    vkUpdateDescriptorSets(device, 2u, writes, 0u, 0);
  }

  {
    VkCommandBufferAllocateInfo alloc_info = {
        VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO};
    VkCommandBufferBeginInfo begin_info = {
        VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO};
    VkSubmitInfo submit_info = {VK_STRUCTURE_TYPE_SUBMIT_INFO};
    VkFenceCreateInfo fence_info = {VK_STRUCTURE_TYPE_FENCE_CREATE_INFO};
    push_data push = {case_id, row_id, width, height};
    alloc_info.commandPool = pool;
    alloc_info.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;
    alloc_info.commandBufferCount = 1u;
    vr = vkAllocateCommandBuffers(device, &alloc_info, &command);
    if (vr != VK_SUCCESS) {
      json("HOST_FAILURE", "command buffer", stable_id, NULL);
      goto done;
    }
    vr = vkBeginCommandBuffer(command, &begin_info);
    if (vr != VK_SUCCESS) {
      json("HOST_FAILURE", "begin command buffer", stable_id, NULL);
      goto done;
    }
    vkCmdBindPipeline(command, VK_PIPELINE_BIND_POINT_COMPUTE, pipeline);
    vkCmdBindDescriptorSets(command, VK_PIPELINE_BIND_POINT_COMPUTE, layout, 0u,
                            1u, &set, 0u, 0);
    vkCmdPushConstants(command, layout, VK_SHADER_STAGE_COMPUTE_BIT, 0u,
                       sizeof(push), &push);
    vkCmdDispatch(command, gx, gy, gz);
    vr = vkEndCommandBuffer(command);
    if (vr != VK_SUCCESS) {
      json("HOST_FAILURE", "end command buffer", stable_id, NULL);
      goto done;
    }
    vr = vkCreateFence(device, &fence_info, 0, &fence);
    if (vr != VK_SUCCESS) {
      json("HOST_FAILURE", "create fence", stable_id, NULL);
      goto done;
    }
    submit_info.commandBufferCount = 1u;
    submit_info.pCommandBuffers = &command;
    vr = vkQueueSubmit(queue, 1u, &submit_info, fence);
    if (vr != VK_SUCCESS) {
      json(vr == VK_ERROR_DEVICE_LOST ? "DEVICE_LOST" : "SUBMIT_FAILED",
           "queue submit", stable_id, NULL);
      goto done;
    }
    for (i = 0u; i < 500u; i++) {
      vr = vkWaitForFences(device, 1u, &fence, VK_TRUE, 10000000ull);
      if (vr == VK_SUCCESS) {
        break;
      }
      if (vr == VK_ERROR_DEVICE_LOST) {
        json("DEVICE_LOST", "fence", stable_id, NULL);
        goto done;
      }
      pause_ms();
    }
    if (vr != VK_SUCCESS) {
      json("TIMEOUT", "deadline exceeded", stable_id, NULL);
      goto done;
    }
  }

  records = (result_record *)result_buffer.mapped;
  for (i = 0u; i < invocations; i++) {
    if (records[i].abi_version != 1u) {
      json("INVALID_RESULT_BUFFER", "ABI version", stable_id, NULL);
      goto done;
    }
    if (failure == NULL && records[i].failed != 0u) {
      failure = &records[i];
    }
  }
  if (failure != NULL) {
    if (!parse_manifest_assertion_reason((const char *)manifest_bytes,
                                         stable_id, failure->assertion_id,
                                         assertion_reason,
                                         sizeof(assertion_reason))) {
      json("HOST_FAILURE", "malformed assertion reason metadata", stable_id,
           NULL);
      goto done;
    }
    json_with_reason("ASSERTION_FAILED", "assertion", stable_id,
                     assertion_reason, failure);
  } else {
    json("PASS", "", stable_id, &records[0]);
  }

done:
  if (device != VK_NULL_HANDLE) {
    vkDeviceWaitIdle(device);
  }
  if (fence != VK_NULL_HANDLE) {
    vkDestroyFence(device, fence, 0);
  }
  if (pool != VK_NULL_HANDLE && command != VK_NULL_HANDLE) {
    vkFreeCommandBuffers(device, pool, 1u, &command);
  }
  if (descriptor_pool != VK_NULL_HANDLE) {
    vkDestroyDescriptorPool(device, descriptor_pool, 0);
  }
  if (device != VK_NULL_HANDLE) {
    destroy_buffer_allocation(device, &input_buffer);
    destroy_buffer_allocation(device, &result_buffer);
  }
  if (pipeline != VK_NULL_HANDLE) {
    vkDestroyPipeline(device, pipeline, 0);
  }
  if (shader != VK_NULL_HANDLE) {
    vkDestroyShaderModule(device, shader, 0);
  }
  if (layout != VK_NULL_HANDLE) {
    vkDestroyPipelineLayout(device, layout, 0);
  }
  if (set_layout != VK_NULL_HANDLE) {
    vkDestroyDescriptorSetLayout(device, set_layout, 0);
  }
  if (pool != VK_NULL_HANDLE) {
    vkDestroyCommandPool(device, pool, 0);
  }
  if (device != VK_NULL_HANDLE) {
    vkDestroyDevice(device, 0);
  }
  if (instance != VK_NULL_HANDLE) {
    vkDestroyInstance(instance, 0);
  }
  free_manifest_input(&input_metadata);
  free(manifest_bytes);
  free(spv_bytes);
  return 0;
}
