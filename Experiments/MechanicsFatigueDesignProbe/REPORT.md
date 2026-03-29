# Mechanics Fatigue Design Probe Report

## 1. Goal

This experiment probes whether the existing Mechanics stack (M0 stress helpers, M0a fatigue decomposition and criteria, and M1 endurance modifiers) is already sufficient for a compact, deterministic fatigue-design workflow.

## 2. Stress case

The probe uses a small combined normal-stress scenario at a critical section:

- axial force range: 18 kN to 42 kN
- bending moment range: 250 N·m to 950 N·m
- section geometry: area = 0.0008 m^2, c = 0.012 m, I = 2.4e-7 m^4

M0 helpers are used directly:

- `NormalStress` for axial stress
- `BendingStress` for bending stress

Combined minimum and maximum normal stress are formed by summing axial and bending components at the same surface location.

## 3. Fatigue decomposition

From the combined stress extrema, M0a helpers compute:

- mean stress via `MeanStress`
- alternating stress via `AlternatingStress`

The chosen loading yields approximately:

- mean stress = 67.5 MPa
- alternating stress = 32.5 MPa

This confirms a clean transition from stress primitives to fatigue-input quantities.

## 4. Endurance-limit modification

The probe applies multiple M1 modifiers to an unmodified endurance limit of 310 MPa:

- surface factor from `SurfaceFactorFromUltimateStrength` (machined-style coefficients)
- size factor from `SizeFactorFromDiameter`
- load factor from `LoadFactorBending`
- temperature factor from `TemperatureFactorDirect`
- reliability factor from `ReliabilityFactorFromPercent(99.0)`

The modifier product is composed explicitly with `EnduranceModifierProduct` and then applied through `DesignEnduranceLimit`.

Resulting design endurance limit is about 171.7 MPa, reduced from baseline in a transparent, reproducible way.

## 5. Criterion comparison

Using one shared load case and one shared material/endurance assumption set (`Sut = 620 MPa`, `Sy = 450 MPa`, modified endurance from Section 4), the probe compares:

- Goodman (`GoodmanFactorOfSafety`)
- Gerber (`GerberFactorOfSafety`)
- Soderberg (`SoderbergFactorOfSafety`)

Observed factors of safety:

- Gerber ≈ 4.43
- Goodman ≈ 3.44
- Soderberg ≈ 3.01

Expected conservatism ordering appears clearly for this scenario:

`Gerber > Goodman > Soderberg`

## 6. Observations

What felt clean:

- unit-aware stress calculation and fatigue decomposition compose naturally
- endurance modifiers remain explicit and inspectable
- criterion comparison is concise while still design-relevant

What still feels missing (left intentionally visible):

- no notch sensitivity / stress-concentration handling in this probe
- no variable-amplitude loading or cycle counting
- no material database integration

Those omissions are deliberate non-goals for this experiment.

## 7. Conclusion

Yes—Mechanics M0 + M0a + M1 now compose into a real fatigue-design probe. The workflow is compact, reproducible, and sufficiently engineering-shaped to validate that the current library can support small end-to-end fatigue design studies without additional framework expansion.
