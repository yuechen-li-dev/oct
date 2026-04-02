# Continuum Computability Boundary — M21

## Constitutive Anisotropy Entry Probe

M21 asks one narrow question: can the transported material-direction carrier from M20 drive a first constitutive anisotropy response without collapsing the architecture into hidden bundled rules?

Scope held intentionally tiny:
- fixed `6x6` Cartesian lattice
- same circle/SDF fixture
- no topology change
- no global solve
- one local constitutive response multiplier rule

## Constitutive rule introduced

Directional scalar constitutive response multiplier:

`response = base * (1 + alpha * abs(dot(m, p)))`

where:
- `m` = local material direction from the material-direction carrier
- `p` = probe/loading direction
- `base = 1.0`
- `alpha = 0.6`

Interpretation:
- stronger response along local material direction
- weaker response across local material direction

## Probe directions used

Exactly two canonical probes:
- horizontal: `p = (1, 0)`
- vertical: `p = (0, 1)`

No additional load cases were introduced.

## Constitutive input paths compared

- **Path A — isotropic baseline**: `response = base`, no material-direction usage.
- **Path B — static material direction**: horizontal material direction everywhere.
- **Path C — transported material direction**: deterministic fixed-pass transported field from sparse seeds.
- **Path D (optional authority-modulated usage)**: intentionally not enabled in M21; authority remains separate and available.

## What changed in M21 (and what did not)

Changed:
- constitutive response now explicitly reads the material-direction field.

Did not change:
- geometry carrier construction
- authority/confidence carrier construction
- explicit policy separation
- topology or discretization regime
- any solver machinery

## Required answers

1. **Was isotropic response clearly insufficient?**
   - **Yes.** Isotropic path lacks along-vs-across directional contrast and differs measurably from anisotropic paths.

2. **Was static material direction too crude?**
   - **Yes.** Static horizontal orientation creates maximum fixed directional contrast but is spatially blunt and less boundary-compatible on the curved fixture.

3. **Did transported material direction improve constitutive coherence?**
   - **Yes.** Transported direction improved interior coherence and improved compatibility with boundary tangential geometry near the SDF band.

4. **Did authority/confidence need to enter constitutive usage already?**
   - **No (not yet).** M21 evidence is sufficient without authority entering constitutive participation.

5. **Does architecture survive first constitutive contact?**
   - **Yes.** Field separation stayed explicit and local constitutive anisotropy was added without hidden coupling.

6. **Next boundary after M21?**
   - Add one richer but still local anisotropic constitutive law (still no global solve), then pressure-test optional authority-modulated constitutive participation as an explicit policy layer.

## M21 verdict

Transported material direction is worth carrying into constitutive usage: it produces meaningful anisotropic response while preserving field-separated architecture and fixed-grid locality.
