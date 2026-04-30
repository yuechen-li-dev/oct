# Random M1b compiled wiring report

## Root cause

M1a failed in compiled mode because `resolveCall` returned empty return type strings for `Random.*` field-access builtins. Tuple destructuring lowering requires tuple return metadata, so lowering failed with `compiled mode destructuring requires tuple return, got ""`.

## Signature fix

Compiled resolve-call builtin mapping now provides return types for:
- `Random.RngSeed -> Random.Rng`
- `Random.RandInt -> (Random.Rng, Int)`
- `Random.RandFloat01 -> (Random.Rng, Float)`
- `Random.RandFloatRange -> (Random.Rng, Float)`
- `Random.RandBernoulli -> (Random.Rng, Bool)`
- `Random.RandNormal -> (Random.Rng, Float)`
- `Random.CryptoRandInt -> Int !`
- `Random.CryptoRandFloat01 -> Float !`
- `Random.CryptoRandBytes -> Bytes !`

## Compiled execution approach

Compiled builtin emission adds Random builtin handling:
- deterministic tuple-return builtins emit single-call destructuring (`rng, value = __octRandom...(...)`) to preserve single RHS evaluation,
- `Random.RngSeed` emits `__octRandomRngSeed(...)`,
- crypto builtins emit fallible wrappers that map helper errors into compiled result types.

Helper implementation was added in codegen prelude (`__octRandomHelpers`) and uses the same deterministic algorithm family as interpreter runtime:
- SplitMix64 seeding,
- xoshiro256** stepping,
- Box-Muller for normal.

## Parity status

`go run ./cmd/oct test Libraries/Random` passes with deterministic tuple-threaded tests in compiled path.

## Crypto compiled status

Crypto builtins are wired for compiled mode in this patch via helper wrappers (`__octCryptoRand*`).

## Validation

- `go test ./internal/build ./internal/interpret ./internal/typecheck`
- `go test ./...`
- `go run ./cmd/oct test Libraries/Random`
