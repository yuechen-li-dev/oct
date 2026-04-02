# Continuum Computability Boundary — M25

## Tiny Off-Diagonal Coupling Probe

M25 asks whether a small nonzero off-diagonal constitutive coupling term introduces real new local constitutive behavior while preserving fixed topology, local evaluation, field separation, and no-solver discipline.

Scope stayed strict:
- same fixed `6x6` Cartesian lattice
- same circle/SDF fixture continuity
- deterministic, cell-local evaluation only
- no mesh generation, topology mutation, matrix assembly, PDE solve, or iterative nonlinear solve

## 1) Nonzero coupling value used

M25 used a single explicit local response object:

- `R.tt = 1.8`
- `R.nn = 0.2`
- `R.tn = 0.15`

with `R.tn` intentionally small relative to tangent channel stiffness (`0.15 << 1.8`) while still nonzero.

## 2) Coupling formula used

With local frame `t` (tangent), `n = perp(t)` (normal), and probe `p`:

- `pt = dot(p, t)`
- `pn = dot(p, n)`

M25 local scalar response:

`response = R.tt * abs(pt) + R.nn * abs(pn) + R.tn * abs(pt * pn)`

This remains local, scalar-output, and magnitude-based while introducing one explicit coupling channel.

## 3) Constitutive paths compared

- **Path A** — isotropic baseline
- **Path B** — M23 axis-split scalar constitutive rule
- **Path C** — M24 tensor-like object with `R.tn = 0`
- **Path D** — M25 tiny off-diagonal coupling with `R.tn = 0.15`
- **Path E (optional, included)** — M25 coupling with static horizontal material direction

Probe set:
- horizontal `p = (1, 0)`
- vertical `p = (0, 1)`
- diagonal `p = (1, 1)/sqrt(2)`

## 4) Required boundary answers (M25)

### A. Capability unlock
**Yes.** `R.tn != 0` produced measurable change (`C_vs_D_L1 > 0`, `B_vs_D_L1 > 0`). M25 is not behaviorally identical to M24/M23.

### B. Mixed-direction sensitivity
**Yes.** The diagonal probe carried the strongest coupling signal (`CouplingDiagonalL1 > CouplingHorizontalL1` and `CouplingDiagonalL1 > CouplingVerticalL1`).

### C. Spatial coherence under coupling
**Yes.** Transported material direction remained coherent (`SpatialCoherenceTransported >= 0.85`) and boundary-compatible.

### D. Static-direction inadequacy under coupling
**Yes, more obvious.** Static-vs-transported M25 difference stayed measurable (`D_vs_E_L1 > 0`) with transported compatibility remaining better.

### E. Architectural cleanliness
**Yes.** Geometry carrier, material-direction carrier, authority/confidence carrier, and constitutive response object remained separate.

### F. Need for authority
**No, still not yet.** Authority remained present but did not need to enter constitutive participation for this tiny coupling pass.

## 5) Blunt verdict

M25 succeeds: tiny off-diagonal coupling produced a real constitutive signal (not just structure), the mixed-direction probe exposed it most clearly, transported direction remained worth carrying, static direction became even more obviously crude, and architecture stayed clean without forcing solver-shaped complexity.

## 6) Next boundary after M25

Based on M25 evidence, the next honest boundary is:
- either a stronger local tensor-like response (still fixed-topology and local),
- or signed/off-axis coupling semantics,

while still deferring authority-modulated constitutive participation and any first tiny global solve until local constitutive signal saturates.
