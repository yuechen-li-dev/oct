#include "bridge.h"

#include <cstdlib>

struct prometheus_runtime {
  int placeholder;
};

int prometheus_runtime_create(prometheus_runtime** out_runtime) {
  if (out_runtime == nullptr) {
    return PROMETHEUS_NATIVE_ERR_INVALID_ARGUMENT;
  }
  const char* unavailable = std::getenv("PROMETHEUS_FORCE_UNAVAILABLE");
  if (unavailable != nullptr && unavailable[0] == '1') {
    *out_runtime = nullptr;
    return PROMETHEUS_NATIVE_ERR_UNAVAILABLE;
  }
  *out_runtime = new prometheus_runtime{1};
  return PROMETHEUS_NATIVE_OK;
}

void prometheus_runtime_destroy(prometheus_runtime* runtime) {
  delete runtime;
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
  if (out_stage != nullptr) {
    *out_stage = PROMETHEUS_NATIVE_STAGE_UNKNOWN;
  }
  if (out_code != nullptr) {
    *out_code = PROMETHEUS_NATIVE_OK;
  }
  if (runtime == nullptr || a == nullptr || b == nullptr || c == nullptr || m <= 0 || n <= 0 || k <= 0) {
    if (out_stage != nullptr) {
      *out_stage = PROMETHEUS_NATIVE_STAGE_INIT;
    }
    if (out_code != nullptr) {
      *out_code = PROMETHEUS_NATIVE_ERR_INVALID_ARGUMENT;
    }
    return PROMETHEUS_NATIVE_ERR_INVALID_ARGUMENT;
  }

  const char* fail_submit = std::getenv("PROMETHEUS_FORCE_SUBMIT_FAILURE");
  if (fail_submit != nullptr && fail_submit[0] == '1') {
    if (out_stage != nullptr) {
      *out_stage = PROMETHEUS_NATIVE_STAGE_SUBMIT;
    }
    if (out_code != nullptr) {
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
