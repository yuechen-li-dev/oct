# Thermofluids M0

Thermofluids M0 provides lumped thermal and simple fluid process helpers for first-order process modeling.

## M0 scope

- Thermal: lumped `ThermalEnergy`, `TemperatureFromEnergy`, `HeatFlowNewtonCooling`, and explicit `FirstOrderThermalStep`.
- Fluid: cylindrical `TankAreaCylinder`/`TankVolumeCylinder`, explicit `TankLevelStep`, `OutflowTorricelli`, and `ResidenceTime`.

## Unit philosophy

M0 uses dimensioned values in both modules:

- Thermal uses `kg`, `m`, `s` for mass, specific heat, energy, heat flow, and time constants; temperature is currently scalar `Float` in M0.
- Fluid uses `m`, `s` for geometry, level, flow, gravity, and residence time.

No unit-erasing conversions are used inside the API.

## Validation and policy choices

- Thermal validates `mass > 0`, `cp > 0`, `area > 0`, `tau > 0`, `dt > 0`.
- Fluid validates `radius > 0`, `height >= 0`, `level >= 0`, `area > 0`, `dt > 0`, `outletArea > 0`, `gravity > 0`, `dischargeCoefficient >= 0`, and `flowRate > 0` for residence time.
- `TankLevelStep` clamps negative computed next level to `0m` (documented M0 non-negativity policy).
- `OutflowTorricelli` returns `0m^3/s` when `level == 0m` and rejects `level < 0m`.

## Non-goals

Thermofluids M0 is intentionally not CFD, not PDE heat transfer, not a property database, and not a full process simulator.
