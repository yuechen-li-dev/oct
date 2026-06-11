# J1 — judgment enum utility selection design

## Status

J1 was **design-only**. J2 implements enum-targeted `when utility` M0 for qualified tag-only enum candidates, with payload candidates deferred.

## Motivation

Oct already has several adjacent pieces:

- nominal enums with qualified variants;
- tag-only and single-payload enum variants;
- exhaustive `match` for payload-binding enum analysis;
- enum switching and comparison behavior;
- Octomata control runtime;
- standalone `when utility` expression form;
- controller utility `when policy` inside flow states.

The missing shape is an explicit way to say:

> Given this enum as the closed judgment space, score candidate variants and return the selected variant.

The intended conceptual split is:

```text
match:
  enum value -> analyze selected variant

judgment utility:
  utility scores -> produce selected enum variant
```

`match` consumes an enum value that has already been chosen. Judgment utility produces an enum value by applying deterministic utility selection over a closed enum result space.

## Terminology

- **Judgment enum**: an ordinary enum used as the closed result space for utility-scored selection. The enum declaration itself is not special; judgment behavior comes from `when utility EnumName { ... }` at an expression site.
- **Judgment space**: the enum type named after `utility`; it bounds the set of possible selected judgments.
- **Candidate variant**: a qualified enum variant construction listed in a utility `case` arm.
- **Utility case**: one candidate variant plus its `when` condition and `score` expression.
- **Fallback/else variant**: the qualified enum variant construction returned by `else` when no utility case qualifies.
- **Selected judgment**: the enum value returned by the judgment utility expression after scoring and deterministic tie-breaking.

## Recommended syntax

J1 recommends extending standalone `when utility` with an optional enum target:

```oct
enum TreatmentDecision {
    Observe
    Retest
    Treat
    Escalate
}

fn Decide(risk: Float, confidence: Float) -> TreatmentDecision {
    return when utility TreatmentDecision {
        case TreatmentDecision.Observe when risk < 0.3 score 40
        case TreatmentDecision.Retest when confidence < 0.7 score 70
        case TreatmentDecision.Treat when risk >= 0.6 score 80
        case TreatmentDecision.Escalate when risk >= 0.9 score 100
        else TreatmentDecision.Observe
    }
}
```

This form is expression-shaped, one-shot, and usable wherever ordinary expressions are allowed.

## Syntax candidates

### Candidate A — enum-targeted `when utility` (recommended)

```oct
when utility TreatmentDecision {
    case TreatmentDecision.Observe when risk < 0.3 score 40
    case TreatmentDecision.Treat when risk >= 0.6 score 80
    else TreatmentDecision.Observe
}
```

Advantages:

- reuses the existing `when utility` family instead of adding a new top-level concept;
- makes the enum decision space explicit at the selection site;
- gives the compiler a target type before checking cases, improving diagnostics;
- keeps scoring policy local to expression sites instead of attaching live policy to declarations;
- clearly communicates that the result type is `TreatmentDecision`;
- naturally specializes existing utility selection machinery.

Disadvantages:

- extends the `when utility` grammar with one more optional shape;
- requires disambiguation from existing policy-field syntax after `utility`.

Recommendation: **choose Candidate A for M0**.

### Candidate B — new keyword `judge`

```oct
judge TreatmentDecision {
    case TreatmentDecision.Observe when risk < 0.3 score 40
    case TreatmentDecision.Treat when risk >= 0.6 score 80
    else TreatmentDecision.Observe
}
```

Advantages:

- short and visually distinct;
- gives judgment enums a memorable surface form;
- avoids overloading `when utility` parsing.

Disadvantages:

- introduces a new keyword or contextual keyword for a concept already covered by utility selection;
- increases the number of decision forms users must learn;
- creates an unnecessary split between ordinary utility selection and enum-targeted utility selection;
- risks implying a separate runtime or policy family.

Recommendation: **reject for M0** unless implementation work discovers that extending `when utility` creates unacceptable ambiguity.

### Candidate C — ordinary `when utility` with inferred enum type

```oct
when utility {
    case TreatmentDecision.Observe when risk < 0.3 score 40
    case TreatmentDecision.Treat when risk >= 0.6 score 80
    else TreatmentDecision.Observe
}
```

Advantages:

- requires no new syntax beyond the existing standalone form;
- may work today conceptually if ordinary result-arm type unification accepts enum values;
- keeps expressions compact.

Disadvantages:

- weakens diagnostics because the checker must infer intent from arms after parsing the body;
- makes wrong-enum candidates look like ordinary result-type mismatch instead of judgment-space errors;
- cannot distinguish “ordinary utility producing enum values” from “this enum is the judgment space”;
- provides no explicit place for future enum-targeted diagnostics, linting, or policy metadata.

Recommendation: **keep existing behavior valid**, but do not treat inference-only as the judgment enum design.

### Candidate D — enum-attached policy

```oct
enum TreatmentDecision {
    Observe
    Treat

    when utility DefaultPolicy(...) {
        ...
    }
}
```

Advantages:

- attaches named policy near the enum declaration;
- could support reusable domain policies later;
- makes a default judgment policy discoverable from the enum definition.

Disadvantages:

- changes enum declarations from pure closed data definitions into live policy containers;
- introduces policy naming, parameterization, overload, and lifecycle questions;
- makes it harder to keep utility scoring one-shot and expression-local;
- risks overlapping with Octomata policy memory and controller behavior;
- is too large for M0.

Recommendation: **defer/reject for M0**. If Oct later needs reusable policies, design them explicitly rather than smuggling them into enum declarations.

## M0 semantic rules

Recommended M0 rules for Candidate A:

1. `when utility EnumName { ... }` is an expression.
2. The expression type is `EnumName`.
3. `EnumName` must resolve to an enum type.
4. Each `case` result must be a qualified variant construction of `EnumName`.
5. `else` is required.
6. The `else` result must be a qualified variant construction of `EnumName`.
7. Case conditions must typecheck as `Bool`.
8. Scores must typecheck as `Int`, matching current utility score behavior.
9. Candidate selection follows existing standalone `when utility` score semantics.
10. Utility selection is deterministic.
11. Policy fields are not part of enum-targeted M0 unless J2 deliberately elects to reuse the existing optional standalone policy-field form.
12. Payload candidate construction is deferred for M0; see [Payload variants](#payload-variants).

Existing utility semantics already document deterministic behavior:

- highest score wins among qualifying cases;
- equal scores choose the earliest matching case in source order;
- `else` is selected only when no case qualifies.

J1 recommends preserving that rule for enum-targeted utility. The `else` arm should not be treated as an arbitrary scored candidate in M0.

### Optional future policy field syntax

If enum-targeted utility later accepts policy fields, two possible shapes are:

```oct
when utility TreatmentDecision {
    hysteresis: 2
    min_commit: 1
} {
    case TreatmentDecision.Treat when risk >= 0.6 score 80
    else TreatmentDecision.Observe
}
```

or another careful reuse of the existing optional standalone `when utility` policy form.

This is not part of the recommended M0 unless J2 explicitly chooses it. Even then, one-shot `when utility` must not secretly become Octomata controller memory.

## Exhaustiveness and `else`

J1 recommends:

- `else` is required in M0;
- not all enum variants must be listed;
- optional future lint may warn when a tag-only enum judgment does not mention all variants.

This intentionally differs from `match`.

`match` analyzes an already-selected enum value and must account for every possible variant. Judgment utility selects among utility-qualified candidates and needs a fallback for the case where no condition holds. Requiring all variants to appear would falsely make utility selection look like pattern matching and would not answer the defaulting problem.

## Payload variants

M0 should be conservative.

Recommendation:

- enum types with payload variants may be used as judgment spaces only if the actual candidate and `else` variants used by `when utility EnumName` are tag-only;
- payload variant construction in candidate or `else` arms is rejected in M0;
- payload candidate support is documented as future work.

Rationale:

- payload construction introduces evaluation-order and side-effect/fallibility questions;
- utility cases should remain simple decision candidates in M0;
- payload-bearing judgments may be useful, but their semantics should be deliberate.

Future payload syntax could look like:

```oct
case ParseDecision.Retry(delay) when unstable score 70
case ParseDecision.Fail("timeout") when timedOut score 100
else ParseDecision.Accept
```

Future design must specify when payload expressions are evaluated: only for selected candidates, for all qualifying candidates, or during candidate construction before scoring. J1 deliberately avoids that choice.

## Relationship to existing `when utility`

Existing standalone utility form is conceptually:

```oct
when utility {
    case value when condition score 10
    else fallback
}
```

Judgment enum target form is:

```oct
when utility EnumName {
    case EnumName.Variant when condition score 10
    else EnumName.Fallback
}
```

Rules:

- it is not a separate control-flow family;
- it is a typed specialization/extension of standalone utility;
- existing standalone `when utility` remains valid;
- enum-targeted form improves intent and diagnostics;
- the eventual implementation should reuse utility scoring machinery where possible.

## Relationship to Octomata

Judgment enum utility is one-shot selection. Octomata states, boards, and policy memory remain responsible for behavioral progression.

Standalone function example:

```oct
enum PumpJudgment {
    Hold
    Prime
    Run
    Fault
}

fn JudgePump(pressure: Float, fault: Bool) -> PumpJudgment {
    return when utility PumpJudgment {
        case PumpJudgment.Fault when fault score 100
        case PumpJudgment.Run when pressure > 20.0 score 60
        case PumpJudgment.Prime when pressure <= 20.0 score 40
        else PumpJudgment.Hold
    }
}
```

Flow using a judgment result:

```oct
flow PumpController(pressure: Float, fault: Bool) -> String {
    state Tick {
        let judgment = JudgePump(pressure, fault)

        when {
            case judgment == PumpJudgment.Fault -> goto Fault
            case judgment == PumpJudgment.Run -> return "run"
            case judgment == PumpJudgment.Prime -> return "prime"
            else -> return "hold"
        }
    }

    state Fault {
        return "fault"
    }
}
```

Design boundaries:

- judgment enum utility does not create controller commitment memory;
- judgment enum utility does not add hidden state to enums;
- users needing hysteresis/min-commit across ticks should use Octomata `when policy` or a future explicit policy support design;
- judgment enum utility is usable outside Octomata because it is an expression form.

## Typechecking diagnostics

Recommended diagnostics:

| Scenario | Diagnostic |
| --- | --- |
| Target is not enum | `when utility target must be an enum type` |
| Candidate is wrong enum | `utility case for TreatmentDecision cannot return PumpJudgment.Run` |
| Unqualified variant | `utility enum cases must use qualified variants, e.g. TreatmentDecision.Treat` |
| Missing `else` | `enum utility selection requires else fallback` |
| Condition is not `Bool` | `utility case condition must be Bool` |
| Score is not `Int` | `utility score must be Int` |
| Payload variant candidate in M0 | `payload enum variants are not supported in judgment utility cases yet` |

Diagnostic guidance:

- prefer judgment-space-specific messages when a target enum is present;
- mention the target enum name in wrong-enum candidate errors;
- do not collapse all errors into ordinary result-arm mismatch diagnostics;
- keep existing standalone `when utility` diagnostics unchanged unless the implementation can improve them without behavior drift.

## Parser/AST/typechecker/interpreter/compiler implementation plan

J1 does not implement this plan. Recommended J2 implementation shape:

### Parser

- Extend standalone `when utility` parsing to optionally accept an enum type target after `utility`.
- Preserve the existing no-target form.
- Preserve the existing optional policy-field form for ordinary standalone `when utility`.
- Disambiguate these starts after `when utility`:
  - `{ case ... }` or `{ policy-fields } { cases ... }`: existing standalone form;
  - `TypeName { case ... }`: enum-targeted form.

### AST

- Add optional `TargetType`/`EnumTarget` to the utility expression node.
- Preserve the existing representation for no-target utility expressions.
- Store source spans for the target type and each candidate result for diagnostics.

### Typechecker

- Resolve the target type as an enum.
- Typecheck each case condition as `Bool`.
- Typecheck each score as `Int`.
- Enforce required `else`.
- Check each candidate and `else` result against the target enum.
- Enforce qualified variant construction.
- Reject wrong-enum variants with a target-specific message.
- Reject payload variant construction in M0 if this design is accepted.

### Interpreter

- Reuse existing utility expression evaluation.
- Represent the selected value as the ordinary enum runtime value.
- Preserve existing deterministic score and source-order tie semantics.

### Compiler

- Lower enum-targeted utility to the same utility selection logic used by existing standalone utility, with enum values as candidate/else results.
- Maintain compiled/interpreted parity for scoring, fallback, and tie-breaking.
- Reuse existing utility-site state only if policy fields are supported; otherwise keep M0 one-shot and stateless.

### Tests

J2 should add:

- parser tests for targeted and non-targeted utility forms;
- typechecker tests for wrong target, wrong enum candidate, unqualified variant, missing `else`, non-`Bool` condition, non-`Int` score, and payload candidate rejection;
- interpreter tests for winner selection, fallback, and tie-breaks;
- compiled parity tests if compiled standalone utility is supported;
- reference examples and invalid examples in `Language/`.

## Examples

### 1. Scientific decision enum

```oct
enum TreatmentDecision {
    Observe
    Retest
    Treat
    Escalate
}

fn DecideTreatment(risk: Float, confidence: Float) -> TreatmentDecision {
    return when utility TreatmentDecision {
        case TreatmentDecision.Escalate when risk >= 0.9 score 100
        case TreatmentDecision.Treat when risk >= 0.6 score 80
        case TreatmentDecision.Retest when confidence < 0.7 score 70
        case TreatmentDecision.Observe when risk < 0.3 score 40
        else TreatmentDecision.Observe
    }
}
```

### 2. Control judgment enum

```oct
enum PumpJudgment {
    Hold
    Prime
    Run
    Fault
}

fn JudgePump(pressure: Float, fault: Bool) -> PumpJudgment {
    return when utility PumpJudgment {
        case PumpJudgment.Fault when fault score 100
        case PumpJudgment.Run when pressure > 20.0 score 60
        case PumpJudgment.Prime when pressure <= 20.0 score 40
        else PumpJudgment.Hold
    }
}
```

### 3. Invalid wrong-enum candidate

```oct
enum TreatmentDecision {
    Observe
    Treat
}

enum PumpJudgment {
    Hold
    Run
}

fn Bad(risk: Float) -> TreatmentDecision {
    return when utility TreatmentDecision {
        case PumpJudgment.Run when risk > 0.5 score 80
        else TreatmentDecision.Observe
    }
}
```

Expected diagnostic shape:

```text
utility case for TreatmentDecision cannot return PumpJudgment.Run
```

### 4. Invalid missing `else`

```oct
enum TreatmentDecision {
    Observe
    Treat
}

fn Bad(risk: Float) -> TreatmentDecision {
    return when utility TreatmentDecision {
        case TreatmentDecision.Treat when risk > 0.5 score 80
    }
}
```

Expected diagnostic shape:

```text
enum utility selection requires else fallback
```

### 5. Invalid payload candidate if payload support is deferred

```oct
enum ParseDecision {
    Accept
    Retry(Int)
    Fail(String)
}

fn Bad(unstable: Bool) -> ParseDecision {
    return when utility ParseDecision {
        case ParseDecision.Retry(3) when unstable score 70
        else ParseDecision.Accept
    }
}
```

Expected diagnostic shape:

```text
payload enum variants are not supported in judgment utility cases yet
```

## Reference docs update policy for J1

J1 lightly updates reference docs with explicitly marked **proposed/design-only** notes because the feature touches existing enum and Octomata/utility concepts and future readers should not confuse `match`, ordinary `when utility`, and enum-targeted utility.

The reference notes must not imply that current parser/typechecker/interpreter/compiler support exists. The normative implementation details remain deferred to J2.

## Recommended next milestone: J2

**J2 — implement enum-targeted `when utility` M0**.

J2 scope:

- parser/AST support;
- typechecking;
- interpreter support;
- compiled support if existing utility expressions compile;
- docs examples;
- tests.

J2 non-goals:

- payload candidates;
- enum-attached policies;
- policy memory/hysteresis;
- LLM hooks;
- new solver/AI runtime.
