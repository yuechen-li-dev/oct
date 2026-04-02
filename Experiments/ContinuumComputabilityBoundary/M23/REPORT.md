# Continuum Computability Boundary — M23

## Local Normal/Tangent Constitutive Split Probe

M23 asks whether explicit local constitutive axes (normal/tangent) add useful structure beyond M22 while preserving the fixed-lattice, field-separated architecture.

Scope remained strict:
- same fixed `6x6` Cartesian lattice
- same circle/SDF fixture continuity from M21/M22
- deterministic, cell-local evaluation only
- no topology mutation, no mesh machinery, no global solve, no matrix assembly

## 1) Explicit normal/tangent constitutive rule introduced

M23 rule (Path C):

`response = a_tangent * abs(dot(t, p)) + a_normal * abs(dot(n, p))`

where:
- `t` = local material tangent (from transported material-direction field)
- `n = perp(t)` = local normal
- `p` = probe direction
- `a_tangent = 1.8`
- `a_normal = 0.2`
- `a_tangent != a_normal`

This is an explicit constitutive axis split rather than generic directional channel naming.

## 2) How it differs from M22 richer rule

M22 comparison rule (Path B):

`response = a_longitudinal * abs(dot(m, p)) + a_transverse * abs(dot(perp(m), p))`

with:
- `a_longitudinal = 1.5`
- `a_transverse = 0.5`

M23 difference:
- explicit constitutive framing in terms of **tangent/normal axes** (`t`, `n`)
- stronger axis asymmetry (`1.8 / 0.2` vs `1.5 / 0.5`) to stress axis-split consequences

## 3) Isotropic vs M22 vs M23 comparison

Constitutive paths:
- **Path A**: isotropic baseline
- **Path B**: M22 richer directional rule with transported material direction
- **Path C**: M23 explicit normal/tangent split with transported material direction
- **Path D (optional, included)**: M23 explicit split with static horizontal direction

Probe directions were kept identical to M21/M22:
- horizontal `p = (1, 0)`
- vertical `p = (0, 1)`

Observed probe-level outcomes:
- `A_vs_B_L1 > 0` and `A_vs_C_L1 > 0`: isotropic remains insufficient.
- `B_vs_C_L1 > 0`: M23 is measurably different from M22.
- `AlongAcrossContrastC > AlongAcrossContrastB`: explicit axis split increases directional contrast.
- `C_vs_D_L1 > 0`: static direction produces a measurably different (and cruder) constitutive field.

## 4) Required boundary answers (M23)

### A. Structural gain over M22?
**Yes.** Explicit normal/tangent axis framing is cleaner and increases directional contrast relative to M22’s generic richer channels.

### B. Spatial coherence under transported material direction?
**Yes.** Transported field remains spatially coherent (`SpatialCoherenceTransported >= 0.85`).

### C. Static-direction inadequacy under explicit axis structure?
**Yes, more obvious.** Transported vs static M23 responses differ (`C_vs_D_L1 > 0`) and transported direction remains more boundary-compatible.

### D. Architectural cleanliness?
**Yes.** Geometry carrier, material-direction carrier, authority/confidence carrier, and constitutive rule remain separate.

### E. Need for authority in constitutive participation?
**No, still not yet.** Authority remains present in architecture but not required by this constitutive split.

## 5) Blunt verdict

M23 succeeds: explicit local normal/tangent constitutive splitting adds meaningful structure over M22, transported material direction remains worth carrying, static direction is more clearly inadequate, and field separation survives unchanged.

## 6) Next boundary after M23

Based on M23 evidence, the next honest boundary is one of:
- a tiny **local tensor-like constitutive response** (still direct and cell-local), or
- a tightly scoped **authority-modulated constitutive participation** probe,

while still deferring any first tiny global solve until local constitutive structure is saturated.
