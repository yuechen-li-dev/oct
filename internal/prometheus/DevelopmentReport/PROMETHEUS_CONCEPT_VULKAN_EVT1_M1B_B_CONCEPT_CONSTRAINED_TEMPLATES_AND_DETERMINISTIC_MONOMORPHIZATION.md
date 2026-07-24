# Concept/Vulkan EVT1 M1B-B — Concept-Constrained Templates and Deterministic Monomorphization

Status: **SUCCESS**

Expected commit for this pass: `concept-vulkan: add constrained templates`

## 1. Starting checkpoint and worktree state

- starting checkpoint: `2fb18a7591bd7beb0a01829220dd191fde625ab3`
- starting branch: `main`
- starting worktree: clean porcelain
- starting repository state: accepted M1B-A checkpoint at `HEAD`, no uncommitted files

## 2. Inspected M1A authority

- `internal/prometheus/DevelopmentReport/PROMETHEUS_CONCEPT_VULKAN_EVT1_M1A_PAYLOAD_ENUMS_AND_EXHAUSTIVE_MATCH.md`
- `docs/Concept-Vulkan/CONCEPT_VULKAN_LANGUAGE_CONSTITUTION.md`
- `examples/Concept-Vulkan/evt1_m1a_language.concept`
- `examples/Concept-Vulkan/evt1_m1a_vulkan.concept`
- checked M1A generated outputs under `internal/conceptvulkan/generated/`

## 3. Inspected M1B-A struct authority

- `internal/prometheus/DevelopmentReport/PROMETHEUS_CONCEPT_VULKAN_EVT1_M1B_A_MUTABLE_STRUCTS_AND_NAMED_CONCEPT_REQUIREMENTS.md`
- `internal/conceptvulkan/evt1_parse.go`
- `internal/conceptvulkan/evt1_validate.go`
- `internal/conceptvulkan/evt1_generate.go`
- `internal/conceptvulkan/evt1_types.go`
- `examples/Concept-Vulkan/evt1_m1b_a_language.concept`
- `examples/Concept-Vulkan/evt1_m1b_a_vulkan.concept`

## 4. Inspected M1B-A immovability authority

- bounded `immovable struct` rules in the M1B-A report and constitution were treated as accepted authority
- the live implementation already enforced final-storage construction, no whole-value copy, no by-value parameter/return, no embedding, and no enum-payload placement
- M1B-B reused those rules after concrete substitution rather than redefining them

## 5. Inspected M1B-A concept authority

- one-parameter named concepts
- required free-function signatures
- prerequisite concepts over the same parameter
- declaration-level concrete satisfaction assertions
- exact-signature matching with no ranking or conversion

## 6. M1B-A evidence preflight

- confirmed the accepted starting checkpoint and clean worktree
- confirmed the M1A and M1B-A reports already recorded deterministic generation, stale-output rejection, JSON validation, strict C11, Vulkan-header compilation, public-header preservation, export preservation, ABI digest preservation, and production-route preservation
- reran the canonical current compiler lanes from this repository state to re-establish that foundation before layering M1B-B

## 7. Status-document drift and reconciliation

- the living status and reviewer handoff still treated M1B-B as deferred
- both were updated to mark kernel-54 as historical accepted foundation, M1A and M1B-A as accepted closures, M1B-B as accepted closure, M1B-C as the next deferred milestone, and DragonGod as deferred post-substrate direction
- the historical kernel-54 `M1C` naming collision with future EVT1 `M1B-C` is now called out explicitly

## 8. Scope and non-goals

This pass adds exactly one bounded facility: concept-constrained one-parameter free-function templates with explicit concrete invocation and deterministic monomorphization.

This pass does not add type deduction, specialization, SFINAE, nested template calls, recursive instantiation, template types, runtime interfaces, compile-time user evaluation, DragonGod, production routing changes, shader changes, or ABI growth.

## 9. Exact template grammar

Accepted declaration form:

```concept
template <typename T>
requires VulkanResource<T>
void DestroyResource(borrow T value)
{
    Destroy(value);
}
```

Adaptation from C++-shaped reference spelling: Concept/Vulkan keeps the already-accepted `borrow` vocabulary instead of `T&`.

## 10. Template parameter scope

- exactly one template type parameter
- scoped to the template declaration and body only
- valid in return types, parameter types, local declaration types, and dependent call signatures
- no default, no variadic form, no non-type parameter, no template parameter packs

## 11. Exact constraint grammar

- exactly one `requires ConceptName<T>` clause
- the concept must already exist and be one-parameter
- the constraint must apply to the template's own parameter, not a concrete type or a different name
- no Boolean combinations, no multiple clauses, no ranking syntax

## 12. Prerequisite closure behavior

- closure traversal is declaration-order depth-first through prerequisite concepts
- shared prerequisite requirements are deduplicated once by exact substituted signature
- each retained closure entry preserves its first deterministic prerequisite path

## 13. Symbolic template-body checking

- templates are checked before any concrete instantiation
- the symbolic environment contains the template parameter identity, the root constraint identity, the ordered prerequisite closure, and the deduplicated required operation set
- ordinary non-dependent names are still resolved against the ordinary free-function environment at template-check time

## 14. Dependent operation binding

- dependent free-function calls are legal only when they match exactly one required operation in the constraint closure
- the binding is recorded by explicit requirement identity in typed MIR as `requirement_call`
- if a call is not guaranteed by the closure, the declaration fails immediately

## 15. Non-dependent name binding

- ordinary calls whose argument types do not depend on `T` still resolve against the ordinary free-function environment
- this pass widened that environment from one-function-per-name to exact-signature selection across same-named free functions so concept requirements can bind `Destroy(BufferRange)` and `Destroy(PipelineState)` without adding ranking

## 16. Forbidden dependent operations

- dependent field access is rejected
- template-to-template calls are rejected
- dependent operators are rejected
- no dependent member-call or method surface exists in the grammar

## 17. Explicit invocation syntax

Accepted call form:

```concept
DestroyResource<PipelineState>(state);
```

Calls without explicit `<ConcreteType>` continue to follow ordinary-function lookup only.

## 18. Comparison-grammar preservation

- `Name<Type>(...)` is recognized only by a narrow lookahead that requires `< type > (`
- statement-start template calls no longer misparse as variable declarations
- ordinary `<` and `>` comparisons still parse as comparisons when that postfix pattern is absent
- a dedicated regression test proves chained comparison reaches type checking rather than template-call misparse

## 19. Concrete substitution algorithm

For each explicit template call, the compiler:

1. resolves the unique template by name;
2. resolves and validates one concrete non-template type argument;
3. proves the named concept constraint for that type;
4. substitutes the concrete type through signature and body type positions;
5. rebinds required operations to unique exact ordinary free functions;
6. revalidates the instantiated function under ordinary ownership/typing rules;
7. records or reuses the canonical instance by semantic key.

## 20. Concept satisfaction before instantiation

- M1B-B reuses the existing M1B-A satisfaction engine
- prerequisite-failure paths remain reported through the same deterministic chain format
- no second concept system was introduced

## 21. Exact concrete operation binding

- same function name
- same arity
- same parameter order
- same parameter types
- same borrow/const qualifiers
- same return type
- no conversions or ranking

The implementation now supports same-named ordinary free functions with different exact signatures, but selection remains exact and deterministic only.

## 22. Ownership revalidation

- instantiated functions are checked again under ordinary by-value boundary rules
- non-copyable value passing/copying is still rejected
- no hidden clone, move, retain, release, or ownership metadata was introduced

## 23. Immovability revalidation

- borrowed `immovable struct` use through templates is accepted
- by-value instantiation over an immovable concrete type is rejected at instantiation time
- no semantic temporary copy is introduced by monomorphization

## 24. Instantiation failure paths

- unknown template
- invalid or non-concrete type argument
- failed named-concept satisfaction
- missing exact concrete operation
- wrong concrete operation qualifiers/signature
- immovable by-value parameter or return after substitution
- ordinary body revalidation failures after substitution

## 25. Absence of SFINAE

Failed instantiation is a hard compiler error. No failed substitution removes a candidate or triggers alternate template search.

## 26. Bounded instantiation graph

- roots come only from explicit template calls in ordinary non-template code
- nested template invocation is rejected
- recursive template instantiation is rejected by construction because templates cannot invoke templates
- no dynamic type-argument generation exists

## 27. Deduplication key

Canonical instance key:

```text
template_name | canonical_concrete_type_identity
```

The concrete type identity is deterministic and derived from semantic type shape, not map order, paths, or timestamps.

## 28. Deterministic instance ordering

- template declaration order
- then concrete type identity lexical order within each template

This ordering drives MIR instance sections and private C instance emission.

## 29. Typed template representation

Typed MIR now carries:

- template identity
- template parameter identity
- constraint concept identity
- ordered prerequisite closure
- deduplicated required operation identities
- symbolic signature
- symbolic operations
- requirement-bound symbolic call nodes

## 30. Typed instance representation

Typed MIR now also carries:

- canonical instance identity
- template source identity
- concrete type identity
- generated private symbol
- concrete requirement bindings
- invocation spans
- concrete operations for the instantiated body

## 31. Private C11 lowering

- template declarations emit no header prototype
- each instantiated specialization lowers to one `static` private C11 function in the generated `.c`
- ordinary generated functions call those private helpers directly

## 32. Deterministic mangling

Template instances use stable private symbols of the form:

```text
concept_vulkan_template_<template>__<type_identity>
```

Same-named ordinary overloads now receive stable exact-signature suffixes only when needed.

## 33. Absence of runtime generic machinery

- no runtime template object
- no runtime concept object
- no witness table
- no vtable
- no RTTI or metadata registry
- headers contain only ordinary structs/enums and ordinary public function declarations

## 34. Complete diagnostic inventory

New or materially extended diagnostics cover:

- missing template constraint
- duplicate template declaration
- unknown template-constraint concept
- wrong template-constraint parameter target
- missing template body
- dependent field access
- explicit-template-call requirement
- nested template invocation
- dependent operator rejection
- call not guaranteed by the constraint
- invalid concrete type argument
- exact-signature ambiguity under same-named free functions

## 35. Parser test matrix

- specimen parse/generate for M1A, M1B-A, and new M1B-B language/vulkan fixtures
- explicit template invocation postfix
- statement-start template calls versus variable declarations
- comparison parsing preserved beside explicit template invocation

## 36. Typing test matrix

- exact constraint grammar
- same-parameter enforcement
- dependent requirement-bound calls
- explicit-call-only rejection
- nested template rejection
- dependent field/operator rejection
- immovable by-value instantiation rejection

## 37. MIR test matrix

- `requirement_call` appears for symbolic template bodies
- template and instance sections are deterministic
- repeated same-type calls produce one instance section

## 38. Generator test matrix

- checked-output match
- deterministic in-memory generation
- deterministic double-directory generation
- stale-output rejection
- private instance emission
- no runtime concept/template text leakage

## 39. Negative specimen matrix

Focused negative sources now cover:

- missing constraint
- concrete type in template constraint
- dependent field access
- missing explicit type argument
- nested template call
- dependent operator use
- non-guaranteed dependent call
- immovable by-value instantiation

## 40. Language specimen

`examples/Concept-Vulkan/evt1_m1b_b_language.concept` proves:

- two concrete types satisfying one named concept graph
- same-named exact free-function operation binding across those types
- repeated same-type template invocation reuse
- distinct type specializations
- borrowed immovable use through templates
- comparison parsing beside explicit template calls

## 41. Vulkan-shaped specimen

`examples/Concept-Vulkan/evt1_m1b_b_vulkan.concept` proves:

- same mechanism for Vulkan-shaped ordinary and immovable host-side types
- compilation against real Vulkan headers
- explicit-only specialization and reuse for a Vulkan-handle range plus immovable pool state

## 42. Native execution result

- hardware-independent template specimen: PASS under Visual Studio 18 `VsDevCmd` and strict C11
- expected result: repeated score `14`, pipeline score `12`, deterministic destroy side effects `[1, 13, 13, 5]`, immovable borrowed path succeeds

## 43. Strict C11 compilation

- PASS under `cl /std:c11 /W4`
- the accepted pass was run through `C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat -no_logo`

## 44. Real Vulkan-header compilation

- PASS with `VULKAN_SDK=C:\VulkanSDK\1.4.350.0`
- the Vulkan-shaped native harness compiled and executed successfully in the Visual Studio 18 developer environment

## 45. Deterministic double generation

- in-memory repeated generation: PASS
- separate-directory double generation: PASS

## 46. Stale-output rejection

`Check` still rejects hand edits with `CV3001`; the lane remains green after the M1B-B changes.

## 47. Source-map and manifest validation

- every generated `.mir.json`, `.map.json`, and `.manifest.json` still parses as JSON in tests
- no timestamps or machine-specific output data were introduced

## 48. M1A regression result

PASS. M1A fixtures, MIR, checked outputs, and generated/native paths remain green under the richer compiler.

## 49. M1B-A regression result

PASS. Structs, immovability, named concepts, and preserved M1A behavior remain green under the richer compiler.

## 50. Public API and ABI preservation

- no public Prometheus header was changed
- no exported symbol count changed
- canonical ABI digest remained `89053790ac5a18d29a21141527e017efc2faa03932d3adc2307891fdb8da0262`

## 51. Production-route preservation

Production remains handwritten. The new template-generated outputs stay under `internal/conceptvulkan/generated/` and are not wired into Prometheus production sources.

## 52. Shader/package/kernel preservation

- shader source unchanged
- SPIR-V unchanged
- package identity unchanged: `prometheus.core@1`
- kernel identities unchanged
- manifests, locks, and kernel authorities unchanged

## 53. Dominatus boundary

Preserved exactly. No policy, authorization, scoring, admission, or lifecycle progression moved into Concept/Vulkan.

## 54. SDSL-V boundary

Preserved exactly. No shader-language feature or shader-runtime responsibility moved into Concept/Vulkan.

## 55. Go experimental-compiler boundary

Preserved. EVT1 remains implemented in the Go experimental compiler line; no production dependency on a future general Concept compiler was introduced.

## 56. Zig-bootstrap backport boundary

Preserved. No broader Zig Concept bootstrap backport or modification was attempted in this repository.

## 57. RQ-M1 paused-state preservation

Preserved. The accepted physical-batch boundary remains paused and unchanged.

## 58. DVT-2 paused-state preservation

Preserved. No DVT-2 optimization or Stage 7 resumption occurred.

## 59. Known limitations

- exactly one template parameter
- exactly one named concept constraint
- explicit-only invocation
- no nested template calls
- no deduction, specialization, or SFINAE
- no operator requirements or dependent field/member surface

## 60. Unresolved blockers

None for M1B-B. The bounded requested vertical converged.

## 61. M1B-B completion assessment

Assessment: **CONCEPT/VULKAN EVT1 M1B-B: SUCCESS**

The constrained-template surface, symbolic checking, exact requirement binding,
explicit-only instantiation, deterministic deduplication, explicit MIR
representation, private C11 lowering, native proof, and regression
preservation all passed.

## 62. Exact proposed M1B-C assignment

```text
Concept/Vulkan EVT1 M1B-C — Bounded Pure Compile-Time Evaluation
```

The intended scope is a narrow terminating evaluator for compile-time graph and
representation validation that consumes, but does not redefine, named concepts
or template monomorphization.

## 63. Exact post-M1B-C DragonGod direction

```text
DragonGod — Typed Lifecycle Automata for Concept/Vulkan Mechanisms
```

It should consume the accepted structs, concepts, constrained templates,
ownership, borrowing, enums, and `yield`-successor substrate to model legal
lifecycle states, typed signals, ordered transition effects, rollback, and
facts reported upward to Dominatus.

## 64. Rollback boundary

Rollback for this pass removes:

- EVT1 template parser/validator/MIR/generator changes in `internal/conceptvulkan`
- new M1B-B specimens
- new M1B-B generated authorities
- this report and the related constitution/status/handoff/evidence-index updates

Rollback does not touch:

- production handwritten routing
- public ABI / exports
- shader or package authority
- Stage 3–6 authorities

## 65. Complete validation matrix

| Lane | Result |
| --- | --- |
| starting checkpoint / branch / clean worktree verification | PASS |
| M1A authority inspection | PASS |
| M1B-A implementation inspection | PASS |
| living-status / handoff drift audit | PASS |
| `go test ./internal/conceptvulkan -count=1` | PASS |
| `go test ./cmd/concept-vulkan ./internal/conceptvulkan -count=1` | PASS |
| `go build ./cmd/concept-vulkan` | PASS |
| deterministic repeated generation | PASS |
| deterministic double-directory generation | PASS |
| checked outputs match | PASS |
| stale-output rejection | PASS |
| source-map / manifest / MIR JSON validation | PASS |
| strict C11 M1B-B language specimen compile+run via `VsDevCmd` | PASS |
| strict C11 M1B-B Vulkan-shaped specimen compile+run via `VsDevCmd` | PASS |
| full `internal/conceptvulkan` package under `VsDevCmd` | PASS |
| native compiler authority | PASS — Visual Studio 18 Community `VsDevCmd.bat`, `cl /std:c11 /W4` |
| Vulkan SDK/header authority | PASS — `VULKAN_SDK=C:\VulkanSDK\1.4.350.0` |
| focused Prometheus authority check (`go run ./tools/prometheus_stage0 -check`) | PASS |
| public-header comparison | PASS — no public Prometheus header changed |
| export comparison | PASS — 84 exports unchanged |
| ABI digest comparison | PASS — `89053790ac5a18d29a21141527e017efc2faa03932d3adc2307891fdb8da0262` |
| production-route preservation | PASS |
| shader/package/manifest/lock/kernel preservation | PASS |
| `git diff --check` | PASS |
