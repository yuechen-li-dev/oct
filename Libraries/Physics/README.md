# Physics

## Purpose

`Libraries/Physics` owns foundational physical laws and constants. Engineering analysis models—beams, fatigue, pressure vessels, and related design relations—remain in `Mechanics`.

Adjacent application owners are [`Mechanics`](../Mechanics/README.md), [`Thermofluids`](../Thermofluids/README.md), [`Electromagnetism`](../Electromagnetism/README.md), [`Optics`](../Optics/README.md), [`Quantum`](../Quantum/README.md), and [`Astrophysics`](../Astrophysics/README.md). They consume this package's constants and laws rather than redeclaring them. Start with `Physics.Constants.oct` or the force/energy facts in `Physics.Mechanics.octest`.

## Chapters

- constants: exact SI-defined constants, measured constants, and compatibility wrappers
- mechanics: force, momentum, energy, work, power, and ideal springs
- dynamics: constant-acceleration kinematics, impulse, circular motion, angular momentum, rotational energy, and two-body center of mass
- waves: wavelength/frequency/speed, angular frequency, wave number, beats, and ideal string/pipe standing modes

Refined Concepts such as `PositiveMass`, `PositiveDuration`, and `PositiveLength` express actual domain preconditions. Runtime values cross those boundaries through explicit fallible admission rather than unchecked construction.

## Naming/style

Constants are exposed as functions (for example `SpeedOfLight()`, `BoltzmannConstant()`, `PlanckConstant()`) to align with existing function-style constant surfaces.

## Unit philosophy

Units are part of the returned values. The current language supports the seven SI base dimensions, so the canonical constants use `K`, `A`, and `mol` directly: prefer `BoltzmannConstantSI`, `ElementaryChargeSI`, `AvogadroConstantSI`, `GasConstantSI`, `StefanBoltzmannConstantSI`, `VacuumPermittivitySI`, and `VacuumPermeabilitySI`. Earlier scalar or partially dimensional functions remain as compatibility wrappers.

The mechanics equations use dimensions that Oct can fully represent:

```oct
let energy: Float<kg*m^2/s^2> = KineticEnergy(2.0kg, 3.0m/s)!
// 9 J in base SI dimensions

let power: Float<kg*m^2/s^3> = AveragePower(energy, 3.0s)!
// 3 W in base SI dimensions
```

`Physics.Mechanics.InvalidDimensions.octfail` proves that passing time as velocity is rejected by the typechecker.

## Constants and model honesty

Comments distinguish exact SI-defined values from post-2019 measured/derived values and conventional rounded values. Kinematics assumes constant acceleration; standing-wave relations assume ideal boundaries; point-particle rotation helpers do not replace rigid-body dynamics.

Physics is not an indiscriminate formula catalog, simulator, electromagnetic field solver, or metrology database.
