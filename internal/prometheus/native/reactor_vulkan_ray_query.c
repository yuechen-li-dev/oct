#include "reactor_vulkan.h"
#include "reactor_shader_package.h"

#include <math.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#if defined(_WIN32)
#include <windows.h>
#else
#include <pthread.h>
#endif

#define PROM_RAY_QUERY_SCENE_MAGIC 0x52515343u
#define PROM_RAY_QUERY_MAX_LIVE_SCENES 16u

typedef struct prom_ray_query_as {
  VkAccelerationStructureKHR handle;
  prom_vk_buffer storage;
  VkDeviceAddress device_address;
} prom_ray_query_as;

typedef struct prom_ray_query_scene {
  uint32_t magic;
  void* runtime_handle;
  prom_vk_runtime_services services;
  uint32_t triangle_count;
  uint32_t sphere_count;
  prom_vk_buffer vertex_buffer;
  prom_vk_buffer triangle_shader_buffer;
  prom_vk_buffer sphere_buffer;
  prom_vk_buffer aabb_buffer;
  prom_vk_buffer instance_buffer;
  prom_ray_query_as triangle_blas;
  prom_ray_query_as procedural_blas;
  prom_ray_query_as tlas;
  prom_vk_buffer evidence_buffer;
  prom_vk_buffer ray_buffer;
  prom_vk_buffer raw_hit_buffer;
  /* Ray and raw-hit buffers are a paired, host-mapped batch capacity.  A
     committed raw scene owns both buffers and is idle whenever they are
     replaced, because this route submits synchronously. */
  uint32_t batch_capacity;
  uint64_t batch_buffer_reallocation_count;
  uint64_t batch_descriptor_rebind_count;
  uint64_t batch_physical_dispatch_count;
  uint64_t batch_physical_submission_count;
  uint32_t batch_last_dispatch_groups_x;
  uint32_t raw_scene;
  VkDescriptorSetLayout descriptor_set_layout;
  VkDescriptorPool descriptor_pool;
  VkDescriptorSet descriptor_set;
  VkPipelineLayout pipeline_layout;
  VkPipeline pipeline;
  /* The RQ-M1 semantic scene is a small host-side builder which owns a
     committed legacy traversal scene once commit succeeds. This preserves the
     existing raw traversal recipe while keeping mutation outside Vulkan. */
  uint32_t committed;
  uint32_t empty_scene;
  struct prom_ray_query_scene* committed_scene;
  PrometheusRayQueryTriangle* staged_triangles;
  uint32_t staged_triangle_count;
  uint32_t staged_triangle_capacity;
  PrometheusRayQuerySphere* staged_spheres;
  uint32_t staged_sphere_count;
  uint32_t staged_sphere_capacity;
} prom_ray_query_scene;

static prom_ray_query_scene* g_ray_query_scenes[PROM_RAY_QUERY_MAX_LIVE_SCENES];

#if defined(_WIN32)
static SRWLOCK g_ray_query_scene_lock = SRWLOCK_INIT;
static void prom_ray_scene_lock(void) { AcquireSRWLockExclusive(&g_ray_query_scene_lock); }
static void prom_ray_scene_unlock(void) { ReleaseSRWLockExclusive(&g_ray_query_scene_lock); }
#else
static pthread_mutex_t g_ray_query_scene_lock = PTHREAD_MUTEX_INITIALIZER;
static void prom_ray_scene_lock(void) { pthread_mutex_lock(&g_ray_query_scene_lock); }
static void prom_ray_scene_unlock(void) { pthread_mutex_unlock(&g_ray_query_scene_lock); }
#endif

static int prom_ray_add_u64(uint64_t left, uint64_t right, uint64_t* out_value) {
  if (out_value == NULL || left > UINT64_MAX - right) return 0;
  *out_value = left + right;
  return 1;
}

static int prom_ray_mul_u64(uint64_t left, uint64_t right, uint64_t* out_value) {
  if (out_value == NULL || (left != 0u && right > UINT64_MAX / left)) return 0;
  *out_value = left * right;
  return 1;
}

_Static_assert(sizeof(PrometheusRayQueryRay) == 48u, "public ray record ABI drift");
_Static_assert(sizeof(PrometheusRayQueryHit) == 80u, "public hit record ABI drift");
_Static_assert(sizeof(PrometheusRayQueryRawHit) == 96u, "shader raw-hit record ABI drift");

enum { PROM_RAY_SHADER_WORDS_PER_RAY = 3u, PROM_RAY_SHADER_BYTES_PER_RAY = 48u };

static int prom_ray_align_address(VkDeviceAddress address, VkDeviceSize alignment,
                                  VkDeviceAddress* out_address, VkDeviceSize* out_offset) {
  VkDeviceAddress remainder;
  VkDeviceAddress offset;
  if (out_address == NULL || out_offset == NULL || alignment == 0u) return 0;
  remainder = address % alignment;
  offset = remainder == 0u ? 0u : alignment - remainder;
  if (address > UINT64_MAX - offset || offset > UINT64_MAX) return 0;
  *out_address = address + offset;
  *out_offset = (VkDeviceSize)offset;
  return 1;
}

static VkDeviceAddress prom_ray_buffer_address(VkDevice device, VkBuffer buffer) {
  VkBufferDeviceAddressInfo info;
  if (device == VK_NULL_HANDLE || buffer == VK_NULL_HANDLE) return 0u;
  memset(&info, 0, sizeof(info));
  info.sType = VK_STRUCTURE_TYPE_BUFFER_DEVICE_ADDRESS_INFO;
  info.buffer = buffer;
  return vkGetBufferDeviceAddress(device, &info);
}

static int prom_ray_triangle_is_valid(const PrometheusRayQueryTriangle* triangle) {
  float ax, ay, az, bx, by, bz, cx, cy, cz;
  float abx, aby, abz, acx, acy, acz;
  float nx, ny, nz;
  if (triangle == NULL) return 0;
  for (uint32_t index = 0u; index < 9u; ++index) {
    if (!isfinite(triangle->positions[index])) return 0;
  }
  ax = triangle->positions[0u]; ay = triangle->positions[1u]; az = triangle->positions[2u];
  bx = triangle->positions[3u]; by = triangle->positions[4u]; bz = triangle->positions[5u];
  cx = triangle->positions[6u]; cy = triangle->positions[7u]; cz = triangle->positions[8u];
  abx = bx - ax; aby = by - ay; abz = bz - az;
  acx = cx - ax; acy = cy - ay; acz = cz - az;
  nx = aby * acz - abz * acy;
  ny = abz * acx - abx * acz;
  nz = abx * acy - aby * acx;
  return isfinite(nx) && isfinite(ny) && isfinite(nz) && (nx * nx + ny * ny + nz * nz) > 1.0e-16f;
}

static int prom_ray_validate_request(const PrometheusRayQueryTriangleSceneCreateRequest* request,
                                     uint64_t* out_vertex_bytes) {
  uint64_t triangle_bytes;
  if (out_vertex_bytes == NULL || request == NULL ||
      request->struct_size < sizeof(PrometheusRayQueryTriangleSceneCreateRequest) ||
      request->triangles == NULL || request->triangle_count == 0u) return 0;
  if (!prom_ray_mul_u64((uint64_t)request->triangle_count, sizeof(PrometheusRayQueryTriangle), &triangle_bytes) ||
      triangle_bytes > UINT32_MAX * 4096ull) return 0;
  for (uint32_t index = 0u; index < request->triangle_count; ++index) {
    if (!prom_ray_triangle_is_valid(&request->triangles[index])) return 0;
  }
  *out_vertex_bytes = triangle_bytes;
  return 1;
}

static int prom_ray_sphere_is_valid(const PrometheusRayQuerySphere* sphere) {
  float minimum, maximum;
  if (sphere == NULL || !isfinite(sphere->radius) || sphere->radius <= 0.0f) return 0;
  for (uint32_t i = 0u; i < 3u; ++i) {
    if (!isfinite(sphere->center[i]) || !isfinite(sphere->albedo[i])) return 0;
    minimum = sphere->center[i] - sphere->radius;
    maximum = sphere->center[i] + sphere->radius;
    if (!isfinite(minimum) || !isfinite(maximum)) return 0;
  }
  return 1;
}

static int prom_ray_validate_scene_request(const PrometheusRayQuerySceneCreateRequest* request,
                                           uint64_t* out_vertex_bytes, uint64_t* out_sphere_bytes,
                                           uint64_t* out_aabb_bytes) {
  if (request == NULL || out_vertex_bytes == NULL || out_sphere_bytes == NULL || out_aabb_bytes == NULL ||
      request->struct_size < sizeof(*request) ||
      (request->triangle_count == 0u && request->sphere_count == 0u) ||
      (request->triangle_count != 0u && request->triangles == NULL) ||
      (request->sphere_count != 0u && request->spheres == NULL)) return 0;
  if (!prom_ray_mul_u64(request->triangle_count, sizeof(PrometheusRayQueryTriangle), out_vertex_bytes) ||
      !prom_ray_mul_u64(request->sphere_count, 2u * sizeof(float) * 4u, out_sphere_bytes) ||
      !prom_ray_mul_u64(request->sphere_count, 6u * sizeof(float), out_aabb_bytes)) return 0;
  if (*out_vertex_bytes > UINT32_MAX * 4096ull || *out_sphere_bytes > UINT32_MAX * 4096ull ||
      *out_aabb_bytes > UINT32_MAX * 4096ull) return 0;
  for (uint32_t i = 0u; i < request->triangle_count; ++i) if (!prom_ray_triangle_is_valid(&request->triangles[i])) return 0;
  for (uint32_t i = 0u; i < request->sphere_count; ++i) if (!prom_ray_sphere_is_valid(&request->spheres[i])) return 0;
  return 1;
}

static int prom_ray_submit_command(const prom_ray_query_scene* scene, VkCommandBuffer command_buffer) {
  VkFence fence = VK_NULL_HANDLE;
  VkFenceCreateInfo fence_info;
  VkSubmitInfo submit_info;
  VkResult result;
  if (scene == NULL || command_buffer == VK_NULL_HANDLE) return 0;
  memset(&fence_info, 0, sizeof(fence_info));
  fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
  result = vkCreateFence(scene->services.device, &fence_info, NULL, &fence);
  if (result != VK_SUCCESS) return 0;
  memset(&submit_info, 0, sizeof(submit_info));
  submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit_info.commandBufferCount = 1u;
  submit_info.pCommandBuffers = &command_buffer;
  result = vkQueueSubmit(scene->services.compute_queue, 1u, &submit_info, fence);
  if (result == VK_SUCCESS) {
    result = vkWaitForFences(scene->services.device, 1u, &fence, VK_TRUE, UINT64_MAX);
  }
  vkDestroyFence(scene->services.device, fence, NULL);
  return result == VK_SUCCESS;
}

static int prom_ray_begin_command(const prom_ray_query_scene* scene, VkCommandBuffer* out_command_buffer) {
  VkCommandBufferAllocateInfo allocate_info;
  VkCommandBufferBeginInfo begin_info;
  VkResult result;
  if (scene == NULL || out_command_buffer == NULL) return 0;
  *out_command_buffer = VK_NULL_HANDLE;
  memset(&allocate_info, 0, sizeof(allocate_info));
  allocate_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO;
  allocate_info.commandPool = scene->services.compute_command_pool;
  allocate_info.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;
  allocate_info.commandBufferCount = 1u;
  result = vkAllocateCommandBuffers(scene->services.device, &allocate_info, out_command_buffer);
  if (result != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  result = vkBeginCommandBuffer(*out_command_buffer, &begin_info);
  if (result != VK_SUCCESS) {
    vkFreeCommandBuffers(scene->services.device, scene->services.compute_command_pool, 1u, out_command_buffer);
    *out_command_buffer = VK_NULL_HANDLE;
    return 0;
  }
  return 1;
}

static int prom_ray_end_submit_and_free(const prom_ray_query_scene* scene, VkCommandBuffer command_buffer) {
  VkResult result;
  int submitted;
  if (scene == NULL || command_buffer == VK_NULL_HANDLE) return 0;
  result = vkEndCommandBuffer(command_buffer);
  submitted = result == VK_SUCCESS && prom_ray_submit_command(scene, command_buffer);
  vkFreeCommandBuffers(scene->services.device, scene->services.compute_command_pool, 1u, &command_buffer);
  return submitted;
}

static void prom_ray_destroy_as(const prom_ray_query_scene* scene, prom_ray_query_as* acceleration_structure) {
  if (scene == NULL || acceleration_structure == NULL) return;
  if (acceleration_structure->handle != VK_NULL_HANDLE && scene->services.destroy_acceleration_structure != NULL) {
    scene->services.destroy_acceleration_structure(scene->services.device, acceleration_structure->handle, NULL);
  }
  acceleration_structure->handle = VK_NULL_HANDLE;
  acceleration_structure->device_address = 0u;
  prom_vk_destroy_buffer(scene->services.device, &acceleration_structure->storage);
}

static void prom_ray_scene_destroy(prom_ray_query_scene* scene) {
  if (scene == NULL) return;
  if (scene->committed_scene != NULL) {
    prom_ray_scene_destroy(scene->committed_scene);
    scene->committed_scene = NULL;
  }
  if (scene->pipeline != VK_NULL_HANDLE) vkDestroyPipeline(scene->services.device, scene->pipeline, NULL);
  if (scene->pipeline_layout != VK_NULL_HANDLE) vkDestroyPipelineLayout(scene->services.device, scene->pipeline_layout, NULL);
  if (scene->descriptor_pool != VK_NULL_HANDLE) vkDestroyDescriptorPool(scene->services.device, scene->descriptor_pool, NULL);
  if (scene->descriptor_set_layout != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(scene->services.device, scene->descriptor_set_layout, NULL);
  prom_vk_destroy_buffer(scene->services.device, &scene->raw_hit_buffer);
  prom_vk_destroy_buffer(scene->services.device, &scene->ray_buffer);
  prom_vk_destroy_buffer(scene->services.device, &scene->evidence_buffer);
  prom_ray_destroy_as(scene, &scene->tlas);
  prom_vk_destroy_buffer(scene->services.device, &scene->instance_buffer);
  prom_ray_destroy_as(scene, &scene->procedural_blas);
  prom_ray_destroy_as(scene, &scene->triangle_blas);
  prom_vk_destroy_buffer(scene->services.device, &scene->aabb_buffer);
  prom_vk_destroy_buffer(scene->services.device, &scene->sphere_buffer);
  prom_vk_destroy_buffer(scene->services.device, &scene->triangle_shader_buffer);
  prom_vk_destroy_buffer(scene->services.device, &scene->vertex_buffer);
  free(scene->staged_triangles);
  free(scene->staged_spheres);
  scene->magic = 0u;
  free(scene);
}

static int prom_ray_create_as(prom_ray_query_scene* scene,
                              VkAccelerationStructureBuildGeometryInfoKHR* build_info,
                              const uint32_t* primitive_counts,
                              const VkAccelerationStructureBuildRangeInfoKHR* build_range,
                              prom_ray_query_as* out_as) {
  VkAccelerationStructureBuildSizesInfoKHR sizes;
  VkAccelerationStructureCreateInfoKHR create_info;
  VkPhysicalDeviceAccelerationStructurePropertiesKHR properties;
  VkPhysicalDeviceProperties2 properties2;
  prom_vk_buffer scratch;
  VkDeviceAddress scratch_base;
  VkDeviceAddress scratch_address;
  VkDeviceSize scratch_offset;
  VkCommandBuffer command_buffer = VK_NULL_HANDLE;
  VkResult result;
  const VkAccelerationStructureBuildRangeInfoKHR* ranges[1];
  uint64_t allocation_bytes;
  if (scene == NULL || build_info == NULL || primitive_counts == NULL || build_range == NULL || out_as == NULL) return 0;
  memset(out_as, 0, sizeof(*out_as));
  memset(&sizes, 0, sizeof(sizes));
  sizes.sType = VK_STRUCTURE_TYPE_ACCELERATION_STRUCTURE_BUILD_SIZES_INFO_KHR;
  scene->services.get_acceleration_structure_build_sizes(
      scene->services.device, VK_ACCELERATION_STRUCTURE_BUILD_TYPE_DEVICE_KHR,
      build_info, primitive_counts, &sizes);
  if (sizes.accelerationStructureSize == 0u || sizes.buildScratchSize == 0u) return 0;
  result = prom_vk_create_device_address_buffer(
      scene->services.physical_device, scene->services.device, scene->services.test_flags,
      sizes.accelerationStructureSize, VK_BUFFER_USAGE_ACCELERATION_STRUCTURE_STORAGE_BIT_KHR,
      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &out_as->storage);
  if (result != VK_SUCCESS) return 0;
  memset(&create_info, 0, sizeof(create_info));
  create_info.sType = VK_STRUCTURE_TYPE_ACCELERATION_STRUCTURE_CREATE_INFO_KHR;
  create_info.buffer = out_as->storage.buffer;
  create_info.size = sizes.accelerationStructureSize;
  create_info.type = build_info->type;
  result = scene->services.create_acceleration_structure(scene->services.device, &create_info, NULL, &out_as->handle);
  if (result != VK_SUCCESS) return 0;

  memset(&properties, 0, sizeof(properties));
  properties.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_ACCELERATION_STRUCTURE_PROPERTIES_KHR;
  memset(&properties2, 0, sizeof(properties2));
  properties2.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_PROPERTIES_2;
  properties2.pNext = &properties;
  vkGetPhysicalDeviceProperties2(scene->services.physical_device, &properties2);
  if (properties.minAccelerationStructureScratchOffsetAlignment == 0u ||
      !prom_ray_add_u64((uint64_t)sizes.buildScratchSize,
                        (uint64_t)properties.minAccelerationStructureScratchOffsetAlignment - 1u,
                        &allocation_bytes)) return 0;
  memset(&scratch, 0, sizeof(scratch));
  result = prom_vk_create_device_address_buffer(
      scene->services.physical_device, scene->services.device, scene->services.test_flags,
      (VkDeviceSize)allocation_bytes, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &scratch);
  if (result != VK_SUCCESS) return 0;
  scratch_base = prom_ray_buffer_address(scene->services.device, scratch.buffer);
  if (scratch_base == 0u ||
      !prom_ray_align_address(scratch_base, properties.minAccelerationStructureScratchOffsetAlignment,
                              &scratch_address, &scratch_offset) ||
      scratch_offset > allocation_bytes || sizes.buildScratchSize > allocation_bytes - scratch_offset) {
    prom_vk_destroy_buffer(scene->services.device, &scratch);
    return 0;
  }
  build_info->mode = VK_BUILD_ACCELERATION_STRUCTURE_MODE_BUILD_KHR;
  build_info->dstAccelerationStructure = out_as->handle;
  build_info->scratchData.deviceAddress = scratch_address;
  if (!prom_ray_begin_command(scene, &command_buffer)) {
    prom_vk_destroy_buffer(scene->services.device, &scratch);
    return 0;
  }
  ranges[0] = build_range;
  scene->services.cmd_build_acceleration_structures(command_buffer, 1u, build_info, ranges);
  if (!prom_ray_end_submit_and_free(scene, command_buffer)) {
    prom_vk_destroy_buffer(scene->services.device, &scratch);
    return 0;
  }
  prom_vk_destroy_buffer(scene->services.device, &scratch);
  {
    VkAccelerationStructureDeviceAddressInfoKHR address_info;
    memset(&address_info, 0, sizeof(address_info));
    address_info.sType = VK_STRUCTURE_TYPE_ACCELERATION_STRUCTURE_DEVICE_ADDRESS_INFO_KHR;
    address_info.accelerationStructure = out_as->handle;
    out_as->device_address = scene->services.get_acceleration_structure_device_address(scene->services.device, &address_info);
  }
  return out_as->device_address != 0u;
}

static int prom_ray_build_triangle_blas(prom_ray_query_scene* scene) {
  VkAccelerationStructureGeometryTrianglesDataKHR triangles;
  VkAccelerationStructureGeometryKHR geometry;
  VkAccelerationStructureBuildGeometryInfoKHR build_info;
  VkAccelerationStructureBuildRangeInfoKHR range;
  uint32_t primitive_count;
  if (scene == NULL || scene->triangle_count == 0u) return 0;
  memset(&triangles, 0, sizeof(triangles));
  triangles.sType = VK_STRUCTURE_TYPE_ACCELERATION_STRUCTURE_GEOMETRY_TRIANGLES_DATA_KHR;
  triangles.vertexFormat = VK_FORMAT_R32G32B32_SFLOAT;
  triangles.vertexData.deviceAddress = prom_ray_buffer_address(scene->services.device, scene->vertex_buffer.buffer);
  triangles.vertexStride = 3u * sizeof(float);
  triangles.maxVertex = scene->triangle_count * 3u - 1u;
  triangles.indexType = VK_INDEX_TYPE_NONE_KHR;
  if (triangles.vertexData.deviceAddress == 0u) return 0;
  memset(&geometry, 0, sizeof(geometry));
  geometry.sType = VK_STRUCTURE_TYPE_ACCELERATION_STRUCTURE_GEOMETRY_KHR;
  geometry.geometryType = VK_GEOMETRY_TYPE_TRIANGLES_KHR;
  geometry.flags = VK_GEOMETRY_OPAQUE_BIT_KHR;
  geometry.geometry.triangles = triangles;
  memset(&build_info, 0, sizeof(build_info));
  build_info.sType = VK_STRUCTURE_TYPE_ACCELERATION_STRUCTURE_BUILD_GEOMETRY_INFO_KHR;
  build_info.type = VK_ACCELERATION_STRUCTURE_TYPE_BOTTOM_LEVEL_KHR;
  build_info.flags = VK_BUILD_ACCELERATION_STRUCTURE_PREFER_FAST_TRACE_BIT_KHR;
  build_info.geometryCount = 1u;
  build_info.pGeometries = &geometry;
  memset(&range, 0, sizeof(range));
  range.primitiveCount = scene->triangle_count;
  primitive_count = scene->triangle_count;
  return prom_ray_create_as(scene, &build_info, &primitive_count, &range, &scene->triangle_blas);
}

static int prom_ray_build_procedural_blas(prom_ray_query_scene* scene) {
  VkAccelerationStructureGeometryAabbsDataKHR aabbs;
  VkAccelerationStructureGeometryKHR geometry;
  VkAccelerationStructureBuildGeometryInfoKHR build_info;
  VkAccelerationStructureBuildRangeInfoKHR range;
  uint32_t primitive_count;
  if (scene == NULL || scene->sphere_count == 0u) return 0;
  memset(&aabbs, 0, sizeof(aabbs));
  aabbs.sType = VK_STRUCTURE_TYPE_ACCELERATION_STRUCTURE_GEOMETRY_AABBS_DATA_KHR;
  aabbs.data.deviceAddress = prom_ray_buffer_address(scene->services.device, scene->aabb_buffer.buffer);
  aabbs.stride = 6u * sizeof(float);
  if (aabbs.data.deviceAddress == 0u) return 0;
  memset(&geometry, 0, sizeof(geometry));
  geometry.sType = VK_STRUCTURE_TYPE_ACCELERATION_STRUCTURE_GEOMETRY_KHR;
  geometry.geometryType = VK_GEOMETRY_TYPE_AABBS_KHR;
  geometry.flags = VK_GEOMETRY_OPAQUE_BIT_KHR;
  geometry.geometry.aabbs = aabbs;
  memset(&build_info, 0, sizeof(build_info));
  build_info.sType = VK_STRUCTURE_TYPE_ACCELERATION_STRUCTURE_BUILD_GEOMETRY_INFO_KHR;
  build_info.type = VK_ACCELERATION_STRUCTURE_TYPE_BOTTOM_LEVEL_KHR;
  build_info.flags = VK_BUILD_ACCELERATION_STRUCTURE_PREFER_FAST_TRACE_BIT_KHR;
  build_info.geometryCount = 1u;
  build_info.pGeometries = &geometry;
  memset(&range, 0, sizeof(range));
  range.primitiveCount = scene->sphere_count;
  primitive_count = scene->sphere_count;
  return prom_ray_create_as(scene, &build_info, &primitive_count, &range, &scene->procedural_blas);
}

static int prom_ray_build_tlas(prom_ray_query_scene* scene) {
  VkAccelerationStructureInstanceKHR instances_data[2];
  VkAccelerationStructureGeometryInstancesDataKHR instances;
  VkAccelerationStructureGeometryKHR geometry;
  VkAccelerationStructureBuildGeometryInfoKHR build_info;
  VkAccelerationStructureBuildRangeInfoKHR range;
  uint32_t primitive_count;
  uint32_t instance_count;
  VkDeviceAddress instance_address;
  if (scene == NULL) return 0;
  instance_count = (scene->triangle_count != 0u ? 1u : 0u) + (scene->sphere_count != 0u ? 1u : 0u);
  if (instance_count == 0u || (scene->triangle_count != 0u && scene->triangle_blas.device_address == 0u) ||
      (scene->sphere_count != 0u && scene->procedural_blas.device_address == 0u)) return 0;
  memset(instances_data, 0, sizeof(instances_data));
  if (scene->triangle_count != 0u) {
    instances_data[0].transform.matrix[0][0] = 1.0f;
    instances_data[0].transform.matrix[1][1] = 1.0f;
    instances_data[0].transform.matrix[2][2] = 1.0f;
    instances_data[0].instanceCustomIndex = 0u;
    instances_data[0].mask = 0xffu;
    instances_data[0].flags = VK_GEOMETRY_INSTANCE_TRIANGLE_FACING_CULL_DISABLE_BIT_KHR;
    instances_data[0].accelerationStructureReference = scene->triangle_blas.device_address;
  }
  if (scene->sphere_count != 0u) {
    uint32_t slot = scene->triangle_count != 0u ? 1u : 0u;
    instances_data[slot].transform.matrix[0][0] = 1.0f;
    instances_data[slot].transform.matrix[1][1] = 1.0f;
    instances_data[slot].transform.matrix[2][2] = 1.0f;
    instances_data[slot].instanceCustomIndex = 1u;
    instances_data[slot].mask = 0xffu;
    instances_data[slot].flags = VK_GEOMETRY_INSTANCE_TRIANGLE_FACING_CULL_DISABLE_BIT_KHR;
    instances_data[slot].accelerationStructureReference = scene->procedural_blas.device_address;
  }
  if (prom_vk_create_device_address_buffer(
          scene->services.physical_device, scene->services.device, scene->services.test_flags,
          sizeof(VkAccelerationStructureInstanceKHR) * instance_count, VK_BUFFER_USAGE_ACCELERATION_STRUCTURE_BUILD_INPUT_READ_ONLY_BIT_KHR,
          VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1,
          &scene->instance_buffer) != VK_SUCCESS || scene->instance_buffer.mapped == NULL) return 0;
  memcpy(scene->instance_buffer.mapped, instances_data, sizeof(VkAccelerationStructureInstanceKHR) * instance_count);
  instance_address = prom_ray_buffer_address(scene->services.device, scene->instance_buffer.buffer);
  if (instance_address == 0u) return 0;
  memset(&instances, 0, sizeof(instances));
  instances.sType = VK_STRUCTURE_TYPE_ACCELERATION_STRUCTURE_GEOMETRY_INSTANCES_DATA_KHR;
  instances.arrayOfPointers = VK_FALSE;
  instances.data.deviceAddress = instance_address;
  memset(&geometry, 0, sizeof(geometry));
  geometry.sType = VK_STRUCTURE_TYPE_ACCELERATION_STRUCTURE_GEOMETRY_KHR;
  geometry.geometryType = VK_GEOMETRY_TYPE_INSTANCES_KHR;
  geometry.geometry.instances = instances;
  memset(&build_info, 0, sizeof(build_info));
  build_info.sType = VK_STRUCTURE_TYPE_ACCELERATION_STRUCTURE_BUILD_GEOMETRY_INFO_KHR;
  build_info.type = VK_ACCELERATION_STRUCTURE_TYPE_TOP_LEVEL_KHR;
  build_info.flags = VK_BUILD_ACCELERATION_STRUCTURE_PREFER_FAST_TRACE_BIT_KHR;
  build_info.geometryCount = 1u;
  build_info.pGeometries = &geometry;
  memset(&range, 0, sizeof(range));
  range.primitiveCount = instance_count;
  primitive_count = instance_count;
  return prom_ray_create_as(scene, &build_info, &primitive_count, &range, &scene->tlas);
}

static int prom_ray_create_compute_resources(prom_ray_query_scene* scene) {
  prom_shader_package* package;
  VkDescriptorSetLayoutBinding bindings[2];
  VkDescriptorSetLayoutCreateInfo layout_info;
  VkPipelineLayoutCreateInfo pipeline_layout_info;
  VkDescriptorPoolSize pool_sizes[2];
  VkDescriptorPoolCreateInfo pool_info;
  VkDescriptorSetAllocateInfo set_allocate_info;
  VkWriteDescriptorSet writes[2];
  VkWriteDescriptorSetAccelerationStructureKHR acceleration_write;
  VkDescriptorBufferInfo evidence_info;
  VkShaderModule module = VK_NULL_HANDLE;
  const char* entry_point = NULL;
  prom_shader_package_diagnostic package_diagnostic;
  VkPipelineShaderStageCreateInfo stage_info;
  VkComputePipelineCreateInfo pipeline_info;
  VkResult result;
  if (scene == NULL || scene->tlas.handle == VK_NULL_HANDLE) return 0;
  if (prom_reactor_runtime_get_shader_package(scene->runtime_handle, &package) != PROM_OK) return 0;
  if (prom_vk_create_buffer(scene->services.physical_device, scene->services.device, scene->services.test_flags,
                            sizeof(uint32_t), VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                            VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1,
                            &scene->evidence_buffer) != VK_SUCCESS || scene->evidence_buffer.mapped == NULL) return 0;
  memset(bindings, 0, sizeof(bindings));
  bindings[0].binding = 0u;
  bindings[0].descriptorType = VK_DESCRIPTOR_TYPE_ACCELERATION_STRUCTURE_KHR;
  bindings[0].descriptorCount = 1u;
  bindings[0].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  bindings[1].binding = 1u;
  bindings[1].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  bindings[1].descriptorCount = 1u;
  bindings[1].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  memset(&layout_info, 0, sizeof(layout_info));
  layout_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO;
  layout_info.bindingCount = 2u;
  layout_info.pBindings = bindings;
  result = vkCreateDescriptorSetLayout(scene->services.device, &layout_info, NULL, &scene->descriptor_set_layout);
  if (result != VK_SUCCESS) return 0;
  memset(&pipeline_layout_info, 0, sizeof(pipeline_layout_info));
  pipeline_layout_info.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO;
  pipeline_layout_info.setLayoutCount = 1u;
  pipeline_layout_info.pSetLayouts = &scene->descriptor_set_layout;
  result = vkCreatePipelineLayout(scene->services.device, &pipeline_layout_info, NULL, &scene->pipeline_layout);
  if (result != VK_SUCCESS) return 0;
  memset(pool_sizes, 0, sizeof(pool_sizes));
  pool_sizes[0].type = VK_DESCRIPTOR_TYPE_ACCELERATION_STRUCTURE_KHR;
  pool_sizes[0].descriptorCount = 1u;
  pool_sizes[1].type = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  pool_sizes[1].descriptorCount = 1u;
  memset(&pool_info, 0, sizeof(pool_info));
  pool_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO;
  pool_info.maxSets = 1u;
  pool_info.poolSizeCount = 2u;
  pool_info.pPoolSizes = pool_sizes;
  result = vkCreateDescriptorPool(scene->services.device, &pool_info, NULL, &scene->descriptor_pool);
  if (result != VK_SUCCESS) return 0;
  memset(&set_allocate_info, 0, sizeof(set_allocate_info));
  set_allocate_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO;
  set_allocate_info.descriptorPool = scene->descriptor_pool;
  set_allocate_info.descriptorSetCount = 1u;
  set_allocate_info.pSetLayouts = &scene->descriptor_set_layout;
  result = vkAllocateDescriptorSets(scene->services.device, &set_allocate_info, &scene->descriptor_set);
  if (result != VK_SUCCESS) return 0;
  memset(&acceleration_write, 0, sizeof(acceleration_write));
  acceleration_write.sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET_ACCELERATION_STRUCTURE_KHR;
  acceleration_write.accelerationStructureCount = 1u;
  acceleration_write.pAccelerationStructures = &scene->tlas.handle;
  memset(&evidence_info, 0, sizeof(evidence_info));
  evidence_info.buffer = scene->evidence_buffer.buffer;
  evidence_info.range = sizeof(uint32_t);
  memset(writes, 0, sizeof(writes));
  writes[0].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
  writes[0].pNext = &acceleration_write;
  writes[0].dstSet = scene->descriptor_set;
  writes[0].dstBinding = 0u;
  writes[0].descriptorCount = 1u;
  writes[0].descriptorType = VK_DESCRIPTOR_TYPE_ACCELERATION_STRUCTURE_KHR;
  writes[1].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
  writes[1].dstSet = scene->descriptor_set;
  writes[1].dstBinding = 1u;
  writes[1].descriptorCount = 1u;
  writes[1].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  writes[1].pBufferInfo = &evidence_info;
  vkUpdateDescriptorSets(scene->services.device, 2u, writes, 0u, NULL);
  if (!prom_shader_package_create_module(package, scene->services.device, "kernel-54-default", &module, &entry_point, &package_diagnostic)) return 0;
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = module;
  stage_info.pName = entry_point;
  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = scene->pipeline_layout;
  result = vkCreateComputePipelines(scene->services.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &scene->pipeline);
  vkDestroyShaderModule(scene->services.device, module, NULL);
  return result == VK_SUCCESS;
}

static int prom_ray_create_raw_compute_resources(prom_ray_query_scene* scene) {
  prom_shader_package* package;
  VkDescriptorSetLayoutBinding bindings[5];
  VkDescriptorSetLayoutCreateInfo layout_info;
  VkPipelineLayoutCreateInfo pipeline_layout_info;
  VkDescriptorPoolSize pool_sizes[2];
  VkDescriptorPoolCreateInfo pool_info;
  VkDescriptorSetAllocateInfo set_allocate_info;
  VkWriteDescriptorSet writes[5];
  VkWriteDescriptorSetAccelerationStructureKHR acceleration_write;
  VkDescriptorBufferInfo infos[4];
  VkPipelineShaderStageCreateInfo stage_info;
  VkComputePipelineCreateInfo pipeline_info;
  VkShaderModule module = VK_NULL_HANDLE;
  const char* entry_point = NULL;
  prom_shader_package_diagnostic package_diagnostic;
  VkResult result;
  if (scene == NULL || scene->tlas.handle == VK_NULL_HANDLE) return 0;
  if (prom_reactor_runtime_get_shader_package(scene->runtime_handle, &package) != PROM_OK) return 0;
  if (prom_vk_create_buffer(scene->services.physical_device, scene->services.device, scene->services.test_flags,
                            PROM_RAY_SHADER_BYTES_PER_RAY, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                            VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1, &scene->ray_buffer) != VK_SUCCESS ||
      prom_vk_create_buffer(scene->services.physical_device, scene->services.device, scene->services.test_flags,
                            sizeof(PrometheusRayQueryRawHit), VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                            VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1, &scene->raw_hit_buffer) != VK_SUCCESS ||
      scene->ray_buffer.mapped == NULL || scene->raw_hit_buffer.mapped == NULL) return 0;
  scene->batch_capacity = 1u;
  memset(bindings, 0, sizeof(bindings));
  for (uint32_t i = 0u; i < 5u; ++i) { bindings[i].binding = i; bindings[i].descriptorCount = 1u; bindings[i].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT; bindings[i].descriptorType = i == 0u ? VK_DESCRIPTOR_TYPE_ACCELERATION_STRUCTURE_KHR : VK_DESCRIPTOR_TYPE_STORAGE_BUFFER; }
  memset(&layout_info, 0, sizeof(layout_info)); layout_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO; layout_info.bindingCount = 5u; layout_info.pBindings = bindings;
  if (vkCreateDescriptorSetLayout(scene->services.device, &layout_info, NULL, &scene->descriptor_set_layout) != VK_SUCCESS) return 0;
  memset(&pipeline_layout_info, 0, sizeof(pipeline_layout_info)); pipeline_layout_info.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO; pipeline_layout_info.setLayoutCount = 1u; pipeline_layout_info.pSetLayouts = &scene->descriptor_set_layout;
  if (vkCreatePipelineLayout(scene->services.device, &pipeline_layout_info, NULL, &scene->pipeline_layout) != VK_SUCCESS) return 0;
  memset(pool_sizes, 0, sizeof(pool_sizes)); pool_sizes[0].type = VK_DESCRIPTOR_TYPE_ACCELERATION_STRUCTURE_KHR; pool_sizes[0].descriptorCount = 1u; pool_sizes[1].type = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER; pool_sizes[1].descriptorCount = 4u;
  memset(&pool_info, 0, sizeof(pool_info)); pool_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO; pool_info.maxSets = 1u; pool_info.poolSizeCount = 2u; pool_info.pPoolSizes = pool_sizes;
  if (vkCreateDescriptorPool(scene->services.device, &pool_info, NULL, &scene->descriptor_pool) != VK_SUCCESS) return 0;
  memset(&set_allocate_info, 0, sizeof(set_allocate_info)); set_allocate_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO; set_allocate_info.descriptorPool = scene->descriptor_pool; set_allocate_info.descriptorSetCount = 1u; set_allocate_info.pSetLayouts = &scene->descriptor_set_layout;
  if (vkAllocateDescriptorSets(scene->services.device, &set_allocate_info, &scene->descriptor_set) != VK_SUCCESS) return 0;
  memset(&acceleration_write, 0, sizeof(acceleration_write)); acceleration_write.sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET_ACCELERATION_STRUCTURE_KHR; acceleration_write.accelerationStructureCount = 1u; acceleration_write.pAccelerationStructures = &scene->tlas.handle;
  memset(infos, 0, sizeof(infos));
  infos[0].buffer = scene->sphere_buffer.buffer; infos[0].range = VK_WHOLE_SIZE;
  infos[1].buffer = scene->ray_buffer.buffer; infos[1].range = VK_WHOLE_SIZE;
  infos[2].buffer = scene->raw_hit_buffer.buffer; infos[2].range = VK_WHOLE_SIZE;
  infos[3].buffer = scene->triangle_shader_buffer.buffer; infos[3].range = VK_WHOLE_SIZE;
  memset(writes, 0, sizeof(writes));
  writes[0].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET; writes[0].pNext = &acceleration_write; writes[0].dstSet = scene->descriptor_set; writes[0].dstBinding = 0u; writes[0].descriptorCount = 1u; writes[0].descriptorType = VK_DESCRIPTOR_TYPE_ACCELERATION_STRUCTURE_KHR;
  for (uint32_t i = 1u; i < 5u; ++i) { writes[i].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET; writes[i].dstSet = scene->descriptor_set; writes[i].dstBinding = i; writes[i].descriptorCount = 1u; writes[i].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER; writes[i].pBufferInfo = &infos[i - 1u]; }
  vkUpdateDescriptorSets(scene->services.device, 5u, writes, 0u, NULL);
  if (!prom_shader_package_create_module(package, scene->services.device, "kernel-55-default", &module, &entry_point, &package_diagnostic)) return 0;
  memset(&stage_info, 0, sizeof(stage_info)); stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO; stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT; stage_info.module = module; stage_info.pName = entry_point;
  memset(&pipeline_info, 0, sizeof(pipeline_info)); pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO; pipeline_info.stage = stage_info; pipeline_info.layout = scene->pipeline_layout;
  result = vkCreateComputePipelines(scene->services.device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &scene->pipeline);
  vkDestroyShaderModule(scene->services.device, module, NULL);
  return result == VK_SUCCESS;
}

static int prom_ray_scene_register(prom_ray_query_scene* scene) {
  uint32_t index;
  if (scene == NULL) return 0;
  prom_ray_scene_lock();
  for (index = 0u; index < PROM_RAY_QUERY_MAX_LIVE_SCENES; ++index) {
    if (g_ray_query_scenes[index] == NULL) {
      g_ray_query_scenes[index] = scene;
      prom_ray_scene_unlock();
      return 1;
    }
  }
  prom_ray_scene_unlock();
  return 0;
}

static prom_ray_query_scene* prom_ray_scene_take(void* runtime_handle, uint64_t scene_id) {
  prom_ray_query_scene* scene = NULL;
  uint32_t index;
  prom_ray_scene_lock();
  for (index = 0u; index < PROM_RAY_QUERY_MAX_LIVE_SCENES; ++index) {
    if (g_ray_query_scenes[index] != NULL && (uint64_t)(uintptr_t)g_ray_query_scenes[index] == scene_id &&
        g_ray_query_scenes[index]->runtime_handle == runtime_handle) {
      scene = g_ray_query_scenes[index];
      g_ray_query_scenes[index] = NULL;
      break;
    }
  }
  prom_ray_scene_unlock();
  return scene;
}

static prom_ray_query_scene* prom_ray_scene_find(void* runtime_handle, uint64_t scene_id) {
  prom_ray_query_scene* scene = NULL;
  uint32_t index;
  prom_ray_scene_lock();
  for (index = 0u; index < PROM_RAY_QUERY_MAX_LIVE_SCENES; ++index) {
    if (g_ray_query_scenes[index] != NULL && (uint64_t)(uintptr_t)g_ray_query_scenes[index] == scene_id &&
        g_ray_query_scenes[index]->runtime_handle == runtime_handle) {
      scene = g_ray_query_scenes[index];
      break;
    }
  }
  prom_ray_scene_unlock();
  return scene;
}

int prom_ray_query_triangle_scene_create_impl(
    void* handle, const PrometheusRayQueryTriangleSceneCreateRequest* request, uint64_t* out_scene_id) {
  prom_ray_query_scene* scene = NULL;
  prom_vk_runtime_services services;
  uint64_t vertex_bytes;
  if (out_scene_id == NULL) return PROM_ERROR;
  *out_scene_id = 0u;
  if (!prom_ray_validate_request(request, &vertex_bytes)) return PROM_ERROR;
  if (prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) return PROM_INVALID_HANDLE;
  if (services.ray_query_state != PROM_VK_RAY_QUERY_DEVICE_FEATURE_ENABLED ||
      services.create_acceleration_structure == NULL || services.destroy_acceleration_structure == NULL ||
      services.get_acceleration_structure_build_sizes == NULL || services.cmd_build_acceleration_structures == NULL ||
      services.get_acceleration_structure_device_address == NULL) return PROM_ERROR;
  scene = (prom_ray_query_scene*)calloc(1u, sizeof(*scene));
  if (scene == NULL) return PROM_ERROR;
  scene->runtime_handle = handle;
  scene->services = services;
  scene->triangle_count = request->triangle_count;
  if (prom_vk_create_device_address_buffer(
          services.physical_device, services.device, services.test_flags, (VkDeviceSize)vertex_bytes,
          VK_BUFFER_USAGE_ACCELERATION_STRUCTURE_BUILD_INPUT_READ_ONLY_BIT_KHR,
          VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1,
          &scene->vertex_buffer) != VK_SUCCESS || scene->vertex_buffer.mapped == NULL) goto fail;
  memcpy(scene->vertex_buffer.mapped, request->triangles, (size_t)vertex_bytes);
  if (!prom_ray_build_triangle_blas(scene) || !prom_ray_build_tlas(scene) || !prom_ray_create_compute_resources(scene)) goto fail;
  scene->magic = PROM_RAY_QUERY_SCENE_MAGIC;
  if (!prom_ray_scene_register(scene)) goto fail;
  *out_scene_id = (uint64_t)(uintptr_t)scene;
  return PROM_OK;
fail:
  prom_ray_scene_destroy(scene);
  return PROM_ERROR;
}

int prom_ray_query_scene_create_impl(void* handle, const PrometheusRayQuerySceneCreateRequest* request, uint64_t* out_scene_id) {
  prom_ray_query_scene* scene = NULL;
  prom_vk_runtime_services services;
  uint64_t vertex_bytes, sphere_bytes, aabb_bytes;
  uint64_t shader_vertex_bytes;
  if (out_scene_id == NULL) return PROM_ERROR;
  *out_scene_id = 0u;
  if (!prom_ray_validate_scene_request(request, &vertex_bytes, &sphere_bytes, &aabb_bytes) ||
      !prom_ray_mul_u64(request->triangle_count == 0u ? 1u : request->triangle_count * 3u, 4u * sizeof(float), &shader_vertex_bytes)) return PROM_ERROR;
  if (prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) return PROM_INVALID_HANDLE;
  if (services.ray_query_state != PROM_VK_RAY_QUERY_DEVICE_FEATURE_ENABLED || services.create_acceleration_structure == NULL ||
      services.destroy_acceleration_structure == NULL || services.get_acceleration_structure_build_sizes == NULL ||
      services.cmd_build_acceleration_structures == NULL || services.get_acceleration_structure_device_address == NULL) return PROM_ERROR;
  scene = (prom_ray_query_scene*)calloc(1u, sizeof(*scene)); if (scene == NULL) return PROM_ERROR;
  scene->runtime_handle = handle; scene->services = services; scene->triangle_count = request->triangle_count; scene->sphere_count = request->sphere_count; scene->raw_scene = 1u;
  if (request->triangle_count != 0u) {
    if (prom_vk_create_device_address_buffer(services.physical_device, services.device, services.test_flags, (VkDeviceSize)vertex_bytes,
        VK_BUFFER_USAGE_ACCELERATION_STRUCTURE_BUILD_INPUT_READ_ONLY_BIT_KHR, VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1, &scene->vertex_buffer) != VK_SUCCESS || scene->vertex_buffer.mapped == NULL) goto fail;
    memcpy(scene->vertex_buffer.mapped, request->triangles, (size_t)vertex_bytes);
  }
  if (prom_vk_create_buffer(services.physical_device, services.device, services.test_flags, (VkDeviceSize)shader_vertex_bytes,
      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT, VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1, &scene->triangle_shader_buffer) != VK_SUCCESS || scene->triangle_shader_buffer.mapped == NULL) goto fail;
  memset(scene->triangle_shader_buffer.mapped, 0, (size_t)shader_vertex_bytes);
  for (uint32_t t = 0u; t < request->triangle_count; ++t) for (uint32_t v = 0u; v < 3u; ++v) {
    float* dst = (float*)scene->triangle_shader_buffer.mapped + (t * 3u + v) * 4u;
    memcpy(dst, &request->triangles[t].positions[v * 3u], 3u * sizeof(float));
  }
  if (prom_vk_create_buffer(services.physical_device, services.device, services.test_flags, (VkDeviceSize)(sphere_bytes == 0u ? 32u : sphere_bytes),
      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT, VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1, &scene->sphere_buffer) != VK_SUCCESS || scene->sphere_buffer.mapped == NULL) goto fail;
  memset(scene->sphere_buffer.mapped, 0, (size_t)(sphere_bytes == 0u ? 32u : sphere_bytes));
  if (request->sphere_count != 0u) {
    if (prom_vk_create_device_address_buffer(services.physical_device, services.device, services.test_flags, (VkDeviceSize)aabb_bytes,
        VK_BUFFER_USAGE_ACCELERATION_STRUCTURE_BUILD_INPUT_READ_ONLY_BIT_KHR, VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1, &scene->aabb_buffer) != VK_SUCCESS || scene->aabb_buffer.mapped == NULL) goto fail;
    for (uint32_t i = 0u; i < request->sphere_count; ++i) {
      const PrometheusRayQuerySphere* src = &request->spheres[i]; float* gpu = (float*)scene->sphere_buffer.mapped + i * 8u; float* aabb = (float*)scene->aabb_buffer.mapped + i * 6u;
      gpu[0]=src->center[0]; gpu[1]=src->center[1]; gpu[2]=src->center[2]; gpu[3]=src->radius; gpu[4]=src->albedo[0]; gpu[5]=src->albedo[1]; gpu[6]=src->albedo[2]; gpu[7]=(float)src->material_id;
      for (uint32_t axis=0u; axis<3u; ++axis) { aabb[axis]=src->center[axis]-src->radius; aabb[axis+3u]=src->center[axis]+src->radius; }
    }
  }
  if ((scene->triangle_count != 0u && !prom_ray_build_triangle_blas(scene)) ||
      (scene->sphere_count != 0u && !prom_ray_build_procedural_blas(scene)) || !prom_ray_build_tlas(scene) || !prom_ray_create_raw_compute_resources(scene)) goto fail;
  scene->magic = PROM_RAY_QUERY_SCENE_MAGIC; if (!prom_ray_scene_register(scene)) goto fail;
  *out_scene_id = (uint64_t)(uintptr_t)scene; return PROM_OK;
fail:
  prom_ray_scene_destroy(scene); return PROM_ERROR;
}

int prom_ray_query_triangle_scene_probe_impl(void* handle, uint64_t scene_id,
                                             PrometheusRayQueryProbeResult* out_result) {
  prom_ray_query_scene* scene;
  prom_vk_runtime_services services;
  VkCommandBuffer command_buffer = VK_NULL_HANDLE;
  uint32_t evidence = 0u;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  if (prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) return PROM_INVALID_HANDLE;
  scene = prom_ray_scene_find(handle, scene_id);
  if (scene == NULL || scene->magic != PROM_RAY_QUERY_SCENE_MAGIC || scene->services.device != services.device) return PROM_INVALID_HANDLE;
  if (scene->evidence_buffer.mapped == NULL || scene->pipeline == VK_NULL_HANDLE) return PROM_ERROR;
  memcpy(scene->evidence_buffer.mapped, &evidence, sizeof(evidence));
  if (!prom_ray_begin_command(scene, &command_buffer)) return PROM_ERROR;
  vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, scene->pipeline);
  vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, scene->pipeline_layout,
                          0u, 1u, &scene->descriptor_set, 0u, NULL);
  vkCmdDispatch(command_buffer, 1u, 1u, 1u);
  if (!prom_ray_end_submit_and_free(scene, command_buffer)) return PROM_ERROR;
  memcpy(&evidence, scene->evidence_buffer.mapped, sizeof(evidence));
  out_result->hit = evidence != 0u ? 1u : 0u;
  out_result->triangle_count = scene->triangle_count;
  out_result->blas_built = scene->triangle_blas.handle != VK_NULL_HANDLE ? 1u : 0u;
  out_result->tlas_built = scene->tlas.handle != VK_NULL_HANDLE ? 1u : 0u;
  out_result->vertex_device_address = scene->vertex_buffer.buffer != VK_NULL_HANDLE ?
      (uint64_t)prom_ray_buffer_address(scene->services.device, scene->vertex_buffer.buffer) : 0u;
  out_result->blas_device_address = (uint64_t)scene->triangle_blas.device_address;
  out_result->tlas_device_address = (uint64_t)scene->tlas.device_address;
  return PROM_OK;
}

static int prom_ray_scene_trace_direct(prom_ray_query_scene* scene,
                                       const PrometheusRayQueryRawRequest* request,
                                       uint32_t visibility_mask,
                                       PrometheusRayQueryRawHit* out_hit) {
  VkCommandBuffer command_buffer = VK_NULL_HANDLE;
  float* words;
  if (out_hit == NULL || request == NULL || request->struct_size < sizeof(*request)) return PROM_ERROR;
  memset(out_hit, 0, sizeof(*out_hit));
  if (!isfinite(request->t_min) || !isfinite(request->t_max) || request->t_min < 0.0f || request->t_max < request->t_min) return PROM_ERROR;
  for (uint32_t i=0u; i<3u; ++i) if (!isfinite(request->origin[i]) || !isfinite(request->direction[i])) return PROM_ERROR;
  if (request->direction[0]*request->direction[0] + request->direction[1]*request->direction[1] + request->direction[2]*request->direction[2] <= 0.0f) return PROM_ERROR;
  if (scene == NULL || scene->magic != PROM_RAY_QUERY_SCENE_MAGIC || !scene->raw_scene ||
      scene->ray_buffer.mapped == NULL || scene->raw_hit_buffer.mapped == NULL || scene->pipeline == VK_NULL_HANDLE) return PROM_INVALID_HANDLE;
  words = (float*)scene->ray_buffer.mapped;
  words[0]=request->origin[0]; words[1]=request->origin[1]; words[2]=request->origin[2]; words[3]=request->t_min;
  words[4]=request->direction[0]; words[5]=request->direction[1]; words[6]=request->direction[2]; words[7]=request->t_max;
  memcpy(&words[8], &visibility_mask, sizeof(visibility_mask));
  words[9]=0.0f; words[10]=0.0f; words[11]=0.0f;
  memset(scene->raw_hit_buffer.mapped, 0, sizeof(*out_hit));
  if (!prom_ray_begin_command(scene, &command_buffer)) return PROM_ERROR;
  vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, scene->pipeline);
  vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, scene->pipeline_layout, 0u, 1u, &scene->descriptor_set, 0u, NULL);
  vkCmdDispatch(command_buffer, 1u, 1u, 1u);
  if (!prom_ray_end_submit_and_free(scene, command_buffer)) return PROM_ERROR;
  memcpy(out_hit, scene->raw_hit_buffer.mapped, sizeof(*out_hit));
  return PROM_OK;
}

int prom_ray_query_scene_trace_impl(void* handle, uint64_t scene_id, const PrometheusRayQueryRawRequest* request,
                                    PrometheusRayQueryRawHit* out_hit) {
  prom_ray_query_scene* scene;
  prom_vk_runtime_services services;
  if (prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) return PROM_INVALID_HANDLE;
  scene = prom_ray_scene_find(handle, scene_id);
  if (scene == NULL || scene->services.device != services.device) return PROM_INVALID_HANDLE;
  return prom_ray_scene_trace_direct(scene, request, PROM_RAY_QUERY_VISIBILITY_MASK_ALL, out_hit);
}

static int prom_ray_scene_append_triangles(prom_ray_query_scene* scene,
                                           const PrometheusRayQueryTriangle* triangles,
                                           uint32_t triangle_count) {
  uint64_t required, bytes;
  uint32_t capacity;
  PrometheusRayQueryTriangle* grown;
  if (scene == NULL || (triangle_count != 0u && triangles == NULL)) return 0;
  if (triangle_count == 0u) return 1;
  for (uint32_t i = 0u; i < triangle_count; ++i) if (!prom_ray_triangle_is_valid(&triangles[i])) return 0;
  if (!prom_ray_add_u64(scene->staged_triangle_count, triangle_count, &required) || required > UINT32_MAX ||
      !prom_ray_mul_u64(required, sizeof(*triangles), &bytes) || bytes > SIZE_MAX) return 0;
  capacity = scene->staged_triangle_capacity == 0u ? 4u : scene->staged_triangle_capacity;
  while ((uint64_t)capacity < required) {
    if (capacity > UINT32_MAX / 2u) { capacity = (uint32_t)required; break; }
    capacity *= 2u;
  }
  if (capacity != scene->staged_triangle_capacity) {
    if (!prom_ray_mul_u64(capacity, sizeof(*triangles), &bytes) || bytes > SIZE_MAX) return 0;
    grown = (PrometheusRayQueryTriangle*)realloc(scene->staged_triangles, (size_t)bytes);
    if (grown == NULL) return 0;
    scene->staged_triangles = grown;
    scene->staged_triangle_capacity = capacity;
  }
  memcpy(&scene->staged_triangles[scene->staged_triangle_count], triangles,
         (size_t)triangle_count * sizeof(*triangles));
  scene->staged_triangle_count = (uint32_t)required;
  return 1;
}

static int prom_ray_scene_append_spheres(prom_ray_query_scene* scene,
                                         const PrometheusRayQuerySphere* spheres,
                                         uint32_t sphere_count) {
  uint64_t required, bytes;
  uint32_t capacity;
  PrometheusRayQuerySphere* grown;
  if (scene == NULL || (sphere_count != 0u && spheres == NULL)) return 0;
  if (sphere_count == 0u) return 1;
  for (uint32_t i = 0u; i < sphere_count; ++i) if (!prom_ray_sphere_is_valid(&spheres[i])) return 0;
  if (!prom_ray_add_u64(scene->staged_sphere_count, sphere_count, &required) || required > UINT32_MAX ||
      !prom_ray_mul_u64(required, sizeof(*spheres), &bytes) || bytes > SIZE_MAX) return 0;
  capacity = scene->staged_sphere_capacity == 0u ? 4u : scene->staged_sphere_capacity;
  while ((uint64_t)capacity < required) {
    if (capacity > UINT32_MAX / 2u) { capacity = (uint32_t)required; break; }
    capacity *= 2u;
  }
  if (capacity != scene->staged_sphere_capacity) {
    if (!prom_ray_mul_u64(capacity, sizeof(*spheres), &bytes) || bytes > SIZE_MAX) return 0;
    grown = (PrometheusRayQuerySphere*)realloc(scene->staged_spheres, (size_t)bytes);
    if (grown == NULL) return 0;
    scene->staged_spheres = grown;
    scene->staged_sphere_capacity = capacity;
  }
  memcpy(&scene->staged_spheres[scene->staged_sphere_count], spheres,
         (size_t)sphere_count * sizeof(*spheres));
  scene->staged_sphere_count = (uint32_t)required;
  return 1;
}

int prom_ray_query_scene_create_empty_impl(void* handle, uint64_t* out_scene_id) {
  prom_ray_query_scene* scene;
  if (out_scene_id == NULL) return PROM_ERROR;
  *out_scene_id = 0u;
  if (!prom_reactor_runtime_validate_handle(handle)) return PROM_INVALID_HANDLE;
  scene = (prom_ray_query_scene*)calloc(1u, sizeof(*scene));
  if (scene == NULL) return PROM_ERROR;
  scene->magic = PROM_RAY_QUERY_SCENE_MAGIC;
  scene->runtime_handle = handle;
  if (!prom_ray_scene_register(scene)) { prom_ray_scene_destroy(scene); return PROM_ERROR; }
  *out_scene_id = (uint64_t)(uintptr_t)scene;
  return PROM_OK;
}

int prom_ray_query_scene_add_triangles_impl(void* handle, uint64_t scene_id,
                                            const PrometheusRayQueryTriangle* triangles,
                                            uint32_t triangle_count) {
  prom_ray_query_scene* scene;
  if (!prom_reactor_runtime_validate_handle(handle)) return PROM_INVALID_HANDLE;
  scene = prom_ray_scene_find(handle, scene_id);
  if (scene == NULL || scene->magic != PROM_RAY_QUERY_SCENE_MAGIC) return PROM_INVALID_HANDLE;
  if (scene->committed || scene->raw_scene) return PROM_ERROR;
  return prom_ray_scene_append_triangles(scene, triangles, triangle_count) ? PROM_OK : PROM_ERROR;
}

int prom_ray_query_scene_add_spheres_impl(void* handle, uint64_t scene_id,
                                          const PrometheusRayQuerySphere* spheres,
                                          uint32_t sphere_count) {
  prom_ray_query_scene* scene;
  if (!prom_reactor_runtime_validate_handle(handle)) return PROM_INVALID_HANDLE;
  scene = prom_ray_scene_find(handle, scene_id);
  if (scene == NULL || scene->magic != PROM_RAY_QUERY_SCENE_MAGIC) return PROM_INVALID_HANDLE;
  if (scene->committed || scene->raw_scene) return PROM_ERROR;
  return prom_ray_scene_append_spheres(scene, spheres, sphere_count) ? PROM_OK : PROM_ERROR;
}

int prom_ray_query_scene_commit_impl(void* handle, uint64_t scene_id) {
  prom_ray_query_scene* scene;
  prom_vk_runtime_services services;
  PrometheusRayQuerySceneCreateRequest request;
  uint64_t committed_id = 0u;
  if (prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) return PROM_INVALID_HANDLE;
  scene = prom_ray_scene_find(handle, scene_id);
  if (scene == NULL || scene->magic != PROM_RAY_QUERY_SCENE_MAGIC || scene->raw_scene || scene->committed) return PROM_ERROR;
  if (services.ray_query_state != PROM_VK_RAY_QUERY_DEVICE_FEATURE_ENABLED) return PROM_ERROR;
  scene->services = services;
  if (scene->staged_triangle_count == 0u && scene->staged_sphere_count == 0u) {
    scene->empty_scene = 1u;
    scene->committed = 1u;
    return PROM_OK;
  }
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.triangles = scene->staged_triangles;
  request.triangle_count = scene->staged_triangle_count;
  request.spheres = scene->staged_spheres;
  request.sphere_count = scene->staged_sphere_count;
  if (prom_ray_query_scene_create_impl(handle, &request, &committed_id) != PROM_OK) return PROM_ERROR;
  scene->committed_scene = prom_ray_scene_take(handle, committed_id);
  if (scene->committed_scene == NULL) return PROM_ERROR;
  free(scene->staged_triangles); scene->staged_triangles = NULL; scene->staged_triangle_capacity = 0u;
  free(scene->staged_spheres); scene->staged_spheres = NULL; scene->staged_sphere_capacity = 0u;
  scene->committed = 1u;
  return PROM_OK;
}

static void prom_ray_public_hit_from_raw(const PrometheusRayQueryRawHit* raw, PrometheusRayQueryHit* out_hit) {
  uint32_t primitive;
  memset(out_hit, 0, sizeof(*out_hit));
  out_hit->instance_id = UINT32_MAX;
  out_hit->primitive_id = UINT32_MAX;
  out_hit->material_id = UINT32_MAX;
  out_hit->distance = -1.0f;
  out_hit->barycentrics[0] = -1.0f;
  out_hit->barycentrics[1] = -1.0f;
  if (raw == NULL || raw->meta[0] == 0u) return;
  memcpy(&primitive, &raw->t_primitive[1], sizeof(primitive));
  out_hit->hit = 1u;
  out_hit->geometry_kind = raw->meta[1];
  out_hit->instance_id = raw->meta[2];
  out_hit->primitive_id = primitive;
  out_hit->distance = raw->t_primitive[0];
  out_hit->barycentrics[0] = raw->barycentrics[0];
  out_hit->barycentrics[1] = raw->barycentrics[1];
  memcpy(out_hit->position, raw->position, sizeof(out_hit->position));
  memcpy(out_hit->normal, raw->normal, sizeof(out_hit->normal));
  memcpy(out_hit->albedo, raw->albedo_material, sizeof(out_hit->albedo));
  if (raw->meta[1] == PROM_RAY_QUERY_GEOMETRY_ANALYTIC_SPHERE) {
    out_hit->material_id = (uint32_t)raw->albedo_material[3];
  }
}

static int prom_ray_public_ray_is_valid(const PrometheusRayQueryRay* ray) {
  float direction_length_squared;
  if (ray == NULL || !isfinite(ray->t_min) || !isfinite(ray->t_max) || ray->t_min < 0.0f ||
      ray->t_max < ray->t_min || !isfinite(ray->origin[0]) || !isfinite(ray->origin[1]) ||
      !isfinite(ray->origin[2]) || !isfinite(ray->direction[0]) || !isfinite(ray->direction[1]) ||
      !isfinite(ray->direction[2])) return 0;
  direction_length_squared = ray->direction[0] * ray->direction[0] +
                             ray->direction[1] * ray->direction[1] +
                             ray->direction[2] * ray->direction[2];
  return isfinite(direction_length_squared) && direction_length_squared > 0.0f;
}

static int prom_ray_batch_byte_sizes(uint32_t ray_count, uint64_t* out_ray_bytes,
                                     uint64_t* out_hit_bytes) {
  return out_ray_bytes != NULL && out_hit_bytes != NULL &&
         prom_ray_mul_u64(ray_count, PROM_RAY_SHADER_BYTES_PER_RAY, out_ray_bytes) &&
         prom_ray_mul_u64(ray_count, sizeof(PrometheusRayQueryRawHit), out_hit_bytes) &&
         *out_ray_bytes <= SIZE_MAX && *out_hit_bytes <= SIZE_MAX &&
         *out_ray_bytes <= UINT64_MAX && *out_hit_bytes <= UINT64_MAX;
}

static int prom_ray_batch_is_admitted(const prom_ray_query_scene* scene, uint32_t ray_count,
                                      uint64_t ray_bytes, uint64_t hit_bytes) {
  VkPhysicalDeviceProperties properties;
  if (scene == NULL || scene->services.physical_device == VK_NULL_HANDLE || ray_count == 0u) return 0;
  memset(&properties, 0, sizeof(properties));
  vkGetPhysicalDeviceProperties(scene->services.physical_device, &properties);
  return ray_count <= properties.limits.maxComputeWorkGroupCount[0] &&
         ray_bytes <= (uint64_t)properties.limits.maxStorageBufferRange &&
         hit_bytes <= (uint64_t)properties.limits.maxStorageBufferRange;
}

static void prom_ray_rebind_batch_descriptors(prom_ray_query_scene* scene) {
  VkDescriptorBufferInfo infos[2];
  VkWriteDescriptorSet writes[2];
  if (scene == NULL || scene->descriptor_set == VK_NULL_HANDLE) return;
  memset(infos, 0, sizeof(infos));
  infos[0].buffer = scene->ray_buffer.buffer;
  infos[0].range = VK_WHOLE_SIZE;
  infos[1].buffer = scene->raw_hit_buffer.buffer;
  infos[1].range = VK_WHOLE_SIZE;
  memset(writes, 0, sizeof(writes));
  for (uint32_t i = 0u; i < 2u; ++i) {
    writes[i].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
    writes[i].dstSet = scene->descriptor_set;
    writes[i].dstBinding = 2u + i;
    writes[i].descriptorCount = 1u;
    writes[i].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    writes[i].pBufferInfo = &infos[i];
  }
  vkUpdateDescriptorSets(scene->services.device, 2u, writes, 0u, NULL);
  ++scene->batch_descriptor_rebind_count;
}

static int prom_ray_ensure_batch_capacity(prom_ray_query_scene* scene, uint32_t required_count,
                                          uint64_t required_ray_bytes, uint64_t required_hit_bytes) {
  uint32_t capacity;
  uint64_t ray_bytes;
  uint64_t hit_bytes;
  prom_vk_buffer new_ray_buffer;
  prom_vk_buffer new_raw_hit_buffer;
  if (scene == NULL || required_count == 0u || required_count <= scene->batch_capacity) return 1;
  capacity = scene->batch_capacity == 0u ? 4u : scene->batch_capacity;
  while (capacity < required_count) {
    if (capacity > UINT32_MAX / 2u) { capacity = required_count; break; }
    capacity *= 2u;
  }
  if (!prom_ray_batch_byte_sizes(capacity, &ray_bytes, &hit_bytes)) return 0;
  /* A doubled capacity is useful only while each storage-buffer descriptor
     remains admitted. Fall back to the exact already-admitted request. */
  if (!prom_ray_batch_is_admitted(scene, capacity, ray_bytes, hit_bytes)) {
    capacity = required_count;
    ray_bytes = required_ray_bytes;
    hit_bytes = required_hit_bytes;
  }
  memset(&new_ray_buffer, 0, sizeof(new_ray_buffer));
  memset(&new_raw_hit_buffer, 0, sizeof(new_raw_hit_buffer));
  if (prom_vk_create_buffer(scene->services.physical_device, scene->services.device,
                            scene->services.test_flags, (VkDeviceSize)ray_bytes,
                            VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                            VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                            1, &new_ray_buffer) != VK_SUCCESS || new_ray_buffer.mapped == NULL ||
      prom_vk_create_buffer(scene->services.physical_device, scene->services.device,
                            scene->services.test_flags, (VkDeviceSize)hit_bytes,
                            VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                            VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                            1, &new_raw_hit_buffer) != VK_SUCCESS || new_raw_hit_buffer.mapped == NULL) {
    prom_vk_destroy_buffer(scene->services.device, &new_raw_hit_buffer);
    prom_vk_destroy_buffer(scene->services.device, &new_ray_buffer);
    return 0;
  }
  /* The synchronous route has completed every prior submission before this
     replacement. Rebinding precedes retirement so the descriptor set never
     observes a retired buffer. */
  {
    prom_vk_buffer old_ray_buffer = scene->ray_buffer;
    prom_vk_buffer old_raw_hit_buffer = scene->raw_hit_buffer;
    scene->ray_buffer = new_ray_buffer;
    scene->raw_hit_buffer = new_raw_hit_buffer;
    prom_ray_rebind_batch_descriptors(scene);
    prom_vk_destroy_buffer(scene->services.device, &old_ray_buffer);
    prom_vk_destroy_buffer(scene->services.device, &old_raw_hit_buffer);
  }
  scene->batch_capacity = capacity;
  ++scene->batch_buffer_reallocation_count;
  return 1;
}

static int prom_ray_scene_trace_batch_direct(prom_ray_query_scene* scene,
                                             const PrometheusRayQueryBatchRequest* request,
                                             uint32_t ray_stride, uint32_t hit_stride,
                                             uint64_t ray_bytes, uint64_t hit_bytes) {
  VkCommandBuffer command_buffer = VK_NULL_HANDLE;
  float* words;
  PrometheusRayQueryRawHit* raw_hits;
  if (scene == NULL || request == NULL || request->ray_count == 0u || !scene->raw_scene ||
      scene->pipeline == VK_NULL_HANDLE || scene->ray_buffer.mapped == NULL ||
      scene->raw_hit_buffer.mapped == NULL) return 0;
  if (!prom_ray_ensure_batch_capacity(scene, request->ray_count, ray_bytes, hit_bytes)) return 0;
  words = (float*)scene->ray_buffer.mapped;
  for (uint32_t i = 0u; i < request->ray_count; ++i) {
    const PrometheusRayQueryRay* ray = (const PrometheusRayQueryRay*)((const unsigned char*)request->rays +
        (size_t)i * ray_stride);
    float* ray_words = words + (size_t)i * PROM_RAY_SHADER_WORDS_PER_RAY * 4u;
    ray_words[0] = ray->origin[0]; ray_words[1] = ray->origin[1]; ray_words[2] = ray->origin[2]; ray_words[3] = ray->t_min;
    ray_words[4] = ray->direction[0]; ray_words[5] = ray->direction[1]; ray_words[6] = ray->direction[2]; ray_words[7] = ray->t_max;
    memcpy(&ray_words[8], &ray->visibility_mask, sizeof(ray->visibility_mask));
    ray_words[9] = 0.0f; ray_words[10] = 0.0f; ray_words[11] = 0.0f;
  }
  memset(scene->raw_hit_buffer.mapped, 0, (size_t)hit_bytes);
  if (!prom_ray_begin_command(scene, &command_buffer)) return 0;
  vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, scene->pipeline);
  vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, scene->pipeline_layout,
                          0u, 1u, &scene->descriptor_set, 0u, NULL);
  vkCmdDispatch(command_buffer, request->ray_count, 1u, 1u);
  ++scene->batch_physical_dispatch_count;
  scene->batch_last_dispatch_groups_x = request->ray_count;
  if (!prom_ray_end_submit_and_free(scene, command_buffer)) return 0;
  ++scene->batch_physical_submission_count;
  raw_hits = (PrometheusRayQueryRawHit*)scene->raw_hit_buffer.mapped;
  for (uint32_t i = 0u; i < request->ray_count; ++i) {
    PrometheusRayQueryHit* hit = (PrometheusRayQueryHit*)((unsigned char*)request->hits +
        (size_t)i * hit_stride);
    memset(hit, 0, hit_stride);
    prom_ray_public_hit_from_raw(&raw_hits[i], hit);
  }
  return 1;
}

int prom_ray_query_scene_submit_batch_impl(void* handle, uint64_t scene_id,
                                           const PrometheusRayQueryBatchRequest* request,
                                           PrometheusRayQueryBatchResult* out_result) {
  prom_ray_query_scene* scene;
  uint32_t ray_stride, hit_stride;
  uint64_t span;
  uint64_t ray_bytes;
  uint64_t hit_bytes;
  if (out_result == NULL || out_result->struct_size < sizeof(*out_result)) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->stage = PROM_STAGE_INIT;
  if (request == NULL || request->struct_size < sizeof(*request)) { out_result->detail_code = PROM_RAY_QUERY_DETAIL_INVALID_REQUEST; return PROM_ERROR; }
  if (!prom_reactor_runtime_validate_handle(handle)) { out_result->detail_code = PROM_RAY_QUERY_DETAIL_INVALID_SCENE; return PROM_INVALID_HANDLE; }
  scene = prom_ray_scene_find(handle, scene_id);
  if (scene == NULL || scene->magic != PROM_RAY_QUERY_SCENE_MAGIC) { out_result->detail_code = PROM_RAY_QUERY_DETAIL_INVALID_SCENE; return PROM_INVALID_HANDLE; }
  if (!scene->committed) { out_result->detail_code = PROM_RAY_QUERY_DETAIL_SCENE_UNCOMMITTED; return PROM_ERROR; }
  out_result->ray_count = request->ray_count;
  ray_stride = request->ray_stride == 0u ? (uint32_t)sizeof(PrometheusRayQueryRay) : request->ray_stride;
  hit_stride = request->hit_stride == 0u ? (uint32_t)sizeof(PrometheusRayQueryHit) : request->hit_stride;
  if (ray_stride < sizeof(PrometheusRayQueryRay) || hit_stride < sizeof(PrometheusRayQueryHit)) { out_result->detail_code = PROM_RAY_QUERY_DETAIL_INVALID_STRIDE; return PROM_ERROR; }
  if (request->ray_count == 0u) { out_result->stage = PROM_STAGE_CLEANUP; return PROM_OK; }
  if (request->rays == NULL) { out_result->detail_code = PROM_RAY_QUERY_DETAIL_NULL_RAYS; return PROM_ERROR; }
  if (request->hits == NULL) { out_result->detail_code = PROM_RAY_QUERY_DETAIL_NULL_HITS; return PROM_ERROR; }
  if (!prom_ray_mul_u64((uint64_t)(request->ray_count - 1u), ray_stride, &span) ||
      !prom_ray_add_u64(span, sizeof(PrometheusRayQueryRay), &span) || span > SIZE_MAX ||
      !prom_ray_mul_u64((uint64_t)(request->ray_count - 1u), hit_stride, &span) ||
       !prom_ray_add_u64(span, sizeof(PrometheusRayQueryHit), &span) || span > SIZE_MAX) { out_result->detail_code = PROM_RAY_QUERY_DETAIL_BATCH_TOO_LARGE; return PROM_ERROR; }
  if (!prom_ray_batch_byte_sizes(request->ray_count, &ray_bytes, &hit_bytes)) {
    out_result->detail_code = PROM_RAY_QUERY_DETAIL_BATCH_TOO_LARGE; return PROM_ERROR;
  }
  for (uint32_t i = 0u; i < request->ray_count; ++i) {
    const PrometheusRayQueryRay* ray = (const PrometheusRayQueryRay*)((const unsigned char*)request->rays + (size_t)i * ray_stride);
    if (!prom_ray_public_ray_is_valid(ray)) {
      out_result->detail_code = PROM_RAY_QUERY_DETAIL_INVALID_RAY; return PROM_ERROR;
    }
  }
  if (scene->empty_scene) {
    for (uint32_t i = 0u; i < request->ray_count; ++i) {
      PrometheusRayQueryHit* hit = (PrometheusRayQueryHit*)((unsigned char*)request->hits + (size_t)i * hit_stride);
      memset(hit, 0, hit_stride);
      prom_ray_public_hit_from_raw(NULL, hit);
    }
    out_result->stage = PROM_STAGE_CLEANUP;
    return PROM_OK;
  }
  if (scene->committed_scene == NULL || !prom_ray_batch_is_admitted(scene->committed_scene, request->ray_count, ray_bytes, hit_bytes)) {
    out_result->detail_code = PROM_RAY_QUERY_DETAIL_BATCH_DISPATCH_LIMIT; return PROM_ERROR;
  }
  if (!prom_ray_scene_trace_batch_direct(scene->committed_scene, request, ray_stride, hit_stride, ray_bytes, hit_bytes)) {
    out_result->detail_code = PROM_RAY_QUERY_DETAIL_SUBMIT_OR_READBACK_FAILED; return PROM_ERROR;
  }
  out_result->stage = PROM_STAGE_CLEANUP;
  return PROM_OK;
}

int prom_ray_query_scene_batch_diagnostics_impl(void* handle, uint64_t scene_id,
                                                prom_ray_query_batch_diagnostics* out_diagnostics) {
  prom_ray_query_scene* scene;
  prom_ray_query_scene* execution_scene;
  if (out_diagnostics == NULL || !prom_reactor_runtime_validate_handle(handle)) return PROM_INVALID_HANDLE;
  memset(out_diagnostics, 0, sizeof(*out_diagnostics));
  scene = prom_ray_scene_find(handle, scene_id);
  if (scene == NULL || !scene->committed || scene->empty_scene) return PROM_INVALID_HANDLE;
  execution_scene = scene->committed_scene;
  if (execution_scene == NULL || !execution_scene->raw_scene) return PROM_INVALID_HANDLE;
  out_diagnostics->retained_capacity = execution_scene->batch_capacity;
  out_diagnostics->last_dispatch_groups_x = execution_scene->batch_last_dispatch_groups_x;
  out_diagnostics->buffer_reallocation_count = execution_scene->batch_buffer_reallocation_count;
  out_diagnostics->descriptor_rebind_count = execution_scene->batch_descriptor_rebind_count;
  out_diagnostics->physical_dispatch_count = execution_scene->batch_physical_dispatch_count;
  out_diagnostics->physical_submission_count = execution_scene->batch_physical_submission_count;
  return PROM_OK;
}

int prom_ray_query_triangle_scene_destroy_impl(void* handle, uint64_t scene_id) {
  prom_ray_query_scene* scene;
  if (!prom_reactor_runtime_validate_handle(handle)) return PROM_INVALID_HANDLE;
  scene = prom_ray_scene_take(handle, scene_id);
  if (scene == NULL) return PROM_INVALID_HANDLE;
  prom_ray_scene_destroy(scene);
  return PROM_OK;
}

int prom_ray_query_scene_destroy_impl(void* handle, uint64_t scene_id) {
  return prom_ray_query_triangle_scene_destroy_impl(handle, scene_id);
}

void prom_ray_query_scene_runtime_destroy_all(void* runtime_handle) {
  prom_ray_query_scene* retired[PROM_RAY_QUERY_MAX_LIVE_SCENES];
  uint32_t retired_count = 0u;
  uint32_t index;
  if (runtime_handle == NULL) return;
  memset(retired, 0, sizeof(retired));
  prom_ray_scene_lock();
  for (index = 0u; index < PROM_RAY_QUERY_MAX_LIVE_SCENES; ++index) {
    if (g_ray_query_scenes[index] != NULL && g_ray_query_scenes[index]->runtime_handle == runtime_handle) {
      retired[retired_count++] = g_ray_query_scenes[index];
      g_ray_query_scenes[index] = NULL;
    }
  }
  prom_ray_scene_unlock();
  for (index = 0u; index < retired_count; ++index) prom_ray_scene_destroy(retired[index]);
}
