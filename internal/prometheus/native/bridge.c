#include "bridge.h"

#include <stdlib.h>
#include <string.h>

struct prometheus_runtime {
  int placeholder;
};

int prometheus_runtime_create(prometheus_runtime** out_runtime) {
  if (out_runtime == NULL) {
    return PROMETHEUS_NATIVE_ERR_INVALID_ARGUMENT;
  }
  const char* unavailable = getenv("PROMETHEUS_FORCE_UNAVAILABLE");
  if (unavailable != NULL && unavailable[0] == '1') {
    *out_runtime = NULL;
    return PROMETHEUS_NATIVE_ERR_UNAVAILABLE;
  }
  const char* force_submit_failure = getenv("PROMETHEUS_FORCE_SUBMIT_FAILURE");
  const char* stub_ready = getenv("PROMETHEUS_STUB_RUNTIME_READY");
  if ((stub_ready == NULL || stub_ready[0] != '1') &&
      (force_submit_failure == NULL || force_submit_failure[0] != '1')) {
    *out_runtime = NULL;
    return PROMETHEUS_NATIVE_ERR_UNAVAILABLE;
  }
  *out_runtime = (prometheus_runtime*)malloc(sizeof(prometheus_runtime));
  if (*out_runtime == NULL) {
    return PROMETHEUS_NATIVE_ERR_INTERNAL;
  }
  (*out_runtime)->placeholder = 1;
  return PROMETHEUS_NATIVE_OK;
}

void prometheus_runtime_destroy(prometheus_runtime* runtime) {
  free(runtime);
}

int prometheus_sgemm(prometheus_runtime* runtime,
                     int m,
                     int n,
                     int k,
                     const float* a,
                     const float* b,
                     float* c,
                     int* out_stage,
                     int* out_code) {
  if (out_stage != NULL) {
    *out_stage = PROMETHEUS_NATIVE_STAGE_UNKNOWN;
  }
  if (out_code != NULL) {
    *out_code = PROMETHEUS_NATIVE_OK;
  }
  if (runtime == NULL || a == NULL || b == NULL || c == NULL || m <= 0 || n <= 0 || k <= 0) {
    if (out_stage != NULL) {
      *out_stage = PROMETHEUS_NATIVE_STAGE_INIT;
    }
    if (out_code != NULL) {
      *out_code = PROMETHEUS_NATIVE_ERR_INVALID_ARGUMENT;
    }
    return PROMETHEUS_NATIVE_ERR_INVALID_ARGUMENT;
  }

  const char* fail_submit = getenv("PROMETHEUS_FORCE_SUBMIT_FAILURE");
  if (fail_submit != NULL && fail_submit[0] == '1') {
    if (out_stage != NULL) {
      *out_stage = PROMETHEUS_NATIVE_STAGE_SUBMIT;
    }
    if (out_code != NULL) {
      *out_code = PROMETHEUS_NATIVE_ERR_INTERNAL;
    }
    return PROMETHEUS_NATIVE_ERR_INTERNAL;
  }

  for (int row = 0; row < m; ++row) {
    for (int col = 0; col < n; ++col) {
      float sum = 0.0f;
      for (int kk = 0; kk < k; ++kk) {
        sum += a[row * k + kk] * b[kk * n + col];
      }
      c[row * n + col] = sum;
    }
  }

  return PROMETHEUS_NATIVE_OK;
}
