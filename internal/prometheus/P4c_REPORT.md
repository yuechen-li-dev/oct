# P4c Report — First SGEMM-Shaped Reactor Path

## Outcome

P4c is implemented with a **CPU-backed Reactor SGEMM path** through the Bridge↔Reactor ABI.

This proves the first real SGEMM-shaped execution contract without introducing Vulkan-kernel complexity in this milestone.

## ABI addition

Added one C ABI entrypoint:

- `prometheus_reactor_runtime_sgemm(handle, a, b, c, m, n, k, out_stage, out_detail_code)`

Contract characteristics:

- C-compatible POD-only signature
- row-major `float32` SGEMM shape (`C = A x B`)
- explicit dimensions `M,N,K`
- explicit stage/detail outputs for status propagation
- explicit error returns (`PROM_OK`, `PROM_ERROR`, `PROM_INVALID_HANDLE`)

Also added stage codes to ABI header for boundary-safe status mapping.

## Bridge behavior after P4c

- Bridge now requires and resolves SGEMM symbol: `prometheus_reactor_runtime_sgemm`
- `nativeRuntime.SGEMM` now calls Reactor SGEMM function pointer (instead of local CPU implementation)
- Native stage/detail status maps into existing `RunStatus` (`ErrorStage` + error code)
- Existing explicit fallback behavior remains unchanged when Reactor is missing
- Existing explicit init-error behavior remains unchanged for bad Reactor init/probe
- Correctness gate remains CPU-oracle-based and mandatory

## Reactor behavior after P4c

- Runtime validates handle and pointer/dimension inputs
- Performs deterministic row-major `float32` SGEMM in native code
- Returns explicit stage/detail status for success and failure paths

## Vulkan access reality (environment honesty)

Attempted Vulkan runtime/dev/tool access in this environment:

1. Initial probe showed Vulkan tools/dev metadata missing (`vulkaninfo` not found, `pkg-config vulkan` missing)
2. Installed narrow packages:
   - `vulkan-tools`
   - `libvulkan-dev`
   - `mesa-vulkan-drivers`
3. Re-probed with `vulkaninfo --summary`

Result:

- Vulkan loader/tools are now callable.
- Available Vulkan device in this environment is `llvmpipe` (`PHYSICAL_DEVICE_TYPE_CPU`) rather than a real GPU path.

Therefore this milestone intentionally keeps **CPU-backed Reactor SGEMM** and does **not** claim real Vulkan-backed Prometheus runtime behavior.

## Correctness

Correctness remains gated against CPU oracle via existing Prometheus comparison flow:

- full output compare
- absolute/relative tolerance checks
- run fails on correctness mismatch

## Status / error / fallback model

Preserved and extended explicitly:

- fallback still explicit when Reactor is not present
- init/probe errors still explicit and non-fallback
- SGEMM runtime errors now carry explicit stage/code from Reactor boundary

## Testing added

### Go side

- Prometheus path verifies native SGEMM entrypoint invocation and correctness
- native SGEMM failure propagation verifies explicit stage/code mapping

### Native Marionette

- SGEMM deterministic product correctness
- invalid argument handling (null handle, null buffers, zero dimension)

## Deferred work (intentional)

- Vulkan compute kernel implementation
- GPU/queue/resource lifetime orchestration
- performance tuning and benchmark claims
- broader kernel/backend surface
