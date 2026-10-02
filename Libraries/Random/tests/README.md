Random tests execute from `Libraries/Random/*.octest` and `*.octfail`.

- Keep assertion helpers (`Assert.*`) in `.octest` files only.
- Production `Libraries/Random/*.oct` files must not depend on `Assert`.
- Nested `Libraries/Random/tests/*.octest` files are documentation-only for now because `oct test Libraries/Random` resolves package symbols per directory.

- Random package `.octest` files run in same-package scope; use unqualified symbols (`RollDice`) instead of `Random.RollDice` inside `package Random` tests.

Random v2 (streams):

- `Random.Stream.octest` holds the expected values and invariants for the native stream builtins, and `Random.Stream.invalid.*.octfail` holds their compile-time contracts. The `.octfail` fixtures are standalone `package Random` programs because an `.octfail` cannot import a repository library.
- Use from another package is checked in `Libraries/RandomUsage`.
- A violated runtime precondition (negative index, reversed bounds, negative standard deviation) is a non-recoverable runtime error, which neither `.octest` nor `.octfail` can assert. Those cases live in `testdata/random_stream_preconditions` and are run by `cmd/oct/random_stream_preconditions_test.go` in the Go `integration` lane.
- Inside `package Random` the v2 builtins accept both spellings, `Unit(...)` and `Random.Unit(...)`.
