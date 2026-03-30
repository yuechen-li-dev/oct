# Oct v0

Oct is a compiled-first, statically typed, function-first language for scientific and technical programming.

The language definition lives in Oct source contracts under `Language/`. The Go implementation (**GoOct**) is the current reference backend, not the definition of the language.

## Start Here (Current Truth)

For current, maintained details, start with:

- `docs/ARCHITECTURE.md` — architecture and execution model
- `docs/REPO_LAYOUT.md` — repository map and responsibilities
- `docs/BACKENDS.md` — backend status and roadmap context
- `docs/TESTING.md` — how `.octest` / `.octfail` are used
- `Language/reference/` — language/reference corpus

Historical milestone reports (`M*_REPORT.md`, `P*_REPORT.md`) are retained as records, but `docs/` + `Language/reference/` are the current truth.

## Current Status

- Oct v0 is in active development.
- **GoOct** (`cmd/`, `internal/`) is the current reference backend.
- A working core path exists across language contracts, packages/libraries, testing, and compiled execution.
- Advanced/deferred areas are intentionally narrow and documented explicitly.

See `docs/ARCHITECTURE.md` for the detailed status narrative and design boundaries.

## Repository Layout at a Glance

- `Language/` — canonical language semantics and contract tests (`.octest`, `.octfail`)
- `Libraries/` — reusable Oct libraries/packages
- `cmd/` + `internal/` — GoOct implementation
- `Backends/ClrOct/` — ClrOct scaffold/placeholder backend path
- `Examples/` — curated examples
- `Experiments/` — exploratory work
- `tools/` — tooling support
- `docs/` — architecture, backend, testing, and repo documentation

## What This Repository Contains

This repository contains:

- language semantics and test corpus
- reusable libraries
- the GoOct reference backend
- the ClrOct scaffold
- examples, experiments, and support tools

Important boundary: GoOct is today’s reference implementation backend, while the language contract itself is expressed in Oct artifacts under `Language/`.

## What Makes Oct Different

- **Units as types:** SI dimensions participate directly in type checking.
- **Explicit fallibility:** error behavior is explicit in signatures and call sites.
- **Native language contracts:** valid and invalid behavior is captured with `.octest` / `.octfail`.
- **Explicit package model:** package surfaces and imports stay concrete and inspectable.

## Minimal Example

```oct
package Main

fn Main() -> Int {
    let x = 10
    return x * 2
}
```

## Prometheus/P2 Vertical Slice (Scoped)

A narrow Prometheus SGEMM slice exists to validate architecture and correctness shape (not performance claims).

- What it is: fixed SGEMM path (`float32`, row-major, explicit backend choice)
- Why it exists: prove backend selection/fallback/error wiring behavior
- CLI entrypoint: `oct prometheus-sgemm <cpu|prometheus> [--octagon-out <file.octagon>]`

See `docs/BACKENDS.md` for backend context and boundaries.
