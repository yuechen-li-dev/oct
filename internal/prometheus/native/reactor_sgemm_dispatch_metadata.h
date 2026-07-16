#ifndef OCT_INTERNAL_PROMETHEUS_REACTOR_SGEMM_DISPATCH_METADATA_H
#define OCT_INTERNAL_PROMETHEUS_REACTOR_SGEMM_DISPATCH_METADATA_H

#include <stdint.h>

typedef struct prom_sgemm_kernel_dispatch_metadata {
  uint32_t threads_x;
  uint32_t threads_y;
  uint32_t threads_z;
  uint32_t outputs_per_invocation_m;
  uint32_t outputs_per_invocation_n;
  uint32_t tile_m;
  uint32_t tile_n;
  uint32_t tile_k;
  uint32_t unroll_k;
  /* Optional whole-workgroup output footprint. Cooperative subgroup kernels
     use this because an invocation-level footprint is not meaningful. */
  uint32_t workgroup_output_m;
  uint32_t workgroup_output_n;
} prom_sgemm_kernel_dispatch_metadata;

typedef struct prom_sgemm_dispatch_geometry {
  uint32_t groups_x;
  uint32_t groups_y;
  uint32_t groups_z;
  uint32_t logical_m_per_group;
  uint32_t logical_n_per_group;
} prom_sgemm_dispatch_geometry;

static inline uint32_t prom_sgemm_ceil_div_u32(uint32_t value, uint32_t divisor) {
  return divisor == 0u ? 0u : (value + (divisor - 1u)) / divisor;
}

static inline prom_sgemm_dispatch_geometry prom_sgemm_dispatch_geometry_for_metadata(
    uint32_t m,
    uint32_t n,
    const prom_sgemm_kernel_dispatch_metadata* metadata) {
  prom_sgemm_dispatch_geometry geometry;
  uint32_t logical_m_per_group;
  uint32_t logical_n_per_group;
  if (metadata == 0) {
    geometry.groups_x = 0u;
    geometry.groups_y = 0u;
    geometry.groups_z = 0u;
    geometry.logical_m_per_group = 0u;
    geometry.logical_n_per_group = 0u;
    return geometry;
  }
  logical_m_per_group = metadata->workgroup_output_m != 0u
                            ? metadata->workgroup_output_m
                            : metadata->threads_x * metadata->outputs_per_invocation_m;
  logical_n_per_group = metadata->workgroup_output_n != 0u
                            ? metadata->workgroup_output_n
                            : metadata->threads_y * metadata->outputs_per_invocation_n;
  geometry.groups_x = prom_sgemm_ceil_div_u32(m, logical_m_per_group);
  geometry.groups_y = prom_sgemm_ceil_div_u32(n, logical_n_per_group);
  geometry.groups_z = 1u;
  geometry.logical_m_per_group = logical_m_per_group;
  geometry.logical_n_per_group = logical_n_per_group;
  return geometry;
}

#endif  // OCT_INTERNAL_PROMETHEUS_REACTOR_SGEMM_DISPATCH_METADATA_H
