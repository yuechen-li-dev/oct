# P16 M4c — FFT Vulkan Runtime Access Seam

## Scope

M4c introduces a **minimal internal Vulkan runtime services seam** so non-SGEMM reactor families (FFT now, fused reduction later) can query generic runtime Vulkan objects without accessing SGEMM-private policy/controller/arena internals.

This milestone does **not** implement FFT execution.

## M4b blocker summary

P16 M4b documented that `reactor_vulkan_fft.c` could not legally dispatch Vulkan compute because runtime-owned Vulkan state lives in `reactor_vulkan_sgemm.c`.

M4c resolves this ownership-access blocker by adding a narrow getter seam.

## Audit answers

1. **Where is the runtime struct currently defined?**
   - `prometheus_runtime` remains defined in `internal/prometheus/native/reactor_vulkan_sgemm.c`.

2. **Which Vulkan fields were SGEMM-private but runtime-generic?**
   - `instance`, `physical_device`, `device`, `compute_queue`, `queue_family_index`, and `command_pool`.
   - backend availability metadata (`available`, `reason_code`) and `test_flags` are also generic runtime context needed for truthful availability behavior.

3. **Which helpers already existed in `reactor_vulkan_common.c`?**
   - status staging helper (`prom_vk_set_status`)
   - checked u32 multiply (`prom_vk_checked_mul_u32`)
   - memory-type selection (`prom_vk_find_memory_type`)
   - shared buffer create/destroy (`prom_vk_create_buffer`, `prom_vk_destroy_buffer`)

4. **Minimal safe service surface FFT needs for future benchmark dispatch**
   - physical + logical device handles
   - compute queue + queue family index
   - compute command pool handle
   - backend availability/reason for truthful short-circuit behavior
   - runtime test flags for injected-failure compatibility in native tests

5. **What remains SGEMM-private?**
   - Dominatus state, policy memory, selector caches
   - SGEMM arena ownership/reuse and buffer artifact keys
   - SGEMM pipelines/layout variants and occupancy controller state
   - batch/HFSM slot internals and SGEMM diagnostics internals

## Chosen seam shape

Implemented **Option A** (opaque service struct + getter):

- Added `prom_vk_runtime_services` to `reactor_vulkan.h` with only generic Vulkan/runtime fields.
- Added `prom_reactor_runtime_get_vk_services(void* handle, prom_vk_runtime_services* out_services)`.

Behavior:
- validates handle using existing runtime handle validation semantics
- returns `PROM_INVALID_HANDLE` for invalid/destroyed handles
- returns `PROM_ERROR` when backend unavailable or essential Vulkan handles are null
- returns `PROM_OK` with borrowed service handles on success

Ownership/lifetime:
- returned handles are borrowed views; ownership/destruction remains runtime-owned.

## Why this is minimal

- No public reactor API expansion was needed.
- No SGEMM execution, policy, or topology behavior moved.
- No god-header expansion beyond a tiny, runtime-generic seam.
- No FFT execution and no capability claim changes.

## Tests added

Marionette FACT coverage added in `reactor_fft_api_tests.cpp`:
- invalid and destroyed handle rejection for service seam
- truthful availability behavior:
  - success path when Vulkan backend is available
  - clean `PROM_ERROR` path when Vulkan backend unavailable

Existing FFT diagnostics default-off and unavailable behavior remains unchanged.

## Validation

Ran:
- `bash internal/prometheus/native/build_stub.sh`
- `out/prometheus/native/marionette_tests PrometheusReactor_Fft`
- `out/prometheus/native/marionette_tests PrometheusReactor_Sgemm`
- `go run ./cmd/oct test Experiments/PrometheusFftAlgorithmLab/M1`
- `go run ./cmd/oct artifact Experiments/PrometheusFftAlgorithmLab/M1`
- `go test ./internal/... ./cmd/oct`

## Next step

P16 M4d / M4b-followup should wire real Vulkan radix-2 benchmark execution in FFT using this seam, while preserving:
- no host-side FFT math
- production path unavailable
- no capability claim until warranted
