# Oct terminology for programmers

[Reference home](README.md)

| Oct term | Meaning and familiar connection | Read next |
| --- | --- | --- |
| `let` / `var` | Immutable binding / rebindable binding; arrays still have value semantics | [Variables](language/variables-and-values.md) |
| `concept Name = T` | Transparent domain name for an existing type | [Concepts](language/concepts-and-refinements.md) |
| `concept Name { ... }` | Nominal product value, using ordinary record semantics | [Concepts](language/concepts-and-refinements.md) |
| Refined concept | A named admissible subset, proved statically or checked at an explicit fallible constructor | [Refinements](language/concepts-and-refinements.md#refined-concepts-m1) |
| `Require` | A bounded compile-time requirement, not a dynamic assertion | [Concepts](language/concepts-and-refinements.md#compile-time-requirements) |
| `record` / `record table` | Nominal product / immutable equal-length typed columns | [Records](language/records-and-tables.md) |
| `with` after a value | Immutable same-type record update | [Records](language/records-and-tables.md#immutable-with-updates) |
| `with` after a function signature | Explicit immutable capture environment | [Function values](language/function-values.md) |
| `T ! Error` | A fallible result, handled by propagation, recovery, or explicit fatal unwrap | [Errors](language/errors.md) |
| `Option<T>` | Normal absence or one value; a builtin enum, read with `match` | [Options](language/enums-and-options.md#option) |
| `template` | Explicit typed specialization into concrete declarations; no runtime generic dictionary | [Templates](language/templates-and-selectors.md) |
| `Selector<R, T>` | An exact field reference that elaborates to a getter; no string field lookup | [Selectors](language/templates-and-selectors.md#typed-selectors) |
| FLOW / Octomata | The explicit state/continuation model behind `flow`, query, and async | [Octomata](runtime/flows-and-state-machines.md) |
| `state` | A named control location in a FLOW; ordinary code runs in its activation | [Octomata](runtime/flows-and-state-machines.md) |
| `board` | Fixed-shape private state owned by a FLOW, retained between turns | [Boards](runtime/flows-and-state-machines.md#blackboards-for-control-loops) |
| Turn | One `Step` from a continuation to yield, suspension, or final return | [Turns](runtime/turns-and-checkpoints.md) |
| `yield` / `suspend` / `return` | Publish a value and pause / pause without a value / complete the FLOW | [Turns](runtime/turns-and-checkpoints.md) |
| `remember` / `resume` | Write / consume one explicit remembered state slot | [Octomata](runtime/flows-and-state-machines.md) |
| `query` | A bounded source scan that lowers to a yielding FLOW; not SQL or a separate runtime | [Queries](runtime/queries.md) |
| `async fn` / `await` | Sequential authoring for generated FLOW continuation; no implicit scheduler | [Async](runtime/async.md) |
| `batch` | Independent array mapping with ordered results and an implicit join | [Batch](runtime/batch.md) |
| Utility | A deterministic ranking by explicit eligibility and integer score | [Utility selection](language/utility-selection.md) |
| Hysteresis / commitment | Score-gap / minimum-age rules at a persistent policy site | [Controller policy](runtime/utility-policy.md) |
| Octagon | A data-only typed value format, distinct from executable source | [Octagon](tooling/octagon.md) |
| Artifact | An explicit build-time output entry, distinct from correctness tests and benchmarks | [Artifacts](tooling/artifacts-and-benchmarks.md) |
| `StaticAssert` | An assertion evaluated while an ordinary helper executes under a compiler-owned static phase | [Static checks](tooling/artifacts-and-benchmarks.md#require-assert-and-staticassert) |
| Oct-XML | Typed markup elaborated to ordinary calls, arrays, and literals | [Markup](language/typed-markup.md) |
| OctGo / OctGen | Go companion contracts / explicitly staged source generation | [Interoperability](tooling/interoperability.md) |
| Octxiliary | Sidecar machinery for supported native or standard-library operations | [Interoperability](tooling/interoperability.md) |
| Compilation profile | Compile-time legality and target choice, currently `Verilog` | [Profiles](language/compilation-profiles.md) |

`concept` does not mean a C++ concept or a trait conformance system. `query`
does not imply a database planner. `async` does not imply a background thread.
Those distinctions are part of choosing the construct that owns your work.

For overloaded punctuation, consult [expressions](language/expressions.md):
`..` ranges, `...` repeats literal elements, postfix `!` unwraps failure,
`not` negates a Boolean, and `@` performs a supported tensor contraction.
