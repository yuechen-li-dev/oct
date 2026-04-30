Random coin-toss tests currently execute from `Libraries/Random/*.octest` and `*.octfail`.

The `oct test Libraries/Random` runner currently resolves package tests per directory, so nested `tests/` Oct files cannot import the parent `Random` package symbols yet.
