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

## Install (Go modules)

Install the main Oct CLI:

```bash
go install github.com/yuechen-li-dev/oct/cmd/oct@latest
```

Install the optional Octxiliary sidecar (needed for compiled programs that use Octxiliary-backed file/directory operations):

```bash
go install github.com/yuechen-li-dev/oct/cmd/octxiliary-io@latest
```

`oct` is the primary CLI. `octxiliary-io` is optional unless your compiled program path requires sidecar-backed wrappers.

Ensure your Go bin directory is on `PATH` (commonly `$(go env GOPATH)/bin` or your configured `GOBIN`), then verify:

```bash
oct --help
octxiliary-io --help
```

## Prometheus (EVT transition scaffold)

Prometheus is now scaffolded as an **optional** runtime capability:

- `oct` core remains pure Go in the default build path.
- Prometheus Reactor is expected as a dynamically loaded native library when requested.
- If no compatible Reactor is found, `prometheus` requests surface explicit `fallback(prometheus_unavailable)` and run on CPU.
- CLI entrypoint remains: `oct prometheus-sgemm <cpu|prometheus> [--octagon-out <file.octagon>]`

See `docs/reports/prometheus/P4A_REPORT.md` for the P4a transition report.
