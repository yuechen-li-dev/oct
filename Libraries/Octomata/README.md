# Octomata M0

## Package family purpose

`Libraries/Octomata` is the package family for reusable control/estimation components layered on top of Octomata core.
Octomata core remains the execution/control substrate; this family extends it with reusable discrete-time components.

## Current M0 scope

- `Octomata.PID`: explicit discrete-time PID update helper
- `Octomata.Filters`: explicit discrete-time low-pass filter update helper

M0 is dimensionless `Float`-only, deterministic, and uses explicit state passing.

## Future family shape

More advanced estimators/controllers may be added later under this same `Octomata` family.

## Explicit non-goals

M0 is not a full control-systems framework and does not include continuous-time math tooling,
transfer-function toolboxes, Kalman filtering, LQR, MPC, or observer frameworks.
