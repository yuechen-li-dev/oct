#ifndef PROMETHEUS_RAY_QUERY_H
#define PROMETHEUS_RAY_QUERY_H

/* Public RQ-M1 host contract. This header intentionally contains no Vulkan,
   SPIR-V, descriptor, or private-reactor declarations. */

#include <stdint.h>

#if defined(_WIN32)
#if defined(PROMETHEUS_REACTOR_BUILD_DLL)
#define PROM_RAY_QUERY_API __declspec(dllexport)
#elif defined(PROMETHEUS_REACTOR_USE_DLL)
#define PROM_RAY_QUERY_API __declspec(dllimport)
#else
#define PROM_RAY_QUERY_API
#endif
#else
#define PROM_RAY_QUERY_API
#endif

#ifdef __cplusplus
extern "C" {
#endif

enum { PROM_RAY_QUERY_OK = 0, PROM_RAY_QUERY_ERROR = 1, PROM_RAY_QUERY_INVALID_HANDLE = 2 };
enum {
  PROM_RAY_QUERY_GEOMETRY_NONE = 0u,
  PROM_RAY_QUERY_GEOMETRY_TRIANGLE = 1u,
  PROM_RAY_QUERY_GEOMETRY_ANALYTIC_SPHERE = 2u,
  PROM_RAY_QUERY_VISIBILITY_MASK_ALL = 0xffu,
};

typedef struct PrometheusRayQueryTriangle { float positions[9]; } PrometheusRayQueryTriangle;
typedef struct PrometheusRayQuerySphere {
  float center[3]; float radius; float albedo[3]; uint32_t material_id;
} PrometheusRayQuerySphere;
typedef struct PrometheusRayQueryRay {
  float origin[3]; float t_min; float direction[3]; float t_max;
  uint32_t visibility_mask; uint32_t reserved[3];
} PrometheusRayQueryRay;
typedef struct PrometheusRayQueryHit {
  uint32_t hit; uint32_t geometry_kind; uint32_t instance_id; uint32_t primitive_id;
  float distance; float barycentrics[2]; float reserved0;
  float position[3]; float reserved1; float normal[3]; float reserved2;
  float albedo[3]; uint32_t material_id;
} PrometheusRayQueryHit;
typedef struct PrometheusRayQueryBatchRequest {
  uint32_t struct_size; const void* rays; uint32_t ray_count; uint32_t ray_stride;
  void* hits; uint32_t hit_stride;
} PrometheusRayQueryBatchRequest;
typedef struct PrometheusRayQueryBatchResult {
  uint32_t struct_size; uint32_t stage; int32_t detail_code; uint32_t ray_count;
  uint64_t upload_wall_ns; uint64_t execution_wall_ns; uint64_t readback_wall_ns;
} PrometheusRayQueryBatchResult;
typedef struct PrometheusRayQueryRuntimeConfig {
  uint32_t struct_size; const char* shader_package_root;
} PrometheusRayQueryRuntimeConfig;

PROM_RAY_QUERY_API int prometheus_ray_query_runtime_create(const PrometheusRayQueryRuntimeConfig* config, void** out_handle);
PROM_RAY_QUERY_API int prometheus_reactor_runtime_destroy(void* handle);
PROM_RAY_QUERY_API int prometheus_reactor_runtime_ray_query_scene_create_empty(void* handle, uint64_t* out_scene_id);
PROM_RAY_QUERY_API int prometheus_reactor_runtime_ray_query_scene_add_triangles(void* handle, uint64_t scene_id, const PrometheusRayQueryTriangle* triangles, uint32_t triangle_count);
PROM_RAY_QUERY_API int prometheus_reactor_runtime_ray_query_scene_add_spheres(void* handle, uint64_t scene_id, const PrometheusRayQuerySphere* spheres, uint32_t sphere_count);
PROM_RAY_QUERY_API int prometheus_reactor_runtime_ray_query_scene_commit(void* handle, uint64_t scene_id);
PROM_RAY_QUERY_API int prometheus_reactor_runtime_ray_query_scene_submit_batch(void* handle, uint64_t scene_id, const PrometheusRayQueryBatchRequest* request, PrometheusRayQueryBatchResult* out_result);
PROM_RAY_QUERY_API int prometheus_reactor_runtime_ray_query_scene_destroy(void* handle, uint64_t scene_id);

#ifdef __cplusplus
}
#endif

#endif
