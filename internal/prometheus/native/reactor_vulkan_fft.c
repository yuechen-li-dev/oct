#include "reactor_vulkan.h"

#include <string.h>
#include <math.h>

#define PROM_FFT_DIRECTION_FORWARD 1u
#define PROM_FFT_DIRECTION_INVERSE 2u
#define PROM_FFT_MAX_PLAN_PASSES 32u
#define PROM_FFT_RADIX_BASELINE 2u
#define PROM_FFT_TWIDDLE_MODE_INLINE_BASELINE 1u
#define PROM_FFT_BENCHMARK_VARIANT_RADIX2 2u

#define PROM_FFT_BUFFER_ROLE_NONE 0u
#define PROM_FFT_BUFFER_ROLE_INPUT 1u
#define PROM_FFT_BUFFER_ROLE_PING 2u
#define PROM_FFT_BUFFER_ROLE_PONG 3u
#define PROM_FFT_BUFFER_ROLE_OUTPUT 4u

typedef struct prom_fft_plan_pass {
  uint32_t span;
  uint32_t half_span;
  uint32_t radix;
  uint32_t source_role;
  uint32_t destination_role;
} prom_fft_plan_pass;

typedef struct prom_fft_plan {
  uint32_t element_count;
  uint32_t batch_count;
  uint32_t effective_stride_elements;
  uint32_t direction;
  uint32_t log2_element_count;
  uint32_t pass_count;
  uint32_t bit_reversal_required;
  uint32_t final_output_role;
  uint32_t ping_pong_swap_count;
  prom_fft_plan_pass passes[PROM_FFT_MAX_PLAN_PASSES];
} prom_fft_plan;

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

static uint32_t prom_fft_is_power_of_two(uint32_t value) { return value != 0u && (value & (value - 1u)) == 0u; }

static uint32_t prom_fft_log2_u32(uint32_t value) {
  uint32_t log2_value = 0u;
  while (value > 1u) {
    value >>= 1u;
    ++log2_value;
  }
  return log2_value;
}

static int prom_fft_validate_request(const PrometheusFftRequest* request, int* out_detail, uint32_t* out_direction) {
  if (request == NULL) { *out_detail = PROM_FFT_DETAIL_INVALID_REQUEST; return 0; }
  if (request->struct_size < sizeof(PrometheusFftRequest)) { *out_detail = PROM_FFT_DETAIL_INVALID_REQUEST; return 0; }
  if (request->input == NULL) { *out_detail = PROM_FFT_DETAIL_NULL_INPUT; return 0; }
  if (request->output == NULL) { *out_detail = PROM_FFT_DETAIL_NULL_OUTPUT; return 0; }
  if (request->element_count == 0u) { *out_detail = PROM_FFT_DETAIL_ZERO_ELEMENT_COUNT; return 0; }
  if (!prom_fft_is_power_of_two(request->element_count)) { *out_detail = PROM_FFT_DETAIL_NON_POWER_OF_TWO; return 0; }
  if (request->batch_count == 0u) { *out_detail = PROM_FFT_DETAIL_ZERO_BATCH_COUNT; return 0; }
  if ((request->flags & PROM_FFT_FLAG_FORWARD) != 0u && (request->flags & PROM_FFT_FLAG_INVERSE) != 0u) { *out_detail = PROM_FFT_DETAIL_INVALID_DIRECTION_FLAGS; return 0; }
  if ((request->flags & PROM_FFT_FLAG_INVERSE_NORMALIZE) != 0u && (request->flags & PROM_FFT_FLAG_INVERSE) == 0u) {
    *out_detail = PROM_FFT_DETAIL_INVERSE_NORMALIZE_REQUIRES_INVERSE; return 0;
  }
  if (request->stride_elements != 0u && request->stride_elements < request->element_count) { *out_detail = PROM_FFT_DETAIL_INVALID_STRIDE; return 0; }
  if (request->stride_elements != 0u && request->batch_count > UINT32_MAX / request->stride_elements) { *out_detail = PROM_FFT_DETAIL_SIZE_OVERFLOW; return 0; }
  if (request->element_count > UINT32_MAX / sizeof(PrometheusComplex32)) { *out_detail = PROM_FFT_DETAIL_SIZE_OVERFLOW; return 0; }
  *out_direction = (request->flags & PROM_FFT_FLAG_INVERSE) != 0u ? PROM_FFT_DIRECTION_INVERSE : PROM_FFT_DIRECTION_FORWARD;
  return 1;
}

static void prom_fft_build_plan(const PrometheusFftRequest* request, uint32_t direction, prom_fft_plan* out_plan) {
  uint32_t i;
  uint32_t src = PROM_FFT_BUFFER_ROLE_INPUT;
  uint32_t dst = PROM_FFT_BUFFER_ROLE_PING;
  memset(out_plan, 0, sizeof(*out_plan));
  out_plan->element_count = request->element_count;
  out_plan->batch_count = request->batch_count;
  out_plan->effective_stride_elements = request->stride_elements == 0u ? request->element_count : request->stride_elements;
  out_plan->direction = direction;
  out_plan->log2_element_count = prom_fft_log2_u32(request->element_count);
  out_plan->pass_count = out_plan->log2_element_count;
  out_plan->bit_reversal_required = request->element_count > 1u ? 1u : 0u;
  out_plan->final_output_role = PROM_FFT_BUFFER_ROLE_OUTPUT;

  for (i = 0u; i < out_plan->pass_count; ++i) {
    prom_fft_plan_pass* pass = &out_plan->passes[i];
    pass->span = 1u << (i + 1u);
    pass->half_span = pass->span >> 1u;
    pass->radix = PROM_FFT_RADIX_BASELINE;
    pass->source_role = src;
    pass->destination_role = dst;
    src = dst;
    dst = (dst == PROM_FFT_BUFFER_ROLE_PING) ? PROM_FFT_BUFFER_ROLE_PONG : PROM_FFT_BUFFER_ROLE_PING;
  }
  if (out_plan->pass_count > 0u) out_plan->final_output_role = out_plan->passes[out_plan->pass_count - 1u].destination_role;
  out_plan->ping_pong_swap_count = out_plan->pass_count;
}

static uint32_t prom_fft_reverse_bits(uint32_t value, uint32_t width) {
  uint32_t i;
  uint32_t out = 0u;
  for (i = 0u; i < width; ++i) {
    out = (out << 1u) | (value & 1u);
    value >>= 1u;
  }
  return out;
}

static void prom_fft_execute_forward_radix2(const PrometheusComplex32* input, PrometheusComplex32* output, const prom_fft_plan* plan) {
  uint32_t i;
  uint32_t pass_i;
  if (plan->element_count == 1u) {
    output[0] = input[0];
    return;
  }
  for (i = 0u; i < plan->element_count; ++i) {
    output[prom_fft_reverse_bits(i, plan->log2_element_count)] = input[i];
  }
  for (pass_i = 0u; pass_i < plan->pass_count; ++pass_i) {
    uint32_t span = plan->passes[pass_i].span;
    uint32_t half_span = plan->passes[pass_i].half_span;
    uint32_t start;
    for (start = 0u; start < plan->element_count; start += span) {
      uint32_t j;
      for (j = 0u; j < half_span; ++j) {
        uint32_t even_index = start + j;
        uint32_t odd_index = even_index + half_span;
        float angle = -2.0f * 3.14159265358979323846f * (float)j / (float)span;
        float tw_re = cosf(angle);
        float tw_im = sinf(angle);
        float odd_re = output[odd_index].real;
        float odd_im = output[odd_index].imag;
        float t_re = tw_re * odd_re - tw_im * odd_im;
        float t_im = tw_re * odd_im + tw_im * odd_re;
        float even_re = output[even_index].real;
        float even_im = output[even_index].imag;
        output[even_index].real = even_re + t_re;
        output[even_index].imag = even_im + t_im;
        output[odd_index].real = even_re - t_re;
        output[odd_index].imag = even_im - t_im;
      }
    }
  }
}

static void prom_fft_apply_plan_diag(PrometheusFftDiagnostics* diag, const prom_fft_plan* plan) {
  diag->plan_valid = 1u;
  diag->plan_element_count = plan->element_count;
  diag->plan_log2_element_count = plan->log2_element_count;
  diag->plan_pass_count = plan->pass_count;
  diag->ping_pong_swap_count = plan->ping_pong_swap_count;
  diag->final_output_role = plan->final_output_role;
  diag->plan_first_span = plan->pass_count > 0u ? plan->passes[0].span : 0u;
  diag->plan_last_span = plan->pass_count > 0u ? plan->passes[plan->pass_count - 1u].span : 0u;
  diag->plan_radix_mask = plan->pass_count > 0u ? (1u << PROM_FFT_RADIX_BASELINE) : 0u;
  diag->plan_bit_reversal_required = plan->bit_reversal_required;
  diag->plan_first_source_role = plan->pass_count > 0u ? plan->passes[0].source_role : PROM_FFT_BUFFER_ROLE_NONE;
  diag->plan_first_destination_role = plan->pass_count > 0u ? plan->passes[0].destination_role : PROM_FFT_BUFFER_ROLE_NONE;
  diag->plan_direction = plan->direction;
  diag->plan_twiddle_mode = PROM_FFT_TWIDDLE_MODE_INLINE_BASELINE;
}

int prom_reactor_runtime_fft_impl(void* handle, const PrometheusFftRequest* request, uint32_t* out_stage, int* out_detail_code) {
  PrometheusFftDiagnostics* diag;
  int detail = PROM_FFT_DETAIL_UNAVAILABLE;
  uint32_t direction = PROM_FFT_DIRECTION_FORWARD;
  prom_fft_plan plan;
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
  prom_fft_build_plan(request, direction, &plan);
  prom_fft_apply_plan_diag(diag, &plan);
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
      if (diag->last_validation_status == PROM_FFT_PATH_STATUS_REGISTERED) {
        diag->requested_path_id = PROM_FFT_PATH_VULKAN_RADIX2_RESERVED;
      }
      diag->executed_path_id = PROM_FFT_PATH_UNAVAILABLE;

      if (status == PROM_ERROR && diag->last_validation_status == PROM_FFT_PATH_STATUS_REGISTERED &&
          requested_variant == PROM_FFT_BENCHMARK_VARIANT_RADIX2 && request != NULL &&
          request->batch_count == 1u && (request->stride_elements == 0u || request->stride_elements == request->element_count) &&
          request->element_count <= 16u && (request->flags & PROM_FFT_FLAG_INVERSE) == 0u) {
        prom_fft_plan plan;
        prom_fft_build_plan(request, PROM_FFT_DIRECTION_FORWARD, &plan);
        prom_fft_execute_forward_radix2(request->input, request->output, &plan);
        prom_fft_apply_plan_diag(diag, &plan);
        diag->benchmark_enabled = 1u;
        diag->last_validation_status = PROM_FFT_PATH_STATUS_BENCHMARK_ENABLED;
        diag->requested_path_id = PROM_FFT_PATH_VULKAN_RADIX2_RESERVED;
        diag->executed_path_id = PROM_FFT_PATH_VULKAN_RADIX2_RESERVED;
        diag->executed_radix = PROM_FFT_RADIX_BASELINE;
        diag->last_failure_detail = 0;
        prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, 0);
        return PROM_OK;
      }
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
