# Prometheus (Experimental)

> **Experimental:** Prometheus is a distinct subsystem.
> Its APIs, behavior, and runtime integration points may change.
> Do not treat Prometheus as stable core language surface.

## Overview

Prometheus is an accelerator-oriented subsystem with explicit fallback behavior.
It is separate from the core builtin story and documented separately for that reason.

For core builtin APIs, see [09 builtins](../language/09-builtins.md).
For standard library APIs, see [17 standard libraries](../language/17-standard-libraries.md).

## Current user-facing Prometheus surface

Prometheus-related language/runtime entry points currently include:

- `PROMETHEUS { ... }` block form (benchmark-scoped execution surface).
- `PrometheusMatMul(a, b)` builtin for `Matrix<Float>` arguments.
- CLI command surfaces such as `oct prometheus-sgemm` and `oct prometheus-m1-async`.

## Conceptual fit

- Core builtins define stable language/runtime primitives.
- Standard libraries define practical user programming APIs.
- Prometheus defines an experimental acceleration path with explicit runtime status reporting and fallback semantics.

## Important caveats

- Prometheus availability depends on environment/runtime setup.
- Fallback to CPU is explicit and expected when Prometheus runtime is unavailable.
- Treat Prometheus as opt-in experimental infrastructure rather than baseline language capability.

## Documentation ownership note

Prometheus-specific surfaces are intentionally excluded from [09 builtins](../language/09-builtins.md) to keep core builtin documentation focused.
