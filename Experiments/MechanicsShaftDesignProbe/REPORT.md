# Mechanics Shaft Design Probe Report

## 1. Goal

This experiment checks whether the current Mechanics stack (M0 → M2) can express a compact, deterministic, unit-aware shaft critical-section workflow that includes both static and fatigue reasoning.

## 2. Shaft section baseline

A single solid circular shaft section is used:

- diameter: 32 mm
- section properties from M2 helpers:
  - `ShaftAreaSolidCircular`
  - `ShaftSecondMomentAreaSolidCircular`
  - `ShaftPolarMomentAreaSolidCircular`
- baseline loads:
  - bending moment = 280 N·m
  - torque = 160 N·m

The section baseline demonstrates that geometry and stress quantities are concise to compute and inspect directly.

## 3. Static check

The static critical-section check uses M2 shaft helpers with explicit concentration factors:

- `Kt = 1.65`
- `Kts = 1.40`

Computed values include:

- concentrated outer-surface bending stress
- concentrated outer-surface torsional shear stress
- combined static equivalent stress (`ShaftCombinedStaticEquivalentStress`)
- yield factor of safety (`ShaftCombinedStaticYieldFactorOfSafety`)

The probe verifies expected behavior: increasing load reduces static factor of safety.

## 4. Fatigue setup

A fluctuating load case is applied to the same shaft section:

- bending moment range: 60 → 300 N·m
- torque range: 50 → 180 N·m

Notch and concentration modeling is explicit:

- notch sensitivity from `NotchSensitivity`
- theoretical factors: `Kt`, `Kts`
- fatigue factors: `Kf`, `Kfs` from M1a helpers

Endurance-limit modification uses M1 factors:

- surface factor from ultimate strength
- size factor from diameter
- bending load factor
- temperature factor
- reliability factor

The modified endurance limit is then used with equivalent alternating/mean stresses built by M2 shaft fatigue helpers.

## 5. Criterion comparison

For one fixed shaft/material/load case, the probe compares:

- Goodman
- Gerber
- Soderberg

All criteria share the same:

- equivalent alternating/mean stress pair
- modified endurance limit
- concentration assumptions

Observed ordering remains visible as expected:

- Gerber least conservative
- Goodman intermediate
- Soderberg most conservative

## 6. Observations

What felt clean:

- section geometry, stress, and equivalent stress are straightforward with M2 helpers
- notch sensitivity and fatigue concentration slot into the same pipeline without ceremony
- endurance modifiers remain explicit and inspectable
- criterion switching is compact once equivalent stresses are available

What still feels intentionally missing in this pass:

- shaft deflection and stiffness checks
- hollow/stepped shaft geometry transitions
- keyway-specific concentration lookup workflows
- bearings, support reactions, and full-shaft load path modeling
- cycle-counting / variable-amplitude fatigue life methods

These omissions are deliberate non-goals for this focused probe.

## 7. Conclusion

Mechanics M0 → M2 now compose into a practical critical-section shaft design probe. The workflow is compact, deterministic, and engineering-shaped, while still leaving larger shaft-system concerns explicitly out of scope.

## 8. Deliverable summary

1. **Experiment name/path**
   - `Experiments/MechanicsShaftDesignProbe`

2. **Oct artifacts added**
   - `manifest.oct`
   - `M0/mechanics_shaft_design_probe.oct`
   - `M0/mechanics_shaft_design_probe.octest`
   - `REPORT.md`

3. **Mechanics behaviors exercised**
   - M2 shaft section properties and shaft stresses
   - M2 static equivalent stress and yield FoS
   - M1a notch sensitivity and fatigue concentration factors
   - M1 endurance modifier composition and modified endurance limit
   - M0a fatigue criteria via M2 shaft wrappers (Goodman/Gerber/Soderberg)

4. **Tests validating claims**
   - section-property checks and stress trends
   - static FoS response to load increase
   - fatigue degradation under `Kf`, `Kfs`, and endurance modification
   - criterion ordering check (`Gerber > Goodman > Soderberg`)
   - deterministic end-to-end pipeline reconstruction

5. **Remaining friction intentionally left visible**
   - no deflection or full shaft-system modeling
   - no keyway lookup workflow
   - no life accumulation or variable-amplitude fatigue modeling
