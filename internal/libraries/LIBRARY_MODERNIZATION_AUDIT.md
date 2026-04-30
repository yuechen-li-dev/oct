# Oct Standard Libraries — Full Modernization Audit Against Current Language Reference

Date: 2026-04-30
Scope audited: `Libraries/` (all files), with reference alignment against `Language/reference` and existing notes in `internal/libraries/` + `internal/random/`.

## 1. Executive summary

- Total files audited under `Libraries/`: **225**
- Source/docs contracts reviewed (`*.oct`, `*.octest`, `*.octfail`, `*.md`): **218**
- Highest-priority modernization areas:
  1. **Frequency/unit typing drift** in RF/Wireless/Signal APIs still using dimensionless `Float` for Hz-like values.
  2. **Structured iteration style drift** (`while` used as manual counting loops) in core numerical libraries.
  3. **Precondition style inconsistency** (mixed `error(...)` argument checks vs `Require(...)` invariant checks).
  4. **Legacy type-policy docs** that conflict with current reference (e.g., nested array parser limitations).

No broad code changes were applied in this milestone (audit-only).

## 2. Current Language/reference snapshot (audit baseline)

The current reference confirms these modern surfaces as canonical:

- `Pow(base, exponent)` for exponentiation (no `^` operator).
- `Require(condition, message)` for non-recoverable preconditions/invariants.
- Unit system supports **signed exponents** (e.g., `s^-1`, `m*s^-2`) and **`Hz` alias** compatible with `s^-1`.
- Typed empty arrays are valid in explicit context (e.g., `var xs: Int[] = []`).
- Random standard-library design uses result records (`RandIntResult`, `RandFloatResult`, `RandBoolResult`) with explicit RNG threading.
- `let` default immutability; `var` only when reassignment is needed.
- `for i in start..end [step k]` preferred for structured range iteration; `while` preferred for condition-driven loops.
- Enums + `switch`/`match` are first-class and should replace magic domain strings/ints where appropriate.
- Fallible signatures use `-> T ! Error`; fallible handling via `?`, `!`, or fallible `match`.

## 3. Findings by category

### 3.1 Unit/frequency modernization

| File path | Symbol/API | Current pattern | Recommended change | Confidence | Risk | Notes |
|---|---|---|---|---|---|---|
| `Libraries/Wireless/Wireless.Core.oct` | `Band.CenterFreqHz`, `ChannelWidthHz`, `ToneSpacingHz`, `EstimateThroughput`, `FreeSpacePathLossDb`, `ThermalNoiseFloorDbm` | Frequency/bandwidth fields and params are plain `Float` with Hz naming/comments | Move Hz-valued fields/params to `Float<Hz>`; keep spectral efficiency and dB quantities dimensionless | High | High | Public API surface; needs staged migration to avoid ecosystem breaks |
| `Libraries/RF/RF.Doppler.oct` | `DopplerShiftHz*`, `MaxDopplerShiftHz`, coherence bandwidth/time helpers | Hz-valued inputs/outputs are plain `Float` | Adopt `Float<Hz>` (or `Float<s^-1>`) for Doppler/coherence bandwidth; preserve dimensionless coefficients | High | Medium | Strong semantic fit with reference units |
| `Libraries/RF/RF.PathLoss.oct` | `frequencyHz` params | Frequency carried as plain `Float` | Type as `Float<Hz>` | High | Medium | Internally used with `distance` units already present |
| `Libraries/RF/RF.Noise.oct` | `bandwidthHz` params/arrays | Bandwidth as plain `Float` with Hz comments | Use `Float<Hz>` for bandwidth arguments/series | High | Medium | Should improve dimensional correctness of kTB formulas |
| `Libraries/RF/RF.SParameters.oct` | `Frequencies: Float[]` and interpolation frequency args | Frequency axis plain `Float` | Candidate for `Float<Hz>[]` + `Float<Hz>` interpolation arg | Medium | Medium | Requires checking interaction with generic interpolation helpers |
| `Libraries/Signal/Signal.Core.oct` | `BinIndexToFrequency` docs and return | Returns normalized cycles/sample as `Float` while docs mention multiply by sample rate in Hz | Keep function dimensionless, but clarify naming/doc (`NormalizedFrequencyFromBin`), optionally add overload helper taking `sampleRateHz: Float<Hz>` | High | Low | This one should remain dimensionless by design |
| `Libraries/Mathematics/README.md` | FFT note | Explicitly says no dimension-aware frequency-axis metadata introduced | Keep as-is for math package scope; cross-link to Signal/RF/Wireless for typed-frequency design | Medium | Low | Not a bug; expectation-setting text |

### 3.2 Unit exponent modernization (signed exponent / inverse units)

| File path | Symbol/API | Current pattern | Recommended change | Confidence | Risk | Notes |
|---|---|---|---|---|---|---|
| `Libraries/Wireless/Wireless.Core.oct` | `symbolDurS = 1.0 / toneSpacingHz * 1.0s` | Inverse-frequency encoded via arithmetic instead of typed frequency | If `toneSpacingHz` becomes `Float<Hz>`, this naturally type-checks as seconds; consider explicit `Float<s^-1>` where “rate” not specifically Hz | High | Medium | Currently works but misses type signal |
| `Libraries/RF/*` frequency APIs | Hz semantics mostly in names/comments | Reliance on naming instead of unit type | Prefer `Float<Hz>`/`Float<s^-1>` signatures | High | Medium | Primary modernization value is compile-time dimensional guarantees |

### 3.3 Pow modernization

| File path | Symbol/API | Current pattern | Recommended change | Confidence | Risk | Notes |
|---|---|---|---|---|---|---|
| `Libraries/Mechanics/Mechanics.Endurance.oct` | `SurfaceFactorGroundMachined` | `a * Exp(b * Ln(sutMpa))` | Replace with `a * Pow(sutMpa, b)` | High | Low | Straight power-law rewrite |
| `Libraries/Mechanics/Mechanics.Endurance.oct` | `SizeFactorBendingRound` | `Exp(exponent * Ln(ratio))` | Replace with `Pow(ratio, exponent)` | High | Low | Straight power-law rewrite |
| `Libraries/Distributions/Distributions.Core.oct` | Gaussian/Exponential/CDF terms | Uses `Exp(...)` directly, no `Ln` workaround | Keep | High | Low | Intentional exponential forms, not power workarounds |
| `Libraries/Random/Random.Distributions.oct` | `Exponential` | `-Ln(1-u)/lambda` form | Keep | High | Low | Correct inverse-CDF log-space math |

### 3.4 Loop modernization (`while` vs `for`)

| File path | Symbol/API | Current pattern | Recommended change | Confidence | Risk | Notes |
|---|---|---|---|---|---|---|
| `Libraries/LinearAlgebra/LinearAlgebra.Core.oct` | many kernels (`MatMul`, `Transpose`, `Identity`, `LU*`, helpers) | Numerous counting/index loops written as `while i < ...` | Convert simple bounded counters to `for` where no state-machine behavior | High | Medium | Large diff; stage carefully to protect numeric behavior |
| `Libraries/Mathematics/Mathematics.Transforms.oct` | FFT staging loops | Nested index-driven loops with clear range bounds | Convert eligible simple counters to `for`; keep bit-reduction/termination loops as `while` when clearer | Medium | Medium | Mixed category: some structural, some condition-driven |
| `Libraries/Statistics/Statistics.Core.oct` | sort/selection loops | Mixed counting and insertion-shift loops | Use `for` for straight counters; keep mutation-driven inner shifting loops as `while` | High | Low | Clear readability win |
| `Libraries/Interpolation/Interpolation.Core.oct` | `while i >= 0` back-substitution | Decreasing-index linear pass | Probably keep as `while` (condition-driven decrement) unless style guide explicitly favors descending `for` | Medium | Low | Already documented as legitimate `while` in package README |
| `Libraries/Optimization/Optimization.Core.oct`, `Libraries/Numerics/Numerics.Roots.oct`, `Libraries/LinearAlgebra/LinearAlgebra.Eigen.oct` | convergence loops | `while iterations < maxIterations and not converged` | Keep as `while` | High | Low | Canonical condition-driven loop use |

### 3.5 Mutability modernization (`let` vs `var`)

| File path | Symbol/API | Current pattern | Recommended change | Confidence | Risk | Notes |
|---|---|---|---|---|---|---|
| `Libraries/Random/Random.Dice.oct` | `CryptoRollDice` | `let values: Int[] = []` then passed recursively (append creates new arrays) | Pattern is valid immutable accumulator handoff; keep | High | Low | Not a mutability bug |
| `Libraries/Random/Random.CoinToss.oct` | `CryptoFlipCoins` | `let flips: CoinSide[] = []` recursive accumulation | Keep | High | Low | Same as above |
| `Libraries/*` numeric kernels | widespread `var` for counters and accumulating arrays | Most are required due to reassignment | Targeted audit during implementation milestone to downgrade only provably immutable bindings | Medium | Low | Mechanical “var→let” should be avoided without proof |

### 3.6 Require modernization

| File path | Symbol/API | Current pattern | Recommended change | Confidence | Risk | Notes |
|---|---|---|---|---|---|---|
| `Libraries/Random/Random.Distributions.oct` | `Jitter`, `Spike` | Uses `Require(...)` preconditions | Keep (reference-aligned) | High | Low | Good exemplar |
| `Libraries/Signal/Signal.Core.oct`, `Libraries/RF/*`, `Libraries/Mathematics/*`, others | Many argument checks return `error(...)` instead of `Require(...)` | Design decision: keep fallible APIs for recoverable caller errors vs migrate strict programmer invariants to `Require` in selected infallible helpers | Medium | Medium | Needs policy pass; avoid inconsistent semantics |
| `Libraries/*` | `Assert.*` in production `.oct` | None found | No action | High | Low | `Assert.*` appears only in test guidance docs |

### 3.7 Typed empty array modernization

| File path | Symbol/API | Current pattern | Recommended change | Confidence | Risk | Notes |
|---|---|---|---|---|---|---|
| `Libraries/Random/Random.Dice.oct`, `Libraries/Random/Random.CoinToss.oct` | `var/let xs: T[] = []` | Already using typed empty arrays | Keep | High | Low | Reference-compliant |
| `Libraries/*` | workaround-first-element initialization patterns like `var out = [first]; for ... Append(...)` | In many places this is used to avoid empty-seed branches | Consider refactor to typed empty seed + uniform loop where it simplifies logic | Medium | Low | Optional readability modernization, not correctness issue |

### 3.8 Random API modernization

| File path | Symbol/API | Current pattern | Recommended change | Confidence | Risk | Notes |
|---|---|---|---|---|---|---|
| `Libraries/Random/Random.Core.oct` and consumers in `Random.*` | Result-record RNG threading (`Next`, `Value`) | Keep | High | Low | Already modern reference design |
| `Libraries/RF/*`, `Libraries/Wireless/*` | deterministic APIs; no hidden RNG | Keep deterministic posture; if stochastic helpers added later, route through `Random.*` result-record patterns | High | Low | No modernization bug currently |

### 3.9 Enum / switch / match modernization

| File path | Symbol/API | Current pattern | Recommended change | Confidence | Risk | Notes |
|---|---|---|---|---|---|---|
| `Libraries/IO/IO.Json.oct` | `JsonRawGraphNode.Kind: String` with values like `"null"`, `"array"`, `"object"` | Magic string domain values | Candidate enum (`JsonNodeKind`) if wrapper payload compatibility allows | Medium | Medium | Potential compatibility impact with wrapper/raw graph surfaces |
| `Libraries/UI/UI.Layout.oct` | `Box.Kind: String` with `"absolute"`/`"anchored"` | Magic string mode tags | Replace with enum for internal safety (`BoxKind`) | High | Low | Internal UI model; likely low migration risk |
| `Libraries/UI/UI.Examples.oct` / `UI.AppModel.oct` | event route strings | Probably acceptable at host boundary, but consider enum core + string adapter | Medium | Medium | Boundary design decision |

### 3.10 Current syntax/reference compliance gaps

| File path | Symbol/API | Current pattern | Recommended change | Confidence | Risk | Notes |
|---|---|---|---|---|---|---|
| `Libraries/LinearAlgebra/README.md` | “nested array types (`Float[][]`) are currently not supported by the Oct parser” | Stale documentation versus current reference (`T[][]` supported) | Update docs to current parser/reference reality | High | Low | Explicit inconsistency with `Language/reference/07-arrays.md` |
| `Libraries/Wireless/Wireless.Core.oct` comment | “Frequency values are plain Float, unit is Hz” | Legacy workaround-era guidance | Update after typed-frequency decision and migration | High | Low | Should not teach old style once unit migration lands |
| `Libraries/*` | old fallible syntax/package forms | No stale syntax found in audited files | No action | High | Low | Current `! Error`, `package`, `import` forms look aligned |

## 4. Proposed modernization milestones

1. **Library Modernization M1 — Documentation and low-risk style alignment**
   - Fix stale docs that conflict with reference (nested arrays note, legacy frequency commentary).
   - Add explicit style notes where loops intentionally remain `while`.

2. **Library Modernization M2 — Loop + mutability readability pass (no API changes)**
   - Convert clear counting `while` loops to `for` in `LinearAlgebra`, `Mathematics.Transforms`, `Statistics`.
   - Opportunistically demote `var` to `let` where no reassignment exists.

3. **Library Modernization M3 — Pow cleanup and precondition policy pass**
   - Replace remaining `Exp(Ln(x)*y)` power-law idioms in Mechanics.
   - Establish policy matrix for `Require` vs `error(...)`; apply consistently.

4. **Library Modernization M4 — RF/Wireless/Signal unit-typed frequency redesign**
   - Introduce `Float<Hz>`/`Float<s^-1>` in RF and Wireless public APIs.
   - Provide migration shims/overloads if compatibility policy requires.
   - Update docs and examples to typed frequency idioms.

5. **Library Modernization M5 — Domain enums for stringly-typed modes/kinds**
   - Introduce enums where internal domain tags are currently strings (`UI.Layout`, possibly `IO.Json` with compatibility plan).

## 5. Deferred / unclear items requiring design review

1. **`Float<Hz>` vs `Float<s^-1>` naming policy** for public APIs:
   - Prefer `Float<Hz>` for ordinary frequency/sample-rate concepts.
   - Use `Float<s^-1>` for generic inverse-time rates not conceptually “frequency”.
   - Needs explicit project-wide rule to avoid mixed style.

2. **`Require` vs fallible `error(...)` in library APIs**:
   - Some packages intentionally expose fallible validation for caller-controlled inputs.
   - Need agreed guideline for when invalid input is “programmer invariant” vs recoverable data error.

3. **Enum migration at wrapper boundaries (`IO.Json`)**:
   - Converting `Kind: String` to enum improves safety but may conflict with raw graph interchange/compatibility layers.

4. **Structured-loop modernization scope in numerics-heavy kernels**:
   - Large mechanical diffs can obscure algorithmic changes; migration should be phased with strict regression checks.

---

## Evidence summary (search-led audit)

Primary repo scans used:

- `rg -n "while" Libraries -g '*.oct' -g '*.md'`
- `rg -n "Exp\(|Ln\(" Libraries -g '*.oct'`
- `rg -n "freq|frequency|sampleRate|cutoff|bandwidth|Hz|omega|rate|symbolRate|carrier|perSecond|throughput|latency" Libraries -i -g '*.oct' -g '*.md'`
- `rg -n "Assert\." Libraries -g '*.oct' -g '*.md'`
- `rg -n "\[\]" Libraries -g '*.oct'`
- `rg -n "Random\.|Rand|noise|jitter|spike" Libraries -i -g '*.oct' -g '*.md'`
- `rg -n "mode|status|kind|type" Libraries -i -g '*.oct' -g '*.md'`

Supplementary reference scans used against `Language/reference/language/*.md` for feature baseline confirmation.


## 6. M1 documentation + historical-slop cleanup (implemented 2026-04-30)

### Files updated

- `Libraries/LinearAlgebra/README.md`
- `Libraries/Wireless/Wireless.Core.oct`
- `Libraries/RF/RF.SParameters.oct`

### Stale docs/comments fixed

- Corrected stale nested-array parser guidance in `LinearAlgebra` docs: `Float[][]` is supported by current Oct syntax; package-level flat row-major storage remains an API/design choice, not a parser limitation.
- Replaced legacy Wireless top-of-file comment that implied dimensionless `Float` frequency is the preferred modern style.
- Added explicit compatibility wording plus TODO modernization note in Wireless for future `Float<Hz>` migration.
- Added explicit compatibility wording plus TODO modernization note in RF S-parameter frequency docs for future `Float<Hz>` migration.

### API changes intentionally deferred in M1

No public API signatures were changed in this pass. In particular, the following remain deferred:

- `Float` -> `Float<Hz>` / `Float<s^-1>` signature migration across RF/Wireless
- loop-structure modernization (`while` to `for`)
- `Require` vs `error(...)` policy normalization
- enum/stringly-domain migrations
- broader numerical refactors

### Follow-up milestones still required

- **M2**: loop + mutability readability pass (no API changes).
- **M3**: precondition-policy consistency pass (`Require` vs `error(...)`) and any remaining low-risk math-style cleanup.
- **M4**: coordinated RF/Wireless typed-frequency API migration (`Float<Hz>`/`Float<s^-1>`) with compatibility strategy.
- **M5**: enum migration for stringly-typed domain fields where appropriate.
