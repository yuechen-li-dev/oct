# P4d Report — First Vulkan-Backed Reactor SGEMM Path

## Outcome

P4d is implemented with a **real Vulkan-backed SGEMM execution path** inside the Prometheus Reactor C ABI.

The Bridge↔Reactor contract remains unchanged at the ABI boundary, with explicit stage and detail status propagation maintained.

## What changed (P4d scope)

### Reactor native implementation

The prior CPU-backed SGEMM stub in `internal/prometheus/native/bridge.c` was replaced by a Vulkan compute path that:

1. initializes Vulkan runtime state during `prometheus_reactor_runtime_create`
2. reports availability via `prometheus_reactor_runtime_probe`
3. for each SGEMM call:
   - allocates host-visible Vulkan storage buffers for A/B/C
   - uploads A and B via mapped memory
   - dispatches one compute pipeline using a single SGEMM shader
   - waits for completion
   - downloads C via mapped memory
   - returns explicit stage/detail status

The shader contract remains narrow and fixed:

- `float32` only
- row-major only
- `C = A x B`
- no transposition, mixed precision, tuning, streams, or backend framework expansion

### ABI/reporting compatibility

ABI remains C/POD-only with opaque handles and explicit stage/detail outputs.

Bridge behavior remains explicit:

- no hidden backend fallback inside Reactor
- fallback behavior for missing Reactor remains in Go-side runtime path
- SGEMM runtime failures remain surfaced with stage + detail code

## Header refinements

`internal/prometheus/native/bridge.h` adds Vulkan-specific backend/reason constants while preserving existing ABI style:

- `PROM_BACKEND_VULKAN`
- `PROM_REASON_VULKAN_UNAVAILABLE`

## Marionette coverage updates

`internal/prometheus/native/Marionette/reactor_stub_tests.cpp` now validates:

- ABI stability
- lifecycle and invalid argument handling
- probe determinism for both available and unavailable Vulkan runtime states
- SGEMM correctness against CPU oracle for starter shapes when Vulkan path is available
- explicit init-stage failure behavior when Vulkan runtime is unavailable

Floating-point correctness uses `ASSERT_NEAR`.

## Environment probing and dependency path

In this environment, initial Vulkan tooling was missing.

Attempted narrow install path:

- `vulkan-tools`
- `libvulkan-dev`
- `mesa-vulkan-drivers`
- `glslang-tools`

Probe after install (`vulkaninfo --summary`) reports Vulkan loader/device availability through Mesa llvmpipe:

- device type: `PHYSICAL_DEVICE_TYPE_CPU`
- device name: `llvmpipe (LLVM 20.1.2, 256 bits)`

So this environment provides Vulkan compute capability through a CPU Vulkan driver, not discrete GPU hardware.

## What was validated here

Validated in this environment:

- Vulkan compile/link path in Reactor build
- Reactor SGEMM dispatch through Vulkan compute pipeline
- deterministic correctness against CPU oracle (Marionette)
- Go-side Prometheus tests still passing where meaningful

## Deferred / not claimed

Not claimed in P4d:

- performance or benchmark gains
- discrete GPU validation
- CUDA or any additional backend
- kernel tuning/autotuning
- expanded operator surface

Hardware-backed validation (non-llvmpipe GPU path) remains a follow-up in an environment with real GPU device access.
