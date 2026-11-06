#include "reactor_vulkan.h"
#include "reactor_shader_package.h"
#include "reactor_concept_fft.h"

#include <string.h>

#define PROM_FFT_DIRECTION_FORWARD 1u
#define PROM_FFT_DIRECTION_INVERSE 2u
#define PROM_FFT_MAX_PLAN_PASSES 20u
#define PROM_FFT_MAX_ELEMENT_COUNT (1u << PROM_FFT_MAX_PLAN_PASSES)
#define PROM_FFT_RADIX_BASELINE 2u
#define PROM_FFT_TWIDDLE_MODE_NATIVE_SIN_COS 2u
#define PROM_FFT_SHADER_BIT_REVERSE 52u
#define PROM_FFT_SHADER_BUTTERFLY 53u

#define PROM_FFT_BUFFER_ROLE_NONE 0u
#define PROM_FFT_BUFFER_ROLE_INPUT 1u
#define PROM_FFT_BUFFER_ROLE_PING 2u
#define PROM_FFT_BUFFER_ROLE_PONG 3u
#define PROM_FFT_BUFFER_ROLE_OUTPUT 4u

typedef struct prom_fft_plan_pass { uint32_t span, half_span, radix, source_role, destination_role; } prom_fft_plan_pass;
typedef struct prom_fft_plan {
  uint32_t element_count, batch_count, effective_stride_elements, direction;
  uint32_t log2_element_count, pass_count, bit_reversal_required, final_output_role, ping_pong_swap_count;
  prom_fft_plan_pass passes[PROM_FFT_MAX_PLAN_PASSES];
} prom_fft_plan;
typedef struct prom_fft_runtime_diag_slot { void* handle; PrometheusFftDiagnostics diag; } prom_fft_runtime_diag_slot;
typedef struct prom_fft_bit_reverse_push { uint32_t element_count, batch_count, stride_elements, reserved; } prom_fft_bit_reverse_push;
typedef struct prom_fft_butterfly_push { uint32_t element_count, batch_count, stride_elements, half_span, direction, normalize_inverse, reserved0, reserved1; } prom_fft_butterfly_push;

static prom_fft_runtime_diag_slot g_fft_diag_slots[32];

static PrometheusFftDiagnostics prom_fft_default_diag(void) {
  PrometheusFftDiagnostics diag; memset(&diag, 0, sizeof(diag));
  diag.struct_size = (uint32_t)sizeof(diag); diag.api_declared = 1u;
  diag.last_failure_detail = PROM_FFT_DETAIL_UNAVAILABLE;
  diag.last_validation_status = PROM_FFT_PATH_STATUS_UNAVAILABLE;
  diag.executed_path_id = PROM_FFT_PATH_UNAVAILABLE;
  return diag;
}

static PrometheusFftDiagnostics* prom_fft_diag_for_handle(void* handle) {
  uint32_t i; prom_fft_runtime_diag_slot* empty = NULL;
  for (i = 0u; i < 32u; ++i) { if (g_fft_diag_slots[i].handle == handle) return &g_fft_diag_slots[i].diag; if (empty == NULL && g_fft_diag_slots[i].handle == NULL) empty = &g_fft_diag_slots[i]; }
  if (empty == NULL) return NULL;
  empty->handle = handle; empty->diag = prom_fft_default_diag(); return &empty->diag;
}

static void prom_fft_stage_request(PrometheusFftDiagnostics* diag, const PrometheusFftRequest* request) {
  if (request == NULL) return;
  diag->last_element_count = request->element_count; diag->last_batch_count = request->batch_count;
  diag->last_stride_elements = request->stride_elements; diag->last_flags = request->flags;
  diag->last_effective_stride_elements = request->stride_elements == 0u ? request->element_count : request->stride_elements;
}
static uint32_t prom_fft_is_power_of_two(uint32_t value) { return value != 0u && (value & (value - 1u)) == 0u; }
static uint32_t prom_fft_log2_u32(uint32_t value) { uint32_t count = 0u; while (value > 1u) { value >>= 1u; ++count; } return count; }

static int prom_fft_validate_request(const PrometheusFftRequest* request, int* out_detail, uint32_t* out_direction, uint64_t* out_bytes) {
  uint64_t physical_elements;
  if (request == NULL || request->struct_size < sizeof(*request)) { *out_detail = PROM_FFT_DETAIL_INVALID_REQUEST; return 0; }
  if (request->input == NULL) { *out_detail = PROM_FFT_DETAIL_NULL_INPUT; return 0; }
  if (request->output == NULL) { *out_detail = PROM_FFT_DETAIL_NULL_OUTPUT; return 0; }
  if (request->element_count == 0u) { *out_detail = PROM_FFT_DETAIL_ZERO_ELEMENT_COUNT; return 0; }
  if (!prom_fft_is_power_of_two(request->element_count)) { *out_detail = PROM_FFT_DETAIL_NON_POWER_OF_TWO; return 0; }
  if (request->element_count > PROM_FFT_MAX_ELEMENT_COUNT) { *out_detail = PROM_FFT_DETAIL_UNSUPPORTED_SIZE; return 0; }
  if (request->batch_count == 0u) { *out_detail = PROM_FFT_DETAIL_ZERO_BATCH_COUNT; return 0; }
  if ((request->flags & PROM_FFT_FLAG_FORWARD) != 0u && (request->flags & PROM_FFT_FLAG_INVERSE) != 0u) { *out_detail = PROM_FFT_DETAIL_INVALID_DIRECTION_FLAGS; return 0; }
  if ((request->flags & PROM_FFT_FLAG_INVERSE_NORMALIZE) != 0u && (request->flags & PROM_FFT_FLAG_INVERSE) == 0u) { *out_detail = PROM_FFT_DETAIL_INVERSE_NORMALIZE_REQUIRES_INVERSE; return 0; }
  if (request->stride_elements != 0u && request->stride_elements < request->element_count) { *out_detail = PROM_FFT_DETAIL_INVALID_STRIDE; return 0; }
  if (request->stride_elements != 0u && request->batch_count > UINT32_MAX / request->stride_elements) { *out_detail = PROM_FFT_DETAIL_SIZE_OVERFLOW; return 0; }
  { uint32_t stride = request->stride_elements == 0u ? request->element_count : request->stride_elements;
    physical_elements = (uint64_t)(request->batch_count - 1u) * (uint64_t)stride + (uint64_t)request->element_count; }
  if (physical_elements == 0u || physical_elements > UINT64_MAX / sizeof(PrometheusComplex32) || physical_elements * sizeof(PrometheusComplex32) > UINT32_MAX) { *out_detail = PROM_FFT_DETAIL_SIZE_OVERFLOW; return 0; }
  /* Portable compute dispatch envelope; reject instead of emitting an invalid
     oversized one-dimensional grid for many small batches. */
  if ((uint64_t)request->element_count * request->batch_count > 65535ull * 256ull) { *out_detail = PROM_FFT_DETAIL_UNSUPPORTED_SIZE; return 0; }
  *out_direction = (request->flags & PROM_FFT_FLAG_INVERSE) != 0u ? PROM_FFT_DIRECTION_INVERSE : PROM_FFT_DIRECTION_FORWARD;
  *out_bytes = physical_elements * sizeof(PrometheusComplex32); return 1;
}

static void prom_fft_build_plan(const PrometheusFftRequest* request, uint32_t direction, prom_fft_plan* out_plan) {
  uint32_t i, src = PROM_FFT_BUFFER_ROLE_PING, dst = PROM_FFT_BUFFER_ROLE_PONG;
  memset(out_plan, 0, sizeof(*out_plan)); out_plan->element_count = request->element_count; out_plan->batch_count = request->batch_count;
  out_plan->effective_stride_elements = request->stride_elements == 0u ? request->element_count : request->stride_elements;
  out_plan->direction = direction; out_plan->log2_element_count = prom_fft_log2_u32(request->element_count); out_plan->pass_count = out_plan->log2_element_count;
  out_plan->bit_reversal_required = request->element_count > 1u ? 1u : 0u; out_plan->final_output_role = PROM_FFT_BUFFER_ROLE_PING;
  for (i = 0u; i < out_plan->pass_count; ++i) { prom_fft_plan_pass* pass = &out_plan->passes[i]; pass->span = 1u << (i + 1u); pass->half_span = pass->span >> 1u; pass->radix = PROM_FFT_RADIX_BASELINE; pass->source_role = src; pass->destination_role = dst; src = dst; dst = dst == PROM_FFT_BUFFER_ROLE_PING ? PROM_FFT_BUFFER_ROLE_PONG : PROM_FFT_BUFFER_ROLE_PING; }
  if (out_plan->pass_count > 0u) out_plan->final_output_role = out_plan->passes[out_plan->pass_count - 1u].destination_role;
  out_plan->ping_pong_swap_count = out_plan->pass_count;
}

static void prom_fft_apply_plan_diag(PrometheusFftDiagnostics* diag, const prom_fft_plan* plan) {
  diag->plan_valid = 1u; diag->plan_element_count = plan->element_count; diag->plan_log2_element_count = plan->log2_element_count;
  diag->plan_pass_count = plan->pass_count; diag->ping_pong_swap_count = plan->ping_pong_swap_count; diag->final_output_role = plan->final_output_role;
  diag->plan_first_span = plan->pass_count > 0u ? plan->passes[0].span : 0u; diag->plan_last_span = plan->pass_count > 0u ? plan->passes[plan->pass_count - 1u].span : 0u;
  diag->plan_radix_mask = plan->pass_count > 0u ? (1u << PROM_FFT_RADIX_BASELINE) : 0u; diag->plan_bit_reversal_required = plan->bit_reversal_required;
  diag->plan_first_source_role = plan->pass_count > 0u ? plan->passes[0].source_role : PROM_FFT_BUFFER_ROLE_INPUT;
  diag->plan_first_destination_role = plan->pass_count > 0u ? plan->passes[0].destination_role : PROM_FFT_BUFFER_ROLE_PING;
  diag->plan_direction = plan->direction; diag->plan_twiddle_mode = PROM_FFT_TWIDDLE_MODE_NATIVE_SIN_COS;
}

int prom_reactor_runtime_fft_impl(void* handle, const PrometheusFftRequest* request, uint32_t* out_stage, int* out_detail_code) {
  PrometheusFftDiagnostics* diag; prom_fft_plan plan; prom_vk_runtime_services services; prom_shader_package* package = NULL; int detail = PROM_FFT_DETAIL_UNAVAILABLE; uint32_t direction = PROM_FFT_DIRECTION_FORWARD; uint64_t bytes = 0u; int status;
  if (!prom_reactor_runtime_validate_handle(handle)) { prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE); return PROM_INVALID_HANDLE; }
  diag = prom_fft_diag_for_handle(handle); if (diag == NULL) { prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INTERNAL_ERROR); return PROM_INTERNAL_ERROR; }
  *diag = prom_fft_default_diag(); prom_fft_stage_request(diag, request);
  if (!prom_fft_validate_request(request, &detail, &direction, &bytes)) { diag->last_direction = direction; diag->last_failure_detail = detail; prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, detail); return PROM_ERROR; }
  prom_fft_build_plan(request, direction, &plan); prom_fft_apply_plan_diag(diag, &plan); diag->last_direction = direction; diag->requested_path_id = PROM_FFT_PATH_VULKAN_RADIX2_RESERVED; diag->requested_radix = PROM_FFT_RADIX_BASELINE;
  if (prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) { diag->last_failure_detail = PROM_FFT_DETAIL_VULKAN_UNAVAILABLE; prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_FFT_DETAIL_VULKAN_UNAVAILABLE); return PROM_ERROR; }
  if (prom_reactor_runtime_get_shader_package(handle, &package) != PROM_OK) { diag->last_failure_detail = PROM_FFT_DETAIL_SHADER_UNAVAILABLE; prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_FFT_DETAIL_SHADER_UNAVAILABLE); return PROM_ERROR; }
  /* M5/M6 has a production-generated shader portfolio and a real Vulkan
     execution route. A particular call can still fail later at allocation,
     upload, dispatch, or readback; that outcome is recorded separately. */
  diag->capability_reported = 1u; diag->production_enabled = 1u; diag->benchmark_enabled = 1u; diag->last_validation_status = PROM_FFT_PATH_STATUS_BENCHMARK_ENABLED;
  status = prom_concept_fft_execute(handle, request, bytes, out_stage, out_detail_code);
  if (status == PROM_OK) { diag->executed_path_id = PROM_FFT_PATH_VULKAN_RADIX2_RESERVED; diag->executed_radix = PROM_FFT_RADIX_BASELINE; diag->last_failure_detail = 0; return PROM_OK; }
  diag->executed_path_id = PROM_FFT_PATH_UNAVAILABLE; diag->last_failure_detail = out_detail_code != NULL ? *out_detail_code : PROM_FFT_DETAIL_DISPATCH_FAILED; return PROM_ERROR;
}

int prom_reactor_runtime_fft_benchmark_variant_impl(void* handle, const PrometheusFftRequest* request, uint32_t requested_variant, uint32_t* out_stage, int* out_detail_code) {
  if (requested_variant != PROM_FFT_BENCHMARK_VARIANT_RADIX2) { prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_FFT_DETAIL_PLAN_DISPATCH_DISAGREEMENT); return PROM_ERROR; }
  return prom_reactor_runtime_fft_impl(handle, request, out_stage, out_detail_code);
}

void prom_fft_diag_forget_handle(void* handle) { for (uint32_t i = 0u; handle != NULL && i < 32u; ++i) if (g_fft_diag_slots[i].handle == handle) { g_fft_diag_slots[i].handle = NULL; g_fft_diag_slots[i].diag = prom_fft_default_diag(); return; } }
int prom_reactor_runtime_fft_diagnostics_sized_impl(void* handle, PrometheusFftDiagnostics* out_diag, uint32_t out_size) { PrometheusFftDiagnostics* src; if (out_diag == NULL || out_size == 0u) return PROM_ERROR; if (!prom_reactor_runtime_validate_handle(handle)) return PROM_INVALID_HANDLE; src = prom_fft_diag_for_handle(handle); if (src == NULL) return PROM_INTERNAL_ERROR; if (out_size > sizeof(*out_diag)) out_size = (uint32_t)sizeof(*out_diag); memset(out_diag, 0, out_size); memcpy(out_diag, src, out_size); return PROM_OK; }
int prom_reactor_runtime_fft_diagnostics_impl(void* handle, PrometheusFftDiagnostics* out_diag) { return prom_reactor_runtime_fft_diagnostics_sized_impl(handle, out_diag, (uint32_t)sizeof(*out_diag)); }
