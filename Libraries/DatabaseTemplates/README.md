# DatabaseTemplates

This package is the canonical M0 catalog of typed semantic specialization patterns. Every `*.template.oct` file is ordinary Oct source. The suffix is a discovery and documentation convention; it does not select another parser, compiler, or runtime. Consumers import both `DatabaseTemplates` and `DatabaseTemplateContracts`; the companion package supplies erased refined Concepts for positive counts, publication identity, and publication version.

Use templates only after profiling the normal OctetDB Go application:

1. Build and profile default mode.
2. Identify a measured hot path and classify its workload.
3. Inspect `oct templates list Libraries/DatabaseTemplates` and `oct templates describe <Name> Libraries/DatabaseTemplates`.
4. Select a small explicit composition.
5. Fill exact type arguments and typed selectors; use `with` for application values admitted by the declared Concepts.
6. Compile to ordinary concrete Oct and generated Go.
7. Compare correctness, ordering, early stopping, allocations, and throughput against default and bespoke controls.
8. Keep the specialization only when its measured ROI is positive.

## Catalog shape

- Semantic primitives: `StableIdentity`, `BoundedExtent`, `RebuildPublication`.
- Reusable patterns: `BoundedKeyedDataset`, `ReadMostlyDataset`, `MaterializedFilter`, `FiniteStateDataset`, and `FilteredView`.
- Application starting points: `JobQueue` and `Inventory`.

The category names `SemanticPrimitive`, `SpecializationPattern`, `SpecializationQuery`, and `ApplicationStartingPoint` are refined Concepts in the catalog source, not free-form tooling strings. Discovery verifies their `Require` declarations. Every documented requirement also identifies whether it is enforced by `Require`, exact typing, structural composition, or the application publication/lifecycle boundary.

The maximum depth is application starting point → reusable pattern → semantic primitive. There is no inheritance or implicit template search.

`ReadMostlyDataset` is not a decorative Boolean: it requires an explicit `RebuildPublication<Record>` callback/source version and records the workload assumption. `MaterializedFilter` is valid only for immutable sources or sources rebuilt at an application-owned publication boundary. It does not provide transactional secondary-index maintenance.

No `EventDedupeDataset` is published in M0. OctetDB already owns exact durable command deduplication and efficient point access; the measured webhook shape is synchronization/point-access dominated, so adding a read materialization template would be cargo-cult specialization.

Compile-time-known zero/negative bounds or limits, empty publication identities, and negative publication versions are rejected by existing Concept/Require admission. Runtime-derived values use the companion package's fallible `AdmitPositiveCount`, `AdmitPublicationSource`, and `AdmitPublicationVersion` APIs; templates add no constraint engine.

These templates express semantic intent. They do not promise a physical layout, cache behavior, allocation count, or throughput result. Compiler/backend mechanism remains compiler-owned.
