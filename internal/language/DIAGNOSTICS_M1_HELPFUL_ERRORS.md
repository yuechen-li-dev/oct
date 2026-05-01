# Diagnostics M1 — Helpful Error Message Pass

## Audited areas
- `internal/typecheck/typecheck.go` focused on statement/type errors with high user confusion.
- Existing Language invalid tests under:
  - `Language/ControlFlow/OctomataBlackboardM6/invalid`
  - `Language/Types/Arrays/invalid`
  - `Language/Types/Tuples/invalid`
  - `Language/Errors/Fallible/invalid`
  - `Language/Functions/Calls/invalid`

## Improved diagnostics
- Immutable assignment now includes binding name and concrete remedies (`var` vs new `let`).
- Fallible expression handling guidance expanded to include `?`, `!`, and `match` suggestions.
- Index assignment diagnostics now include concrete syntax examples and type-specific remediation.
- Non-`board` field assignment inside flow states now explains allowed `board` updates and immutable-record `with` workflow.
- `when` action-block diagnostics now explain allowed action classes and deterministic control-transfer requirement.
- Empty array diagnostics now explain why `[]` is ambiguous and show explicit annotation fix.
- Tuple-type rejection diagnostics now align with public-language decision and direct users to records.
- Builtin arity grammar corrected where touched (`expects 1 argument`).

## Before/after samples
- Before: `only board field assignment is supported`
  After: `inside flow state bodies, field assignment is only supported on board ... ordinary records are immutable and must use with`.
- Before: `empty array literals require an explicit array type`
  After: `empty array literal [] requires an expected array type; write var values: Int[] = [] ...`.
- Before: `fallible expression must be handled explicitly`
  After: `fallible expression must be handled explicitly; use ?, !, or match ...`.

## Intentionally unchanged
- Parser-level terse "expected ..." diagnostics that are position/syntax constrained were not broadly rewritten in M1.
- Unsupported-statement and many lower-level operator diagnostics were left unchanged to avoid broad churn.
- Tuple internals were not removed in this pass; only user-facing tuple rejection messaging was aligned.

## Tests updated/added
- Updated:
  - `Language/Types/Arrays/invalid/empty_array_without_context_m1.octfail`
  - `Language/Types/Arrays/invalid/empty_array_without_context_var_m1.octfail`
  - `Language/ControlFlow/OctomataBlackboardM6/invalid/non_board_field_assignment_rejected.octfail`
  - `Language/Types/Tuples/invalid/tuple_return_syntax_rejected.octfail`
- Added:
  - `Language/Functions/Calls/invalid/immutable_assignment_guidance.octfail`
  - `Language/Errors/Fallible/invalid/fallible_assignment_guidance.octfail`
  - `Language/Functions/Calls/invalid/standalone_expression_guidance.octfail`

## Validation results
- Ran `go test ./...`.
- Ran `go run ./cmd/oct test Language`.
- Ran `go run ./cmd/oct test Libraries`.
