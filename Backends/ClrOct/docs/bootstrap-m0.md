# ClrOct Bootstrap M0: .NET 10 + Minimal Hosted octest Harness

## Intent

M0 is a scaffold milestone that proves a CLR-hosted, Oct-shaped test path can exist.

## Explicit Claims

This milestone claims only:

1. A real .NET 10 solution exists under `Backends/ClrOct/`.
2. A tiny hosted octest harness exists with Fact + Assert semantics.
3. Pass/fail behavior is deterministic and testable via standard CLR test hosting.

## Non-Claims

This milestone does **not** claim:

- MIR lowering
- parser/frontend implementation
- package loading
- runtime parity
- backend parity with GoOct

GoOct remains the reference backend.

## Why octest-first

The first CLR bootstrap step is `octest`-first to establish semantic shape and confidence in host integration before backend implementation work begins.
