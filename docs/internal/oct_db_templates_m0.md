# OCT-DB-TEMPLATES-M0 — Composable Semantic Specialization Templates

## 1. Verdict

Success

The modern parametric language removes the original blocker. Ten inspectable typed templates now compose into ordinary monomorphized Oct, the normal FLOW/MIR path, and generated Go. Valid and invalid tests pass in both execution modes, two unrelated applications reuse the catalog, and no runtime template mechanism or new language feature was added. OCT-TEMPLATE-CODEGEN-M0 closed the remaining parity question: equivalent source has structurally identical normalized MIR and byte-identical normalized FLOW Go, and the Go compiler reports the same inlining and escape profile. The earlier 11.7% W5 result was invalidly attributed to templates; controls that changed only benchmark/helper placement reversed its direction.

## 2. Prior Honest Stop

The first run correctly stopped because Oct had no user-defined type parameters, parametric queries, or typed selectors. It proved only that `*.template.oct` was ordinary source and concrete `with` worked; generic syntax, `.SKU`, and cross-record predicate reuse failed. That report is preserved in Git history and its raw probe output remains in OctetDB at `docs/product/evidence/OCT_DB_TEMPLATES_M0/probe-results.txt`. This rerun replaces its conclusion rather than treating it as a failed implementation.

## 3. Parametrics prerequisite result

OCT-PARAMETRICS-M0 supplies template record/flow/query declarations, deterministic early monomorphization, `Selector<Record,Field>`, exact function substitution, `with`, concrete `FieldRef` provenance, and normal FLOW lowering. The implementation and reference were audited before catalog design. The catalog needed no syntax, type-system, constraint-engine, or runtime extension.

## 4. Rerun of original blockers

The before/after matrix is recorded in `docs/internal/evidence/OCT_DB_TEMPLATES_M0/probes/README.md`. In summary: generic Record/Key records now instantiate; `fn(Record)->Bool` retargets independently to Job and InventoryItem; `.ID` and `.SKU` remain owner-correct; `FilteredView<Record>` specializes source/result FLOW; and cross-application specializations remain nominally distinct. The canonical suite passed 3 valid facts plus 11 expected compile failures in interpreted mode and compiled mode, with 3 compiled cases and zero fallback.

## 5. Template semantics

Templates encode application semantics only: nominal record/key/state types, exact selectors and predicates, logical bounds, source order/early-stop behavior, state domains, and rebuild/publication ownership. They contain no layout choice, allocation policy, hash-table knob, cache parameter, or throughput promise. Selection is explicit author intent; the compiler performs no template inference or profiling.

## 6. `*.template.oct` convention

The four catalog files under `Libraries/DatabaseTemplates` are ordinary Oct package source. The suffix affects deterministic discovery, documentation, and provenance only. Four category Concepts live beside the declarations; the small `DatabaseTemplateContracts` companion contains three refined value Concepts and admission APIs. Parsing, checking, interpretation, monomorphization, FLOW lowering, and Go emission use existing paths.

## 7. Catalog architecture

The catalog is deliberately limited to ten declarations and three shallow levels. Each documented Category names an actual String-refining Concept with a `Require` for its canonical value:

- primitives: `StableIdentity`, `BoundedExtent`, `RebuildPublication`;
- reusable patterns/query: `BoundedKeyedDataset`, `ReadMostlyDataset`, `MaterializedFilter`, `FiniteStateDataset`, `FilteredView`;
- starting points: `JobQueue`, `Inventory`.

The maximum composition depth is starting point → reusable pattern → primitive. There is no inheritance, implicit search, Webhook, Ledger, or speculative expansion.

## 8. BoundedKeyedDataset

`BoundedKeyedDataset<Record,Key>` explicitly contains `StableIdentity<Record,Key>` and `BoundedExtent<Record>`. `StableIdentity.KeyOf` is `Selector<Record,Key>`, so Job/ID and InventoryItem/SKU elaborate to different concrete owners even though both keys are String. Compile-fail cases reject a Job selector on InventoryItem and an Int selector where String is required.

## 9. MaterializedFilter

`MaterializedFilter<Record>` requires `ReadMostlyDataset<Record>`, an exact `fn(Record)->Bool`, and a positive application-owned Limit assumption. Its execution is the reusable `FilteredView<Record>` Query-M0 FLOW, which filters in source order and stops at Limit. Job/IsReady and InventoryItem/IsLowStock use the same declaration but become distinct concrete functions and types. It promises no transactional view maintenance.

## 10. FiniteStateDataset

`FiniteStateDataset<Record,State>` carries an exact state selector, the complete application-provided state values, and an application transition predicate. JobQueue uses it without moving workflow logic into the library. Wrong-owner state selectors fail statically. Inventory does not force this pattern.

## 11. Additional templates

`ReadMostlyDataset` is not a Boolean decoration: it requires a `RebuildPublication<Record>` callback/source version and records the measured reads-per-publication assumption. `RebuildPublication` makes the coherence authority explicit. `EventDedupeDataset` was explored and intentionally omitted because webhook point lookup and durable exact replay are already default OctetDB responsibilities; adding read materialization would be unjustified specialization.

## 12. Composition + `with`

Type arguments choose structure (`Record`, `Key`, `State`). Ordinary construction fills typed selectors/callbacks, while `with` remains the primary way to change application values. The proofs update bounds, expected reads per publication, and result limits with `with`; no alternative instantiate/override language exists. JobQueue composes bounded identity, finite state, and a Ready view; Inventory composes bounded SKU identity and a low-stock view.

## 13. Contracts

Existing exact typing enforces selector ownership/result, predicate input, state ownership, required fields, and compatible nominal composition. Required publication evidence is structural: a MaterializedFilter cannot be constructed without its `ReadMostlyDataset`, which cannot be constructed without `RebuildPublication`. Existing refined Concepts now constrain `MaxRecords`, `Limit`, and reads-per-publication to positive values, publication identity to a non-empty String, and publication version to a non-negative Int. Compile-time-known values fail through ordinary Concept/Require admission and erase to their base representation. Cross-package specialization retains the qualified refinement identity, so consumers explicitly import `DatabaseTemplateContracts`; no template-specific bound syntax or engine was added.

## 14. Invalid compositions

Eleven isolated `.octfail` cases cover cross-owner selector, incompatible composition, missing bound, missing publication, wrong state selector, wrong key type, wrong predicate type, non-positive bound/limit, empty publication identity, and negative publication version. Diagnostics include the concrete specialized field and expected/actual types or the exact Concept requirement. Because `.octfail` files compile in isolated temporary packages, each negative fixture mirrors the relevant canonical signature; valid canonical definitions are independently exercised by the core suite.

## 15. Provenance

MIR now retains compile-time-only specialization entries: declaration identity, concrete package/name, type arguments, kind, exact selector FieldRefs, and record `with` override fields. Generated Go emits comments such as `FilteredView__Job <- DatabaseTemplates.FilteredView<Main.Job>` and override comments, with no executable registry. Declaration lowering is name-sorted; a five-reload regression and the W5 generator prove byte-identical output before recording its SHA-256. The W5 sidecar records the Oct base revision plus milestone working-tree state, source/catalog hashes, and generated artifact SHA-256/size. Provenance does not serialize backend choices.

## 16. Discovery/catalog tooling

`oct templates list [root] [--json]` and `oct templates describe <Name> [root] [--json]` recursively parse ordinary source and deterministically derive name, kind, category, summary, type parameters, configuration fields, requirements, provided semantics, usage guidance, and source path. Discovery rejects a Category that does not resolve to a String-refining Concept with `Require`, and rejects unclassified requirement prose. Every requirement is labeled `Require`, `Type`, `Structure`, or `Application` in human and JSON output, so executable value admission is not confused with owner typing or lifecycle obligations. There is one source of truth: adjacent `///` docs plus declarations. Query-M0 currently drops doc comments from its reduced AST, so discovery recovers the adjacent ordinary comment block for query declarations without changing semantics.

## 17. Monomorphization/lowering

The backend audit observed 9 template records, 1 template FLOW, 15 record instantiations, 2 FLOW instantiations, and 3 resolved selectors in the canonical program; MIR contained 17 specialization provenance entries. Elaboration registered between the Windows timer floor and 0.520 ms. After early specialization, all nodes use existing concrete record/function/FLOW lowering. Generated source contains no `TemplateRuntime`, generic dictionary, VM, registry, reflection, or dynamic dispatch. General fallible refinement-admission functions are ordinary Concept machinery and remain uncalled for compile-time-known catalog values.

## 18. MIR/generated-code equivalence

The compiler test compares `FilteredView<Job>` with a handwritten concrete `BespokeFilteredView` after normalizing only the function name; their MIR is exactly equal by structural comparison. It then emits each FLOW core and public facade independently: after normalizing the nominal FLOW name and checkpoint fingerprint, the Go is byte-identical. The fingerprint remains intentionally distinct because checkpoint compatibility includes package/FLOW identity; provenance is comment-only. `go test -gcflags=-m=2` reports the same constructor/core/facade inlining and escape decisions. W5 result parity covers limits 1, 10, 2500, and 5000 with identical result/order behavior.

## 19. Compile-time/code-size cost

Catalog discovery medians 0.315 ms (about 123 KB and 571 allocations); template elaboration stayed below 0.520 ms. The final W5 generation series measured 346.91 ms first and 249.77–292.83 ms thereafter, versus retained bespoke warm samples of 144.00–175.08 ms. The four template files, including category Concepts, are 144 physical/128 nonblank lines; the companion value Concepts/APIs add 31/25 lines. W5 template application is 47/41 lines and 1,476 bytes; generated Go is 1,314/1,206 lines and 47,641 bytes. The retained bespoke W5 control is 34/26 Oct lines and generated 1,473/1,339 lines, 46,598 bytes. General Concept admission helpers create a small source-size tax but are unreferenced in the measured static-value runtime. Monomorphization cost is explicit and acceptable.

## 20. Architecture decision

A. Typed template composition cleanly elaborates into ordinary Oct and should be the standard advanced specialization authoring model.

## 21. Remaining limitations

Consumers must explicitly import the companion contracts package because imported template dependencies are not automatically re-exported into the specialization package. Calling configuration-held callbacks directly and generic exact-type assertions are rough edges observed by fresh agents. Compile-time Concept admission is zero-cost, but the general fallible admission helpers remain in emitted Go source until normal Go dead-code elimination. Materialized views remain immutable or explicitly rebuilt; transactional secondary-index maintenance is absent by design. W5 absolute timings remain sensitive to Go symbol and benchmark layout, so they establish parity/no attributable template tax rather than a template speedup. Provenance identifies the base compiler revision plus dirty milestone sources because this evidence precedes a commit.

## 22. Exactly one next recommendation

Productize this small catalog as the opt-in, profile-first advanced path while performing one bounded usability pass on configuration-held callback invocation and exact-typed assertion diagnostics before declaring the catalog stable.
