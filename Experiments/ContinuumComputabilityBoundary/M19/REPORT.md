# Continuum Computability Boundary — M19

## Scope

M19 probes whether **material orientation** on the fixed Cartesian lattice is just a static label or needs its own explicit spatial organization/transport law.

Held fixed from M18:
- same circle fixture on the same fixed `6x6` Cartesian lattice
- same narrow-band SDF geometry cue
- same boundary-tangent geometry carrier
- same coupling worldview: geometry carrier + material carrier + authority/confidence field
- same one-step orientation-weighted diffusion-like downstream consequence

Not introduced:
- mesh/topology mutation
- PDE solve or optimization framework
- convergence loop
- constitutive anisotropy/stiffness tensors

## Material-direction paths compared

1. **Path A — Static global material field**
   - Horizontal everywhere (`Ox=1, Oy=0, strength=1`).
   - Control baseline.

2. **Path B — Region-tagged material field**
   - Left half horizontal, right half vertical.
   - Deterministic piecewise-static map with an internal seam.

3. **Path C — Seeded transported material field**
   - Two interior seeds: one horizontal, one vertical.
   - Deterministic local neighbor-averaging transport on the same lattice.
   - Fixed `8` sweep passes, no solve, no convergence target.

## Required comparisons

### A. Interior coherence
Path C produced the strongest interior coherence across deep-interior cells. Path A stayed coherent but trivial (single direction everywhere). Path B dropped coherence at the forced interior seam.

### B. Internal seam behavior
Path B was the most brittle: the hard left/right tag split creates the strongest non-geometry internal seam artifact.

### C. Coupling compatibility
All three paths were evaluated under the same explicit confidence coupling. Path C was most compatible with this worldview because it supplies a spatially organized material carrier instead of a global or hard-partition label.

### D. Need for transport
Transport is not cosmetic in this probe: Path C yields better interior organization while reducing seam brittleness versus Path B and avoiding Path A's oversimplification.

## Blunt answers to M19 required questions

1. **What three material-direction paths were tested?**
   - Static global, region-tagged, seeded transported.
2. **Was static global material direction sufficient?**
   - No. It is stable but too coarse as an interior material-direction model.
3. **How did region-tagged material direction behave?**
   - It added heterogeneity, but mostly by injecting brittle non-geometry seams.
4. **How did seeded material transport behave?**
   - It produced deterministic spatial organization from interior seeds and smoother transitions.
5. **Which path produced the best interior coherence?**
   - Path C (seeded transported).
6. **Which path produced the worst internal seam behavior?**
   - Path B (region-tagged).
7. **Does material direction now appear to need its own transport/organization law?**
   - Yes. Evidence favors explicit transport/organization over static labeling.
8. **What is the next boundary after M19?**
   - Couple transported material direction with richer authority/confidence maps before entering constitutive anisotropy.

## M19 verdict

Material direction should now be treated as a **real field carrier** with explicit local organization/transport, not as a static annotation.
