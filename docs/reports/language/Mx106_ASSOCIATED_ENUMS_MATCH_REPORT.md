# Mx106 Report — Narrow Associated-Data Enums + `match` Payload Binding

## 1) Audit: current surface before this pass

### What enums could do before
- Enums were tag-only (`enum Name { A B }`) with no per-variant payload shape.
- Construction was limited to qualified tags (`Name.Variant`).
- Exhaustive enum branching existed through `switch`.

### Where enums were used before
- Existing tests and docs predominantly used enums as selectors/dispatch tags (`switch` over enum tags).
- Runtime/typechecker represented enum values as nominal `{type, variant}` pairs only.

### What `match` did before
- `match` existed as a fallible-handling form (`ok(...)` / `err(...)`) in error handling.
- It did not provide enum variant payload binding.

### Semantic gap closed by this pass
- Real closed-domain shapes like `Ok(value)/Err(message)` and `Some(value)/None` were impossible to express directly in enums.
- Authors had to use sentinel records/discriminators for non-exceptional alternatives.

### Narrow alternatives considered and why insufficient
- Keep tag-only enums + sentinel records: still duplicates state across discriminator + payload fields.
- Expand `switch` only: cannot bind payload values cleanly.
- Full ADT/pattern system now: too broad for this milestone and not required to close core modeling gap.

## 2) Feature shape selected
- Enum variants now support:
  - tag-only (`Variant`)
  - single-payload (`Variant(Type)`)
- Construction supports:
  - `Enum.Tag`
  - `Enum.PayloadVariant(value)`
- New enum `match` expression supports payload binding per arm:
  - `case Enum.PayloadVariant(v) => ...`
  - `case Enum.Tag => ...`
- Exhaustiveness is required for enum `match`.

## 3) Why one-payload first
- It directly unlocks the dominant closed-shape use cases (`Ok/Err`, `Some/None`) with minimal grammar/runtime/typechecker expansion.
- It avoids overcommitting to generalized pattern matching semantics.

## 4) `match` vs `switch`
- `switch`: value/literal dispatch and tag-level enum dispatch.
- `match`: variant-sensitive enum analysis with payload binding.

## 5) Intentional out-of-scope
- Multi-field payloads.
- Tuple/record/array destructuring patterns.
- Nested patterns, guards, and broad pattern language features.

## 6) Compatibility
- Existing tag-only enum construction and `switch` behavior remain supported.
- Existing fallible `match` statement form (`ok/err`) remains supported.

## 7) Explicit consistency notes
- `Language/reference/language/12-enums.md` and `04-control-flow.md` now describe enum payload + enum `match` roles.
- `Language/reference/language/06-errors.md` still documents fallible `match`; this is intentionally called out as a separate form to avoid ambiguity with enum `match`.
