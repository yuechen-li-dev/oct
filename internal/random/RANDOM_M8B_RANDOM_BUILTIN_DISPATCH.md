# RANDOM_M8B_RANDOM_BUILTIN_DISPATCH

Date: 2026-05-01

## Root cause

Random deterministic and crypto APIs were implemented in runtime/compiler builtin handlers, but same-package unqualified calls (`RngSeed(...)`) were not recognized as builtin names. The interpreter/typechecker/compiler therefore resolved those calls to `Libraries/Random/Random.Core.oct` function bodies, which were constant stubs.

## Call path before fix

- `RngSeed`/`RandFloat01` in package `Random` -> not in `builtin.IsName` as unqualified names.
- Normal function resolution selected `Random.Core.oct` definitions.
- Stub body executed (constant/placeholder behavior).
- `evalBuiltinCallExpr` and compiled Random helpers were bypassed for same-package unqualified calls.

## Call path after fix

- Builtin registry now includes both qualified and unqualified Random API names.
- In typecheck/build, unqualified Random names in package `Random` are normalized to qualified builtin names (`Random.*`).
- Calls dispatch to runtime builtin path in interpreter and compiler.
- `Random.Core.oct` stubs are guarded with `Require(false, ...)` to prevent silent fallback execution.

## Random.Core.oct stub policy

`Random.Core.oct` keeps public type/result record/function signatures for package surface compatibility, but executable fallback stub behavior is disallowed. Any direct execution now fails loudly.

## Interpreted vs compiled behavior

Both interpreted (`internal/interpret/evalBuiltinCallExpr`) and compiled (`internal/build` Random helper emission + call lowering) paths route to SplitMix64 + xoshiro256** logic for deterministic APIs and crypto entropy for crypto APIs.

## Tests added/fixed

- Revalidated strengthened non-degeneracy/state-threading coverage in `Libraries/Random/Random.Core.octest` under same-package unqualified dispatch (the original failing path).
- `go run ./cmd/oct test Libraries/Random` now passes with builtin-routed calls instead of executing stubs.

## P14 M2 status

See validation command results in this change; this M8b fix removes the Random stub-dispatch class of failure.
