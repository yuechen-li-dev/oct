# ClrOct Bootstrap (M0)

ClrOct is an **escape hatch backend path** for Oct on the CLR.

## Status

This directory now contains an **octest-first bootstrap proof**, not a backend implementation.

- A real .NET 10 solution exists under `Backends/ClrOct/`.
- A tiny hosted octest harness exists in C#.
- Deterministic Fact/Assert pass-fail behavior is demonstrated through the CLR test host.

## What This Is

- A scaffold proving ClrOct can be a real backend track.
- A minimal hosted test model shaped like Oct's `octest` semantics.
- An explicit first step before backend internals.

## What This Is Not

- Not MIR lowering.
- Not parser/frontend work.
- Not package loading.
- Not runtime/backend parity with GoOct.

GoOct remains the reference backend.

## Bootstrap Direction

This milestone is deliberately **octest-first, not arithmetic-first**:

1. Host minimal Fact/Assert test semantics on CLR.
2. Keep behavior deterministic and inspectable.
3. Expand only after bootstrap proof quality is established.

## Layout

- `ClrOct.sln` — .NET 10 solution root.
- `src/ClrOct.Octest/` — minimal hosted octest harness types.
- `tests/ClrOct.Octest.Tests/` — CLR-hosted tests proving pass/fail semantics.
- `docs/` — concise milestone documentation.
