# P4E REPORT — Prometheus Hardening (Correctness, Structure, Boundary Clarity)

## Scope

P4e hardens the existing SGEMM Vulkan reactor path without expanding feature surface.

## Structural changes

- Split native reactor boundary into:
  - `reactor_api.c` / `reactor_api.h`: ABI surface only (exports + argument forwarding).
  - `reactor_vulkan.c` / `reactor_vulkan.h`: Vulkan runtime + SGEMM implementation.
- `bridge.h` is now a compatibility include that forwards to `reactor_api.h`.
- Build wiring updated so reactor library and Marionette compile against `reactor_api.c` + `reactor_vulkan.c`.

## Correctness hardening

- Expanded Marionette SGEMM shape coverage to include:
  - non-square shapes
  - degenerate `1xN` and `Nx1`
  - awkward small shapes
- Added deterministic repeated-run SGEMM assertion.
- Added explicit finite-value checks (`std::isfinite`) and tolerance checks (`ASSERT_NEAR`).
- Added Go-side `compareAgainstOracle` test that rejects NaN/Inf.

## Failure-path hardening

- Added test-only config flags via `PrometheusReactorConfig` to inject controlled failures for:
  - device creation
  - pipeline creation
  - buffer allocation
  - upload
  - dispatch
  - download
- Failure injections are surfaced with explicit stage + detail codes and never silent success.
- Marionette now asserts stage/detail correctness for injected failures.

## Reporting integrity hardening

- Added Go runtime invariant test to ensure native Prometheus failures remain error status and are never reported as fallback success.
- Preserved explicit requested-vs-used backend behavior.

## Environment honesty

- Marionette runtime-path tests explicitly skip only when Vulkan runtime is unavailable.
- Unavailable Vulkan remains a reported unavailability path, not a hidden success path.
