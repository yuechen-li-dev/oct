#ifndef PROM_CONCEPT_VULKAN_M1D_TRACE_H
#define PROM_CONCEPT_VULKAN_M1D_TRACE_H

/* Private, link-time conformance vocabulary.  Production does not define the
 * conformance macro, include this header, or provide these symbols. */
#ifdef PROM_CONCEPT_VULKAN_CONFORMANCE
#include <stdint.h>

enum {
  PROM_CONCEPT_VULKAN_M1D_HANDWRITTEN = 1u,
  PROM_CONCEPT_VULKAN_M1D_GENERATED = 2u
};

enum {
  PROM_CONCEPT_VULKAN_M1D_PACKAGE = 1u,
  PROM_CONCEPT_VULKAN_M1D_PERSISTENT_RESOURCES = 2u,
  PROM_CONCEPT_VULKAN_M1D_EVIDENCE = 3u,
  PROM_CONCEPT_VULKAN_M1D_DESCRIPTORS = 4u,
  PROM_CONCEPT_VULKAN_M1D_PIPELINE = 5u,
  PROM_CONCEPT_VULKAN_M1D_COMMAND_ALLOCATE = 6u,
  PROM_CONCEPT_VULKAN_M1D_COMMAND_BEGIN = 7u,
  PROM_CONCEPT_VULKAN_M1D_TLAS_READ = 8u,
  PROM_CONCEPT_VULKAN_M1D_EVIDENCE_WRITE = 9u,
  PROM_CONCEPT_VULKAN_M1D_DISPATCH = 10u,
  PROM_CONCEPT_VULKAN_M1D_COMMAND_END = 11u,
  PROM_CONCEPT_VULKAN_M1D_SUBMIT_WAIT = 12u,
  PROM_CONCEPT_VULKAN_M1D_OBSERVE = 13u,
  PROM_CONCEPT_VULKAN_M1D_CLEANUP = 14u
};

void prom_concept_vulkan_m1d_trace_begin(uint32_t path);
void prom_concept_vulkan_m1d_trace_event(uint32_t event);
void prom_concept_vulkan_m1d_trace_result(int result);
#endif

#endif
