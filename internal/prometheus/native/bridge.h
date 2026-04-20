#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_BRIDGE_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_BRIDGE_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

enum {
  PROM_OK = 0,
  PROM_ERROR = 1,
  PROM_INVALID_HANDLE = 2,
  PROM_INTERNAL_ERROR = 3,
};

enum {
  PROM_BACKEND_UNKNOWN = 0,
  PROM_BACKEND_STUB = 1,
};

enum {
  PROM_REASON_NONE = 0,
  PROM_REASON_STUB_UNAVAILABLE = 1,
};

typedef struct PrometheusCaps {
  uint32_t available;
  uint32_t backend_type;
  uint32_t reason_code;
} PrometheusCaps;

uint32_t prometheus_reactor_abi_version(void);
int prometheus_reactor_runtime_create(void* config, void** out_handle);
int prometheus_reactor_runtime_destroy(void* handle);
int prometheus_reactor_runtime_probe(void* handle, PrometheusCaps* out_caps);

/* Backward-compat aliases for earlier contract drafts. */
int prometheus_runtime_create(void* config, void** out_handle);
int prometheus_runtime_destroy(void* handle);
int prometheus_runtime_probe(void* handle, PrometheusCaps* out_caps);

#ifdef __cplusplus
}
#endif

#endif
