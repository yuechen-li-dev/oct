# P16 M4b — FFT Vulkan Radix-2 Benchmark Execution (Status Report)

## Status

**Honest stop (not converged to execution):** The benchmark variant remains unavailable in this snapshot. No host-side FFT execution has been added.

## What was validated first

- Reconfirmed `internal/prometheus/native/reactor_vulkan_fft.c` contains validation/plan diagnostics only and does not perform butterfly math in C.
- Reconfirmed benchmark variant currently records requested radix-2 path while preserving `executed_path_id = PROM_FFT_PATH_UNAVAILABLE`.
- Reconfirmed production API remains unavailable.

## Blocker isolated

The current native layout keeps Vulkan runtime ownership (`prometheus_runtime` with `VkDevice`, queues, command pools, descriptor/pipeline resources) private to `reactor_vulkan_sgemm.c`, while FFT plumbing lives in `reactor_vulkan_fft.c` without access to those owned objects.

To implement real Vulkan FFT dispatch, one of these structural changes is required first:

1. Move FFT Vulkan execution into the runtime ownership translation unit (`reactor_vulkan_sgemm.c`), **or**
2. Introduce a shared runtime/internal header that exports the runtime layout or narrow accessor APIs for command submission/pipeline creation/buffer operations used by FFT.

Without this split-resolution, `reactor_vulkan_fft.c` cannot legally create FFT compute pipeline/descriptor/command submissions against runtime-owned Vulkan objects.

## Scope deliberately not attempted in this stop

- No host-side C FFT fallback was introduced.
- No semantic authority duplication was introduced.
- No capability claim was added.
- No production FFT enablement was added.

## Next concrete step

Create a minimal internal FFT Vulkan execution seam (recommended option 1: colocate FFT dispatch with runtime owner TU), then implement:

- FFT SPIR-V shader module and pipeline
- per-call benchmark buffers (upload/ping/pong/readback)
- command record/dispatch per radix-2 pass
- truthful diagnostics for executed benchmark path and dispatch counters

