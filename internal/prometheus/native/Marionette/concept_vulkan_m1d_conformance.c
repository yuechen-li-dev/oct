#include "../reactor_api.h"
#include "../reactor_vulkan.h"
#include "../reactor_concept_vulkan_kernel54.generated.h"
#include "../reactor_concept_vulkan_m1d_trace.h"

#include <stdint.h>
#include <stdio.h>
#include <string.h>

enum { PROM_CONCEPT_VULKAN_M1D_TRACE_CAPACITY = 64u };

typedef struct prom_concept_vulkan_m1d_trace {
  uint32_t path;
  uint32_t events[PROM_CONCEPT_VULKAN_M1D_TRACE_CAPACITY];
  uint32_t count;
  int result;
} prom_concept_vulkan_m1d_trace;

static prom_concept_vulkan_m1d_trace g_traces[3];
static prom_concept_vulkan_m1d_trace* g_active_trace;

void prom_concept_vulkan_m1d_trace_begin(uint32_t path) {
  if (path > PROM_CONCEPT_VULKAN_M1D_GENERATED) return;
  memset(&g_traces[path], 0, sizeof(g_traces[path]));
  g_traces[path].path = path;
  g_active_trace = &g_traces[path];
}

void prom_concept_vulkan_m1d_trace_event(uint32_t event) {
  if (g_active_trace == NULL || g_active_trace->count >= PROM_CONCEPT_VULKAN_M1D_TRACE_CAPACITY) return;
  g_active_trace->events[g_active_trace->count++] = event;
}

void prom_concept_vulkan_m1d_trace_result(int result) {
  if (g_active_trace != NULL) g_active_trace->result = result;
}

static int prom_concept_vulkan_m1d_equal_probe(const PrometheusRayQueryProbeResult* left,
                                                const PrometheusRayQueryProbeResult* right) {
  return memcmp(left, right, sizeof(*left)) == 0;
}

static int prom_concept_vulkan_m1d_contains(const prom_concept_vulkan_m1d_trace* trace, uint32_t event) {
  uint32_t i;
  for (i = 0u; i < trace->count; ++i) if (trace->events[i] == event) return 1;
  return 0;
}

static int prom_concept_vulkan_m1d_verify_core_trace(const prom_concept_vulkan_m1d_trace* trace) {
  static const uint32_t core[] = {
      PROM_CONCEPT_VULKAN_M1D_EVIDENCE, PROM_CONCEPT_VULKAN_M1D_COMMAND_ALLOCATE,
      PROM_CONCEPT_VULKAN_M1D_COMMAND_BEGIN, PROM_CONCEPT_VULKAN_M1D_TLAS_READ,
      PROM_CONCEPT_VULKAN_M1D_EVIDENCE_WRITE, PROM_CONCEPT_VULKAN_M1D_DISPATCH,
      PROM_CONCEPT_VULKAN_M1D_COMMAND_END, PROM_CONCEPT_VULKAN_M1D_SUBMIT_WAIT,
      PROM_CONCEPT_VULKAN_M1D_OBSERVE, PROM_CONCEPT_VULKAN_M1D_CLEANUP};
  uint32_t expected = 0u, i;
  for (i = 0u; i < trace->count && expected < (uint32_t)(sizeof(core) / sizeof(core[0])); ++i) {
    if (trace->events[i] == core[expected]) ++expected;
  }
  return expected == (uint32_t)(sizeof(core) / sizeof(core[0]));
}

static int prom_concept_vulkan_m1d_create_scene(void* runtime, uint64_t* out_scene) {
  const PrometheusRayQueryTriangle triangles[] = {{{-1.0f, -1.0f, 2.0f,
                                                     1.0f, -1.0f, 2.0f,
                                                     0.0f, 1.0f, 2.0f}}};
  PrometheusRayQueryTriangleSceneCreateRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = (uint32_t)sizeof(request);
  request.triangles = triangles;
  request.triangle_count = 1u;
  return prometheus_reactor_runtime_ray_query_triangle_scene_create(runtime, &request, out_scene);
}

int main(int argc, char** argv) {
  PrometheusReactorConfig config;
  prom_vk_runtime_services services;
  prom_vk_runtime_services after_handwritten;
  prom_vk_runtime_services after_generated;
  PrometheusRayQueryProbeResult handwritten;
  PrometheusRayQueryProbeResult generated;
  uint64_t handwritten_scene = 0u, generated_scene = 0u;
  void* runtime = NULL;
  int handwritten_result, generated_result;
  uint32_t iteration;
  if (argc != 2) {
    fprintf(stderr, "usage: concept_vulkan_m1d_conformance <shader-package-root>\n");
    return 2;
  }
  memset(&config, 0, sizeof(config));
  config.struct_size = (uint32_t)sizeof(config);
  config.shader_package_root = argv[1];
  if (prometheus_reactor_runtime_create(&config, &runtime) != PROM_OK ||
      prom_reactor_runtime_get_vk_services(runtime, &services) != PROM_OK) return 3;
  if (services.ray_query_state != PROM_VK_RAY_QUERY_DEVICE_FEATURE_ENABLED) {
    prometheus_reactor_runtime_destroy(runtime);
    puts("SKIP: ray-query feature bundle is unavailable");
    return 77;
  }
  if (prom_concept_vulkan_kernel54_handwritten_adapter(NULL, 0u, &handwritten) != PROM_INVALID_HANDLE ||
      prom_concept_vulkan_kernel54_generated_adapter(NULL, 0u, &generated) != PROM_INVALID_HANDLE ||
      prom_concept_vulkan_kernel54_handwritten_adapter(runtime, 0u, NULL) != PROM_ERROR ||
      prom_concept_vulkan_kernel54_generated_adapter(runtime, 0u, NULL) != PROM_ERROR) return 4;
  for (iteration = 0u; iteration < 2u; ++iteration) {
    if (prom_concept_vulkan_m1d_create_scene(runtime, &handwritten_scene) != PROM_OK) return 5;
    memset(&handwritten, 0, sizeof(handwritten));
    handwritten_result = prom_concept_vulkan_kernel54_handwritten_adapter(runtime, handwritten_scene, &handwritten);
    if (handwritten_result != PROM_OK || handwritten.hit != 1u ||
        prometheus_reactor_runtime_ray_query_triangle_scene_destroy(runtime, handwritten_scene) != PROM_OK) return 6;
    if (prom_reactor_runtime_get_vk_services(runtime, &after_handwritten) != PROM_OK ||
        after_handwritten.validation_error_count != services.validation_error_count ||
        after_handwritten.ray_query_state != services.ray_query_state) return 11;
    if (prom_concept_vulkan_m1d_create_scene(runtime, &generated_scene) != PROM_OK) return 7;
    memset(&generated, 0, sizeof(generated));
    generated_result = prom_concept_vulkan_kernel54_generated_adapter(runtime, generated_scene, &generated);
    if (generated_result != PROM_OK || generated.hit != 1u ||
        prometheus_reactor_runtime_ray_query_triangle_scene_destroy(runtime, generated_scene) != PROM_OK) return 8;
    if (prom_reactor_runtime_get_vk_services(runtime, &after_generated) != PROM_OK ||
        after_generated.validation_error_count != services.validation_error_count ||
        after_generated.ray_query_state != services.ray_query_state) return 12;
    if (!prom_concept_vulkan_m1d_equal_probe(&handwritten, &generated) ||
        !prom_concept_vulkan_m1d_verify_core_trace(&g_traces[PROM_CONCEPT_VULKAN_M1D_HANDWRITTEN]) ||
        !prom_concept_vulkan_m1d_verify_core_trace(&g_traces[PROM_CONCEPT_VULKAN_M1D_GENERATED]) ||
        !prom_concept_vulkan_m1d_contains(&g_traces[PROM_CONCEPT_VULKAN_M1D_HANDWRITTEN], PROM_CONCEPT_VULKAN_M1D_PERSISTENT_RESOURCES) ||
        !prom_concept_vulkan_m1d_contains(&g_traces[PROM_CONCEPT_VULKAN_M1D_GENERATED], PROM_CONCEPT_VULKAN_M1D_DESCRIPTORS) ||
        !prom_concept_vulkan_m1d_contains(&g_traces[PROM_CONCEPT_VULKAN_M1D_GENERATED], PROM_CONCEPT_VULKAN_M1D_PIPELINE)) return 9;
    printf("PASS iteration=%u handwritten-events=%u generated-events=%u evidence=%u\n",
           iteration + 1u, g_traces[PROM_CONCEPT_VULKAN_M1D_HANDWRITTEN].count,
           g_traces[PROM_CONCEPT_VULKAN_M1D_GENERATED].count, generated.hit);
  }
  if (prometheus_reactor_runtime_destroy(runtime) != PROM_OK) return 10;
  return 0;
}
