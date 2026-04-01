# Tensor / Einstein Notation M2 Report

## Scope executed

M2 was executed as a strictness-preserving usability pass over the M1 indexed Einstein surface:

- improved composability for indexed expressions in local expression trees
- improved diagnostics for common misuse classes
- no broad semantic expansion beyond matrix-backed M0/M1 scope

## Composability improvements

M1 required both operands of an indexed binary operation to be direct indexed terms in that exact binary node.
This made expressions like:

- `(A[i, k] * B[k, j]) + C[i, j]`

fail, even though the left side is itself a valid indexed Einstein result.

M2 now preserves indexed-term metadata across indexed `+`/`*` results, so nested indexed expressions compose naturally in expression trees while keeping the same contraction/free-index checks.

This improves authoring ergonomics without adding arbitrary rank behavior, symbolic rewriting, or implicit shape-driven semantics.

## Diagnostics improved

M2 tightened error messages to explicitly show observed index structure where possible:

- malformed mixed indexed/non-indexed binary:
  - now reports whether left/right operands are indexed terms
- mismatched free-index addition:
  - now includes left/right free-index order observed
- repeated-index multiplicity misuse:
  - now includes the concrete indexed pair pattern that violated the rule
- mixed matrix index operand kinds:
  - now reports the exact index-type pair observed (for example `[Index, Int]`)

These changes keep rejection strictness intact while making failure causes faster to localize.

## Optional expansion decision (trace-style contraction)

Trace-style contraction (`A[i, i]`) was deliberately **not added** in M2.

Reason:

- it is mathematically natural, but introducing scalar-producing indexed terms would broaden the current matrix-result model and require additional surface/type/runtime rules best handled in a dedicated pass
- M2 goals were met through composability + diagnostics without widening semantic scope

## Strict boundaries preserved

Still rejected:

- repeated index multiplicity > 2 in Einstein multiplication patterns
- mismatched free-index order in Einstein addition
- mixed indexed/non-indexed binary misuse
- matrix indexed access with mixed `Int`/`Index` operand kinds
- non-matrix indexed-Einstein operand patterns

Still out of scope:

- arbitrary rank-N tensors
- symbolic tensor calculus
- coordinate transforms
- derivative operators
- continuum mechanics libraries

## Readiness finding

M2 meaningfully improves local scientific authoring ergonomics for matrix-backed indexed expressions and materially improves teachability through better diagnostics, while preserving strict boundaries.

This surface is now suitable for **early** continuum-mechanics-adjacent experimentation at matrix level, but not yet for broader tensor calculus workflows.

## Recommendation for M3

Run one more narrow tensor-surface pass before continuum mechanics libraries:

1. Decide whether to add bounded trace-style contraction (`A[i,i]`) with explicit scalar result rules.
2. Add label-aware diagnostics for deeper nested indexed trees (including source-side operand path hints).
3. Evaluate whether a minimal, explicit bridge for named indexed intermediates is needed beyond expression-tree composability.
