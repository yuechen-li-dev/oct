# Continuum Computability Boundary — M17

## Carrier coupling probe scope

M17 isolates coupling between two anisotropy carriers on the same fixed Cartesian lattice:

- geometry-side boundary orientation from narrow-band SDF tangents
- material-side interior orientation from explicit material field(s)

No topology changes, no mesh changes, no solver machinery, and no constitutive-model expansion were introduced.

## Coupling policies tested

1. **Path A — Hard handoff**
   - `abs(sdf) <= bandWidth` uses geometry tangent orientation.
   - Else uses material orientation.
2. **Path B — Smooth distance blend**
   - `w = clamp01(1 - abs(sdf)/blendWidth)`.
   - `O = normalize(w * O_geom + (1 - w) * O_mat)`.
3. **Path C — Authority/confidence coupling**
   - geometry authority from boundary proximity
   - material authority from explicit material strength
   - winner-takes-most outside tie zone, authority-weighted blend inside tie zone

## Material field(s)

- Primary: constant horizontal field (`Ox=1, Oy=0, Strength=1`).
- Sensitivity check: constant vertical field (`Ox=0, Oy=1, Strength=1`).

## Shared downstream consequence

All policies were compared with one identical consequence:

- orientation-driven edge weighting
- one deterministic diffusion-like local update
- one step only

## Blunt findings

- **Most brittle seam behavior:** Path A (hard handoff).
- **Smoothest seam behavior:** Path B (distance blend).
- **Most conceptually honest carrier treatment:** Path C (authority/confidence), because it explicitly models local ownership rather than relying only on geometric distance.
- **Boundary fidelity winner (near-interface tangency):** geometry-aware paths; hard and authority maintain stronger boundary ownership than material-only regions.
- **Interior coherence winner (material alignment):** blend/authority outperform hard handoff away from boundary.

## Interpolation vs authority verdict

M17 evidence supports:

- interpolation helps smooth transitions,
- but **coupling is fundamentally an ownership/authority question** when two distinct carriers coexist.

## Next boundary after M17

Keep Cartesian topology fixed and pressure **interior material transport / heterogeneous confidence fields** next, before moving to constitutive anisotropy or topology changes.
