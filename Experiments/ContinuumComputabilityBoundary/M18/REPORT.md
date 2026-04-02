# Continuum Computability Boundary — M18

## Scope

M18 probes whether geometry/material coupling should be represented by a **spatial authority/confidence field** instead of one fixed global authority rule.

Kept fixed by design:
- same circle fixture and same `6x6` Cartesian lattice
- same narrow-band SDF geometry cue
- same boundary-tangent geometry orientation
- same material orientation concept (constant horizontal)
- same one-step orientation-weighted diffusion-like downstream consequence

Not introduced:
- topology change
- mesh/graph mutation
- solver machinery
- constitutive anisotropy

## Authority strategies tested

1. **Path A — Fixed authority baseline**
   - Geometry owns `abs(sdf) <= dx`.
   - Material owns everything else.
   - One global rule everywhere.

2. **Path B — Geometry-proximity authority**
   - Geometry confidence: `clamp01(1 - abs(sdf)/authorityWidth)`.
   - Material confidence fixed globally at `0.55`.
   - Local blend/winner behavior uses geometry proximity only.

3. **Path C — Explicit heterogeneous confidence field**
   - Geometry confidence from SDF proximity (same as Path B).
   - Material confidence from explicit spatial map:
     - left half high (`0.90`)
     - right half low (`0.25`)
   - Local winner/weighted tie blend by confidence comparison.

## Shared downstream consequence

All paths use one identical consequence:
- orientation-driven edge weighting
- one deterministic diffusion-like local update
- one step only

## Required comparison answers

### A) Seam behavior
Yes. Heterogeneous confidence changes seam behavior relative to fixed authority, and differs from geometry-proximity-only authority.

### B) Boundary fidelity
Geometry still dominates near the boundary for all three paths. The strongest boundary fidelity remains geometry-led strategies.

### C) Interior coherence
Interior coherence changes across paths; heterogeneous confidence can preserve material alignment better in high-material-confidence regions than a single fixed rule.

### D) Spatial honesty
A spatial confidence field is more honest than one global authority rule because authority is visibly local, inspectable, and heterogeneous rather than hidden in a universal handoff.

## Blunt verdicts

1. **Was fixed authority sufficient?**
   - No, not as a general coupling policy.
2. **Did geometry-proximity authority help?**
   - Yes, compared with a hard global handoff.
3. **Did explicit heterogeneous confidence behave better?**
   - Yes; it exposes and controls region-dependent authority explicitly.
4. **Does coupling now require a third field?**
   - For this probe, yes: coupling is best described as geometry carrier + material carrier + authority/confidence field.
5. **Next boundary after M18**
   - Keep topology fixed and pressure richer material confidence maps plus interior material-orientation transport before constitutive anisotropy.
