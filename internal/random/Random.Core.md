# Random.Core (M0 Oct-Shaped Edition)

This document defines the **Oct-shaped** public design for `Random.Core`.

Tuple-threaded public APIs are rejected for Random. Random draws are modeled as explicit state transitions using **records** with named fields.

## 1) Why tuple-threading was rejected

The previously explored public shape:

```oct
rng, value = Random.RandInt(rng, 1, 6)
```

is rejected for `Random.Core` because it introduces positional meaning at the call site, pushes tuple-first user ergonomics, and reflects host-language implementation convenience more than Oct-facing design.

Random in Oct should expose state transitions via named data, not position.

## 2) Oct-shaped design principles

Random.Core follows existing Oct constructs:

- **Record**: named heterogeneous data.
- **Enum**: closed domain outcomes.
- **Match**: consume structured enum alternatives (especially payload-bearing cases).
- **Array**: homogeneous sequences.

No new language features are required.

If any existing experiments or older docs conflict with this direction, this document is the source of truth for Random.Core API shape.

## 3) Public `Random.Core` API

```oct
package Random

type Rng

record RandIntResult {
    Next: Rng
    Value: Int
}

record RandFloatResult {
    Next: Rng
    Value: Float
}

record RandBoolResult {
    Next: Rng
    Value: Bool
}

RngSeed(seed: Int) -> Rng

RandInt(rng: Rng, min: Int, max: Int) -> RandIntResult
RandFloat01(rng: Rng) -> RandFloatResult
RandFloatRange(rng: Rng, min: Float, max: Float) -> RandFloatResult
RandBernoulli(rng: Rng, p: Float) -> RandBoolResult
RandNormal(rng: Rng, mean: Float, stddev: Float) -> RandFloatResult

CryptoRandInt!(min: Int, max: Int) -> Int
CryptoRandFloat01!() -> Float
CryptoRandBytes!(count: Int) -> Bytes
```

Deterministic APIs are explicit-state and infallible (for valid programmer input). Crypto APIs are fallible and entropy-backed.

## 4) Result record definitions

Result records must expose these field names:

- `Next`: next RNG state after the draw.
- `Value`: sampled value.

`Next` is preferred over `Rng` as the transition field name because it communicates progression rather than storage.

## 5) Deterministic usage examples

### Basic usage

```oct
let rng0 = Random.RngSeed(42)

let r1 = Random.RandInt(rng0, 1, 6)
let die = r1.Value

let r2 = Random.RandFloat01(r1.Next)
let u = r2.Value
```

### Loop usage

```oct
var rng = Random.RngSeed(42)
var total = 0

for i in 1..100 {
    let roll = Random.RandInt(rng, 1, 6)
    rng = roll.Next
    total = total + roll.Value
}
```

### P14 noise usage

```oct
var rng = Random.RngSeed(123)
var sum = 0.0

for i in 1..n {
    let noise = Random.RandNormal(rng, 0.0, sigma)
    rng = noise.Next
    sum = sum + noise.Value
}
```

## 6) Crypto usage examples

```oct
let secureDie = Random.CryptoRandInt!(1, 6)
let secureU = Random.CryptoRandFloat01!()
let keyBytes = Random.CryptoRandBytes!(32)
```

Crypto APIs:

- do not accept `Rng`,
- are not reproducible,
- are backed by OS crypto entropy,
- surface errors via `!`.

## 7) Enum/match design for higher-level libraries

Higher-level Random libraries should model domain outcomes with enums and keep RNG transitions in records.

```oct
enum CoinSide {
    Heads
    Tails
}

record CoinFlipResult {
    Next: Rng
    Side: CoinSide
}
```

Guidance:

- **Enum** = domain outcome (`CoinSide`).
- **Record** = draw result + next state (`CoinFlipResult`).
- **switch** is often enough for tag-only enums.
- **match** is preferred when enum variants carry payloads.

`CoinSide` should not be represented as `Bool`, `Text`, or `Int` in high-level APIs.

## 8) Octomata/stochastic simulation design note

Random.Core should remain primitive and focused: deterministic state transitions and crypto draws.

Octomata or flow-style systems can carry `Rng` inside larger simulation state:

```text
board:
  Rng
  Index
  Sum

state Sample:
  let draw = Random.RandNormal(Rng, 0.0, sigma)
  Rng = draw.Next
  Sum = Sum + draw.Value
```

Conclusion:

- Random.Core provides primitive state transitions.
- Octomata structures stochastic processes.

Random.Core itself should not become an Octomata subsystem.

## 9) Validation rules

Deterministic API validation:

- `RandInt`
  - requires `min <= max`
  - range is inclusive `[min, max]`
- `RandFloat01`
  - range is `[0.0, 1.0)`
- `RandFloatRange`
  - requires `min <= max`
  - range is `[min, max)`
  - `min == max` returns `min`
- `RandBernoulli`
  - requires `0.0 <= p <= 1.0`
  - `p == 0` returns `false`
  - `p == 1` returns `true`
- `RandNormal`
  - requires `stddev >= 0`
  - `stddev == 0` returns `mean`

Invalid deterministic input is a non-fallible programmer error and should fail loudly.

Invalid crypto input returns a fallible error.

## 10) M1 implementation direction

Implementation target for later M1 runtime work:

- SplitMix64 for seeding.
- xoshiro256** for deterministic generation.
- Go `crypto/rand` for crypto APIs.

Public API stays type-clean and Oct-facing:

- `Int`, `Float`, `Bool`, `Bytes`, `Rng`, records, enums.

No public exposure of host-width internals such as `Uint64`, `Float64`, `byte[]`, or `Int32`.

## 11) M1b compiled record-result emission fix (2026-04-30)

### Root cause

Compiled Random builtin helpers in `internal/build/compiler.go` return and accept Go types named:

- `Random_Rng`
- `Random_RandIntResult`
- `Random_RandFloatResult`
- `Random_RandBoolResult`

These names match compiled record naming (`Package_Record`), but some compiled test paths invoked Random builtins without carrying Random package record declarations into `m.Records`. In those paths, helper functions were emitted but the record structs were not, causing unresolved Go symbols.

### Chosen fix path

M1b uses the smallest compatible fix: when Random helpers are needed, the compiler now emits fallback declarations for the four Random record-result structs if they were not already emitted from package records.

This preserves:

- the Oct public API shape (`Rng`, `Rand*Result` records),
- compiled helper signatures,
- deterministic algorithm behavior (SplitMix64 + xoshiro256**),
- and normal record emission when package records are present.

### How Random record-result types are emitted now

In compiled mode:

1. All `m.Records` are emitted first with the canonical `Package_Record` struct naming.
2. If Random builtins are used, compiler checks whether:
   - `Random.Rng`
   - `Random.RandIntResult`
   - `Random.RandFloatResult`
   - `Random.RandBoolResult`
   were already emitted.
3. Missing ones are emitted as fallback structs before helper functions.

This avoids duplicate type declarations while guaranteeing helper references resolve.

## 12) M8 deterministic PRNG state-advancement correction (2026-05-01)

The public API shape was correct earlier, but `.oct` library bodies in `Libraries/Random/Random.Core.oct` were still constant stubs.

Interpreter and compiled execution both route Random deterministic calls to runtime helpers, not to `.oct` stub bodies:

- Interpreter: `internal/interpret/interpret.go` Random builtin dispatch.
- Compiled: `internal/build/compiler.go` Random helper emission and call lowering.

Deterministic algorithm is real and shared across modes:

- SplitMix64 seeding into 4×uint64-equivalent state words.
- xoshiro256** stepping for each draw.

State advancement decisions:

- `RandFloat01`: always advances once.
- `RandInt`: advances at least once; may advance additional times for rejection sampling.
- `RandFloatRange`: advances once for `min != max`; for `min == max` currently does **not** advance.
- `RandBernoulli`: advances once for `0<p<1`; for `p==0`/`p==1` currently does **not** advance.
- `RandNormal`: advances twice for `stddev>0`; for `stddev==0` currently does **not** advance.

These degenerate-case non-advancement behaviors match current runtime implementation and tests.

Same-state reuse property is preserved by design:

- Calling a draw twice with the exact same input `Rng` returns the same value and same `Next` each time.

M8 test coverage explicitly verifies non-degeneracy and state progression so constant stubs cannot pass.
