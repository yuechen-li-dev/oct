# OCT-LAYOUT-CONTRACT-M2 — Typed Static Fact Provenance

## 1. Verdict

**Success**

The artifact evaluator now preserves two bounded semantic facts about an exact evaluated record-table value, compiled-data promotes those facts into the exact publication's `LayoutContract`, and the backend emits a zero-initialization binary-search lookup only when both facts are present. The existing catalog fixture now produces `CatalogDataLookupByID`; same-named fields on another table, a transformed post-proof value, and an unproved table do not.

## 2. M1 limitation

M1 received only a validated typed value. `StaticAssert.True(AllIDsUnique(table))` and `StaticAssert.True(IDsSorted(table))` had already collapsed to ordinary `Bool` results. The evaluator discarded the table instance, field, comparison coverage, assertion site, and proof phase. Consequently, uniqueness or sortedness could not lawfully become a key-related invariant. M1 correctly withheld that promotion.

## 3. Current static-evaluation architecture

The audited path is:

```text
oct artifact
  -> package load, bind, and typecheck
  -> interpreted execution with ArtifactWriteCapability
  -> ordinary helper/function execution
  -> evalAssertCallExpr for StaticAssert.*
  -> evalArtifactWriteCompiledDataBuiltin
  -> compileddata.EmitGo
  -> validate typed Dataset
  -> derive LayoutContract
  -> emit and gofmt static Go source
  -> staged publication
```

Static assertions execute in `internal/interpret` only when a compiler-owned artifact capability is present. At that point the evaluator has typed `Value` instances, nominal record-table declarations, ordered field declarations, concrete row indices, scalar comparison operators/results, the `StaticAssert` call line/column, and the artifact entry identity. Before M2, helper calls retained nominal type but not evaluated subject identity. Assertions retained only success/failure; compiled-data received no assertion evidence.

The minimum missing carrier was therefore not a proposition language. It was an evaluated table-instance identity, a subject-scoped field reference, a recognized fact kind, assertion provenance, and a fact attachment passed to compiled-data.

## 4. StaticFact model

The implemented internal model lives beside `LayoutContract`:

```text
StaticFact {
    Subject    DataSubjectRef
    Kind       StaticFactKind
    Fields     []FieldRef
    Provenance StaticFactProvenance
}

StaticFactSet { Facts []StaticFact }
```

`StaticFactSet.Add` de-duplicates identical evidence and preserves deterministic evaluation order. There are no connectives, quantifiers, proof terms, dependencies, or general predicates.

## 5. Subject/field identity

Each evaluated record-table value receives a deterministic `DataSubjectRef` of kind `static-evaluation-value`, such as `Catalog#2`. Passing a value through a helper preserves that identity. A table literal, Octagon materialization, or immutable table `with` result receives its own identity.

`FieldRef` is:

```text
FieldRef {
    Subject DataSubjectRef
    Ordinal int
    Name    string
}
```

The ordinal is the typed schema position; the name remains diagnostic/generated-name material. Promotion requires the fact subject, field subject, publication proof subject, schema ordinal, and schema name all to agree. A global `Unique("ID")` representation does not exist.

At publication, accepted facts are explicitly rebound from the exact evaluated subject to the exact compiled-data root. This is the only identity transition.

## 6. Provenance model

`StaticFactProvenance` records:

```text
Phase    = artifact-static-evaluation
Source   = static-assert
Identity = <package>.<artifact-entry>:<line>:<column>/field-comparison-coverage
```

Only `artifact-static-evaluation` plus `static-assert` is accepted by compiled-data promotion. The bounded source enum has no runtime-check or profile-hint variant, so those sources cannot accidentally become semantic invariants. Provenance is compiler metadata and is not emitted into the runtime artifact.

The M1 formatter now prints field ordinal and proof phase/source/site beside promoted facts, for example:

```text
subject=compiled-data-root:CatalogData ...
fact=unique:ID[0] provenance=artifact-static-evaluation/static-assert:CompiledDataValid.PublishCatalog:41:5/field-comparison-coverage
fact=sorted-ascending:ID[0] provenance=artifact-static-evaluation/static-assert:CompiledDataValid.PublishCatalog:42:5/field-comparison-coverage
metadata=binary-search:ID
```

## 7. Supported fact kinds

M2 adds exactly:

- `Unique`
- `SortedAscending`

Extraction is deliberately limited to `Int` record-table fields used by the current fixtures and backend. Exact extent, logical row order, nominal identity, and immutable publication remain authoritative M1 structural facts and are not duplicated as `StaticFact` kinds.

## 8. Proof extraction

While evaluating the condition of `StaticAssert.True`, the evaluator traces typed comparisons whose operands came from table row fields. Each operand carries exact subject, field ordinal/name, row index, and table extent.

The trace recognizes only:

- every unordered row pair compared unequal for one exact field: `Unique`;
- every adjacent pair compared without a descending result: `SortedAscending`.

Coverage must be complete. Merely seeing one comparison, a true helper result, a function named `AllIDsUnique`, or a field named `ID` emits nothing. The implementation never matches helper names. Failed assertions discard the active trace.

This is evaluation evidence, not backend inspection: the compiled-data backend receives typed facts and never sees helper AST or names.

## 9. StaticAssert relationship

`StaticAssert.True`, `False`, `Equal`, `Near`, and `Error` retain their existing behavior. Ordinary assertions validate and disappear. Only `StaticAssert.True` temporarily enables the bounded field-comparison collector; it emits facts only after the condition succeeds and recognized coverage is complete.

No Oct syntax or library API changed. `StaticAssert` did not become a general proof language.

## 10. LayoutContract enrichment

Compiled-data first derives the M1 contract from the validated typed value. It then accepts only facts whose source subject equals `Dataset.ProofSubject`, whose field subject matches that same subject, whose ordinal/name matches the dataset schema, and whose provenance is compile-time `StaticAssert` evaluation.

Accepted facts are rebound to the compiled-data root and promoted separately into:

```text
Invariants.UniqueFields
Invariants.SortedFields
```

The deterministic enrichment layer is upstream of backend planning.

## 11. Key-role restraint

M2 does not define or infer `PrimaryLookupKey`, entity identity, stable identity, or a public key role. `Unique` and `SortedAscending` remain independent invariants. Only their conjunction on the same `Int` field produces internal binary-search eligibility. The metadata describes an optimization candidate, not a primary key.

## 12. Backend consumer

The compiled-data backend consumes `LayoutContract.Metadata.SearchIndexes`. Eligibility requires:

```text
exact same field has Unique
and exact same field has SortedAscending
and field type is Int
and M1 static column projection exists
```

Without that metadata, the backend emits the M1 row array and scalar projections only. It does not inspect values for apparent sortedness and does not inspect source/helper names.

## 13. Generated representation

The proof-aware artifact reuses M1's static projected key column and adds one generated function:

```go
func CatalogDataLookupByID(key int) (CatalogRow, bool)
```

The function performs lower-bound binary search over `CatalogDataIDColumn` and returns the corresponding static row. It creates no map, index array, decoder, reflection path, `init`, append loop, unsafe operation, or runtime constructor.

`SearchIndexEligibility` records the derived-from subject, covered field, and proof basis `[Unique, SortedAscending]`. No universal relationship graph was added.

## 14. Correctness

Generated-source tests compile and execute the real emitted Go. They compare every row against every M1 projection and exercise lookup for:

- first key;
- middle key;
- last key;
- missing below range;
- missing between keys;
- missing above range.

The actual artifact fixture also emits the proof-aware catalog lookup through the production publication path.

## 15. Negative/invalidation tests

Coverage includes:

- duplicate IDs: an actual duplicate table makes the existing uniqueness helper return false; publication fails and `must-not-publish.go` is absent;
- unsorted IDs: an actual out-of-order table makes the existing sorted helper return false; publication fails and output is absent;
- same field name on another table: no fact or lookup crosses the subject boundary;
- old transformed subject: `with` creates a fresh subject, so proof for the old value cannot specialize the transformed value;
- missing proof: M1 projections remain, but no lookup is emitted;
- incomplete comparison coverage: no fact is emitted;
- fact/field subject or schema mismatch: compiled-data drops the fact.

M2 conservatively drops all facts across table `with`, including unrelated-column replacement. Re-proving the new value is cheap for current fixtures and safe. Key-column replacement and unknown transformations therefore cannot retain stale facts.

## 16. Benchmark results

Measurements were taken on Windows/amd64, AMD Ryzen 7 7700X, with `go test ./internal/compileddata -bench '^BenchmarkCatalogLookupScale$' -benchmem -benchtime=300ms`. M1 is a linear scan over the already-materialized projected ID column; M2 is the emitted binary-search strategy. The existing key is near the upper edge.

| Rows | Strategy | Existing lookup ns/op | Missing lookup ns/op | allocs/op | B/op |
| ---: | -------- | --------------------: | -------------------: | --------: | ---: |
| 1,000 | M1 projected linear | 217.0 | 215.3 | 0 | 0 |
| 1,000 | M2 proved binary | 4.385 | 3.152 | 0 | 0 |
| 10,000 | M1 projected linear | 2,025 | 1,992 | 0 | 0 |
| 10,000 | M2 proved binary | 5.572 | 4.670 | 0 | 0 |
| 100,000 | M1 projected linear | 19,866 | 19,652 | 0 | 0 |
| 100,000 | M2 proved binary | 7.251 | 6.220 | 0 | 0 |

Edge-key results were 220.5/4.262 ns at 1k, 2,008/5.690 ns at 10k, and 19,501/7.673 ns at 100k for M1/M2 respectively. Runtime initialization remains zero.

Generated source size is:

| Rows | M1 source bytes | M2 source bytes | M2 delta | Projected key data on amd64 |
| ---: | ---: | ---: | ---: | ---: |
| 1,000 | 108,226 | 108,570 | +344 | 8,000 bytes |
| 10,000 | 1,114,071 | 1,114,415 | +344 | 80,000 bytes |
| 100,000 | 11,532,476 | 11,532,820 | +344 | 800,000 bytes |

M2 adds no data structure: it reuses the M1 key projection. The 344-byte delta is the lookup function.

## 17. Compile/publication cost

The isolated 100k-row contract derivation/enrichment benchmark is 568.4 ns/op, 1,545 B/op, and 14 allocs/op. Three-iteration complete-emitter measurements were noisy but showed no material M2 cost beyond the fixed generated function: 9.66/9.00 ms at 1k, 82.20/84.56 ms at 10k, and 868.77/854.06 ms at 100k for M1/M2. Formatting the large generated literal dominates.

The proof collector plus coverage verification costs 32.1 µs and 6,192 bytes for the six-row-style case. At 1,000 rows, the existing all-pairs uniqueness helper/coverage shape costs 174.7 ms and 55.7 MB in the isolated benchmark. This is not claimed to scale to 100k: the existing uniqueness helper is quadratic, and M2 intentionally does not invent a theorem or new helper syntax to replace it. The 1k/10k/100k lookup measurements isolate the backend payoff once a fact has been proved.

## 18. Hash/semantic compatibility

Tests emit the same dataset through M1 projection-only and M2 proof-aware modes and assert equal logical hash, schema hash, and row count. Hashes are computed from canonical type/value before physical planning. Generated source bytes differ only by the lookup function.

## 19. Non-table sanity check

The M1 `PublishedIDs` static-array test still derives exact extent, logical order, and immutable publication, and still emits a fixed array. It has no nominal table identity, field proof, search index, or fake key. Optional `StaticFact` absence is valid.

## 20. What remains unproved

M2 does not prove primary-key role, entity identity, lookup intent, dense domain, bounded key range, foreign-key relationships, stability across transformations, arbitrary predicates, non-`Int` field order, or facts for empty/single-row tables. It also does not make the current quadratic uniqueness helper suitable for very large static catalogs.

The generated lookup is an internal compiled-data API choice. No compatibility promise is made for its name or representation.

## 21. Complexity assessment

The proof layer stayed bounded: two fact kinds, one provenance source, one subject-scoped field reference, one comparison-coverage collector, one enrichment path, and one backend consumer. The most substantial cost is explicit evidence tracking for all-pairs uniqueness, not a growing proof vocabulary. No parser, typechecker syntax, Concept, Octagon format, runtime format, or theorem infrastructure changed.

## 22. Final recommendation

**1. Keep StaticFact provenance narrow and add more backend consumers opportunistically**

## 23. What NOT to implement next

Do not add source-level key/constraint/proof syntax, helper-name recognition, arbitrary `StaticAssert` promotion, primary-key inference, entity identity inference, proof connectives, quantifiers, theorem dependencies, SMT, symbolic algebra, a general relationship graph, runtime index construction, or a direct dense index without a real domain proof. Do not generalize fact kinds merely to avoid the measured quadratic helper cost.

## 24. Exactly one next recommendation

Evaluate one additional backend consumer of the existing `Unique`/`SortedAscending` facts before adding any new fact kind.
