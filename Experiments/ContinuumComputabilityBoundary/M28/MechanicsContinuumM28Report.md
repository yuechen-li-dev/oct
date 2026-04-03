# Mechanics Continuum M28 — Richer Local Coupling Surface Probe

## Scope and constraints respected

M28 keeps the fixed Cartesian circle fixture and remains local/cell-evaluable. It adds no mesh generation, topology mutation, matrix assembly, PDE constitutive evolution, nonlinear loops, or global solve machinery.

Field separation remains explicit:
- geometry carrier: fixed circle mask on Cartesian lattice
- material-direction carrier: transported tangent/normal frame (with static-frame check)
- authority/confidence carrier: independent local scalar `AuthorityField(i,j)`
- constitutive response surface: explicit local coefficients in `ConstitutiveResponseField2D`

## M28 richer local constitutive surface

M27 control (Path A) is preserved with coupling-specific authority modulation on the M27 signed/off-axis coupling term.

M28 richer surface introduces two local coupling channels:
- `TNSame` for `pt*pn >= 0`
- `TNMixed` for `pt*pn < 0`

Using local frame projections `pt = dot(p,t)` and `pn = dot(p,n)`:

- `base = TT*abs(pt) + NN*abs(pn)`
- `coupling = TNSame*abs(pt*pn)` for same-sign, else `TNMixed*abs(pt*pn)`
- with coupling-specific authority: `response = base + authorityWeight*coupling`
- without authority: `response = base + coupling`

Coefficients used are explicit and small relative to `TT`:
- `TT=1.8`
- `NN=0.2`
- `TNSame=0.12`
- `TNMixed=0.05`

## Paths compared (required)

- **Path A** — M27 best baseline: signed/off-axis local semantics + coupling-specific authority modulation
- **Path B** — M28 richer coupling split + coupling-specific authority modulation
- **Path C** — M28 richer coupling split + no authority modulation

## Probe set (unchanged)

- horizontal `(1,0)`
- vertical `(0,1)`
- diagonal `(1,1)/sqrt(2)`
- opposite diagonal `(1,-1)/sqrt(2)`

## Evidence summary

### A) Richness vs noise
The richer local coupling surface adds interpretable signal beyond M27 (nontrivial A-vs-B L1 differences), and the strongest separation remains in diagonal-family probes.

### B) Authority value-add
With the richer surface fixed, Path B vs Path C remains measurably nonzero; authority still contributes meaningfully when constrained to coupling-only modulation.

### C) Probe sensitivity
Diagonal and opposite-diagonal probes remain dominant revealers for both A-vs-B and B-vs-C comparisons.

### D) Spatial coherence
Transported material direction remains useful: transported-frame variation is nonzero while static-frame variation collapses.

### E) Architectural cleanliness
Field separation survives with explicit, narrow authority participation in only the coupling-sensitive part.

## Required blunt answers

1) **What richer local coupling surface was introduced?**
- A two-channel local coupling split: same-sign (`TNSame`) vs mixed-sign (`TNMixed`) using `abs(pt*pn)`.

2) **How did it differ from the M27 constitutive baseline?**
- M27 used one signed/off-axis coupling channel (`TN*(pt*pn)` with mixed-sign threshold asymmetry); M28 uses two explicit same/mixed channels while keeping tangent/normal base terms unchanged.

3) **Did it add meaningful signal?**
- Yes. A-vs-B differences are measurable and diagonals reveal the largest changes.

4) **Did coupling-specific authority modulation still help?**
- Yes. B-vs-C is measurably nonzero under the same richer surface.

5) **Which probes exposed the difference most clearly?**
- Diagonal and opposite-diagonal probes.

6) **Did transported material direction remain useful?**
- Yes. Static frame under-resolves local structure compared with transported frame.

7) **Did field separation survive?**
- Yes. Geometry, material direction, authority, and constitutive surface remain distinct fields.

8) **What is the next boundary after M28?**
- Either (a) first tiny global consistency pass while keeping constitutive map explicit, or (b) one stronger local signed constitutive map; still no full tensor or solver framework expansion yet.

## M28 verdict

M28 succeeds: a one-notch richer local coupling surface (`same` vs `mixed`) stays interpretable, remains local and field-separated, and still benefits from coupling-specific authority modulation. Diagonal-family probes continue to be the principal revealers, and transported material direction remains worth keeping.
