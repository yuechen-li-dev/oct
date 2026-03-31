# Machina UI Coordinate Model — Typed Layout Space Probe

## Scope

This note is a **narrowing/design probe** for Machina UI coordinate typing. It does not redesign layout primitives and does not add responsive systems.

In current authoring (see Storefront M0), Machina UI already uses two semantic spaces:

- **Anchored / normalized space** for parent-relative macro layout (`0.01`, `0.99`, etc.)
- **Absolute / pixel-like space** for concrete repeated card placement (`320`, `232`, `158`, etc.)

The question is whether those spaces should remain plain numerics or become statically distinct types.

---

## Problem Statement

Current signatures are numeric and permissive:

- `AnchoredBox(left: Float, top: Float, right: Float, bottom: Float)`
- `AbsoluteBox(x: Float, y: Float, width: Float, height: Float)`

This allows accidental mixing like:

- giving `320` to anchored slots
- giving `0.27` to absolute widths

Both may be syntactically valid but semantically wrong. The model is workable, but ambiguous for readers and generators.

---

## Required Coordinate Split

### 1) Anchored Space (normalized)

Properties:

- parent/container-relative
- normalized range intent (`0..1`)
- used for macro region composition
- should not silently accept pixel values

### 2) Absolute Space (concrete)

Properties:

- concrete offsets/sizes
- used for dense repeated placement (card grids)
- pixel-like author intent
- should not silently accept normalized fractions

These spaces are **incompatible by default**.

---

## Candidate Comparison

### Candidate A — Status Quo (all Float)

Shape:

- `AbsoluteBox(Float, Float, Float, Float)`
- `AnchoredBox(Float, Float, Float, Float)`

Assessment:

- **Readability:** weak. Value space is inferred from call site only.
- **Mistake prevention:** weak. Typechecker cannot block cross-space mistakes.
- **Authoring clarity:** moderate at best; requires mental bookkeeping.
- **LLM clarity:** weak; ambiguous numeric literals increase wrong-slot placement.
- **Fit with Oct philosophy:** poor for "correct by construction".
- **Implementation burden:** none.

Verdict: easy now, expensive in long-term correctness.

---

### Candidate B — Raw Numeric Split (e.g., Absolute Int/Float, Anchored Float)

Shape:

- `AbsoluteBox(Int or Float, ...)`
- `AnchoredBox(Float, ...)`

Assessment:

- **Readability:** slightly better if `Int` is used for some absolute values, still ambiguous in mixed literals.
- **Mistake prevention:** still weak; numeric families remain easily mixed.
- **Authoring clarity:** marginal improvement only.
- **LLM clarity:** still unclear; generator must infer semantics from function names.
- **Fit with Oct philosophy:** better than A but still not explicit enough.
- **Implementation burden:** low.

Verdict: not enough separation to justify as final model.

---

### Candidate C — Fully Typed Split (preferred)

Shape:

- `AbsoluteBox(px, px, px, px)`
- `AnchoredBox(ui, ui, ui, ui)`

Example syntax direction:

- `320 px`
- `158 px`
- `0.27 ui`
- `0.99 ui`

Assessment:

- **Readability:** strong. Coordinate space is visible at literal level.
- **Mistake prevention:** strong. Cross-space misuse becomes compile-time type error.
- **Authoring clarity:** strong. Scanning/editing constants is straightforward.
- **LLM clarity:** strong. Unitized literals lower semantic ambiguity.
- **Fit with Oct philosophy:** strong match (explicit, narrow, correct-by-construction).
- **Implementation burden:** moderate (unit types, parsing/typechecker wiring, targeted diagnostics).

Verdict: best correctness-to-complexity ratio.

---

## Concrete Source-Level Illustration

Preferred typed authoring shape:

```oct
let HeaderTop = 0.01 ui
let HeaderBottom = 0.09 ui
let SupportTop = 0.80 ui
let FooterTop = 0.92 ui

let CardX0 = 320 px
let CardWidth = 232 px
let CardHeight = 158 px

Place(AnchoredBox(0.01 ui, HeaderTop, 0.99 ui, HeaderBottom), HeaderBar(model))
Place(AbsoluteBox(CardX0, 236 px, CardWidth, CardHeight), ProductCard(...))
```

Failure mode prevented by typing:

```oct
# should fail typecheck: ui passed where px is required
Place(AbsoluteBox(0.27 ui, 236 px, CardWidth, CardHeight), ProductCard(...))
```

This is the exact correctness boundary desired for Machina UI.

---

## Design Decisions (This Probe)

### 1) Should `AbsoluteBox` use `Int`, `Float`, or typed numeric?

**Decision:** typed numeric (`px`).

Reason: `Int`/`Float` alone cannot encode coordinate-space identity.

### 2) Should `AnchoredBox` use `Float` or dedicated normalized unit?

**Decision:** dedicated normalized unit (`ui`, name tentative).

Reason: anchored values are semantically not plain numeric magnitudes; they are normalized layout fractions.

### 3) Are spaces incompatible by default?

**Decision:** yes.

Reason: they model different coordinate domains with different intended invariants.

### 4) What conversion story is needed?

**Decision:** no implicit conversion; explicit conversion only if later proven necessary.

First pass should avoid general conversion APIs. If a bridge is required later, add one explicit function at a specific boundary.

### 5) Does typed model materially improve readability + error prevention?

**Decision:** yes, materially.

The improvement is immediate in literals, reviewability, and compiler-detectable misuse.

---

## Unit Naming Direction

Tentative naming:

- **`px`** for absolute layout coordinates/sizes
- **`ui`** for normalized anchored coordinates

Alternative normalized name (`frac`) is acceptable, but `ui` better signals "layout-space intent" rather than generic math fraction.

Recommendation: start with `ui`; rename only if ergonomics evidence appears in follow-up probes.

---

## Non-Goals Reconfirmed

This proposal does **not** include:

- CSS-like unit families (`em/rem/vh/vw/...`)
- responsive framework design
- implicit coercions between layout spaces
- redesign of `AnchoredBox`, `AbsoluteBox`, `Row/Column`, `Canvas/Place`
- broad unit conversion infrastructure

---

## Implementation Recommendation

**Recommend implementing typed coordinate spaces now** (small, focused pass), because candidate C yields clear correctness and readability wins with constrained scope.

### Suggested first implementation slice

1. Add unitized numeric support sufficient for `px` and `ui` literals.
2. Update Machina UI box signatures to require the corresponding unit types.
3. Add focused diagnostics for cross-space misuse.
4. Migrate Storefront M0 coordinate constants to unitized form as proof of authoring clarity.
5. Add language contracts covering valid usage and invalid cross-space mixing.

This keeps the change narrow while directly testing the architectural claim.
