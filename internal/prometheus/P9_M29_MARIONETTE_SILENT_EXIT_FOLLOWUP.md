# P9 M29 Follow-up — Marionette Full-Suite Silent Exit

## What happened

- Running `out/prometheus/native/marionette_tests` through a non-tty command path produced no printed harness lines and surfaced tool exit `-1`.
- Re-running with line-buffer forcing (`stdbuf`) showed the native process actually terminated with **SIGSEGV** (exit `139`) during full-suite execution.

## Isolation steps

1. Built native artifacts with `bash internal/prometheus/native/build_stub.sh`.
2. Ran full suite with line-buffered stdout redirection:
   - `stdbuf -oL -eL out/prometheus/native/marionette_tests > /tmp/marionette_full.log 2>&1`
   - observed segfault and partial log.
3. Used sorted test names and filter runs to isolate the first crashing test:
   - `PrometheusReactor_FailedCallDoesNotOverwritePacked4SelectedLayoutDiagnostic`
4. Reproduced crash in a detached worktree at `HEAD^` (pre-M29 slot integration baseline), confirming crash existed before M29.

## Root cause

`prom_reactor_runtime_sgemm_impl` evaluated policy/tolerance facts using raw matrix pointers **before** guarding extreme dimension overflow input used by this test (`UINT32_MAX` shape). That allowed out-of-bounds reads in preflight analysis loops and caused segfault instead of returning deterministic `PROM_DETAIL_SIZE_OVERFLOW`.

## M29 implication

Not attributable to fixed-double slot orchestration. The crash reproduces in the immediate pre-M29 parent revision.

## Fix applied

1. Added early dimension overflow guard in `prom_reactor_runtime_sgemm_impl` before tolerance/path preflight that dereferences input buffers.
2. Guard now returns transfer-in `PROM_DETAIL_SIZE_OVERFLOW` deterministically for impossible dimensions.
3. Added minimal harness observability improvement:
   - print `[RUN] <TestName>` and flush before each test starts, so future crashes identify the currently executing test even in buffered/non-tty contexts.

## Final validation

- `out/prometheus/native/marionette_tests PrometheusReactor_FailedCallDoesNotOverwritePacked4SelectedLayoutDiagnostic` now completes and passes (no segfault).
- Full suite now runs to completion and emits summary output.
- M29 targeted filter still passes/skips as expected on software Vulkan:
  - `out/prometheus/native/marionette_tests M29_FixedDouble`
