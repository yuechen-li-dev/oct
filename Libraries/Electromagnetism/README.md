# Electromagnetism

## Shelf boundary

`Electromagnetism` owns electrostatics, ideal lumped circuits, magnetism, and induction. [`Physics`](../Physics/README.md) owns shared constants and generic laws; [`RF`](../RF/README.md) owns radio-frequency system mathematics, and [`Wireless`](../Wireless/README.md) owns communication/link calculations. Use [`DifferentialEquations`](../DifferentialEquations/README.md) or [`Simulation`](../Simulation/README.md) when an electrical model needs numerical time progression. Start with the RC quick start below.

This package is a bounded executable chapter from charge through induction:

1. point-charge force, field, and potential
2. capacitance and stored electric energy
3. Ohm law, power, resistor networks, and RC discharge
4. magnetic flux and force
5. ideal straight-wire field, Faraday induction, inductor energy, and RL response

Electrical dimensions are ordinary Oct SI expressions: charge is `A*s`, voltage is `kg*m^2/(A*s^3)`, resistance is `kg*m^2/(A^2*s^3)`, capacitance is `A^2*s^4/(kg*m^2)`, magnetic flux is `kg*m^2/(A*s^2)`, and inductance is `kg*m^2/(A^2*s^2)`. No string-named pseudo-units are used.

The models state their idealizations: point charges in vacuum, uniform parallel plates without fringing, ideal lumped R/C/L elements, uniform magnetic fields, and infinitely long thin wires. This is not a field solver or circuit simulator.

## Quick start

External consumers qualify calls and propagate domain failures:

```oct
import Electromagnetism

let resistance = 1000.0kg*m^2/s^3/A^2
let capacitance = 0.000001A^2*s^4/kg/m^2
let tau = Electromagnetism.RCTimeConstant(resistance, capacitance)?
let halfVoltageTime = tau * Ln(2.0)
```

Here `kg*m^2/(s^3*A^2)` is ohms and `A^2*s^4/(kg*m^2)` is farads. The product is seconds. “RC half-life” needs a quantity: voltage, charge, and discharge-current magnitude halve at `tau*ln(2)`, while stored energy halves at `tau*ln(2)/2`. `RCDischargeVoltage` rejects a zero time constant even though the algebraic `RCTimeConstant` helper can return zero.
