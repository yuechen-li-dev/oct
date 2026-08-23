# Physics M0

## Purpose

`Libraries/Physics` provides reusable physical constants and a small set of unit-checked textbook mechanics equations.

## M0 scope

The mechanics surface includes force, momentum, kinetic and gravitational potential energy, work, average power, and linear-spring energy. `PositiveMass` and `PositiveDuration` make true domain preconditions explicit.

## Naming/style

Constants are exposed as functions (for example `SpeedOfLight()`, `BoltzmannConstant()`, `PlanckConstant()`) to align with existing function-style constant surfaces.

## Unit philosophy

Units are part of the returned values whenever directly representable in the current Oct unit system.
Constants that depend on currently unavailable base dimensions (for example electric current, amount of substance, or thermodynamic temperature) are documented and exposed with the closest explicit representation available today.

The mechanics equations use dimensions that Oct can fully represent:

```oct
let energy: Float<kg*m^2/s^2> = KineticEnergy(2.0kg, 3.0m/s)!
// 9 J in base SI dimensions

let power: Float<kg*m^2/s^3> = AveragePower(energy, 3.0s)!
// 3 W in base SI dimensions
```

`Physics.Mechanics.InvalidDimensions.octfail` proves that passing time as velocity is rejected by the typechecker.

## Non-goals

Physics M0 is not a formula catalog, simulator, electromagnetism model, or constants database.
