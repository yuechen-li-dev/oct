# Mechanics / Continuum M24 report

## Scope

M24 introduces a **tiny local tensor-like constitutive response object** on a fixed 6x6 lattice with circle inclusion, while preserving:

- fixed topology
- local evaluation only
- field separation (geometry, transported direction, authority, constitutive object)
- no solver/PDE/global assembly

## Response object added

M24 uses:

- `R.tt`
- `R.nn`
- `R.tn`

with the deliberate current policy:

- `R.tn = 0.0`

Evaluation in the local `(t, n)` frame:

- `pt = dot(p, t)`
- `pn = dot(p, n)`
- `response = R.tt * |pt| + R.nn * |pn|`

## Constitutive paths compared

- **Path A (isotropic):** `|p|`
- **Path B (M23 scalar axis split):** `a_t * |pt| + a_n * |pn|`
- **Path C (M24 response object):** `R.tt * |pt| + R.nn * |pn|`, `R.tn = 0`
- **Path D (optional static-direction comparison):** Path C with static `(t, n) = ((1,0), (0,1))`

Probes are exactly:

- horizontal `(1,0)`
- vertical `(0,1)`

## Results

### A) Structural gain

Yes. M24 gives clearer constitutive structure than M23 because coefficients are grouped as a local response object instead of free scalars.

### B) Behavioral equivalence vs extension

With `R.tn = 0`, M24 is **behaviorally equivalent** to M23 (L1 difference is zero for both probes). So current pass is primarily structural, not yet a behavior unlock.

### C) Spatial coherence

Transported material direction still produces coherent spatially varying response on the circle fixture for both probes.

### D) Static inadequacy

Static direction collapses spatial variation in this setup (near-zero variation field), so it is less informative than transported direction under the same constitutive object.

### E) Architectural cleanliness

Architecture held. Field roles remained separate and explicit.

### F) Need for authority

Authority still did not need to enter constitutive participation. It remains present as a separate field only.

## Blunt boundary answers

1. **Did tensor-like structure add beyond M23?**
   - Structurally yes; behaviorally no (with `tn = 0`).
2. **Refactor or capability unlock?**
   - Refactor-level structure gain in M24; capability unlock deferred.
3. **Transported direction still essential?**
   - Yes.
4. **Static direction degrade further?**
   - Yes; it suppresses spatial informativeness under this probe.
5. **Authority needed now?**
   - No.
6. **Architecture still hold?**
   - Yes.

## Next boundary recommendation

Next most direct boundary is:

1. **Introduce tiny off-diagonal coupling (`tn != 0`)** while staying local and fixed-topology.

Then, only after observing whether coupling adds real signal:

2. probe authority-modulated constitutive participation.
3. postpone any global solve boundary until local constitutive signal is exhausted.
