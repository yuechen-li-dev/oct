# Mx100c Report — Normalize Explicit Type Argument Syntax to `<T>`

## 1) Surfaces that previously used `[T]`

The repository previously used square-bracket explicit type arguments in call sites for:

- `LoadOctagon[T](path)`
- `Matrix.zeros[T](rows, cols)`
- `Matrix.identity[T](n)`

These appeared in parser/typechecker/compiler tests, language contracts under `Language/`, and language/tooling documentation.

## 2) What was normalized to `<T>`

All explicit call-site type-argument examples and tests for the affected builtins were normalized to angle brackets:

- `LoadOctagon<T>(path)`
- `Matrix.zeros<T>(rows, cols)`
- `Matrix.identity<T>(n)`

Parser call-site type-argument parsing was changed to consume `<...>` instead of `[...]`.

## 3) Compatibility behavior decision

**Option A (strict normalization) was chosen.**

Legacy bracket call syntax `[...]` is no longer accepted for call-site type arguments. The parser now emits a targeted error diagnostic:

- `type arguments must use '<...>'; legacy '[...]' syntax is no longer supported`

## 4) Docs/Examples/tests updated

Updated locations include:

- Language references for vectors/matrices and Octagon tooling
- Language Octagon load contracts and matrix-related contracts
- Mechanics examples that use `Matrix.identity`
- Parser/typechecker/compiler and CLI integration tests exercising `LoadOctagon` / `Matrix` call-site type arguments
- Existing milestone docs/reports where old call syntax appeared

## 5) Parser ambiguity work

`<...>` introduces potential overlap with comparison operators. To avoid ambiguity at call sites, the parser now performs a lookahead check that only treats `<...>` as type arguments when:

1. `<` starts a valid type reference,
2. it is closed by `>`, and
3. it is immediately followed by `(`.

Otherwise, `<` continues to parse as a normal comparison operator.
