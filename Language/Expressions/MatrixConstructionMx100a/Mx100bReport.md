# Mx100b Report

## What was refactored

- Prometheus benchmark parser/compiler fixtures now construct benchmark matrices with `Matrix.fill(...)` instead of hard-coded matrix literals.
- Mechanics and dimensioned linear-algebra language examples were selectively updated where identity/diagonal matrix intent is clearer with the new construction surface (`Matrix.identity<...>`, `Matrix.tabulate(...)`).
- Existing benchmark integration fixture from Mx100a remains constructor-based (`Matrix.fill(...)`) for deterministic setup.

## Why these cases

These were chosen because they are benchmark/experiment-facing or repeated-diagonal setup patterns where constructor APIs reduce noise and better communicate intent.

## Docs updated

- `Language/reference/language/16-vectors-and-matrices.md` now documents constructor APIs, literal-vs-constructor guidance, shape accessors (`rows`, `cols`), and compiled parity for `m[r, c]`.
- `Language/reference/language/08-units.md` now shows a dimensioned matrix example using `Matrix.tabulate(...)`.

## What intentionally remained as literals

- Small 2x2 examples that are primarily teaching basic matrix arithmetic/indexing mechanics remain literals where they are still the clearest form.
- This milestone avoids broad mechanical rewrites and keeps scope focused on discoverability and high-value adoption sites.
