# Utility selection

[Reference home](../README.md) · Prerequisites: [control flow](control-flow.md), [enums](enums-and-options.md)

Use `when utility` when several eligible outcomes compete on an explicit score.
It makes one deterministic choice. Use `if` for local conditional work and
`switch` for value dispatch or ordered decision ladders.

## One-shot selection

```oct
fn PickOwner(a: Int, b: Int) -> Int {
    return when utility {
        case 100 when a > 0 score a
        case 200 when b > 0 score b
        else -1
    }
}
```

Conditions are `Bool`. Scores are dimensionless `Int`, and the largest score
wins. Equal scores choose the earliest source case. An eligible case wins
even if its score is zero or negative. `else` is required and is selected
only when every case is ineligible; it is not a zero-score competitor.

All result arms have the same type. The expression returns that type directly.
It does not return an explanation object or an implicit flow handle.

## Evaluation order

1. Visit conditions in source order, once each.
2. Evaluate a score only when that case's condition is true.
3. Select the greatest score, using source order for ties.
4. Evaluate only the selected result expression, or `else` if none is eligible.

A losing result is not evaluated. Neither a call in that result nor a `?`
inside it runs. This rule holds for plain and enum-targeted selection, inside
functions and FLOW states, in interpreted and compiled software execution.

Standalone utility has no commitment state and accepts no policy block.
`hysteresis` and `min_commit` are errors here because there is no previous
choice to retain. See [controller policy](../runtime/utility-policy.md) for
the form that owns persistent selection state.

## Enum-targeted selection

Use an explicit enum target when the decision has a named closed vocabulary:

```oct
enum SampleAction { Keep Retest(Int) Discard }

fn DecideSample(suspect: Bool, repeatable: Bool) -> SampleAction {
    return when utility SampleAction {
        case SampleAction.Discard when suspect score 100
        case SampleAction.Retest(3) when repeatable score 60
        else SampleAction.Keep
    }
}
```

The target is authoritative. Every case and fallback constructs a qualified
variant of that enum, and the expression returns exactly that enum type.
Payload construction is selected-only too. Use ordinary enum `match` to read
the chosen payload later; utility cases do not bind payload patterns.

Not every enum variant needs to be offered. `else` is still required.
`Option<T>` is a valid target, and supplies the expected type for
`Option.Some(...)` and `Option.None`.

## Three forms named `when`

| Form | Where | How it chooses | Memory |
| --- | --- | --- | --- |
| `when { case condition -> action ... }` | FLOW state | First true guard, then required `else` | FLOW control only |
| `when utility [Enum] { ... }` | Expression positions | Greatest eligible `Int` score | None |
| `when policy { ... } { ... }` | FLOW state expression | Ranked choice subject to commitment | One persistent site |

Use [guard `when`](../runtime/flows-and-state-machines.md#guard-when) for visible state
transitions and [policy](../runtime/utility-policy.md) for repeated ranked
choices. The enum itself never owns policy or hidden mutable state.

## Executable contracts

The [lesson](../../ReferenceExamples/valid/unusual_features.octest) contains a
complete enum-targeted example. The [standalone contracts](../../Expressions/UtilityWhen/valid/)
cover ordinary selection, scores, ties, and selected-only evaluation. Controller
behavior is covered separately under [Octomata utility](../../ControlFlow/OctomataUtilityWhen/).
