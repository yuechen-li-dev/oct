Random tests execute from `Libraries/Random/*.octest` and `*.octfail`.

- Keep assertion helpers (`Assert.*`) in `.octest` files only.
- Production `Libraries/Random/*.oct` files use `Assert.True` only for runtime preconditions, as `Language/reference/language/09-builtins.md` allows. Test assertions stay in `.octest` files.
- Nested `Libraries/Random/tests/*.octest` files are documentation-only for now because `oct test Libraries/Random` resolves package symbols per directory.

- Random package `.octest` files run in same-package scope; use unqualified symbols (`RollDice`) instead of `Random.RollDice` inside `package Random` tests.

Streams:

- `Random.Stream.octest` holds the expected values and invariants for the native stream builtins, and `Random.Stream.invalid.*.octfail` holds their compile-time contracts. `Random.Stream.invalid.RemovedGeneratorState.octfail` pins that the Random 0.1.0 names are undefined. The `.octfail` fixtures are standalone `package Random` programs: they were written when an `.octfail` could not import a repository library, and they check the builtins from inside the package.
- Use from another package is checked in `Libraries/RandomUsage`.
- A violated runtime precondition (negative index, reversed bounds, negative standard deviation) is a non-recoverable runtime error, which neither `.octest` nor `.octfail` can assert. Those cases live in `testdata/random_stream_preconditions` and are run by `cmd/oct/random_stream_preconditions_test.go` in the Go `integration` lane.
- The `Entropy` builtins follow the same rules. Their contracts are in `Language/Builtins/Entropy`, `Libraries/Entropy` and the same precondition fixture.
- Inside `package Random` the stream builtins accept both spellings, `Unit(...)` and `Random.Unit(...)`.
