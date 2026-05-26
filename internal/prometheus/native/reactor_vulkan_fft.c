#include "reactor_vulkan.h"

#include <string.h>

#define PROM_FFT_DIRECTION_FORWARD 1u
#define PROM_FFT_DIRECTION_INVERSE 2u

typedef struct prom_fft_runtime_diag_slot {
  void* handle;
  PrometheusFftDiagnostics diag;
} prom_fft_runtime_diag_slot;

static prom_fft_runtime_diag_slot g_fft_diag_slots[32];

static PrometheusFftDiagnostics prom_fft_default_diag(void) {
  PrometheusFftDiagnostics diag;
  memset(&diag, 0, sizeof(diag));
  diag.struct_size = (uint32_t)sizeof(PrometheusFftDiagnostics);
  diag.api_declared = 1u;
  diag.last_failure_detail = PROM_FFT_DETAIL_UNAVAILABLE;
  diag.last_validation_status = PROM_FFT_PATH_STATUS_UNAVAILABLE;
  diag.requested_path_id = PROM_FFT_PATH_NONE;
  diag.executed_path_id = PROM_FFT_PATH_UNAVAILABLE;
  return diag;
}

static PrometheusFftDiagnostics* prom_fft_diag_for_handle(void* handle) {
  uint32_t i;
  prom_fft_runtime_diag_slot* empty = NULL;
  for (i = 0u; i < 32u; ++i) {
    if (g_fft_diag_slots[i].handle == handle) return &g_fft_diag_slots[i].diag;
    if (empty == NULL && g_fft_diag_slots[i].handle == NULL) empty = &g_fft_diag_slots[i];
  }
  if (empty == NULL) return NULL;
  empty->handle = handle;
  empty->diag = prom_fft_default_diag();
  return &empty->diag;
}

static void prom_fft_stage_request(PrometheusFftDiagnostics* diag, const PrometheusFftRequest* request) {
  uint32_t effective_stride = 0u;
  if (request != NULL) {
    diag->last_element_count = request->element_count;
    diag->last_batch_count = request->batch_count;
    diag->last_stride_elements = request->stride_elements;
    diag->last_flags = request->flags;
    effective_stride = request->stride_elements == 0u ? request->element_count : request->stride_elements;
    diag->last_effective_stride_elements = effective_stride;
  }
}

static int prom_fft_validate_request(const PrometheusFftRequest* request, int* out_detail, uint32_t* out_direction) {
  if (request == NULL) { *out_detail = PROM_FFT_DETAIL_INVALID_REQUEST; return 0; }
  if (request->struct_size != sizeof(PrometheusFftRequest)) { *out_detail = PROM_FFT_DETAIL_INVALID_REQUEST; return 0; }
  if (request->input == NULL) { *out_detail = PROM_FFT_DETAIL_NULL_INPUT; return 0; }
  if (request->output == NULL) { *out_detail = PROM_FFT_DETAIL_NULL_OUTPUT; return 0; }
  if (request->element_count == 0u) { *out_detail = PROM_FFT_DETAIL_ZERO_ELEMENT_COUNT; return 0; }
  if ((request->element_count & (request->element_count - 1u)) != 0u) { *out_detail = PROM_FFT_DETAIL_NON_POWER_OF_TWO; return 0; }
  if (request->batch_count == 0u) { *out_detail = PROM_FFT_DETAIL_ZERO_BATCH_COUNT; return 0; }
  if ((request->flags & PROM_FFT_FLAG_FORWARD) != 0u && (request->flags & PROM_FFT_FLAG_INVERSE) != 0u) { *out_detail = PROM_FFT_DETAIL_INVALID_DIRECTION_FLAGS; return 0; }
  if (request->stride_elements != 0u && request->stride_elements < request->element_count) { *out_detail = PROM_FFT_DETAIL_INVALID_STRIDE; return 0; }
  *out_direction = (request->flags & PROM_FFT_FLAG_INVERSE) != 0u ? PROM_FFT_DIRECTION_INVERSE : PROM_FFT_DIRECTION_FORWARD;
  return 1;
}

int prom_reactor_runtime_fft_impl(void* handle, const PrometheusFftRequest* request, uint32_t* out_stage, int* out_detail_code) {
  PrometheusFftDiagnostics* diag;
  int detail = PROM_FFT_DETAIL_UNAVAILABLE;
  uint32_t direction = PROM_FFT_DIRECTION_FORWARD;
  if (!prom_reactor_runtime_validate_handle(handle)) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  diag = prom_fft_diag_for_handle(handle);
  if (diag == NULL) {
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INTERNAL_ERROR);
    return PROM_INTERNAL_ERROR;
  }
  *diag = prom_fft_default_diag();
  prom_fft_stage_request(diag, request);
  if (!prom_fft_validate_request(request, &detail, &direction)) {
    diag->last_direction = direction;
    diag->last_failure_detail = detail;
    prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, detail);
    return PROM_ERROR;
  }
  diag->last_direction = direction;
  diag->last_validation_status = PROM_FFT_PATH_STATUS_REGISTERED;
  diag->requested_path_id = PROM_FFT_PATH_CPU_ORACLE_RESERVED;
  diag->executed_path_id = PROM_FFT_PATH_UNAVAILABLE;
  diag->last_failure_detail = PROM_FFT_DETAIL_UNAVAILABLE;
  prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_FFT_DETAIL_UNAVAILABLE);
  return PROM_ERROR;
}

int prom_reactor_runtime_fft_benchmark_variant_impl(void* handle,
                                                    const PrometheusFftRequest* request,
                                                    uint32_t requested_variant,
                                                    uint32_t* out_stage,
                                                    int* out_detail_code) {
  int status = prom_reactor_runtime_fft_impl(handle, request, out_stage, out_detail_code);
  if (status == PROM_INVALID_HANDLE) return status;
  if (prom_reactor_runtime_validate_handle(handle)) {
    PrometheusFftDiagnostics* diag = prom_fft_diag_for_handle(handle);
    if (diag != NULL) {
      diag->requested_radix = requested_variant;
      diag->requested_path_id = PROM_FFT_PATH_VULKAN_RADIX2_RESERVED;
      diag->executed_path_id = PROM_FFT_PATH_UNAVAILABLE;
    }
  }
  return status;
}

int prom_reactor_runtime_fft_diagnostics_sized_impl(void* handle, PrometheusFftDiagnostics* out_diag, uint32_t out_size) {
  PrometheusFftDiagnostics* src;
  if (out_diag == NULL || out_size == 0u) return PROM_ERROR;
  if (!prom_reactor_runtime_validate_handle(handle)) return PROM_INVALID_HANDLE;
  src = prom_fft_diag_for_handle(handle);
  if (src == NULL) return PROM_INTERNAL_ERROR;
  if (out_size > sizeof(PrometheusFftDiagnostics)) out_size = (uint32_t)sizeof(PrometheusFftDiagnostics);
  memset(out_diag, 0, out_size);
  memcpy(out_diag, src, out_size);
  return PROM_OK;
}

int prom_reactor_runtime_fft_diagnostics_impl(void* handle, PrometheusFftDiagnostics* out_diag) {
  return prom_reactor_runtime_fft_diagnostics_sized_impl(handle, out_diag, (uint32_t)sizeof(PrometheusFftDiagnostics));
}
