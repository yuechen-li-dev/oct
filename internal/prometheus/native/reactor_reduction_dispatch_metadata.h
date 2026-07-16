#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_REDUCTION_DISPATCH_METADATA_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_REDUCTION_DISPATCH_METADATA_H

#include <stdint.h>

typedef struct prom_reduction_kernel_dispatch_metadata {
  uint32_t threads_x;
  uint32_t threads_y;
  uint32_t threads_z;
  uint32_t elements_per_invocation;
  uint32_t elements_per_partial;
  uint32_t descriptor_binding_count;
  uint32_t push_constant_bytes;
  uint32_t stage_role;
  uint32_t minimum_row_width;
  uint32_t maximum_row_width;
} prom_reduction_kernel_dispatch_metadata;

#endif
