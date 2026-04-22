# Mx102 Report — Arrow Unification (`=>` and `->`)

## Scope

Mx102 unifies source-level arrow spellings so `=>` and `->` are accepted as the same Arrow token family across all existing arrow-bearing syntax.

## 1) Arrow usage audit (before change)

Audited parser acceptance sites in `internal/parse/parse.go` and lexer tokenization in `internal/lex/lex.go`.

### `->` accepted before Mx102

- function declarations (`fn ... -> ReturnType`)
- flow declarations (`flow ... -> ReturnType`)
- function type references (`fn(T) -> R`)
- flow `when` guard arms (`case ... -> action`, `else -> action`)

### `=>` accepted before Mx102

- `match` arms (`ok(v) => { ... }`, `err(e) => { ... }`)
- `switch` arms (`case ... => expr`, `else => expr`)

### Diagnostics/tests/docs assumptions before Mx102

- parser diagnostics named specific spellings (`expected '->' ...`, `expected '=>' ...`)
- parser tests asserted those spelling-specific diagnostics
- reference docs predominantly taught split usage (`->` for signatures/when, `=>` for switch/match)

## 2) Ambiguity analysis

No new grammar ambiguity was found.

Reasoning:

- `=>` and `->` both occur in existing positions that already require an arrow separator token.
- lexical longest-match behavior already distinguishes `=>` from `=` and `->` from `-`.
- there is no expression grammar production where either arrow participates as a binary operator.

Result: unifying to a single internal Arrow token does not create parse conflicts in existing grammar productions.

## 3) Normalization approach

Normalization was implemented at the **lexer token-kind level**:

- both `=>` and `->` now emit `lex.Arrow`
- parser sites now consistently expect `lex.Arrow`
- spelling-specific semantic branches were removed (single parser-level arrow concept)

This is the narrowest practical approach and avoids duplicated “fat vs thin” semantic handling.

## 4) Formatter behavior

`ocfmt` now canonicalizes arrow output to `->`.

- mixed input forms are accepted
- formatter output converges to one house style (`->`)
- no formatter profile system was introduced in this milestone

## 5) Tests proving parity

Added/updated tests:

- lexer tests now validate unified token kind usage for both spellings
- parser parity test validates equivalent ASTs for paired `=>` / `->` forms across:
  - function signatures
  - function type references
  - `match` arms
  - `switch` arms
  - `flow` signatures and `when` arms
- formatter test validates normalization of mixed arrows to `->`
- language contract tests added under `Language/Expressions/ArrowUnificationMx102` with:
  - valid cross-surface parity coverage
  - invalid diagnostic coverage for missing arrow positions

## 6) Intentionally unchanged

- No new arrow forms were added.
- No semantic dispatch/type-system feature was added.
- Formatter remains non-configurable in this milestone; canonical style is `->`.
