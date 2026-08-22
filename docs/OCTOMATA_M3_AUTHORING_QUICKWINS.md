# Octomata M3 Authoring Quick-Wins (May 2026)

## Minimal flow-smoke lane (authoring checklist)

For new Octomata experiments, keep a fast smoke lane before artifact coupling:

1. Start with a tiny `flow` using supported board fields (`Bool`, `String`, `Int`/`Int<D>`, `Float`/`Float<D>`, or arrays of those scalar types).
2. Validate `Complete()`, `Result()`, and `StateHistory()` in a small `[Fact]` test.
3. Only after smoke lane passes, add artifact writes (`[Artifact]` path).
4. Keep `[Artifact]` helpers separate from `[Fact]` / `[Theory]` flow-smoke tests.

This keeps flow-state debugging and artifact IO/debugging decoupled, so failures isolate quickly.

## Design follow-up: board arrays / accumulator shape

### Current support

Current board field types accept scalar `Bool`, `String`, `Int`/`Int<D>`, and `Float`/`Float<D>`, plus arrays (including nested arrays) of those types. Records, enums, vectors, matrices, and other aggregates remain unsupported.

This support landed after the original quick-win pass; the older scalar-only note was stale.

### Why it matters

Kalman-style M3 authoring needs history-like lanes (e.g., recovered/innovation arrays), which naturally push authors toward board arrays.

### Candidate design directions (deferred)

- **A. Scalar arrays on board**, e.g. `Float[]`, are supported.
- **B. External records/layers** remain appropriate when accumulator state is not behavior-local flow state.

### Questions to resolve before implementation

- dirty tracking and update diffs
- persistence/snapshot semantics
- trace compactness vs inspectability
- mutation/copy behavior of board-held arrays
- compiled-lane support posture and parity expectations
- artifact/report rendering conventions for array-shaped board history
