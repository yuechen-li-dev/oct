/* A bounded foreign-style client for Prometheus RQ-M1. It deliberately knows
   only the public semantic header; no Vulkan, SPIR-V, or private reactor API
   is included here. Usage: camera_diagnostic <shader-package-root> <output-dir> */

#include "../../internal/prometheus/native/include/prometheus_ray_query.h"

#include <math.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

enum { k_width = 64, k_height = 64, k_pixel_count = k_width * k_height };

static unsigned char clamp_byte(float value) {
  if (value <= 0.0f) return 0u;
  if (value >= 1.0f) return 255u;
  return (unsigned char)(value * 255.0f + 0.5f);
}

static void color_for_mode(uint32_t mode, const PrometheusRayQueryHit* hit, unsigned char* pixel) {
  static const unsigned char kind_triangle[3] = { 64u, 160u, 255u };
  static const unsigned char kind_sphere[3] = { 255u, 144u, 64u };
  uint32_t identity;
  if (hit->hit == 0u) { pixel[0] = 0u; pixel[1] = 0u; pixel[2] = 0u; return; }
  switch (mode) {
    case 0u: pixel[0] = 255u; pixel[1] = 255u; pixel[2] = 255u; break;
    case 1u: pixel[0] = pixel[1] = pixel[2] = clamp_byte(hit->distance / 16.0f); break;
    case 2u:
      identity = hit->primitive_id * 0x9e3779b9u + hit->instance_id * 0x85ebca6bu;
      pixel[0] = (unsigned char)(identity >> 0u); pixel[1] = (unsigned char)(identity >> 8u); pixel[2] = (unsigned char)(identity >> 16u);
      break;
    case 3u:
      if (hit->geometry_kind == PROM_RAY_QUERY_GEOMETRY_TRIANGLE) memcpy(pixel, kind_triangle, 3u);
      else memcpy(pixel, kind_sphere, 3u);
      break;
    case 4u:
      pixel[0] = clamp_byte(hit->normal[0] * 0.5f + 0.5f);
      pixel[1] = clamp_byte(hit->normal[1] * 0.5f + 0.5f);
      pixel[2] = clamp_byte(hit->normal[2] * 0.5f + 0.5f);
      break;
    default:
      pixel[0] = clamp_byte(hit->barycentrics[0]);
      pixel[1] = clamp_byte(hit->barycentrics[1]);
      pixel[2] = clamp_byte(1.0f - hit->barycentrics[0] - hit->barycentrics[1]);
      break;
  }
}

static int write_mode(const char* directory, const char* name, uint32_t mode, const PrometheusRayQueryHit* hits) {
  char path[1024];
  unsigned char pixels[k_pixel_count * 3u];
  FILE* file;
  if (snprintf(path, sizeof(path), "%s/%s.ppm", directory, name) < 0) return 0;
  for (uint32_t index = 0u; index < k_pixel_count; ++index) color_for_mode(mode, &hits[index], &pixels[index * 3u]);
  file = fopen(path, "wb");
  if (file == NULL) return 0;
  if (fprintf(file, "P6\n%d %d\n255\n", k_width, k_height) < 0 || fwrite(pixels, 1u, sizeof(pixels), file) != sizeof(pixels)) {
    fclose(file); return 0;
  }
  return fclose(file) == 0;
}

int main(int argc, char** argv) {
  static const char* const names[] = { "hit", "distance", "identity", "geometry", "normal", "barycentrics" };
  PrometheusRayQueryRuntimeConfig config;
  PrometheusRayQueryTriangle triangle = {{-3.0f, -2.0f, 6.0f, 3.0f, -2.0f, 6.0f, 0.0f, 3.0f, 6.0f}};
  PrometheusRayQuerySphere sphere = {{0.8f, 0.0f, 3.5f}, 1.0f, {0.9f, 0.35f, 0.15f}, 7u};
  PrometheusRayQueryRay* rays;
  PrometheusRayQueryHit* hits;
  PrometheusRayQueryBatchRequest batch;
  PrometheusRayQueryBatchResult result;
  void* runtime = NULL;
  uint64_t scene = 0u;
  int exit_code = 1;
  if (argc != 3) { fprintf(stderr, "usage: %s <shader-package-root> <existing-output-dir>\n", argv[0]); return 2; }
  rays = (PrometheusRayQueryRay*)calloc(k_pixel_count, sizeof(*rays));
  hits = (PrometheusRayQueryHit*)calloc(k_pixel_count, sizeof(*hits));
  if (rays == NULL || hits == NULL) goto done;
  for (uint32_t y = 0u; y < k_height; ++y) for (uint32_t x = 0u; x < k_width; ++x) {
    const uint32_t index = y * k_width + x;
    const float sx = ((float)x + 0.5f) * (2.0f / (float)k_width) - 1.0f;
    const float sy = 1.0f - ((float)y + 0.5f) * (2.0f / (float)k_height);
    const float length = sqrtf(sx * sx + sy * sy + 1.0f);
    rays[index].origin[2] = -4.0f;
    rays[index].direction[0] = sx / length; rays[index].direction[1] = sy / length; rays[index].direction[2] = 1.0f / length;
    rays[index].t_max = 100.0f;
    rays[index].visibility_mask = PROM_RAY_QUERY_VISIBILITY_MASK_ALL;
  }
  memset(&config, 0, sizeof(config)); config.struct_size = sizeof(config); config.shader_package_root = argv[1];
  if (prometheus_ray_query_runtime_create(&config, &runtime) != PROM_RAY_QUERY_OK ||
      prometheus_reactor_runtime_ray_query_scene_create_empty(runtime, &scene) != PROM_RAY_QUERY_OK ||
      prometheus_reactor_runtime_ray_query_scene_add_triangles(runtime, scene, &triangle, 1u) != PROM_RAY_QUERY_OK ||
      prometheus_reactor_runtime_ray_query_scene_add_spheres(runtime, scene, &sphere, 1u) != PROM_RAY_QUERY_OK ||
      prometheus_reactor_runtime_ray_query_scene_commit(runtime, scene) != PROM_RAY_QUERY_OK) {
    fprintf(stderr, "Prometheus ray-query scene setup failed (the admitted capability may be unavailable).\n"); goto done;
  }
  memset(&batch, 0, sizeof(batch)); batch.struct_size = sizeof(batch); batch.rays = rays; batch.ray_count = k_pixel_count; batch.hits = hits;
  memset(&result, 0, sizeof(result)); result.struct_size = sizeof(result);
  if (prometheus_reactor_runtime_ray_query_scene_submit_batch(runtime, scene, &batch, &result) != PROM_RAY_QUERY_OK) {
    fprintf(stderr, "Prometheus ray-query batch failed: detail=%d stage=%u\n", result.detail_code, result.stage); goto done;
  }
  for (uint32_t mode = 0u; mode < 6u; ++mode) if (!write_mode(argv[2], names[mode], mode, hits)) { fprintf(stderr, "failed to write %s diagnostic\n", names[mode]); goto done; }
  exit_code = 0;
done:
  if (scene != 0u) prometheus_reactor_runtime_ray_query_scene_destroy(runtime, scene);
  if (runtime != NULL) prometheus_reactor_runtime_destroy(runtime);
  free(hits); free(rays);
  return exit_code;
}
