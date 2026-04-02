# Mechanics Continuum M27 — Authority-Modulated Constitutive Participation Probe

## Scope and constraints respected

M27 keeps the same fixed Cartesian circle fixture and remains fully local/cell-evaluable. It introduces no topology mutation, mesh generation, matrix assembly, PDE constitutive evolution, nonlinear solve loops, or global equilibrium solve machinery.

Field separation remains explicit:
- geometry carrier: fixed circle inclusion on lattice
- material-direction carrier: transported tangent/normal frame (with static check retained)
- authority/confidence carrier: independent local scalar `AuthorityField(i,j)`
- constitutive response object/rule: `ConstitutiveResponseField2D { TT, NN, TN, Probe, Mode }`

## 1) Constitutive baseline used from M26

M27 keeps M26’s strongest local constitutive semantic baseline unchanged:
- signed/off-axis local coupling semantics
- mixed-sign threshold asymmetry gate
- same coefficients: `TT=1.8`, `NN=0.2`, `TN=0.15`

Baseline formula retained:

`response_base = TT*abs(pt) + NN*abs(pn) + asymmetryScale*TN*(pt*pn)`

with the same mixed-sign gate from M26 for `asymmetryScale`.

## 2) Authority participation modes tested

M27 compares exactly three constitutive participation modes:

- **Path A — No authority in constitutive participation (control):**
  authority is present architecturally but excluded from constitutive response.

- **Path B — Scalar authority participation (whole-response weight):**
  `response = authorityWeight * (base + coupling)`

- **Path C — Coupling-specific authority participation:**
  tangent/normal baseline remains untouched; only coupling-sensitive term is authority-weighted:
  `response = base + authorityWeight * coupling`

Authority entry is explicit, local, and policy-like in each mode.

## 3) Probe set (same as M26)

- horizontal `(1,0)`
- vertical `(0,1)`
- diagonal `(1,1)/sqrt(2)`
- opposite diagonal `(1,-1)/sqrt(2)`

## 4) M27 evidence summary

### A. Signal vs noise
Yes—authority modulation adds measurable constitutive signal:
- no-authority vs scalar L1 is nonzero on all probe families
- no-authority vs coupling-specific L1 is nonzero on diagonal-family probes

### B. Where authority matters
Most visible differences appear in coupling-sensitive probes/regions:
- diagonal and opposite-diagonal probes separate scalar vs coupling-specific behavior most clearly
- boundary-local vs interior contrasts remain measurable, with scalar mode acting as blunter attenuation

### C. Coherence
Transported material direction remains coherent and useful:
- transported-frame diagonal variation is nonzero
- static-frame diagonal variation collapses

### D. Overreach
Scalar mode is too blunt for semantic preservation:
- it perturbs the whole response and drifts farther from control diagonal-family asymmetry
- coupling-specific mode better preserves constitutive diagonal-family semantics while still introducing authority signal

### E. Architectural cleanliness
Field separation survives:
- authority enters only via explicit policy pathways
- geometry/material/authority/constitutive objects remain conceptually distinct

## 5) Required blunt answers

1) **What constitutive baseline from M26 was used?**
- M26 off-axis signed local coupling semantics with mixed-sign gate and unchanged coefficients `TT=1.8`, `NN=0.2`, `TN=0.15`.

2) **What authority participation modes were tested?**
- none (control), scalar whole-response weighting, coupling-only weighting.

3) **Did authority modulation add meaningful constitutive signal?**
- Yes. It produced measurable local L1 differences relative to control.

4) **Was scalar authority too blunt?**
- Yes. It behaves mostly like broad attenuation and perturbs core material semantics more than needed.

5) **Was coupling-specific authority modulation better?**
- Yes. It provides interpretable local signal while preserving baseline tangent/normal response semantics.

6) **Which probes/regions exposed differences most clearly?**
- diagonal/opposite-diagonal probes and boundary-local/transition-sensitive regions.

7) **Did transported material direction remain useful?**
- Yes. Static frame still under-resolves local structure compared with transported direction.

8) **Did field separation survive?**
- Yes. Authority participation remained explicit and did not collapse geometry/material/authority distinctions.

9) **What is the next boundary after M27?**
- Keep locality and test slightly richer local tensor-like/signed constitutive coupling surfaces under coupling-specific authority modulation; defer first tiny global solve until local constitutive-authority interplay saturates.

## M27 verdict

Authority can now participate constitutively, but default inclusion should be **coupling-specific** rather than blanket scalar weighting. Authority is not just a carrier-composition field anymore when constrained to explicit local coupling modulation; whole-response scalar participation is generally too crude.
