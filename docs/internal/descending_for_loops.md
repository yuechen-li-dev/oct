# Descending `for` loops design/recon

## Current audit

- Accepted loop syntax before this change was `for name in start..end { ... }` and `for name in start..end step n { ... }`.
- `step` is optional; an omitted step means magnitude `1`.
- `for` ranges are closed range expressions with half-open loop execution: ascending loops include `start` and exclude `end`.
- Ascending `start > end` is invalid, not empty. Equal bounds are valid and produce zero iterations.
- Steps are positive `Int` magnitudes. Literal zero and negative steps are rejected by the typechecker; dynamic invalid steps are rejected at runtime/lowering validation.
- Runtime validation exists in the interpreter range evaluator and in compiled `lowerForStmt`, before entering the loop body. Compiled implicit-step ascending loops skip the explicit positive-step validation branch because step `1` is known valid.
- Interpreter execution evaluates range expressions into `Range` values, validates them once, then executes `current < end`. Compiled lowering evaluates start/end/step into locals, validates before the condition block, and emits MIR condition/body/update blocks.
- Existing invalid coverage includes open-ended `for` ranges, zero/negative `step`, non-`Int` range endpoints/steps, and ascending `start > end` runtime rejection.

## Syntax decision

M0 supports:

```oct
for i in start..end descend n {
    ...
}
```

It also accepts the low-complexity shorthand:

```oct
for i in start..end descend {
    ...
}
```

which means `descend 1`.

`step` and `descend` are mutually exclusive. `step -n` remains invalid; descending intent is expressed by `descend` with a positive magnitude.

## Semantics

Ascending behavior remains unchanged:

```oct
for i in start..end step s { ... }
```

is equivalent to evaluating `start`, `end`, and `s` once, validating `s > 0` and `start <= end`, then executing while `i < end` and incrementing by `s`.

Descending behavior is:

```oct
for i in start..end descend s { ... }
```

Evaluate `start`, `end`, and `s` once, validate `s > 0` and `start >= end`, then execute while `i > end` and decrement by `s`.

Both directions are half-open. Examples:

- `0..5` visits `0, 1, 2, 3, 4`.
- `5..0 descend 1` visits `5, 4, 3, 2, 1`.
- `5..0 descend 2` visits `5, 3, 1`.
- `5..-1 descend 1` visits `5, 4, 3, 2, 1, 0`.

## Validation rules

- Loop variable type remains `Int`.
- Bounds and explicit magnitudes must be `Int`.
- `descend 0` and `descend -1` are invalid; literal cases are rejected statically where possible, dynamic cases at runtime.
- `for i in 0..10 descend 1` is invalid because the declared direction contradicts the bounds.
- `for i in 10..0` remains invalid and does not auto-descend.
- Equal bounds are valid and perform zero iterations.

## Implementation shape

`ast.ForStmt` carries an explicit `ForDirection` enum plus an optional descending magnitude expression. Direction is not smuggled through a signed step. The parser recognizes `descend` after the range expression. The interpreter and compiled lowering mirror ascending loops with `>` and subtraction for descending loops, keeping validation outside the loop body.
