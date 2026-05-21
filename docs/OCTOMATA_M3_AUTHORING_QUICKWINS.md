# Octomata M3 Authoring Quick-Wins (May 2026)

## Minimal flow-smoke lane (authoring checklist)

For new Octomata experiments, keep a fast smoke lane before artifact coupling:

1. Start with a tiny `flow` using **scalar board fields only** (`Bool`, `Int`, `Float`, `String`).
2. Validate `Complete()`, `Result()`, and `StateHistory()` in a small `[Fact]` test.
3. Only after smoke lane passes, add artifact writes (`[Artifact]` path).
4. Keep `[Artifact]` helpers separate from `[Fact]` / `[Theory]` flow-smoke tests.

This keeps flow-state debugging and artifact IO/debugging decoupled, so failures isolate quickly.

## Design follow-up: board arrays / accumulator shape

### Observed blocker

Current board field types reject arrays:

- supported: `Bool`, `Int`, `Float`, `String`
- unsupported: array board fields such as `Float[]`

This is a known design/runtime boundary and is **not changed in this quick-win pass**.

### Why it matters

Kalman-style M3 authoring needs history-like lanes (e.g., recovered/innovation arrays), which naturally push authors toward board arrays.

### Candidate design directions (deferred)

- **A. Allow scalar arrays on board**, e.g. `Float[]` board fields.
- **B. Keep board scalar-only** and route accumulators/history via external records/layers outside flow board.

### Questions to resolve before implementation

- dirty tracking and update diffs
- persistence/snapshot semantics
- trace compactness vs inspectability
- mutation/copy behavior of board-held arrays
- compiled-lane support posture and parity expectations
- artifact/report rendering conventions for array-shaped board history
