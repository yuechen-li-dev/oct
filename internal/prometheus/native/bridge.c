#include "bridge.h"

#include <stddef.h>
#include <stdlib.h>

#define PROMETHEUS_REACTOR_ABI_V1 1u
#define PROMETHEUS_RUNTIME_MAGIC 0x50524f4du
#define PROMETHEUS_MAX_TRACKED_HANDLES 256

typedef struct prometheus_runtime {
  uint32_t magic;
} prometheus_runtime;

static void* g_active_handles[PROMETHEUS_MAX_TRACKED_HANDLES];

static int registry_contains(void* handle) {
  size_t i;
  for (i = 0; i < PROMETHEUS_MAX_TRACKED_HANDLES; ++i) {
    if (g_active_handles[i] == handle) {
      return 1;
    }
  }
  return 0;
}

static int registry_add(void* handle) {
  size_t i;
  for (i = 0; i < PROMETHEUS_MAX_TRACKED_HANDLES; ++i) {
    if (g_active_handles[i] == NULL) {
      g_active_handles[i] = handle;
      return 1;
    }
  }
  return 0;
}

static void registry_remove(void* handle) {
  size_t i;
  for (i = 0; i < PROMETHEUS_MAX_TRACKED_HANDLES; ++i) {
    if (g_active_handles[i] == handle) {
      g_active_handles[i] = NULL;
      return;
    }
  }
}

uint32_t prometheus_reactor_abi_version(void) {
  return PROMETHEUS_REACTOR_ABI_V1;
}

int prometheus_reactor_runtime_create(void* config, void** out_handle) {
  (void)config;

  if (out_handle == NULL) {
    return PROM_ERROR;
  }

  *out_handle = NULL;
  prometheus_runtime* runtime = (prometheus_runtime*)malloc(sizeof(prometheus_runtime));
  if (runtime == NULL) {
    return PROM_INTERNAL_ERROR;
  }

  runtime->magic = PROMETHEUS_RUNTIME_MAGIC;
  if (!registry_add(runtime)) {
    free(runtime);
    return PROM_INTERNAL_ERROR;
  }

  *out_handle = runtime;
  return PROM_OK;
}

int prometheus_reactor_runtime_destroy(void* handle) {
  if (handle == NULL) {
    return PROM_OK;
  }

  if (!registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }

  registry_remove(handle);
  free(handle);
  return PROM_OK;
}

int prometheus_reactor_runtime_probe(void* handle, PrometheusCaps* out_caps) {
  if (out_caps == NULL) {
    return PROM_ERROR;
  }
  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }

  out_caps->available = 0u;
  out_caps->backend_type = PROM_BACKEND_STUB;
  out_caps->reason_code = PROM_REASON_STUB_UNAVAILABLE;
  return PROM_OK;
}

int prometheus_runtime_create(void* config, void** out_handle) {
  return prometheus_reactor_runtime_create(config, out_handle);
}

int prometheus_runtime_destroy(void* handle) {
  return prometheus_reactor_runtime_destroy(handle);
}

int prometheus_runtime_probe(void* handle, PrometheusCaps* out_caps) {
  return prometheus_reactor_runtime_probe(handle, out_caps);
}
