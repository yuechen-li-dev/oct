# OCT-DB-TEMPLATES-M0 — Composable Semantic Specialization Templates

## 1. Verdict

Honest stop

## 2. Motivation

The goal was to amortize OctetDB specialization expertise through typed,
inspectable semantic patterns. A useful template must retarget a pattern across
application record types while preserving typed keys, predicates, ordering,
and publication contracts. Metadata alone does not meet that bar.

## 3. Existing composition mechanisms

Oct already has nominal records and record tables, record-shaped and refined
Concepts, compile-time `Require`, immutable `with`, directory packages/imports,
constants, typed function values, compiled data, StaticAssert-derived
StaticFacts, internal LayoutContract derivation, FLOW, query syntax, and batch.
These mechanisms remain the correct lowering/runtime foundation.

The decisive limitation is equally established: user-defined generics are not
supported. Function values require exact signatures; record identity is
nominal; package-qualified concept aliases are deferred; and no type value or
typed field-selector value exists. The earlier
`array_map_generics_friction_f1.md` audit independently reaches the same result
for reusable collection helpers.

## 4. `*.template.oct` semantics

`filepath.Ext("BoundedKeyedDataset.template.oct")` is `.oct`, so current package
discovery already loads the file as ordinary Oct. The convention probe compiles
and executes through the normal path. The suffix needs no parser mode, runtime
semantics, or compiler behavior. It would affect intended use and future
discovery tooling only.

## 5. Why no new runtime exists

No template VM, runtime registry, backend, text expansion, AST macro system, or
unrestricted compile-time evaluator was added. The valid probe becomes an
ordinary typed record update and follows the existing interpreter/compiler
path.

## 6. `with` customization model

`with` correctly checks fields, exact field types, and immutable same-type
updates. It can customize a concrete `MaxRecords`. It deliberately cannot turn
`JobFilterTemplate { Predicate: fn(Job) -> Bool }` into an inventory template
using `fn(Item) -> Bool`; the nominal-predicate probe is rejected. This is good
type safety, but it prevents the requested cross-application customization.

## 7. Contract/type checking

Existing `Require`, refined Concepts, StaticAssert/StaticFacts, and
LayoutContract can enforce facts once a concrete subject exists. They cannot
declare a reusable `BoundedKeyedDataset<Record, Key>` or prove that a `.SKU`
selector belongs to `Record`, because neither type parameters nor field
selectors exist. Encoding names as `String` would become the prohibited
`map[string]any` design in record form.

## 8. Composition rules

Ordinary records/modules compose concrete values explicitly, and incompatible
concrete function signatures fail clearly. They do not provide parametric
composition across Job, InventoryItem, and webhook record identities. A catalog
made only of booleans/enums such as `ReadMostly = true` would decorate rather
than elaborate the bespoke query and data model.

## 9. Standard template catalog

No canonical five-template catalog was installed. Each required candidate
needs at least one missing capability:

- `BoundedKeyedDataset`: record/key type parameters and a typed key selector.
- `ReadMostlyDataset`: can describe intent, but cannot retain the dataset's
  typed identity when composed.
- `MaterializedFilter`: a predicate parameter whose input is the source record
  type, plus lifecycle/coherence evidence tied to that source.
- `JobQueue` and `Inventory`: distinct concrete applications can be written,
  but cannot reuse the lower-level patterns without duplicating their typed
  query declarations.

Publishing placeholders would violate the template inspection rule and make
the compiler accept semantically disconnected assemblies.

## 10. Provenance

The evidence records exact Oct and OctetDB commits and source paths. Generated
specialization provenance was not added because no valid reusable
specialization elaboration exists. A filename/hash registry would be truthful
for discovery but could not prove that its overrides affected generated code.

## 11. Diagnostics

The probes preserve application-facing evidence at three boundaries: generic
record syntax is rejected by parsing; a cross-nominal predicate override is
rejected by ordinary exact function typing; and `.SKU` is rejected because no
field-selector expression/type exists. Template-origin context cannot be
meaningfully attached before a template application construct exists.

## 12. Discovery/tooling

Directory enumeration could trivially list `*.template.oct`, but adding `oct
templates list/describe` now would advertise unusable composition. The file
suffix itself is deterministically discoverable and requires no compiler
change; catalog metadata is deferred with the catalog.

## 13. Generated lowering

The convention probe lowers normally and adds no runtime abstraction. The
existing W5 query already lowers filter/map/take directly to the established
FLOW machinery. There is no honest template-authored W5 source to compare:
without parametric queries, the query declaration remains bespoke.

## 14. Compile-time cost

The convention probe adds no special elaboration phase. No template compile-time
benchmark is claimed because metadata-only records would not measure the
requested capability.

## 15. Invalid-composition tests

Reproducible compiler probes cover unsupported parametric records, wrong
record/predicate types, and missing typed field selectors. The requested six
semantic composition failures cannot all be authored until template subjects,
selectors, and publication contracts can be expressed. Inventing string flags
solely to manufacture those failures was rejected.

## 16. Architecture decision

**D. The current type/composition model cannot express reusable specialization
cleanly.**

Architecture A handles filename convention and concrete configuration only.
B adds discovery/provenance but not typed reuse. C would require a bounded new
language construct whose type substitution, selector typing, query lowering,
diagnostics, and provenance amount to a separate compiler milestone, not a
minimal M0 patch.

## 17. Remaining limitations

There are no user-defined type parameters, typed member selectors, parametric
records/concepts/queries, or a way for `with` to produce a related nominal type.
Materialized-view coherence also remains read-only or explicitly rebuilt, as in
PERF-M4; templates must not conceal that later.

## 18. Exactly one next recommendation

Run one prerequisite language milestone that adds the minimum user-defined
parametric records/functions/queries plus typed field selectors, with ordinary
monomorphization and no compile-time execution, then rerun OCT-DB-TEMPLATES-M0.
