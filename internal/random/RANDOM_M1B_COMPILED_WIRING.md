# Random M1b compiled wiring report (historical, superseded API shape)

> Status (2026-04-30): This report documents historical compiled wiring work that used tuple-return Random builtins. The **current public API direction** is the Oct-shaped record-result model in `internal/random/Random.Core.md`.

## Historical root cause

M1a failed in compiled mode because `resolveCall` returned empty return type strings for `Random.*` field-access builtins. Tuple destructuring lowering required tuple return metadata, so lowering failed with `compiled mode destructuring requires tuple return, got ""`.

## Historical signature fix

At that time, compiled resolve-call builtin mapping provided return types for tuple-return signatures such as `Random.RandInt -> (Random.Rng, Int)` and related functions.

This is now superseded by the record-result API direction (`Next` / `Value`) for public Random.Core design.

## Historical compiled execution approach

Compiled builtin emission added Random builtin handling for deterministic tuple-return destructuring and crypto wrappers.

Helper implementation in codegen prelude used:

- SplitMix64 seeding,
- xoshiro256** stepping,
- Box-Muller for normal.

These algorithm notes remain relevant to implementation lineage, while tuple-return public API shape does not.

## Historical validation snapshot

- `go test ./internal/build ./internal/interpret ./internal/typecheck`
- `go test ./...`
- `go run ./cmd/oct test Libraries/Random`

For current API intent and examples, use `internal/random/Random.Core.md`.
