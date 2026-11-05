# Learn and use Oct

Oct is a statically typed language for scientific programs. If you already
program, start with the five short lessons below. You will recognize functions,
values, arrays, and loops; the lessons introduce Oct's domain types, explicit
errors, and unusual features when they solve a concrete problem.

**The correct way should be the easy way.** Prefer immutable values, put units
in types, handle failure where it belongs, and choose the smallest construct
that expresses your intent. You do not need FLOW, templates, or a GPU to write
a useful Oct program.

## Learn by running programs

| Lesson | What you will be able to do |
| --- | --- |
| [1. Your first program](learn/01-first-program.md) | Read Oct syntax, write a function, and run a real test |
| [2. Values with meaning](learn/02-domain-values.md) | Name domain values, use units, update values, and handle failure |
| [3. Functions and alternatives](learn/03-functions-and-alternatives.md) | Pass functions, capture values, and distinguish absence from errors |
| [4. Collections and mathematics](learn/04-collections.md) | Choose arrays, masks, vectors, matrices, and counted loops |
| [5. Choosing Oct's unusual features](learn/05-choosing-features.md) | Use queries, templates, utility decisions, FLOW, async, and batch appropriately |

From a checkout, run all lesson contracts with:

```text
go run ./cmd/oct test Language/ReferenceExamples/valid --execution compiled
```

Use `oct` instead of `go run ./cmd/oct` with an installed CLI. See the
[overview](overview.md) for execution modes and the
[glossary and translation guide](glossary.md) for terminology from other languages.

## Everyday language

| Topic | Reference |
| --- | --- |
| Source, comments, naming, escapes, and arrows | [Lexical structure](language/lexical-structure.md), [formatting](tooling/formatting.md) |
| Bindings, mutation, scopes, and value semantics | [Variables](language/variables-and-values.md), [arrays](language/arrays.md), [global rules](language/rules-and-style.md) |
| Types, numeric admission, and identity | [Types](language/types.md) |
| Operators, evaluation order, ranges, and precedence | [Expressions](language/expressions.md) |
| Functions and calls | [Functions](language/functions.md), [function values and explicit captures](language/function-values.md) |
| Conditions, branching, and loops | [Control flow](language/control-flow.md) |
| Errors and recovery | [Fallible results, `?`, `!`, and `match`](language/errors.md) |
| Product values and immutable updates | [Records and record tables](language/records-and-tables.md) |
| Alternatives and missing values | [Enums, payload matching, and `Option<T>`](language/enums-and-options.md) |
| Imports and package identity | [Packages](language/packages-and-imports.md), [manifests and dependencies](tooling/packages-and-dependencies.md) |

## Scientific and domain programming

| Need | Reference |
| --- | --- |
| Dimensional correctness | [Units](language/units.md) |
| Domain names, checked values, and compile-time requirements | [Concepts and refinements](language/concepts-and-refinements.md) |
| Linear algebra, Einstein notation, and differential operators | [Vectors, matrices, and tensors](language/vectors-matrices-and-tensors.md) |
| Scalar math, conversions, strings, and core operations | [Builtins](language/builtins.md) |
| Numerical, scientific, data, and output libraries | [Library guide](language/standard-libraries.md), [complete library directory](libraries.md) |
| Reusable typed structure and exact field references | [Templates and selectors](language/templates-and-selectors.md) |
| Typed template markup | [Oct-XML](language/typed-markup.md) |

## Decisions, iteration, and behavior

| Need | Reference |
| --- | --- |
| Rank eligible alternatives once | [Standalone `when utility`](language/utility-selection.md) |
| Filter, project, and consume a read-only source | [Queries](runtime/queries.md) |
| Map independent array items and join their results | [Batch](runtime/batch.md) |
| Model modes, transitions, interruption, and local memory | [Octomata / FLOW](runtime/flows-and-state-machines.md) |
| Exchange input and yields, inspect state, or checkpoint | [Turns and checkpoints](runtime/turns-and-checkpoints.md) |
| Stabilize repeated choices over time | [Controller `when policy`](runtime/utility-policy.md) |
| Write a resumable computation with explicit awaits | [Async functions](runtime/async.md) |

## Develop, verify, and publish

| Task | Reference |
| --- | --- |
| Test valid behavior and expected failures | [Octest](tooling/testing.md) |
| Publish typed outputs, check static invariants, or measure code | [Artifacts, StaticAssert, compiled data, and benchmarks](tooling/artifacts-and-benchmarks.md) |
| Exchange typed data | [Octagon](tooling/octagon.md), [JSON](language/standard-libraries.md#json) |
| Build and run | [CLI](tooling/cli.md), [targets and profiles](tooling/targets.md) |
| Connect to Go, native wrappers, or a UI host | [Interoperability](tooling/interoperability.md) |
| Publish semantic documentation graphs | [Atlas](tooling/atlas.md) |
| Read the editor's human-oriented view | [VSCode](tooling/editor-views.md) |

## Specialized surfaces and maintenance

The [support boundary](support.md) distinguishes ordinary Oct from experimental
or separately governed facilities. Those facilities remain discoverable:
[Verilog](language/compilation-profiles.md), [SDSL-V](../../docs/SDSL_V_LANGUAGE_SPEC.md),
[Prometheus](runtime/prometheus.md), [UIBridge protocol](tooling/uibridge-protocol.md),
and [experimental Make and OctGen workflows](tooling/interoperability.md#build-and-source-generation-tools).

The [coverage map](coverage.md) connects language features to reference chapters
and executable contracts. The [maintenance guide](maintaining.md) explains how
to keep links, lessons, and coverage current. Chapters use descriptive filenames
and grouped navigation; the sections above define the reading order.
