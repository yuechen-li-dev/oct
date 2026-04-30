Random tests execute from `Libraries/Random/*.octest` and `*.octfail`.

- Keep assertion helpers (`Assert.*`) in `.octest` files only.
- Production `Libraries/Random/*.oct` files must not depend on `Assert`.
- Nested `Libraries/Random/tests/*.octest` files are documentation-only for now because `oct test Libraries/Random` resolves package symbols per directory.

- Random package `.octest` files run in same-package scope; use unqualified symbols (`RollDice`) instead of `Random.RollDice` inside `package Random` tests.
