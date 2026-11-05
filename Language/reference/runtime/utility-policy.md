# Controller utility policy

[Reference home](../README.md) · Prerequisites: [utility selection](../language/utility-selection.md), [Octomata](flows-and-state-machines.md)

Use `when policy` when a FLOW repeatedly ranks alternatives and must avoid
switching on every small score fluctuation. The selection site remembers its
committed arm. A fresh `when utility` expression has no such memory.

## Syntax and ownership

This is a declaration fragment inside a FLOW state, using the current turn's
`input` value:

```oct
let selected = when policy {
    hysteresis: 2
    min_commit: 3
} {
    case 1 when input.CanHeat score input.HeatScore
    case 2 when input.CanCool score input.CoolScore
    else 0
}
```

Both settings are required dimensionless `Int` values. Case conditions are
Boolean, case scores are dimensionless integers, and result arms share one
type. This form is legal only in FLOW state bodies. It may be the initializer
of a binding, the whole return value, or part of a larger expression.

Use a yielding or suspending FLOW that revisits the site to observe commitment
over time. A FLOW that immediately returns completes after its first choice;
it does not demonstrate stabilization across turns.

## Selection at each evaluation

Conditions and eligible scores are evaluated in source order. Highest score
wins, with earliest source case breaking a tie. Only the selected arm's result
is evaluated. The site is committed to the arm's identity, not its previous
value or previous score.

While the committed arm remains eligible, retain it if either:

- its age is below `min_commit` evaluations; or
- the leading arm does not beat its **current** score by more than `hysteresis`.

Otherwise select the leading arm and establish that commitment. An ineligible
committed arm cannot block a valid replacement. When no condition is true,
choose `else`; a prior fallback is never held against an eligible case.

`min_commit` counts site evaluations, not seconds or scheduler turns. Reaching
the site multiple times evaluates it multiple times. `hysteresis` compares
this evaluation's scores; it does not store the winning score from the past.

If an arm's value changes, holding it produces its new value. Two arms that
produce equal values remain distinct choices. These details matter for typed
payloads and for checkpoint/restore.

## State, checkpoints, and boundaries

Each lexical policy site owns its commitment on the flow instance. Ordinary
state progression and explicit board memory retain their existing meanings.
The logical checkpoint includes commitment arm and age; see
[turns and checkpoints](turns-and-checkpoints.md).

Policy does not provide an AI planner, probabilistic score model, scheduler,
wall-clock timer, or effect system. It is explicit deterministic arbitration
over the candidates written at the site. Use a guard when one Boolean rule is
authoritative rather than assigning it an artificial score.

See the [persistent controller contract](../../ControlFlow/OctomataUtilityWhen/runtime/valid/persistent_policy_controller.octest)
and the [utility corpus](../../ControlFlow/OctomataUtilityWhen/) for complete
controllers and the edge cases in both software lanes.
