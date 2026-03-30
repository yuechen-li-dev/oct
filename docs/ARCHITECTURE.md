# Architecture (Current Truth)

This document is the current architecture truth for the Oct repository.

## Layer Model

Oct uses a strict separation of concerns:

- **Oct (language layer):** language semantics, contracts, and reusable ecosystem code.
- **Go (implementation layer):** parser, type checker, runtime, CLI, and reference backend implementation.
- **Backend layer:** backend-specific implementation paths, where **GoOct** is the active reference backend and **ClrOct** is scaffold-only.

## Backend Naming

- **GoOct** = current reference backend implemented in `cmd/` + `internal/`.
- **ClrOct** = explicit escape-hatch backend path under `Backends/ClrOct/` (scaffold only at this stage).

## Contract Ownership

- Language semantics and contracts live in `Language/` (`.octest` and `.octfail`).
- Reusable user-space packages live in `Libraries/`.
- Backend/compiler/runtime behavior is implemented in Go.

If behavior is user-visible language semantics, it should be expressed in `Language/` rather than re-specified in Go tests.

## Repository Reading Order

For onboarding, read in this order:

1. `docs/REPO_LAYOUT.md`
2. `docs/BACKENDS.md`
3. `docs/TESTING.md`
4. `docs/LLM_LAB.md`

Historical milestone reports remain valuable context but are not the primary source of current architecture truth.
