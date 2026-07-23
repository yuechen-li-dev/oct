#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_SHADER_PACKAGE_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_SHADER_PACKAGE_H

#include <stdint.h>
#include <vulkan/vulkan.h>

#ifdef __cplusplus
extern "C" {
#endif

/* The package is immutable after open and is owned by one reactor runtime. */
typedef struct prom_shader_package prom_shader_package;

typedef enum prom_shader_package_error {
  PROM_SHADER_PACKAGE_OK = 0,
  PROM_SHADER_PACKAGE_ROOT_UNAVAILABLE,
  PROM_SHADER_PACKAGE_MANIFEST_UNAVAILABLE,
  PROM_SHADER_PACKAGE_MANIFEST_MALFORMED,
  PROM_SHADER_PACKAGE_IDENTITY_MISMATCH,
  PROM_SHADER_PACKAGE_TABLE_INVALID,
  PROM_SHADER_PACKAGE_VARIANT_UNAVAILABLE,
  PROM_SHADER_PACKAGE_ARTIFACT_UNAVAILABLE,
  PROM_SHADER_PACKAGE_ARTIFACT_SIZE_MISMATCH,
  PROM_SHADER_PACKAGE_ARTIFACT_DIGEST_MISMATCH,
  PROM_SHADER_PACKAGE_SPIRV_INVALID,
  PROM_SHADER_PACKAGE_VULKAN_FAILURE,
} prom_shader_package_error;

typedef struct prom_shader_package_diagnostic {
  prom_shader_package_error code;
  char message[192];
} prom_shader_package_diagnostic;

/* Opens and fully validates the declarative prometheus.core v1 manifest. */
int prom_shader_package_open(const char* root, prom_shader_package** out_package,
                             prom_shader_package_diagnostic* out_diagnostic);
void prom_shader_package_destroy(prom_shader_package* package);

/* Creates one exact selected module. The temporary object contents are released
   before this function returns. out_entry_point is package-owned and remains
   valid until prom_shader_package_destroy. */
int prom_shader_package_create_module(prom_shader_package* package, VkDevice device,
                                      const char* variant_id, VkShaderModule* out_module,
                                      const char** out_entry_point,
                                      prom_shader_package_diagnostic* out_diagnostic);

uint64_t prom_shader_package_artifact_open_count(const prom_shader_package* package);
const char* prom_shader_package_root(const prom_shader_package* package);

#ifdef __cplusplus
}
#endif

#endif
