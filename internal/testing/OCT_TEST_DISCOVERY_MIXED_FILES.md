# OCT test discovery: mixed `.octest` and `.octfail`

## Root cause

`internal/tester.Execute` suppressed **all** `project.LoadForTest` errors whenever at least one `.octfail` file was present.

This meant parse/typecheck errors in `.octest` files could be hidden, and only `.octfail` execution would appear in output.

## Fix

Adjusted the load-error guard in `internal/tester/tester.go` so load errors are only ignored for the existing `unknown package` case.

Now:

- mixed directories run discovered `.octest` facts/theories and `.octfail` fixtures together;
- `.octest` parse/typecheck failures are surfaced as real failures even when `.octfail` files exist.

## Expected behavior

For a directory containing both file kinds:

- `.octest` discovery/execution contributes to pass/fail counts,
- `.octfail` discovery/execution contributes to pass/fail counts,
- summary combines both result sets.

## Validation

- Added `cmd/oct/m24j_mixed_discovery_test.go` coverage for mixed execution and non-masking of `.octest` parse errors.
- Verified CLI behavior with `go run ./cmd/oct test Libraries/Random` after runner change.
