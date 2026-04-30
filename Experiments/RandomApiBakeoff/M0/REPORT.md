# Random API Shape Bakeoff (M0)

## 1) Problem statement
Random M1 needs a public deterministic RNG API that keeps correctness obvious in common usage (CoinToss/Dice) and in state-heavy workflows (P14 noise/filter simulation).

## 2) Candidate definitions
- **A. Tuple return**: `rng, x = RandInt(rng, lo, hi)`.
- **B. Result record**: `r = RandInt(...); rng = r.Rng; x = r.Value`.
- **C. Mutable handle**: `x = RandInt(rng, lo, hi)` with hidden mutation.
- **D. Split state/value**: `rng = RandStep(rng); x = RandIntFromState(rng, lo, hi)`.
- **E. Scoped block**: `with rng = RngSeed(42) { ... }`.

## 3) Evaluation criteria
Scores are computed in `random_api_bakeoff.oct` with explicit formulas over:
- determinism clarity
- state visibility
- syntax friction
- cognitive load
- teaching cost
- error-proneness
- alignment with Oct philosophy
- implementation complexity
- language feature risk
- fit for P14 noise/filter simulations
- fit for CoinToss/Dice APIs

Additional per-scenario scores are computed for:
1. simple draw
2. sequential draws
3. simulation loop
4. conditional randomness
5. composition
6. misuse/error case

## 4) Results

### Axis score table (0..10 per axis, plus totals)

| API | Determ. | State vis. | Friction | Cog. | Teach | Error | Align | Impl | Lang risk | P14 fit | Coin/Dice fit | Axis total |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| A.TupleReturn | 8 | 9 | 8 | 7 | 7 | 7 | 6 | 6 | 5 | 8 | 7 | 78 |
| B.ResultRecord | 8 | 9 | 2 | 0 | 4 | 6 | 7 | 10 | 10 | 5 | 4 | 65 |
| C.MutableHandle | 0 | 0 | 8 | 7 | 8 | 0 | 0 | 10 | 10 | 2 | 5 | 50 |
| D.SplitStateValue | 8 | 9 | 5 | 3 | 6 | 4 | 7 | 10 | 10 | 6 | 5 | 73 |
| E.ScopedBlock | 0 | 0 | 7 | 6 | 6 | 1 | 0 | 5 | 5 | 2 | 4 | 36 |

### Scenario table (0..10 per scenario, plus totals)

| API | Simple | Sequential | Simulation | Conditional | Composition | Error case | Scenario total |
|---|---:|---:|---:|---:|---:|---:|---:|
| A.TupleReturn | 10 | 9 | 8 | 9 | 8 | 7 | 51 |
| B.ResultRecord | 9 | 3 | 0 | 3 | 0 | 6 | 21 |
| C.MutableHandle | 5 | 3 | 2 | 3 | 2 | 0 | 15 |
| D.SplitStateValue | 9 | 5 | 3 | 5 | 3 | 4 | 29 |
| E.ScopedBlock | 4 | 2 | 1 | 2 | 1 | 1 | 11 |

### Final ranking (Axis total + Scenario total)
1. **A.TupleReturn** = 129
2. **D.SplitStateValue** = 102
3. **B.ResultRecord** = 86
4. **C.MutableHandle** = 65
5. **E.ScopedBlock** = 47

## 5) Analysis
- **Tuple return (A)** dominates on practical scenarios while preserving explicit state flow.
- **Split state/value (D)** is semantically clean but pays a steady verbosity/cognitive tax.
- **Result record (B)** is safe and implementable today, but assignment overhead materially hurts sequential/composed usage.
- **Mutable handle (C)** is syntactically convenient but poor on determinism clarity and misuse resilience.
- **Scoped block (E)** has high language/implementation risk with weak clarity benefits.

Unexpected finding: record shape is competitive on architecture risk but loses strongly on everyday friction, especially repeated draws and composition pipelines.

## 6) Recommendation
For **Random M1**, implement **A.TupleReturn** as the target API shape (best total score and best scenario behavior), with **D.SplitStateValue** as runner-up fallback if tuple introduction is delayed.

## 7) Consequences
If choosing A:
- Runtime/library surface should return `(nextRng, value)` from draw operations.
- Main risk is language feature dependency on tuple return/destructuring.
- Future extensions (coin tosses, dice pools, simulation operators) compose naturally by threading RNG first.

## Inconsistency / documentation gaps surfaced
- This experiment models tuple-based APIs, but tuple return/destructuring support is not clearly established in `Language/reference` pages used here. That is a doc/spec gap to resolve before locking M1.
