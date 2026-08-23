# OCT-LAYOUT-CONTRACT-M1 — Internal Semantic LayoutContract Prototype and Materialization Proof

## 1. Verdict

**Success**

The internal contract earned a backend consumer and a measurable result without changing Oct source, Octagon, or runtime formats. A compiled immutable record table now retains table identity through MIR and publication derives exact extent, logical order, and immutability. Those facts permit the Go compiled-data backend to emit static scalar column projections beside the compatibility row array. On the deterministic 100,000-row catalog-shaped benchmark, projected lookup and filtering were 2.3–3.3 times faster with zero allocations and zero startup work.

## 2. M0 recommendation carried forward

M0 recommended: **add semantic `LayoutContract` IR only; derive it from existing Oct constructs, no new syntax yet**. M1 follows that recommendation. It does not add `layout table`, a key annotation, physical layout directives, a runtime format, or a second type graph.

## 3. Implementation scope

The primary subject is the existing `CompiledDataValid.Catalog` publication. The exactly one non-table subject is its existing numerical/static `PublishedIDs: Int[]` publication. The implementation adds:

- compiler-internal `DataSubjectRef`, invariant, metadata, provenance, contract, and deterministic formatter types;
- `MIRRecordKind` for ordinary records, record tables, and synthetic table rows;
- record-table contracts on `MIRModule`;
- compiled-data contract derivation after typed value validation;
- an internal generic-emitter control;
- static scalar column projections selected by the Go backend;
- an exact-extent constant for the non-table static array;
- focused correctness, determinism, invalidation, generated-Go, hash, and benchmark coverage.

No source or reference-language file changed.

## 4. Pipeline architecture

The actual paths are:

```text
AST/typecheck record declaration
  -> lowerProgram
  -> MIRRecord.Kind + DataSubjectRef
  -> MIRModule.LayoutContracts
  -> compiled Go lowering (record identity remains inspectable)

typed artifact evaluation
  -> compileddata.Type + compileddata.Value
  -> validate shared table extent/value shape
  -> deriveLayoutContract
  -> emit generic row array
  -> Go backend optionally emits static column projections
  -> format/write generated Go
```

MIR contracts are created during `lowerProgram`. Publication contracts are created only after current type/value validation, enriched with eligibility metadata derived from the validated field types, consumed immediately by `EmitGo`, returned in `compileddata.Result` for tests/debugging, and otherwise discarded before runtime. There is no persistent runtime contract.

## 5. Record-table MIR preservation

`MIRRecord.Kind` now has three values:

```text
record
record-table
synthetic-table-row
```

A table and its synthetic row also carry references to the same existing nominal table identity. The synthetic row does not receive a table contract of its own. This prevents an ordinary record containing arrays from being confused with a shared-extent record table and prevents the projected row from acquiring table semantics.

Representative MIR dump:

```text
record Main.Rows kind=record-table
record Main.__oct_table_row_Rows kind=synthetic-table-row
layout-contract subject=mir-record-table:Main.Rows identity=nominal
```

## 6. LayoutContract IR

The implemented shape is intentionally smaller than M0's conceptual inventory:

```text
Contract {
    Subject DataSubjectRef { Kind, Identity }
    Invariants {
        NominalIdentity?
        ExactExtent?
        LogicalOrder?
        Immutable?
    }
    Metadata {
        ColumnProjections[]
    }
}
```

Each fact holds a phase/source provenance. `DataSubjectRef` points to an existing MIR or compiled-data identity; it does not copy fields, element types, refinements, or a type graph.

## 7. Invariant vs metadata separation

`Invariants` and `Metadata` are distinct Go types. `ExactExtent`, `LogicalOrder`, `Immutable`, and `NominalIdentity` are correctness facts. `ColumnProjectionEligibility` is an optional backend opportunity and cannot be assigned where an invariant is required. The emitter checks the semantic prerequisites separately before consuming projection metadata.

There is no estimated size field, capacity hint, or likely-key field, so an estimate cannot be mistaken for a hard bound and a hint cannot be mistaken for uniqueness proof.

## 8. Derivation rules

The prototype derives only these rules:

| Subject/fact | Derivation |
| --- | --- |
| MIR record-table identity | Current `ast.RecordDecl.IsTable` and nominal package/name during lowering |
| Compiled table exact extent | Validated equal column lengths in the concrete publication value |
| Compiled table logical order | Existing record-table column/index semantics preserve row order |
| Compiled table immutability | `Artifact.WriteCompiledData` emits a static publication value |
| Projection eligibility | Validated scalar/refinement/enum column with exact extent, order, and immutable publication |
| Static array exact extent/order | Concrete validated array value and its current index order |
| Static array immutability | Static compiled-data publication boundary |

Nothing is inferred from field spelling. In particular, `ID` does not become a key.

## 9. Proof provenance

Implemented proof carriers record the fact, the referenced subject, phase (`mir-lowering` or `publication-materialization`), and source (`package.type` or `Artifact.WriteCompiledData`). Exact extent is proved after table shared-extent validation for the concrete published value.

The catalog's `StaticAssert.True(AllIDsUnique(...))` and `IDsSorted(...)` results are still Boolean evaluation results, not typed field proofs. M1 therefore does **not** create a `KeySpec`, uniqueness invariant, sortedness invariant, binary search, or key index from those assertions. Doing so from helper names or the field name would violate the proof boundary. A future narrow proof carrier must recognize and preserve what was proved before a key consumer is legal.

## 10. Primary compiled-data subject

The unchanged fixture `Language/Tooling/CompiledData/valid/compiled_data_publication.octest` loads the six-row Octagon catalog, updates prices, derives published IDs, validates it, and calls `Artifact.WriteCompiledData`. Its table contract formats as:

```text
subject=compiled-data-root:CatalogData identity=nominal extent=exact:6 order=logical publication=immutable metadata=column-projection:ID metadata=column-projection:Status metadata=column-projection:Price metadata=column-projection:Name
```

The eligibility list follows declaration order and is deterministic.

## 11. Generic baseline realization

The internal `emitGoGeneric` control executes the previous materialization: declarations, hashes, row count, and one fixed Go row array. It emits no projection, parser, decoder, reflection, map, or initialization function. It is unexported and cannot become a user/compiler flag.

The six-row catalog baseline is 917 bytes of generated source.

## 12. LayoutContract-aware realization

The default `EmitGo` retains the compatibility row array and, when exact extent, logical order, immutability, and field eligibility are all present, emits static scalar arrays such as:

```go
var CatalogDataIDColumn = [...]int{1, 2, 3, 4, 5, 6}
var CatalogDataStatusColumn = [...]Status{
    Status_Draft, Status_Published, Status_Published,
    Status_Draft, Status_Published, Status_Published,
}
```

The backend chose this physical dual realization. Neither source nor contract says SoA, column array, Go array, or lookup algorithm. If any prerequisite is absent, the emitter retains the row-array baseline.

## 13. Generated Go comparison

Both lanes contain the same semantic declarations, row data, row count, logical hash, and schema hash. The aware lane adds only compile-time literals. Generated-source tests compile the result and compare every projected element with its row field.

Audit findings:

- no runtime parser or decoder;
- no reflection;
- no `init` index construction;
- no lazy first-use work;
- no map build;
- no `unsafe`;
- no allocator or ownership machinery.

## 14. Benchmark results

Command:

```text
go test ./internal/compileddata -run ^$ -bench BenchmarkCatalog -benchtime=500ms -benchmem -count=3
```

The deterministic scaled fixture has 100,000 rows with the catalog schema. Values below are the medians of three runs on Windows/amd64, AMD Ryzen 7 7700X. Lookup is a linear equality lookup in both lanes: the generic lane walks full rows; the aware lane walks the statically projected ID array and uses the matched ordinal to address the row. No key/sortedness optimization is claimed.

| Metric | Generic baseline | LayoutContract-aware | Change |
| --- | ---: | ---: | ---: |
| existing-key lookup ns/op | 69,832 | 24,667 | -64.7% |
| missing-key lookup ns/op | 74,880 | 22,620 | -69.8% |
| Status filter ns/op | 103,909 | 44,966 | -56.7% |
| allocs/op | 0 | 0 | unchanged |
| B/op | 0 | 0 | unchanged |
| generated catalog source size | 917 bytes | 1,269 bytes | +38.4% |
| runtime init work | none | none | unchanged |

The result is a speed/size tradeoff, not a universal mandate to duplicate every column. M1 deliberately lacks use-site hot-field proof, so the backend conservatively projects all eligible scalar columns for the prototype.

## 15. Allocation/startup results

All measured query classes report 0 B/op and 0 allocs/op. Both generated lanes consist only of static literals and constants. Startup performs no parsing, reflection, hashing, map construction, schema construction, or projection construction.

## 16. Semantic/hash compatibility

The aware and generic controls compute hashes before backend materialization from the same canonical type and value. Tests assert identical logical hash, schema hash, and row count across lanes. The real artifact retained:

```text
CatalogDataLogicalHash sha256:7254fe47f7c490d2b8c02bcc483a0036dbfe210acbef139b8168e275576be5b8
CatalogDataSchemaHash  sha256:5a3a7e85506278f12d456f5c03aa7a3faad3f67850fa2a4432e5e084fea816e8
CatalogDataRowCount   6
```

Only generated Go bytes and their artifact SHA-256 change. Octagon was not changed.

## 17. Non-table subject

The exactly one non-table subject is the fixture's existing numerical/static `PublishedIDs: Int[]`. Its contract is:

```text
subject=static-array:PublishedIDs extent=exact:4 order=logical publication=immutable
```

It has no nominal row/entity identity, key, origin, publication epoch, or projection metadata. The backend consumes exact extent to emit `const PublishedIDsExtent = 4` beside the fixed Go array. This is a small compile-time shape realization, but it proves the contract can describe an ordered numerical collection without inventing table concepts.

## 18. Access metadata

M1 implements one narrow metadata family: scalar column projection eligibility. It is derived statically after value/type validation, has publication-phase provenance, may be ignored, and is not a correctness promise. It does **not** claim that a field is hot, lookup-heavy, unique, or a key. No profiling infrastructure or source performance annotation was added.

## 19. Facts deliberately NOT implemented

Not implemented: key, uniqueness, sortedness, stable external identity, typed origin, publication epoch, canonical order, semantic hard upper bound, capacity estimate, hot fields/axes, access frequency, batch shape, index topology, relationship graph, FLOW state facts, or checkpoint facts.

The most important omission is key proof. Although the fixture checks IDs using ordinary helper functions and `StaticAssert`, current evaluation does not preserve a typed “field X of subject Y is unique/sorted” result. M1 refuses to bridge that gap with naming conventions.

## 20. Physical layout boundary

AoS/SoA, Go arrays/slices, offsets, padding, alignment, cache-line placement, byte order, dense slots, pointer shape, buffer ownership, allocator, and arena remain backend mechanisms. The contract says that ordered immutable fields share an exact extent and that projection is eligible; the Go emitter alone decides to materialize row and scalar arrays.

## 21. Failure/invalidation handling

Publication contracts are derived after each value has passed current shape validation; they are not cached across record updates. A focused test publishes extents 8 then 3 and proves the second contract and row count are rederived as 3. A mismatched table column still fails validation before contract creation. MIR synthetic rows carry a subject reference but do not inherit the table contract, preventing stale table semantics after row projection.

## 22. Complexity cost

The new authority is small: one internal package defines references, four invariant types, one metadata type, provenance, and a deterministic formatter. MIR gains a kind, a subject reference, and a contract slice. Compiled data gains one derivation function, one eligibility predicate, one projection emitter, and one unexported baseline seam. No parser, typechecker rule, runtime, Octagon, Concept, batch, or FLOW graph changed.

The cost is not zero: projection duplicates static data and generated API surface, and field eligibility is presently broader than actual hot-field evidence. That limitation is visible in the +38.4% six-row source-size result.

## 23. Was the IR worth it?

Yes, for an internal prototype. It preserved the exact semantic prerequisites needed to make a deterministic backend choice, improved all three measured query classes materially, added no runtime work, and generalized to a non-table ordered static array. It also made an important negative boundary executable: the compiler cannot emit a proved-key lookup until it has a real typed proof carrier.

This evidence does not justify user-facing layout syntax. The current win was derived entirely from existing constructs.

## 24. Final M1 recommendation

**1. LayoutContract is useful and should remain internal while more consumers are added**

## 25. What NOT to implement next

Do not add `layout table`, `key ID`, AoS/SoA, alignment, packing, cache-line, allocator, arena, pointerless, buffer-ownership, or capacity syntax. Do not change Octagon or compiled-data runtime format. Do not infer keys from names or snapshot uniqueness. Do not add profile infrastructure merely to choose columns. Do not globalize FLOW board state. Do not expose the contract dump or generic baseline as a stable CLI/API. Do not make the prototype's duplicate-all-scalars policy a language promise.

## 26. Exactly one next recommendation

**Preserve a narrow typed field-proof result from the existing `StaticAssert` evaluation path, then test a proved unique/sorted catalog field as a second internal compiled-data consumer.**

That next experiment should recognize what was proved structurally, carry subject/field/phase provenance, and permit a backend binary-search or static index choice only when the proof is present. It should not add syntax or generalize `StaticAssert` into a theorem system.
