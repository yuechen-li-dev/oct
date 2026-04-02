# Continuum Computability Boundary — M20

## Scope

M20 probes whether a **transported material-direction field** and a **heterogeneous authority/confidence field** compose cleanly, or drift into hidden coupling.

Kept fixed from M18 + M19:
- same circle fixture on the same fixed `6x6` Cartesian lattice
- same narrow-band SDF geometry cue
- same boundary-tangent geometry carrier
- same one-step orientation-weighted downstream update

Not introduced:
- topology/mesh mutation
- solver loops or convergence machinery
- constitutive anisotropy/stiffness tensors

## Independent field construction

1. **Material-direction carrier (independent baseline)**
   - M19 best case only: seeded transported material field.
   - Two deterministic seeds, deterministic local neighbor propagation, fixed 8 passes.
   - No authority used in this baseline transport construction.

2. **Authority/confidence carrier (independent)**
   - Geometry confidence from SDF proximity (with floor).
   - Material confidence from explicit heterogeneous map (left-half high, right-half low).
   - Built without reading material-direction values.

## Interaction strategies

### Path A — Decoupled interaction
- Material transport is independent.
- Authority field is independent.
- Authority only scales downstream usage strength after material field exists.
- No feedback into transport.

### Path B — Authority-modulated material usage
- Material transport remains independent.
- Authority controls per-cell weight of material contribution during geometry/material composition.
- No feedback into transport.

### Path C — Authority-influenced transport (controlled)
- Authority modulates local propagation weights during material transport itself.
- Still local-only, deterministic, fixed-pass, no solve/no convergence.
- Downstream consequence remains identical.

## Required comparison outcomes

### A) Field independence
- A and B remain cleanly separable by construction (material field unchanged from baseline).
- C is intentionally non-separable (controlled coupling) because authority changes transport.

### B) Stability
- All paths stayed deterministic and interpretable.
- A/B tend to be the most stable because interaction is explicit and one-way.
- C can improve targeted shaping but is more coupling-sensitive.

### C) Boundary fidelity
- Geometry remains dominant near the SDF band across all paths.
- No strategy displaced geometry as boundary owner.

### D) Interior coherence
- A and B preserve baseline transported interior coherence.
- C changes interior coherence in a measurable way (sometimes beneficial, but coupling-dependent).

### E) Interaction honesty
- **Most honest default:** Path B.
  - Still field-separable.
  - Explicit local usage control.
  - No hidden feedback loops.
- Path C is useful as an optional escalation, but it is explicit coupling, not clean separation.

## Blunt answers to required M20 questions

1. **How were material and authority fields constructed independently?**
   - Material: seeded deterministic transport (fixed passes) with no authority input.
   - Authority: independent SDF-proximity + heterogeneous map, with no material-direction input.
2. **What interaction strategies were tested?**
   - Decoupled downstream-only (A), authority-modulated usage (B), authority-influenced transport (C).
3. **Did decoupled fields (Path A) behave sufficiently well?**
   - Yes, stable and interpretable.
4. **Did authority-modulated usage (Path B) improve behavior?**
   - Yes, it gives more local control without sacrificing separability.
5. **Did feedback into material transport (Path C) help or destabilize?**
   - It helped in some interior shaping cases but increased coupling sensitivity.
6. **Do material and authority remain cleanly separable?**
   - Yes in A/B; intentionally no in C.
7. **Which interaction model is most honest?**
   - Path B for default composition honesty.
8. **What is the next boundary after M20?**
   - Introduce constitutive anisotropy using transported material direction while preserving explicit field-separation contracts and optional coupling policy.

## M20 verdict

Transported material direction and heterogeneous authority **can compose cleanly** (A/B) without hidden coupling rules. Feedback coupling (C) is optional and should remain explicit, local, and policy-controlled.
