# Entropy

`Entropy` draws from the operating system's random source. It is the one
place nondeterminism enters an Oct program: [`Random`](../Random/README.md)
is reproducible from a seed, and `Entropy` is where an unrecorded seed comes
from.

```oct
import Random

fn NoiseForThisRun() -> Random.Stream ! Error {
    let seed = Entropy.Seed()?
    Print(seed)
    return Random.Seeded(seed)
}
```

Record the seed. Every draw after `Random.Seeded(seed)` is a function of it,
so a logged seed replays the run.

## API

- `Entropy.Seed() -> Int ! Error`: any `Int`, for `Random.Seeded`.
- `Entropy.IntBetween(lo, hi) -> Int ! Error`: uniform on the closed range `[lo, hi]`, without modulo bias.
- `Entropy.Unit() -> Float ! Error`: uniform on `[0, 1)`, a multiple of `2^-53`.
- `Entropy.Bytes(count) -> Bytes ! Error`: `count` random bytes.

`Entropy` is a compiler-owned namespace, like `Artifact`. The four functions
are builtins implemented in Go for both execution lanes, and they need no
`import`. `import Entropy` is allowed.

## Rules

- Every call is fallible, and the only `Error` is a failure of the operating
  system's random source.
- A violated precondition (`lo > hi`, `count < 0`) is a non-recoverable
  runtime error, as in `Random`. It is not an `Error` and `match` does not
  catch it.
- Artifact evaluation and capability discovery reject every `Entropy` call
  before it reads anything.
- There are no coin or dice helpers. Seed a `Random` stream and use
  `Random.FlipCoin` or `Random.RollDie`, or call `Entropy.IntBetween(1, sides)`.

## Tests

- `Language/Builtins/Entropy/valid`: types, ranges and fallibility from
  another package, without an import.
- `Language/Builtins/Entropy/invalid`: compile-time contracts.
- `Entropy.Core.octest`: the unqualified spelling inside `package Entropy`.
- `Libraries/RandomUsage`: seeding a `Random` stream, with `import Entropy`.
- `testdata/random_stream_preconditions`: runtime preconditions in both
  lanes, run by the Go `integration` lane.
- `testdata/entropy_source_failure`: a failed read as an ordinary `Error`,
  in both lanes, run by the Go tests `cmd/oct/entropy_source_failure*_test.go`
  against a source that always fails.
- `Language/Tooling/Artifacts/invalid/ambient_entropy_*.octest` and
  `Language/Tooling/ConceptCapabilitiesM2/invalid/entropy_discovery.octest`:
  rejection during artifact evaluation and capability discovery.

Specification: `internal/random/RANDOM_V2_LADDER.md`, section 3.5.
