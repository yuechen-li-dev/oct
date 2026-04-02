# Continuum Computability Boundary — M22

## Richer Local Constitutive Anisotropy Probe

M22 asks whether the field-separated architecture still holds when local constitutive anisotropy gains internal structure beyond the M21 scalar alignment multiplier.

Scope remains intentionally strict:
- fixed `6x6` Cartesian lattice
- same circle/SDF fixture continuity from M21
- deterministic local cell-wise evaluation only
- no topology edits, no mesh machinery, no global solve, no matrix assembly

## Richer rule introduced (Path C)

M22 adds a two-channel local directional constitutive rule:

`response = a_longitudinal * abs(dot(m, p)) + a_transverse * abs(dot(t, p))`

where:
- `m` = transported local material direction
- `t = perp(m)` = local perpendicular direction
- `p` = probe direction
- `a_longitudinal = 1.5`
- `a_transverse = 0.5`
- `a_longitudinal > a_transverse`

This keeps scalar output but introduces explicit internal directional channels.

## Difference from M21 scalar rule (Path B)

M21 scalar comparison rule retained:

`response = base * (1 + alpha * abs(dot(m, p)))`

with `base = 1.0`, `alpha = 0.6`.

M22 difference:
- M21: one directional alignment channel
- M22: two directional channels (longitudinal + transverse) with explicit asymmetric weights

## Probe directions (unchanged from M21)

Exactly the same two probes:
- horizontal: `p = (1, 0)`
- vertical: `p = (0, 1)`

No extra loading cases were introduced.

## Constitutive paths compared

- **Path A**: isotropic baseline (`response = base`)
- **Path B**: M21 scalar anisotropy using transported material direction
- **Path C**: M22 richer anisotropy using transported material direction
- **Path D (comparison-only)**: M22 richer anisotropy with static horizontal material direction

Transported material direction remains the primary constitutive input for the main comparison paths.

## Required M22 answers

1. **Structural usefulness — does richer rule reveal what M21 flattened?**
   - **Yes.** Path C differs measurably from Path B (`B_vs_C_L1 > 0`) and produces stronger directional contrast (`AlongAcrossContrastC > AlongAcrossContrastB`).

2. **Spatial coherence under transported direction?**
   - **Yes.** Transported material field remains spatially coherent in the interior (`SpatialCoherenceTransported >= 0.85`).

3. **Static direction inadequacy under richer constitutive model?**
   - **Yes (clearer than M21).** Richer transported vs richer static responses differ (`C_vs_D_L1 > 0`), and transported direction remains better boundary-compatible.

4. **Architectural cleanliness under richer constitutive contact?**
   - **Yes.** Geometry carrier, material-direction carrier, authority/confidence carrier, and constitutive rule stay explicitly separated.

5. **Did authority/confidence need to enter constitutive participation?**
   - **No, not yet.** Authority stays present and separate; M22 evidence did not force constitutive coupling.

## M22 verdict

The architecture survives richer local constitutive contact: the two-channel rule adds meaningful anisotropic constitutive structure while preserving locality and field separation. Transported material direction remains worth carrying as constitutive input, and static direction is more visibly too crude under the richer rule.

## Next boundary after M22

Stay local but increase constitutive directional expressiveness one notch:
- explicit local normal/tangent constitutive split (tensor-like two-axis local response surface)
- then test whether authority should modulate constitutive participation
- still defer any first tiny global solve until local constitutive structure saturates
