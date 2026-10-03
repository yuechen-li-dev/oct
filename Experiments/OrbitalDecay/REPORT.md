# OrbitalDecay Report

## M0: Low Earth Orbit Atmospheric Drag Simulation

### Objective
Explore Oct's statically checked dimensional units (`Length`, `Velocity`, `Time`, `Mass`, `Area`, `Density`, `Force`, `GravParam`) on a physical orbital mechanics simulation: tracking the altitude decay of a 500 kg satellite in Low Earth Orbit (LEO) subject to thermospheric drag.

### Physics Formulation
- Earth radius $R_E = 6,371\text{ km}$, gravitational parameter $\mu = 3.986004418 \times 10^{14}\text{ m}^3\text{s}^{-2}$.
- Circular velocity: $v = \sqrt{\mu / r}$ (units statically verified: $\text{m/s}$).
- Orbital period: $T = 2\pi r / v$ (units statically verified: $\text{s}$).
- Atmospheric density (exponential thermosphere profile):
  $\rho(h) = \rho_0 \exp\left(-\frac{h - h_0}{H}\right)$ where $h_0 = 250\text{ km}$, $\rho_0 = 7.248 \times 10^{-11}\text{ kg}\cdot\text{m}^{-3}$, and $H = 45\text{ km}$.
- Drag force: $F_d = \frac{1}{2} C_d A \rho v^2$ (units: $\text{kg}\cdot\text{m}\cdot\text{s}^{-2} = \text{N}$).
- Decay per revolution (from Keplerian circular orbit energy dissipation):
  $\Delta r_{\text{rev}} = 2\pi \left(\frac{C_d A}{m}\right) \rho r^2$.

### Results
Starting at an altitude of $300\text{ km}$ ($m = 500\text{ kg}$, $A = 1.5\text{ m}^2$, $C_d = 2.2$):
- **Initial Orbit (0)**: Altitude = 300.00 km, Orbital Speed = 7.730 km/s, Drag = 0.0024 N, Decay = 44.03 m/rev.
- **Midpoint (Orbit 500)**: Altitude = 269.97 km, Orbital Speed = 7.747 km/s, Drag = 0.0046 N, Decay = 85.06 m/rev.
- **De-orbit Threshold (200 km)**:
  - Reached after **922 revolutions**.
  - Total elapsed duration: **57.44 days** (1,378.5 hours).
  - Average decay rate: **108.72 m/revolution**.

### Verified Language Behaviors
1. Static dimensional checking correctly propagated across composite types: $\text{Sqrt}(\text{GravParam} / \text{Length}) \to \text{Velocity}$.
2. Compound dimensional exponent notation (`kg*m^-3`, `m^3*s^-2`, `kg*m*s^-2`) compiled cleanly in Go native compilation mode.
3. Both interpreted and native compiled test execution (`oct test --execution compiled`) succeeded with zero fallbacks.

---

## M1: Parametric Templates & Telemetry Analysis

### Objective
Explore Oct's **parametric templates** (`template record`, `template fn`) parameterized over dimensioned SI types (`Length`, `Velocity`, `Force`, `Time`), and verify monomorphization and unit preservation across statistical analysis and numerical integration.

### Implemented Parametric Constructs
1. **Generic Data Containers:**
   - `template record TimeSeries<T>`: Associates a timestamp array (`Float<s>[]`) with values (`T[]`).
   - `template record StatisticsEnvelope<T>`: Generic minimum, maximum, and mean statistical container.
   - `template record Measurement<T>`: Generic value and unit label container.
2. **Generic Reducers & Numerical Solvers:**
   - `template fn Min<T>(values: T[]) -> T`
   - `template fn Max<T>(values: T[]) -> T`
   - `template fn Mean<T>(values: T[]) -> T`
   - `template fn ComputeEnvelope<T>(values: T[]) -> StatisticsEnvelope<T>`
   - `template fn EulerStep<State, Derivative, Time>(state: State, derivative: Derivative, dt: Time) -> State`

### Findings & Telemetry Results
Simulating across 10 snapshot telemetry points sampled every 50 orbits:
- **Altitude Envelope:** Max = 300.00 km, Min = 274.03 km, Mean = 288.10 km (`Length` preserved).
- **Velocity Envelope:** Min = 7.730 km/s, Max = 7.745 km/s, Mean = 7.737 km/s (`Velocity` preserved).
- **Drag Envelope:** Min = 0.00235 N, Max = 0.00421 N, Mean = 0.00312 N (`Force` preserved).
- **Euler Integration:** `EulerStep<Length, Velocity, Time>(1000.0m, 50.0m/s, 10.0s)` cleanly monomorphized and evaluated to `1500.0m`.

### Takeaway on Oct Templates
- Monomorphization is clean and transparent; templates work seamlessly with physical dimension types.
- Generics preserve dimensional unit safety across arrays, records, and higher-order functions.
- Native compiled mode (`oct test --execution compiled`) compiles all template instantiations without runtime type dictionaries or reflection overhead.
