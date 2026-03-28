#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_BRIDGE_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_BRIDGE_H

#ifdef __cplusplus
extern "C" {
#endif

typedef struct prometheus_runtime prometheus_runtime;

enum {
  PROMETHEUS_NATIVE_OK = 0,
  PROMETHEUS_NATIVE_ERR_UNAVAILABLE = 1,
  PROMETHEUS_NATIVE_ERR_INVALID_ARGUMENT = 2,
  PROMETHEUS_NATIVE_ERR_INTERNAL = 3,
};

enum {
  PROMETHEUS_NATIVE_STAGE_UNKNOWN = 0,
  PROMETHEUS_NATIVE_STAGE_INIT = 1,
  PROMETHEUS_NATIVE_STAGE_TRANSFER_IN = 2,
  PROMETHEUS_NATIVE_STAGE_SUBMIT = 3,
  PROMETHEUS_NATIVE_STAGE_TRANSFER_OUT = 4,
  PROMETHEUS_NATIVE_STAGE_CLEANUP = 5,
};

int prometheus_runtime_create(prometheus_runtime** out_runtime);
void prometheus_runtime_destroy(prometheus_runtime* runtime);

int prometheus_sgemm(prometheus_runtime* runtime,
                     int m,
                     int n,
                     int k,
                     const float* a,
                     const float* b,
                     float* c,
                     int* out_stage,
                     int* out_code);

#ifdef __cplusplus
}
#endif

#endif
