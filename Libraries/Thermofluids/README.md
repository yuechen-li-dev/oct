# Thermofluids

Thermofluids is an executable textbook chapter spanning thermodynamics, foundational fluid mechanics, dimensionless analysis, and bounded heat transfer. The original lumped process helpers remain compatible.

This package supplies thermal/fluid equations; it does not own general time integrators or scenario runners. Use [`DifferentialEquations`](../DifferentialEquations/README.md) to integrate a cooling ODE and [`Simulation`](../Simulation/README.md) for traces or parameter sweeps. [`Physics`](../Physics/README.md) owns shared constants, while [`Cooking`](../Cooking/README.md) owns culinary applications. Start with the heat-transfer quick start below.

## Chapters

- Thermodynamics: ideal-gas state relations, heat capacities, first law, isentropic temperature, efficiencies, COP, sensible and latent heat.
- Fluids: hydrostatics, buoyancy, continuity, Bernoulli terms, head, hydraulic diameter, Reynolds-regime friction, and Darcy-Weisbach loss.
- Dimensionless analysis: Reynolds, Mach, Froude, Euler, Prandtl, Nusselt, Grashof, Rayleigh, Biot, Fourier, Peclet, Weber, Schmidt, and Sherwood.
- Heat transfer: plane-wall conduction/resistance, convection, diffuse-gray radiation, and lumped-capacitance transients.
- Process compatibility: cylindrical tanks, Torricelli outflow, residence time, and the original scalar-temperature first-order helpers.

## Unit philosophy

New M1 physical APIs retain `K`, `A`, and `mol` dimensions wherever relevant. The older `ThermalEnergy`/`HeatFlowNewtonCooling` scalar-temperature APIs remain for compatibility; prefer `SensibleHeat`, `ConvectionHeatRate`, and `LumpedTemperature` for new code.

Empirical correlations are deliberately bounded. `LaminarPipeFrictionFactor` rejects Reynolds numbers above 2300. `HaalandFrictionFactor` rejects transition flow, Reynolds below 4000, and relative roughness outside `[0, 0.05]`.

No unit-erasing conversions are used inside the API.

## Heat-transfer quick start

For a plane wall with inside/outside films, the steady one-dimensional model is `Qdot = (T_inside - T_outside) / R_total`:

```oct
import Thermofluids

let area = 12.0m^2
let inside = Thermofluids.ConvectiveThermalResistance(8.0kg/s^3/K, area)?
let wall = Thermofluids.PlaneWallThermalResistance(0.15m, 0.04kg*m/s^3/K, area)?
let outside = Thermofluids.ConvectiveThermalResistance(25.0kg/s^3/K, area)?
let total = Thermofluids.SeriesThermalResistance([inside, wall, outside])?
let heatRate = Thermofluids.HeatRateFromResistance(293.15K - 273.15K, total)?
```

The base-SI spellings are `kg*m/s^3/K = W/(m*K)`, `kg/s^3/K = W/(m^2*K)`, `K*s^3/kg/m^2 = K/W`, and `kg*m^2/s^3 = W`. A negative heat rate indicates flow opposite the chosen temperature difference.

## Validation and policy choices

- Thermal validates `mass > 0`, `cp > 0`, `area > 0`, `tau > 0`, `dt > 0`.
- Fluid validates `radius > 0`, `height >= 0`, `level >= 0`, `area > 0`, `dt > 0`, `outletArea > 0`, `gravity > 0`, `dischargeCoefficient >= 0`, and `flowRate > 0` for residence time.
- `TankLevelStep` clamps negative computed next level to `0m` (documented M0 non-negativity policy).
- `OutflowTorricelli` returns `0m^3/s` when `level == 0m` and rejects `level < 0m`.

## Model honesty and non-goals

Bernoulli is the steady, incompressible, inviscid streamline equation without pumps, turbines, or loss. Ideal-gas functions do not model real-gas properties. Beam-like one-dimensional conduction assumes constant properties. Lumped transients require negligible internal gradients; `Bi < 0.1` is exposed as a conventional screening helper, not a proof.

This package is not CFD, a thermodynamic property database, a steam table, a PDE heat solver, or a general process simulator.
