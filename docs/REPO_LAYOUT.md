# Repository Layout

This file documents the intended top-level structure and ownership boundaries.

## Top-Level Map

- `Language/` — canonical language contracts, reference docs, and semantic test corpus.
- `Libraries/` — reusable Oct package ecosystem.
- `cmd/` + `internal/` — GoOct implementation (CLI, compiler/runtime pipeline, reference backend internals).
- `Backends/ClrOct/` — CLR backend scaffold and planning surface (no backend implementation yet).
- `docs/` — current architecture and workflow truth.
- `Examples/` — polished, onboarding-friendly examples.
- `Experiments/` — exploratory/scratch/lab artifacts and probes.
- `tools/` — support tooling (editor integrations, helpers, etc.).
- `testdata/` — fixtures/transitional data (not the canonical semantics source).

## Examples vs Experiments

- **Examples/** are curated and stable for readers learning Oct.
- **Experiments/** are intentionally exploratory and may include milestone-specific artifacts.

When in doubt:

- polished educational artifact → `Examples/`
- exploratory investigation/probe → `Experiments/`

## Documentation Priority

Current truth lives in `docs/`.

Milestone reports (for example `M*_REPORT.md`, `P*_REPORT.md`) are historical records and should be read as timeline context, not as canonical current-state architecture documents.
