# OCT-PARAMETRICS-M0

## 1. Verdict

Success

## 2. Motivation from TEMPLATES-M0

The previous milestone isolated the gap correctly: ordinary Oct source, `with`, Concepts, Require, StaticFacts, LayoutContract, query, FLOW, batch, and Go generation already supplied the semantic/runtime substrate. Reusable database-shaped declarations lacked typed substitution, exact typed field references, and deterministic specialization. This milestone adds only those authoring mechanisms.

## 3. Existing type system

Records are nominal and package-qualified. Concept aliases/refinements expand before checking; `Require` and static facts use existing exact types and subjects. Function values are exact, including `fn(Job) -> Bool` versus `fn(InventoryItem) -> Bool`. Arrays, record tables, queries, FLOW, immutable same-type `with`, and direct field access already lower concretely. Compiler types are AST `TypeRef` values resolved by project loading and the ordinary typechecker; Go lowering consumes checked concrete declarations. Built-ins have limited type-like syntax, but there was no user-defined declaration parameterization or selector value.

The new fit is deliberately early: parsing records open type references, project elaboration resolves applications and selectors, and only then do Concept expansion, ordinary checking, interpretation, FLOW/MIR, and Go emission proceed.

## 4. Parametric declaration syntax

M0 accepts `template record`, `template fn`, `template flow`, and `template query` with `<T, U>`. Type parameters may appear in record fields, arrays, exact function signatures, results, query sources/yields, and `Selector<Record, Field>`. Applications supply every type argument explicitly: `Box<Job>`, `Identity<Job>(job)`, and `Filtered<Job>(jobs, IsReady, 2)`. Generic record literals use `Box<Job> { ... }`. Ordinary declarations reject type parameter lists.

The `<...>` audit preserves built-in dimensional syntax and comparison expressions; only declaration/type/call positions parse type arguments. The formatter has a regression test for the distinction.

## 5. `template` keyword decision

`template` is a contextual top-level keyword with a bounded role: it marks open authoring declarations that must disappear before ordinary execution, improves declaration/application diagnostics, and records origin metadata. It provides no evaluation model.

## 6. Type-parameter semantics

Parameters denote exact concrete Oct types. Substitution is invariant and recursive through arrays, function types, selector types, record fields, statements, expressions, record literals, and query-lowered FLOW declarations. M0 has no value parameters or inference; ordinary fields such as `MaxRecords` remain values.

## 7. Monomorphization model

Project loading gathers open templates, removes them from the executable declaration set, discovers explicit applications, and emits concrete declarations to a fixed point. The canonical key contains template package/name, declaration kind, consumer package, and canonical concrete arguments. Identical keys deduplicate. A bounded instantiation stack rejects recursive specialization. Every load path uses this elaborator, so interpreter and compiler receive the same result.

## 8. Nominal identity

Concrete record names are deterministic specializations such as `BoundedKeyedDataset__Job__String`. Their package-qualified declarations remain nominal. `BoundedKeyedDataset<Job, String>` and `BoundedKeyedDataset<InventoryItem, String>` cannot be assigned across one another even when their field shapes coincide; the invalid suite proves rejection.

## 9. Typed selector model

The public type is `Selector<Record, FieldType>` and the concise expression is `.Field` under contextual typing. Resolution requires one exact record, an existing field, and an exact result type. It elaborates to a generated `fn(Record) -> FieldType` getter, so it is a first-class function-compatible value without reflection. Cross-owner and wrong-result assignments fail statically.

## 10. Selector lowering to FieldRef

Generated getters perform direct field access. Separately, MIR records selector provenance using the existing `layoutcontract.FieldRef`: exact nominal subject, ordinal, and field name. There is no second string-based field identity. The backend test checks `Job.ID` and `InventoryItem.SKU` retain different exact subjects.

## 11. Parametric functions

`FirstWhere<T>` and `Selected<Record, Key>` prove reusable bodies, arrays, results, selector arguments, and exact function-value substitution. `fn(T) -> Bool` becomes `fn(Job) -> Bool` or `fn(InventoryItem) -> Bool`; the checker never introduces `any` or an erased callable.

## 12. Parametric records

`BoundedKeyedDataset<Record, Key>` carries `Selector<Record, Key>` and ordinary `MaxRecords`. `MaterializedFilter<Record>` carries `Record[]` and `fn(Record) -> Bool`. Concrete applications retain all field types and distinct nominal identities. Parametric record-table use is inherited when its element type is concrete after elaboration; no collection redesign was added.

## 13. Parametric queries

`template query Filtered<T>(source: T[], predicate: fn(T) -> Bool, limit: Int) yields T` specializes for unrelated record types. The proof checks source ordering, predicate filtering, early `take` completion, and exact yielded types in both execution lanes.

## 14. FLOW/lowering relationship

Oct's parser already represents Query-M0 syntax as its established FLOW skeleton rather than retaining a separate query AST. Therefore syntactic query desugaring occurs while parsing open type references; monomorphization substitutes that skeleton before ordinary typecheck, MIR, runtime, or Go lowering. The resulting concrete flows use the same single `Scan` state as handwritten Query-M0. FLOW itself gained no runtime generic mechanism.

## 15. Concepts/Require interaction

Existing transparent Concept aliases expand through type arguments. Existing `Require` expressions in template bodies are checked after concrete substitution and follow the normal proof path. The valid contract combines a Concept alias, a parametric function, `Require`, and `with`. No `where` syntax, traits, typeclasses, or implicit search was added.

## 16. StaticFacts/LayoutContract interaction

Template declarations disappear before ordinary semantic checking, so facts attach only to concrete subjects. Selector metadata lowers into existing exact-subject `FieldRef`; LayoutContract enrichment therefore retains its current cross-subject rejection behavior. Tests assert exact nominal selector subjects and reject cross-instantiation reuse.

## 17. `with` interaction

Instantiation produces an ordinary concrete record value, after which existing immutable same-type `with` applies unchanged. Type arguments customize structure; fields customize values. No derived nominal update type, structural surgery, const generic, or alternate override language was needed.

## 18. Diagnostics

Diagnostics name source-level owners and expected exact types: `Selector .SKU does not exist on Job`, selector result mismatches, and `expects fn(InventoryItem) -> Bool, got fn(Job) -> Bool`. Recursive specialization reports `infinite template instantiation detected` with an application chain. `TemplateOrigin` retains the declaration and concrete arguments for later tooling. One ergonomic limitation remains: calling a function-valued record field directly parses as package qualification, so users bind it to a local first; fresh agents found this and the public reference now states it.

## 19. Separate-package behavior

Imported templates specialize deterministically in the consumer. This admits consumer-owned nominal arguments without a reverse dependency or global registry. A compiled and interpreted package proof imports `Box<T>`/`Identity<T>`-style declarations. M0 cross-package template bodies should be self-contained or use names visible to the consumer; automatic qualification of arbitrary template-package-private dependencies is deferred.

## 20. Interpreter/compiler parity

All positive packages pass interpreted and compiled with zero fallback: core records/functions/selectors 1/1, query/materialized filter 2/2, Concepts/Require/with 1/1, separate-package use 1/1, and Algorithms 1/1. The six invalid contracts also pass in both requested lanes. Parametrics are elaborated at project load, not implemented only in Go emission.

## 21. Generated Go audit

Generated output contains concrete specialized records, functions, FLOW machines, and direct selector getter accesses. Backend tests require their names, compile/run the real path, and reject marker strings for template runtimes, dictionaries, or retained type arguments. Parametric and handwritten query MIR are deeply equal after normalizing only the flow name. No manual Go repair is required.

## 22. Compile-time/code-size results

For 1/10/100 distinct `Identity<Rn>` applications, observed generated source was 1,399/8,480/81,954 bytes and instantiation counts were exactly 1/10/100. Project-load times were 7.79/2.09/10.38 ms in one Windows run; isolated phases below the clock resolution reported zero. Growth is approximately linear. Identical semantic keys deduplicate within a consumer, but M0 intentionally does not attempt linker-scale cross-package code folding.

The runtime sanity test reports zero allocations for both variants and identical MIR. Its latest raw medians were 68,425 ns/op handwritten and 88,394 ns/op parametric; because identical MIR rules out generic dispatch and Windows runs varied strongly with order/layout, this milestone does not claim equal raw throughput from that microbenchmark. The evidence records the discrepancy rather than smoothing it away.

## 23. General non-database proof

`Libraries/Algorithms` adds `FirstWhere<T>(items: T[], predicate: fn(T) -> Bool) -> T ! Error`. It specializes for unrelated types, preserves exact predicate typing, and passes interpreted and compiled. This proves the feature is language infrastructure rather than a database special case.

## 24. BoundedKeyedDataset proof

One declaration is instantiated as `BoundedKeyedDataset<Job, String>` with `.ID` and `BoundedKeyedDataset<InventoryItem, String>` with `.SKU`. Both typed reads and independent `MaxRecords` values pass. An additional fresh-agent trial uses different key result types (`String` and `Int`). Nominal cross-assignment is rejected.

## 25. MaterializedFilter proof

`MaterializedFilter<Job>` stores a `Job[]` and `fn(Job) -> Bool` for Ready jobs. `MaterializedFilter<InventoryItem>` stores an `InventoryItem[]` and exact low-stock predicate. `FirstWhere<T>` consumes each safely. Read-only/rebuild intent remains ordinary explicit source; no publication runtime was invented.

## 26. W5 query reuse proof

The reusable portion is `Filtered<T>`. The Job specialization preserves source order, filters Ready records, yields two matches, and completes early at the limit. The InventoryItem specialization reuses the same declaration. Backend inspection proves the same FLOW state class and MIR as a handwritten concrete query, with safe concrete Go and no new query runtime.

## 27. Invalid-selector/predicate tests

Six `.octfail` contracts cover absent fields (`Job.SKU`), incompatible field result type, selector cross-owner reuse, exact predicate cross-owner reuse, nominal specialization distinction, and bounded recursive instantiation failure. All expected diagnostics match.

## 28. LLM trials

Three fresh agents received only public/reference material. A passed on its first reusable predicate/query source with no hallucinations. B corrected one unsupported direct call of a selector-valued record field. C used exact owner diagnostics to repair deliberately wrong selector and predicate reuse, then made the same field-call correction. All final sources passed interpreted and compiled without fallback. Detailed turns, times, diagnostics, and readability notes are in the evidence directory.

## 29. Readability review

The concrete forms repeat source/predicate/limit or key/bound structure for every domain type. The parametric forms state that shared structure once while applications keep domain types, selectors, predicates, and ordinary values explicit. This makes the invariant clearer without asking readers to execute compile-time code. Explicit arguments add visible ceremony, but that ceremony usefully exposes nominal specialization. The selector-field call workaround is less readable and is the clearest follow-up ergonomics issue; it does not outweigh the reduction in duplicated semantic declarations.

## 30. Architecture decision

B. Parametrics work, but `template` deserves a thin distinct declaration role for diagnostics/provenance/discovery.

## 31. Template-keyword decision

T1. Add `template` as a thin parametric authoring declaration.

## 32. Selector decision

S1. First-class typed `Selector<Record, Field>` values are clean and sufficient.

## 33. Downstream/template decision

P1. Language prerequisites are sufficient to rerun OCT-DB-TEMPLATES-M0 immediately.

## 34. Explicit rejected metaprogramming features

M0 adds no higher-kinded/type-level/dependent types, const generics, compile-time execution, macros, AST/token generation, quote/unquote, eval, reflection, field enumeration, traits/typeclasses, implicit specialization, runtime generic VM/dictionaries, dynamic template registry, partial specialization, or lifetimes. It performs bounded typed substitution only.

## 35. Remaining limitations

All type arguments are explicit. Recursive template calls are conservatively rejected rather than distinguishing runtime recursion from expanding type recursion. Function-valued record fields must be bound before invocation. Cross-package bodies do not automatically qualify arbitrary template-private dependencies. Provenance is retained in compiler data but has no catalog UI. Dedup is consumer-local. The throughput microbenchmark remains noisy despite structurally identical MIR. There are no advanced bounds, inferred arguments, value parameters, field enumeration, or runtime generic values.

## 36. Exactly one next recommendation

Rerun OCT-DB-TEMPLATES-M0 using `BoundedKeyedDataset<Record, Key>`, `MaterializedFilter<Record>`, and typed selectors as the authoring layer, while keeping storage, query, publication, StaticFact, LayoutContract, FLOW, and Go runtime behavior unchanged.
