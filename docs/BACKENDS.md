# Backends

## Current State

Oct currently has one active backend path:

- **GoOct (reference backend):** implemented through `cmd/` and `internal/`.

A second path now exists as scaffolding:

- **ClrOct (escape hatch path):** `Backends/ClrOct/`.

## Why ClrOct Exists

`Backends/ClrOct/` is an explicit place to evaluate a CLR-oriented backend strategy without destabilizing GoOct.

It exists to prove feasibility boundaries and backend portability strategy over time.

## What ClrOct Is Not (Yet)

- Not a functioning backend implementation.
- Not a replacement for GoOct today.
- Not an architecture redesign effort.

## Reference Backend Policy

GoOct remains the canonical reference backend for language behavior and implementation reality.

Any future ClrOct work must preserve the language contract ownership model:

- semantics in `Language/`
- implementation in backend/runtime code

## Bootstrap Strategy (Octest-First)

Any ClrOct bring-up should follow an **octest-first** validation strategy:

1. Reuse existing language contracts from `Language/`.
2. Bring up backend capability incrementally against that corpus.
3. Avoid introducing duplicate semantic definitions in backend-specific test code.

This keeps `Language/` as the single source of truth while allowing backend experimentation.
