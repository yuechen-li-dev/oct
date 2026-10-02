# Random v2 — M1: `internal/octrandom` Go core

Date: 2026-10-01
Ladder: `internal/random/RANDOM_V2_LADDER.md`

## Verdict

**SUCCESS.** The Go core for Random v2 exists as `internal/octrandom`, matches
the Random123 published known-answer vectors for Philox4x32-10, and implements
ladder sections 3.2 and 3.3. Nothing imports it yet; no Oct-visible behavior
changed.

## What was added

| File | Contents |
|---|---|
| `doc.go` | Package contract and the determinism tiers (ladder I5) |
| `philox.go` | `Philox4x32` (10 rounds) |
| `fnv.go` | `FNV1a64`, the `Fork` label hash |
| `stream.go` | `Key`, `Seeded`, `Fork`, `Child`, counter layout with domain word |
| `draw.go` | `Unit`, `Between`, `IntBetween`, `Normal`, precondition errors |
| `*_test.go` | 32 tests |

The package depends only on the standard library (`math`, `math/bits`,
`errors`).

## Go API

```go
type Key uint64

func Seeded(seed int64) Key
func Fork(k Key, label string) Key
func Child(k Key, i int64) (Key, error)

func Unit(k Key, i int64) (float64, error)
func Between(k Key, i int64, lo, hi float64) (float64, error)
func IntBetween(k Key, i int64, lo, hi int64) (int64, error)
func Normal(k Key, i int64, mean, stddev float64) (float64, error)
```

Precondition failures are returned as sentinel errors (`ErrNegativeIndex`,
`ErrFloatRange`, `ErrFloatSpan`, `ErrIntRange`, `ErrNormalStddev`,
`ErrNormalNotReal`). M3 maps them to a non-recoverable runtime error in the
interpreter and a panic in generated programs, using the same text in both.

## Evidence

- **External correctness.** `TestPhilox4x32KnownAnswers` checks the three
  `philox4x32 10` vectors copied verbatim from Random123 `tests/kat_vectors`
  at commit `9545ff6413f258be2f04c1d319d99aaef7521150`. `FNV1a64` is checked
  against the published FNV-1a 64 reference values and against `hash/fnv`.
- **Specification conformance.** The layout tests recompute `Child`, `Fork`
  and draw words directly from the counter table in ladder 3.2, without the
  package's helpers. `IntBetween` is checked against an independently written
  reference over eleven ranges, including the full `Int` range (span wraps to
  0), span `2^63 + 1` (worst rejection rate) and span `2^64 - 1`.
- **Edge cases.** `Unit` word bounds; `Between` on a one-ulp interval, where
  about half of all draws hit the upper-bound clamp; `Normal` at `u1 = 1` and
  at the largest producible magnitude (about 8.57 sigma); every precondition.
- **Statistical sanity.** Mean of 200,000 `Unit` draws; mean, variance, third
  moment and 1/2-sigma coverage of 200,000 `Normal` draws; chi-square on
  120,000 d6 rolls; correlation between two forks; lag-1 serial correlation.
- **Invariant I3.** `TestParameterIsolationAcrossForks` reproduces the
  measurement-lab shape: zeroing or changing jitter leaves the spike stream
  untouched.
- **Mutation check.** 18 deliberate faults were injected one at a time
  (off-by-one in the rejection bound, rejection removed, second word skipped,
  clamp removed or weakened, `u1` without the `+1`, wrong shift, swapped
  domain tags, swapped index or key words, weakened index check, swapped Weyl
  constants, 9 rounds, swapped multipliers, wrong FNV step, `sin` for `cos`,
  full-range path removed). Every one was caught by at least one test.

## Validation run

```
go vet ./internal/octrandom
go test ./internal/octrandom -count=1        # ok, 32 tests
go test ./internal/octrandom -count=1 -race  # ok
```

Run with Go 1.25.0 and Go 1.24.7 on linux/amd64, in an isolated module holding
only this package. `go vet` also passes for `GOOS=windows GOARCH=amd64` and
`GOOS=linux GOARCH=arm64`.

**Not yet run:** the tests have not been executed inside the oct module on the
development machine (Windows), nor on arm64. The package has no dependencies
outside the standard library, so `go test ./internal/octrandom` from the repo
root is the confirming command.

## Notes carried forward

- `Between` rejects NaN and infinite bounds as well as an infinite span. The
  ladder text in 3.3 was tightened to say so.
- `IntBetween` has no iteration cap. Each word is accepted with probability
  above 1/2, so the chance of needing more than `n` words is below `2^-n`.
- M3 goldens can be generated from this package; correctness rests on the
  known-answer vectors above, not on those goldens.
