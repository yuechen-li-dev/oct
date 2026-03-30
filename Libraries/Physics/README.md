# Physics M0

## Purpose

`Libraries/Physics` provides reusable physical constants for scientific and engineering packages.

## M0 scope

Physics M0 is constants-only. It intentionally does not include formula catalogs, simulators, or broader domain models.

## Naming/style

Constants are exposed as functions (for example `SpeedOfLight()`, `BoltzmannConstant()`, `PlanckConstant()`) to align with existing function-style constant surfaces.

## Unit philosophy

Units are part of the returned values whenever directly representable in the current Oct unit system.
Constants that depend on currently unavailable base dimensions (for example electric current, amount of substance, or thermodynamic temperature) are documented and exposed with the closest explicit representation available today.

## Non-goals

Physics M0 is not a full physics formula library.
